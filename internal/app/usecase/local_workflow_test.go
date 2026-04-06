package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/internal/service/tool"
	typesSnapshot "ai-sync-manager/internal/types/snapshot"
)

type stubDetector struct {
	result   *tool.ToolDetectionResult
	err      error
	lastOpts *tool.ScanOptions
}

func (s *stubDetector) DetectWithOptions(_ context.Context, opts *tool.ScanOptions) (*tool.ToolDetectionResult, error) {
	s.lastOpts = opts
	return s.result, s.err
}

type stubScanRuleManager struct {
	addCustomTool tool.ToolType
	addCustomPath string
	addCustomErr  error

	removeCustomTool tool.ToolType
	removeCustomPath string
	removeCustomErr  error

	addProjectTool tool.ToolType
	addProjectName string
	addProjectPath string
	addProjectErr  error

	removeProjectTool tool.ToolType
	removeProjectPath string
	removeProjectErr  error

	customRules []models.CustomSyncRule
	projects    []models.RegisteredProject
	listErr     error
}

func (s *stubScanRuleManager) AddCustomRule(toolType tool.ToolType, absolutePath string) error {
	s.addCustomTool = toolType
	s.addCustomPath = absolutePath
	return s.addCustomErr
}

func (s *stubScanRuleManager) RemoveCustomRule(toolType tool.ToolType, absolutePath string) error {
	s.removeCustomTool = toolType
	s.removeCustomPath = absolutePath
	return s.removeCustomErr
}

func (s *stubScanRuleManager) AddProject(toolType tool.ToolType, projectName, projectPath string) error {
	s.addProjectTool = toolType
	s.addProjectName = projectName
	s.addProjectPath = projectPath
	return s.addProjectErr
}

func (s *stubScanRuleManager) RemoveProject(toolType tool.ToolType, projectPath string) error {
	s.removeProjectTool = toolType
	s.removeProjectPath = projectPath
	return s.removeProjectErr
}

func (s *stubScanRuleManager) ListCustomRules(toolType *tool.ToolType) ([]models.CustomSyncRule, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if toolType == nil {
		return s.customRules, nil
	}
	filtered := make([]models.CustomSyncRule, 0, len(s.customRules))
	for _, rule := range s.customRules {
		if rule.ToolType == toolType.String() {
			filtered = append(filtered, rule)
		}
	}
	return filtered, nil
}

func (s *stubScanRuleManager) ListRegisteredProjects(toolType *tool.ToolType) ([]models.RegisteredProject, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if toolType == nil {
		return s.projects, nil
	}
	filtered := make([]models.RegisteredProject, 0, len(s.projects))
	for _, project := range s.projects {
		if project.ToolType == toolType.String() {
			filtered = append(filtered, project)
		}
	}
	return filtered, nil
}

type stubSnapshotService struct {
	createInput  typesSnapshot.CreateSnapshotOptions
	createResult *typesSnapshot.SnapshotPackage
	createErr    error
	listLimit    int
	listOffset   int
	listResult   []*models.Snapshot
	listErr      error
	countResult  int
	countErr     error
}

