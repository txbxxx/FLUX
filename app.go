package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/internal/service/crypto"
	"ai-sync-manager/internal/service/git"
	snapshotpkg "ai-sync-manager/internal/service/snapshot"
	"ai-sync-manager/internal/service/sync"
	"ai-sync-manager/internal/service/tool"
	"ai-sync-manager/pkg/database"
	"ai-sync-manager/pkg/logger"
	"go.uber.org/zap"
)

// App struct
type App struct {
	ctx          context.Context
	version      string
	db           *database.DB
	toolDetector *tool.ToolDetector
	snapshotSvc  *snapshotpkg.Service
	syncSvc      *sync.Service
	cryptoSvc    *crypto.Service
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		version: "1.0.0-alpha",
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 初始化日志
	if err := logger.Init(logger.DefaultConfig()); err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
	}

	logger.Info("AI Sync Manager starting",
		zap.String("version", a.version),
	)

	// 初始化数据库
	dataDir := getDataDir()
	db, err := database.InitDB(dataDir)
	if err != nil {
		logger.Error("初始化数据库失败", zap.Error(err))
	} else {
		a.db = db
		logger.Info("数据库初始化成功")
	}

	// 初始化工具检测器
	a.toolDetector = tool.NewToolDetector()

	// 初始化快照服务
	if a.db != nil {
		a.snapshotSvc = snapshotpkg.NewService(a.db, a.toolDetector)
		a.syncSvc = sync.NewService(a.db)
		logger.Info("服务初始化完成")
	}

	// 初始化加密服务
	encryptionConfig := &models.EncryptionConfig{
		Enabled:   false, // 默认不启用，需要用户配置
		Algorithm: "aes256-gcm",
	}
	cryptoSvc, err := crypto.NewService(encryptionConfig)
	if err != nil {
		logger.Warn("加密服务初始化失败", zap.Error(err))
	} else {
		a.cryptoSvc = cryptoSvc
		logger.Info("加密服务初始化完成")
	}
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	logger.Info("AI Sync Manager shutting down")

	// 关闭数据库
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			logger.Error("关闭数据库失败", zap.Error(err))
		}
	}

	_ = logger.Sync()
}

// getDataDir 获取数据目录
func getDataDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".ai-sync-manager")
}

// ==================== 基础接口（测试用）====================

// Hello 返回问候语
func (a *App) Hello(name string) string {
	logger.Debug("Hello called", zap.String("name", name))
	return fmt.Sprintf("Hello, %s!", name)
}

// GetVersion 获取应用版本
func (a *App) GetVersion() string {
	return a.version
}

// GetSystemInfo 获取系统信息
func (a *App) GetSystemInfo() map[string]string {
	return map[string]string{
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"go_version":  runtime.Version(),
		"app_version": a.version,
	}
}

// HealthCheck 健康检查
func (a *App) HealthCheck() map[string]interface{} {
	return map[string]interface{}{
		"status":  "ok",
		"version": a.version,
		"os":      runtime.GOOS,
		"arch":    runtime.GOARCH,
	}
}

// ==================== 工具检测接口 ====================

// ScanTools 扫描本地 AI 工具（仅全局配置）
func (a *App) ScanTools() ([]tool.ToolInstallation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("开始扫描工具")

	detector := tool.NewToolDetector()

	// 默认扫描选项：只扫描全局配置
	opts := &tool.ScanOptions{
		ScanGlobal:   true,
		ScanProjects:  false,
		IncludeFiles: true,
		MaxDepth:     1,
	}

	result, err := detector.DetectWithOptions(ctx, opts)
	if err != nil {
		logger.Error("工具扫描失败", zap.Error(err))
		return nil, err
	}

	installations := result.ToInstallations()
	logger.Info("工具扫描完成", zap.Int("count", len(installations)))

	return installations, nil
}

