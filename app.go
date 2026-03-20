package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"ai-sync-manager/internal/service/git"
	"ai-sync-manager/internal/service/tool"
	"ai-sync-manager/pkg/logger"
	"go.uber.org/zap"
)

// App struct
type App struct {
	ctx     context.Context
	version string
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
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	logger.Info("AI Sync Manager shutting down")
	_ = logger.Sync()
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

// ==================== 预留业务接口 ====================
// 以下接口在后续阶段实现

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
