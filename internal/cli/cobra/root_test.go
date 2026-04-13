package cobra

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"ai-sync-manager/internal/app/usecase"
	typesRemote "ai-sync-manager/internal/types/remote"
	typesSnapshot "ai-sync-manager/internal/types/snapshot"
	typesSync "ai-sync-manager/internal/types/sync"
)

type stubWorkflow struct {
	scanInput                   usecase.ScanInput
	scanResult                  *usecase.ScanResult
	scanErr                     error
	addRuleInput                usecase.AddCustomRuleInput
	addRuleErr                  error
	removeRuleInput             usecase.RemoveCustomRuleInput
	removeRuleErr               error
	addProjectInput             usecase.AddProjectInput
	addProjectErr               error
	removeProjectInput          usecase.RemoveProjectInput
	removeProjectErr            error
	listRulesInput              usecase.ListScanRulesInput
	listRulesResult             *usecase.ListScanRulesResult
	listRulesErr                error
	createInput                 usecase.CreateSnapshotInput
	createResult                *usecase.SnapshotSummary
	createErr                   error
	listInput                   usecase.ListSnapshotsInput
	listResult                  *usecase.ListSnapshotsResult
	listErr                     error
	deleteSnapshotInput         usecase.DeleteSnapshotInput
	deleteSnapshotErr           error
	getInput                    usecase.GetConfigInput
	getResult                   *usecase.GetConfigResult
	getErr                      error
	saveInput                   usecase.SaveConfigInput
	saveErr                     error
	createAISettingInput        usecase.CreateAISettingInput
	createAISettingResult       *usecase.CreateAISettingResult
	createAISettingErr          error
	listAISettingsInput         usecase.ListAISettingsInput
	listAISettingsResult        *usecase.ListAISettingsResult
	listAISettingsErr           error
	getAISettingInput           usecase.GetAISettingInput
	getAISettingResult          *usecase.GetAISettingResult
	getAISettingErr             error
	deleteAISettingInput        usecase.DeleteAISettingInput
	deleteAISettingErr          error
	switchAISettingInput        usecase.SwitchAISettingInput
	switchAISettingResult       *usecase.SwitchAISettingResult
	switchAISettingErr          error
	getAISettingsBatchInput     usecase.GetAISettingsBatchInput
	getAISettingsBatchResult    *usecase.GetAISettingsBatchResult
	getAISettingsBatchErr       error
	deleteAISettingsBatchInput  usecase.DeleteAISettingsBatchInput
	deleteAISettingsBatchResult *usecase.DeleteAISettingsBatchResult
	deleteAISettingsBatchErr    error
	editAISettingInput          usecase.EditAISettingInput
	editAISettingResult         *usecase.EditAISettingResult
	editAISettingErr            error
}

func (s *stubWorkflow) Scan(_ context.Context, input usecase.ScanInput) (*usecase.ScanResult, error) {
	s.scanInput = input
	return s.scanResult, s.scanErr
}

func (s *stubWorkflow) AddCustomRule(_ context.Context, input usecase.AddCustomRuleInput) error {
	s.addRuleInput = input
	return s.addRuleErr
}

func (s *stubWorkflow) RemoveCustomRule(_ context.Context, input usecase.RemoveCustomRuleInput) error {
	s.removeRuleInput = input
	return s.removeRuleErr
}

func (s *stubWorkflow) AddProject(_ context.Context, input usecase.AddProjectInput) error {
	s.addProjectInput = input
	return s.addProjectErr
}

func (s *stubWorkflow) RemoveProject(_ context.Context, input usecase.RemoveProjectInput) error {
	s.removeProjectInput = input
	return s.removeProjectErr
}

func (s *stubWorkflow) ListScanRules(_ context.Context, input usecase.ListScanRulesInput) (*usecase.ListScanRulesResult, error) {
	s.listRulesInput = input
	return s.listRulesResult, s.listRulesErr
}

