package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"ai-sync-manager/internal/app/usecase"
)

func renderScanPage(m *Model) string {
	var builder strings.Builder
	builder.WriteString("扫描结果\n\n")

	if m.ScanResult == nil || len(m.ScanResult.Tools) == 0 {
		builder.WriteString("暂无扫描结果\n")
	} else {
		for _, item := range m.ScanResult.Tools {
			builder.WriteString(fmt.Sprintf("%s\n", displayScanSummaryTitle(item)))
			builder.WriteString(fmt.Sprintf("  检测结果: %s\n", item.ResultText))
			if item.Path != "" {
				builder.WriteString(fmt.Sprintf("  配置目录: %s\n", item.Path))
			}
			builder.WriteString(fmt.Sprintf("  可同步项: %d 项\n", item.ConfigCount))
			if item.Reason != "" {
				builder.WriteString(fmt.Sprintf("  原因: %s\n", item.Reason))
			}

			if len(item.Items) > 0 {
				builder.WriteString("\n")
				lastGroup := ""
				for _, config := range item.Items {
					if config.Group != "" && config.Group != lastGroup {
						builder.WriteString(fmt.Sprintf("  %s:\n", config.Group))
						lastGroup = config.Group
					}
					label := config.Label
					if strings.TrimSpace(label) == "" {
						label = config.RelativePath
					}
					builder.WriteString(fmt.Sprintf("    - %s: %s\n", label, config.RelativePath))
				}
			}

			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n" + m.Help.View(m.Keys) + "\n")
	return builder.String()
}

func displayToolName(tool string) string {
	switch strings.ToLower(strings.TrimSpace(tool)) {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	default:
		if tool == "" {
			return "未知工具"
		}
		return tool
	}
}

func displayScanSummaryTitle(item usecase.ToolSummary) string {
	if item.Scope == "project" {
		projectName := strings.TrimSpace(item.ProjectName)
		if projectName == "" && strings.TrimSpace(item.Path) != "" {
			projectName = filepath.Base(item.Path)
		}
		if projectName == "" {
			projectName = "未命名项目"
		}
		return fmt.Sprintf("%s（%s 项目）", projectName, displayToolName(item.Tool))
	}
	return displayToolName(item.Tool) + "（全局）"
}