func (s *stubSnapshotService) CreateSnapshot(input typesSnapshot.CreateSnapshotOptions) (*typesSnapshot.SnapshotPackage, error) {
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
			ProjectInstallations: []*tool.ToolInstallation{
				{
					Scope:       tool.ScopeProject,
					ToolType:    tool.ToolTypeCodex,
					Status:      tool.StatusInstalled,
					ProjectName: "demo",
					ProjectPath: "/workspace/demo",
					ConfigFiles: []tool.ConfigFile{
						{Name: ".codex", Path: "/workspace/demo/.codex", Category: tool.CategoryConfigFile, IsDir: true},
						{Name: "AGENTS.md", Path: "/workspace/demo/AGENTS.md", Category: tool.CategoryAgents},
					},
				},
				{
					Scope:       tool.ScopeProject,
					ToolType:    tool.ToolTypeClaude,
					Status:      tool.StatusInstalled,
					ProjectName: "claude-global",
					ProjectPath: "/home/test/.claude",
					ConfigFiles: []tool.ConfigFile{
						{Name: "settings.json", Path: "/home/test/.claude/settings.json", Category: tool.CategoryConfigFile},
						{Name: "CLAUDE.md", Path: "/home/test/.claude/CLAUDE.md", Category: tool.CategoryConfigFile},
					},
				},
			},
		},
	}

	workflow := NewLocalWorkflow(detector, &stubSnapshotService{})

	result, err := workflow.Scan(context.Background(), ScanInput{})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if detector.lastOpts == nil {
		t.Fatal("expected scan options to be passed to detector")
	}
	if !detector.lastOpts.ScanGlobal || !detector.lastOpts.ScanProjects || !detector.lastOpts.IncludeFiles || detector.lastOpts.MaxDepth != 1 {
		t.Fatalf("unexpected scan options: %+v", detector.lastOpts)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("expected 2 scan entries, got %d", len(result.Tools))
	}
	if result.Tools[0].Tool != "codex" || result.Tools[0].Scope != "project" || result.Tools[0].ProjectName != "demo" || result.Tools[0].ConfigCount != 2 {
		t.Fatalf("unexpected first tool summary: %+v", result.Tools[0])
	}
	if result.Tools[0].ResultText != "可同步" {
		t.Fatalf("expected demo to be syncable, got %+v", result.Tools[0])
	}
	if len(result.Tools[0].Items) != 2 || result.Tools[0].Items[0].Label != "项目配置目录" {
		t.Fatalf("expected mapped config items, got %+v", result.Tools[0].Items)
	}
	if result.Tools[1].Tool != "claude" || result.Tools[1].Scope != "project" || result.Tools[1].ProjectName != "claude-global" {
		t.Fatalf("unexpected second tool summary: %+v", result.Tools[1])
	}
}

func TestLocalWorkflowScanExplainsWhyToolIsNotSyncable(t *testing.T) {
	workflow := NewLocalWorkflow(&stubDetector{
		result: &tool.ToolDetectionResult{
			ProjectInstallations: []*tool.ToolInstallation{
				{
					Scope:       tool.ScopeProject,
					ToolType:    tool.ToolTypeCodex,
					Status:      tool.StatusNotInstalled,
					ProjectName: "codex-global",
					ProjectPath: "/home/test/.codex",
				},
				{
					Scope:       tool.ScopeProject,
					ToolType:    tool.ToolTypeClaude,
					Status:      tool.StatusPartial,
					ProjectName: "claude-global",
					ProjectPath: "/home/test/.claude",
				},
			},
		},
	}, &stubSnapshotService{})

	result, err := workflow.Scan(context.Background(), ScanInput{})
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
		createResult: &typesSnapshot.SnapshotPackage{
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
		Tools:       []string{"codex", "claude"},
		Message:     "backup before change",
		Name:        "Daily backup",
		ProjectName: "codex-global",
	})
	if err != nil {
		t.Fatalf("CreateSnapshot() error = %v", err)
	}

	if service.createInput.ProjectName != "codex-global" || service.createInput.Message != "backup before change" {
		t.Fatalf("unexpected create input: %+v", service.createInput)
	}
	if result.ID != "snap-123" || result.FileCount != 4 || result.Size != 2048 {
		t.Fatalf("unexpected snapshot summary: %+v", result)
	}
}

func TestLocalWorkflowScanFiltersApps(t *testing.T) {
	workflow := NewLocalWorkflow(&stubDetector{
		result: &tool.ToolDetectionResult{
			ProjectInstallations: []*tool.ToolInstallation{
				{ToolType: tool.ToolTypeCodex, Status: tool.StatusInstalled, ProjectName: "demo", ProjectPath: "/workspace/demo"},
				{ToolType: tool.ToolTypeClaude, Status: tool.StatusInstalled, ProjectName: "claude-global", ProjectPath: "/home/test/.claude"},
			},
		},
	}, &stubSnapshotService{})

	result, err := workflow.Scan(context.Background(), ScanInput{Apps: []string{"claude"}})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(result.Tools) != 1 || result.Tools[0].Tool != "claude" {
		t.Fatalf("expected only claude project entry, got %+v", result.Tools)
	}
}

