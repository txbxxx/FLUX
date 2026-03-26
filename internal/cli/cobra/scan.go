package cobra

import (
	"fmt"

	"ai-sync-manager/internal/app/usecase"

	spcobra "github.com/spf13/cobra"
)

func newScanCommand(deps Dependencies) *spcobra.Command {
	command := &spcobra.Command{
		Use:   "scan [app-or-project...]",
		Short: "扫描本地 AI 工具配置",
		RunE: func(cmd *spcobra.Command, args []string) error {
			result, err := deps.Workflow.Scan(cmd.Context(), usecase.ScanInput{Apps: args})
			if err != nil {
				return err
			}

			printScanResult(cmd.OutOrStdout(), result)
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
					return fmt.Errorf("用法: ai-sync scan add --project <app> <project-name> <project-absolute-path>")
				}
				return deps.Workflow.AddProject(cmd.Context(), usecase.AddProjectInput{
					App:         args[0],
					ProjectName: args[1],
					ProjectPath: args[2],
				})
			}

			if len(args) != 2 {
				return fmt.Errorf("用法: ai-sync scan add <app> <absolute-file-path>")
			}
			return deps.Workflow.AddCustomRule(cmd.Context(), usecase.AddCustomRuleInput{
				App:          args[0],
				AbsolutePath: args[1],
			})
		},
	}
	addCommand.Flags().BoolVar(&addProjectMode, "project", false, "注册项目路径")

	var removeProjectMode bool
	removeCommand := &spcobra.Command{
		Use:   "remove",
		Short: "删除扫描规则或已注册项目",
		RunE: func(cmd *spcobra.Command, args []string) error {
			if removeProjectMode {
				if len(args) != 2 {
					return fmt.Errorf("用法: ai-sync scan remove --project <app> <project-absolute-path>")
				}
				return deps.Workflow.RemoveProject(cmd.Context(), usecase.RemoveProjectInput{
					App:         args[0],
					ProjectPath: args[1],
				})
			}

			if len(args) != 2 {
				return fmt.Errorf("用法: ai-sync scan remove <app> <absolute-file-path>")
			}
			return deps.Workflow.RemoveCustomRule(cmd.Context(), usecase.RemoveCustomRuleInput{
				App:          args[0],
				AbsolutePath: args[1],
			})
		},
	}
	removeCommand.Flags().BoolVar(&removeProjectMode, "project", false, "删除已注册项目")

	listCommand := &spcobra.Command{
		Use:   "list [app-or-project]",
		Short: "查看当前扫描规则",
		RunE: func(cmd *spcobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("用法: ai-sync scan list [app-or-project]")
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

	command.AddCommand(addCommand, removeCommand, listCommand)
	return command
}
