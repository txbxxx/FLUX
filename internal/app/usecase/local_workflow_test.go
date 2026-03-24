package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/internal/service/tool"
)

type stubDetector struct {
	result  *tool.ToolDetectionResult
	err     error
	lastOpts *tool.ScanOptions
}

func (s *stubDetector) DetectWithOptions(_ context.Context, opts *tool.ScanOptions) (*tool.ToolDetectionResult, error) {
	s.lastOpts = opts
	return s.result, s.err
}

type stubSnapshotService struct {
	createInput models.CreateSnapshotOptions
	createResult *models.SnapshotPackage
	createErr   error
	listLimit   int
	listOffset  int
	listResult  []*models.Snapshot
	listErr     error
	countResult int
	countErr    error
}

func (s *stubSnapshotService) CreateSnapshot(input models.CreateSnapshotOptions) (*models.SnapshotPackage, error) {
	s.createInput = input
	return s.createResult, s.createErr
}

func (s *stubSnapshotService) ListSnapshots(limit, offset int) ([]*models.Snapshot, error) {
	s.listLimit = limit
	s.listOffset = offset
	return s.listResult, s.listErr
}

func (s *stubSnapshotService) CountSnapshots() (int, error) {
	return s.countResult, s.countErr
}

func TestLocalWorkflowScanMapsDetectedTools(t *testing.T) {
	detector := &stubDetector{
		result: &tool.ToolDetectionResult{
			Codex: &tool.ToolInstallation{
				ToolType:    tool.ToolTypeCodex,
				Status:      tool.StatusInstalled,
				GlobalPath:  "/home/test/.codex",
				ConfigFiles: []tool.ConfigFile{
					{Name: "config.toml", Path: "/home/test/.codex/config.toml", Category: tool.CategoryConfigFile},
					{Name: "skills", Path: "/home/test/.codex/skills", Category: tool.CategorySkills, IsDir: true},
				},
			},
			Claude: &tool.ToolInstallation{
				ToolType:    tool.ToolTypeClaude,
				Status:      tool.StatusInstalled,
				GlobalPath:  "/home/test/.claude",
				ConfigFiles: []tool.ConfigFile{{Name: "settings.json", Path: "/home/test/.claude/settings.json", Category: tool.CategoryConfigFile}},
			},
		},
	}

	workflow := NewLocalWorkflow(detector, &stubSnapshotService{})

	result, err := workflow.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if detector.lastOpts == nil {
		t.Fatal("expected scan options to be passed to detector")
	}
	if !detector.lastOpts.ScanGlobal || detector.lastOpts.ScanProjects || !detector.lastOpts.IncludeFiles || detector.lastOpts.MaxDepth != 1 {
		t.Fatalf("unexpected scan options: %+v", detector.lastOpts)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result.Tools))
	}
	if result.Tools[0].Tool != "codex" || result.Tools[0].ConfigCount != 2 {
		t.Fatalf("unexpected first tool summary: %+v", result.Tools[0])
	}
	if result.Tools[0].ResultText != "可同步" {
		t.Fatalf("expected codex to be syncable, got %+v", result.Tools[0])
	}
	if len(result.Tools[0].Items) != 2 || result.Tools[0].Items[0].Label != "主配置" {
		t.Fatalf("expected mapped config items, got %+v", result.Tools[0].Items)
	}
	if result.Tools[1].Tool != "claude" || result.Tools[1].ResultText != "可同步" {
		t.Fatalf("unexpected second tool summary: %+v", result.Tools[1])
	}
}

func TestLocalWorkflowScanExplainsWhyToolIsNotSyncable(t *testing.T) {
	workflow := NewLocalWorkflow(&stubDetector{
		result: &tool.ToolDetectionResult{
			Codex: &tool.ToolInstallation{
				ToolType: tool.ToolTypeCodex,
				Status:   tool.StatusNotInstalled,
			},
			Claude: &tool.ToolInstallation{
				ToolType:   tool.ToolTypeClaude,
				Status:     tool.StatusPartial,
				GlobalPath: "/home/test/.claude",
			},
		},
	}, &stubSnapshotService{})

	result, err := workflow.Scan(context.Background())
	if err != nil {
		t.Fatalf("expected scan result with reasons, got error %v", err)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result.Tools))
	}
	if result.Tools[0].ResultText != "不可同步" || result.Tools[0].Reason == "" {
		t.Fatalf("expected codex reason, got %+v", result.Tools[0])
	}
	if result.Tools[1].ResultText != "暂不可同步" || result.Tools[1].Reason == "" {
		t.Fatalf("expected claude partial reason, got %+v", result.Tools[1])
	}
}

