package cobra

import (
	"fmt"

	spcobra "github.com/spf13/cobra"
)

// newSnapshotCommand 组装 snapshot 命令树。
func newSnapshotCommand(deps Dependencies) *spcobra.Command {
	command := &spcobra.Command{
		Use:   "snapshot",
		Short: "创建或查看本地快照",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "请指定 snapshot 操作，例如: ai-sync snapshot create")
			return errCommandHandled
		},
	}

	command.AddCommand(
		newSnapshotCreateCommand(deps),
		newSnapshotListCommand(deps),
	)

	return command
}
