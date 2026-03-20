package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-sync-manager/internal/service/tool"

	"github.com/stretchr/testify/assert"
)

// TestNewApp 测试 App 创建
func TestNewApp(t *testing.T) {
	app := NewApp()
	assert.NotNil(t, app)
	assert.Equal(t, "1.0.0-alpha", app.version)
}

// TestHello 测试问候接口
func TestHello(t *testing.T) {
	app := NewApp()
	result := app.Hello("World")
	assert.Equal(t, "Hello, World!", result)

	result = app.Hello("")
	assert.Equal(t, "Hello, !", result)
}

// TestGetVersion 测试版本获取
func TestGetVersion(t *testing.T) {
	app := NewApp()
	version := app.GetVersion()
	assert.Equal(t, "1.0.0-alpha", version)
}

// TestGetSystemInfo 测试系统信息获取
func TestGetSystemInfo(t *testing.T) {
	app := NewApp()
	info := app.GetSystemInfo()

	assert.NotEmpty(t, info["os"])
	assert.NotEmpty(t, info["arch"])
	assert.NotEmpty(t, info["go_version"])
	assert.NotEmpty(t, info["app_version"])
	assert.Equal(t, "1.0.0-alpha", info["app_version"])
}

// TestHealthCheck 测试健康检查
func TestHealthCheck(t *testing.T) {
	app := NewApp()
	health := app.HealthCheck()

	assert.Equal(t, "ok", health["status"])
	assert.NotEmpty(t, health["version"])
	assert.NotEmpty(t, health["os"])
	assert.NotEmpty(t, health["arch"])
}

// TestScanTools 测试工具扫描
func TestScanTools(t *testing.T) {
	app := NewApp()

	result, err := app.ScanTools()
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 验证返回的是列表（可能为空）
	assert.IsType(t, []tool.ToolInstallation{}, result)

	// 验证每个工具的基本字段
	for _, installation := range result {
		assert.NotEmpty(t, installation.ToolType, "tool_type 不应为空")
		assert.NotEmpty(t, installation.Status, "status 不应为空")
		assert.NotZero(t, installation.DetectedAt, "detected_at 应设置")
	}
}

// TestScanTools_WithTimeout 测试扫描超时控制
func TestScanTools_WithTimeout(t *testing.T) {
	app := NewApp()

	// 多次调用验证不会卡住
	done := make(chan bool, 1)

	go func() {
		_, _ = app.ScanTools()
		done <- true
	}()

	select {
	case <-done:
		// 正常完成
	case <-time.After(35 * time.Second):
		t.Fatal("扫描超时，应在30秒内完成")
	}
}

// TestScanToolsWithProjects 测试带项目路径的工具扫描
func TestScanToolsWithProjects(t *testing.T) {
	app := NewApp()

	// 创建临时测试项目
	tempDir := filepath.Join(os.TempDir(), "ai-sync-app-test-project")
	_ = os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	// 创建 .git 标记为项目目录
	gitDir := filepath.Join(tempDir, ".git")
	_ = os.MkdirAll(gitDir, 0755)

	result, err := app.ScanToolsWithProjects([]string{tempDir})
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.IsType(t, []tool.ToolInstallation{}, result)
}

