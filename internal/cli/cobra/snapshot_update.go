package cobra

import (
	"fmt"
	"io"

	spcobra "github.com/spf13/cobra"

	"flux/internal/app/usecase"
	typesSnapshot "flux/internal/types/snapshot"
)

// newSnapshotUpdateCommand 创建 snapshot update 子命令。
// 重新扫描关联 project 的配置文件，对比哈希后更新 SQLite 中的快照数据。
//
// 参数说明：
//   - <id-or-name>：   快照 ID 或名称（必填）
//   - -m / --message： 更新说明（可选，不传则保留原有 message）
func newSnapshotUpdateCommand(deps Dependencies) *spcobra.Command {
	var message string

	command := &spcobra.Command{
		Use:   "update <id-or-name>",
		Short: "更新快照（重新扫描配置文件）",
		Args:  validateExactOneArg("ai-sync snapshot update <id-or-name>"),
		RunE: func(cmd *spcobra.Command, args []string) error {
			result, err := deps.Workflow.UpdateSnapshot(cmd.Context(), usecase.UpdateSnapshotInput{
				IDOrName: args[0],
				Message:  message,
			})
			if err != nil {
				return err
			}

			printUpdatedSnapshot(cmd.OutOrStdout(), result)
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVarP(&message, "message", "m", "", "更新说明（可选）")

	return command
}

// printUpdatedSnapshot 渲染快照更新结果到终端输出。
func printUpdatedSnapshot(w io.Writer, result *typesSnapshot.UpdateSnapshotResult) {
	if result.NoChanges {
		fmt.Fprintf(w, "快照 \"%s\" 无变更，所有文件内容与当前配置一致。\n", result.SnapshotName)
		return
	}

	fmt.Fprintf(w, "快照 \"%s\" 已更新\n\n", result.SnapshotName)

	if result.FilesAdded > 0 {
		fmt.Fprintf(w, "  新增: %d 个文件\n", result.FilesAdded)
	}
	if result.FilesUpdated > 0 {
		fmt.Fprintf(w, "  更新: %d 个文件\n", result.FilesUpdated)
	}
	if result.FilesRemoved > 0 {
		fmt.Fprintf(w, "  删除: %d 个文件\n", result.FilesRemoved)
	}
	if result.FilesUnchanged > 0 {
		fmt.Fprintf(w, "  未变: %d 个文件\n", result.FilesUnchanged)
	}
	fmt.Fprintln(w)
}
