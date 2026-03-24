package cobra

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"ai-sync-manager/internal/app/usecase"
	"ai-sync-manager/internal/models"
)

type stubWorkflow struct {
	scanResult   *usecase.ScanResult
	scanErr      error
	createInput  usecase.CreateSnapshotInput
	createResult *usecase.SnapshotSummary
	createErr    error
	listInput    usecase.ListSnapshotsInput
	listResult   *usecase.ListSnapshotsResult
	listErr      error
	getInput     usecase.GetConfigInput
	getResult    *usecase.GetConfigResult
	getErr       error
	saveInput    usecase.SaveConfigInput
	saveErr      error
}

func (s *stubWorkflow) Scan(context.Context) (*usecase.ScanResult, error) {
	return s.scanResult, s.scanErr
}

func (s *stubWorkflow) CreateSnapshot(_ context.Context, input usecase.CreateSnapshotInput) (*usecase.SnapshotSummary, error) {
	s.createInput = input
	return s.createResult, s.createErr
}

func (s *stubWorkflow) ListSnapshots(_ context.Context, input usecase.ListSnapshotsInput) (*usecase.ListSnapshotsResult, error) {
	s.listInput = input
	return s.listResult, s.listErr
}

func (s *stubWorkflow) GetConfig(_ context.Context, input usecase.GetConfigInput) (*usecase.GetConfigResult, error) {
	s.getInput = input
	return s.getResult, s.getErr
}

func (s *stubWorkflow) SaveConfig(_ context.Context, input usecase.SaveConfigInput) error {
	s.saveInput = input
	return s.saveErr
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
					Tool:       "codex",
					ResultText: "可同步",
					Path:       "/home/test/.codex",
					ConfigCount: 2,
					Items: []usecase.ToolConfigItem{
						{Group: "关键配置", Label: "主配置", RelativePath: "config.toml"},
						{Group: "扩展内容", Label: "技能目录", RelativePath: "skills"},
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
	if !strings.Contains(stdout.String(), "Codex") ||
		!strings.Contains(stdout.String(), "检测结果: 可同步") ||
		!strings.Contains(stdout.String(), "主配置: config.toml") ||
		!strings.Contains(stdout.String(), "技能目录: skills") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %s", stderr.String())
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
	if !strings.Contains(stdout.String(), "Claude") || !strings.Contains(stdout.String(), "原因: 找到了配置目录，但未识别到可同步的配置文件") {
		t.Fatalf("unexpected stdout: %s", stdout.String())
	}
}

func TestExecuteSnapshotCreateParsesFlags(t *testing.T) {
	workflow := &stubWorkflow{
		createResult: &usecase.SnapshotSummary{
			ID:        "snap-1",
			Name:      "Daily backup",
			Message:   "before change",
			Tools:     []string{"codex", "claude"},
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
		"--scope", "global",
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if workflow.createInput.Scope != models.ScopeGlobal || len(workflow.createInput.Tools) != 2 {
		t.Fatalf("unexpected create input: %+v", workflow.createInput)
	}
	if !strings.Contains(stdout.String(), "snap-1") || !strings.Contains(stdout.String(), "files: 3") {
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
	if !strings.Contains(stdout.String(), "Codex > skills") ||
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
	if !strings.Contains(stdout.String(), "Codex > skills/README.md") || !strings.Contains(stdout.String(), "# hello") {
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
	if !strings.Contains(stderr.String(), "未检测到任何可同步工具") || !strings.Contains(stderr.String(), "请先确认") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
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
