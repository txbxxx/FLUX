package tool

import (
	"path/filepath"
	"testing"

	"flux/pkg/database"
)

func TestRuleManagerAddCustomRuleRequiresAbsoluteFile(t *testing.T) {
	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	manager := NewRuleManager(db)
	err = manager.AddCustomRule(ToolTypeClaude, ".claude.json")
	if err == nil {
		t.Fatal("expected relative path to be rejected")
	}
}

func TestRuleManagerStoresNormalizedRulesAndProjects(t *testing.T) {
	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	tempDir := t.TempDir()
	customFile := filepath.Join(tempDir, "nested", "..", "claude.json")
	CreateMockFile(t, customFile, `{"mcpServers":[]}`)

	projectPath := filepath.Join(tempDir, "project", ".")
	CreateMockFile(t, filepath.Join(projectPath, ".codex", "config.toml"), "[codex]")

	manager := NewRuleManager(db)
	if err := manager.AddCustomRule(ToolTypeClaude, customFile); err != nil {
		t.Fatalf("add custom rule: %v", err)
	}
	if err := manager.AddProject(ToolTypeCodex, "demo", projectPath); err != nil {
		t.Fatalf("add project: %v", err)
	}

	customRules, err := manager.ListCustomRules(nil)
	if err != nil {
		t.Fatalf("list custom rules: %v", err)
	}
	if len(customRules) != 1 {
		t.Fatalf("expected 1 custom rule, got %+v", customRules)
	}
	if customRules[0].AbsolutePath != filepath.Clean(customFile) {
		t.Fatalf("expected normalized custom path, got %q", customRules[0].AbsolutePath)
	}

	projects, err := manager.ListRegisteredProjects(nil)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %+v", projects)
	}
	if projects[0].ProjectPath != filepath.Clean(projectPath) {
		t.Fatalf("expected normalized project path, got %q", projects[0].ProjectPath)
	}
}
