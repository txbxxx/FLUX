package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"flux/internal/app/usecase"
)

func newCreateForm() CreateForm {
	return CreateForm{
		// ProjectName 由用户输入，无默认值
	}
}

// focusCreateForm 当前表单使用纯文本输入，暂不需要额外焦点同步逻辑。
func (m *Model) focusCreateForm() {}

// nextFormFocus / previousFormFocus 在固定字段集合中循环切换焦点。
func (m *Model) nextFormFocus() {
	m.FormFocus++
	if m.FormFocus > formFocusSubmit {
		m.FormFocus = formFocusTools
	}
	m.focusCreateForm()
}

func (m *Model) previousFormFocus() {
	m.FormFocus--
	if m.FormFocus < formFocusTools {
		m.FormFocus = formFocusSubmit
	}
	m.focusCreateForm()
}

// createSnapshotCmd 先创建快照，再顺手刷新列表页数据。
func (m *Model) createSnapshotCmd() tea.Cmd {
	input := usecase.CreateSnapshotInput{
		Tools:       splitTools(m.Form.Tools),
		Message:     strings.TrimSpace(m.Form.Message),
		Name:        strings.TrimSpace(m.Form.Name),
		ProjectName: strings.TrimSpace(m.Form.ProjectName),
	}

	return func() tea.Msg {
		result, err := m.workflow.CreateSnapshot(context.Background(), input)
		if err != nil {
			return createSnapshotMsg{err: err}
		}

		list, err := m.workflow.ListSnapshots(context.Background(), usecase.ListSnapshotsInput{})
		if err != nil {
			return createSnapshotMsg{err: err}
		}

		return createSnapshotMsg{
			result: list,
			id:     result.ID,
		}
	}
}

// splitTools 把逗号分隔输入转换成去空白后的工具列表。
func splitTools(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}

	return items
}

// updateFocusedField 只处理当前简化表单需要的退格和普通字符输入。
func (m *Model) updateFocusedField(value string, msg tea.KeyMsg) string {
	switch msg.Type {
	case tea.KeyBackspace:
		if len(value) == 0 {
			return value
		}
		return value[:len(value)-1]
	case tea.KeyRunes:
		return value + string(msg.Runes)
	default:
		return value
	}
}
