package models

import (
	"testing"
	"time"

	"flux/pkg/database"
)

func TestCustomSyncRuleDAOCreateListDelete(t *testing.T) {
	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	dao := NewCustomSyncRuleDAO(db)
	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	rule := &CustomSyncRule{
		ID:           0,
		ToolType:     "claude",
		AbsolutePath: `C:\Users\tester\.claude.json`,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := dao.Create(rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	items, err := dao.ListByTool("claude")
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(items))
	}
	if items[0].AbsolutePath != rule.AbsolutePath {
		t.Fatalf("unexpected rule path: %+v", items[0])
	}

	if err := dao.DeleteByToolAndPath("claude", rule.AbsolutePath); err != nil {
		t.Fatalf("delete rule: %v", err)
	}

	items, err = dao.ListByTool("claude")
	if err != nil {
		t.Fatalf("list rules after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty rules after delete, got %d", len(items))
	}
}

func TestCustomSyncRuleDAORejectsDuplicatePath(t *testing.T) {
	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	dao := NewCustomSyncRuleDAO(db)
	now := time.Date(2026, 3, 26, 10, 30, 0, 0, time.UTC)
	rule := &CustomSyncRule{
		ID:           0,
		ToolType:     "codex",
		AbsolutePath: `D:\workspace\demo\extra.toml`,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := dao.Create(rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}

	duplicate := &CustomSyncRule{
		ID:           0,
		ToolType:     "codex",
		AbsolutePath: rule.AbsolutePath,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := dao.Create(duplicate); err == nil {
		t.Fatal("expected duplicate rule create to fail")
	}
}
