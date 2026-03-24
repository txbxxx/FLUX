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

type editorBuffer struct {
	ShowLineNumbers bool
	lines           []string
	width           int
	height          int
}

func newEditorBuffer(content string) editorBuffer {
	buffer := editorBuffer{
		ShowLineNumbers: true,
		width:           80,
		height:          20,
	}
	buffer.SetValue(content)
	return buffer
}

func (b *editorBuffer) SetValue(content string) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	b.lines = strings.Split(normalized, "\n")
	if len(b.lines) == 0 {
		b.lines = []string{""}
	}
}

func (b editorBuffer) Value() string {
	return strings.Join(b.lines, "\n")
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
		statusMessage:   "编辑模式",
		width:           80,
		height:          24,
	}
}

func (m *EditorModel) Init() tea.Cmd {
	return nil
}

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

func (m *EditorModel) resizeEditor() {
	editorWidth := max(20, m.width-4)
	editorHeight := max(5, m.height-6)
	m.editor.SetWidth(editorWidth)
	m.editor.SetHeight(editorHeight)
}

func (m *EditorModel) insertRunes(value string) {
	line := m.editor.lines[m.cursorRow]
	if m.cursorCol > len(line) {
		m.cursorCol = len(line)
	}
	m.editor.lines[m.cursorRow] = line[:m.cursorCol] + value + line[m.cursorCol:]
	m.cursorCol += len(value)
}

func (m *EditorModel) insertNewLine() {
	line := m.editor.lines[m.cursorRow]
	head := line[:m.cursorCol]
	tail := line[m.cursorCol:]
	m.editor.lines[m.cursorRow] = head
	m.editor.lines = append(m.editor.lines[:m.cursorRow+1], append([]string{tail}, m.editor.lines[m.cursorRow+1:]...)...)
	m.cursorRow++
	m.cursorCol = 0
}

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

func (m *EditorModel) ensureCursorInBounds() {
	if len(m.editor.lines) == 0 {
		m.editor.lines = []string{""}
	}
	m.cursorRow = clamp(m.cursorRow, 0, len(m.editor.lines)-1)
	m.cursorCol = clamp(m.cursorCol, 0, len(m.editor.lines[m.cursorRow]))
}

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

func (m *EditorModel) updateDirtyState() {
	m.dirty = m.editor.Value() != m.originalContent
	if m.dirty {
		m.statusMessage = "已修改"
	}
}

func insertCursor(line string, col int) string {
	col = clamp(col, 0, len(line))
	return line[:col] + "|" + line[col:]
}

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

func clamp(v, low, high int) int {
	if high < low {
		low, high = high, low
	}
	return min(high, max(low, v))
}
