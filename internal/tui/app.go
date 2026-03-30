package tui

import (
	"context"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type Runner struct {
	model *Model
	in    io.Reader
	out   io.Writer
}

// NewRunner 创建 TUI 运行器，并处理输入输出的默认值。
func NewRunner(workflow Workflow, dataDir string, in io.Reader, out io.Writer) *Runner {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = io.Discard
	}

	return &Runner{
		model: NewModel(workflow, dataDir),
		in:    in,
		out:   out,
	}
}

// Run 启动 Bubble Tea 程序，并在退出后回收最终 model 状态。
func (r *Runner) Run(_ context.Context) error {
	program := tea.NewProgram(
		r.model,
		tea.WithInput(r.in),
		tea.WithOutput(r.out),
	)

	finalModel, err := program.Run()
	if err != nil {
		return err
	}

	if model, ok := finalModel.(*Model); ok {
		r.model = model
	}

	return nil
}
