package usecase

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	pathpkg "path/filepath"
	"testing"

	"flux/internal/models"
	"flux/internal/types/snapshot"
	typesSync "flux/internal/types/sync"
)

// --- updateSnapshotFile tests ---

func TestUpdateSnapshotFile_UpdatesExistingFile(t *testing.T) {
	snapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "settings.json", Content: []byte("old content"), Size: 11, Hash: "oldhash"},
		{Path: "CLAUDE.md", Content: []byte("claude"), Size: 6, Hash: "claudehash"},
	}}

	newContent := []byte("new content here")
	(&LocalWorkflow{}).updateSnapshotFile(snapshot, "settings.json", newContent, "", "")

	if len(snapshot.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(snapshot.Files))
	}
	if string(snapshot.Files[0].Content) != "new content here" {
		t.Fatalf("expected updated content, got %q", string(snapshot.Files[0].Content))
	}
	// "new content here" is 16 bytes
	if snapshot.Files[0].Size != 16 {
		t.Fatalf("expected size 16, got %d", snapshot.Files[0].Size)
	}
	if snapshot.Files[0].Hash == "oldhash" {
		t.Fatal("expected hash to be recomputed, still has old hash")
	}
}

func TestUpdateSnapshotFile_AddsNewFile(t *testing.T) {
	snapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "settings.json", Content: []byte("existing")},
	}}

	(&LocalWorkflow{}).updateSnapshotFile(snapshot, "new-file.txt", []byte("brand new"), "", "")

	if len(snapshot.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(snapshot.Files))
	}
	if snapshot.Files[1].Path != "new-file.txt" {
		t.Fatalf("expected path new-file.txt, got %q", snapshot.Files[1].Path)
	}
	if string(snapshot.Files[1].Content) != "brand new" {
		t.Fatalf("expected content 'brand new', got %q", string(snapshot.Files[1].Content))
	}
	if snapshot.Files[1].Size != 9 {
		t.Fatalf("expected size 9, got %d", snapshot.Files[1].Size)
	}
	if snapshot.Files[1].Hash == "" {
		t.Fatal("expected hash to be computed")
	}
}

func TestUpdateSnapshotFile_WindowsBackslashPath(t *testing.T) {
	snapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "claude/settings.json", Content: []byte("old")},
	}}

	(&LocalWorkflow{}).updateSnapshotFile(snapshot, "claude/settings.json", []byte("updated"), "", "")

	if len(snapshot.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(snapshot.Files))
	}
	if snapshot.Files[0].Path != "claude/settings.json" {
		t.Fatalf("expected normalized path 'claude/settings.json', got %q", snapshot.Files[0].Path)
	}
	if string(snapshot.Files[0].Content) != "updated" {
		t.Fatalf("expected content 'updated', got %q", string(snapshot.Files[0].Content))
	}
}

func TestUpdateSnapshotFile_AddsNewFileWithWindowsPath(t *testing.T) {
	snapshot := &models.Snapshot{Files: []models.SnapshotFile{}}

	(&LocalWorkflow{}).updateSnapshotFile(snapshot, "skills/lark-approval/SKILL.md", []byte("skill content"), "", "")

	if len(snapshot.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(snapshot.Files))
	}
	if snapshot.Files[0].Path != "skills/lark-approval/SKILL.md" {
		t.Fatalf("expected 'skills/lark-approval/SKILL.md', got %q", snapshot.Files[0].Path)
	}
}

func TestUpdateSnapshotFile_OriginalPathPreserved(t *testing.T) {
	snapshot := &models.Snapshot{Files: []models.SnapshotFile{}}

	(&LocalWorkflow{}).updateSnapshotFile(snapshot, "sub/nested/file.txt", []byte("content"), "", "")

	if len(snapshot.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(snapshot.Files))
	}
	if snapshot.Files[0].OriginalPath != snapshot.Files[0].Path {
		t.Fatalf("expected OriginalPath == Path, got %q vs %q", snapshot.Files[0].OriginalPath, snapshot.Files[0].Path)
	}
}

