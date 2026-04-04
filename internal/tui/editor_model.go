package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ai-sync-manager/internal/app/usecase"
)

type editorKeyMap struct {
	Save key.Binding
	Quit key.Binding
}

func (k editorKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Save, k.Quit}
}

func (k editorKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Save, k.Quit}}
}

// editorBuffer 用极简行缓冲实现终端编辑，不依赖外部编辑器组件。
type editorBuffer struct {
	ShowLineNumbers bool
	lines           []string
	width           int
	height          int
	lineEnding      string // 保留原始换行符风格（\r\n 或 \n）
}

// newEditorBuffer 创建带默认尺寸的编辑缓冲。
func newEditorBuffer(content string) editorBuffer {
	buffer := editorBuffer{
		ShowLineNumbers: true,
		width:           80,
		height:          20,
	}
	buffer.SetValue(content)
	return buffer
}

// SetValue 会先统一换行符，再拆成按行编辑的内部结构。
func (b *editorBuffer) SetValue(content string) {
	b.lineEnding = "\n"
	if strings.Contains(content, "\r\n") {
		b.lineEnding = "\r\n"
	}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	b.lines = strings.Split(normalized, "\n")
	if len(b.lines) == 0 {
		b.lines = []string{""}
	}
}

// Value 把当前缓冲重新拼成完整文本，保留原始换行符风格。
func (b editorBuffer) Value() string {
	return strings.Join(b.lines, b.lineEnding)
}

func (b *editorBuffer) SetWidth(width int) {
	b.width = max(1, width)
}

func (b *editorBuffer) SetHeight(height int) {
	b.height = max(1, height)
}

func (b editorBuffer) Width() int {
	return b.width
}

func (b editorBuffer) Height() int {
	return b.height
}

// EditorModel 保存终端编辑器的全部状态。
type EditorModel struct {
	result          *usecase.GetConfigResult
	editor          editorBuffer
	help            help.Model
	keys            editorKeyMap
	save            func(string) error
	originalContent string
	statusMessage   string
	width           int
	height          int
	dirty           bool
	quitting        bool
	cursorRow       int
	cursorCol       int
	scrollOffset    int
}

// newEditorModel 根据配置读取结果创建编辑器状态。
func newEditorModel(result *usecase.GetConfigResult, save func(string) error) *EditorModel {
	buffer := newEditorBuffer(result.Content)

	return &EditorModel{
		result: result,
		editor: buffer,
		help:   help.New(),
		keys: editorKeyMap{
			Save: key.NewBinding(
				key.WithKeys("ctrl+s"),
				key.WithHelp("ctrl+s", "保存"),
			),
			Quit: key.NewBinding(
				key.WithKeys("esc", "ctrl+c"),
				key.WithHelp("esc", "退出"),
			),
		},
		save:            save,
		originalContent: result.Content,
		statusMessage:   "编辑中（Ctrl+S 保存 / Esc 退出）",
		width:           80,
		height:          24,
	}
}

// Init 当前没有预加载命令。
func (m *EditorModel) Init() tea.Cmd {
	return nil
}

// Update 处理窗口尺寸变化、保存、退出和基础文本编辑行为。
func (m *EditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeEditor()
		m.ensureCursorInBounds()
		m.ensureViewport()
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlS:
			if m.save != nil {
				if err := m.save(m.editor.Value()); err != nil {
					m.statusMessage = "保存失败: " + err.Error()
					return m, nil
				}
			}
			m.originalContent = m.editor.Value()
			m.dirty = false
			m.statusMessage = "已保存"
			return m, nil
		case tea.KeyEsc, tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit
		case tea.KeyUp:
			if m.cursorRow > 0 {
				m.cursorRow--
			}
		case tea.KeyDown:
			if m.cursorRow < len(m.editor.lines)-1 {
				m.cursorRow++
			}
		case tea.KeyLeft:
			if m.cursorCol > 0 {
				m.cursorCol--
			} else if m.cursorRow > 0 {
				m.cursorRow--
				m.cursorCol = len(m.editor.lines[m.cursorRow])
			}
		case tea.KeyRight:
			lineLen := len(m.editor.lines[m.cursorRow])
			if m.cursorCol < lineLen {
				m.cursorCol++
			} else if m.cursorRow < len(m.editor.lines)-1 {
				m.cursorRow++
				m.cursorCol = 0
			}
		case tea.KeyHome:
			m.cursorCol = 0
		case tea.KeyEnd:
			m.cursorCol = len(m.editor.lines[m.cursorRow])
		case tea.KeyPgUp:
			m.cursorRow = max(0, m.cursorRow-m.editor.Height())
		case tea.KeyPgDown:
			m.cursorRow = min(len(m.editor.lines)-1, m.cursorRow+m.editor.Height())
		case tea.KeyEnter:
			m.insertNewLine()
		case tea.KeyBackspace, tea.KeyCtrlH:
			m.deleteBackward()
		case tea.KeyRunes:
			m.insertRunes(string(msg.Runes))
		}
	}

	m.ensureCursorInBounds()
	m.ensureViewport()
	m.updateDirtyState()
	return m, nil
}

