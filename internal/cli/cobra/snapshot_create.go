package cobra

import (
	"strings"

	spcobra "github.com/spf13/cobra"

	"ai-sync-manager/internal/app/usecase"
	"ai-sync-manager/internal/models"
)

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
	flags.StringVar(&tools, "tools", "", "Comma-separated tool list")
	flags.StringVar(&message, "message", "", "Snapshot message")
	flags.StringVar(&name, "name", "", "Snapshot name")
	flags.StringVar(&scope, "scope", string(models.ScopeGlobal), "Snapshot scope")
	flags.StringVar(&projectPath, "project-path", "", "Project path")

	return command
}

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
