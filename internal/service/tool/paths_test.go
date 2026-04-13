package tool

import (
	"path/filepath"
	"runtime"
	"testing"

	"flux/pkg/utils"

	"github.com/stretchr/testify/assert"
)

// TestGetDefaultGlobalPath_Codex 测试获取 Codex 全局路径
func TestGetDefaultGlobalPath_Codex(t *testing.T) {
	expected := filepath.Join(GetUserHomeDirForTest(), ".codex")
	result := GetDefaultGlobalPath(ToolTypeCodex)
	assert.Equal(t, expected, result)
}

// TestGetDefaultGlobalPath_Claude 测试获取 Claude 全局路径
func TestGetDefaultGlobalPath_Claude(t *testing.T) {
	expected := filepath.Join(GetUserHomeDirForTest(), ".claude")
	result := GetDefaultGlobalPath(ToolTypeClaude)
	assert.Equal(t, expected, result)
}

// TestGetDefaultGlobalPath_Unknown 测试未知工具类型
func TestGetDefaultGlobalPath_Unknown(t *testing.T) {
	result := GetDefaultGlobalPath(ToolType("unknown"))
	assert.Empty(t, result, "未知工具类型应返回空字符串")
}

// TestGetDefaultProjectPath_Codex 测试获取 Codex 项目路径
func TestGetDefaultProjectPath_Codex(t *testing.T) {
	result := GetDefaultProjectPath(ToolTypeCodex)
	assert.Equal(t, ".codex", result)
}

// TestGetDefaultProjectPath_Claude 测试获取 Claude 项目路径
func TestGetDefaultProjectPath_Claude(t *testing.T) {
	result := GetDefaultProjectPath(ToolTypeClaude)
	assert.Equal(t, ".claude", result)
}

// TestExpandPath_Tilde 测试 ~ 展开
func TestExpandPath_Tilde(t *testing.T) {
	input := "~/test/config"
	expected := filepath.Join(GetUserHomeDirForTest(), "test/config")
	result := ExpandPath(input)
	assert.Equal(t, expected, result)
}

// TestExpandPath_TildeAtEnd 测试路径末尾的 ~
func TestExpandPath_TildeAtEnd(t *testing.T) {
	input := "path/to/~"
	// ~ 在末尾时不展开，但路径分隔符根据平台变化
	result := ExpandPath(input)
	// Windows 使用 \，其他平台使用 /
	if runtime.GOOS == "windows" {
		assert.Equal(t, "path\\to\\~", result)
	} else {
		assert.Equal(t, "path/to/~", result)
	}
}

// TestExpandPath_Empty 测试空路径
func TestExpandPath_Empty(t *testing.T) {
	result := ExpandPath("")
	// filepath.Clean 将空字符串转换为 "."
	assert.Equal(t, ".", result)
}

// TestExpandPath_RelativePath 测试相对路径
func TestExpandPath_RelativePath(t *testing.T) {
	input := "./config"
	result := ExpandPath(input)
	// filepath.Clean 会移除前导 ./
	assert.Equal(t, "config", result, "filepath.Clean 会移除前导 ./")
}

// TestExpandPath_WindowsUserProfile 测试 Windows %USERPROFILE% 替换
func TestExpandPath_WindowsUserProfile(t *testing.T) {
	if !utils.IsWindows() {
		t.Skip("仅在 Windows 上测试")
	}

	input := "%USERPROFILE%\\test"
	expected := filepath.Join(GetUserHomeDirForTest(), "test")
	result := ExpandPath(input)
	assert.Equal(t, expected, result)
}

// TestResolveToolPath_GlobalScope 测试全局作用域路径解析
func TestResolveToolPath_GlobalScope(t *testing.T) {
	toolType := ToolTypeCodex
	basePath := "" // 全局配置不需要 basePath

	result := ResolveToolPath(toolType, ScopeGlobal, basePath)
	expected := filepath.Join(GetUserHomeDirForTest(), ".codex")

	assert.Equal(t, expected, result)
}

// TestResolveToolPath_ProjectScope 测试项目作用域路径解析
func TestResolveToolPath_ProjectScope(t *testing.T) {
	toolType := ToolTypeCodex
	projectPath := "D:\\myproject"
	scope := ScopeProject

	result := ResolveToolPath(toolType, scope, projectPath)
	expected := filepath.Join(projectPath, ".codex")

	assert.Equal(t, expected, result)
}

// TestIsWindows 测试 Windows 判断
func TestIsWindows(t *testing.T) {
	result := IsWindows()
	if runtime.GOOS == "windows" {
		assert.True(t, result)
	} else {
		assert.False(t, result)
	}
}

