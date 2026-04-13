# CLI 统一输出层 — 技术设计文档

## 概述

新增 `internal/cli/output/` 包，提供统一的表格渲染能力，替换 `cobra` 命令层中散落的 `fmt.Fprintf` 渲染逻辑。

## 新增文件

```
internal/cli/output/
├── table.go        # 通用表格渲染器
├── style.go        # 全局样式常量
└── table_test.go   # 表格渲染器单元测试
```

## 核心数据结构

### style.go

```go
package output

import "github.com/charmbracelet/lipgloss"

// 全局样式常量，集中管理 CLI 输出的视觉风格。
var (
    // HeaderStyle 表头样式：加粗。
    HeaderStyle = lipgloss.NewStyle().Bold(true)
    // HighlightStyle 高亮样式：蓝色前景，用于当前生效行、状态标签。
    HighlightStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
    // DimStyle 次要信息样式：灰色前景，用于汇总文字。
    DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// DisplayWidth 计算字符串在终端中的显示宽度。
// 中文字符和全角字符占 2 列宽，其余占 1 列宽。
func DisplayWidth(s string) int
```

### table.go

```go
package output

// Column 定义表格列。
type Column struct {
    Title string  // 列标题
    Width int     // 最小列宽，0 表示自动计算
}

// Row 是表格数据行，每个元素对应一列。
type Row struct {
    Cells          []string  // 单元格内容
    Highlight      bool      // 是否高亮整行（蓝色前景）
}

// Table 提供统一的表格渲染能力。
type Table struct {
    Columns    []Column  // 列定义
    Rows       []Row     // 数据行
    Footer     string    // 表格下方的汇总文字（空则不显示）
}

// Render 渲染表格为字符串，包含边框、表头、数据行和汇总。
//
// 边框使用 Unicode box-drawing 字符：
//
//	┌──────┬──────┐
//	│ Col1 │ Col2 │
//	├──────┼──────┤
//	│ val1 │ val2 │
//	└──────┴──────┘
func (t *Table) Render() string
```

## 渲染逻辑

### Table.Render() 算法

1. **计算列宽**：遍历表头和所有行，取每列最大 `DisplayWidth`，加 2（左右内边距各 1 空格）
2. **渲染顶边框**：`┌─` + `─×width` + `┬─` ... `┐`
3. **渲染表头**：`│ ` + HeaderStyle(标题) + padding + `│` ...
4. **渲染分隔线**：`├─` + `─×width` + `┼─` ... `┤`
5. **渲染数据行**：
   - 普通行：`│ ` + 值 + padding + `│`
   - 高亮行：`│ ` + HighlightStyle(值) + padding + `│`
6. **渲染底边框**：`└─` + `─×width` + `┴─` ... `┘`
7. **渲染 Footer**：如果有，追加换行 + DimStyle(footer 文字)

### 中英文混排对齐

复用现有 `displayWidth` 算法（迁移到 `output.DisplayWidth`）：
- ASCII 字符（rune ≤ 0x7F）：宽度 1
- CJK 及其他宽字符（rune > 0x7F）：宽度 2

填充使用空格：`padding = colWidth - DisplayWidth(cell)`。

## 改动清单

### 新增文件

| 文件 | 说明 |
|------|------|
| `internal/cli/output/style.go` | 样式常量 + DisplayWidth |
| `internal/cli/output/table.go` | Table 结构体 + Render 方法 |
| `internal/cli/output/table_test.go` | 表格渲染单元测试 |

### 修改文件

| 文件 | 改动 |
|------|------|
| `internal/cli/cobra/root.go` | 1. 删除 `displayWidth`、`printAlignedRow` 函数<br>2. `printSnapshotList` 改用 `output.Table` 渲染<br>3. `printScanResult` 改用 `output.Table` 渲染<br>4. `printConfigEntries` 保留不变（不是 list 命令） |
| `internal/cli/cobra/scan.go` | `scan list` 子命令添加 `--verbose` / `-v` flag，传递给 `printScanResult` |
| `internal/cli/cobra/setting.go` | `printSettingList` 改用 `output.Table` 渲染 |
| `internal/cli/cobra/root_test.go` | 更新测试断言以匹配新表格格式 |
| `CLAUDE.md` | 更新项目结构，添加 `output/` 目录 |

### 不改动的文件

| 文件 | 原因 |
|------|------|
| `printScanRuleList` | 不是 list 命令，不在本次范围 |
| `printConfigEntries/File` | 不是 list 命令，不在本次范围 |
| `printSettingDetail` | 单条详情，不是列表 |
| `printCreatedSetting` | 创建反馈，不是列表 |
| TUI 相关文件 | 设计明确排除 |

