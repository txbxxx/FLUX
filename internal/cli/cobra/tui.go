package cobra

import (
	spcobra "github.com/spf13/cobra"
)

func newTUICommand(deps Dependencies) *spcobra.Command {
	return &spcobra.Command{
		Use:   "tui",
		Short: "启动终端交互界面",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			return deps.TUI.Run(cmd.Context())
		},
	}
}
