package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"flux/internal/models"
	typesRemote "flux/internal/types/remote"

	"github.com/google/uuid"
)

// RemoteConfigManager abstracts remote configuration persistence.
type RemoteConfigManager interface {
	CreateRemoteConfig(config *models.RemoteConfig) error
	GetDefaultRemoteConfig() (*models.RemoteConfig, error)
	ListRemoteConfigs() ([]*models.RemoteConfig, error)
	UpdateRemoteConfig(config *models.RemoteConfig) error
	DeleteRemoteConfig(id string) error
	SetDefaultRemoteConfig(id string) error
}

// RemoteConfigDAOAdapter adapts models.RemoteConfigDAO to RemoteConfigManager interface.
// Why: The DAO method names (Create/GetDefault/List/Update/Delete/SetDefault) differ
// from the interface names (CreateRemoteConfig/...), so we need a thin adapter.
type RemoteConfigDAOAdapter struct {
	dao *models.RemoteConfigDAO
}

// NewRemoteConfigDAOAdapter creates a new adapter wrapping a RemoteConfigDAO.
func NewRemoteConfigDAOAdapter(dao *models.RemoteConfigDAO) *RemoteConfigDAOAdapter {
	return &RemoteConfigDAOAdapter{dao: dao}
}

func (a *RemoteConfigDAOAdapter) CreateRemoteConfig(config *models.RemoteConfig) error {
	return a.dao.Create(config)
}

func (a *RemoteConfigDAOAdapter) GetDefaultRemoteConfig() (*models.RemoteConfig, error) {
	return a.dao.GetDefault()
}

func (a *RemoteConfigDAOAdapter) ListRemoteConfigs() ([]*models.RemoteConfig, error) {
	return a.dao.List()
}

func (a *RemoteConfigDAOAdapter) UpdateRemoteConfig(config *models.RemoteConfig) error {
	return a.dao.Update(config)
}

func (a *RemoteConfigDAOAdapter) DeleteRemoteConfig(id string) error {
	return a.dao.Delete(id)
}

func (a *RemoteConfigDAOAdapter) SetDefaultRemoteConfig(id string) error {
	return a.dao.SetDefault(id)
}

// RemoteConnectionTester abstracts remote connectivity testing.
type RemoteConnectionTester interface {
	ValidateRemoteConfig(ctx context.Context, config *models.RemoteConfig) error
}

