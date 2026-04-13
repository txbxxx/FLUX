package cobra

import (
	"fmt"
	"io"
	"strings"

	spcobra "github.com/spf13/cobra"

	"ai-sync-manager/internal/app/usecase"
	typesSnapshot "ai-sync-manager/internal/types/snapshot"
)

// newSnapshotHistoryCommand creates the snapshot history sub-command.
func newSnapshotHistoryCommand(deps Dependencies) *spcobra.Command {
	var limit int

	command := &spcobra.Command{
		Use:   "history <id-or-name>",
		Short: "查看快照的版本历史",
		Long: `查看快照在远端仓库中的版本历史记录。

展示每次推送的时间、提交说明和版本哈希，
可通过 restore --version <hash> 恢复到指定版本。`,
		Args: validateExactOneArg("ai-sync snapshot history <id-or-name>"),
		RunE: func(cmd *spcobra.Command, args []string) error {
			result, err := deps.Workflow.SnapshotHistory(cmd.Context(), usecase.SnapshotHistoryInput{
				IDOrName: args[0],
				Limit:    limit,
			})
			if err != nil {
				return err
			}

			printSnapshotHistory(cmd.OutOrStdout(), result)
			return nil
		},
	}

	flags := command.Flags()
	flags.IntVarP(&limit, "limit", "l", 20, "显示条数")

	return command
}

func printSnapshotHistory(w io.Writer, result *typesSnapshot.HistoryResult) {
	fmt.Fprintf(w, "项目: %s\n\n", result.Project)

	if len(result.Entries) == 0 {
		fmt.Fprintln(w, "暂无版本历史")
		return
	}

	fmt.Fprintf(w, "共 %d 条记录\n\n", result.Total)

	for i, entry := range result.Entries {
		shortHash := entry.CommitHash
		if len(shortHash) > 8 {
			shortHash = shortHash[:8]
		}

		fmt.Fprintf(w, "  [%d] %s  %s\n", i+1, shortHash, entry.Date.Format("2006-01-02 15:04"))

		// 只显示 commit message 的第一行
		firstLine := entry.Message
		if idx := strings.Index(entry.Message, "\n"); idx > 0 {
			firstLine = entry.Message[:idx]
		}
		if len(firstLine) > 60 {
			firstLine = firstLine[:60] + "..."
		}
		fmt.Fprintf(w, "      %s\n\n", firstLine)
	}

	fmt.Fprintln(w, "使用 ai-sync snapshot restore <id> --version <hash> 恢复指定版本")
}
