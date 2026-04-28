package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"flux/internal/app/usecase"
)

// SettingEditorKeyMap 定义 setting 编辑器的按键绑定。
type SettingEditorKeyMap struct {
	Save     key.Binding
	Quit     key.Binding
	NextField key.Binding
	PrevField key.Binding
}

func (k SettingEditorKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Save, k.Quit, k.NextField, k.PrevField}
}

func (k SettingEditorKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Save, k.Quit}, {k.NextField, k.PrevField}}
}

// SettingEditorModel setting 编辑器模型。
type SettingEditorModel struct {
	original   *usecase.GetAISettingResult
	inputs     []*textinput.Model // 字段输入框列表
	labels     []string            // 字段标签
	help       help.Model
	keys       SettingEditorKeyMap
	save       func(*usecase.EditAISettingInput) error
	statusMsg  string
	width      int
	height     int
	focusIndex int
	quitting   bool
}

// newSettingEditorModel 创建 setting 编辑器模型。
func newSettingEditorModel(original *usecase.GetAISettingResult, save func(*usecase.EditAISettingInput) error) *SettingEditorModel {
	var inputs []*textinput.Model
	labels := []string{"配置名称", "Token", "API 地址", "模型列表"}
	placeholders := []string{"配置名称", "Token（<unchanged> 保持原值）", "API 地址", "模型列表"}

	values := []string{
		original.Name,
		maskTokenForEditor(original.Token),
		original.BaseURL,
		strings.Join(original.Models, ", "),
	}

	for i := 0; i < 4; i++ {
		ti := textinput.New()
		ti.Placeholder = placeholders[i]
		ti.SetValue(values[i])
		ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
		ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		if i == 0 {
			ti.Focus()
		}
		inputs = append(inputs, &ti)
	}

	return &SettingEditorModel{
		original:   original,
		inputs:     inputs,
		labels:     labels,
		help:       help.New(),
		keys: SettingEditorKeyMap{
			Save: key.NewBinding(
				key.WithKeys("ctrl+s"),
				key.WithHelp("ctrl+s", "保存"),
			),
			Quit: key.NewBinding(
				key.WithKeys("esc", "ctrl+c"),
				key.WithHelp("esc", "退出"),
			),
			NextField: key.NewBinding(
				key.WithKeys("tab", "down"),
				key.WithHelp("tab/↓", "下一字段"),
			),
			PrevField: key.NewBinding(
				key.WithKeys("shift+tab", "up"),
				key.WithHelp("shift+tab/↑", "上一字段"),
			),
		},
		save:      save,
		statusMsg: "编辑中（Ctrl+S 保存 / Esc 退出）",
		width:     80,
		height:    24,
	}
}

// Init 初始化模型。
func (m *SettingEditorModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update 处理消息。
func (m *SettingEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlS:
			return m.saveChanges()

		case tea.KeyEsc, tea.KeyCtrlC:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyTab, tea.KeyDown:
			m.focusNext()
			return m, nil

		case tea.KeyShiftTab, tea.KeyUp:
			m.focusPrev()
			return m, nil
		}

		// 将按键传递给当前聚焦的输入框
		if m.focusIndex >= 0 && m.focusIndex < len(m.inputs) {
			var cmd tea.Cmd
			var model textinput.Model
			model, cmd = (*m.inputs[m.focusIndex]).Update(msg)
			m.inputs[m.focusIndex] = &model
			return m, cmd
		}
	}

	return m, nil
}

// View 渲染界面。
func (m *SettingEditorModel) View() string {
	if m.quitting {
		return ""
	}

	title := lipgloss.NewStyle().Bold(true).Render("AI 配置编辑器")
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(m.statusMsg)

	// 构建表单
	var form strings.Builder
	form.WriteString(title + "\n\n")
	form.WriteString(status + "\n\n")

	// 配置信息提示
	info := lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render(
		fmt.Sprintf("配置名称: %s | 当前生效: %v", m.original.Name, m.original.IsCurrent),
	)
	form.WriteString(info + "\n\n")

	// 字段编辑区
	for i, label := range m.labels {
		labelStyled := lipgloss.NewStyle().Width(15).Render(label + ":")
		form.WriteString(fmt.Sprintf("%s%s\n", labelStyled, (*m.inputs[i]).View()))
	}

	form.WriteString("\n")
	form.WriteString(m.help.View(m.keys))

	// 居中显示
	content := form.String()
	width := max(60, min(m.width-4, 80))
	return lipgloss.NewStyle().Width(width).Render(content)
}

// focusNext 聚焦下一个输入框。
func (m *SettingEditorModel) focusNext() {
	if len(m.inputs) == 0 {
		return
	}

	// Blur 当前
	if m.focusIndex >= 0 && m.focusIndex < len(m.inputs) {
		(*m.inputs[m.focusIndex]).Blur()
	}

	// Focus 下一个
	m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
	(*m.inputs[m.focusIndex]).Focus()
}

// focusPrev 聚焦上一个输入框。
func (m *SettingEditorModel) focusPrev() {
	if len(m.inputs) == 0 {
		return
	}

	// Blur 当前
	if m.focusIndex >= 0 && m.focusIndex < len(m.inputs) {
		(*m.inputs[m.focusIndex]).Blur()
	}

	// Focus 上一个
	m.focusIndex--
	if m.focusIndex < 0 {
		m.focusIndex = len(m.inputs) - 1
	}
	(*m.inputs[m.focusIndex]).Focus()
}

// saveChanges 保存更改。
func (m *SettingEditorModel) saveChanges() (tea.Model, tea.Cmd) {
	// 获取token输入框的值
	tokenValue := (*m.inputs[1]).Value()

	// 如果 token 值与脱敏显示值匹配或是占位符，说明用户没有修改，使用原始完整token
	// 为什么：maskTokenForEditor 会将token脱敏显示（如 4e5b****QEXh），如果用户没有修改该字段，
	// 直接保存脱敏值会导致token失效。因此需要检测并还原原始值。
	if tokenValue == maskTokenForEditor(m.original.Token) || tokenValue == "<unchanged>" {
		tokenValue = m.original.Token
	}

	// 解析模型列表
	var models []string
	if modelsStr := (*m.inputs[3]).Value(); modelsStr != "" {
		models = strings.Split(modelsStr, ",")
		for i := range models {
			models[i] = strings.TrimSpace(models[i])
		}
	}

	input := &usecase.EditAISettingInput{
		Name:    m.original.Name,
		NewName: (*m.inputs[0]).Value(),
		Token:   tokenValue,
		BaseURL: (*m.inputs[2]).Value(),
		Models:  models,
	}

	if m.save != nil {
		if err := m.save(input); err != nil {
			m.statusMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("保存失败: " + err.Error())
			return m, nil
		}
	}

	m.statusMsg = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Render("已保存")
	m.quitting = true
	return m, tea.Quit
}

// maskTokenForEditor 脱敏显示 token（编辑器模式）。
func maskTokenForEditor(token string) string {
	if token == "" {
		return "<unchanged>"
	}
	if len(token) > 8 {
		return token[:4] + "****" + token[len(token)-4:]
	}
	return "****"
}
