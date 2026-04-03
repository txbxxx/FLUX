package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 应用全局配置
type Config struct {
	App      AppConfig             `yaml:"app"`
	Logger   LoggerConfig          `yaml:"logger"`
	Database DatabaseConfig        `yaml:"database"`
	Sync     SyncConfig            `yaml:"sync"`
	Tools    map[string]ToolConfig `yaml:"tools"`
}

// AppConfig 应用基础配置
type AppConfig struct {
	Version string `yaml:"version"`
	DataDir string `yaml:"data_dir"`
}

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level      string `yaml:"level"`
	FilePath   string `yaml:"file_path"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
	ConsoleOut bool   `yaml:"console_out"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Filename        string                 `yaml:"filename"`
	MaxOpenConns    int                    `yaml:"max_open_conns"`
	MaxIdleConns    int                    `yaml:"max_idle_conns"`
	ConnMaxLifetime string                 `yaml:"conn_max_lifetime"`
	Pragmas         map[string]interface{} `yaml:"pragmas"`
}

// 以下方法使 DatabaseConfig 满足 database.DatabaseConfig 接口

// GetFilename 返回数据库文件名。
func (d *DatabaseConfig) GetFilename() string { return d.Filename }

// GetMaxOpenConns 返回最大打开连接数。
func (d *DatabaseConfig) GetMaxOpenConns() int { return d.MaxOpenConns }

// GetMaxIdleConns 返回最大空闲连接数。
func (d *DatabaseConfig) GetMaxIdleConns() int { return d.MaxIdleConns }

// GetConnMaxLifetime 解析并返回连接最大生命周期。
func (d *DatabaseConfig) GetConnMaxLifetime() time.Duration {
	if d.ConnMaxLifetime == "" {
		return time.Hour
	}
	dur, err := time.ParseDuration(d.ConnMaxLifetime)
	if err != nil {
		return time.Hour
	}
	return dur
}

// GetPragmas 返回 PRAGMA 配置。
func (d *DatabaseConfig) GetPragmas() map[string]interface{} { return d.Pragmas }

// SyncConfig 同步/Git 默认配置
type SyncConfig struct {
	DefaultBranch string `yaml:"default_branch"`
	DefaultRemote string `yaml:"default_remote"`
}

// ToolConfig 单个工具的配置
type ToolConfig struct {
	GlobalDir           string    `yaml:"global_dir"`
	ProjectDir          string    `yaml:"project_dir"`
	RequiredGlobalPaths []string  `yaml:"required_global_paths"`
	GlobalRules         []RuleDef `yaml:"global_rules"`
	ProjectRules        []RuleDef `yaml:"project_rules"`
}

// RuleDef 单条规则定义
type RuleDef struct {
	Path     string `yaml:"path"`
	Category string `yaml:"category"`
	IsDir    bool   `yaml:"is_dir"`
}

// DefaultConfig 从嵌入的 YAML 返回默认配置，保证单一数据源。
func DefaultConfig() *Config {
	cfg, err := LoadFrom(defaultYAML)
	if err != nil {
		// 嵌入 YAML 不应出错，如果出错则返回代码内硬编码的最小配置
		homeDir, _ := os.UserHomeDir()
		dataDir := filepath.Join(homeDir, ".ai-sync-manager")
		return &Config{
			App:      AppConfig{Version: "1.0.0-alpha", DataDir: dataDir},
			Database: DatabaseConfig{Filename: "ai-sync-manager.db", MaxOpenConns: 1, MaxIdleConns: 1, ConnMaxLifetime: "1h"},
			Sync:     SyncConfig{DefaultBranch: "main", DefaultRemote: "origin"},
		}
	}
	return cfg
}

// LoadFrom 从 YAML 字节解析配置。
func LoadFrom(yamlBytes []byte) (*Config, error) {
	cfg := &Config{}
	if err := yaml.Unmarshal(yamlBytes, cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	return cfg, nil
}

// userConfigPath 返回用户配置文件路径。
func userConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".ai-sync-manager", "config.yaml")
}

