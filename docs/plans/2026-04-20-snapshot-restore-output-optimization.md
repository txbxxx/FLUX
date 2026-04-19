# snapshot restore 输出优化 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 优化 `snapshot restore` 输出，移除跳过文件的逐行列出，改为分类聚合展示，解决刷屏问题。

**Architecture:** 纯 CLI 层改动。修改 `printRestorePreview` 函数，将 AppliedFiles 按新增/更新分类展示（每类最多 5 个，超出显示汇总），移除 SkippedFiles 逐行输出改为仅显示数量。`printRestoreResult` 已是摘要格式无需修改。参考 sync pull 的目录聚合模式保持输出风格一致。

**Tech Stack:** Go, 标准库 fmt/io/strings

---

### Task 1: 为 printRestorePreview 编写失败测试

**Files:**
- Create: `internal/cli/cobra/snapshot_restore_test.go`
- Reference: `internal/cli/cobra/snapshot_restore.go:135-153` (当前 printRestorePreview)
- Reference: `internal/types/snapshot/restore.go` (RestoreResult 类型)
- Reference: `internal/types/snapshot/apply.go` (AppliedFile/SkippedFile 类型)

**Step 1: 编写测试文件，覆盖以下场景**

```go
package cobra

import (
	"bytes"
	"strings"
	"testing"

	typesSnapshot "flux/internal/types/snapshot"
)

// 辅助函数：构建包含指定数量 AppliedFile/SkippedFile 的 RestoreResult
func makeRestoreResult(appliedCount, skippedCount int) *typesSnapshot.RestoreResult {
	result := &typesSnapshot.RestoreResult{
		SnapshotID:    "snap-001",
		SnapshotName:  "test-snapshot",
		AppliedCount:  appliedCount,
		SkippedCount:  skippedCount,
		AppliedFiles:  make([]typesSnapshot.AppliedFile, appliedCount),
		SkippedFiles:  make([]typesSnapshot.SkippedFile, skippedCount),
	}
	for i := 0; i < appliedCount; i++ {
		action := "updated"
		if i%2 == 0 {
			action = "created"
		}
		result.AppliedFiles[i] = typesSnapshot.AppliedFile{
			Path:   fmt.Sprintf("dir%d/file%d.txt", i/3, i),
			Action: action,
		}
	}
	for i := 0; i < skippedCount; i++ {
		result.SkippedFiles[i] = typesSnapshot.SkippedFile{
			Path:   fmt.Sprintf("skip/file%d.txt", i),
			Reason: "内容相同",
		}
	}
	return result
}

func TestPrintRestorePreview_HidesSkippedFiles(t *testing.T) {
	var buf bytes.Buffer
	result := makeRestoreResult(2, 100) // 2 个应用 + 100 个跳过
	printRestorePreview(&buf, result)
	output := buf.String()

	// 不应逐行列出跳过文件
	if strings.Contains(output, "skip/file0.txt") {
		t.Errorf("should not list individual skipped files, got: %s", output)
	}
	// 应显示跳过数量
	if !strings.Contains(output, "100") {
		t.Errorf("should show skipped count, got: %s", output)
	}
}

func TestPrintRestorePreview_ShowsAppliedFilesCategorized(t *testing.T) {
	var buf bytes.Buffer
	result := makeRestoreResult(3, 5)
	printRestorePreview(&buf, result)
	output := buf.String()

	// 应显示分类标签：新增和更新
	if !strings.Contains(output, "新增") && !strings.Contains(output, "更新") {
		t.Errorf("should show action categories, got: %s", output)
	}
}

func TestPrintRestorePreview_LimitsAppliedFilesPerCategory(t *testing.T) {
	var buf bytes.Buffer
	// 10 个文件全部是 created，应只显示前 5 个 + 汇总
	result := &typesSnapshot.RestoreResult{
		SnapshotID:   "snap-001",
		SnapshotName: "test-snapshot",
		AppliedCount: 10,
		SkippedCount: 0,
		AppliedFiles: make([]typesSnapshot.AppliedFile, 10),
	}
	for i := 0; i < 10; i++ {
		result.AppliedFiles[i] = typesSnapshot.AppliedFile{
			Path:   fmt.Sprintf("file%d.txt", i),
			Action: "created",
		}
	}
	printRestorePreview(&buf, result)
	output := buf.String()

	// 应显示前 5 个文件
	if !strings.Contains(output, "file0.txt") {
		t.Errorf("should show first files, got: %s", output)
	}
	// 应显示汇总（"... 等 N 个"）
	if !strings.Contains(output, "等") {
		t.Errorf("should show summary for excess files, got: %s", output)
	}
}

func TestPrintRestorePreview_AllFilesSkipped(t *testing.T) {
	var buf bytes.Buffer
	result := &typesSnapshot.RestoreResult{
		SnapshotID:    "snap-001",
		SnapshotName:  "test-snapshot",
		AppliedCount:  0,
		SkippedCount:  50,
		AppliedFiles:  nil,
		SkippedFiles:  make([]typesSnapshot.SkippedFile, 50),
	}
	for i := 0; i < 50; i++ {
		result.SkippedFiles[i] = typesSnapshot.SkippedFile{
			Path:   fmt.Sprintf("skip%d.txt", i),
			Reason: "内容相同",
		}
	}
	printRestorePreview(&buf, result)
	output := buf.String()

	// AppliedCount == 0 应走 "所有文件内容相同，无需恢复" 分支
	if !strings.Contains(output, "所有文件内容相同") {
		t.Errorf("should show 'all identical' message, got: %s", output)
	}
}

func TestPrintRestorePreview_SmallResultSetShowsAll(t *testing.T) {
	var buf bytes.Buffer
	// 少量文件时应全部显示，无需汇总
	result := &typesSnapshot.RestoreResult{
		SnapshotID:   "snap-001",
		SnapshotName: "test-snapshot",
		AppliedCount: 3,
		SkippedCount: 2,
		AppliedFiles: []typesSnapshot.AppliedFile{
			{Path: "a.txt", Action: "created"},
			{Path: "b.txt", Action: "updated"},
			{Path: "c.txt", Action: "created"},
		},
		SkippedFiles: []typesSnapshot.SkippedFile{
			{Path: "d.txt", Reason: "内容相同"},
			{Path: "e.txt", Reason: "内容相同"},
		},
	}
	printRestorePreview(&buf, result)
	output := buf.String()

	// 所有 AppliedFiles 都应显示
	for _, f := range []string{"a.txt", "b.txt", "c.txt"} {
		if !strings.Contains(output, f) {
			t.Errorf("should show file %s for small result set, got: %s", f, output)
		}
	}
	// SkippedFiles 不应逐行显示
	if strings.Contains(output, "d.txt") {
		t.Errorf("should not list individual skipped files, got: %s", output)
	}
	// 但应显示跳过数量
	if !strings.Contains(output, "2") {
		t.Errorf("should show skipped count, got: %s", output)
	}
}
```

