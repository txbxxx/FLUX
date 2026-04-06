package cobra

import (
	"fmt"
	"io"
	"time"

	spcobra "github.com/spf13/cobra"

	"ai-sync-manager/internal/app/usecase"
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
		Use:   "get <name>",
		Short: "获取指定配置的详情",
		Args:  spcobra.ExactArgs(1),
		RunE: func(cmd *spcobra.Command, args []string) error {
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
		Use:   "delete <name>",
		Short: "删除指定的配置",
		Args:  spcobra.ExactArgs(1),
		RunE: func(cmd *spcobra.Command, args []string) error {
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
		Args:  spcobra.ExactArgs(1),
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

	command.AddCommand(createCommand, listCommand, getCommand, deleteCommand, switchCommand)
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

	fmt.Fprintf(w, "配置列表（共 %d 个）\n\n", result.Total)
	for _, item := range result.Items {
		prefix := "  "
		if item.IsCurrent {
			prefix = "* "
		}
		fmt.Fprintf(w, "%s%s\n", prefix, item.Name)
		fmt.Fprintf(w, "    Base URL: %s\n", item.BaseURL)
		if item.OpusModel != "" {
			fmt.Fprintf(w, "    Opus 模型: %s\n", item.OpusModel)
		}
		if item.SonnetModel != "" {
			fmt.Fprintf(w, "    Sonnet 模型: %s\n", item.SonnetModel)
		}
		if item.IsCurrent {
			fmt.Fprintf(w, "    (当前生效)\n")
		}
		fmt.Fprintln(w)
	}

	if result.Current != "" {
		fmt.Fprintf(w, "当前生效配置: %s\n", result.Current)
	}
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