// TestScanToolsWithProjects_EmptyPaths 测试空项目路径列表
func TestScanToolsWithProjects_EmptyPaths(t *testing.T) {
	app := NewApp()

	result, err := app.ScanToolsWithProjects([]string{})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestScanToolsWithProjects_MultipleProjects 测试多项目扫描
func TestScanToolsWithProjects_MultipleProjects(t *testing.T) {
	app := NewApp()

	// 创建多个测试项目
	project1 := filepath.Join(os.TempDir(), "ai-sync-test-proj1")
	project2 := filepath.Join(os.TempDir(), "ai-sync-test-proj2")

	_ = os.MkdirAll(filepath.Join(project1, ".git"), 0755)
	_ = os.MkdirAll(filepath.Join(project2, ".git"), 0755)

	defer os.RemoveAll(project1)
	defer os.RemoveAll(project2)

	result, err := app.ScanToolsWithProjects([]string{project1, project2})
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestGetToolConfigPath_Codex 测试获取 Codex 配置路径
func TestGetToolConfigPath_Codex(t *testing.T) {
	app := NewApp()

	// 测试已安装的 Codex
	path, err := app.GetToolConfigPath("codex")
	if err != nil {
		// Codex 未安装，跳过测试
		t.Skip("Codex 未安装，跳过测试")
	}

	assert.NotEmpty(t, path)
	assert.Contains(t, path, ".codex")
}

// TestGetToolConfigPath_Claude 测试获取 Claude 配置路径
func TestGetToolConfigPath_Claude(t *testing.T) {
	app := NewApp()

	// 测试已安装的 Claude
	path, err := app.GetToolConfigPath("claude")
	if err != nil {
		// Claude 未安装，跳过测试
		t.Skip("Claude 未安装，跳过测试")
	}

	assert.NotEmpty(t, path)
	assert.Contains(t, path, ".claude")
}

// TestGetToolConfigPath_Unknown 测试未知工具类型
func TestGetToolConfigPath_Unknown(t *testing.T) {
	app := NewApp()

	_, err := app.GetToolConfigPath("unknown")
	assert.Error(t, err, "未知工具类型应返回错误")
}

// TestGetToolConfigPath_NonExistent 测试不存在的工具配置
func TestGetToolConfigPath_NonExistent(t *testing.T) {
	app := NewApp()

	// 创建一个已知不存在的路径
	// 注意：这个测试依赖于系统中确实没有安装该工具
	_, err := app.GetToolConfigPath("codex")
	if err == nil {
		t.Skip("Codex 已安装，无法测试不存在的情况")
	}

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// TestApp_startup_shutdown 测试应用启动和关闭
func TestApp_startup_shutdown(t *testing.T) {
	app := NewApp()

	// 测试 startup
	ctx := context.Background()
	assert.NotPanics(t, func() {
		app.startup(ctx)
	})

	// 验证 context 被保存
	assert.Equal(t, ctx, app.ctx)

	// 测试 shutdown
	assert.NotPanics(t, func() {
		app.shutdown(ctx)
	})
}

// TestIsDirExists 测试目录存在性检查
func TestIsDirExists(t *testing.T) {
	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "ai-sync-dir-exists-test")
	_ = os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	// 测试存在的目录
	assert.True(t, isDirExists(tempDir))

	// 测试不存在的目录
	nonExistent := filepath.Join(os.TempDir(), "non-existent-dir-12345")
	assert.False(t, isDirExists(nonExistent))

	// 测试文件而非目录
	tempFile := filepath.Join(os.TempDir(), "test-file-12345")
	_ = os.WriteFile(tempFile, []byte("test"), 0644)
	defer os.Remove(tempFile)

	assert.False(t, isDirExists(tempFile), "文件应返回 false")
}

// Table驱动测试：GetToolConfigPath 各种输入
func TestGetToolConfigPath_Table(t *testing.T) {
	app := NewApp()

	tests := []struct {
		name      string
		toolType  string
		wantError bool
	}{
		{"Codex", "codex", false}, // 如果安装了
		{"Claude", "claude", false}, // 如果安装了
		{"未知工具", "unknown", true},
		{"空字符串", "", true},
		{"大小写混合", "CoDeX", true}, // 大小写敏感
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := app.GetToolConfigPath(tt.toolType)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				// 如果工具已安装，不应有错误
				// 如果工具未安装，这是预期行为
				if err != nil {
					t.Logf("工具 %s 未安装: %v", tt.toolType, err)
				} else {
					assert.NotEmpty(t, path)
				}
			}
		})
	}
}

// BenchmarkScanTools 性能基准测试
func BenchmarkScanTools(b *testing.B) {
	app := NewApp()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = app.ScanTools()
	}
}

// BenchmarkScanToolsWithProjects 性能基准测试
func BenchmarkScanToolsWithProjects(b *testing.B) {
	app := NewApp()

	// 创建测试项目
	tempDir := filepath.Join(os.TempDir(), "ai-sync-bench-project")
	_ = os.MkdirAll(filepath.Join(tempDir, ".git"), 0755)
	defer os.RemoveAll(tempDir)

	projectPaths := []string{tempDir}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = app.ScanToolsWithProjects(projectPaths)
	}
}
