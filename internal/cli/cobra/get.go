package cobra

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	spcobra "github.com/spf13/cobra"

	"flux/internal/app/usecase"
)

// validateGetArgs validates arguments for the get command with user-friendly Chinese error messages.
func validateGetArgs(cmd *spcobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("请指定项目名称，例如：fl get claude 或 fl get claude settings.json\n查看支持的项目：fl scan list")
	}
	if len(args) > 2 {
		return fmt.Errorf("参数过多，get 命令接受 1-2 个参数：<project> [path]")
	}
	return nil
}

func newGetCommand(deps Dependencies) *spcobra.Command {
	var edit bool
	var snapshot string

	command := &spcobra.Command{
		Use:   "get <project> [path]",
		Short: "查看或编辑指定 AI 工具配置",
		Args:  validateGetArgs,
		RunE: func(cmd *spcobra.Command, args []string) error {
			path := ""
			if len(args) > 1 {
				path = strings.Join(args[1:], string(filepath.Separator))
			}
			result, err := deps.Workflow.GetConfig(cmd.Context(), usecase.GetConfigInput{
				Tool:     args[0],
				Path:     path,
				Edit:     edit,
				Snapshot: snapshot,
			})
			if err != nil {
				return err
			}

			if edit {
				if deps.Editor == nil {
					return errors.New("编辑器未就绪，无法进入编辑模式")
				}
				return deps.Editor.Run(cmd.Context(), result, func(content string) error {
					return deps.Workflow.SaveConfig(cmd.Context(), usecase.SaveConfigInput{
						Tool:     args[0],
						Path:     path,
						Content:  content,
						Snapshot: snapshot,
					})
				})
			}

			switch result.Kind {
			case usecase.ConfigTargetDirectory:
				printConfigEntries(cmd.OutOrStdout(), result)
			case usecase.ConfigTargetFile:
				printConfigFile(cmd.OutOrStdout(), result)
			}

			return nil
		},
	}

	command.Flags().BoolVarP(&edit, "edit", "e", false, "Open file in terminal editor")
	command.Flags().StringVarP(&snapshot, "snapshot", "s", "", "浏览指定快照中的文件（传入快照 ID 或名称）")
	return command
}
