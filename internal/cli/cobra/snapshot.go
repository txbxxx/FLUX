package cobra

import (
	"fmt"

	spcobra "github.com/spf13/cobra"
)

func newSnapshotCommand(deps Dependencies) *spcobra.Command {
	command := &spcobra.Command{
		Use:   "snapshot",
		Short: "创建或查看本地快照",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "缺少 snapshot 子命令")
			return errCommandHandled
		},
	}

	command.AddCommand(
		newSnapshotCreateCommand(deps),
		newSnapshotListCommand(deps),
	)

	return command
}
