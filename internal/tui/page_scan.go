package tui

import (
	"fmt"
	"strings"
)

func renderScanPage(m *Model) string {
	var builder strings.Builder
	builder.WriteString("扫描结果\n\n")

	if m.ScanResult == nil || len(m.ScanResult.Tools) == 0 {
		builder.WriteString("暂无扫描结果\n")
	} else {
		for _, item := range m.ScanResult.Tools {
			builder.WriteString(fmt.Sprintf("%s | %s | %d\n", item.Tool, item.Status, item.ConfigCount))
		}
	}

	builder.WriteString("\n" + m.Help.View(m.Keys) + "\n")
	return builder.String()
}