// ScanToolsWithProjects 扫描工具（包含项目配置）
func (a *App) ScanToolsWithProjects(projectPaths []string) ([]tool.ToolInstallation, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("开始扫描工具（含项目配置）", zap.Int("projects", len(projectPaths)))

	detector := tool.NewToolDetector()

	opts := &tool.ScanOptions{
		ScanGlobal:   true,
		ScanProjects:  true,
		ProjectPaths:  projectPaths,
		IncludeFiles:  true,
		MaxDepth:     1,
	}

	result, err := detector.DetectWithOptions(ctx, opts)
	if err != nil {
		logger.Error("工具扫描失败", zap.Error(err))
		return nil, err
	}

	installations := result.ToInstallations()
	logger.Info("工具扫描完成", zap.Int("count", len(installations)))

	return installations, nil
}

// GetToolConfigPath 获取工具配置路径
func (a *App) GetToolConfigPath(toolTypeStr string) (string, error) {
	toolType := tool.ToolType(toolTypeStr)
	path := tool.GetDefaultGlobalPath(toolType)

	if !isDirExists(path) {
		return "", fmt.Errorf("工具配置路径不存在: %s", path)
	}

	return path, nil
}

// ==================== 快照管理接口 ====================

// CreateSnapshotInput 创建快照输入参数
type CreateSnapshotInput struct {
	Message    string   `json:"message"`     // 快照描述
	Tools      []string `json:"tools"`       // 包含的工具
	Name       string   `json:"name"`        // 快照名称（可选）
	ProjectPath string  `json:"project_path"` // 项目路径（可选）
}

// SnapshotInfo 快照简要信息
type SnapshotInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   string    `json:"created_at"`
	Tools       []string  `json:"tools"`
	FileCount   int       `json:"file_count"`
	Size        int64     `json:"size"`
}

// CreateSnapshot 创建配置快照
func (a *App) CreateSnapshot(input CreateSnapshotInput) (*SnapshotInfo, error) {
	if a.snapshotSvc == nil {
		return nil, fmt.Errorf("快照服务未初始化")
	}

	logger.Info("创建快照请求",
		zap.String("message", input.Message),
		zap.Strings("tools", input.Tools),
		zap.String("name", input.Name),
	)

	// 构建创建选项
	options := models.CreateSnapshotOptions{
		Message:     input.Message,
		Tools:       input.Tools,
		Name:        input.Name,
		ProjectPath: input.ProjectPath,
		Scope:       models.ScopeGlobal, // 默认全局范围
		Tags:        []string{},
	}

	// 调用服务层创建快照
	pkg, err := a.snapshotSvc.CreateSnapshot(options)
	if err != nil {
		logger.Error("创建快照失败", zap.Error(err))
		return nil, err
	}

	// 返回简要信息
	info := &SnapshotInfo{
		ID:          pkg.Snapshot.ID,
		Name:        pkg.Snapshot.Name,
		Description: pkg.Snapshot.Description,
		CreatedAt:   pkg.Snapshot.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Tools:       pkg.Snapshot.Tools,
		FileCount:   pkg.FileCount,
		Size:        pkg.Size,
	}

	logger.Info("快照创建成功",
		zap.String("id", info.ID),
		zap.String("name", info.Name),
		zap.Int("file_count", info.FileCount),
	)

	return info, nil
}

// ListSnapshots 列出本地快照
func (a *App) ListSnapshots(limit int, offset int) ([]SnapshotInfo, error) {
	if a.snapshotSvc == nil {
		return nil, fmt.Errorf("快照服务未初始化")
	}

	logger.Info("列出快照请求",
		zap.Int("limit", limit),
		zap.Int("offset", offset),
	)

	snapshots, err := a.snapshotSvc.ListSnapshots(limit, offset)
	if err != nil {
		logger.Error("列出快照失败", zap.Error(err))
		return nil, err
	}

	// 转换为简要信息
	infoList := make([]SnapshotInfo, len(snapshots))
	for i, snap := range snapshots {
		infoList[i] = SnapshotInfo{
			ID:          snap.ID,
			Name:        snap.Name,
			Description: snap.Description,
			CreatedAt:   snap.CreatedAt.Format("2006-01-02T15:04:05Z"),
			Tools:       snap.Tools,
			FileCount:   len(snap.Files),
			Size:        calculateTotalSize(snap.Files),
		}
	}

	logger.Info("列出快照成功", zap.Int("count", len(infoList)))
	return infoList, nil
}

