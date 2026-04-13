package cobra

import (
	"fmt"
	"io"
	"time"

	spcobra "github.com/spf13/cobra"

	"ai-sync-manager/internal/app/usecase"
	"ai-sync-manager/internal/cli/output"
)

func newSettingCommand(deps Dependencies) *spcobra.Command {
	command := &spcobra.Command{
		Use:   "setting <command>",
		Short: "管理 Claude AI 配置",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			printSettingUsage(cmd.ErrOrStderr())
			return errCommandHandled
		},
	}

	createCommand := &spcobra.Command{
		Use:   "create",
		Short: "创建新的 AI 配置",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			var name, token, api, opusModel, sonnetModel string
			name, _ = cmd.Flags().GetString("name")
			token, _ = cmd.Flags().GetString("token")
			api, _ = cmd.Flags().GetString("api")
			opusModel, _ = cmd.Flags().GetString("opus-model")
			sonnetModel, _ = cmd.Flags().GetString("sonnet-model")

			result, err := deps.Workflow.CreateAISetting(cmd.Context(), usecase.CreateAISettingInput{
				Name:        name,
				Token:       token,
				BaseURL:     api,
				OpusModel:   opusModel,
				SonnetModel: sonnetModel,
			})
			if err != nil {
				return err
			}

			printCreatedSetting(cmd.OutOrStdout(), result)
			return nil
		},
	}
	createCommand.Flags().String("name", "", "配置名称（必填）")
	createCommand.Flags().String("token", "", "Auth token（必填）")
	createCommand.Flags().String("api", "", "API base URL（必填）")
	createCommand.Flags().String("opus-model", "", "Opus 模型（可选）")
	createCommand.Flags().String("sonnet-model", "", "Sonnet 模型（可选）")
	createCommand.MarkFlagRequired("name")
	createCommand.MarkFlagRequired("token")
	createCommand.MarkFlagRequired("api")

	listCommand := &spcobra.Command{
		Use:   "list",
		Short: "列出所有已保存的配置",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			var limit, offset int
			limit, _ = cmd.Flags().GetInt("limit")
			offset, _ = cmd.Flags().GetInt("offset")

			result, err := deps.Workflow.ListAISettings(cmd.Context(), usecase.ListAISettingsInput{
				Limit:  limit,
				Offset: offset,
			})
			if err != nil {
				return err
			}

			printSettingList(cmd.OutOrStdout(), result)
			return nil
		},
	}
	listCommand.Flags().IntP("limit", "l", 0, "每页条数（0 表示全部）")
	listCommand.Flags().IntP("offset", "o", 0, "偏移量")

	getCommand := &spcobra.Command{
		Use:   "get <name> [name...]",
		Short: "获取指定配置的详情",
		Args:  validateAtLeastOneArg("ai-sync setting get <name>"),
		RunE: func(cmd *spcobra.Command, args []string) error {
			// 判断是否为批量操作
			if len(args) > 1 {
				// 批量获取
				batchResult, err := deps.Workflow.GetAISettingsBatch(cmd.Context(), usecase.GetAISettingsBatchInput{
					Names: args,
				})
				if err != nil {
					return err
				}

				printBatchSettingDetails(cmd.OutOrStdout(), batchResult)
				return nil
			}

			// 单个获取（保持原有逻辑）
			result, err := deps.Workflow.GetAISetting(cmd.Context(), usecase.GetAISettingInput{
				Name: args[0],
			})
			if err != nil {
				return err
			}

			printSettingDetail(cmd.OutOrStdout(), result)
			return nil
		},
	}

	deleteCommand := &spcobra.Command{
		Use:   "delete <name> [name...]",
		Short: "删除指定的配置",
		Args:  validateAtLeastOneArg("ai-sync setting delete <name>"),
		RunE: func(cmd *spcobra.Command, args []string) error {
			// 判断是否为批量操作
			if len(args) > 1 {
				// 批量删除：先确认，再执行
				fmt.Fprintln(cmd.OutOrStdout(), "将删除以下配置：")
				for _, name := range args {
					fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", name)
				}

				// 确认删除
				fmt.Fprint(cmd.OutOrStderr(), "确认删除？(y/yes): ")
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "y" && confirm != "yes" {
					fmt.Fprintln(cmd.OutOrStdout(), "已取消删除")
					return nil
				}

				batchResult, err := deps.Workflow.DeleteAISettingsBatch(cmd.Context(), usecase.DeleteAISettingsBatchInput{
					Names: args,
				})
				if err != nil {
					return err
				}

				printBatchDeleteResult(cmd.OutOrStdout(), batchResult)
				return nil
			}

			// 单个删除（保持原有逻辑）
			if err := deps.Workflow.DeleteAISetting(cmd.Context(), usecase.DeleteAISettingInput{
				Name: args[0],
			}); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "配置已删除: %s\n", args[0])
			return nil
		},
	}

	switchCommand := &spcobra.Command{
		Use:   "switch <name>",
		Short: "切换到指定的配置",
		Args:  validateExactOneArg("ai-sync setting switch <name>"),
		RunE: func(cmd *spcobra.Command, args []string) error {
			result, err := deps.Workflow.SwitchAISetting(cmd.Context(), usecase.SwitchAISettingInput{
				Name: args[0],
			})
			if err != nil {
				return err
			}

			printSwitchResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	editCommand := newSettingEditCommand(deps)

	command.AddCommand(createCommand, listCommand, getCommand, deleteCommand, switchCommand, editCommand)
	return command
}

// printCreatedSetting 输出创建成功的配置信息。
func printCreatedSetting(w io.Writer, result *usecase.CreateAISettingResult) {
	fmt.Fprintf(w, "配置已创建: %s\n", result.ID)
}

// printSettingDetail 输出配置详情。
func printSettingDetail(w io.Writer, result *usecase.GetAISettingResult) {
	fmt.Fprintf(w, "配置名称: %s\n", result.Name)
	fmt.Fprintf(w, "配置 ID: %s\n", result.ID)

	// Token 脱敏展示
	maskedToken := maskToken(result.Token)
	fmt.Fprintf(w, "Token: %s\n", maskedToken)

	fmt.Fprintf(w, "Base URL: %s\n", result.BaseURL)
	if result.OpusModel != "" {
		fmt.Fprintf(w, "Opus 模型: %s\n", result.OpusModel)
	}
	if result.SonnetModel != "" {
		fmt.Fprintf(w, "Sonnet 模型: %s\n", result.SonnetModel)
	}
	if result.IsCurrent {
		fmt.Fprintln(w, "当前生效: 是")
	} else {
		fmt.Fprintln(w, "当前生效: 否")
	}
	fmt.Fprintf(w, "创建时间: %s\n", result.CreatedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "更新时间: %s\n", result.UpdatedAt.Format(time.RFC3339))
}

// maskToken 对 token 进行脱敏处理。
// 规则：长度 > 8 时显示前4位+****+后4位；否则全部遮盖。
func maskToken(token string) string {
	if len(token) > 8 {
		return token[:4] + "****" + token[len(token)-4:]
	}
	if len(token) > 0 {
		return "****"
	}
	return ""
}

// printSettingList 输出配置列表。
func printSettingList(w io.Writer, result *usecase.ListAISettingsResult) {
	if result.Total == 0 {
		fmt.Fprintln(w, "暂无保存的配置")
		return
	}

	tbl := &output.Table{
		Columns: []output.Column{
			{Title: "名称"},
			{Title: "Base URL"},
			{Title: "Opus 模型"},
			{Title: "Sonnet 模型"},
		},
		Footer: fmt.Sprintf("当前生效配置: %s", result.Current),
	}
	for _, item := range result.Items {
		name := item.Name
		if item.IsCurrent {
			name = "* " + name
		}
		tbl.Rows = append(tbl.Rows, output.Row{
			Cells:     []string{name, item.BaseURL, item.OpusModel, item.SonnetModel},
			Highlight: item.IsCurrent,
		})
	}
	fmt.Fprint(w, tbl.Render())
}

// printSwitchResult 输出切换结果。
func printSwitchResult(w io.Writer, result *usecase.SwitchAISettingResult) {
	if result.PreviousName != "" {
		fmt.Fprintf(w, "已从 %q 切换到 %q\n", result.PreviousName, result.NewName)
	} else {
		fmt.Fprintf(w, "已切换到 %q\n", result.NewName)
	}
	fmt.Fprintf(w, "备份路径: %s\n", result.BackupPath)
}

// printSettingUsage 输出简版帮助。
func printSettingUsage(w io.Writer) {
	fmt.Fprintln(w, "请指定子命令，例如: ai-sync setting list")
}

// printBatchSettingDetails 输出批量获取配置的结果。
func printBatchSettingDetails(w io.Writer, result *usecase.GetAISettingsBatchResult) {
	for i, item := range result.Items {
		if i > 0 {
			fmt.Fprintln(w, "---")
		}
		fmt.Fprintf(w, "配置: %s\n", item.Name)
		printSettingDetail(w, item)
	}

	// 输出失败汇总
	if len(result.Failed) > 0 {
		fmt.Fprintf(w, "\n获取失败的配置: %s\n", joinStrings(result.Failed, ", "))
	}

	// 输出总汇总
	total := len(result.Items) + len(result.Failed)
	fmt.Fprintf(w, "\n汇总：成功 %d 个，失败 %d 个（共 %d 个）\n",
		len(result.Items), len(result.Failed), total)
}

// joinStrings 连接字符串切片，用于兼容性。
func joinStrings(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	result := items[0]
	for i := 1; i < len(items); i++ {
		result += sep + items[i]
	}
	return result
}

// printBatchDeleteResult 输出批量删除配置的结果。
func printBatchDeleteResult(w io.Writer, result *usecase.DeleteAISettingsBatchResult) {
	if len(result.Deleted) > 0 {
		fmt.Fprintf(w, "已删除: %s\n", joinStrings(result.Deleted, ", "))
	}

	if len(result.Failed) > 0 {
		fmt.Fprintf(w, "删除失败: %s\n", joinStrings(result.Failed, ", "))
	}

	// 输出总汇总
	total := len(result.Deleted) + len(result.Failed)
	fmt.Fprintf(w, "汇总：成功 %d 个，失败 %d 个（共 %d 个）\n",
		len(result.Deleted), len(result.Failed), total)
}
