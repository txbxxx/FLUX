package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// CreateTestRepo 创建测试仓库
func CreateTestRepo(t *testing.T) string {
	t.Helper()

	tempDir := filepath.Join(os.TempDir(), "ai-sync-git-test")
	_ = os.RemoveAll(tempDir)
	_ = os.MkdirAll(tempDir, 0755)

	// 初始化 Git 仓库
	repo, err := InitRepository(tempDir, false)
	if err != nil {
		t.Fatalf("初始化测试仓库失败: %v", err)
	}

	// 创建测试文件
	testFile := filepath.Join(tempDir, "README.md")
	_ = os.WriteFile(testFile, []byte("# Test Repository\n"), 0644)

	_ = repo

	return tempDir
}

// CreateTestRepoWithCommit 创建带提交的测试仓库
func CreateTestRepoWithCommit(t *testing.T) string {
	t.Helper()

	tempDir := filepath.Join(os.TempDir(), "ai-sync-git-test-commit")
	_ = os.RemoveAll(tempDir)
	_ = os.MkdirAll(tempDir, 0755)

	// 初始化 Git 仓库
	_, err := InitRepository(tempDir, false)
	if err != nil {
		t.Fatalf("初始化测试仓库失败: %v", err)
	}

	// 创建测试文件并提交
	testFile := filepath.Join(tempDir, "config.toml")
	_ = os.WriteFile(testFile, []byte("[test]\nvalue = \"hello\"\n"), 0644)

	client := NewGitClient()
	_, _ = client.Commit(context.TODO(), &CommitOptions{
		Path:    tempDir,
		Message: "Initial commit",
		All:     true,
	})

	return tempDir
}

// CleanupTestRepo 清理测试仓库
func CleanupTestRepo(t *testing.T, path string) {
	t.Helper()
	_ = os.RemoveAll(path)
}

// CreateTestSSHKey 创建测试 SSH 密钥
func CreateTestSSHKey(t *testing.T) string {
	t.Helper()

	tempDir := filepath.Join(os.TempDir(), "ai-sync-ssh-test")
	_ = os.MkdirAll(tempDir, 0755)

	// 创建一个假的私钥文件用于测试
	keyFile := filepath.Join(tempDir, "id_test")
	_ = os.WriteFile(keyFile, []byte("-----BEGIN FAKE KEY-----\n"), 0600)

	return keyFile
}

// GetTestHomeDir 获取测试主目录
func GetTestHomeDir() string {
	home, _ := os.UserHomeDir()
	return home
}
