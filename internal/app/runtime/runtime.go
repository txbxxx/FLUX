package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"ai-sync-manager/internal/service/snapshot"
	"ai-sync-manager/internal/service/tool"
	"ai-sync-manager/pkg/database"
	"ai-sync-manager/pkg/logger"
)

type Options struct {
	Version           string
	DataDir           string
	DisableConsoleLog bool
}

// Runtime 聚合命令行入口需要复用的基础依赖。
type Runtime struct {
	Version         string
	DataDir         string
	DB              *database.DB
	Detector        *tool.ToolDetector
	RuleResolver    *tool.RuleResolver
	RuleManager     *tool.RuleManager
	SnapshotService *snapshot.Service
}

// New 按“日志 -> 数据库 -> 规则 -> 业务服务”的顺序构建运行时。
func New(options Options) (*Runtime, error) {
	dataDir := options.DataDir
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}

	logConfig := logger.DefaultConfig()
	if options.DisableConsoleLog {
		logConfig.ConsoleOut = false
	}

	if err := logger.Init(logConfig); err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	db, err := database.InitDB(dataDir)
	if err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	ruleManager := tool.NewRuleManager(db)
	resolver := tool.NewRuleResolver(ruleManager.Store())
	detector := tool.NewToolDetectorWithResolver(resolver)

	return &Runtime{
		Version:         options.Version,
		DataDir:         dataDir,
		DB:              db,
		Detector:        detector,
		RuleResolver:    resolver,
		RuleManager:     ruleManager,
		SnapshotService: snapshot.NewService(db, resolver),
	}, nil
}

// DefaultDataDir 返回默认数据目录；用户主目录不可用时退回相对目录。
func DefaultDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".ai-sync-manager"
	}
	return filepath.Join(homeDir, ".ai-sync-manager")
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
