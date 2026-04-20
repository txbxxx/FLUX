package cobra

import (
	"fmt"

	"flux/internal/app/usecase"
	"flux/internal/cli/output"

	spcobra "github.com/spf13/cobra"
)

// newScanCommand 组装 scan 命令及其规则管理子命令。
func newScanCommand(deps Dependencies) *spcobra.Command {
	command := &spcobra.Command{
		Use:   "scan [app-or-project...]",
		Short: "扫描本地 AI 工具配置",
		RunE: func(cmd *spcobra.Command, args []string) error {
			result, err := deps.Workflow.Scan(cmd.Context(), usecase.ScanInput{Apps: args})
			if err != nil {
				return err
			}

			printScanResult(cmd.OutOrStdout(), result, false)
			return nil
		},
	}

	var addProjectMode bool
	addCommand := &spcobra.Command{
		Use:   "add",
		Short: "添加扫描规则或注册项目",
		RunE: func(cmd *spcobra.Command, args []string) error {
			if addProjectMode {
				if len(args) != 3 {
					return fmt.Errorf("请按格式输入: fl scan add --project <应用名> <项目名> <项目绝对路径>")
				}
				return deps.Workflow.AddProject(cmd.Context(), usecase.AddProjectInput{
					App:         args[0],
					ProjectName: args[1],
					ProjectPath: args[2],
				})
			}

			if len(args) != 2 {
				return fmt.Errorf("请按格式输入: fl scan add <应用名> <文件绝对路径>")
			}
			return deps.Workflow.AddCustomRule(cmd.Context(), usecase.AddCustomRuleInput{
				App:          args[0],
				AbsolutePath: args[1],
			})
		},
	}
	addCommand.Flags().BoolVarP(&addProjectMode, "project", "p", false, "注册项目路径")

	var removeProjectMode bool
	removeCommand := &spcobra.Command{
		Use:   "remove",
		Short: "删除扫描规则或已注册项目",
		RunE: func(cmd *spcobra.Command, args []string) error {
			if removeProjectMode {
				if len(args) != 2 {
					return fmt.Errorf("请按格式输入: fl scan remove --project <应用名> <项目绝对路径>")
				}
				return deps.Workflow.RemoveProject(cmd.Context(), usecase.RemoveProjectInput{
					App:         args[0],
					ProjectPath: args[1],
				})
			}

			if len(args) != 2 {
				return fmt.Errorf("请按格式输入: fl scan remove <应用名> <文件绝对路径>")
			}
			return deps.Workflow.RemoveCustomRule(cmd.Context(), usecase.RemoveCustomRuleInput{
				App:          args[0],
				AbsolutePath: args[1],
			})
		},
	}
	removeCommand.Flags().BoolVarP(&removeProjectMode, "project", "p", false, "删除已注册项目")

	var verbose bool
	listCommand := &spcobra.Command{
		Use:   "list [app-or-project...]",
		Short: "查看当前扫描结果",
		RunE: func(cmd *spcobra.Command, args []string) error {
			formatStr, _ := cmd.Flags().GetString("format")

			result, err := deps.Workflow.Scan(cmd.Context(), usecase.ScanInput{Apps: args})
			if err != nil {
				return err
			}

			// 准备表格数据
			tbl := buildScanTable(result, verbose)

			// 准备 JSON/YAML 数据
			rawData := toScanListData(result)

			// 根据格式输出
			return output.Print(cmd.OutOrStdout(), output.Format(formatStr), tbl, rawData)
		},
	}
	listCommand.Flags().BoolVarP(&verbose, "verbose", "v", false, "显示详细配置项")
	listCommand.Flags().String("format", "table", "输出格式: table, json, yaml")

	rulesCommand := &spcobra.Command{
		Use:   "rules [app-or-project]",
		Short: "查看当前扫描规则",
		RunE: func(cmd *spcobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("请按格式输入: fl scan rules [应用或项目名]")
			}

			app := ""
			if len(args) == 1 {
				app = args[0]
			}

			result, err := deps.Workflow.ListScanRules(cmd.Context(), usecase.ListScanRulesInput{App: app})
			if err != nil {
				return err
			}
			printScanRuleList(cmd.OutOrStdout(), result)
			return nil
		},
	}

	command.AddCommand(addCommand, removeCommand, listCommand, rulesCommand)
	return command
}

// toScanListData 将 usecase 返回结果转换为 types 结构体（用于 JSON/YAML）
func toScanListData(result *usecase.ScanResult) interface{} {
	items := make([]map[string]interface{}, len(result.Tools))
	for i, tool := range result.Tools {
		items[i] = map[string]interface{}{
			"project":      tool.ProjectName,
			"tool_type":    tool.Tool,
			"path":         tool.Path,
			"status":       tool.ResultText,
			"config_count": tool.ConfigCount,
			"items":        tool.Items, // 详细配置项
		}
	}
	return map[string]interface{}{
		"items": items,
	}
}

// buildScanTable 构建扫描结果表格
func buildScanTable(result *usecase.ScanResult, verbose bool) *output.Table {
	if len(result.Tools) == 0 {
		return &output.Table{}
	}

	tbl := &output.Table{
		Columns: []output.Column{
			{Title: "项目"},
			{Title: "类型"},
			{Title: "配置目录"},
			{Title: "状态"},
			{Title: "可同步项"},
		},
	}
	for _, item := range result.Tools {
		projectName := displayScanSummaryTitle(item)
		toolType := displayToolName(item.Tool)
		configCount := ""
		if item.ConfigCount > 0 {
			configCount = fmt.Sprintf("%d 项", item.ConfigCount)
		}
		tbl.Rows = append(tbl.Rows, output.Row{
			Cells: []string{projectName, toolType, item.Path, item.ResultText, configCount},
		})
	}
	return tbl
}