func TestUpdateSnapshotFile_AllFieldsPopulated(t *testing.T) {
	snapshot := &models.Snapshot{Files: []models.SnapshotFile{}}

	(&LocalWorkflow{}).updateSnapshotFile(snapshot, "file.txt", []byte("hello world"), "", "")

	f := snapshot.Files[0]
	if f.Path == "" || f.OriginalPath == "" || f.Size == 0 || f.Hash == "" || len(f.Content) == 0 {
		t.Errorf("not all fields populated: Path=%q OriginalPath=%q Size=%d Hash=%q ContentLen=%d",
			f.Path, f.OriginalPath, f.Size, f.Hash, len(f.Content))
	}
	if f.Size != 11 {
		t.Errorf("expected Size=11, got %d", f.Size)
	}
	if f.Hash != computeHash([]byte("hello world")) {
		t.Errorf("expected Hash=%s, got %s", computeHash([]byte("hello world")), f.Hash)
	}
}

func TestUpdateSnapshotFile_ByteContentPreserved(t *testing.T) {
	binaryContent := []byte{0xFF, 0xFE, 0x00, 0x01, '\n', '\t'}

	snapshot := &models.Snapshot{Files: []models.SnapshotFile{}}
	(&LocalWorkflow{}).updateSnapshotFile(snapshot, "binary.bin", binaryContent, "", "")

	if !bytes.Equal(snapshot.Files[0].Content, binaryContent) {
		t.Error("binary content not preserved correctly")
	}
	if snapshot.Files[0].Size != 6 {
		t.Errorf("expected size 6, got %d", snapshot.Files[0].Size)
	}
	if snapshot.Files[0].Hash != computeHash(binaryContent) {
		t.Error("hash doesn't match binary content")
	}
}

func TestUpdateSnapshotFile_EmptyContent(t *testing.T) {
	snapshot := &models.Snapshot{Files: []models.SnapshotFile{}}

	(&LocalWorkflow{}).updateSnapshotFile(snapshot, "empty.txt", []byte{}, "", "")

	if len(snapshot.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(snapshot.Files))
	}
	if snapshot.Files[0].Size != 0 {
		t.Errorf("expected size 0, got %d", snapshot.Files[0].Size)
	}
	if snapshot.Files[0].Hash == "" {
		t.Error("expected hash even for empty content")
	}
}

func TestUpdateSnapshotFile_UnicodeContent(t *testing.T) {
	unicodeContent := []byte("你好世界 🌍 مرحبا")

	snapshot := &models.Snapshot{Files: []models.SnapshotFile{}}
	(&LocalWorkflow{}).updateSnapshotFile(snapshot, "unicode.txt", unicodeContent, "", "")

	if string(snapshot.Files[0].Content) != "你好世界 🌍 مرحبا" {
		t.Error("unicode content not preserved")
	}
	if snapshot.Files[0].Size != int64(len(unicodeContent)) {
		t.Errorf("expected size %d, got %d", len(unicodeContent), snapshot.Files[0].Size)
	}
}

// --- computeHash tests ---

func TestComputeHash_Deterministic(t *testing.T) {
	content := []byte("hello world")
	h1 := computeHash(content)
	h2 := computeHash(content)

	if h1 != h2 {
		t.Fatalf("expected deterministic hash, got %q vs %q", h1, h2)
	}
	if h1 != "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9" {
		t.Fatalf("unexpected SHA256 hash: %s", h1)
	}
}

func TestComputeHash_DifferentContentDifferentHash(t *testing.T) {
	h1 := computeHash([]byte("content a"))
	h2 := computeHash([]byte("content b"))

	if h1 == h2 {
		t.Fatal("expected different hashes for different content")
	}
}

// --- compareSnapshotWithRemote tests ---

func TestCompareSnapshotWithRemote_DetectsModifiedFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteDir, "settings.json"), []byte("remote content"), 0644); err != nil {
		t.Fatal(err)
	}

	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "settings.json", Content: []byte("local content")},
	}}

	w := &LocalWorkflow{}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	conflicts, err := w.compareSnapshotWithRemote(context.Background(), repoPath, "remoteHash123", localSnapshot, projectPrefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Path != "settings.json" {
		t.Fatalf("expected path 'settings.json', got %q", conflicts[0].Path)
	}
	if conflicts[0].ConflictType != "both_modified" {
		t.Fatalf("expected conflict type 'both_modified', got %q", conflicts[0].ConflictType)
	}
}

