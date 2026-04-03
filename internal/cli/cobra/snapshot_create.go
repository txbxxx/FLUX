package cobra

import (
	"strings"

	spcobra "github.com/spf13/cobra"

	"ai-sync-manager/internal/app/usecase"
	"ai-sync-manager/internal/models"
)

// newSnapshotCreateCommand 把命令行参数映射成创建快照用例输入。
func newSnapshotCreateCommand(deps Dependencies) *spcobra.Command {
	var tools string
	var message string
	var name string
	var scope string
	var projectPath string

	command := &spcobra.Command{
		Use:   "create",
		Short: "创建本地快照",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			result, err := deps.Workflow.CreateSnapshot(cmd.Context(), usecase.CreateSnapshotInput{
				Tools:       splitCSV(tools),
				Message:     message,
				Name:        name,
				Scope:       parseScope(scope),
				ProjectPath: projectPath,
			})
			if err != nil {
				return err
			}

			printCreatedSnapshot(cmd.OutOrStdout(), result)
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVar(&tools, "tools", "", "指定要备份的工具，多个用逗号分隔（如 codex,claude）")
	flags.StringVar(&message, "message", "", "快照说明（必填）")
	flags.StringVar(&name, "name", "", "快照名称（可选）")
	flags.StringVar(&scope, "scope", string(models.ScopeGlobal), "快照范围: global、project 或 both")
	flags.StringVar(&projectPath, "project-path", "", "项目路径（可选）")

	return command
}

// splitCSV / parseScope 负责把 CLI 字符串参数规范化成模型层输入。
func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func parseScope(value string) models.SnapshotScope {
	switch strings.TrimSpace(value) {
	case string(models.ScopeProject):
		return models.ScopeProject
	case string(models.ScopeBoth):
		return models.ScopeBoth
	default:
		return models.ScopeGlobal
	}
}
