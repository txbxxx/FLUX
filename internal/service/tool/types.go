package tool

import (
	"time"
)

// ToolType 工具类型
type ToolType string

const (
	ToolTypeCodex  ToolType = "codex"
	ToolTypeClaude ToolType = "claude"
)

// String 返回工具类型的字符串表示
func (t ToolType) String() string {
	return string(t)
}

// InstallationStatus 安装状态
type InstallationStatus string

const (
	StatusInstalled     InstallationStatus = "installed"     // 已安装
	StatusNotInstalled InstallationStatus = "not_installed" // 未安装
	StatusPartial       InstallationStatus = "partial"       // 部分安装
)

// ConfigScope 配置作用域
type ConfigScope string

const (
	ScopeGlobal  ConfigScope = "global"  // 全局配置（用户主目录）
	ScopeProject ConfigScope = "project" // 项目配置（项目目录）
)

// ConfigCategory 配置类别
type ConfigCategory string

const (
	CategorySkills     ConfigCategory = "skills"      // 技能目录
	CategoryCommands   ConfigCategory = "commands"    // 命令目录
	CategoryPlugins    ConfigCategory = "plugins"     // 插件目录
	CategoryMCP        ConfigCategory = "mcp"         // MCP 配置
	CategoryConfigFile ConfigCategory = "config"      // 配置文件
	CategoryAgents     ConfigCategory = "agents"      // 代理文件
	CategoryRules      ConfigCategory = "rules"       // 规则目录
	CategoryDocs       ConfigCategory = "docs"        // 文档
)

// ConfigFile 配置文件信息
type ConfigFile struct {
	Name       string        `json:"name"`        // 文件/目录名
	Path       string        `json:"path"`        // 完整路径
	Category   ConfigCategory `json:"category"`    // 配置类别
	Scope      ConfigScope    `json:"scope"`       // 作用域（global/project）
	Size       int64         `json:"size"`        // 大小（字节）
	ModifiedAt time.Time     `json:"modified_at"` // 最后修改时间
	IsDir      bool          `json:"is_dir"`      // 是否为目录
}

// ProjectInfo 项目信息
type ProjectInfo struct {
	Path      string `json:"path"`      // 项目路径
	Name      string `json:"name"`      // 项目名称
	HasCodex  bool   `json:"has_codex"` // 是否包含 Codex 配置
	HasClaude bool   `json:"has_claude"`// 是否包含 Claude 配置
}

// ToolInstallation 工具安装信息
type ToolInstallation struct {
	ToolType     ToolType           `json:"tool_type"`      // 工具类型
	Status       InstallationStatus `json:"status"`        // 安装状态
	Version      string             `json:"version,omitempty"` // 版本号（可选）
	GlobalPath   string             `json:"global_path,omitempty"` // 全局配置路径
	ProjectPaths []string           `json:"project_paths,omitempty"` // 检测到的项目路径
	ConfigFiles  []ConfigFile       `json:"config_files"`   // 配置文件列表
	Projects     []ProjectInfo      `json:"projects,omitempty"` // 关联项目信息
	DetectedAt   time.Time          `json:"detected_at"`   // 检测时间
}

// ToolDetectionResult 检测结果（内部使用）
type ToolDetectionResult struct {
	Codex   *ToolInstallation `json:"codex,omitempty"`
	Claude  *ToolInstallation `json:"claude,omitempty"`
	Projects []ProjectInfo    `json:"projects,omitempty"`
}

// ToInstallations 转换为 ToolInstallation 列表（过滤未安装的）
func (r *ToolDetectionResult) ToInstallations() []ToolInstallation {
	var result []ToolInstallation

	if r.Codex != nil && r.Codex.Status != StatusNotInstalled {
		result = append(result, *r.Codex)
	}

	if r.Claude != nil && r.Claude.Status != StatusNotInstalled {
		result = append(result, *r.Claude)
	}

	return result
}

// ScanOptions 扫描选项
type ScanOptions struct {
	ScanGlobal   bool     `json:"scan_global"`    // 是否扫描全局配置（默认true）
	ScanProjects  bool     `json:"scan_projects"`   // 是否扫描项目配置（默认false）
	ProjectPaths  []string `json:"project_paths"`   // 指定扫描的项目路径列表
	CurrentDir    string   `json:"current_dir"`     // 当前工作目录（可选）
	IncludeFiles  bool     `json:"include_files"`  // 是否包含文件列表（默认true）
	MaxDepth      int      `json:"max_depth"`      // 扫描深度（默认1，仅顶级）
}

// DefaultScanOptions 返回默认扫描选项
func DefaultScanOptions() *ScanOptions {
	return &ScanOptions{
		ScanGlobal:  true,
		ScanProjects: false,
		IncludeFiles: true,
		MaxDepth:     1,
	}
}