func TestLocalWorkflowCreateSnapshotMapsServiceResult(t *testing.T) {
	now := time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)
	service := &stubSnapshotService{
		createResult: &models.SnapshotPackage{
			Snapshot: &models.Snapshot{
				ID:        "snap-123",
				Name:      "Daily backup",
				Message:   "backup before change",
				CreatedAt: now,
				Tools:     []string{"codex", "claude"},
			},
			FileCount: 4,
			Size:      2048,
			CreatedAt: now,
		},
	}
	workflow := NewLocalWorkflow(&stubDetector{}, service)

	result, err := workflow.CreateSnapshot(context.Background(), CreateSnapshotInput{
		Tools:   []string{"codex", "claude"},
		Message: "backup before change",
		Name:    "Daily backup",
		Scope:   models.ScopeGlobal,
	})
	if err != nil {
		t.Fatalf("CreateSnapshot() error = %v", err)
	}

	if service.createInput.Scope != models.ScopeGlobal || service.createInput.Message != "backup before change" {
		t.Fatalf("unexpected create input: %+v", service.createInput)
	}
	if result.ID != "snap-123" || result.FileCount != 4 || result.Size != 2048 {
		t.Fatalf("unexpected snapshot summary: %+v", result)
	}
}

func TestLocalWorkflowCreateSnapshotValidatesTools(t *testing.T) {
	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{})

	_, err := workflow.CreateSnapshot(context.Background(), CreateSnapshotInput{
		Message: "missing tools",
		Scope:   models.ScopeGlobal,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected UserError, got %T", err)
	}
	if userErr.Message != "创建快照失败：工具列表不能为空" {
		t.Fatalf("unexpected user-facing message: %q", userErr.Message)
	}
}

func TestLocalWorkflowCreateSnapshotMapsNoConfigFilesError(t *testing.T) {
	service := &stubSnapshotService{
		createErr: errors.New("未找到任何配置文件"),
	}
	workflow := NewLocalWorkflow(&stubDetector{}, service)

	_, err := workflow.CreateSnapshot(context.Background(), CreateSnapshotInput{
		Tools:   []string{"codex"},
		Message: "backup",
		Scope:   models.ScopeGlobal,
	})
	if err == nil {
		t.Fatal("expected create snapshot error")
	}

	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected UserError, got %T", err)
	}
	if userErr.Suggestion == "" {
		t.Fatal("expected suggestion for missing configuration files")
	}
}

func TestLocalWorkflowListSnapshotsUsesDefaultPagination(t *testing.T) {
	now := time.Date(2026, 3, 23, 11, 0, 0, 0, time.UTC)
	service := &stubSnapshotService{
		listResult: []*models.Snapshot{
			{
				ID:        "snap-2",
				Name:      "Snapshot 2",
				Message:   "second",
				CreatedAt: now,
				Tools:     []string{"codex"},
			},
		},
		countResult: 1,
	}
	workflow := NewLocalWorkflow(&stubDetector{}, service)

	result, err := workflow.ListSnapshots(context.Background(), ListSnapshotsInput{})
	if err != nil {
		t.Fatalf("ListSnapshots() error = %v", err)
	}

	if service.listLimit != DefaultListLimit || service.listOffset != 0 {
		t.Fatalf("unexpected pagination: limit=%d offset=%d", service.listLimit, service.listOffset)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected list result: %+v", result)
	}
	if result.Items[0].ID != "snap-2" || result.Items[0].Message != "second" {
		t.Fatalf("unexpected snapshot summary: %+v", result.Items[0])
	}
}