func TestCompareSnapshotWithRemote_DetectsRemoteNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteDir, "remote-file.txt"), []byte("remote only"), 0644); err != nil {
		t.Fatal(err)
	}

	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "settings.json", Content: []byte("local")},
	}}

	w := &LocalWorkflow{}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	conflicts, err := w.compareSnapshotWithRemote(context.Background(), repoPath, "remoteHash123", localSnapshot, projectPrefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}
	if conflicts[0].Path != "remote-file.txt" {
		t.Fatalf("expected path 'remote-file.txt', got %q", conflicts[0].Path)
	}
	if conflicts[0].ConflictType != "remote_new" {
		t.Fatalf("expected conflict type 'remote_new', got %q", conflicts[0].ConflictType)
	}
}

func TestCompareSnapshotWithRemote_NoConflictWhenIdentical(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteDir, "settings.json"), []byte("same content"), 0644); err != nil {
		t.Fatal(err)
	}

	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "settings.json", Content: []byte("same content")},
	}}

	w := &LocalWorkflow{}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	conflicts, err := w.compareSnapshotWithRemote(context.Background(), repoPath, "remoteHash123", localSnapshot, projectPrefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(conflicts) != 0 {
		t.Fatalf("expected 0 conflicts, got %d", len(conflicts))
	}
}

func TestCompareSnapshotWithRemote_WindowsBackslashPath(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	subDir := filepath.Join(remoteDir, "claude")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "settings.json"), []byte("remote"), 0644); err != nil {
		t.Fatal(err)
	}

	// Local snapshot: Windows backslash path
	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "claude\\settings.json", Content: []byte("local")},
	}}

	w := &LocalWorkflow{}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	conflicts, err := w.compareSnapshotWithRemote(context.Background(), repoPath, "remoteHash123", localSnapshot, projectPrefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect conflict because paths are normalized to forward slash
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict (Windows backslash normalized), got %d", len(conflicts))
	}
	if conflicts[0].Path != "claude/settings.json" {
		t.Fatalf("expected normalized path 'claude/settings.json', got %q", conflicts[0].Path)
	}
}

// --- applyRemoteToSnapshot tests ---

// snapshotServiceForTest implements SnapshotManager interface with no-op UpdateSnapshot
type snapshotServiceForTest struct{}

func (s *snapshotServiceForTest) CreateSnapshot(opts snapshot.CreateSnapshotOptions) (*snapshot.SnapshotPackage, error) {
	return nil, nil
}
func (s *snapshotServiceForTest) ListSnapshots(limit, offset int) ([]*snapshot.SnapshotListItem, error) {
	return nil, nil
}
func (s *snapshotServiceForTest) CountSnapshots() (int, error)              { return 0, nil }
func (s *snapshotServiceForTest) DeleteSnapshot(id string) error            { return nil }
func (s *snapshotServiceForTest) GetSnapshot(id string) (*models.Snapshot, error) {
	return nil, nil
}
func (s *snapshotServiceForTest) UpdateSnapshot(snapshot *models.Snapshot) error { return nil }
func (s *snapshotServiceForTest) RestoreSnapshot(id string, files []string, opts snapshot.ApplyOptions) (*snapshot.RestoreResult, error) {
	return nil, nil
}
func (s *snapshotServiceForTest) DiffSnapshots(src, tgt string, verbose bool, tool, path string, ctx int) (*snapshot.DiffResult, error) {
	return nil, nil
}

func TestApplyRemoteToSnapshot_UpdatesExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteDir, "settings.json"), []byte("remote content"), 0644); err != nil {
		t.Fatal(err)
	}

	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "settings.json", Content: []byte("local content")},
	}}

	w := &LocalWorkflow{snapshots: &snapshotServiceForTest{}}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	err := w.applyRemoteToSnapshot(context.Background(), repoPath, "remoteHash", localSnapshot, projectPrefix, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(localSnapshot.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(localSnapshot.Files))
	}
	if string(localSnapshot.Files[0].Content) != "remote content" {
		t.Fatalf("expected 'remote content', got %q", string(localSnapshot.Files[0].Content))
	}
}