func (s *stubWorkflow) CreateSnapshot(_ context.Context, input usecase.CreateSnapshotInput) (*usecase.SnapshotSummary, error) {
	s.createInput = input
	return s.createResult, s.createErr
}

func (s *stubWorkflow) ListSnapshots(_ context.Context, input usecase.ListSnapshotsInput) (*usecase.ListSnapshotsResult, error) {
	s.listInput = input
	return s.listResult, s.listErr
}

func (s *stubWorkflow) DeleteSnapshot(_ context.Context, input usecase.DeleteSnapshotInput) error {
	s.deleteSnapshotInput = input
	return s.deleteSnapshotErr
}

func (s *stubWorkflow) RestoreSnapshot(_ context.Context, input usecase.RestoreSnapshotInput) (*typesSnapshot.RestoreResult, error) {
	return nil, nil
}

func (s *stubWorkflow) DiffSnapshots(_ context.Context, input usecase.DiffSnapshotsInput) (*typesSnapshot.DiffResult, error) {
	return nil, nil
}

func (s *stubWorkflow) GetConfig(_ context.Context, input usecase.GetConfigInput) (*usecase.GetConfigResult, error) {
	s.getInput = input
	return s.getResult, s.getErr
}

func (s *stubWorkflow) SaveConfig(_ context.Context, input usecase.SaveConfigInput) error {
	s.saveInput = input
	return s.saveErr
}

func (s *stubWorkflow) CreateAISetting(_ context.Context, input usecase.CreateAISettingInput) (*usecase.CreateAISettingResult, error) {
	s.createAISettingInput = input
	return s.createAISettingResult, s.createAISettingErr
}

func (s *stubWorkflow) ListAISettings(_ context.Context, input usecase.ListAISettingsInput) (*usecase.ListAISettingsResult, error) {
	s.listAISettingsInput = input
	return s.listAISettingsResult, s.listAISettingsErr
}

func (s *stubWorkflow) GetAISetting(_ context.Context, input usecase.GetAISettingInput) (*usecase.GetAISettingResult, error) {
	s.getAISettingInput = input
	return s.getAISettingResult, s.getAISettingErr
}

func (s *stubWorkflow) DeleteAISetting(_ context.Context, input usecase.DeleteAISettingInput) error {
	s.deleteAISettingInput = input
	return s.deleteAISettingErr
}

func (s *stubWorkflow) SwitchAISetting(_ context.Context, input usecase.SwitchAISettingInput) (*usecase.SwitchAISettingResult, error) {
	s.switchAISettingInput = input
	return s.switchAISettingResult, s.switchAISettingErr
}

func (s *stubWorkflow) GetAISettingsBatch(_ context.Context, input usecase.GetAISettingsBatchInput) (*usecase.GetAISettingsBatchResult, error) {
	s.getAISettingsBatchInput = input
	return s.getAISettingsBatchResult, s.getAISettingsBatchErr
}

func (s *stubWorkflow) DeleteAISettingsBatch(_ context.Context, input usecase.DeleteAISettingsBatchInput) (*usecase.DeleteAISettingsBatchResult, error) {
	s.deleteAISettingsBatchInput = input
	return s.deleteAISettingsBatchResult, s.deleteAISettingsBatchErr
}

func (s *stubWorkflow) EditAISetting(_ context.Context, input usecase.EditAISettingInput) (*usecase.EditAISettingResult, error) {
	s.editAISettingInput = input
	return s.editAISettingResult, s.editAISettingErr
}

func (s *stubWorkflow) AddRemote(_ context.Context, _ typesRemote.AddRemoteInput) (*typesRemote.AddRemoteResult, error) {
	return nil, nil
}

func (s *stubWorkflow) ListRemotes(_ context.Context) (*typesRemote.ListRemotesResult, error) {
	return nil, nil
}

func (s *stubWorkflow) RemoveRemote(_ context.Context, _ typesRemote.RemoveRemoteInput) (*typesRemote.ListRemotesResult, error) {
	return nil, nil
}

