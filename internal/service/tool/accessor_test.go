package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ai-sync-manager/pkg/database"
)

func TestConfigAccessorListDirReturnsImmediateEntries(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	root := filepath.Join(homeDir, ".codex")
	CreateMockFile(t, filepath.Join(root, "skills", "aiskills", "README.md"), "# readme")
	CreateMockFile(t, filepath.Join(root, "skills", "workflow.md"), "# workflow")

	accessor := NewConfigAccessor()

	target, err := accessor.Resolve(ToolTypeCodex, "skills")
	if err != nil {
		t.Fatalf("expected skills directory to resolve: %v", err)
	}
	if !target.IsDir {
		t.Fatalf("expected skills to resolve as directory: %+v", target)
	}

	entries, err := accessor.ListDir(target)
	if err != nil {
		t.Fatalf("expected directory entries: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Name != "aiskills" || !entries[0].IsDir {
		t.Fatalf("expected first entry to be aiskills dir, got %+v", entries[0])
	}
	if entries[1].Name != "workflow.md" || entries[1].IsDir {
		t.Fatalf("expected second entry to be workflow.md file, got %+v", entries[1])
	}
}

func TestConfigAccessorReadFileReturnsContent(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	root := filepath.Join(homeDir, ".codex")
	path := filepath.Join(root, "skills", "aiskills", "README.md")
	CreateMockFile(t, path, "# hello")

	accessor := NewConfigAccessor()

	target, err := accessor.Resolve(ToolTypeCodex, filepath.Join("skills", "aiskills", "README.md"))
	if err != nil {
		t.Fatalf("expected file to resolve: %v", err)
	}
	if target.IsDir {
		t.Fatalf("expected resolved target to be file: %+v", target)
	}

	content, err := accessor.ReadFile(target)
	if err != nil {
		t.Fatalf("expected to read file content: %v", err)
	}
	if content != "# hello" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestConfigAccessorResolveRejectsTraversal(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	root := filepath.Join(homeDir, ".codex")
	CreateMockFile(t, filepath.Join(root, "config.toml"), "name = 'codex'")

	accessor := NewConfigAccessor()

	_, err := accessor.Resolve(ToolTypeCodex, filepath.Join("..", "Desktop", "secret.txt"))
	if err == nil {
		t.Fatal("expected traversal path to be rejected")
	}
	if !strings.Contains(err.Error(), "允许范围") && !strings.Contains(err.Error(), "越界") {
		t.Fatalf("expected traversal error to mention bounds, got %v", err)
	}
}

func TestConfigAccessorResolveRejectsPathOutsideAllowedRoots(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	root := filepath.Join(homeDir, ".codex")
	CreateMockFile(t, filepath.Join(root, "notes", "todo.txt"), "hidden")

	accessor := NewConfigAccessor()

	_, err := accessor.Resolve(ToolTypeCodex, filepath.Join("notes", "todo.txt"))
	if err == nil {
		t.Fatal("expected non-whitelisted path to be rejected")
	}
	if !strings.Contains(err.Error(), "允许范围") {
		t.Fatalf("expected whitelist error, got %v", err)
	}
}

// TestConfigAccessorReadFileScrubNullBytes 验证含空字节的文件会被自动清理后返回。
// 为什么：修复 get -e 编辑后再次打开报"无法读取文件"的 bug，
// settings.json 因损坏嵌入空字节时，应清理后返回而非直接拒绝。
func TestConfigAccessorReadFileScrubNullBytes(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	root := filepath.Join(homeDir, ".codex")
	path := filepath.Join(root, "skills", "bin.dat")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	accessor := NewConfigAccessor()

	target, err := accessor.Resolve(ToolTypeCodex, filepath.Join("skills", "bin.dat"))
	if err != nil {
		t.Fatalf("expected file to resolve: %v", err)
	}

	content, err := accessor.ReadFile(target)
	if err != nil {
		t.Fatalf("expected scrubbed file to be readable: %v", err)
	}
	// 空字节被清理后，剩余 \x01\x02 作为文本返回
	if content != "\x01\x02" {
		t.Fatalf("expected scrubbed content '\\x01\\x02', got %q", content)
	}
}

func TestConfigAccessorWriteFileReplacesOriginalContent(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	root := filepath.Join(homeDir, ".codex")
	path := filepath.Join(root, "skills", "aiskills", "README.md")
	CreateMockFile(t, path, "# before")

	accessor := NewConfigAccessor()

	target, err := accessor.Resolve(ToolTypeCodex, filepath.Join("skills", "aiskills", "README.md"))
	if err != nil {
		t.Fatalf("expected file to resolve: %v", err)
	}

	if err := accessor.WriteFile(target, "# after"); err != nil {
		t.Fatalf("expected write to succeed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected updated file on disk: %v", err)
	}
	if string(raw) != "# after" {
		t.Fatalf("unexpected disk content: %q", string(raw))
	}
}

func TestConfigAccessorResolveAllowsRegisteredAbsoluteFile(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	CreateMockFile(t, filepath.Join(homeDir, ".claude", "settings.json"), "{}")
	customFile := filepath.Join(t.TempDir(), "claude.json")
	CreateMockFile(t, customFile, `{"mcpServers":[]}`)

	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	manager := NewRuleManager(db)
	if err := manager.AddCustomRule(ToolTypeClaude, customFile); err != nil {
		t.Fatalf("add custom rule: %v", err)
	}

	accessor := NewConfigAccessor(NewRuleResolver(manager.Store()))
	target, err := accessor.Resolve(ToolTypeClaude, customFile)
	if err != nil {
		t.Fatalf("expected registered absolute file to resolve: %v", err)
	}
	if target.RelativePath != filepath.Clean(customFile) {
		t.Fatalf("expected absolute request path to be preserved, got %+v", target)
	}
}

func TestConfigAccessorResolveAllowsRegisteredProjectAbsolutePath(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	CreateMockFile(t, filepath.Join(homeDir, ".codex", "config.toml"), "[codex]")
	projectPath := filepath.Join(t.TempDir(), "demo-project")
	projectFile := filepath.Join(projectPath, ".codex", "config.toml")
	CreateMockFile(t, projectFile, "[project]")

	db, err := database.InitTestDB(t)
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}

	manager := NewRuleManager(db)
	if err := manager.AddProject(ToolTypeCodex, "demo", projectPath); err != nil {
		t.Fatalf("add project: %v", err)
	}

	accessor := NewConfigAccessor(NewRuleResolver(manager.Store()))
	target, err := accessor.Resolve(ToolTypeCodex, projectFile)
	if err != nil {
		t.Fatalf("expected registered project file to resolve: %v", err)
	}
	if target.AbsolutePath != filepath.Clean(projectFile) {
		t.Fatalf("unexpected absolute path: %+v", target)
	}
}
