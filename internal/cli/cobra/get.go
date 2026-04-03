package cobra

import (
	"errors"

	spcobra "github.com/spf13/cobra"

	"ai-sync-manager/internal/app/usecase"
)

func newGetCommand(deps Dependencies) *spcobra.Command {
	var edit bool

	command := &spcobra.Command{
		Use:   "get <app> <path>",
		Short: "查看或编辑指定 AI 工具配置",
		Args:  spcobra.ExactArgs(2),
		RunE: func(cmd *spcobra.Command, args []string) error {
			result, err := deps.Workflow.GetConfig(cmd.Context(), usecase.GetConfigInput{
				Tool: args[0],
				Path: args[1],
				Edit: edit,
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
						Tool:    args[0],
						Path:    args[1],
						Content: content,
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

	command.Flags().BoolVar(&edit, "edit", false, "Open file in terminal editor")
	return command
}
