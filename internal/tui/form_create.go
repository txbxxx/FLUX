package tui

import (
	"context"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"ai-sync-manager/internal/app/usecase"
	"ai-sync-manager/internal/models"
)

func newCreateForm() CreateForm {
	return CreateForm{
		Scope:       models.ScopeGlobal,
	}
}

func (m *Model) focusCreateForm() {}

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

func (m *Model) createSnapshotCmd() tea.Cmd {
	input := usecase.CreateSnapshotInput{
		Tools:       splitTools(m.Form.Tools),
		Message:     strings.TrimSpace(m.Form.Message),
		Name:        strings.TrimSpace(m.Form.Name),
		Scope:       m.Form.Scope,
		ProjectPath: strings.TrimSpace(m.Form.ProjectPath),
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
