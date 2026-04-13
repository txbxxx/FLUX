package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"flux/internal/models"
	"flux/internal/service/snapshot"
	"flux/internal/service/tool"
	"flux/pkg/config"
	"flux/pkg/database"
	"flux/pkg/logger"
)

type Options struct {
	Version           string
	DataDir           string
	DisableConsoleLog bool
}

// Runtime 聚合命令行入口需要复用的基础依赖。
type Runtime struct {
	Version          string
	DataDir          string
	Config           *config.Config
	DB               *database.DB
	Detector         *tool.ToolDetector
	RuleResolver     *tool.RuleResolver
	RuleManager      *tool.RuleManager
	SnapshotService  *snapshot.Service
	AISettingDAO    *models.AISettingDAO
}

// New 按"配置 -> 日志 -> 数据库 -> 规则 -> 业务服务"的顺序构建运行时。
func New(options Options) (*Runtime, error) {
	// 1. 加载配置（嵌入默认值 + 用户覆盖）
	cfg, err := config.Load()
	if err != nil {
		cfg = config.DefaultConfig()
		fmt.Fprintf(os.Stderr, "加载配置失败，使用默认值: %v\n", err)
	}

	// 2. 确定数据目录
	dataDir := options.DataDir
	if dataDir == "" {
		dataDir = cfg.App.DataDir
		if dataDir == "" {
			dataDir = DefaultDataDir()
		}
	}
	// 展开 ~ 为用户主目录
	dataDir = expandHome(dataDir)
	cfg.App.DataDir = dataDir

	// 3. 命令行参数覆盖
	version := options.Version
	if version == "" {
		version = cfg.App.Version
	}

	// 4. 初始化日志
	logConfig := loggerConfigFrom(cfg)
	if options.DisableConsoleLog {
		logConfig.ConsoleOut = false
	}

	if err := logger.Init(logConfig); err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	// 5. 初始化数据库
	db, err := database.InitDBWithConfig(dataDir, &cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 6. 注入工具配置
	tool.SetToolsConfig(cfg.Tools)

	ruleManager := tool.NewRuleManager(db)

	// 7. 自动注册全局项目（codex-global, claude-global）
	if err := ruleManager.EnsureGlobalProjectsRegistered(); err != nil {
		logger.Warn("自动注册全局项目失败，将使用硬编码默认值")
		// 不阻塞启动，仅记录警告
	}

	resolver := tool.NewRuleResolver(ruleManager.Store())
	detector := tool.NewToolDetectorWithResolver(resolver)

	return &Runtime{
		Version:          version,
		DataDir:          dataDir,
		Config:           cfg,
		DB:               db,
		Detector:         detector,
		RuleResolver:     resolver,
		RuleManager:      ruleManager,
		SnapshotService:  snapshot.NewService(db, resolver, ruleManager),
		AISettingDAO:    models.NewAISettingDAO(db),
	}, nil
}

// loggerConfigFrom 将全局配置转换为 logger 包的 Config。
func loggerConfigFrom(cfg *config.Config) *logger.Config {
	filePath := expandHome(cfg.Logger.FilePath)
	return &logger.Config{
		Level:      cfg.Logger.Level,
		FilePath:   filePath,
		MaxSize:    cfg.Logger.MaxSize,
		MaxBackups: cfg.Logger.MaxBackups,
		MaxAge:     cfg.Logger.MaxAge,
		Compress:   cfg.Logger.Compress,
		ConsoleOut: cfg.Logger.ConsoleOut,
	}
}

// expandHome 将路径中的 ~ 展开为用户主目录。
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[1:])
	}
	return path
}

// DefaultDataDir 返回默认数据目录；用户主目录不可用时退回相对目录。
func DefaultDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".flux"
	}
	return filepath.Join(homeDir, ".flux")
}

// Close 负责关闭数据库并刷新日志缓冲。
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	if r.DB != nil {
		_ = r.DB.Close()
	}
	return logger.Sync()
}
