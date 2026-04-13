package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"flux/internal/app/usecase"
)

func TestEditorModelInitializesWithContentAndLineNumbers(t *testing.T) {
	model := newEditorModel(&usecase.GetConfigResult{
		Tool:         "codex",
		RelativePath: "skills/README.md",
		Content:      "# hello\nworld",
	}, func(string) error { return nil })

	if !model.editor.ShowLineNumbers {
		t.Fatal("expected line numbers to be enabled")
	}
	if model.editor.Value() != "# hello\nworld" {
		t.Fatalf("unexpected editor value: %q", model.editor.Value())
	}
}

func TestEditorModelCtrlSSavesCurrentContent(t *testing.T) {
	var saved string

	model := newEditorModel(&usecase.GetConfigResult{
		Tool:         "codex",
		RelativePath: "skills/README.md",
		Content:      "# before",
	}, func(content string) error {
		saved = content
		return nil
	})
	model.editor.SetValue("# after")
	model.dirty = true

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	next := updated.(*EditorModel)

	if saved != "# after" {
		t.Fatalf("expected save callback to receive updated content, got %q", saved)
	}
	if next.dirty {
		t.Fatal("expected dirty state to be cleared after save")
	}
	if !strings.Contains(next.statusMessage, "已保存") {
		t.Fatalf("expected saved status, got %q", next.statusMessage)
	}
}

func TestEditorModelEscQuits(t *testing.T) {
	model := newEditorModel(&usecase.GetConfigResult{
		Tool:         "codex",
		RelativePath: "skills/README.md",
		Content:      "# hello",
	}, func(string) error { return nil })

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	next := updated.(*EditorModel)

	if !next.quitting {
		t.Fatal("expected editor to enter quitting state")
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestEditorModelWindowResizeAdjustsTextareaSize(t *testing.T) {
	model := newEditorModel(&usecase.GetConfigResult{
		Tool:         "codex",
		RelativePath: "skills/README.md",
		Content:      "# hello",
	}, func(string) error { return nil })

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	next := updated.(*EditorModel)

	if next.width != 120 || next.height != 40 {
		t.Fatalf("expected model size to update, got %dx%d", next.width, next.height)
	}
	if next.editor.Width() == 0 || next.editor.Height() == 0 {
		t.Fatalf("expected textarea size to update, got %dx%d", next.editor.Width(), next.editor.Height())
	}
}
