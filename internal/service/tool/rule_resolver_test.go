package tool

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"flux/internal/models"
	"flux/pkg/database"
)

func TestRuleResolverMarksToolPartialWhenRequiredFileMissing(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	CreateMockFile(t, filepath.Join(homeDir, ".codex", "skills", "workflow.md"), "# workflow")

	resolver := NewRuleResolver(nil)
	report, err := resolver.ResolveTool(ToolTypeCodex)
	if err != nil {
		t.Fatalf("resolve tool: %v", err)
	}

	if report.Status != StatusPartial {
		t.Fatalf("expected partial status, got %s", report.Status)
	}
	if len(report.MissingRequiredPaths) != 1 || report.MissingRequiredPaths[0] != "config.toml" {
		t.Fatalf("unexpected missing required paths: %#v", report.MissingRequiredPaths)
	}
	if len(report.DefaultMatches) != 1 || report.DefaultMatches[0].RelativePath != "skills" {
		t.Fatalf("unexpected default matches: %+v", report.DefaultMatches)
	}
}

func TestRuleResolverListsCustomRulesAndMissingCustomRules(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	CreateMockFile(t, filepath.Join(homeDir, ".claude", "settings.json"), "{}")

	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	now := time.Date(2026, 3, 26, 13, 0, 0, 0, time.UTC)
	customDAO := models.NewCustomSyncRuleDAO(db)
	if err := customDAO.Create(&models.CustomSyncRule{
		ID:           0,
		ToolType:     "claude",
		AbsolutePath: filepath.Join(homeDir, ".claude.json"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("create custom rule: %v", err)
	}
	if err := customDAO.Create(&models.CustomSyncRule{
		ID:           0,
		ToolType:     "claude",
		AbsolutePath: filepath.Join(homeDir, "missing-claude.json"),
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("create missing custom rule: %v", err)
	}
	CreateMockFile(t, filepath.Join(homeDir, ".claude.json"), `{"mcpServers":[]}`)

	store := NewRuleStore(customDAO, models.NewRegisteredProjectDAO(db))
	resolver := NewRuleResolver(store)
	report, err := resolver.ResolveTool(ToolTypeClaude)
	if err != nil {
		t.Fatalf("resolve tool: %v", err)
	}

	if report.Status != StatusInstalled {
		t.Fatalf("expected installed status, got %s", report.Status)
	}
	if len(report.CustomMatches) != 1 || report.CustomMatches[0].AbsolutePath != filepath.Join(homeDir, ".claude.json") {
		t.Fatalf("unexpected custom matches: %+v", report.CustomMatches)
	}
	if len(report.MissingCustomRules) != 1 || report.MissingCustomRules[0].AbsolutePath != filepath.Join(homeDir, "missing-claude.json") {
		t.Fatalf("unexpected missing custom rules: %+v", report.MissingCustomRules)
	}
}

func TestRuleResolverMapsRegisteredProjectsToProjectTemplates(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	CreateMockFile(t, filepath.Join(homeDir, ".codex", "config.toml"), "[codex]")

	projectPath := filepath.Join(t.TempDir(), "demo-project")
	CreateMockFile(t, filepath.Join(projectPath, ".codex", "config.toml"), "[project]")
	CreateMockFile(t, filepath.Join(projectPath, "AGENTS.md"), "# agents")

	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	projectDAO := models.NewRegisteredProjectDAO(db)
	now := time.Date(2026, 3, 26, 13, 30, 0, 0, time.UTC)
	if err := projectDAO.Create(&models.RegisteredProject{
		ID:          0,
		ToolType:    "codex",
		ProjectName: "demo",
		ProjectPath: projectPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}

	store := NewRuleStore(models.NewCustomSyncRuleDAO(db), projectDAO)
	resolver := NewRuleResolver(store)
	report, err := resolver.ResolveTool(ToolTypeCodex)
	if err != nil {
		t.Fatalf("resolve tool: %v", err)
	}

	if len(report.ProjectMatches) != 1 {
		t.Fatalf("expected 1 project match, got %+v", report.ProjectMatches)
	}
	project := report.ProjectMatches[0]
	if project.ProjectPath != projectPath || project.ProjectName != "demo" {
		t.Fatalf("unexpected project match: %+v", project)
	}
	if len(project.Matches) != 2 {
		t.Fatalf("expected 2 project matches, got %+v", project.Matches)
	}
}

func TestDetectToolRequiresKeyFileForClaude(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	CreateMockFile(t, filepath.Join(homeDir, ".claude", "skills", "README.md"), "# readme")

	detector := NewToolDetector()
	result, err := detector.DetectTool(context.Background(), ToolTypeClaude, &ScanOptions{
		ScanGlobal:   true,
		IncludeFiles: true,
		MaxDepth:     1,
	})
	if err != nil {
		t.Fatalf("detect tool: %v", err)
	}

	if result.Status != StatusPartial {
		t.Fatalf("expected partial status, got %s", result.Status)
	}
}

func TestDetectToolUsesClaudePluginWhitelist(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	CreateMockFile(t, filepath.Join(homeDir, ".claude", "settings.json"), "{}")
	CreateMockFile(t, filepath.Join(homeDir, ".claude", "plugins", "extra.json"), "{}")
	CreateMockFile(t, filepath.Join(homeDir, ".claude", "plugins", "installed_plugins.json"), "[]")

	detector := NewToolDetector()
	result, err := detector.DetectTool(context.Background(), ToolTypeClaude, &ScanOptions{
		ScanGlobal:   true,
		IncludeFiles: true,
		MaxDepth:     1,
	})
	if err != nil {
		t.Fatalf("detect tool: %v", err)
	}

	if result.Status != StatusInstalled {
		t.Fatalf("expected installed status, got %s", result.Status)
	}

	for _, item := range result.ConfigFiles {
		if filepath.Base(item.Path) == "extra.json" {
			t.Fatalf("unexpected non-whitelist plugin file in config files: %+v", item)
		}
	}
}
