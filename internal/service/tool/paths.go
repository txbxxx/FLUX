package tool

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"ai-sync-manager/pkg/utils"
)

// GetDefaultGlobalPath 获取工具的默认全局配置路径
func GetDefaultGlobalPath(toolType ToolType) string {
	homeDir := GetUserHomeDir()

	switch toolType {
	case ToolTypeCodex:
		return filepath.Join(homeDir, ".codex")
	case ToolTypeClaude:
		return filepath.Join(homeDir, ".claude")
	default:
		return ""
	}
}

// GetDefaultProjectPath 获取工具的默认项目配置路径
func GetDefaultProjectPath(toolType ToolType) string {
	switch toolType {
	case ToolTypeCodex:
		return ".codex"
	case ToolTypeClaude:
		return ".claude"
	default:
		return ""
	}
}

// CodexFileDefinitions Codex 配置文件定义
type CodexFileDefinition struct {
	Name     string        // 文件/目录名
	Path     string        // 相对于配置根目录的路径
	Category ConfigCategory // 配置类别
	Scope    ConfigScope   // 作用域
	IsDir    bool          // 是否为目录
}

// GetCodexFileDefinitions 获取 Codex 配置文件定义列表
func GetCodexFileDefinitions() []CodexFileDefinition {
	return []CodexFileDefinition{
		// 全局配置
		{
			Name:     "config.toml",
			Path:     "config.toml",
			Category: CategoryConfigFile,
			Scope:    ScopeGlobal,
			IsDir:    false,
		},
		{
			Name:     "skills",
			Path:     "skills",
			Category: CategorySkills,
			Scope:    ScopeGlobal,
			IsDir:    true,
		},
		{
			Name:     "rules",
			Path:     "rules",
			Category: CategoryRules,
			Scope:    ScopeGlobal,
			IsDir:    true,
		},
		{
			Name:     "superpowers",
			Path:     "superpowers",
			Category: CategoryConfigFile,
			Scope:    ScopeGlobal,
			IsDir:    true,
		},
		// 项目配置
		{
			Name:     "config.toml",
			Path:     "config.toml",
			Category: CategoryConfigFile,
			Scope:    ScopeProject,
			IsDir:    false,
		},
		{
			Name:     "AGENTS.md",
			Path:     "AGENTS.md",
			Category: CategoryAgents,
			Scope:    ScopeProject,
			IsDir:    false,
		},
	}
}

// ClaudeFileDefinition Claude 配置文件定义
type ClaudeFileDefinition struct {
	Name     string        // 文件/目录名
	Path     string        // 相对于配置根目录的路径
	Category ConfigCategory // 配置类别
	Scope    ConfigScope   // 作用域
	IsDir    bool          // 是否为目录
}

// GetClaudeFileDefinitions 获取 Claude 配置文件定义列表
func GetClaudeFileDefinitions() []ClaudeFileDefinition {
	return []ClaudeFileDefinition{
		// 全局配置
		{
			Name:     "skills",
			Path:     "skills",
			Category: CategorySkills,
			Scope:    ScopeGlobal,
			IsDir:    true,
		},
		{
			Name:     "commands",
			Path:     "commands",
			Category: CategoryCommands,
			Scope:    ScopeGlobal,
			IsDir:    true,
		},
		{
			Name:     "plugins",
			Path:     "plugins",
			Category: CategoryPlugins,
			Scope:    ScopeGlobal,
			IsDir:    true,
		},
		{
			Name:     "output-styles",
			Path:     "output-styles",
			Category: CategoryConfigFile,
			Scope:    ScopeGlobal,
			IsDir:    true,
		},
		{
			Name:     "CLAUDE.md",
			Path:     "CLAUDE.md",
			Category: CategoryConfigFile,
			Scope:    ScopeGlobal,
			IsDir:    false,
		},
		{
			Name:     "settings.json",
			Path:     "settings.json",
			Category: CategoryConfigFile,
			Scope:    ScopeGlobal,
			IsDir:    false,
		},
		// 项目配置
		{
			Name:     ".claude",
			Path:     ".",
			Category: CategoryConfigFile,
			Scope:    ScopeProject,
			IsDir:    true,
		},
		{
			Name:     "skills",
			Path:     "skills",
			Category: CategorySkills,
			Scope:    ScopeProject,
			IsDir:    true,
		},
		{
			Name:     "CLAUDE.md",
			Path:     "CLAUDE.md",
			Category: CategoryConfigFile,
			Scope:    ScopeProject,
			IsDir:    false,
		},
	}
}

// ExpandPath 展开路径中的环境变量和用户目录
func ExpandPath(path string) string {
	// 处理 ~
	if len(path) > 0 && path[0] == '~' {
		homeDir := GetUserHomeDir()
		if homeDir != "" && homeDir != "~" {
			if len(path) == 1 {
				return homeDir
			}
			return filepath.Join(homeDir, path[2:])
		}
	}

	// 处理 %USERPROFILE%
	if utils.IsWindows() {
		homeDir := GetUserHomeDir()
		if homeDir != "" && homeDir != "~" {
			path = strings.ReplaceAll(path, "%USERPROFILE%", homeDir)
		}
	}

	return filepath.Clean(path)
}

// ResolveToolPath 解析工具配置路径
func ResolveToolPath(toolType ToolType, scope ConfigScope, basePath string) string {
	if scope == ScopeGlobal {
		return GetDefaultGlobalPath(toolType)
	}

	// 项目配置：basePath 是项目根目录
	return filepath.Join(basePath, GetDefaultProjectPath(toolType))
}

// IsWindows 判断是否为 Windows 系统
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// GetUserHomeDir 获取用户主目录
func GetUserHomeDir() string {
	if homeDir, err := os.UserHomeDir(); err == nil && isResolvedHomeDir(homeDir) {
		return homeDir
	}

	if currentUser, err := user.Current(); err == nil && isResolvedHomeDir(currentUser.HomeDir) {
		return currentUser.HomeDir
	}

	if homeDir := os.Getenv("HOME"); isResolvedHomeDir(homeDir) {
		return homeDir
	}

	if homeDir := os.Getenv("USERPROFILE"); isResolvedHomeDir(homeDir) {
		return homeDir
	}

	homeDrive := os.Getenv("HOMEDRIVE")
	homePath := os.Getenv("HOMEPATH")
	if homeDrive != "" && homePath != "" {
		combined := homeDrive + homePath
		if isResolvedHomeDir(combined) {
			return combined
		}
	}

	return "~"
}

// GetHomeConfigPath 获取用户主目录下的配置路径
func GetHomeConfigPath(toolType ToolType, relativePath string) string {
	homeDir := GetUserHomeDir()
	var toolDir string

	switch toolType {
	case ToolTypeCodex:
		toolDir = ".codex"
	case ToolTypeClaude:
		toolDir = ".claude"
	default:
		toolDir = string(toolType)
	}

	return filepath.Join(homeDir, toolDir, relativePath)
}

func isResolvedHomeDir(path string) bool {
	if path == "" || path == "~" {
		return false
	}

	if strings.HasPrefix(path, "~\\") || strings.HasPrefix(path, "~/") {
		return false
	}

	return true
}