// View 渲染标题、状态栏、编辑区和帮助信息。
func (m *EditorModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("%s > %s", displayEditorToolName(m.result.Tool), m.result.RelativePath),
	)
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.statusMessage)
	helpView := m.help.View(m.keys)

	return strings.Join([]string{
		title,
		status,
		"",
		m.renderBuffer(),
		helpView,
	}, "\n")
}

// renderBuffer 只渲染当前视口内的内容，避免大文件时整屏重排。
func (m *EditorModel) renderBuffer() string {
	var lines []string

	start := min(m.scrollOffset, len(m.editor.lines))
	end := min(len(m.editor.lines), start+m.editor.Height())
	for index := start; index < end; index++ {
		line := m.editor.lines[index]
		if index == m.cursorRow {
			line = insertCursor(line, m.cursorCol)
		}
		if m.editor.ShowLineNumbers {
			lines = append(lines, fmt.Sprintf("%4d %s", index+1, line))
		} else {
			lines = append(lines, line)
		}
	}

	for len(lines) < m.editor.Height() {
		if m.editor.ShowLineNumbers {
			lines = append(lines, "     ")
		} else {
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n")
}

// resizeEditor 根据终端窗口尺寸调整可视区大小。
func (m *EditorModel) resizeEditor() {
	editorWidth := max(20, m.width-4)
	editorHeight := max(5, m.height-6)
	m.editor.SetWidth(editorWidth)
	m.editor.SetHeight(editorHeight)
}

// insertRunes 在当前光标位置插入文本。
func (m *EditorModel) insertRunes(value string) {
	line := m.editor.lines[m.cursorRow]
	if m.cursorCol > len(line) {
		m.cursorCol = len(line)
	}
	m.editor.lines[m.cursorRow] = line[:m.cursorCol] + value + line[m.cursorCol:]
	m.cursorCol += len(value)
}

// insertNewLine 把当前行拆成两行，模拟常规编辑器的回车行为。
func (m *EditorModel) insertNewLine() {
	line := m.editor.lines[m.cursorRow]
	head := line[:m.cursorCol]
	tail := line[m.cursorCol:]
	m.editor.lines[m.cursorRow] = head
	m.editor.lines = append(m.editor.lines[:m.cursorRow+1], append([]string{tail}, m.editor.lines[m.cursorRow+1:]...)...)
	m.cursorRow++
	m.cursorCol = 0
}

// deleteBackward 实现退格：优先删当前字符，行首时与上一行合并。
func (m *EditorModel) deleteBackward() {
	if m.cursorCol > 0 {
		line := m.editor.lines[m.cursorRow]
		m.editor.lines[m.cursorRow] = line[:m.cursorCol-1] + line[m.cursorCol:]
		m.cursorCol--
		return
	}
	if m.cursorRow == 0 {
		return
	}

	previous := m.editor.lines[m.cursorRow-1]
	current := m.editor.lines[m.cursorRow]
	m.cursorCol = len(previous)
	m.editor.lines[m.cursorRow-1] = previous + current
	m.editor.lines = append(m.editor.lines[:m.cursorRow], m.editor.lines[m.cursorRow+1:]...)
	m.cursorRow--
}

// ensureCursorInBounds 防止光标因窗口变化或编辑操作越界。
func (m *EditorModel) ensureCursorInBounds() {
	if len(m.editor.lines) == 0 {
		m.editor.lines = []string{""}
	}
	m.cursorRow = clamp(m.cursorRow, 0, len(m.editor.lines)-1)
	m.cursorCol = clamp(m.cursorCol, 0, len(m.editor.lines[m.cursorRow]))
}

// ensureViewport 保证光标始终落在当前可视区域内。
func (m *EditorModel) ensureViewport() {
	height := m.editor.Height()
	if m.cursorRow < m.scrollOffset {
		m.scrollOffset = m.cursorRow
		return
	}
	if m.cursorRow >= m.scrollOffset+height {
		m.scrollOffset = m.cursorRow - height + 1
		return
	}
	if m.scrollOffset > max(0, len(m.editor.lines)-height) {
		m.scrollOffset = max(0, len(m.editor.lines)-height)
	}
}

// updateDirtyState 根据当前内容和原始内容比较来更新脏状态。
func (m *EditorModel) updateDirtyState() {
	m.dirty = m.editor.Value() != m.originalContent
	if m.dirty {
		m.statusMessage = "已修改（未保存）"
	}
}

// insertCursor 用可见字符标记光标位置，保持纯文本渲染简单可控。
func insertCursor(line string, col int) string {
	col = clamp(col, 0, len(line))
	return line[:col] + "|" + line[col:]
}

// displayEditorToolName 统一编辑器标题中的工具展示名。
func displayEditorToolName(name string) string {
	switch name {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	default:
		return strings.Title(name)
	}
}

// clamp 用于光标和滚动范围约束。
func clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}