// GetSnapshotDetail 获取快照详情
func (a *App) GetSnapshotDetail(id string) (*models.Snapshot, error) {
	if a.snapshotSvc == nil {
		return nil, fmt.Errorf("快照服务未初始化")
	}

	logger.Info("获取快照详情", zap.String("id", id))

	return a.snapshotSvc.GetSnapshot(id)
}

// DeleteSnapshot 删除快照
func (a *App) DeleteSnapshot(id string) error {
	if a.snapshotSvc == nil {
		return fmt.Errorf("快照服务未初始化")
	}

	logger.Info("删除快照", zap.String("id", id))

	return a.snapshotSvc.DeleteSnapshot(id)
}

// ApplySnapshotInput 应用快照输入参数
type ApplySnapshotInput struct {
	SnapshotID   string `json:"snapshot_id"`   // 快照 ID
	CreateBackup bool   `json:"create_backup"` // 是否创建备份
	Force        bool   `json:"force"`         // 是否强制覆盖
	DryRun       bool   `json:"dry_run"`       // 是否仅预览
}

// ApplySnapshotInfo 应用结果信息
type ApplySnapshotInfo struct {
	Success       bool               `json:"success"`
	AppliedCount  int                `json:"applied_count"`
	SkippedCount  int                `json:"skipped_count"`
	Errors        []string           `json:"errors,omitempty"`
	CreatedFiles  []string           `json:"created_files,omitempty"`
	UpdatedFiles  []string           `json:"updated_files,omitempty"`
	Summary       models.ChangeSummary `json:"summary"`
}

// ApplySnapshot 应用快照到本地
func (a *App) ApplySnapshot(input ApplySnapshotInput) (*ApplySnapshotInfo, error) {
	if a.snapshotSvc == nil {
		return nil, fmt.Errorf("快照服务未初始化")
	}

	logger.Info("应用快照请求",
		zap.String("snapshot_id", input.SnapshotID),
		zap.Bool("create_backup", input.CreateBackup),
		zap.Bool("force", input.Force),
		zap.Bool("dry_run", input.DryRun),
	)

	// 获取快照
	snapData, err := a.snapshotSvc.GetSnapshot(input.SnapshotID)
	if err != nil {
		return nil, err
	}

	// 构建应用选项
	options := models.ApplyOptions{
		CreateBackup: input.CreateBackup,
		Force:        input.Force,
		DryRun:       input.DryRun,
	}

	// 获取应用器
	collector := snapshotpkg.NewCollector(a.toolDetector)
	applier := snapshotpkg.NewApplier(collector)

	// 应用快照
	result, err := applier.ApplySnapshot(snapData, options)
	if err != nil {
		logger.Error("应用快照失败", zap.Error(err))
		return nil, err
	}

	// 转换结果
	info := &ApplySnapshotInfo{
		Success:      result.Success,
		AppliedCount: len(result.AppliedFiles),
		SkippedCount: len(result.SkippedFiles),
		Summary:      result.Summary,
	}

	for _, e := range result.Errors {
		info.Errors = append(info.Errors, e.Message)
	}

	for _, f := range result.AppliedFiles {
		info.CreatedFiles = append(info.CreatedFiles, f.Path)
	}

	logger.Info("应用快照完成",
		zap.Bool("success", info.Success),
		zap.Int("applied", info.AppliedCount),
		zap.Int("skipped", info.SkippedCount),
	)

	return info, nil
}

// ==================== 同步接口 ====================

// PushSnapshotInput 推送快照输入参数
type PushSnapshotInput struct {
	SnapshotID string `json:"snapshot_id"` // 快照 ID
	RemoteURL  string `json:"remote_url"`  // 远端仓库 URL
	Branch     string `json:"branch"`      // 分支名
	ForcePush  bool   `json:"force_push"`  // 是否强制推送
}

// PushResultInfo 推送结果信息
type PushResultInfo struct {
	Success    bool   `json:"success"`
	SnapshotID string `json:"snapshot_id"`
	CommitHash string `json:"commit_hash"`
	Branch     string `json:"branch"`
	PushedAt   string `json:"pushed_at"`
	Message    string `json:"message"`
}

