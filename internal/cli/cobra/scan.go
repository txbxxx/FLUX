package cobra

import (
	spcobra "github.com/spf13/cobra"
)

func newScanCommand(deps Dependencies) *spcobra.Command {
	return &spcobra.Command{
		Use:   "scan",
		Short: "扫描本地 AI 工具配置",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			result, err := deps.Workflow.Scan(cmd.Context())
			if err != nil {
				return err
			}

			printScanResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
}
