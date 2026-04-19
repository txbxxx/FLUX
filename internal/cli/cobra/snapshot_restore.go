package cobra

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	spcobra "github.com/spf13/cobra"

	"flux/internal/app/usecase"
	"flux/internal/cli/output"
	typesSnapshot "flux/internal/types/snapshot"
)

// newSnapshotRestoreCommand creates the snapshot restore sub-command.
// Supports full restore, selective restore (--files), preview mode (--dry-run),
// and skip confirmation (--force).
func newSnapshotRestoreCommand(deps Dependencies) *spcobra.Command {
	var files string
	var dryRun bool
	var force bool

	command := &spcobra.Command{
		Use:   "restore <id-or-name>",
		Short: "恢复快照到本地配置",
		Long: `将快照中的配置文件恢复到磁盘原始路径。

支持三种模式：
  - 全量恢复：恢复快照中的所有文件
  - 选择性恢复：通过 --files 指定要恢复的文件
  - 预览模式：通过 --dry-run 仅查看变更，不实际写入

恢复前会自动备份当前配置到 ~/.flux/backup/<timestamp>/。`,
		Args: validateExactOneArg("fl snapshot restore <id-or-name>"),
		RunE: func(cmd *spcobra.Command, args []string) error {
			input := usecase.RestoreSnapshotInput{
				IDOrName:  args[0],
				Files:     splitCSV(files),
				DryRun:    dryRun,
				Force:     force,
				BackupDir: deps.DataDir,
			}

			// dry-run 模式：展示 diff + 恢复摘要
			if dryRun {
				result, err := deps.Workflow.RestoreSnapshot(cmd.Context(), input)
				if err != nil {
					return err
				}

				// 使用 diff 渲染展示差异详情
				diffResult, _ := deps.Workflow.DiffSnapshots(cmd.Context(), usecase.DiffSnapshotsInput{
					SourceID: args[0],
					Verbose:  true,
					Context:  5,
				})
				if diffResult != nil && diffResult.HasDiff {
					output.RenderUnifiedDiff(cmd.OutOrStdout(), diffResult, true)
				}

				// 追加恢复摘要
				printRestorePreview(cmd.OutOrStdout(), result)
				return nil
			}

			// 非 force 模式：先预览，再确认，最后执行
			if !force {
				previewInput := input
				previewInput.DryRun = true
				preview, err := deps.Workflow.RestoreSnapshot(cmd.Context(), previewInput)
				if err != nil {
					return err
				}

				if preview.AppliedCount == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "所有文件内容相同，无需恢复。")
					return nil
				}

				printRestorePreview(cmd.OutOrStdout(), preview)

				fmt.Fprint(cmd.OutOrStdout(), "\n确认恢复？[y/N]: ")
				reader := bufio.NewReader(os.Stdin)
				response, err := reader.ReadString('\n')
				if err != nil || strings.TrimSpace(strings.ToLower(response)) != "y" {
					fmt.Fprintln(cmd.OutOrStdout(), "已取消")
					return nil
				}
			}

			// 正式恢复（含备份）
			result, err := deps.Workflow.RestoreSnapshot(cmd.Context(), input)
			if err != nil {
				return err
			}

			printRestoreResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVar(&files, "files", "", "指定要恢复的文件路径，逗号分隔")
	flags.BoolVar(&dryRun, "dry-run", false, "仅预览变更，不实际写入")
	flags.BoolVar(&force, "force", false, "跳过确认步骤，但仍自动备份")

	return command
}

// printRestorePreview renders the dry-run preview output.
// Shows snapshot info and a list of files that would be affected.
func printRestorePreview(w io.Writer, result *typesSnapshot.RestoreResult) {
	fmt.Fprintf(w, "快照: %s (%s)\n", result.SnapshotName, result.SnapshotID)
	if result.AppliedCount == 0 {
		fmt.Fprintln(w, "所有文件内容相同，无需恢复。")
		return
	}

	fmt.Fprintf(w, "即将恢复 %d 个文件:\n", result.AppliedCount)
	for _, f := range result.AppliedFiles {
		label := "更新"
		if f.Action == "created" {
			label = "新增"
		}
		fmt.Fprintf(w, "  %s: %s\n", label, f.Path)
	}
	for _, f := range result.SkippedFiles {
		fmt.Fprintf(w, "  跳过: %s (%s)\n", f.Path, f.Reason)
	}
}

// printRestoreResult renders the final restore output.
// Shows the restore summary and backup path.
func printRestoreResult(w io.Writer, result *typesSnapshot.RestoreResult) {
	fmt.Fprintf(w, "快照: %s (%s)\n", result.SnapshotName, result.SnapshotID)

	if result.BackupPath != "" {
		fmt.Fprintf(w, "\n备份已创建: %s\n", result.BackupPath)
	}

	fmt.Fprintln(w, "\n恢复完成！")
	fmt.Fprintf(w, "  成功: %d 个文件\n", result.AppliedCount)
	fmt.Fprintf(w, "  跳过: %d 个文件\n", result.SkippedCount)
	fmt.Fprintf(w, "  失败: %d 个文件\n", result.ErrorCount)

	if result.BackupPath != "" {
		fmt.Fprintf(w, "  备份: %s\n", result.BackupPath)
	}

	// 展示失败详情
	for _, e := range result.Errors {
		fmt.Fprintf(w, "\n  失败: %s — %s", e.Path, e.Message)
	}
}
