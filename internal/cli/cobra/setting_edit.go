package cobra

import (
	"fmt"
	"io"
	"time"

	spcobra "github.com/spf13/cobra"

	"flux/internal/app/usecase"
)

// newSettingEditCommand 创建 edit 子命令。
func newSettingEditCommand(deps Dependencies) *spcobra.Command {
	var newName, token, api string
	var models []string

	command := &spcobra.Command{
		Use:   "edit <name>",
		Short: "编辑 AI 配置",
		Args:  validateExactOneArg("fl setting edit <name>"),
		RunE: func(cmd *spcobra.Command, args []string) error {
			name := args[0]

			// 命令行参数模式
			result, err := deps.Workflow.EditAISetting(cmd.Context(), usecase.EditAISettingInput{
				Name:    name,
				NewName: newName,
				Token:   token,
				BaseURL: api,
				Models:  models,
			})
			if err != nil {
				return err
			}

			printEditResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	command.Flags().StringVar(&newName, "name", "", "新名称")
	command.Flags().StringVarP(&token, "token", "t", "", "新 Token")
	command.Flags().StringVarP(&api, "api", "a", "", "新 API base URL")
	command.Flags().StringSliceVar(&models, "model", nil, "新模型列表")

	return command
}

// printEditResult 输出编辑结果。
func printEditResult(w io.Writer, result *usecase.EditAISettingResult) {
	// 如果是当前配置，给出提示
	if result.IsCurrent {
		fmt.Fprintln(w, "注意：这是当前生效的配置，修改后可能需要重新执行 switch 才能生效。")
		fmt.Fprintln(w)
	}

	if len(result.Changes) == 0 {
		fmt.Fprintln(w, "无变更")
		return
	}

	if len(result.Changes) == 0 {
		fmt.Fprintln(w, "无变更")
		return
	}

	fmt.Fprintln(w, "变更内容：")
	for _, change := range result.Changes {
		status := "（已修改）"
		if change.OldValue == "" {
			status = "（新增）"
		}
		fmt.Fprintf(w, "- %s: %s → %s %s\n", change.Field, change.OldValue, change.NewValue, status)
	}

	fmt.Fprintf(w, "\n更新时间: %s\n", result.UpdatedAt.Format(time.RFC3339))
}
