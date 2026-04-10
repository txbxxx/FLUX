package cobra

import (
	"errors"
	"path/filepath"
	"strings"

	spcobra "github.com/spf13/cobra"

	"ai-sync-manager/internal/app/usecase"
)

func newGetCommand(deps Dependencies) *spcobra.Command {
	var edit bool
	var snapshot string

	command := &spcobra.Command{
		Use:   "get <project> [path]",
		Short: "查看或编辑指定 AI 工具配置",
		Args:  spcobra.MinimumNArgs(1),
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