// PushSnapshot 推送快照到远端仓库
func (a *App) PushSnapshot(input PushSnapshotInput) (*PushResultInfo, error) {
	if a.syncSvc == nil {
		return nil, fmt.Errorf("同步服务未初始化")
	}

	logger.Info("推送快照请求",
		zap.String("snapshot_id", input.SnapshotID),
		zap.String("remote_url", input.RemoteURL),
		zap.String("branch", input.Branch),
	)

	// 获取快照
	snap, err := a.snapshotSvc.GetSnapshot(input.SnapshotID)
	if err != nil {
		return nil, fmt.Errorf("获取快照失败: %w", err)
	}

	// 构建同步选项
	ctx := context.Background()
	syncOptions := sync.SyncOptions{
		RemoteURL:  input.RemoteURL,
		RemoteName: "origin",
		Branch:     input.Branch,
		ForcePush:  input.ForcePush,
	}

	// 推送快照
	result, err := a.syncSvc.PushSnapshot(ctx, snap, syncOptions)
	if err != nil {
		logger.Error("推送快照失败", zap.Error(err))
		return nil, err
	}

	info := &PushResultInfo{
		Success:    result.Success,
		SnapshotID: result.SnapshotID,
		CommitHash: result.CommitHash,
		Branch:     result.Branch,
		PushedAt:   result.PushedAt,
		Message:    "快照推送成功",
	}

	logger.Info("推送快照成功",
		zap.String("commit_hash", info.CommitHash),
	)

	return info, nil
}

// PullSnapshotsInput 拉取快照输入参数
type PullSnapshotsInput struct {
	RemoteURL string `json:"remote_url"` // 远端仓库 URL
	Branch    string `json:"branch"`     // 分支名
	RepoPath  string `json:"repo_path"`  // 本地仓库路径
}

// PullResultInfo 拉取结果信息
type PullResultInfo struct {
	Success     bool   `json:"success"`
	Branch      string `json:"branch"`
	NewSnapshots int   `json:"new_snapshots"`
	Message     string `json:"message"`
}

// PullSnapshots 从远端仓库拉取快照
func (a *App) PullSnapshots(input PullSnapshotsInput) (*PullResultInfo, error) {
	if a.syncSvc == nil {
		return nil, fmt.Errorf("同步服务未初始化")
	}

	logger.Info("拉取快照请求",
		zap.String("remote_url", input.RemoteURL),
		zap.String("branch", input.Branch),
		zap.String("repo_path", input.RepoPath),
	)

	ctx := context.Background()
	syncOptions := sync.SyncOptions{
		RemoteURL:  input.RemoteURL,
		RemoteName: "origin",
		Branch:     input.Branch,
	}

	result, err := a.syncSvc.PullSnapshots(ctx, input.RepoPath, syncOptions)
	if err != nil {
		logger.Error("拉取快照失败", zap.Error(err))
		return nil, err
	}

	info := &PullResultInfo{
		Success:     result.Success,
		Branch:      result.Branch,
		NewSnapshots: result.NewSnapshots,
		Message:     fmt.Sprintf("成功拉取，新增 %d 个快照", result.NewSnapshots),
	}

	logger.Info("拉取快照成功",
		zap.Int("new_snapshots", info.NewSnapshots),
	)

	return info, nil
}

// ==================== 辅助函数 ====================

// calculateTotalSize 计算文件总大小
func calculateTotalSize(files []models.SnapshotFile) int64 {
	var total int64
	for _, f := range files {
		total += f.Size
	}
	return total
}

// ==================== 加密接口 ====================

// EncryptionConfigInput 加密配置输入
type EncryptionConfigInput struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"`
	KeyPath   string `json:"key_path"`
	KeyEnvVar string `json:"key_env_var"`
}

// EncryptionStatus 加密状态信息
type EncryptionStatus struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"`
	HasKey    bool   `json:"has_key"`
	KeySource string `json:"key_source"` // "file", "env", "generated"
}

// GetEncryptionStatus 获取加密状态
func (a *App) GetEncryptionStatus() (*EncryptionStatus, error) {
	if a.cryptoSvc == nil {
		return &EncryptionStatus{
			Enabled:   false,
			Algorithm: "none",
			HasKey:    false,
			KeySource: "not_initialized",
		}, nil
	}

	return &EncryptionStatus{
		Enabled:   a.cryptoSvc.IsEnabled(),
		Algorithm: a.cryptoSvc.GetAlgorithm(),
		HasKey:    a.cryptoSvc.IsEnabled(), // 简化判断
		KeySource: "configured",
	}, nil
}

