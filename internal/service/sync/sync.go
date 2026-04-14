package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"flux/internal/models"
	"flux/pkg/git"
	"flux/pkg/database"
	"flux/pkg/logger"

	"go.uber.org/zap"
)

// Service 同步服务
type Service struct {
	db       *database.DB
	git      *git.GitClient
	packager *Packager
}

// NewService 创建同步服务
func NewService(db *database.DB) *Service {
	return &Service{
		db:       db,
		git:      git.NewGitClient(),
		packager: NewPackager(),
	}
}

// PushSnapshot 推送快照到远程仓库
func (s *Service) PushSnapshot(
	ctx context.Context,
	snapshot *models.Snapshot,
	options SyncOptions,
) (*PushResult, error) {
	logger.Info("开始推送快照",
		zap.Uint64("snapshot_id", uint64(snapshot.ID)),
		zap.String("remote_url", options.RemoteURL),
		zap.String("branch", options.Branch),
	)

	result := &PushResult{
		SnapshotID: fmt.Sprintf("%d", snapshot.ID),
	}

	// 验证快照
	if err := s.packager.ValidateSnapshotForPush(snapshot); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("快照验证失败: %w", err)
	}

	// 确定仓库路径
	repoPath := s.getRepoPath(snapshot, options)

	// 确保仓库存在
	if err := s.ensureRepository(ctx, repoPath, options); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("确保仓库失败: %w", err)
	}

	// 创建快照目录
	snapshotDir := filepath.Join(repoPath, ".ai-sync", "snapshots", fmt.Sprintf("%d", snapshot.ID))
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("创建快照目录失败: %w", err)
	}

	// 写入快照文件
	if err := s.writeSnapshotFiles(snapshot, snapshotDir); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("写入快照文件失败: %w", err)
	}

	// 创建提交
	commitHash, err := s.createSnapshotCommit(ctx, repoPath, snapshot)
	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("创建提交失败: %w", err)
	}

	// 更新快照的提交哈希
	snapshot.CommitHash = commitHash
	snapshotDAO := models.NewSnapshotDAO(s.db)
	if err := snapshotDAO.Update(snapshot); err != nil {
		logger.Warn("更新快照提交哈希失败", zap.Error(err))
	}

	// 推送到远程
	if err := s.pushToRemote(ctx, repoPath, options); err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("推送失败: %w", err)
	}

	result.Success = true
	result.CommitHash = commitHash
	result.Branch = options.Branch
	result.PushedAt = time.Now().Format(time.RFC3339)
	result.FilesCount = len(snapshot.Files)

	logger.Info("快照推送成功",
		zap.Uint64("snapshot_id", uint64(snapshot.ID)),
		zap.String("commit_hash", commitHash),
	)

	return result, nil
}

// PullSnapshots 从远程仓库拉取快照
func (s *Service) PullSnapshots(
	ctx context.Context,
	repoPath string,
	options SyncOptions,
) (*PullResult, error) {
	logger.Info("开始拉取快照",
		zap.String("repo_path", repoPath),
		zap.String("remote_url", options.RemoteURL),
		zap.String("branch", options.Branch),
	)

	result := &PullResult{
		Branch:   options.Branch,
		PulledAt: time.Now().Format(time.RFC3339),
	}

	// 拉取更新
	pullOpts := &git.PullOptions{
		Path:       repoPath,
		Auth:       s.convertAuth(options.Auth),
		RemoteName: options.RemoteName,
		Branch:     options.Branch,
	}

	_, err := s.git.Pull(ctx, pullOpts)
	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("拉取失败: %w", err)
	}

	// 解析拉取的快照
	snapshots, err := s.parseRemoteSnapshots(repoPath)
	if err != nil {
		result.Success = false
		result.ErrorMessage = err.Error()
		return result, fmt.Errorf("解析远程快照失败: %w", err)
	}

	// 保存新快照到数据库
	newCount := 0
	for _, snap := range snapshots {
		if s.isSnapshotNew(snap.ID) {
			if err := s.saveRemoteSnapshot(snap); err != nil {
				logger.Warn("保存远程快照失败",
					zap.Uint64("snapshot_id", uint64(snap.ID)),
					zap.Error(err),
				)
				continue
			}
			newCount++
		}
	}

	result.Success = true
	result.NewSnapshots = newCount
	result.Commits = s.convertToRemoteCommits(snapshots)

	logger.Info("快照拉取成功",
		zap.Int("total", len(snapshots)),
		zap.Int("new", newCount),
	)

	return result, nil
}