func (s *stubWorkflow) UpdateSnapshot(_ context.Context, _ usecase.UpdateSnapshotInput) (*typesSnapshot.UpdateSnapshotResult, error) {
	return nil, nil
}

func (s *stubWorkflow) SyncPush(_ context.Context, _ typesSync.SyncPushInput) (*typesSync.SyncPushResult, error) {
	return nil, nil
}

func (s *stubWorkflow) SyncPull(_ context.Context, _ typesSync.SyncPullInput) (*typesSync.SyncPullResult, error) {
	return nil, nil
}

func (s *stubWorkflow) SyncStatus(_ context.Context, _ typesSync.SyncStatusInput) (*typesSync.SyncStatusResult, error) {
	return nil, nil
}

func (s *stubWorkflow) SnapshotHistory(_ context.Context, _ usecase.SnapshotHistoryInput) (*typesSnapshot.HistoryResult, error) {
	return nil, nil
}

func (s *stubWorkflow) RestoreFromHistory(_ context.Context, _ usecase.RestoreFromHistoryInput) (*typesSnapshot.RestoreResult, error) {
	return nil, nil
}

type stubTUIRunner struct {
	called bool
	err    error
}

func (s *stubTUIRunner) Run(context.Context) error {
	s.called = true
	return s.err
}

type stubEditorRunner struct {
	called bool
	result *usecase.GetConfigResult
	err    error
}

func (s *stubEditorRunner) Run(_ context.Context, result *usecase.GetConfigResult, save func(string) error) error {
	s.called = true
	s.result = result
	if s.err != nil {
		return s.err
	}
	if save != nil {
		return save(result.Content)
	}
	return nil
}

func TestRootCommandRegistersSubcommands(t *testing.T) {
	cmd := NewRootCommand(Dependencies{
		Workflow: &stubWorkflow{},
		TUI:      &stubTUIRunner{},
		Out:      &bytes.Buffer{},
		Err:      &bytes.Buffer{},
	})

	commands := cmd.Commands()
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Name())
	}

	assertContains(t, names, "scan")
	assertContains(t, names, "snapshot")
	assertContains(t, names, "tui")
	assertContains(t, names, "get")
}

func TestExecuteScanCommandPrintsToolSummary(t *testing.T) {
	workflow := &stubWorkflow{
		scanResult: &usecase.ScanResult{
			Tools: []usecase.ToolSummary{
				{
					Tool:        "codex",
					Scope:       "global",
					ResultText:  "可同步",
					Path:        "/home/test/.codex",
					ConfigCount: 2,
					Items: []usecase.ToolConfigItem{
						{Group: "关键配置", Label: "主配置", RelativePath: "config.toml"},
						{Group: "扩展内容", Label: "技能目录", RelativePath: "skills"},
					},
				},
				{
					Tool:        "codex",
					Scope:       "project",
					ProjectName: "demo",
					ResultText:  "可同步",
					Path:        "/workspace/demo",
					ConfigCount: 2,
					Items: []usecase.ToolConfigItem{
						{Group: "关键配置", Label: "项目配置目录", RelativePath: ".codex/"},
						{Group: "关键配置", Label: "代理规则", RelativePath: "AGENTS.md"},
					},
				},
			},
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"scan"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if len(workflow.scanInput.Apps) != 0 {
		t.Fatalf("expected no app filters, got %+v", workflow.scanInput)
	}
	if !strings.Contains(stdout.String(), "Codex（全局）") ||
		!strings.Contains(stdout.String(), "demo（Codex 项目）") ||
		!strings.Contains(stdout.String(), "可同步") ||
		!strings.Contains(stdout.String(), "2 项") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}
}

func TestExecuteScanCommandPassesAppOrder(t *testing.T) {
	workflow := &stubWorkflow{
		scanResult: &usecase.ScanResult{
			Tools: []usecase.ToolSummary{
				{Tool: "claude", ResultText: "可同步"},
				{Tool: "codex", ResultText: "可同步"},
			},
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"scan", "claude", "codex"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if len(workflow.scanInput.Apps) != 2 || workflow.scanInput.Apps[0] != "claude" || workflow.scanInput.Apps[1] != "codex" {
		t.Fatalf("unexpected scan input order: %+v", workflow.scanInput)
	}
}

func TestExecuteScanAddProjectCommand(t *testing.T) {
	workflow := &stubWorkflow{}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"scan", "add", "--project", "codex", "demo", `D:\workspace\demo`})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", exitCode, stderr.String())
	}
	if workflow.addProjectInput.App != "codex" || workflow.addProjectInput.ProjectName != "demo" || workflow.addProjectInput.ProjectPath != `D:\workspace\demo` {
		t.Fatalf("unexpected add project input: %+v", workflow.addProjectInput)
	}
}