// ConfigureEncryption 配置加密
func (a *App) ConfigureEncryption(input EncryptionConfigInput) error {
	logger.Info("配置加密",
		zap.Bool("enabled", input.Enabled),
		zap.String("algorithm", input.Algorithm),
	)

	config := &models.EncryptionConfig{
		Enabled:   input.Enabled,
		Algorithm: input.Algorithm,
		KeyPath:   input.KeyPath,
		KeyEnvVar: input.KeyEnvVar,
	}

	cryptoSvc, err := crypto.NewService(config)
	if err != nil {
		logger.Error("加密服务初始化失败", zap.Error(err))
		return fmt.Errorf("加密服务初始化失败: %w", err)
	}

	a.cryptoSvc = cryptoSvc
	logger.Info("加密配置完成")

	return nil
}

// EncryptData 加密数据
func (a *App) EncryptData(plaintext string) (string, error) {
	if a.cryptoSvc == nil {
		return "", fmt.Errorf("加密服务未初始化")
	}

	result, err := a.cryptoSvc.Encrypt(plaintext)
	if err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("加密失败: %s", result.Error)
	}

	return result.Data, nil
}

// DecryptData 解密数据
func (a *App) DecryptData(ciphertext string) (string, error) {
	if a.cryptoSvc == nil {
		return "", fmt.Errorf("加密服务未初始化")
	}

	result, err := a.cryptoSvc.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("解密失败: %s", result.Error)
	}

	return result.Data, nil
}

// EncryptPassword 加密密码
func (a *App) EncryptPassword(password string) (string, error) {
	if a.cryptoSvc == nil {
		return "", fmt.Errorf("加密服务未初始化")
	}

	return a.cryptoSvc.EncryptPassword(password)
}

// DecryptPassword 解密密码
func (a *App) DecryptPassword(encrypted string) (string, error) {
	if a.cryptoSvc == nil {
		return "", fmt.Errorf("加密服务未初始化")
	}

	return a.cryptoSvc.DecryptPassword(encrypted)
}

// EncryptToken 加密 Token
func (a *App) EncryptToken(token string) (string, error) {
	if a.cryptoSvc == nil {
		return "", fmt.Errorf("加密服务未初始化")
	}

	return a.cryptoSvc.EncryptToken(token)
}

// DecryptToken 解密 Token
func (a *App) DecryptToken(encrypted string) (string, error) {
	if a.cryptoSvc == nil {
		return "", fmt.Errorf("加密服务未初始化")
	}

	return a.cryptoSvc.DecryptToken(encrypted)
}

// GenerateEncryptionKey 生成新的加密密钥
func (a *App) GenerateEncryptionKey() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return "", fmt.Errorf("生成密钥失败: %w", err)
	}

	logger.Info("生成新的加密密钥")

	return key, nil
}

// ==================== Git 操作接口 ====================

// GitClone 克隆 Git 仓库
func (a *App) GitClone(url string, path string, branch string) (*git.RepositoryInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Info("Git 克隆请求",
		zap.String("url", url),
		zap.String("path", path),
		zap.String("branch", branch),
	)

	client := git.NewGitClient()
	opts := &git.CloneOptions{
		URL:    url,
		Path:   path,
		Branch: branch,
	}

	info, err := client.Clone(ctx, opts)
	if err != nil {
		logger.Error("Git 克隆失败", zap.Error(err))
		return nil, err
	}

	return info, nil
}

// GitPull 拉取 Git 仓库更新
func (a *App) GitPull(path string) (*git.OperationResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	logger.Info("Git 拉取请求", zap.String("path", path))

	client := git.NewGitClient()
	opts := &git.PullOptions{
		Path: path,
	}

	result, err := client.Pull(ctx, opts)
	if err != nil {
		logger.Error("Git 拉取失败", zap.Error(err))
		return nil, err
	}

	return result, nil
}

