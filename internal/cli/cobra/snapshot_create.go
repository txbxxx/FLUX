package cobra

import (
	"strings"

	spcobra "github.com/spf13/cobra"

	"ai-sync-manager/internal/app/usecase"
)

// newSnapshotCreateCommand 创建 snapshot create 子命令。
// 将命令行 flag 参数映射为 usecase.CreateSnapshotInput，调用 Workflow.CreateSnapshot 完成快照创建。
//
// 参数说明：
//   - -t / --tools：      要备份的工具列表（可选），多个用逗号分隔。省略时从 -p 项目名自动推导。
//   - -m / --message：    快照说明（必填），用于记录本次快照的目的。
//   - -n / --name：       快照名称（可选），不填时由系统自动生成。
//   - -p / --project：    项目名称（必填），指定要备份哪个项目的配置。
//
// 工具类型自动推导示例：
//
//	ai-sync snapshot create -m "备份" -p claude        → 自动推导 tools=["claude"]
//	ai-sync snapshot create -m "备份" -p codex-global  → 自动推导 tools=["codex"]
//	ai-sync snapshot create -m "备份" -p my-project    → 从数据库查找项目 → 取 ToolType
func newSnapshotCreateCommand(deps Dependencies) *spcobra.Command {
	var tools string
	var message string
	var name string
	var projectName string

	command := &spcobra.Command{
		Use:   "create",
		Short: "创建本地快照",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			result, err := deps.Workflow.CreateSnapshot(cmd.Context(), usecase.CreateSnapshotInput{
				Tools:       splitCSV(tools),
				Message:     message,
				Name:        name,
				ProjectName: projectName,
			})
			if err != nil {
				return err
			}

			printCreatedSnapshot(cmd.OutOrStdout(), result)
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVarP(&tools, "tools", "t", "", "指定要备份的工具，多个用逗号分隔（如 codex,claude）")
	flags.StringVarP(&message, "message", "m", "", "快照说明（必填）")
	flags.StringVarP(&name, "name", "n", "", "快照名称（可选）")
	flags.StringVarP(&projectName, "project", "p", "", "项目名称（必填，如 codex-global、claude-global 或用户注册的项目）")

	return command
}

// splitCSV 将逗号分隔的 CLI 字符串参数拆分为字符串切片。
// 处理规则：去除每段的前后空格，跳过空段。
// 例如 "codex, claude " → ["codex", "claude"]，"" → nil
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
