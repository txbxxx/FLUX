# 表格行间分隔线 + scan rules 全表格化 实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让所有 CLI 表格在数据行之间显示水平分隔线，并将 scan rules 输出改为统一的全表格样式。

**Architecture:** 修改 `output.Table.Render()` 在数据行之间插入分隔线；将 `printScanRuleList` 改为使用 `output.Table` 渲染所有分组。

**Tech Stack:** Go、Bubbletea/Lipgloss（已有样式）

---

### Task 1: 修改 Table.Render() 添加行间分隔线

**Files:**
- Modify: `internal/cli/output/table.go:55-59`
- Test: `internal/cli/output/table_test.go`

**Step 1: 修改 Render() 方法中的数据行循环**

将 `table.go` 第 55-59 行的数据行循环从：

```go
// 第四步：数据行
for _, row := range t.Rows {
    b.WriteString(t.renderRow(row, widths))
    b.WriteByte('\n')
}
```

改为：

```go
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
```

同时更新文件顶部 Table 的 godoc 示例，将：

```go
// 渲染为 Unicode box-drawing 边框表格：
//
//	┌──────┬──────┐
//	│ Col1 │ Col2 │
//	├──────┼──────┤
//	│ val1 │ val2 │
//	└──────┴──────┘
```

改为：

```go
// 渲染为 Unicode box-drawing 边框表格：
//
//	┌──────┬──────┐
//	│ Col1 │ Col2 │
//	├──────┼──────┤
//	│ val1 │ val2 │
//	├──────┼──────┤
//	│ val3 │ val4 │
//	└──────┴──────┘
```

**Step 2: 运行现有测试确认兼容**

Run: `go test ./internal/cli/output/... -v`
Expected: 部分测试可能因行数变化而失败（空表格的行数从 4 变为 4 无变化，因为空表格无数据行，不会新增分隔线）

**Step 3: 更新 table_test.go**

在 `TestTableRenderBasic` 中验证行间分隔线。在测试文件末尾新增测试：

```go
func TestTableRenderRowSeparators(t *testing.T) {
	tbl := &Table{
		Columns: []Column{
			{Title: "名称"},
			{Title: "状态"},
		},
		Rows: []Row{
			{Cells: []string{"a", "ok"}},
			{Cells: []string{"b", "err"}},
			{Cells: []string{"c", "ok"}},
		},
	}
	result := tbl.Render()
	lines := strings.Split(result, "\n")

	// 预期行数：顶边框 + 表头 + 表头分隔线 + 3数据行 + 2行间分隔线 + 底边框 = 9 行
	// (无 footer 时最后一行后无空行，所以 lines 长度为 9)
	expectedLines := 9
	if len(lines) != expectedLines {
		t.Errorf("3行数据应有 %d 行输出, got %d: %v", expectedLines, len(lines), lines)
	}

	// 验证行间分隔线存在（第 5 行和第 7 行，0-indexed 为 4 和 6）
	// 结构：顶边框[0], 表头[1], 表头分隔线[2], 数据1[3], 行间分隔线[4], 数据2[5], 行间分隔线[6], 数据3[7], 底边框[8]
	if !strings.Contains(lines[4], "├") || !strings.Contains(lines[4], "┼") {
		t.Errorf("第1条行间分隔线缺失: %s", lines[4])
	}
	if !strings.Contains(lines[6], "├") || !strings.Contains(lines[6], "┼") {
		t.Errorf("第2条行间分隔线缺失: %s", lines[6])
	}
	// 底边框应为 └──┴──┘
	if !strings.Contains(lines[8], "└") || !strings.Contains(lines[8], "┴") {
		t.Errorf("底边框不正确: %s", lines[8])
	}
}

func TestTableRenderSingleRowNoSeparator(t *testing.T) {
	tbl := &Table{
		Columns: []Column{
			{Title: "名称"},
		},
		Rows: []Row{
			{Cells: []string{"only"}},
		},
	}
	result := tbl.Render()
	// 单行数据不应有行间分隔线
	// 结构：顶边框 + 表头 + 表头分隔线 + 1数据行 + 底边框 = 5 行
	lines := strings.Split(result, "\n")
	expectedLines := 5
	if len(lines) != expectedLines {
		t.Errorf("1行数据应有 %d 行输出, got %d: %v", expectedLines, len(lines), lines)
	}
}
```

**Step 4: 运行全部测试**

Run: `go test ./internal/cli/output/... -v`
Expected: PASS

**Step 5: 构建并手动验证**

Run: `make build && ./bin/ai-sync.exe setting list`
Expected: 数据行之间出现 `├──┼──┤` 分隔线

