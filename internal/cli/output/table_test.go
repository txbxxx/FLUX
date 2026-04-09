package output

import (
	"strings"
	"testing"
)

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"", 0},
		{"abc123", 6},
		{"中文", 4},
		{"hello世界", 9},
		{"可同步", 6},
		{"a中b", 4},
	}
	for _, tt := range tests {
		got := DisplayWidth(tt.input)
		if got != tt.want {
			t.Errorf("DisplayWidth(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestTableRenderBasic(t *testing.T) {
	tbl := &Table{
		Columns: []Column{
			{Title: "名称"},
			{Title: "状态"},
		},
		Rows: []Row{
			{Cells: []string{"test", "ok"}},
		},
	}

	result := tbl.Render()

	if !strings.Contains(result, "┌") || !strings.Contains(result, "└") {
		t.Errorf("缺少边框字符: %s", result)
	}
	if !strings.Contains(result, "名称") || !strings.Contains(result, "状态") {
		t.Errorf("缺少表头: %s", result)
	}
	if !strings.Contains(result, "test") || !strings.Contains(result, "ok") {
		t.Errorf("缺少数据行: %s", result)
	}
}

func TestTableRenderEmpty(t *testing.T) {
	tbl := &Table{
		Columns: []Column{
			{Title: "Col"},
		},
	}
	result := tbl.Render()
	// 顶边框、表头、分隔线、底边框 = 4 行
	lines := strings.Split(result, "\n")
	if len(lines) != 4 {
		t.Errorf("空行表格应有4行, got %d: %v", len(lines), lines)
	}
}

func TestTableRenderHighlight(t *testing.T) {
	tbl := &Table{
		Columns: []Column{
			{Title: "名称"},
		},
		Rows: []Row{
			{Cells: []string{"normal"}, Highlight: false},
			{Cells: []string{"*active"}, Highlight: true},
		},
	}
	result := tbl.Render()

	if !strings.Contains(result, "normal") {
		t.Errorf("缺少普通行: %s", result)
	}
	if !strings.Contains(result, "*active") {
		t.Errorf("缺少高亮行: %s", result)
	}
}

func TestTableRenderFooter(t *testing.T) {
	tbl := &Table{
		Columns: []Column{
			{Title: "ID"},
		},
		Rows: []Row{
			{Cells: []string{"abc"}},
		},
		Footer: "共 1 条记录",
	}
	result := tbl.Render()
	if !strings.Contains(result, "共 1 条记录") {
		t.Errorf("缺少 Footer: %s", result)
	}
}

func TestTableRenderChineseAlignment(t *testing.T) {
	tbl := &Table{
		Columns: []Column{
			{Title: "名称"},
			{Title: "状态"},
		},
		Rows: []Row{
			{Cells: []string{"claude-global", "可同步"}},
			{Cells: []string{"codex-global", "不可同步"}},
		},
	}
	result := tbl.Render()

	// 验证中英文混排对齐：所有行中 │ 出现的显示位置应一致
	lines := strings.Split(result, "\n")
	pipeLines := []string{}
	for _, line := range lines {
		if strings.Contains(line, "│") {
			pipeLines = append(pipeLines, line)
		}
	}

	if len(pipeLines) < 3 {
		t.Fatalf("至少应有3行含竖线(头+2数据), got %d: %v", len(pipeLines), pipeLines)
	}

	// 基于 display width 检查每行中 │ 的位置是否一致
	ref := displayPipePositions(pipeLines[0])
	for i, line := range pipeLines[1:] {
		pos := displayPipePositions(line)
		if len(pos) != len(ref) {
			t.Errorf("行%d 竖线数量不一致: %v vs %v\n%s\n%s", i+1, pos, ref, pipeLines[0], line)
			continue
		}
		for j := range pos {
			if pos[j] != ref[j] {
				t.Errorf("行%d 竖线不对齐 at %d: %d vs %d\n%s\n%s",
					i+1, j, pos[j], ref[j], pipeLines[0], line)
				break
			}
		}
	}
}

func TestTableRenderNoColumns(t *testing.T) {
	tbl := &Table{}
	result := tbl.Render()
	if result != "" {
		t.Errorf("无列时返回空字符串, got: %q", result)
	}
}

func TestTableRenderMultipleRowsAlignment(t *testing.T) {
	tbl := &Table{
		Columns: []Column{
			{Title: "项目"},
			{Title: "类型"},
			{Title: "可同步项"},
		},
		Rows: []Row{
			{Cells: []string{"claude-global", "Claude", "6 项"}},
			{Cells: []string{"codex-global", "Codex", "4 项"}},
		},
	}
	result := tbl.Render()

	// 验证所有行的竖线在显示宽度上对齐
	lines := strings.Split(result, "\n")
	pipeLines := []string{}
	for _, line := range lines {
		if strings.Contains(line, "│") {
			pipeLines = append(pipeLines, line)
		}
	}
	if len(pipeLines) < 3 {
		t.Fatalf("至少应有3行含竖线(头+2数据), got %d", len(pipeLines))
	}

	ref := displayPipePositions(pipeLines[0])
	for i, line := range pipeLines[1:] {
		pos := displayPipePositions(line)
		if len(pos) != len(ref) {
			t.Errorf("行%d 竖线数量不一致: %v vs %v", i+1, pos, ref)
			continue
		}
		for j := range pos {
			if pos[j] != ref[j] {
				t.Errorf("行%d 竖线不对齐 at %d: %d vs %d\n%s\n%s",
					i+1, j, pos[j], ref[j], pipeLines[0], line)
				break
			}
		}
	}
}

// displayPipePositions 返回每行中 │ 出现的显示列位置（忽略 ANSI 转义码）。
func displayPipePositions(s string) []int {
	var pos []int
	col := 0
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		if r == '│' {
			pos = append(pos, col)
		}
		if r > 0x7F {
			col += 2
		} else {
			col++
		}
	}
	return pos
}