func TestExecuteScanListCommandPrintsScanResult(t *testing.T) {
	workflow := &stubWorkflow{
		scanResult: &usecase.ScanResult{
			Tools: []usecase.ToolSummary{
				{
					Tool:       "claude",
					Scope:      "global",
					ResultText: "可同步",
					Path:       "/home/test/.claude",
				},
			},
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"scan", "list", "claude"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if len(workflow.scanInput.Apps) != 1 || workflow.scanInput.Apps[0] != "claude" {
		t.Fatalf("unexpected scan input: %+v", workflow.scanInput)
	}
	if !strings.Contains(stdout.String(), "Claude（全局）") || !strings.Contains(stdout.String(), "可同步") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestExecuteScanRulesCommandPrintsRules(t *testing.T) {
	workflow := &stubWorkflow{
		listRulesResult: &usecase.ListScanRulesResult{
			App: "claude",
			DefaultGlobalRules: []usecase.RuleItem{
				{Path: "settings.json", Kind: "file"},
			},
			ProjectRuleTemplates: []usecase.RuleItem{
				{Path: ".claude", Kind: "dir"},
			},
			CustomRules: []usecase.RuleItem{
				{Path: `C:\Users\tester\.claude.json`, Kind: "file"},
			},
			RegisteredProjects: []usecase.RegisteredProjectItem{
				{Name: "demo", Path: `D:\workspace\demo`},
			},
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"scan", "rules", "claude"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if workflow.listRulesInput.App != "claude" {
		t.Fatalf("unexpected list input: %+v", workflow.listRulesInput)
	}
	if !strings.Contains(stdout.String(), "默认全局规则") ||
		!strings.Contains(stdout.String(), "settings.json") ||
		!strings.Contains(stdout.String(), "已注册项目扫描模板") ||
		!strings.Contains(stdout.String(), "已注册项目") ||
		!strings.Contains(stdout.String(), `D:\workspace\demo`) {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestExecuteScanRulesCommandHidesProjectTemplatesWithoutRegisteredProjects(t *testing.T) {
	workflow := &stubWorkflow{
		listRulesResult: &usecase.ListScanRulesResult{
			App: "codex",
			DefaultGlobalRules: []usecase.RuleItem{
				{Path: "config.toml", Kind: "file"},
			},
			ProjectRuleTemplates: []usecase.RuleItem{
				{Path: ".codex", Kind: "dir"},
				{Path: "AGENTS.md", Kind: "file"},
			},
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"scan", "rules", "codex"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if strings.Contains(stdout.String(), "项目规则模板") || strings.Contains(stdout.String(), "已注册项目扫描模板") {
		t.Fatalf("expected project template section to be hidden, got %s", stdout.String())
	}
}

func TestExecuteScanRulesCommandPassesRegisteredProjectName(t *testing.T) {
	workflow := &stubWorkflow{
		listRulesResult: &usecase.ListScanRulesResult{
			App: "codex",
			DefaultGlobalRules: []usecase.RuleItem{
				{Path: "config.toml", Kind: "file"},
			},
			ProjectRuleTemplates: []usecase.RuleItem{
				{Path: ".codex", Kind: "dir"},
				{Path: "AGENTS.md", Kind: "file"},
			},
			RegisteredProjects: []usecase.RegisteredProjectItem{
				{Name: "ai-sync-manager", Path: `D:\workspace\ai-sync-manager`},
			},
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"scan", "rules", "ai-sync-manager"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if workflow.listRulesInput.App != "ai-sync-manager" {
		t.Fatalf("unexpected list input: %+v", workflow.listRulesInput)
	}
	if !strings.Contains(stdout.String(), "Codex 规则") || !strings.Contains(stdout.String(), `D:\workspace\ai-sync-manager`) {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestExecuteScanCommandPrintsNotSyncableReason(t *testing.T) {
	workflow := &stubWorkflow{
		scanResult: &usecase.ScanResult{
			Tools: []usecase.ToolSummary{
				{
					Tool:       "claude",
					ResultText: "暂不可同步",
					Path:       "/home/test/.claude",
					Reason:     "找到了配置目录，但未识别到可同步的配置文件",
				},
			},
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"scan"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "Claude（全局）") || !strings.Contains(stdout.String(), "暂不可同步") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestExecuteSnapshotCreateParsesFlags(t *testing.T) {
	workflow := &stubWorkflow{
		createResult: &usecase.SnapshotSummary{
			ID:          0,
			Name:      "Daily backup",
			Message:   "before change",
			Project:   "codex-global",
			FileCount: 3,
			Size:      128,
			CreatedAt: time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC),
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{
		"snapshot", "create",
		"--tools", "codex,claude",
		"--message", "before change",
		"--name", "Daily backup",
		"--project", "codex-global",
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if workflow.createInput.ProjectName != "codex-global" || len(workflow.createInput.Tools) != 2 {
		t.Fatalf("unexpected create input: %+v", workflow.createInput)
	}
	if !strings.Contains(stdout.String(), "快照已创建: 0") || !strings.Contains(stdout.String(), "文件数: 3") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}
}

func TestExecuteGetDirectoryCommandPrintsEntries(t *testing.T) {
	workflow := &stubWorkflow{
		getResult: &usecase.GetConfigResult{
			Tool:         "codex",
			RelativePath: "skills",
			AbsolutePath: "/home/user/.codex/skills",
			Kind:         usecase.ConfigTargetDirectory,
			Entries: []usecase.ConfigEntry{
				{Name: "aiskills", RelativePath: "skills/aiskills", IsDir: true},
				{Name: "README.md", RelativePath: "skills/README.md", IsDir: false},
			},
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Editor:   &stubEditorRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"get", "codex", "skills"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "/home/user/.codex/skills") ||
		!strings.Contains(stdout.String(), "目录") ||
		!strings.Contains(stdout.String(), "aiskills") ||
		!strings.Contains(stdout.String(), "README.md") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestExecuteGetFileCommandPrintsContent(t *testing.T) {
	workflow := &stubWorkflow{
		getResult: &usecase.GetConfigResult{
			Tool:         "codex",
			RelativePath: "skills/README.md",
			AbsolutePath: "/home/user/.codex/skills/README.md",
			Kind:         usecase.ConfigTargetFile,
			Content:      "# hello",
			Editable:     true,
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Editor:   &stubEditorRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"get", "codex", "skills/README.md"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "/home/user/.codex/skills/README.md") || !strings.Contains(stdout.String(), "# hello") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestExecuteGetEditCommandInvokesEditor(t *testing.T) {
	workflow := &stubWorkflow{
		getResult: &usecase.GetConfigResult{
			Tool:         "codex",
			RelativePath: "skills/README.md",
			Kind:         usecase.ConfigTargetFile,
			Content:      "# hello",
			Editable:     true,
		},
	}
	editor := &stubEditorRunner{}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Editor:   editor,
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"get", "codex", "skills/README.md", "--edit"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !editor.called {
		t.Fatal("expected editor runner to be invoked")
	}
	if workflow.saveInput.Path != "skills/README.md" || workflow.saveInput.Content != "# hello" {
		t.Fatalf("expected save callback to use get result, got %+v", workflow.saveInput)
	}
}

func TestExecuteSnapshotListShowsEmptyState(t *testing.T) {
	workflow := &stubWorkflow{
		listResult: &usecase.ListSnapshotsResult{},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"snapshot", "list"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "暂无本地快照") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
	}
}

func TestExecuteCommandPrintsFriendlyError(t *testing.T) {
	workflow := &stubWorkflow{
		scanErr: &usecase.UserError{
			Message:    "未检测到任何可同步工具",
			Suggestion: "请先确认 ~/.codex 或 ~/.claude 是否存在",
			Err:        errors.New("not found"),
		},
	}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: workflow,
		TUI:      &stubTUIRunner{},
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"scan"})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	// 改造后输出应包含原始错误信息
	output := stderr.String()
	if !strings.Contains(output, "未检测到任何可同步工具: not found") {
		t.Fatalf("expected message with original error, got %q", output)
	}
	if !strings.Contains(output, "请先确认") {
		t.Fatalf("expected suggestion in output, got %q", output)
	}
}

func TestPrintError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		want   []string
		dont   []string
	}{
		{
			name: "UserError 有 Err 时追加原始错误",
			err: &usecase.UserError{
				Message: "创建快照失败",
				Err:     errors.New("no space left on device"),
			},
			want: []string{"创建快照失败: no space left on device"},
		},
		{
			name: "UserError 有 Err 有 Suggestion",
			err: &usecase.UserError{
				Message:    "扫描失败",
				Suggestion: "请确认目录存在",
				Err:        errors.New("permission denied"),
			},
			want: []string{"扫描失败: permission denied", "请确认目录存在"},
		},
		{
			name: "UserError 无 Err 时不追加冒号",
			err: &usecase.UserError{
				Message:    "创建快照失败：message 不能为空",
				Suggestion: "请通过 --message 输入",
			},
			want: []string{"创建快照失败：message 不能为空", "请通过 --message 输入"},
			dont: []string{": :"},
		},
		{
			name: "UserError Message 已包含 Err 时去重",
			err: &usecase.UserError{
				Message: "添加项目失败：项目名已存在",
				Err:     errors.New("项目名已存在"),
			},
			want: []string{"添加项目失败：项目名已存在"},
			dont: []string{"项目名已存在: 项目名已存在"},
		},
		{
			name: "非 UserError 直接输出 Error()",
			err:  errors.New("标准错误信息"),
			want: []string{"标准错误信息"},
		},
		{
			name: "UserError 无 Suggestion 只输出 Message",
			err: &usecase.UserError{
				Message: "删除快照失败",
				Err:     errors.New("database is locked"),
			},
			want: []string{"删除快照失败: database is locked"},
		},
		{
			name: "UserError 空 Suggestion 不输出空行",
			err: &usecase.UserError{
				Message:    "失败",
				Suggestion: "   ",
				Err:        errors.New("io timeout"),
			},
			want: []string{"失败: io timeout"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printError(&buf, tt.err)
			output := buf.String()

			for _, w := range tt.want {
				if !strings.Contains(output, w) {
					t.Errorf("expected output to contain %q, got %q", w, output)
				}
			}
			for _, d := range tt.dont {
				if strings.Contains(output, d) {
					t.Errorf("expected output NOT to contain %q, got %q", d, output)
				}
			}
		})
	}
}

func TestExecuteTUICommandInvokesRunner(t *testing.T) {
	runner := &stubTUIRunner{}

	var stdout, stderr bytes.Buffer
	exitCode := Execute(Dependencies{
		Workflow: &stubWorkflow{},
		TUI:      runner,
		Out:      &stdout,
		Err:      &stderr,
	}, []string{"tui"})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !runner.called {
		t.Fatal("expected tui runner to be invoked")
	}
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("expected %q in %v", want, values)
}
