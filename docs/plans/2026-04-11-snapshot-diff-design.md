# 快照对比功能技术设计

> 版本：v1.0 | 日期：2026-04-11

## 1. 概述

基于 PRD 要求，实现 `snapshot diff` 命令及 `restore --dry-run` 改造。底层复用已有的 `comparator.go`，主要工作在 CLI 层参数解析和输出渲染。

## 2. 现有基础

### comparator.go 已有能力

| 方法 | 功能 | 可直接复用 |
|------|------|-----------|
| `CompareSnapshots()` | 比较两个快照，返回 ChangeSummary | 是 |
| `CompareWithFileSystem()` | 比较快照与文件系统 | 是 |
| `GetFileChanges()` | 获取详细 FileComparison 列表 | 是 |
| `GetDiff()` | 简单行级 diff | 需增强（加上下文行数控制） |
| `FindChangedFiles()` | 按时间查找变更文件 | 部分复用 |

### types/snapshot 已有类型

- `ChangeSummary`：按状态/工具/分类统计变更数
- `RestoreResult`：恢复结果（含 AppliedFiles、SkippedFiles、Errors）

## 3. 架构设计

### 3.1 分层调用链

```
CLI (snapshot_diff.go)
  → UseCase (local_workflow.go: DiffSnapshots)
    → Service (comparator.go: CompareSnapshots / CompareWithFileSystem)
      → DAO (snapshot.go: 加载快照文件)
    → Service (comparator.go: GetFileChanges / GetDiff)
```

### 3.2 新增文件清单

| 文件 | 职责 |
|------|------|
| `internal/cli/cobra/snapshot_diff.go` | diff 命令定义、参数解析 |
| `internal/cli/output/diff.go` | diff 输出渲染（摘要、unified、并排） |
| `internal/types/snapshot/diff.go` | diff 相关返回结构体 |

### 3.3 修改文件清单

| 文件 | 修改内容 |
|------|---------|
| `internal/cli/cobra/snapshot.go` | 注册 diff 子命令 |
| `internal/app/usecase/local_workflow.go` | 新增 DiffSnapshots 方法 |
| `internal/cli/cobra/snapshot_restore.go` | dry-run 输出改造，复用 diff 渲染 |
| `internal/service/snapshot/comparator.go` | GetDiff 增加上下文行数参数 |

## 4. 数据结构

### 4.1 types/snapshot/diff.go

```go
package snapshot

// DiffInput is the input for snapshot diff operations.
type DiffInput struct {
    SourceID    string   // 源快照 ID 或名称
    TargetID    string   // 目标快照 ID 或名称（空则对比文件系统）
    Verbose     bool     // 是否显示内容级 diff
    SideBySide  bool     // 是否并排显示
    Tool        string   // 按工具类型过滤（空则不过滤）
    PathPattern string   // 按路径模式过滤（空则不过滤）
    Color       string   // 颜色控制：always/auto/never
    Context     int      // 上下文行数（默认 5）
}

// DiffFileChange represents a single file's diff result.
type DiffFileChange struct {
    Path     string       // 文件相对路径
    Status   FileStatus   // added / modified / deleted / unchanged
    IsBinary bool         // 是否二进制文件
    OldSize  int64        // 变更前大小（字节）
    NewSize  int64        // 变更后大小（字节）
    AddLines int          // 新增行数（二进制文件为 0）
    DelLines int          // 删除行数（二进制文件为 0）
    ToolType string       // 所属工具类型
    Hunks    []DiffHunk   // 差异块（仅 -v 模式填充）
}

// FileStatus represents the change status of a file.
type FileStatus string

const (
    FileAdded     FileStatus = "added"
    FileModified  FileStatus = "modified"
    FileDeleted   FileStatus = "deleted"
    FileUnchanged FileStatus = "unchanged"
)

// DiffHunk represents a contiguous block of changes.
type DiffHunk struct {
    OldStart int         // 旧文件起始行号
    OldCount int         // 旧文件行数
    NewStart int         // 新文件起始行号
    NewCount int         // 新文件行数
    Lines    []DiffLine  // 差异行（含上下文）
}

// DiffLine represents a single line in a diff hunk.
type DiffLine struct {
    Type     DiffLineType // 行类型
    Content  string       // 行内容
    OldLineNo int         // 旧文件行号（上下文和删除行有效）
    NewLineNo int         // 新文件行号（上下文和新增行有效）
}

// DiffLineType represents the type of a diff line.
type DiffLineType int

const (
    LineContext DiffLineType = iota // 上下文行
    LineAdded                       // 新增行
    LineDeleted                     // 删除行
)

// DiffResult is the complete result of a diff operation.
type DiffResult struct {
    SourceName string          // 源快照名称
    TargetName string          // 目标快照名称（或 "当前文件系统"）
    Files      []DiffFileChange // 变更文件列表
    Stats      DiffStats       // 变更统计
    HasDiff    bool            // 是否存在差异
}

// DiffStats holds aggregate statistics for a diff.
type DiffStats struct {
    TotalFiles  int // 变更文件总数
    AddedFiles  int // 新增文件数
    ModifiedFiles int // 修改文件数
    DeletedFiles int // 删除文件数
    AddLines    int // 总新增行数
    DelLines    int // 总删除行数
}
```

### 4.2 UseCase 层输入

