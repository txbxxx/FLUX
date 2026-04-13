package tool

import (
	"os"
	"path/filepath"
	"testing"
)

// CreateMockConfigDir 创建模拟配置目录
// 返回创建的目录路径，测试结束后需要调用 CleanupMockDir 清理
func CreateMockConfigDir(t *testing.T, toolType ToolType, scope ConfigScope) string {
	t.Helper()

	baseDir := filepath.Join(os.TempDir(), "fl"-test", string(toolType), string(scope))
	err := os.MkdirAll(baseDir, 0755)
	if err != nil {
		t.Fatalf("创建模拟目录失败: %v", err)
	}

	// 根据工具类型和作用域创建对应的文件结构
	switch toolType {
	case ToolTypeCodex:
		createMockCodexConfig(t, baseDir, scope)
	case ToolTypeClaude:
		createMockClaudeConfig(t, baseDir, scope)
	}

	return baseDir
}

// createMockCodexConfig 创建 Codex 模拟配置
func createMockCodexConfig(t *testing.T, baseDir string, scope ConfigScope) {
	t.Helper()

	if scope == ScopeGlobal {
		// 全局配置
		createMockFile(t, filepath.Join(baseDir, "config.toml"), "[codex]\nversion = \"1.0.0\"")
		createMockDir(t, filepath.Join(baseDir, "skills"))
		createMockDir(t, filepath.Join(baseDir, "rules"))
		createMockDir(t, filepath.Join(baseDir, "superpowers"))
	} else {
		// 项目配置
		createMockFile(t, filepath.Join(baseDir, "config.toml"), "[codex]\nversion = \"1.0.0\"")
		// AGENTS.md 在项目根目录（不在此 baseDir 内）
	}
}

// createMockClaudeConfig 创建 Claude 模拟配置
func createMockClaudeConfig(t *testing.T, baseDir string, scope ConfigScope) {
	t.Helper()

	if scope == ScopeGlobal {
		// 全局配置
		createMockDir(t, filepath.Join(baseDir, "skills"))
		createMockDir(t, filepath.Join(baseDir, "commands"))
		createMockDir(t, filepath.Join(baseDir, "plugins"))
		createMockDir(t, filepath.Join(baseDir, "output-styles"))
		createMockFile(t, filepath.Join(baseDir, "CLAUDE.md"), "# Claude Config")
		createMockFile(t, filepath.Join(baseDir, "settings.json"), "{}")
	} else {
		// 项目配置
		createMockDir(t, filepath.Join(baseDir, "skills"))
		createMockFile(t, filepath.Join(baseDir, "CLAUDE.md"), "# Project Claude Config")
	}
}

// CreateMockFile 创建模拟文件
func CreateMockFile(t *testing.T, path, content string) {
	t.Helper()
	createMockFile(t, path, content)
}

// createMockFile 内部实现
func createMockFile(t *testing.T, path, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}
}

// createMockDir 创建模拟目录
func createMockDir(t *testing.T, path string) {
	t.Helper()

	err := os.MkdirAll(path, 0755)
	if err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
}

// CleanupMockDir 清理模拟目录
func CleanupMockDir(t *testing.T, path string) {
	t.Helper()

	if path != "" {
		_ = os.RemoveAll(filepath.Join(os.TempDir(), "fl"-test"))
	}
}

// CreateMockProject 创建模拟项目
func CreateMockProject(t *testing.T, name string, hasCodex, hasClaude bool) string {
	t.Helper()

	projectDir := filepath.Join(os.TempDir(), "fl"-test-projects", name)
	err := os.MkdirAll(projectDir, 0755)
	if err != nil {
		t.Fatalf("创建项目目录失败: %v", err)
	}

	// 标记为项目目录
	createFileInDir(projectDir, ".git", "")

	if hasCodex {
		codexDir := filepath.Join(projectDir, ".codex")
		createMockDir(t, codexDir)
		createMockFile(t, filepath.Join(projectDir, "AGENTS.md"), "# Agents")
	}

	if hasClaude {
		claudeDir := filepath.Join(projectDir, ".claude")
		createMockDir(t, claudeDir)
		createMockFile(t, filepath.Join(projectDir, ".claude", "CLAUDE.md"), "# Project Config")
	}

	return projectDir
}

// createFileInDir 在指定目录创建文件
func createFileInDir(dir, name, content string) {
	path := filepath.Join(dir, name)
	_ = os.WriteFile(path, []byte(content), 0644)
}

// CleanupMockProjects 清理所有模拟项目
func CleanupMockProjects(t *testing.T) {
	t.Helper()
	_ = os.RemoveAll(filepath.Join(os.TempDir(), "fl"-test-projects"))
}

// GetUserHomeDirForTest 获取用户主目录（测试用）
func GetUserHomeDirForTest() string {
	return GetUserHomeDir()
}
