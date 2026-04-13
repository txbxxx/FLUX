package tool

import (
	"testing"
	"time"

	"flux/internal/models"
	"flux/pkg/database"
)

func TestRuleStoreListsCustomRulesAndProjects(t *testing.T) {
	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	customDAO := models.NewCustomSyncRuleDAO(db)
	projectDAO := models.NewRegisteredProjectDAO(db)
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)

	if err := customDAO.Create(&models.CustomSyncRule{
		ID:           "rule-1",
		ToolType:     "claude",
		AbsolutePath: `C:\Users\tester\.claude.json`,
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("create custom rule: %v", err)
	}

	if err := projectDAO.Create(&models.RegisteredProject{
		ID:          "project-1",
		ToolType:    "claude",
		ProjectName: "demo",
		ProjectPath: `D:\workspace\demo`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("create registered project: %v", err)
	}

	store := NewRuleStore(customDAO, projectDAO)

	customRules, err := store.ListCustomRules(ToolTypeClaude)
	if err != nil {
		t.Fatalf("list custom rules: %v", err)
	}
	if len(customRules) != 1 || customRules[0].AbsolutePath != `C:\Users\tester\.claude.json` {
		t.Fatalf("unexpected custom rules: %+v", customRules)
	}

	projects, err := store.ListRegisteredProjects(ToolTypeClaude)
	if err != nil {
		t.Fatalf("list registered projects: %v", err)
	}
	if len(projects) != 1 || projects[0].ProjectPath != `D:\workspace\demo` {
		t.Fatalf("unexpected projects: %+v", projects)
	}
}