// TestGetUserHomeDir 测试获取用户主目录
func TestGetUserHomeDir(t *testing.T) {
	result := GetUserHomeDir()
	assert.NotEmpty(t, result)
	assert.NotEqual(t, "~", result, "应返回实际路径而非 ~")
}

// TestGetHomeConfigPath 测试获取主目录配置路径
func TestGetHomeConfigPath_Codex(t *testing.T) {
	toolType := ToolTypeCodex
	relativePath := "skills/test"

	result := GetHomeConfigPath(toolType, relativePath)
	expected := filepath.Join(GetUserHomeDirForTest(), ".codex", "skills", "test")

	assert.Equal(t, expected, result)
}

// TestGetCodexFileDefinitions 测试 Codex 文件定义
func TestGetCodexFileDefinitions(t *testing.T) {
	definitions := GetCodexFileDefinitions()

	// 验证返回非空
	assert.NotEmpty(t, definitions, "Codex 文件定义不应为空")

	// 验证包含关键文件
	hasConfigFile := false
	hasSkillsDir := false
	hasAgentsMd := false

	for _, def := range definitions {
		if def.Name == "config.toml" {
			hasConfigFile = true
		}
		if def.Name == "skills" {
			hasSkillsDir = true
		}
		if def.Name == "AGENTS.md" {
			hasAgentsMd = true
		}
	}

	assert.True(t, hasConfigFile, "应包含 config.toml")
	assert.True(t, hasSkillsDir, "应包含 skills 目录")
	assert.True(t, hasAgentsMd, "应包含 AGENTS.md")
}

// TestGetCodexFileDefinitions_Scopes 测试作用域分类
func TestGetCodexFileDefinitions_Scopes(t *testing.T) {
	definitions := GetCodexFileDefinitions()

	hasGlobal := false
	hasProject := false

	for _, def := range definitions {
		if def.Scope == ScopeGlobal {
			hasGlobal = true
		}
		if def.Scope == ScopeProject {
			hasProject = true
		}
	}

	assert.True(t, hasGlobal, "应有全局作用域配置")
	assert.True(t, hasProject, "应有项目作用域配置")
}

// TestGetClaudeFileDefinitions 测试 Claude 文件定义
func TestGetClaudeFileDefinitions(t *testing.T) {
	definitions := GetClaudeFileDefinitions()

	// 验证返回非空
	assert.NotEmpty(t, definitions, "Claude 文件定义不应为空")

	// 验证包含关键文件
	hasSkillsDir := false
	hasCLAUDEMd := false
	hasConfigFile := false

	for _, def := range definitions {
		if def.Name == "skills" {
			hasSkillsDir = true
		}
		if def.Name == "CLAUDE.md" {
			hasCLAUDEMd = true
		}
		if def.Category == CategoryConfigFile {
			hasConfigFile = true
		}
	}

	assert.True(t, hasSkillsDir, "应包含 skills 目录")
	assert.True(t, hasCLAUDEMd, "应包含 CLAUDE.md")
	assert.True(t, hasConfigFile, "应包含配置文件")
}

// TestGetClaudeFileDefinitions_Scopes 测试作用域分类
func TestGetClaudeFileDefinitions_Scopes(t *testing.T) {
	definitions := GetClaudeFileDefinitions()

	hasGlobal := false
	hasProject := false

	for _, def := range definitions {
		if def.Scope == ScopeGlobal {
			hasGlobal = true
		}
		if def.Scope == ScopeProject {
			hasProject = true
		}
	}

	assert.True(t, hasGlobal, "应有全局作用域配置")
	assert.True(t, hasProject, "应有项目作用域配置")
}

// Table驱动测试：ExpandPath 各种输入
func TestExpandPath_Table(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"空字符串", "", "."}, // filepath.Clean 将空字符串转为 "."
		{"纯路径", "path/to/file", filepath.Join("path", "to", "file")}, // 使用 filepath.Join 以支持平台分隔符
		{"波浪号开头", "~/Documents", filepath.Join(GetUserHomeDirForTest(), "Documents")},
		{"波浪号在中间", filepath.Join("path", "~", "file"), filepath.Join("path", "~", "file")},
		{"波浪号在末尾", filepath.Join("path", "~"), filepath.Join("path", "~")},
		{"多个波浪号", filepath.Join(GetUserHomeDirForTest(), "path", "~", "file"), filepath.Join(GetUserHomeDirForTest(), "path", "~", "file")},
	}

	if utils.IsWindows() {
		tests = append(tests, struct {
			name     string
			input    string
			expected string
		}{
			name:     "USERPROFILE",
			input:    "%USERPROFILE%\\config",
			expected: filepath.Join(GetUserHomeDirForTest(), "config"),
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandPath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
