package cobra

import (
	spcobra "github.com/spf13/cobra"

	"ai-sync-manager/internal/app/usecase"
)

// newSnapshotListCommand 组装本地快照列表命令。
func newSnapshotListCommand(deps Dependencies) *spcobra.Command {
	var limit int
	var offset int

	command := &spcobra.Command{
		Use:   "list",
		Short: "列出本地快照",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			result, err := deps.Workflow.ListSnapshots(cmd.Context(), usecase.ListSnapshotsInput{
				Limit:  limit,
				Offset: offset,
			})
			if err != nil {
				return err
			}

			printSnapshotList(cmd.OutOrStdout(), result)
			return nil
		},
	}

	flags := command.Flags()
	flags.IntVar(&limit, "limit", 0, "Max items")
	flags.IntVar(&offset, "offset", 0, "Offset")

	return command
}