// AddRemote adds a new remote repository configuration.
//
// 流程：
//  1. 验证 URL 格式
//  2. 构建 RemoteConfig + AuthConfig
//  3. 连通性测试（浅克隆验证）
//  4. 保存到数据库
//  5. 如果指定了 project，初始化工作目录
func (w *LocalWorkflow) AddRemote(ctx context.Context, input typesRemote.AddRemoteInput) (*typesRemote.AddRemoteResult, error) {
	// 第一步：参数校验
	url := strings.TrimSpace(input.URL)
	if url == "" {
		return nil, &UserError{
			Message:    "添加远端仓库失败：请提供仓库地址",
			Suggestion: "例如: ai-sync remote add https://github.com/user/configs.git",
		}
	}

	// 新增：URL 格式校验
	if !isValidGitURL(url) {
		return nil, &UserError{
			Message: "添加远端仓库失败：仓库地址格式不正确",
			Suggestion: "支持的格式：\n  HTTPS: https://github.com/user/repo.git\n  SSH:   git@github.com:user/repo.git\n  本地:  /path/to/repo.git",
		}
	}

	if w.remoteConfigs == nil {
		return nil, &UserError{
			Message:    "添加远端仓库失败：远端配置服务不可用",
			Suggestion: "请重新启动程序",
		}
	}

	// 第二步：生成配置名称（如果未指定）
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = deriveRemoteName(url)
	}

	// 检查名称唯一性
	existing, _ := w.remoteConfigs.ListRemoteConfigs()
	for _, cfg := range existing {
		if cfg.Name == name {
			return nil, &UserError{
				Message:    "添加远端仓库失败：名称 \"" + name + "\" 已存在",
				Suggestion: "请使用 --name 指定不同的名称，或先删除已有配置",
			}
		}
	}

	// 第三步：构建 RemoteConfig
	branch := strings.TrimSpace(input.Branch)
	if branch == "" {
		branch = "main"
	}

	config := &models.RemoteConfig{
		ID:        uuid.New().String(),
		Name:      name,
		URL:       url,
		Branch:    branch,
		IsDefault: input.Project != "",
		Status:    models.StatusInactive,
		Auth: models.AuthConfig{
			Type:       models.AuthType(input.AuthType),
			Username:   strings.TrimSpace(input.Username),
			Password:   strings.TrimSpace(input.Token),
			SSHKey:     strings.TrimSpace(input.SSHKey),
			Passphrase: strings.TrimSpace(input.Password),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 第四步：连通性测试
	if w.remoteTester != nil {
		if err := w.remoteTester.ValidateRemoteConfig(ctx, config); err != nil {
			config.Status = models.StatusError
			_ = w.remoteConfigs.CreateRemoteConfig(config)
			return nil, &UserError{
				Message:    "添加远端仓库失败：无法连接到仓库",
				Suggestion: "请检查仓库地址和网络连接。错误：" + err.Error(),
				Err:        err,
			}
		}
	}
	config.Status = models.StatusActive

	// 第五步：保存到数据库
	if err := w.remoteConfigs.CreateRemoteConfig(config); err != nil {
		return nil, &UserError{
			Message:    "添加远端仓库失败：保存配置失败",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	return &typesRemote.AddRemoteResult{
		ConfigID:  config.ID,
		Name:      name,
		URL:       url,
		Branch:    branch,
		Connected: config.Status == models.StatusActive,
		Project:   strings.TrimSpace(input.Project),
	}, nil
}

// ListRemotes returns all configured remote repositories.
func (w *LocalWorkflow) ListRemotes(_ context.Context) (*typesRemote.ListRemotesResult, error) {
	if w.remoteConfigs == nil {
		return nil, &UserError{
			Message:    "查看远端仓库失败：远端配置服务不可用",
			Suggestion: "请重新启动程序",
		}
	}

	configs, err := w.remoteConfigs.ListRemoteConfigs()
	if err != nil {
		return nil, &UserError{
			Message:    "查看远端仓库失败",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	items := make([]typesRemote.RemoteItem, 0, len(configs))
	for _, cfg := range configs {
		statusText := "未验证"
		switch cfg.Status {
		case models.StatusActive:
			statusText = "活跃"
		case models.StatusInactive:
			statusText = "未激活"
		case models.StatusError:
			statusText = "错误"
		}

		items = append(items, typesRemote.RemoteItem{
			Name:       cfg.Name,
			URL:        cfg.URL,
			Branch:     cfg.Branch,
			IsDefault:  cfg.IsDefault,
			Status:     statusText,
			LastSynced: cfg.LastSynced,
		})
	}

	return &typesRemote.ListRemotesResult{Remotes: items}, nil
}

// RemoveRemote removes a remote repository configuration.
func (w *LocalWorkflow) RemoveRemote(_ context.Context, input typesRemote.RemoveRemoteInput) (*typesRemote.ListRemotesResult, error) {
	if w.remoteConfigs == nil {
		return nil, &UserError{
			Message:    "删除远端仓库失败：远端配置服务不可用",
			Suggestion: "请重新启动程序",
		}
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, &UserError{
			Message:    "删除远端仓库失败：请指定配置名称",
			Suggestion: "使用 remote list 查看已配置的远端仓库",
		}
	}

	// 查找配置
	configs, err := w.remoteConfigs.ListRemoteConfigs()
	if err != nil {
		return nil, &UserError{
			Message:    "删除远端仓库失败",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	var found *models.RemoteConfig
	for _, cfg := range configs {
		if cfg.Name == name {
			found = cfg
			break
		}
	}

	if found == nil {
		return nil, &UserError{
			Message:    "删除远端仓库失败：未找到名称为 \"" + name + "\" 的配置",
			Suggestion: "使用 remote list 查看已配置的远端仓库",
		}
	}

	if err := w.remoteConfigs.DeleteRemoteConfig(found.ID); err != nil {
		return nil, &UserError{
			Message:    "删除远端仓库失败",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	return nil, nil
}

// deriveRemoteName derives a configuration name from a Git URL.
// Extracts the repo name without extension, e.g.:
//   - https://github.com/user/my-configs.git → my-configs
//   - git@github.com:user/my-configs.git → my-configs
func deriveRemoteName(url string) string {
	// Remove trailing .git
	s := strings.TrimSuffix(url, ".git")

	// Take last path segment
	if idx := strings.LastIndex(s, "/"); idx >= 0 {
		s = s[idx+1:]
	}
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		s = s[idx+1:]
	}

	if s == "" {
		return fmt.Sprintf("remote-%d", time.Now().Unix())
	}
	return s
}

// WithRemoteConfigs injects the remote configuration manager dependency.
func (w *LocalWorkflow) WithRemoteConfigs(mgr RemoteConfigManager) *LocalWorkflow {
	w.remoteConfigs = mgr
	return w
}

// WithRemoteTester injects the remote connection tester dependency.
func (w *LocalWorkflow) WithRemoteTester(tester RemoteConnectionTester) *LocalWorkflow {
	w.remoteTester = tester
	return w
}

// WithDataDir injects the application data directory path.
func (w *LocalWorkflow) WithDataDir(dir string) *LocalWorkflow {
	w.dataDir = dir
	return w
}

// remoteConfigPath returns the repos directory path for a given project.
func remoteConfigPath(dataDir, projectName string) string {
	return filepath.Join(dataDir, "repos", projectName)
}

// isValidGitURL validates if the given URL is a valid Git repository URL.
// 支持的格式：
//   - HTTPS: https://github.com/user/repo.git
//   - SSH:   git@github.com:user/repo.git
//   - 本地绝对路径: /path/to/repo.git
func isValidGitURL(url string) bool {
	// 拒绝明显的无效格式
	if len(url) < 4 {
		return false
	}

	// 拒绝单个字符或特殊值
	if url == "-" || url == "." || url == ".." {
		return false
	}

	// HTTPS 格式
	if strings.HasPrefix(url, "https://") {
		// 检查是否有基本的域名结构（至少有一个点）
		rest := strings.TrimPrefix(url, "https://")
		if strings.Contains(rest, ".") && !strings.HasPrefix(rest, ".") {
			return true
		}
		return false
	}

	// HTTP 格式（不推荐但允许）
	if strings.HasPrefix(url, "http://") {
		rest := strings.TrimPrefix(url, "http://")
		if strings.Contains(rest, ".") && !strings.HasPrefix(rest, ".") {
			return true
		}
		return false
	}

	// SSH 格式: git@host:path
	if strings.Contains(url, "@") && strings.Contains(url, ":") {
		parts := strings.Split(url, "@")
		if len(parts) == 2 && strings.Contains(parts[1], ":") {
			return true
		}
	}

	// SSH 协议格式: ssh://host/path
	if strings.HasPrefix(url, "ssh://") {
		return true
	}

	// 本地绝对路径（Unix/Linux/Mac）
	if strings.HasPrefix(url, "/") {
		return true
	}

	// Windows 绝对路径
	if len(url) >= 2 && ((url[0] >= 'a' && url[0] <= 'z') || (url[0] >= 'A' && url[0] <= 'Z')) && url[1] == ':' {
		return true
	}

	return false
}