func TestApplyRemoteToSnapshot_AddsNewRemoteFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteDir, "settings.json"), []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteDir, "new-file.txt"), []byte("brand new"), 0644); err != nil {
		t.Fatal(err)
	}

	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "settings.json", Content: []byte("existing")},
	}}

	w := &LocalWorkflow{snapshots: &snapshotServiceForTest{}}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	err := w.applyRemoteToSnapshot(context.Background(), repoPath, "remoteHash", localSnapshot, projectPrefix, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(localSnapshot.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(localSnapshot.Files))
	}
	found := false
	for _, f := range localSnapshot.Files {
		if f.Path == "new-file.txt" && string(f.Content) == "brand new" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected new-file.txt to be added to snapshot")
	}
}

func TestApplyRemoteToSnapshot_RemovesDeletedFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteDir, "settings.json"), []byte("still here"), 0644); err != nil {
		t.Fatal(err)
	}

	// Local snapshot: settings.json + deleted-file.txt
	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "settings.json", Content: []byte("still here")},
		{Path: "deleted-file.txt", Content: []byte("should be removed")},
	}}

	w := &LocalWorkflow{snapshots: &snapshotServiceForTest{}}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	err := w.applyRemoteToSnapshot(context.Background(), repoPath, "remoteHash", localSnapshot, projectPrefix, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(localSnapshot.Files) != 1 {
		t.Fatalf("expected 1 file after cleanup, got %d", len(localSnapshot.Files))
	}
	if localSnapshot.Files[0].Path != "settings.json" {
		t.Fatalf("expected only settings.json, got %q", localSnapshot.Files[0].Path)
	}
}

func TestApplyRemoteToSnapshot_HandlesSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	subDir := filepath.Join(remoteDir, "claude")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "settings.json"), []byte("nested remote"), 0644); err != nil {
		t.Fatal(err)
	}

	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{}}

	w := &LocalWorkflow{snapshots: &snapshotServiceForTest{}}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	err := w.applyRemoteToSnapshot(context.Background(), repoPath, "remoteHash", localSnapshot, projectPrefix, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(localSnapshot.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(localSnapshot.Files))
	}
	if localSnapshot.Files[0].Path != "claude/settings.json" {
		t.Fatalf("expected 'claude/settings.json', got %q", localSnapshot.Files[0].Path)
	}
	if string(localSnapshot.Files[0].Content) != "nested remote" {
		t.Fatalf("expected 'nested remote', got %q", string(localSnapshot.Files[0].Content))
	}
}

func TestApplyRemoteToSnapshot_WindowsBackslashRemovesDeletedFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteDir, "settings.json"), []byte("remote"), 0644); err != nil {
		t.Fatal(err)
	}

	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "settings.json", Content: []byte("local")},
		{Path: "old\\file.txt", Content: []byte("deleted on remote")},
	}}

	w := &LocalWorkflow{snapshots: &snapshotServiceForTest{}}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	err := w.applyRemoteToSnapshot(context.Background(), repoPath, "remoteHash", localSnapshot, projectPrefix, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(localSnapshot.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(localSnapshot.Files))
	}
}

func TestApplyRemoteToSnapshot_WindowsBackslashAddsNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	subDir := filepath.Join(remoteDir, "skills")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "lark.md"), []byte("skill content"), 0644); err != nil {
		t.Fatal(err)
	}

	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{}}

	w := &LocalWorkflow{snapshots: &snapshotServiceForTest{}}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	err := w.applyRemoteToSnapshot(context.Background(), repoPath, "remoteHash", localSnapshot, projectPrefix, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(localSnapshot.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(localSnapshot.Files))
	}
	if localSnapshot.Files[0].Path != "skills/lark.md" {
		t.Fatalf("expected 'skills/lark.md', got %q", localSnapshot.Files[0].Path)
	}
}