Run: `./bin/ai-sync.exe snapshot list`
Expected: 同上

**Step 6: 提交**

```bash
git add internal/cli/output/table.go internal/cli/output/table_test.go
git commit -m "feat: 表格数据行间添加水平分隔线"
```

---

### Task 2: 改造 scan rules 输出为全表格形式

**Files:**
- Modify: `internal/cli/cobra/root.go:196-230`（`printScanRuleList` 函数）

**Step 1: 重写 printScanRuleList 函数**

将 `root.go` 中的 `printScanRuleList` 函数替换为：

```go
// printScanRuleList 把默认规则、自定义规则和项目规则用表格渲染输出。
func printScanRuleList(w io.Writer, result *usecase.ListScanRulesResult) {
	if strings.TrimSpace(result.App) != "" {
		fmt.Fprintf(w, "%s 规则\n\n", HeaderStyle.Render(displayToolName(result.App)))
	}

	// 第一步：默认全局规则
	fmt.Fprintln(w, HeaderStyle.Render("默认全局规则:"))
	tbl := &output.Table{
		Columns: []output.Column{{Title: "路径"}},
	}
	for _, item := range result.DefaultGlobalRules {
		tbl.Rows = append(tbl.Rows, output.Row{Cells: []string{item.Path}})
	}
	fmt.Fprint(w, tbl.Render())

	// 第二步：已注册项目扫描模板
	if len(result.RegisteredProjects) > 0 && len(result.ProjectRuleTemplates) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, HeaderStyle.Render("已注册项目扫描模板:"))
		tblTemplate := &output.Table{
			Columns: []output.Column{{Title: "路径"}},
		}
		for _, item := range result.ProjectRuleTemplates {
			tblTemplate.Rows = append(tblTemplate.Rows, output.Row{Cells: []string{item.Path}})
		}
		fmt.Fprint(w, tblTemplate.Render())
	}

	// 第三步：自定义规则
	fmt.Fprintln(w)
	fmt.Fprintln(w, HeaderStyle.Render("自定义规则:"))
	tblCustom := &output.Table{
		Columns: []output.Column{{Title: "路径"}},
	}
	if len(result.CustomRules) == 0 {
		tblCustom.Rows = append(tblCustom.Rows, output.Row{Cells: []string{"暂无"}})
	} else {
		for _, item := range result.CustomRules {
			tblCustom.Rows = append(tblCustom.Rows, output.Row{Cells: []string{item.Path}})
		}
	}
	fmt.Fprint(w, tblCustom.Render())

	// 第四步：已注册项目
	fmt.Fprintln(w)
	fmt.Fprintln(w, HeaderStyle.Render("已注册项目:"))
	tblProjects := &output.Table{
		Columns: []output.Column{{Title: "名称"}, {Title: "路径"}},
	}
	if len(result.RegisteredProjects) == 0 {
		tblProjects.Rows = append(tblProjects.Rows, output.Row{Cells: []string{"暂无", ""}})
	} else {
		for _, item := range result.RegisteredProjects {
			tblProjects.Rows = append(tblProjects.Rows, output.Row{Cells: []string{item.Name, item.Path}})
		}
	}
	fmt.Fprint(w, tblProjects.Render())
}
```

注意：需要确认 `HeaderStyle` 在 cobra 包中可用（它定义在 `output` 包中），需要在 import 中已有 `output` 包（已有），使用 `output.HeaderStyle`。

修正上面的代码，标题加粗应使用 `output.HeaderStyle`：

```go
fmt.Fprintln(w, output.HeaderStyle.Render("默认全局规则:"))
```

**Step 2: 构建并手动验证**

Run: `make build && ./bin/ai-sync.exe scan rules`
Expected: 所有分组以表格形式展示，行间有分隔线

**Step 3: 运行全部测试**

Run: `go test ./... `
Expected: PASS

**Step 4: 提交**

```bash
git add internal/cli/cobra/root.go
git commit -m "feat: scan rules 输出改为全表格样式"
```

---

### Task 3: 清理设计文档并最终验证

**Step 1: 运行全部测试**

Run: `go test ./... -cover`
Expected: PASS，覆盖率不下降

**Step 2: 手动验证所有受影响的命令**

Run:
```bash
./bin/ai-sync.exe setting list
./bin/ai-sync.exe snapshot list
./bin/ai-sync.exe scan list
./bin/ai-sync.exe scan rules
```

Expected: 所有命令表格行间有分隔线，scan rules 为全表格输出

**Step 3: 删除设计文档（可选）或保留**

设计文档已在 `docs/plans/2026-04-09-table-row-separator-design.md`，保留作为记录。
