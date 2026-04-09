package output

import "github.com/charmbracelet/lipgloss"

// HeaderStyle 表头样式：加粗。
var HeaderStyle = lipgloss.NewStyle().Bold(true)

// HighlightStyle 高亮样式：蓝色前景，用于当前生效行、状态标签。
var HighlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

// DimStyle 次要信息样式：灰色前景，用于汇总文字。
var DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

// DisplayWidth 计算字符串在终端中的显示宽度。
// 中文字符和全角字符占 2 列宽，其余占 1 列宽。
func DisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 0x7F {
			w += 2
		} else {
			w++
		}
	}
	return w
}
