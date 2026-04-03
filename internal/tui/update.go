package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"ai-sync-manager/internal/app/usecase"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKey(msg)
	case scanLoadedMsg:
		if msg.err != nil {
			m.setError(msg.err)
			return m, nil
		}
		m.ScanResult = msg.result
		m.Page = PageScan
		m.ErrorMessage = ""
		return m, nil
	case snapshotsLoadedMsg:
		if msg.err != nil {
			m.setError(msg.err)
			return m, nil
		}
		if msg.result != nil {
			m.Snapshots = msg.result.Items
		} else {
			m.Snapshots = nil
		}
		m.Page = PageSnapshots
		m.ErrorMessage = ""
		return m, nil
	case createSnapshotMsg:
		if msg.err != nil {
			m.setError(msg.err)
			return m, nil
		}
		if msg.result != nil {
			m.Snapshots = msg.result.Items
		} else {
			m.Snapshots = nil
		}
		m.Page = PageSnapshots
		m.ErrorMessage = ""
		m.StatusMessage = "快照已创建 (ID: " + msg.id + ")"
		return m, nil
	}

	return m, nil
}

func (m *Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.Page {
	case PageHome:
		return m.updateHomeKey(msg)
	case PageCreate:
		return m.updateCreateKey(msg)
	case PageScan, PageSnapshots:
		return m.updateDetailKey(msg)
	default:
		return m, nil
	}
}

func (m *Model) updateHomeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.Quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
		}
	case "down", "j":
		if m.Cursor < len(homeItems)-1 {
			m.Cursor++
		}
	case "enter":
		switch m.Cursor {
		case 0:
			return m, m.loadScanCmd()
		case 1:
			m.Page = PageCreate
			m.ErrorMessage = ""
			m.StatusMessage = ""
			m.FormFocus = formFocusTools
			m.focusCreateForm()
		case 2:
			return m, m.loadSnapshotsCmd()
		case 3:
			m.Quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *Model) updateDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "enter":
		m.Page = PageHome
		m.ErrorMessage = ""
		m.StatusMessage = ""
	}

	return m, nil
}

func (m *Model) updateCreateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.Page = PageHome
		m.ErrorMessage = ""
		m.StatusMessage = ""
		m.focusCreateForm()
		return m, nil
	case "tab", "down":
		m.nextFormFocus()
		return m, nil
	case "shift+tab", "up":
		m.previousFormFocus()
		return m, nil
	case "enter":
		if m.FormFocus == formFocusSubmit {
			return m, m.createSnapshotCmd()
		}
		m.nextFormFocus()
		return m, nil
	}

	var cmd tea.Cmd
	switch m.FormFocus {
	case formFocusTools:
		m.Form.Tools = m.updateFocusedField(m.Form.Tools, msg)
	case formFocusMessage:
		m.Form.Message = m.updateFocusedField(m.Form.Message, msg)
	case formFocusName:
		m.Form.Name = m.updateFocusedField(m.Form.Name, msg)
	case formFocusProjectPath:
		m.Form.ProjectPath = m.updateFocusedField(m.Form.ProjectPath, msg)
	}

	return m, cmd
}

func (m *Model) loadScanCmd() tea.Cmd {
	return func() tea.Msg {
		result, err := m.workflow.Scan(context.Background(), usecase.ScanInput{})
		return scanLoadedMsg{result: result, err: err}
	}
}

func (m *Model) loadSnapshotsCmd() tea.Cmd {
	return func() tea.Msg {
		result, err := m.workflow.ListSnapshots(context.Background(), usecase.ListSnapshotsInput{})
		return snapshotsLoadedMsg{result: result, err: err}
	}
}

func (m *Model) setError(err error) {
	m.ErrorMessage = err.Error()
}
