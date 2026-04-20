package cobra

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	spcobra "github.com/spf13/cobra"

	"flux/internal/app/usecase"
	"flux/internal/cli/output"
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
	var formatStr string

	command := &spcobra.Command{
		Use:   "get <project> [path]",
		Short: "查看或编辑指定 AI 工具配置",
		Args:  validateGetArgs,
		RunE: func(cmd *spcobra.Command, args []string) error {
			formatStr, _ = cmd.Flags().GetString("format")

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

			// 编辑模式不支持 --format
			if edit {
				if formatStr != "table" && formatStr != "" {
					return fmt.Errorf("--format 参数与编辑模式不兼容")
				}
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
				// 目录浏览支持 --format
				tbl := buildBrowseTable(result)
				rawData := toBrowseData(result)

				// 空列表特殊处理
				if tbl == nil && formatStr == "table" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", result.AbsolutePath)
					fmt.Fprintln(cmd.OutOrStdout(), "目录为空")
					return nil
				}

				// 表格模式：先输出绝对路径，再输出表格
				if formatStr == "table" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", result.AbsolutePath)
					return output.Print(cmd.OutOrStdout(), output.Format(formatStr), tbl, rawData)
				}

				return output.Print(cmd.OutOrStdout(), output.Format(formatStr), tbl, rawData)
			case usecase.ConfigTargetFile:
				// 文件内容直接输出，不支持 --format
				if formatStr != "table" && formatStr != "" {
					return fmt.Errorf("--format 参数只支持目录浏览，文件内容直接输出")
				}
				printConfigFile(cmd.OutOrStdout(), result)
			}

			return nil
		},
	}

	command.Flags().BoolVarP(&edit, "edit", "e", false, "Open file in terminal editor")
	command.Flags().StringVarP(&snapshot, "snapshot", "s", "", "浏览指定快照中的文件（传入快照 ID 或名称）")
	command.Flags().String("format", "table", "输出格式: table, json, yaml（仅支持目录浏览）")
	return command
}

// toBrowseData 将目录浏览结果转换为结构化数据（用于 JSON/YAML）
func toBrowseData(result *usecase.GetConfigResult) interface{} {
	entries := make([]map[string]interface{}, len(result.Entries))
	for i, entry := range result.Entries {
		entries[i] = map[string]interface{}{
			"name":   entry.Name,
			"path":   entry.RelativePath,
			"is_dir": entry.IsDir,
		}
	}
	return map[string]interface{}{
		"path":    result.AbsolutePath,
		"entries": entries,
	}
}

// buildBrowseTable 构建目录浏览表格
func buildBrowseTable(result *usecase.GetConfigResult) *output.Table {
	if len(result.Entries) == 0 {
		return nil // 返回 nil 表示无数据
	}

	tbl := &output.Table{
		Columns: []output.Column{
			{Title: "类型"},
			{Title: "名称"},
			{Title: "路径"},
		},
	}
	for _, entry := range result.Entries {
		entryType := "文件"
		path := entry.RelativePath
		if entry.IsDir {
			entryType = "目录"
			if !strings.HasSuffix(path, "/") && !strings.HasSuffix(path, "\\") {
				path += string(filepath.Separator)
			}
		}
		tbl.Rows = append(tbl.Rows, output.Row{
			Cells: []string{entryType, entry.Name, path},
		})
	}
	return tbl
}