```go
// DiffSnapshotsInput is the input for the DiffSnapshots use case.
type DiffSnapshotsInput struct {
    SourceID    string
    TargetID    string   // 空则对比文件系统
    Verbose     bool
    SideBySide  bool
    Tool        string
    PathPattern string
    Context     int      // 上下文行数，默认 5
}
```

## 5. 核心流程

### 5.1 diff 命令主流程

```
snapshot_diff.go (CLI 层)
│
├── 解析参数：source-id, target-id, -v, --side-by-side, --tool, --path, --color
│
├── 构建 DiffSnapshotsInput
│
├── 调用 Workflow.DiffSnapshots(ctx, input)
│   │
│   ├── 解析快照 ID/名称 → 获取 Snapshot 实体
│   │   ├── targetID 为空 → CompareWithFileSystem()
│   │   └── targetID 非空 → CompareSnapshots()
│   │
│   ├── 获取 FileChanges 列表
│   │
│   ├── 应用过滤（--tool、--path）
│   │
│   └── verbose=true 时计算每个文件的 Hunks
│       └── comparator.GetDiff(oldContent, newContent, contextLines)
│
├── 构建 DiffResult
│
└── 渲染输出
    ├── 非 verbose → renderDiffSummary(result)
    ├── verbose + 非 side-by-side → renderUnifiedDiff(result)
    └── verbose + side-by-side → renderSideBySideDiff(result)
```

### 5.2 restore --dry-run 改造流程

```
snapshot_restore.go (现有 dry-run 分支)
│
├── 调用 Workflow.RestoreSnapshot(ctx, input) — 现有逻辑不变
│
└── 替换 printRestorePreview → 使用 diff 渲染
    ├── 构建 DiffInput（sourceID=快照, targetID=空, verbose=true）
    ├── 调用 Workflow.DiffSnapshots 获取 DiffResult
    ├── 调用 renderUnifiedDiff(result) 输出内容差异
    └── 追加恢复摘要（覆盖/跳过文件数、备份路径）
```

## 6. 输出渲染

### 6.1 renderDiffSummary

输出 `git diff --stat` 风格摘要，使用 `lipgloss` 着色：

```
快照对比: my-config-v1 → my-config-v2

 claude/settings.json | 3 +--
 claude/CLAUDE.md     | 12 +++++++-----
 agents/review.md     | 8 ++++++++
 plugins/old.json     | 45 -------------------------
 5 files changed, 15 insertions(+), 50 deletions(-)
```

- 文件名根据状态着色（绿=新增，红=删除，白=修改）
- 二进制文件显示 `Binary file changed (1.2KB → 2.1KB)`

### 6.2 renderUnifiedDiff

标准 unified diff 格式，5 行上下文：

```
diff --snapshot a/claude/settings.json b/claude/settings.json
--- a/claude/settings.json
+++ b/claude/settings.json
@@ -3,10 +3,10 @@
 {
   "version": 2,
-  "model": "sonnet",
+  "model": "opus",
```

着色规则：
- `@@ ... @@` → 青色
- `-` 开头行 → 红色
- `+` 开头行 → 绿色
- 上下文行 → 白色

### 6.3 renderSideBySide

```
--- claude/settings.json ---                    --- claude/settings.json ---
  1 {                                            1 {
  2   "version": 2,                              2   "version": 2,
  3   "model": "sonnet",                       | 3   "model": "opus",
```

- 检测终端宽度 `os.Stdout.Stat()` 或 term 库
- 左右各 `(width - separator) / 2`
- 差异行着色 + `|` 分隔符
- 窄终端（< 80 列）输出提示信息

## 7. comparator.go 改造

现有 `GetDiff` 方法需增强：

```go
// 改造前
func (c *Comparator) GetDiff(oldContent, newContent []byte) []string

// 改造后
func (c *Comparator) GetDiff(oldContent, newContent []byte, contextLines int) []DiffHunk
```

- 新增 `contextLines` 参数，控制上下文行数（默认 5）
- 返回结构化的 `[]DiffHunk` 而非 `[]string`，便于渲染层控制格式

## 8. diff 算法选型

使用 Go 标准库或轻量第三方库实现行级 diff：

| 方案 | 优点 | 缺点 |
|------|------|------|
| `go.diff` (sergi/go-diff) | 成熟稳定、Myers diff 算法 | 额外依赖 |
| 自实现简单 LCS | 无外部依赖 | 功能有限、无 hunks 支持 |

**推荐**：使用 `github.com/sergi/go-diff/diffmatchpatch` 或 `github.com/hexops/gotextdiff`。

`gotextdiff` 更轻量且原生支持 unified diff 输出，适合本场景。

## 9. 错误处理

| 场景 | 处理 |
|------|------|
| 快照 ID/名称不存在 | 退出码 2，输出"快照不存在" |
| 两个快照无关联（不同 project） | 允许对比，但提示"不同项目的快照" |
| 文件系统读取失败（快照vs磁盘） | 跳过该文件，标记为错误 |
| 终端宽度不足（并排模式） | 输出提示"终端宽度不足，建议使用 -v 模式" |
| 二进制文件 | 标记 `Binary files differ`，不抛错 |

## 10. 测试策略

| 测试类型 | 覆盖范围 |
|---------|---------|
| 单元测试 | comparator 增强（contextLines、结构化输出） |
| 单元测试 | diff 过滤逻辑（--tool、--path） |
| 单元测试 | 输出渲染（摘要、unified、并排） |
| 集成测试 | CLI 命令端到端（创建快照 → diff → 验证输出） |
| 集成测试 | restore --dry-run 输出验证 |
