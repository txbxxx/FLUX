package cobra

import (
	"fmt"

	spcobra "github.com/spf13/cobra"

	"flux/internal/app/usecase"
	"flux/internal/cli/output"
	"flux/internal/types/snapshot"
)

// newSnapshotListCommand 组装本地快照列表命令。
func newSnapshotListCommand(deps Dependencies) *spcobra.Command {
	var limit int
	var offset int
	var formatStr string

	command := &spcobra.Command{
		Use:   "list",
		Short: "列出本地快照",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			result, err := deps.Workflow.ListSnapshots(cmd.Context(), usecase.ListSnapshotsInput{
				Limit:  limit,
				Offset: offset,
			})
			if err != nil {
				return err
			}

			// 准备表格数据
			tbl := buildSnapshotTable(result)

			// 准备 JSON/YAML 数据
			rawData := toSnapshotListData(result)

			// 空列表特殊处理
			if tbl == nil && formatStr == "table" {
				fmt.Fprintln(cmd.OutOrStdout(), "暂无本地快照")
				return nil
			}

			// 根据格式输出
			return output.Print(cmd.OutOrStdout(), output.Format(formatStr), tbl, rawData)
		},
	}

	flags := command.Flags()
	flags.IntVarP(&limit, "limit", "l", 0, "最多显示条数")
	flags.IntVarP(&offset, "offset", "o", 0, "从第几条开始")
	flags.StringVar(&formatStr, "format", "table", "输出格式: table, json, yaml")

	return command
}

// toSnapshotListData 将 usecase 返回结果转换为 types 结构体（用于 JSON/YAML）
func toSnapshotListData(result *usecase.ListSnapshotsResult) interface{} {
	items := make([]snapshot.SnapshotInfo, len(result.Items))
	for i, item := range result.Items {
		items[i] = snapshot.SnapshotInfo{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Message,
			CreatedAt:   item.CreatedAt,
			Project:     item.Project,
		}
	}
	return map[string]interface{}{
		"total": result.Total,
		"items": items,
	}
}

// buildSnapshotTable 构建快照列表表格
func buildSnapshotTable(result *usecase.ListSnapshotsResult) *output.Table {
	if len(result.Items) == 0 {
		return nil // 返回 nil 表示无数据
	}

	tbl := &output.Table{
		Columns: []output.Column{
			{Title: "ID"},
			{Title: "名称"},
			{Title: "项目"},
			{Title: "说明"},
			{Title: "文件数"},
			{Title: "创建时间"},
		},
		Footer: fmt.Sprintf("共 %d 条快照", result.Total),
	}
	for _, item := range result.Items {
		tbl.Rows = append(tbl.Rows, output.Row{
			Cells: []string{
				fmt.Sprintf("%d", item.ID),
				item.Name,
				item.Project,
				item.Message,
				fmt.Sprintf("%d", item.FileCount),
				item.CreatedAt.Format("2006-01-02 15:04"),
			},
		})
	}
	return tbl
}
