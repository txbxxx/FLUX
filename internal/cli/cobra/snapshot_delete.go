package cobra

import (
	"fmt"

	spcobra "github.com/spf13/cobra"

	"flux/internal/app/usecase"
)

// newSnapshotDeleteCommand 创建 snapshot delete 子命令。
// 支持通过快照 ID（UUID）或名称删除快照。
func newSnapshotDeleteCommand(deps Dependencies) *spcobra.Command {
	command := &spcobra.Command{
		Use:   "delete <id-or-name>",
		Short: "删除本地快照",
		Args:  validateExactOneArg("ai-sync snapshot delete <id-or-name>"),
		RunE: func(cmd *spcobra.Command, args []string) error {
			err := deps.Workflow.DeleteSnapshot(cmd.Context(), usecase.DeleteSnapshotInput{
				IDOrName: args[0],
			})
			if err != nil {
				return err
			}

			fmt.Fprintln(cmd.OutOrStdout(), "快照已删除")
			return nil
		},
	}

	return command
}