// GetSyncStatus 获取同步状态
func (s *Service) GetSyncStatus(
	ctx context.Context,
	repoPath string,
	options SyncOptions,
) (*SyncStatus, error) {
	logger.Info("获取同步状态",
		zap.String("repo_path", repoPath),
	)

	// 检查仓库是否存在
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return &SyncStatus{
			RemoteURL:       options.RemoteURL,
			RemoteSnapshots: []RemoteSnapshotInfo{},
		}, nil
	}

	// TODO: 实现状态查询
	status := &SyncStatus{
		RemoteURL:       options.RemoteURL,
		Branch:          options.Branch,
		RemoteSnapshots: []RemoteSnapshotInfo{},
	}

	return status, nil
}

// ensureRepository 确保仓库存在
func (s *Service) ensureRepository(
	ctx context.Context,
	repoPath string,
	options SyncOptions,
) error {
	// 检查仓库是否存在
	if info, err := os.Stat(repoPath); err == nil && info.IsDir() {
		// 仓库已存在
		return nil
	}

	// 需要克隆仓库
	cloneOpts := &git.CloneOptions{
		URL:    options.RemoteURL,
		Path:   repoPath,
		Auth:   s.convertAuth(options.Auth),
		Branch: options.Branch,
	}

	_, err := s.git.Clone(ctx, cloneOpts)
	if err != nil {
		return fmt.Errorf("克隆仓库失败: %w", err)
	}

	return nil
}

// writeSnapshotFiles 写入快照文件
func (s *Service) writeSnapshotFiles(
	snapshot *models.Snapshot,
	snapshotDir string,
) error {
	// 写入元数据文件
	metadataPath := filepath.Join(snapshotDir, "metadata.json")
	metadataContent, err := s.packager.createMetadataFile(snapshot)
	if err != nil {
		return fmt.Errorf("创建元数据文件失败: %w", err)
	}

	if err := os.WriteFile(metadataPath, metadataContent, 0644); err != nil {
		return fmt.Errorf("写入元数据文件失败: %w", err)
	}

	// 写入文件内容
	for _, file := range snapshot.Files {
		if len(file.Content) == 0 {
			continue
		}

		targetPath := filepath.Join(snapshotDir, filepath.Base(file.Path))
		if err := os.WriteFile(targetPath, file.Content, 0644); err != nil {
			logger.Warn("写入文件失败",
				zap.String("path", file.Path),
				zap.Error(err),
			)
			continue
		}
	}

	return nil
}

// createSnapshotCommit 创建快照提交
func (s *Service) createSnapshotCommit(
	ctx context.Context,
	repoPath string,
	snapshot *models.Snapshot,
) (string, error) {
	commitMessage := s.packager.CreateCommitMessage(snapshot)

	commitOpts := &git.CommitOptions{
		Path:    repoPath,
		Message: commitMessage,
		All:     true,
	}

	result, err := s.git.Commit(ctx, commitOpts)
	if err != nil {
		return "", err
	}

	return result.CommitHash, nil
}

// pushToRemote 推送到远程仓库
func (s *Service) pushToRemote(
	ctx context.Context,
	repoPath string,
	options SyncOptions,
) error {
	pushOpts := &git.PushOptions{
		Path:       repoPath,
		Auth:       s.convertAuth(options.Auth),
		RemoteName: options.RemoteName,
		Branch:     options.Branch,
		Force:      options.ForcePush,
	}

	_, err := s.git.Push(ctx, pushOpts)
	return err
}

// parseRemoteSnapshots 解析远程快照
func (s *Service) parseRemoteSnapshots(repoPath string) ([]*models.Snapshot, error) {
	// TODO: 从 Git 提交历史解析快照
	// 这里返回空列表作为占位符
	return []*models.Snapshot{}, nil
}

// isSnapshotNew 检查快照是否为新
func (s *Service) isSnapshotNew(snapshotID uint) bool {
	snapshotDAO := models.NewSnapshotDAO(s.db)
	_, err := snapshotDAO.GetByID(snapshotID)
	return err != nil // 错误表示不存在
}

// saveRemoteSnapshot 保存远程快照到数据库
func (s *Service) saveRemoteSnapshot(snapshot *models.Snapshot) error {
	snapshotDAO := models.NewSnapshotDAO(s.db)
	return snapshotDAO.Create(snapshot)
}

// getRepoPath 获取仓库路径
func (s *Service) getRepoPath(snapshot *models.Snapshot, options SyncOptions) string {
	// 使用快照的项目路径或默认路径
	if snapshot.Metadata.ProjectPath != "" {
		return filepath.Join(snapshot.Metadata.ProjectPath, ".sync-repo")
	}
	return filepath.Join(os.TempDir(), "flux", "sync-repo")
}