func TestSnapshotFilesConsistency_AfterApplyRemote(t *testing.T) {
	tmpDir := t.TempDir()
	projectPrefix := "claude"

	remoteDir := filepath.Join(tmpDir, "repos", "claude-global", projectPrefix)
	if err := os.MkdirAll(remoteDir, 0755); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(remoteDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteDir, "settings.json"), []byte("remote v2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteDir, "new-file.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "settings.json"), []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	localSnapshot := &models.Snapshot{Files: []models.SnapshotFile{
		{Path: "settings.json", Content: []byte("local v1")},
		{Path: "deleted-file.txt", Content: []byte("will be removed")},
	}}

	w := &LocalWorkflow{snapshots: &snapshotServiceForTest{}}
	repoPath := filepath.Join(tmpDir, "repos", "claude-global")
	err := w.applyRemoteToSnapshot(context.Background(), repoPath, "remoteHash", localSnapshot, projectPrefix, "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(localSnapshot.Files) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(localSnapshot.Files), filePaths(localSnapshot.Files))
	}

	paths := make(map[string]bool)
	for _, f := range localSnapshot.Files {
		paths[f.Path] = true
	}

	if paths["deleted-file.txt"] {
		t.Error("deleted-file.txt should have been removed")
	}
	if !paths["settings.json"] || !paths["new-file.txt"] || !paths["sub/settings.json"] {
		t.Errorf("missing expected files, got: %v", filePaths(localSnapshot.Files))
	}

	for _, f := range localSnapshot.Files {
		if f.Path == "settings.json" && string(f.Content) != "remote v2" {
			t.Errorf("settings.json: expected 'remote v2', got %q", string(f.Content))
		}
		if f.Path == "new-file.txt" && string(f.Content) != "new" {
			t.Errorf("new-file.txt: expected 'new', got %q", string(f.Content))
		}
		if f.Path == "sub/settings.json" && string(f.Content) != "nested" {
			t.Errorf("sub/settings.json: expected 'nested', got %q", string(f.Content))
		}
	}
}

// --- ConflictInfo / path normalization tests ---

func TestConflictInfo_ResolveKeepsLocal(t *testing.T) {
	conflict := typesSync.ConflictInfo{
		Path:          "settings.json",
		ConflictType:  "both_modified",
		LocalSummary:  "本地已修改 (100 字节)",
		RemoteSummary: "远端已修改 (150 字节)",
		LocalHash:     "localHash123",
		RemoteHash:    "remoteHash456",
	}

	if conflict.LocalHash == "" {
		t.Fatal("LocalHash should be populated")
	}
	if conflict.RemoteHash == "" {
		t.Fatal("RemoteHash should be populated")
	}
	if conflict.ConflictType != "both_modified" {
		t.Fatalf("expected 'both_modified', got %q", conflict.ConflictType)
	}
}

func TestConflictInfo_RemoteNewFile(t *testing.T) {
	conflict := typesSync.ConflictInfo{
		Path:         "new-on-remote.txt",
		ConflictType: "remote_new",
		LocalSummary: "本地无此文件",
	}

	if conflict.LocalHash != "" {
		t.Fatal("LocalHash should be empty for remote_new")
	}
	// RemoteHash may or may not be populated depending on caller
	_ = conflict.RemoteHash
}

func TestPathNormalization_WindowsBackslash(t *testing.T) {
	winPath := "claude\\settings.json"
	normalized := pathpkg.ToSlash(winPath)
	if normalized != "claude/settings.json" {
		t.Fatalf("expected 'claude/settings.json', got %q", normalized)
	}
}

func TestPathNormalization_NestedWindowsPath(t *testing.T) {
	winPath := "skills\\lark-approval\\SKILL.md"
	normalized := pathpkg.ToSlash(winPath)
	if normalized != "skills/lark-approval/SKILL.md" {
		t.Fatalf("expected 'skills/lark-approval/SKILL.md', got %q", normalized)
	}
}

// --- helper ---

func filePaths(files []models.SnapshotFile) []string {
	result := make([]string, len(files))
	for i, f := range files {
		result[i] = f.Path
	}
	return result
}
