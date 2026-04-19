package cobra

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	typesSnapshot "flux/internal/types/snapshot"
)

// makeRestoreResult builds a RestoreResult with the given counts of applied and skipped files.
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
	result := makeRestoreResult(2, 100)
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

	// 应显示前几个文件
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

	if !strings.Contains(output, "所有文件内容相同") {
		t.Errorf("should show 'all identical' message, got: %s", output)
	}
}

func TestPrintRestorePreview_SmallResultSetShowsAll(t *testing.T) {
	var buf bytes.Buffer
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
	if !strings.Contains(output, "跳过: 2") {
		t.Errorf("should show skipped count, got: %s", output)
	}
}
