package tui

import (
	"context"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ai-sync-manager/internal/app/usecase"
)

// SettingEditorRunner setting 编辑器运行器。
type SettingEditorRunner struct {
	in  io.Reader
	out io.Writer
}

// NewSettingEditor 创建 setting 编辑器运行器。
func NewSettingEditor(in io.Reader, out io.Writer) *SettingEditorRunner {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}

	return &SettingEditorRunner{
		in:  in,
		out: out,
	}
}

// Run 启动 setting 编辑器。
func (r *SettingEditorRunner) Run(_ context.Context, original *usecase.GetAISettingResult, save func(*usecase.EditAISettingInput) error) error {
	model := newSettingEditorModel(original, save)
	program := tea.NewProgram(
		model,
		tea.WithInput(r.in),
		tea.WithOutput(r.out),
		tea.WithAltScreen(),
	)

	_, err := program.Run()
	return err
}
