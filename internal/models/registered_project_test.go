package models

import (
	"testing"
	"time"

	"flux/pkg/database"
)

func TestRegisteredProjectDAOCreateListDelete(t *testing.T) {
	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	dao := NewRegisteredProjectDAO(db)
	now := time.Date(2026, 3, 26, 11, 0, 0, 0, time.UTC)
	project := &RegisteredProject{
		ID:          "project-1",
		ToolType:    "codex",
		ProjectName: "demo",
		ProjectPath: `D:\workspace\demo`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := dao.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	items, err := dao.ListByTool("codex")
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 project, got %d", len(items))
	}
	if items[0].ProjectPath != project.ProjectPath || items[0].ProjectName != project.ProjectName {
		t.Fatalf("unexpected project: %+v", items[0])
	}

	if err := dao.DeleteByToolAndPath("codex", project.ProjectPath); err != nil {
		t.Fatalf("delete project: %v", err)
	}

	items, err = dao.ListByTool("codex")
	if err != nil {
		t.Fatalf("list projects after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty projects after delete, got %d", len(items))
	}
}

func TestRegisteredProjectDAORejectsDuplicatePath(t *testing.T) {
	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	dao := NewRegisteredProjectDAO(db)
	now := time.Date(2026, 3, 26, 11, 30, 0, 0, time.UTC)
	project := &RegisteredProject{
		ID:          "project-1",
		ToolType:    "claude",
		ProjectName: "app-one",
		ProjectPath: `D:\workspace\app-one`,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := dao.Create(project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	duplicate := &RegisteredProject{
		ID:          "project-2",
		ToolType:    "claude",
		ProjectName: "app-two",
		ProjectPath: project.ProjectPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := dao.Create(duplicate); err == nil {
		t.Fatal("expected duplicate project create to fail")
	}
}
