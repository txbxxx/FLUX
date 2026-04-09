package output

import (
	"strings"
)

// Column 定义表格列。
type Column struct {
	Title string // 列标题
	Width int    // 最小列宽，0 表示自动计算
}

// Row 是表格数据行。
type Row struct {
	Cells     []string // 单元格内容
	Highlight bool     // 是否高亮整行（蓝色前景）
}

// Table 提供统一的表格渲染能力。
//
// 渲染为 Unicode box-drawing 边框表格：
//
//	┌──────┬──────┐
//	│ Col1 │ Col2 │
//	├──────┼──────┤
//	│ val1 │ val2 │
//	├──────┼──────┤
//	│ val3 │ val4 │
//	└──────┴──────┘
type Table struct {
	Columns []Column // 列定义
	Rows    []Row    // 数据行
	Footer  string   // 表格下方的汇总文字，空则不显示
}

// Render 将表格渲染为字符串，包含边框、表头、数据行和汇总。
func (t *Table) Render() string {
	if len(t.Columns) == 0 {
		return ""
	}

	widths := t.calcWidths()
	var b strings.Builder

	// 第一步：顶边框
	b.WriteString(t.renderBorder("┌", "┬", "┐", "─", widths))
	b.WriteByte('\n')

	// 第二步：表头
	b.WriteString(t.renderHeader(widths))
	b.WriteByte('\n')

	// 第三步：分隔线
	b.WriteString(t.renderBorder("├", "┼", "┤", "─", widths))
	b.WriteByte('\n')

	// 第四步：数据行（行间加分隔线）
	for i, row := range t.Rows {
		b.WriteString(t.renderRow(row, widths))
		b.WriteByte('\n')
		// 最后一行不加分隔线，底边框会紧接其后
		if i < len(t.Rows)-1 {
			b.WriteString(t.renderBorder("├", "┼", "┤", "─", widths))
			b.WriteByte('\n')
		}
	}

	// 第五步：底边框
	b.WriteString(t.renderBorder("└", "┴", "┘", "─", widths))

	// 第六步：Footer
	if t.Footer != "" {
		b.WriteByte('\n')
		b.WriteString(DimStyle.Render(t.Footer))
	}

	return b.String()
}

// calcWidths 计算每列实际宽度。
// 取表头标题宽度与所有行中该列最大宽度的最大值，再加 2（左右内边距各 1）。
func (t *Table) calcWidths() []int {
	widths := make([]int, len(t.Columns))
	for i, col := range t.Columns {
		widths[i] = DisplayWidth(col.Title)
		if col.Width > widths[i] {
			widths[i] = col.Width
		}
	}
	for _, row := range t.Rows {
		for i, cell := range row.Cells {
			if i < len(widths) {
				if dw := DisplayWidth(cell); dw > widths[i] {
					widths[i] = dw
				}
			}
		}
	}
	// 加内边距
	for i := range widths {
		widths[i] += 2
	}
	return widths
}

// renderBorder 渲染水平边框线。
func (t *Table) renderBorder(left, mid, right, fill string, widths []int) string {
	var b strings.Builder
	b.WriteString(left)
	for i, w := range widths {
		if i > 0 {
			b.WriteString(mid)
		}
		b.WriteString(strings.Repeat(fill, w))
	}
	b.WriteString(right)
	return b.String()
}

// renderHeader 渲染表头行，标题加粗。
func (t *Table) renderHeader(widths []int) string {
	var b strings.Builder
	b.WriteString("│")
	for i, col := range t.Columns {
		title := HeaderStyle.Render(col.Title)
		padding := widths[i] - DisplayWidth(col.Title)
		b.WriteString(" ")
		b.WriteString(title)
		b.WriteString(strings.Repeat(" ", padding-1))
		b.WriteString("│")
	}
	return b.String()
}

// renderRow 渲染数据行，高亮行使用蓝色前景。
func (t *Table) renderRow(row Row, widths []int) string {
	var b strings.Builder
	b.WriteString("│")
	for i, w := range widths {
		cell := ""
		if i < len(row.Cells) {
			cell = row.Cells[i]
		}
		displayCell := cell
		if row.Highlight {
			displayCell = HighlightStyle.Render(cell)
		}
		padding := w - DisplayWidth(cell)
		b.WriteString(" ")
		b.WriteString(displayCell)
		b.WriteString(strings.Repeat(" ", padding-1))
		b.WriteString("│")
	}
	return b.String()
}