**Step 2: 运行测试确认失败**

Run: `go test ./internal/cli/cobra/... -run TestPrintRestorePreview -v`
Expected: FAIL — 当前 printRestorePreview 会逐行列出 SkippedFiles，与断言矛盾

**Step 3: Commit 测试**

```bash
git add internal/cli/cobra/snapshot_restore_test.go
git commit -m "test: 添加 snapshot restore 输出优化的失败测试"
```

---

### Task 2: 实现 printRestorePreview 输出优化

**Files:**
- Modify: `internal/cli/cobra/snapshot_restore.go:135-153` (printRestorePreview 函数)

**Step 1: 重写 printRestorePreview 函数**

将 `snapshot_restore.go:135-153` 的 `printRestorePreview` 替换为：

```go
// maxFilesPerCategory controls how many files to show per action category
// before collapsing into a summary line.
const maxFilesPerCategory = 5

// printRestorePreview renders the dry-run preview output.
// AppliedFiles are grouped by action (新增/更新), each category shows at most
// maxFilesPerCategory entries with a summary for the rest.
// SkippedFiles are shown as a count only — listing hundreds of identical files
// is noise, not signal.
func printRestorePreview(w io.Writer, result *typesSnapshot.RestoreResult) {
	fmt.Fprintf(w, "快照: %s (%s)\n", result.SnapshotName, result.SnapshotID)
	if result.AppliedCount == 0 {
		fmt.Fprintln(w, "所有文件内容相同，无需恢复。")
		return
	}

	// Group applied files by action.
	created, updated := groupByAction(result.AppliedFiles)

	fmt.Fprintf(w, "即将恢复 %d 个文件:\n", result.AppliedCount)

	printFileCategory(w, "新增", created)
	printFileCategory(w, "更新", updated)

	if result.SkippedCount > 0 {
		fmt.Fprintf(w, "  跳过: %d 个文件\n", result.SkippedCount)
	}
}

// groupByAction splits applied files into "created" and "updated" slices.
func groupByAction(files []typesSnapshot.AppliedFile) (created, updated []typesSnapshot.AppliedFile) {
	for _, f := range files {
		if f.Action == "created" {
			created = append(created, f)
		} else {
			updated = append(updated, f)
		}
	}
	return
}

// printFileCategory prints files under a labeled category.
// Shows at most maxFilesPerCategory entries; excess files are summarized.
func printFileCategory(w io.Writer, label string, files []typesSnapshot.AppliedFile) {
	if len(files) == 0 {
		return
	}

	visible := files
	remaining := 0
	if len(files) > maxFilesPerCategory {
		visible = files[:maxFilesPerCategory]
		remaining = len(files) - maxFilesPerCategory
	}

	for _, f := range visible {
		fmt.Fprintf(w, "  %s: %s\n", label, f.Path)
	}

	if remaining > 0 {
		fmt.Fprintf(w, "  ... 等 %d 个%s文件\n", remaining, label)
	}
}
```

