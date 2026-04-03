package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// App
	if cfg.App.Version != "1.0.0-alpha" {
		t.Errorf("App.Version = %q, want %q", cfg.App.Version, "1.0.0-alpha")
	}
	if cfg.App.DataDir == "" {
		t.Error("App.DataDir should not be empty")
	}

	// Logger
	if cfg.Logger.Level != "info" {
		t.Errorf("Logger.Level = %q, want %q", cfg.Logger.Level, "info")
	}
	if cfg.Logger.MaxSize != 10 {
		t.Errorf("Logger.MaxSize = %d, want 10", cfg.Logger.MaxSize)
	}
	if cfg.Logger.MaxBackups != 5 {
		t.Errorf("Logger.MaxBackups = %d, want 5", cfg.Logger.MaxBackups)
	}
	if cfg.Logger.MaxAge != 30 {
		t.Errorf("Logger.MaxAge = %d, want 30", cfg.Logger.MaxAge)
	}
	if !cfg.Logger.Compress {
		t.Error("Logger.Compress should be true")
	}
	if !cfg.Logger.ConsoleOut {
		t.Error("Logger.ConsoleOut should be true")
	}

	// Database
	if cfg.Database.Filename != "ai-sync-manager.db" {
		t.Errorf("Database.Filename = %q, want %q", cfg.Database.Filename, "ai-sync-manager.db")
	}
	if cfg.Database.MaxOpenConns != 1 {
		t.Errorf("Database.MaxOpenConns = %d, want 1", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.ConnMaxLifetime != "1h" {
		t.Errorf("Database.ConnMaxLifetime = %q, want %q", cfg.Database.ConnMaxLifetime, "1h")
	}

	// Sync
	if cfg.Sync.DefaultBranch != "main" {
		t.Errorf("Sync.DefaultBranch = %q, want %q", cfg.Sync.DefaultBranch, "main")
	}
	if cfg.Sync.DefaultRemote != "origin" {
		t.Errorf("Sync.DefaultRemote = %q, want %q", cfg.Sync.DefaultRemote, "origin")
	}

	// Tools
	if len(cfg.Tools) != 2 {
		t.Fatalf("Tools count = %d, want 2", len(cfg.Tools))
	}
	codex, ok := cfg.Tools["codex"]
	if !ok {
		t.Fatal("Tools should contain 'codex'")
	}
	if codex.GlobalDir != ".codex" {
		t.Errorf("codex.GlobalDir = %q, want %q", codex.GlobalDir, ".codex")
	}
	if len(codex.GlobalRules) != 4 {
		t.Errorf("codex.GlobalRules count = %d, want 4", len(codex.GlobalRules))
	}
	if len(codex.ProjectRules) != 2 {
		t.Errorf("codex.ProjectRules count = %d, want 2", len(codex.ProjectRules))
	}

	claude, ok := cfg.Tools["claude"]
	if !ok {
		t.Fatal("Tools should contain 'claude'")
	}
	if len(claude.GlobalRules) != 8 {
		t.Errorf("claude.GlobalRules count = %d, want 8", len(claude.GlobalRules))
	}
}

func TestLoadFrom(t *testing.T) {
	yaml := `
app:
  version: "2.0.0"
  data_dir: "/tmp/test-data"
logger:
  level: "debug"
  max_size: 20
database:
  filename: "test.db"
sync:
  default_branch: "develop"
`

	cfg, err := LoadFrom([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	if cfg.App.Version != "2.0.0" {
		t.Errorf("App.Version = %q, want %q", cfg.App.Version, "2.0.0")
	}
	if cfg.App.DataDir != "/tmp/test-data" {
		t.Errorf("App.DataDir = %q, want %q", cfg.App.DataDir, "/tmp/test-data")
	}
	if cfg.Logger.Level != "debug" {
		t.Errorf("Logger.Level = %q, want %q", cfg.Logger.Level, "debug")
	}
	if cfg.Logger.MaxSize != 20 {
		t.Errorf("Logger.MaxSize = %d, want 20", cfg.Logger.MaxSize)
	}
	if cfg.Database.Filename != "test.db" {
		t.Errorf("Database.Filename = %q, want %q", cfg.Database.Filename, "test.db")
	}
	if cfg.Sync.DefaultBranch != "develop" {
		t.Errorf("Sync.DefaultBranch = %q, want %q", cfg.Sync.DefaultBranch, "develop")
	}
}

func TestLoadFromDefaultYAML(t *testing.T) {
	cfg, err := LoadFrom(defaultYAML)
	if err != nil {
		t.Fatalf("LoadFrom embedded default YAML failed: %v", err)
	}

	if cfg.App.Version != "1.0.0-alpha" {
		t.Errorf("App.Version = %q, want %q", cfg.App.Version, "1.0.0-alpha")
	}
	if len(cfg.Tools) != 2 {
		t.Errorf("Tools count = %d, want 2", len(cfg.Tools))
	}
}

func TestMerge(t *testing.T) {
	defaults := DefaultConfig()

	userYAML := `
logger:
  level: "debug"
  max_size: 50
sync:
  default_branch: "develop"
`
	userCfg, err := LoadFrom([]byte(userYAML))
	if err != nil {
		t.Fatalf("LoadFrom user config failed: %v", err)
	}

	merge(defaults, userCfg)

	// 覆盖的值
	if defaults.Logger.Level != "debug" {
		t.Errorf("Logger.Level = %q, want %q", defaults.Logger.Level, "debug")
	}
	if defaults.Logger.MaxSize != 50 {
		t.Errorf("Logger.MaxSize = %d, want 50", defaults.Logger.MaxSize)
	}
	if defaults.Sync.DefaultBranch != "develop" {
		t.Errorf("Sync.DefaultBranch = %q, want %q", defaults.Sync.DefaultBranch, "develop")
	}

	// 未覆盖的值保持不变
	if defaults.App.Version != "1.0.0-alpha" {
		t.Errorf("App.Version should remain %q", "1.0.0-alpha")
	}
	if defaults.Database.Filename != "ai-sync-manager.db" {
		t.Errorf("Database.Filename should remain %q", "ai-sync-manager.db")
	}
	if defaults.Sync.DefaultRemote != "origin" {
		t.Errorf("Sync.DefaultRemote should remain %q", "origin")
	}
}

func TestMergeNewTool(t *testing.T) {
	defaults := DefaultConfig()

	userYAML := `
tools:
  cursor:
    global_dir: ".cursor"
    project_dir: ".cursor"
    required_global_paths:
      - "settings.json"
    global_rules:
      - path: "settings.json"
        category: "config"
        is_dir: false
`
	userCfg, err := LoadFrom([]byte(userYAML))
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	merge(defaults, userCfg)

	if len(defaults.Tools) != 3 {
		t.Errorf("Tools count = %d, want 3", len(defaults.Tools))
	}
	cursor, ok := defaults.Tools["cursor"]
	if !ok {
		t.Fatal("Tools should contain 'cursor'")
	}
	if cursor.GlobalDir != ".cursor" {
		t.Errorf("cursor.GlobalDir = %q, want %q", cursor.GlobalDir, ".cursor")
	}
}

func TestGetConnMaxLifetime(t *testing.T) {
	cfg := &DatabaseConfig{ConnMaxLifetime: "2h30m"}
	dur := cfg.GetConnMaxLifetime()
	if dur != 2*time.Hour+30*time.Minute {
		t.Errorf("GetConnMaxLifetime = %v, want 2h30m", dur)
	}

	cfgEmpty := &DatabaseConfig{ConnMaxLifetime: ""}
	durEmpty := cfgEmpty.GetConnMaxLifetime()
	if durEmpty != time.Hour {
		t.Errorf("Empty ConnMaxLifetime should default to 1h, got %v", durEmpty)
	}
}

func TestLoadNoUserFile(t *testing.T) {
	homeDir, _ := os.UserHomeDir()
	userPath := filepath.Join(homeDir, ".ai-sync-manager", "config.yaml")

	if _, err := os.Stat(userPath); err == nil {
		t.Skip("用户配置文件已存在，跳过")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.App.Version != "1.0.0-alpha" {
		t.Errorf("App.Version = %q, want %q", cfg.App.Version, "1.0.0-alpha")
	}
}

func TestLoadFromInvalidYAML(t *testing.T) {
	_, err := LoadFrom([]byte("invalid: [yaml: content"))
	if err == nil {
		t.Error("LoadFrom should return error for invalid YAML")
	}
}
