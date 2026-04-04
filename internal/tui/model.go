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

// CreateForm 是快照创建表单的数据模型。
// 各字段对应表单中可编辑的输入项，由 FormFocus 控制当前聚焦。
type CreateForm struct {
	Tools       string // 要备份的工具列表，逗号分隔（如 "codex,claude"），空则从项目名自动推导
	Message     string // 快照说明（必填）
	Name        string // 快照名称（可选，空则自动生成）
	ProjectName string // 项目名称（必填，如 claude-global 或用户注册的项目名）
}

// KeyMap 统一管理 TUI 全局按键绑定。
// 实现 help.KeyMap 接口，用于在页面底部显示快捷键提示。
type KeyMap struct {
	Up     key.Binding // 上移光标
	Down   key.Binding // 下移光标
	Select key.Binding // 确认/选中当前项
	Back   key.Binding // 返回上一页
	Quit   key.Binding // 退出程序
	Next   key.Binding // 在表单中切换到下一个输入项
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

// Model 是 Bubbletea TUI 应用的顶层状态模型。
// 包含当前页面、光标位置、表单数据、异步消息等所有 TUI 需要的状态。
type Model struct {
	workflow      Workflow               // 用例层接口，屏蔽业务实现细节
	DataDir       string                 // 应用数据目录路径（存放数据库、配置等）
	Page          Page                   // 当前页面（home / scan / create / snapshots）
	Cursor        int                    // 当前光标位置（列表页表示选中行，表单页表示聚焦项）
	Quitting      bool                   // 是否正在退出
	ErrorMessage  string                 // 待展示的错误信息（非空时在页面顶部显示）
	StatusMessage string                 // 待展示的状态信息（如"快照已创建"）
	ScanResult    *usecase.ScanResult    // 最近一次扫描结果（scan 页面数据源）
	Snapshots     []usecase.SnapshotSummary // 快照列表（snapshots 页面数据源）
	Form          CreateForm             // 快照创建表单数据
	FormFocus     int                    // 当前表单焦点项索引（对应 formFocus 常量）
	Help          help.Model             // Bubbletea help 组件，用于底部快捷键提示
	Keys          KeyMap                 // 全局快捷键绑定
}

// scanLoadedMsg 是异步扫描完成后的 Bubbletea 消息。
type scanLoadedMsg struct {
	result *usecase.ScanResult // 扫描结果，nil 表示扫描失败
	err    error               // 扫描错误，nil 表示成功
}

// snapshotsLoadedMsg 是异步快照列表加载完成后的 Bubbletea 消息。
type snapshotsLoadedMsg struct {
	result *usecase.ListSnapshotsResult // 快照列表查询结果
	err    error                        // 查询错误
}

// createSnapshotMsg 是异步快照创建完成后的 Bubbletea 消息。
type createSnapshotMsg struct {
	result *usecase.ListSnapshotsResult // 创建后刷新的快照列表（用于更新页面）
	id     string                       // 创建成功的快照 ID
	err    error                        // 创建错误
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
