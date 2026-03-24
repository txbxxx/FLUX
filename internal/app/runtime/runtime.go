package runtime

import (
	"fmt"
	"os"
	"path/filepath"

	"ai-sync-manager/internal/database"
	"ai-sync-manager/internal/service/snapshot"
	"ai-sync-manager/internal/service/tool"
	"ai-sync-manager/pkg/logger"
)

type Options struct {
	Version           string
	DataDir           string
	DisableConsoleLog bool
}

type Runtime struct {
	Version         string
	DataDir         string
	DB              *database.DB
	Detector        *tool.ToolDetector
	SnapshotService *snapshot.Service
}

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

	detector := tool.NewToolDetector()

	return &Runtime{
		Version:         options.Version,
		DataDir:         dataDir,
		DB:              db,
		Detector:        detector,
		SnapshotService: snapshot.NewService(db, detector),
	}, nil
}

func DefaultDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".ai-sync-manager"
	}
	return filepath.Join(homeDir, ".ai-sync-manager")
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	if r.DB != nil {
		_ = r.DB.Close()
	}
	return logger.Sync()
}
