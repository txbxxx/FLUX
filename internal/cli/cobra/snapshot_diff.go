package cobra

import (
	"fmt"
	"os"

	"ai-sync-manager/internal/app/usecase"
	"ai-sync-manager/internal/cli/output"

	spcobra "github.com/spf13/cobra"
)

// errDiffHasChanges is returned when the diff finds differences.
// The CLI uses exit code 1 to signal changes (like git diff).
var errDiffHasChanges = fmt.Errorf("diff has changes")

func newSnapshotDiffCommand(deps Dependencies) *spcobra.Command {
	var verbose bool
	var sideBySide bool
	var tool string
	var pathPattern string
	var color string

	command := &spcobra.Command{
		Use:   "diff <source-id> [<target-id>]",
		Short: "对比快照差异",
		Long: `对比两个快照之间的差异，或快照与当前文件系统的差异。

参数个数决定对比类型：
  1个参数 → 快照 vs 当前文件系统
  2个参数 → 快照 vs 快照`,
		Args: spcobra.RangeArgs(1, 2),
		RunE: func(cmd *spcobra.Command, args []string) error {
			targetID := ""
			if len(args) > 1 {
				targetID = args[1]
			}

			result, err := deps.Workflow.DiffSnapshots(cmd.Context(), usecase.DiffSnapshotsInput{
				SourceID:    args[0],
				TargetID:    targetID,
				Verbose:     verbose,
				SideBySide:  sideBySide,
				Tool:        tool,
				PathPattern: pathPattern,
				Context:     5,
			})
			if err != nil {
				return err
			}

			useColor := shouldUseColor(color, cmd)
			out := cmd.OutOrStdout()

			// 根据模式选择渲染方式
			if !verbose {
				output.RenderDiffSummary(out, result, useColor)
			} else if sideBySide {
				output.RenderSideBySideDiff(out, result, useColor)
			} else {
				output.RenderUnifiedDiff(out, result, useColor)
			}

			// 退出码：有差异返回 1
			if result.HasDiff {
				return errDiffHasChanges
			}
			return nil
		},
	}

	flags := command.Flags()
	flags.BoolVarP(&verbose, "verbose", "v", false, "显示内容级差异（上下文行）")
	flags.BoolVar(&sideBySide, "side-by-side", false, "并排显示差异（配合 -v 使用）")
	flags.StringVar(&tool, "tool", "", "按工具类型过滤（如 claude、codex）")
	flags.StringVar(&pathPattern, "path", "", "按路径模式过滤（如 \"mcp/*\"）")
	flags.StringVar(&color, "color", "auto", "颜色控制：always/auto/never")

	return command
}

// shouldUseColor determines whether to use colored output.
func shouldUseColor(colorFlag string, cmd *spcobra.Command) bool {
	switch colorFlag {
	case "always":
		return true
	case "never":
		return false
	default: // "auto"
		// Check if output is a terminal
		out := cmd.OutOrStdout()
		if f, ok := out.(*os.File); ok {
			return isTerminal(f)
		}
		return false
	}
}

// isTerminal checks if the file descriptor is a terminal.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