## 各命令改造细节

### snapshot list

当前 `printSnapshotList` 已是表格逻辑，改造最小：

```go
func printSnapshotList(w io.Writer, result *usecase.ListSnapshotsResult) {
    if len(result.Items) == 0 {
        fmt.Fprintln(w, "暂无本地快照")
        return
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
                item.ID,
                item.Name,
                item.Project,
                item.Message,
                fmt.Sprintf("%d", item.FileCount),
                item.CreatedAt.Format("2006-01-02 15:04"),
            },
        })
    }
    fmt.Fprint(w, tbl.Render())
}
```

### setting list

从缩进列表改为表格：

```go
func printSettingList(w io.Writer, result *usecase.ListAISettingsResult) {
    if result.Total == 0 {
        fmt.Fprintln(w, "暂无保存的配置")
        return
    }
    tbl := &output.Table{
        Columns: []output.Column{
            {Title: "名称"},
            {Title: "Base URL"},
            {Title: "Opus 模型"},
            {Title: "Sonnet 模型"},
        },
        Footer: fmt.Sprintf("当前生效配置: %s", result.Current),
    }
    for _, item := range result.Items {
        name := item.Name
        if item.IsCurrent {
            name = "* " + name
        }
        tbl.Rows = append(tbl.Rows, output.Row{
            Cells:     []string{name, item.BaseURL, item.OpusModel, item.SonnetModel},
            Highlight: item.IsCurrent,
        })
    }
    fmt.Fprint(w, tbl.Render())
}
```

### scan list

精简为表格，`--verbose` 时追加详细配置项：

`scan.go` 新增 flag：

```go
var verbose bool
listCommand.Flags().BoolVarP(&verbose, "verbose", "v", false, "显示详细配置项")
```

`root.go` 中 `printScanResult` 改造：

```go
func printScanResult(w io.Writer, result *usecase.ScanResult, verbose bool) {
    // 表格：项目 | 类型 | 配置目录 | 状态 | 可同步项
    tbl := &output.Table{
        Columns: []output.Column{
            {Title: "项目"},
            {Title: "类型"},
            {Title: "配置目录"},
            {Title: "状态"},
            {Title: "可同步项"},
        },
    }
    for _, item := range result.Tools {
        projectName := displayScanSummaryTitle(item)
        toolType := displayToolName(item.Tool)
        status := item.ResultText
        configCount := ""
        if item.ConfigCount > 0 {
            configCount = fmt.Sprintf("%d 项", item.ConfigCount)
        }
        tbl.Rows = append(tbl.Rows, output.Row{
            Cells: []string{projectName, toolType, item.Path, status, configCount},
        })
    }
    fmt.Fprint(w, tbl.Render())

    // verbose 模式：追加每个项目的详细配置项
    if verbose {
        fmt.Fprintln(w)
        for _, item := range result.Tools {
            // 复用现有分组渲染逻辑
            printScanDetail(w, item)
        }
    }
}
```

## 测试策略

### 新增测试

`output/table_test.go`：

| 测试用例 | 验证内容 |
|----------|----------|
| TestTableRenderBasic | 基本表格：边框、表头、数据行 |
| TestTableRenderEmpty | 空行时的渲染 |
| TestTableRenderHighlight | 高亮行是否使用蓝色 |
| TestTableRenderFooter | Footer 是否显示且灰色 |
| TestTableRenderChineseAlignment | 中英文混排对齐 |
| TestDisplayWidth | ASCII=1, CJK=2 |

### 修改测试

`root_test.go` 中的断言需要更新以匹配新表格格式：
- `TestExecuteScanCommandPrintsToolSummary`：检查新表格格式的内容
- `TestExecuteScanListCommandPrintsScanResult`：同上
- `TestExecuteSnapshotListShowsEmptyState`：空状态文本不变，无需改

## 风险与注意事项

1. **Windows 终端兼容性**：Unicode box-drawing 字符在 Windows Terminal 和 PowerShell 7 中均支持，但在旧版 cmd.exe 中可能显示异常。目标用户是开发者，默认使用现代终端，风险可接受。
2. **管道输出**：表格边框在管道输出时也会渲染。当前所有 print 函数都接受 `io.Writer`，不影响管道功能。lipgloss 颜色会自动检测 TTY，非 TTY 时不输出 ANSI 转义码。
3. **测试兼容**：`printScanResult` 函数签名新增 `verbose bool` 参数，所有调用处需同步更新。