**Step 2: 运行测试确认通过**

Run: `go test ./internal/cli/cobra/... -run TestPrintRestorePreview -v`
Expected: PASS

**Step 3: 运行全量测试确认无回归**

Run: `go test ./...`
Expected: 全部通过

**Step 4: Commit 实现**

```bash
git add internal/cli/cobra/snapshot_restore.go
git commit -m "fix: snapshot restore 预览输出不再逐行列出跳过文件

完成了什么：
- printRestorePreview 按新增/更新分类显示 AppliedFiles
- 每个分类最多显示 5 个文件，超出显示汇总
- SkippedFiles 仅显示跳过数量，不再逐行列出
- 提取 groupByAction 和 printFileCategory 辅助函数

有什么作用：
- 解决 node_modules/.git 等大目录跳过文件刷屏问题
- 输出风格与 sync pull 保持一致

| 测试场景 | 输入/操作 | 预期结果 | 实际结果 |
|----------|-----------|----------|----------|
| 100 个跳过文件 | printRestorePreview | 不逐行列出 | 通过 |
| 10 个同类 AppliedFiles | printRestorePreview | 显示前 5 + 汇总 | 通过 |
| 少量文件 | printRestorePreview | 全部显示 | 通过 |
| AppliedCount=0 | printRestorePreview | 显示无需恢复 | 通过 |
| go test | go test ./... | 全部通过 | 通过 |"
```

---

### Task 3: 验证 printRestoreResult 无需修改

**Files:**
- Read-only: `internal/cli/cobra/snapshot_restore.go:157-177` (printRestoreResult)

**Step 1: 确认 printRestoreResult 当前行为**

`printRestoreResult`（行 157-177）已只显示汇总数量（成功/跳过/失败），不逐行列出 SkippedFiles。符合 issue 期望行为，无需修改。

**Step 2: Commit（如有文档更新需要）**

此 Task 不产生代码变更，跳过 Commit。
