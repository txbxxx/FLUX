package tui

import (
	"fmt"
	"strings"
)

func renderSnapshotsPage(m *Model) string {
	var builder strings.Builder
	builder.WriteString("快照列表\n\n")

	if len(m.Snapshots) == 0 {
		builder.WriteString("还没有快照，返回首页创建一个吧\n")
	} else {
		for _, item := range m.Snapshots {
			builder.WriteString(fmt.Sprintf("%s | %s\n", item.ID, item.Message))
		}
	}

	builder.WriteString("\n" + m.Help.View(m.Keys) + "\n")
	return builder.String()
}