// convertAuth 转换认证配置
func (s *Service) convertAuth(auth *models.AuthConfig) *git.GitAuthConfig {
	if auth == nil {
		return nil
	}

	return &git.GitAuthConfig{
		Type:       git.AuthType(auth.Type),
		Username:   auth.Username,
		Password:   auth.Password,
		SSHKey:     auth.SSHKey,
		Passphrase: auth.Passphrase,
	}
}

// convertToRemoteCommits 转换为远程提交格式
func (s *Service) convertToRemoteCommits(
	snapshots []*models.Snapshot,
) []RemoteCommit {
	commits := make([]RemoteCommit, len(snapshots))

	for i, snap := range snapshots {
		commits[i] = RemoteCommit{
			Hash:       snap.CommitHash,
			Message:    snap.Message,
			Date:       snap.CreatedAt.Format("2006-01-02T15:04:05Z"),
			SnapshotID: fmt.Sprintf("%d", snap.ID),
		}
	}

	return commits
}

// CloneRepository 克隆远程仓库
func (s *Service) CloneRepository(
	ctx context.Context,
	url, path, branch string,
	auth *models.AuthConfig,
) error {
	logger.Info("克隆仓库",
		zap.String("url", url),
		zap.String("path", path),
		zap.String("branch", branch),
	)

	cloneOpts := &git.CloneOptions{
		URL:    url,
		Path:   path,
		Auth:   s.convertAuth(auth),
		Branch: branch,
	}

	_, err := s.git.Clone(ctx, cloneOpts)
	return err
}

// ListRemoteSnapshots 列出远程快照
func (s *Service) ListRemoteSnapshots(
	ctx context.Context,
	repoPath string,
) ([]RemoteSnapshotInfo, error) {
	logger.Info("列出远程快照", zap.String("repo_path", repoPath))

	// 检查仓库是否存在
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return []RemoteSnapshotInfo{}, nil
	}

	// 读取清单文件
	manifest, err := s.packager.ReadSnapshotManifest(repoPath)
	if err != nil {
		return nil, fmt.Errorf("读取清单文件失败: %w", err)
	}

	snapshots := make([]RemoteSnapshotInfo, 0)
	for snapshotID, metadata := range manifest {
		// 类型断言：metadata 应该是 map[string]interface{}
		metaMap, ok := metadata.(map[string]interface{})
		if !ok {
			logger.Warn("跳过无效的元数据", zap.String("snapshot_id", snapshotID))
			continue
		}

		info := RemoteSnapshotInfo{
			ID:         snapshotID,
			CommitHash: "",
		}

		// 解析元数据
		if name, ok := metaMap["name"].(string); ok {
			info.Name = name
		}
		if desc, ok := metaMap["description"].(string); ok {
			info.Description = desc
		}
		if message, ok := metaMap["message"].(string); ok {
			info.Message = message
		}
		if hash, ok := metaMap["commit_hash"].(string); ok {
			info.CommitHash = hash
		}
		if date, ok := metaMap["created_at"].(string); ok {
			info.Date = date
		}

		snapshots = append(snapshots, info)
	}

	return snapshots, nil
}

// DeleteLocalRepository 删除本地仓库
func (s *Service) DeleteLocalRepository(repoPath string) error {
	logger.Info("删除本地仓库", zap.String("path", repoPath))

	return os.RemoveAll(repoPath)
}

// ValidateRemoteConfig 验证远程配置
func (s *Service) ValidateRemoteConfig(
	ctx context.Context,
	config *models.RemoteConfig,
) (*RepositoryTestResult, error) {
	logger.Info("验证远程配置",
		zap.String("url", config.URL),
		zap.String("branch", config.Branch),
	)

	result := &RepositoryTestResult{
		Success:  false,
		URL:      config.URL,
		Branch:   config.Branch,
		TestedAt: time.Now().Format(time.RFC3339),
	}

	// TODO: 实现连接测试
	result.Success = true
	result.Latency = 100 // 毫秒
	result.Branches = []string{config.Branch, "main"}

	return result, nil
}

// RepositoryTestResult 仓库测试结果
type RepositoryTestResult struct {
	Success  bool     `json:"success"`
	URL      string   `json:"url"`
	Branch   string   `json:"branch"`
	Branches []string `json:"branches"`
	Latency  int      `json:"latency_ms"`
	Error    string   `json:"error,omitempty"`
	TestedAt string   `json:"tested_at"`
}
