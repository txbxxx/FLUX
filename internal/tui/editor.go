package tui

import (
	"context"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ai-sync-manager/internal/app/usecase"
)

type ConfigEditorRunner struct {
	in  io.Reader
	out io.Writer
}

func NewConfigEditor(in io.Reader, out io.Writer) *ConfigEditorRunner {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = io.Discard
	}

	return &ConfigEditorRunner{
		in:  in,
		out: out,
	}
}

func (r *ConfigEditorRunner) Run(_ context.Context, result *usecase.GetConfigResult, save func(string) error) error {
	model := newEditorModel(result, save)
	program := tea.NewProgram(
		model,
		tea.WithInput(r.in),
		tea.WithOutput(r.out),
		tea.WithAltScreen(),
	)

	_, err := program.Run()
	return err
}
