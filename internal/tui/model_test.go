package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ai-sync-manager/internal/app/usecase"
)

type stubWorkflow struct {
	scanResult   *usecase.ScanResult
	scanErr      error
	createInput  usecase.CreateSnapshotInput
	createResult *usecase.SnapshotSummary
	createErr    error
	listResult   *usecase.ListSnapshotsResult
	listErr      error
}

func (s *stubWorkflow) Scan(context.Context, usecase.ScanInput) (*usecase.ScanResult, error) {
	return s.scanResult, s.scanErr
}

func (s *stubWorkflow) CreateSnapshot(_ context.Context, input usecase.CreateSnapshotInput) (*usecase.SnapshotSummary, error) {
	s.createInput = input
	return s.createResult, s.createErr
}

func (s *stubWorkflow) ListSnapshots(_ context.Context, _ usecase.ListSnapshotsInput) (*usecase.ListSnapshotsResult, error) {
	return s.listResult, s.listErr
}

func TestModelHomeQQuits(t *testing.T) {
	model := NewModel(&stubWorkflow{}, "D:/data")

	updated, cmd := model.Update(keyRunes("q"))
	next := updated.(*Model)

	if !next.Quitting {
		t.Fatal("expected model to enter quitting state")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestModelHomeEnterLoadsScanPage(t *testing.T) {
	model := NewModel(&stubWorkflow{
		scanResult: &usecase.ScanResult{
			Tools: []usecase.ToolSummary{
				{
					Tool:        "codex",
					Scope:       "global",
					Status:      "installed",
					ResultText:  "可同步",
					Path:        "D:/Users/test/.codex",
					ConfigCount: 2,
					Reason:      "",
				},
			},
		},
	}, "D:/data")

	updated, cmd := model.Update(keyEnter())
	if cmd == nil {
		t.Fatal("expected scan command")
	}

	updated, _ = updated.Update(cmd())
	next := updated.(*Model)
	if next.Page != PageScan {
		t.Fatalf("expected scan page, got %q", next.Page)
	}
	if next.ScanResult == nil || len(next.ScanResult.Tools) != 1 {
		t.Fatalf("unexpected scan result: %+v", next.ScanResult)
	}
}

func TestScanViewShowsChineseSummary(t *testing.T) {
	model := NewModel(&stubWorkflow{}, "D:/data")
	model.Page = PageScan
	model.ScanResult = &usecase.ScanResult{
		Tools: []usecase.ToolSummary{
			{
				Tool:        "claude",
				Scope:       "project",
				ProjectName: "demo",
				ResultText:  "暂不可同步",
				Path:        "D:/workspace/demo",
				ConfigCount: 1,
				Reason:      "已注册项目下未识别到可同步的配置文件",
				Items: []usecase.ToolConfigItem{
					{Group: "关键配置", Label: "说明文件", RelativePath: "CLAUDE.md"},
				},
			},
		},
	}

	view := model.View()
	if !strings.Contains(view, "demo（Claude 项目）") {
		t.Fatalf("expected tool name in view, got %q", view)
	}
	if !strings.Contains(view, "检测结果: 暂不可同步") {
		t.Fatalf("expected chinese result in view, got %q", view)
	}
	if !strings.Contains(view, "原因: 已注册项目下未识别到可同步的配置文件") {
		t.Fatalf("expected reason in view, got %q", view)
	}
	if !strings.Contains(view, "说明文件: CLAUDE.md") {
		t.Fatalf("expected project config item in view, got %q", view)
	}
}

func TestModelCreateSnapshotSuccessMovesToSnapshots(t *testing.T) {
	model := NewModel(&stubWorkflow{
		createResult: &usecase.SnapshotSummary{
			ID:        "snap-1",
			Name:      "Snapshot 1",
			Message:   "created from tui",
			Project:   "codex",
			FileCount: 2,
			Size:      512,
			CreatedAt: time.Date(2026, 3, 23, 12, 30, 0, 0, time.UTC),
		},
		listResult: &usecase.ListSnapshotsResult{
			Total: 1,
			Items: []usecase.SnapshotSummary{
				{
					ID:        "snap-1",
					Name:      "Snapshot 1",
					Message:   "created from tui",
					Project:   "codex",
					FileCount: 2,
					Size:      512,
					CreatedAt: time.Date(2026, 3, 23, 12, 30, 0, 0, time.UTC),
				},
			},
		},
	}, "D:/data")

	model.Page = PageCreate
	model.Form.Tools = "codex"
	model.Form.Message = "created from tui"
	model.Form.Name = "Snapshot 1"
	model.FormFocus = formFocusSubmit

	updated, cmd := model.Update(keyEnter())
	if cmd == nil {
		t.Fatal("expected submit command")
	}

	updated, _ = updated.Update(cmd())
	next := updated.(*Model)
	if next.Page != PageSnapshots {
		t.Fatalf("expected snapshots page after create, got %q", next.Page)
	}
	if len(next.Snapshots) != 1 || next.Snapshots[0].ID != "snap-1" {
		t.Fatalf("unexpected snapshots state: %+v", next.Snapshots)
	}
}

func TestSnapshotListViewShowsEmptyState(t *testing.T) {
	model := NewModel(&stubWorkflow{}, "D:/data")
	model.Page = PageSnapshots

	if view := model.View(); !strings.Contains(view, "还没有快照") {
		t.Fatalf("expected empty state in view, got %q", view)
	}
}

func keyEnter() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
