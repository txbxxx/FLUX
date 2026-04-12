package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/internal/service/git"
	"ai-sync-manager/internal/service/sync"
	typesSync "ai-sync-manager/internal/types/sync"
)

// SyncService defines the interface for sync-level operations (packaging, git interaction).
type SyncService interface {
	PushSnapshot(ctx context.Context, snapshot *models.Snapshot, options sync.SyncOptions) (*sync.PushResult, error)
	ValidateRemoteConfig(ctx context.Context, config *models.RemoteConfig) error
}

// SyncPush pushes the latest snapshot of a project to its configured remote repository.
//
// 流程：
//  1. 查找 project 最新快照
//  2. 查找 project 的远端配置
//  3. 确保工作目录存在（不存在则 clone）
//  4. 从 SQLite 读文件 → 写入工作目录 → Git commit + push
func (w *LocalWorkflow) SyncPush(ctx context.Context, input typesSync.SyncPushInput) (*typesSync.SyncPushResult, error) {
	// 第一步：参数校验
	projectName := strings.TrimSpace(input.Project)
	if projectName == "" && !input.All {
		return nil, &UserError{
			Message:    "推送失败：请指定项目名称",
			Suggestion: "使用 --project 指定项目，或 --all 推送所有项目",
		}
	}

	if w.remoteConfigs == nil {
		return nil, &UserError{
			Message:    "推送失败：远端配置服务不可用",
			Suggestion: "请先执行 ai-sync remote add <url>",
		}
	}

	// 第二步：获取远端配置
	configs, err := w.remoteConfigs.ListRemoteConfigs()
	if err != nil || len(configs) == 0 {
		return nil, &UserError{
			Message:    "推送失败：未配置远端仓库",
			Suggestion: "请先执行 ai-sync remote add <url> 添加远端仓库",
		}
	}

	// 使用第一个活跃的配置（简化实现，后续可按 project 绑定）
	var remoteConfig *models.RemoteConfig
	for _, cfg := range configs {
		if cfg.Status == models.StatusActive {
			remoteConfig = cfg
			break
		}
	}
	if remoteConfig == nil {
		return nil, &UserError{
			Message:    "推送失败：没有可用的活跃远端配置",
			Suggestion: "请先执行 ai-sync remote add 添加一个有效的远端仓库",
		}
	}

	// 第三步：查找 project 最新快照
	if w.snapshots == nil {
		return nil, &UserError{
			Message:    "推送失败：快照服务不可用",
			Suggestion: "请重新启动程序",
		}
	}
	items, err := w.snapshots.ListSnapshots(0, 0)
	if err != nil {
		return nil, &UserError{
			Message:    "推送失败：无法读取快照列表",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	var targetID string
	for _, item := range items {
		if item.Project == projectName {
			targetID = item.ID
			break
		}
	}
	if targetID == "" {
		return nil, &UserError{
			Message:    "推送失败：项目 \"" + projectName + "\" 没有快照",
			Suggestion: "请先执行 ai-sync snapshot create -p " + projectName + " 创建快照",
		}
	}

	snapshot, err := w.snapshots.GetSnapshot(targetID)
	if err != nil {
		return nil, &UserError{
			Message:    "推送失败：无法读取快照数据",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	// 第四步：确保工作目录存在
	dataDir := w.dataDir
	repoPath := filepath.Join(dataDir, "repos", projectName)

	gitClient := git.NewGitClient()
	if !git.IsRepository(repoPath) {
		// 尝试 clone 远端仓库
		cloneOpts := &git.CloneOptions{
			URL:    remoteConfig.URL,
			Path:   repoPath,
			Auth:   convertAuthFromModel(&remoteConfig.Auth),
			Branch: remoteConfig.Branch,
		}
		if _, cloneErr := gitClient.Clone(ctx, cloneOpts); cloneErr != nil {
			// clone 失败则初始化新仓库
			if _, initErr := git.InitRepository(repoPath, false); initErr != nil {
				return nil, &UserError{
					Message:    "推送失败：无法创建工作目录",
					Suggestion: "请检查磁盘空间和文件权限",
					Err:        initErr,
				}
			}
			_ = git.AddRemote(repoPath, "origin", remoteConfig.URL)
		}
	}

	// 第五步：从 SQLite 读文件 → 写入工作目录
	for _, file := range snapshot.Files {
		if len(file.Content) == 0 {
			continue
		}
		targetPath := filepath.Join(repoPath, projectName, file.Path)
		dir := filepath.Dir(targetPath)
		if mkErr := mkdirAll(dir); mkErr != nil {
			continue
		}
		_ = writeFile(targetPath, file.Content)
	}

	// 第六步：Git add + commit
	commitMsg := fmt.Sprintf("Snapshot: %s\nID: %s\nProject: %s\nFiles: %d",
		snapshot.Name, snapshot.ID, snapshot.Project, len(snapshot.Files))
	commitResult, err := gitClient.Commit(ctx, &git.CommitOptions{
		Path:    repoPath,
		Message: commitMsg,
		All:     true,
	})
	if err != nil {
		// commit 失败可能是因为没有变更
		if strings.Contains(err.Error(), "nothing to commit") {
			return &typesSync.SyncPushResult{
				Success:     true,
				Project:     projectName,
				FilesPushed: 0,
				RemoteURL:   remoteConfig.URL,
			}, nil
		}
		return nil, &UserError{
			Message:    "推送失败：Git commit 失败",
			Suggestion: "请检查工作目录状态",
			Err:        err,
		}
	}

	// 第七步：Git push
	_, err = gitClient.Push(ctx, &git.PushOptions{
		Path:       repoPath,
		Auth:       convertAuthFromModel(&remoteConfig.Auth),
		RemoteName: "origin",
		Branch:     remoteConfig.Branch,
	})
	if err != nil {
		// push 失败不影响本地
		return nil, &UserError{
			Message:    "推送失败：无法推送到远端",
			Suggestion: "请检查网络连接和仓库权限。本地数据未受影响。",
			Err:        err,
		}
	}

	commitHash := ""
	if commitResult != nil {
		commitHash = commitResult.CommitHash
	}

	return &typesSync.SyncPushResult{
		Success:     true,
		Project:     projectName,
		FilesPushed: len(snapshot.Files),
		CommitHash:  commitHash,
		RemoteURL:   remoteConfig.URL,
	}, nil
}

// SyncPull pulls the latest configuration from remote and updates SQLite.
func (w *LocalWorkflow) SyncPull(ctx context.Context, input typesSync.SyncPullInput) (*typesSync.SyncPullResult, error) {
	projectName := strings.TrimSpace(input.Project)
	if projectName == "" {
		return nil, &UserError{
			Message:    "拉取失败：请指定项目名称",
			Suggestion: "使用 --project 指定项目",
		}
	}

	return nil, &UserError{
		Message:    "拉取功能正在开发中",
		Suggestion: "请等待下一版本更新",
	}
}

// SyncStatus checks the sync status of a project.
func (w *LocalWorkflow) SyncStatus(ctx context.Context, input typesSync.SyncStatusInput) (*typesSync.SyncStatusResult, error) {
	return nil, &UserError{
		Message:    "同步状态查询正在开发中",
		Suggestion: "请等待下一版本更新",
	}
}

// convertAuthFromModel converts models.AuthConfig to git.GitAuthConfig.
func convertAuthFromModel(auth *models.AuthConfig) *git.GitAuthConfig {
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

// mkdirAll creates a directory and all parent directories.
func mkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

// writeFile writes data to a file.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