func TestLocalWorkflowScanFiltersRegisteredProjectByName(t *testing.T) {
	workflow := NewLocalWorkflow(&stubDetector{
		result: &tool.ToolDetectionResult{
			ProjectInstallations: []*tool.ToolInstallation{
				{ToolType: tool.ToolTypeCodex, Status: tool.StatusInstalled, ProjectName: "ai-sync-manager", ProjectPath: "/workspace/ai-sync-manager"},
			},
		},
	}, &stubSnapshotService{})

	result, err := workflow.Scan(context.Background(), ScanInput{Apps: []string{"ai-sync-manager"}})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(result.Tools) != 1 || result.Tools[0].ProjectName != "ai-sync-manager" || result.Tools[0].Scope != "project" {
		t.Fatalf("expected only the registered project entry, got %+v", result.Tools)
	}
}

func TestLocalWorkflowAddProjectDelegatesToRuleManager(t *testing.T) {
	ruleManager := &stubScanRuleManager{}
	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}).WithScanRuleManager(ruleManager)

	err := workflow.AddProject(context.Background(), AddProjectInput{
		App:         "codex",
		ProjectName: "demo",
		ProjectPath: `D:\workspace\demo`,
	})
	if err != nil {
		t.Fatalf("AddProject() error = %v", err)
	}
	if ruleManager.addProjectTool != tool.ToolTypeCodex || ruleManager.addProjectName != "demo" || ruleManager.addProjectPath != `D:\workspace\demo` {
		t.Fatalf("unexpected add project call: %+v", ruleManager)
	}
}

func TestLocalWorkflowListRulesIncludesDefaultsAndRegisteredData(t *testing.T) {
	ruleManager := &stubScanRuleManager{
		customRules: []models.CustomSyncRule{
			{ToolType: "claude", AbsolutePath: `C:\Users\tester\.claude.json`},
		},
		projects: []models.RegisteredProject{
			{ToolType: "claude", ProjectName: "demo", ProjectPath: `D:\workspace\demo`},
		},
	}
	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}).WithScanRuleManager(ruleManager)

	result, err := workflow.ListScanRules(context.Background(), ListScanRulesInput{App: "claude"})
	if err != nil {
		t.Fatalf("ListScanRules() error = %v", err)
	}
	if result.App != "claude" {
		t.Fatalf("expected claude app, got %+v", result)
	}
	if len(result.DefaultGlobalRules) == 0 || len(result.ProjectRuleTemplates) == 0 {
		t.Fatalf("expected defaults and templates, got %+v", result)
	}
	if len(result.CustomRules) != 1 || len(result.RegisteredProjects) != 1 {
		t.Fatalf("expected custom rules and projects, got %+v", result)
	}
}

func TestLocalWorkflowListRulesSupportsRegisteredProjectNameFilter(t *testing.T) {
	ruleManager := &stubScanRuleManager{
		customRules: []models.CustomSyncRule{
			{ToolType: "codex", AbsolutePath: `D:\custom\codex-extra.toml`},
			{ToolType: "claude", AbsolutePath: `C:\Users\tester\.claude.json`},
		},
		projects: []models.RegisteredProject{
			{ToolType: "codex", ProjectName: "ai-sync-manager", ProjectPath: `D:\workspace\ai-sync-manager`},
			{ToolType: "claude", ProjectName: "demo", ProjectPath: `D:\workspace\demo`},
		},
	}
	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}).WithScanRuleManager(ruleManager)

	result, err := workflow.ListScanRules(context.Background(), ListScanRulesInput{App: "ai-sync-manager"})
	if err != nil {
		t.Fatalf("ListScanRules() error = %v", err)
	}
	if result.App != "codex" {
		t.Fatalf("expected codex rules for project filter, got %+v", result)
	}
	if len(result.RegisteredProjects) != 1 || result.RegisteredProjects[0].Name != "ai-sync-manager" {
		t.Fatalf("expected only matched project, got %+v", result.RegisteredProjects)
	}
	if len(result.DefaultGlobalRules) == 0 || len(result.ProjectRuleTemplates) == 0 {
		t.Fatalf("expected codex defaults and project templates, got %+v", result)
	}
	if len(result.CustomRules) != 1 || result.CustomRules[0].Path != `D:\custom\codex-extra.toml` {
		t.Fatalf("expected codex-only custom rules, got %+v", result.CustomRules)
	}
}

func TestLocalWorkflowCreateSnapshotValidatesTools(t *testing.T) {
	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{})

	_, err := workflow.CreateSnapshot(context.Background(), CreateSnapshotInput{
		Message: "missing tools",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}

	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected UserError, got %T", err)
	}
	if userErr.Message != "创建快照失败：无法确定工具类型" {
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
