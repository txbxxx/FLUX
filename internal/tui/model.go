package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"ai-sync-manager/internal/app/usecase"
)

type Workflow interface {
	Scan(ctx context.Context, input usecase.ScanInput) (*usecase.ScanResult, error)
	CreateSnapshot(ctx context.Context, input usecase.CreateSnapshotInput) (*usecase.SnapshotSummary, error)
	ListSnapshots(ctx context.Context, input usecase.ListSnapshotsInput) (*usecase.ListSnapshotsResult, error)
}

type Page string

const (
	PageHome      Page = "home"
	PageScan      Page = "scan"
	PageCreate    Page = "create"
	PageSnapshots Page = "snapshots"
)

const (
	formFocusTools = iota
	formFocusMessage
	formFocusName
	formFocusProjectName
	formFocusSubmit
)

var homeItems = []string{
	"扫描工具",
	"创建快照",
	"查看快照",
	"退出",
}

type CreateForm struct {
	Tools       string
	Message     string
	Name        string
	ProjectName string // 项目名称（必填）
}

type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Select key.Binding
	Back   key.Binding
	Quit   key.Binding
	Next   key.Binding
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Back, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select},
		{k.Next, k.Back, k.Quit},
	}
}

type Model struct {
	workflow      Workflow
	DataDir       string
	Page          Page
	Cursor        int
	Quitting      bool
	ErrorMessage  string
	StatusMessage string
	ScanResult    *usecase.ScanResult
	Snapshots     []usecase.SnapshotSummary
	Form          CreateForm
	FormFocus     int
	Help          help.Model
	Keys          KeyMap
}

type scanLoadedMsg struct {
	result *usecase.ScanResult
	err    error
}

type snapshotsLoadedMsg struct {
	result *usecase.ListSnapshotsResult
	err    error
}

type createSnapshotMsg struct {
	result *usecase.ListSnapshotsResult
	id     string
	err    error
}

func NewModel(workflow Workflow, dataDir string) *Model {
	model := &Model{
		workflow:  workflow,
		DataDir:   dataDir,
		Page:      PageHome,
		Form:      newCreateForm(),
		FormFocus: formFocusTools,
		Help:      help.New(),
		Keys: KeyMap{
			Up: key.NewBinding(
				key.WithKeys("up", "k"),
				key.WithHelp("↑/k", "上移"),
			),
			Down: key.NewBinding(
				key.WithKeys("down", "j"),
				key.WithHelp("↓/j", "下移"),
			),
			Select: key.NewBinding(
				key.WithKeys("enter"),
				key.WithHelp("enter", "确认"),
			),
			Back: key.NewBinding(
				key.WithKeys("esc"),
				key.WithHelp("esc", "返回"),
			),
			Quit: key.NewBinding(
				key.WithKeys("q", "ctrl+c"),
				key.WithHelp("q", "退出"),
			),
			Next: key.NewBinding(
				key.WithKeys("tab"),
				key.WithHelp("tab", "切换"),
			),
		},
	}
	model.focusCreateForm()
	return model
}

func (m *Model) Init() tea.Cmd {
	return nil
}