// GitPush 推送到 Git 远端仓库
func (a *App) GitPush(path string, branch string, force bool) (*git.OperationResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	logger.Info("Git 推送请求",
		zap.String("path", path),
		zap.String("branch", branch),
		zap.Bool("force", force),
	)

	client := git.NewGitClient()
	opts := &git.PushOptions{
		Path:   path,
		Branch: branch,
		Force:  force,
	}

	result, err := client.Push(ctx, opts)
	if err != nil {
		logger.Error("Git 推送失败", zap.Error(err))
		return nil, err
	}

	return result, nil
}

// GitGetStatus 获取 Git 仓库状态
func (a *App) GitGetStatus(path string) (*git.RepositoryStatus, error) {
	logger.Info("Git 状态查询请求", zap.String("path", path))

	client := git.NewGitClient()
	opts := &git.StatusOptions{
		Path: path,
	}

	status, err := client.GetStatus(opts)
	if err != nil {
		logger.Error("获取 Git 状态失败", zap.Error(err))
		return nil, err
	}

	return status, nil
}

// GitCommit 提交更改到 Git 仓库
func (a *App) GitCommit(path string, message string) (*git.CommitResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	logger.Info("Git 提交请求",
		zap.String("path", path),
		zap.String("message", message),
	)

	client := git.NewGitClient()
	opts := &git.CommitOptions{
		Path:    path,
		Message: message,
		All:     true,
	}

	result, err := client.Commit(ctx, opts)
	if err != nil {
		logger.Error("Git 提交失败", zap.Error(err))
		return nil, err
	}

	return result, nil
}

// GitGetRepositoryInfo 获取 Git 仓库信息
func (a *App) GitGetRepositoryInfo(path string) (*git.RepositoryInfo, error) {
	logger.Info("Git 仓库信息查询请求", zap.String("path", path))

	client := git.NewGitClient()
	info, err := client.GetRepositoryInfo(path)
	if err != nil {
		logger.Error("获取 Git 仓库信息失败", zap.Error(err))
		return nil, err
	}

	return info, nil
}

// GitListBranches 列出 Git 仓库的所有分支
func (a *App) GitListBranches(path string) ([]git.BranchInfo, error) {
	logger.Info("Git 分支列表查询请求", zap.String("path", path))

	client := git.NewGitClient()
	branches, err := client.ListBranches(path)
	if err != nil {
		logger.Error("获取 Git 分支列表失败", zap.Error(err))
		return nil, err
	}

	return branches, nil
}

// GitIsRepository 检查路径是否为 Git 仓库
func (a *App) GitIsRepository(path string) bool {
	return git.IsRepository(path)
}

// GitValidateAuth 验证 Git 认证配置
func (a *App) GitValidateAuth(authType string, username string, password string) error {
	auth := &git.GitAuthConfig{
		Type:     git.AuthType(authType),
		Username: username,
		Password: password,
	}
	return git.ValidateAuthConfig(auth)
}

// CreateSnapshot 创建配置快照
// func (a *App) CreateSnapshot(options CreateSnapshotOptions) (*SnapshotPackage, error) {
// 	return nil, errors.New("not implemented yet")
// }

// ListRemoteSnapshots 列出远端快照
// func (a *App) ListRemoteSnapshots() ([]SnapshotInfo, error) {
// 	return nil, errors.New("not implemented yet")
// }

// PullAndApply 拉取并应用快照
// func (a *App) PullAndApply(snapshotID string, options ApplyOptions) (*ApplyResult, error) {
// 	return nil, errors.New("not implemented yet")
// }

// CompareWithRemote 比较本地与远端差异
// func (a *App) CompareWithRemote(snapshotID string) (*ChangeSummary, error) {
// 	return nil, errors.New("not implemented yet")
// }

// ConfigureGitRemote 配置 Git 远端仓库
// func (a *App) ConfigureGitRemote(config GitRemoteConfigInput) error {
// 	return errors.New("not implemented yet")
// }

// SetSyncScope 设置同步范围
// func (a *App) SetSyncScope(scope SyncScopeConfig) error {
// 	return errors.New("not implemented yet")
// }

// isDirExists 检查目录是否存在
func isDirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