// Load 加载配置：先加载嵌入的默认值，再用用户配置覆盖。
func Load() (*Config, error) {
	// 1. 从嵌入的默认 YAML 解析
	defaults, err := LoadFrom(defaultYAML)
	if err != nil {
		return nil, fmt.Errorf("加载默认配置失败: %w", err)
	}

	// 2. 检查用户配置文件
	userPath := userConfigPath()
	if userPath == "" {
		return defaults, nil
	}

	data, err := os.ReadFile(userPath)
	if err != nil {
		// 文件不存在或不可读，使用默认值
		return defaults, nil
	}

	// 3. 解析用户配置并合并
	userCfg := &Config{}
	if err := yaml.Unmarshal(data, userCfg); err != nil {
		return nil, fmt.Errorf("解析用户配置 %s 失败: %w", userPath, err)
	}

	merge(defaults, userCfg)
	return defaults, nil
}

// merge 将 userCfg 中的非零值覆盖到 defaults 上。
func merge(defaults, userCfg *Config) {
	// App
	if userCfg.App.Version != "" {
		defaults.App.Version = userCfg.App.Version
	}
	if userCfg.App.DataDir != "" {
		defaults.App.DataDir = userCfg.App.DataDir
	}

	// Logger
	if userCfg.Logger.Level != "" {
		defaults.Logger.Level = userCfg.Logger.Level
	}
	if userCfg.Logger.FilePath != "" {
		defaults.Logger.FilePath = userCfg.Logger.FilePath
	}
	if userCfg.Logger.MaxSize != 0 {
		defaults.Logger.MaxSize = userCfg.Logger.MaxSize
	}
	if userCfg.Logger.MaxBackups != 0 {
		defaults.Logger.MaxBackups = userCfg.Logger.MaxBackups
	}
	if userCfg.Logger.MaxAge != 0 {
		defaults.Logger.MaxAge = userCfg.Logger.MaxAge
	}
	// bool: 如果用户配置了 logger 段中任一数值字段，认为整个段是用户意图
	if userCfg.Logger.MaxSize != 0 {
		defaults.Logger.Compress = userCfg.Logger.Compress
		defaults.Logger.ConsoleOut = userCfg.Logger.ConsoleOut
	}

	// Database
	if userCfg.Database.Filename != "" {
		defaults.Database.Filename = userCfg.Database.Filename
	}
	if userCfg.Database.MaxOpenConns != 0 {
		defaults.Database.MaxOpenConns = userCfg.Database.MaxOpenConns
	}
	if userCfg.Database.MaxIdleConns != 0 {
		defaults.Database.MaxIdleConns = userCfg.Database.MaxIdleConns
	}
	if userCfg.Database.ConnMaxLifetime != "" {
		defaults.Database.ConnMaxLifetime = userCfg.Database.ConnMaxLifetime
	}
	if len(userCfg.Database.Pragmas) > 0 {
		defaults.Database.Pragmas = userCfg.Database.Pragmas
	}

	// Sync
	if userCfg.Sync.DefaultBranch != "" {
		defaults.Sync.DefaultBranch = userCfg.Sync.DefaultBranch
	}
	if userCfg.Sync.DefaultRemote != "" {
		defaults.Sync.DefaultRemote = userCfg.Sync.DefaultRemote
	}

	// Tools: 按工具名合并
	for name, userTool := range userCfg.Tools {
		if defaultTool, ok := defaults.Tools[name]; ok {
			mergeTool(&defaultTool, &userTool)
			defaults.Tools[name] = defaultTool
		} else {
			// 新增工具
			defaults.Tools[name] = userTool
		}
	}
}

// mergeTool 合并单个工具配置。
func mergeTool(defaults, user *ToolConfig) {
	if user.GlobalDir != "" {
		defaults.GlobalDir = user.GlobalDir
	}
	if user.ProjectDir != "" {
		defaults.ProjectDir = user.ProjectDir
	}
	if len(user.RequiredGlobalPaths) > 0 {
		defaults.RequiredGlobalPaths = user.RequiredGlobalPaths
	}
	if len(user.GlobalRules) > 0 {
		defaults.GlobalRules = user.GlobalRules
	}
	if len(user.ProjectRules) > 0 {
		defaults.ProjectRules = user.ProjectRules
	}
}
