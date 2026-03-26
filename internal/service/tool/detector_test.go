package tool

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"ai-sync-manager/pkg/logger"

	"github.com/stretchr/testify/assert"
)

func init() {
	// 初始化日志用于测试
	_ = logger.Init(logger.DefaultConfig())
}

// TestNewToolDetector 测试检测器创建
func TestNewToolDetector(t *testing.T) {
	detector := NewToolDetector()
	assert.NotNil(t, detector)
}

// TestDetectWithOptions_EmptyResult 测试无工具安装时的空结果
func TestDetectWithOptions_EmptyResult(t *testing.T) {
	ctx := context.Background()
	detector := NewToolDetector()

	// 使用临时目录作为主目录
	tempHome := filepath.Join(os.TempDir(), "ai-sync-empty-home-test")
	_ = os.MkdirAll(tempHome, 0755)
	defer os.RemoveAll(tempHome)

	// 修改用户主目录为临时目录进行测试
	oldHome := os.Getenv("HOME")
	oldUserHome := os.Getenv("USERPROFILE")
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("USERPROFILE", oldUserHome)
	}()

	if runtime.GOOS == "windows" {
		_ = os.Setenv("USERPROFILE", tempHome)
	} else {
		_ = os.Setenv("HOME", tempHome)
	}

	opts := &ScanOptions{
		ScanGlobal:   true,
		ScanProjects: false,
		IncludeFiles: true,
		MaxDepth:     1,
	}

	result, err := detector.DetectWithOptions(ctx, opts)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// 空目录时，工具状态应为 not_installed
	if result.Codex != nil {
		assert.Equal(t, StatusNotInstalled, result.Codex.Status)
	}
	if result.Claude != nil {
		assert.Equal(t, StatusNotInstalled, result.Claude.Status)
	}
}

// TestDetectWithOptions_CodexGlobal 测试检测 Codex 全局配置
func TestDetectWithOptions_CodexGlobal(t *testing.T) {
	ctx := context.Background()
	detector := NewToolDetector()

	opts := &ScanOptions{
		ScanGlobal:   true,
		ScanProjects: false,
		IncludeFiles: true,
		MaxDepth:     1,
	}

	result, err := detector.DetectWithOptions(ctx, opts)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 验证不会崩溃，实际检测结果取决于系统环境
	_ = result.Codex
	_ = result.Claude
}

// TestDetectWithOptions_ProjectScope 测试项目作用域检测
func TestDetectWithOptions_ProjectScope(t *testing.T) {
	ctx := context.Background()
	detector := NewToolDetector()

	// 创建模拟项目
	mockProject := CreateMockProject(t, "test-project", true, false)
	defer CleanupMockProjects(t)

	opts := &ScanOptions{
		ScanGlobal:   false,
		ScanProjects: true,
		ProjectPaths: []string{mockProject},
		IncludeFiles: true,
		MaxDepth:     1,
	}

	result, err := detector.DetectWithOptions(ctx, opts)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 验证项目路径被记录
	assert.NotEmpty(t, result.Projects, "应检测到项目信息")
	assert.Len(t, result.ProjectInstallations, 1, "应返回独立项目扫描对象")
	assert.Equal(t, ScopeProject, result.ProjectInstallations[0].Scope)
	assert.Equal(t, ToolTypeCodex, result.ProjectInstallations[0].ToolType)
}

// TestDetectWithOptions_ContextCancellation 测试上下文取消
func TestDetectWithOptions_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	detector := NewToolDetector()

	// 立即取消上下文
	cancel()

	opts := &ScanOptions{
		ScanGlobal:   true,
		ScanProjects: false,
		IncludeFiles: true,
		MaxDepth:     1,
	}

	_, err := detector.DetectWithOptions(ctx, opts)
	// 取消可能返回错误或空结果
	_ = err
}

// TestDetectWithOptions_Timeout 测试超时控制
func TestDetectWithOptions_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	detector := NewToolDetector()

	opts := &ScanOptions{
		ScanGlobal:   true,
		ScanProjects: false,
		IncludeFiles: true,
		MaxDepth:     100,
	}

	// 等待超时
	time.Sleep(10 * time.Millisecond)

	_, err := detector.DetectWithOptions(ctx, opts)
	// 验证不会崩溃
	assert.NotPanics(t, func() {
		detector.DetectWithOptions(ctx, opts)
	})
	_ = err
}

// TestToInstallations 测试转换为安装信息列表
func TestToInstallations(t *testing.T) {
	result := &ToolDetectionResult{
		Codex: &ToolInstallation{
			Scope:      ScopeGlobal,
			ToolType:   ToolTypeCodex,
			Status:     StatusInstalled,
			GlobalPath: filepath.Join(GetUserHomeDirForTest(), ".codex"),
			ConfigFiles: []ConfigFile{
				{
					Name:  "config.toml",
					Path:  filepath.Join(GetUserHomeDirForTest(), ".codex", "config.toml"),
					Scope: ScopeGlobal,
					IsDir: false,
					Size:  1024,
				},
			},
		},
		ProjectInstallations: []*ToolInstallation{
			{
				Scope:       ScopeProject,
				ToolType:    ToolTypeCodex,
				Status:      StatusInstalled,
				ProjectName: "test",
				ProjectPath: "D:\\test\\project",
				ConfigFiles: []ConfigFile{
					{
						Name:  "AGENTS.md",
						Path:  "D:\\test\\project\\AGENTS.md",
						Scope: ScopeProject,
						IsDir: false,
						Size:  128,
					},
				},
			},
		},
		Projects: []ProjectInfo{{Path: "D:\\test\\project", Name: "test"}},
	}

	installations := result.ToInstallations()

	assert.Len(t, installations, 2, "应返回全局对象和项目对象两个扫描结果")

	globalCodex := installations[0]
	assert.Equal(t, ToolTypeCodex, globalCodex.ToolType)
	assert.Equal(t, ScopeGlobal, globalCodex.Scope)
	assert.Equal(t, StatusInstalled, globalCodex.Status)
	assert.NotEmpty(t, globalCodex.GlobalPath)
	assert.NotEmpty(t, globalCodex.ConfigFiles)

	projectCodex := installations[1]
	assert.Equal(t, ToolTypeCodex, projectCodex.ToolType)
	assert.Equal(t, ScopeProject, projectCodex.Scope)
	assert.Equal(t, "test", projectCodex.ProjectName)
	assert.Equal(t, "D:\\test\\project", projectCodex.ProjectPath)
	assert.Len(t, projectCodex.ConfigFiles, 1)
}

// TestToInstallations_EmptyResult 测试空结果的转换
func TestToInstallations_EmptyResult(t *testing.T) {
	result := &ToolDetectionResult{
		Codex:    nil,
		Claude:   nil,
		Projects: []ProjectInfo{},
	}

	installations := result.ToInstallations()
	assert.Empty(t, installations, "空结果应返回空列表")
}

// TestToInstallations_NotInstalled 测试未安装工具的过滤
func TestToInstallations_NotInstalled(t *testing.T) {
	result := &ToolDetectionResult{
		Codex: &ToolInstallation{
			ToolType: ToolTypeCodex,
			Status:   StatusNotInstalled,
		},
		Claude: &ToolInstallation{
			ToolType: ToolTypeClaude,
			Status:   StatusInstalled,
		},
	}

	installations := result.ToInstallations()

	assert.Len(t, installations, 1, "应仅返回已安装的工具")
	if len(installations) > 0 {
		assert.Equal(t, ToolTypeClaude, installations[0].ToolType)
	}
}

// TestDetectWithOptions_WithDepth 测试深度控制
func TestDetectWithOptions_WithDepth(t *testing.T) {
	ctx := context.Background()
	detector := NewToolDetector()

	// 创建包含深层嵌套的模拟项目
	mockProject := CreateMockProject(t, "deep-project", true, true)
	defer CleanupMockProjects(t)

	// 创建深层文件
	deepDir := filepath.Join(mockProject, ".codex", "skills", "nested", "deep")
	_ = os.MkdirAll(deepDir, 0755)
	deepFile := filepath.Join(deepDir, "deep.md")
	_ = os.WriteFile(deepFile, []byte("# Deep file"), 0644)

	tests := []struct {
		name     string
		maxDepth int
	}{
		{"零深度", 0},
		{"一级深度", 1},
		{"二级深度", 2},
		{"无限深度", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &ScanOptions{
				ScanGlobal:   false,
				ScanProjects: true,
				ProjectPaths: []string{mockProject},
				IncludeFiles: true,
				MaxDepth:     tt.maxDepth,
			}

			result, err := detector.DetectWithOptions(ctx, opts)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			// 深度控制不应导致崩溃
		})
	}
}

// Table驱动测试：配置文件扫描
func TestDetectWithOptions_ConfigFiles(t *testing.T) {
	ctx := context.Background()
	detector := NewToolDetector()

	tests := []struct {
		name       string
		toolType   ToolType
		scope      ConfigScope
		wantConfig bool
	}{
		{"Codex全局配置", ToolTypeCodex, ScopeGlobal, true},
		{"Codex项目配置", ToolTypeCodex, ScopeProject, true},
		{"Claude全局配置", ToolTypeClaude, ScopeGlobal, true},
		{"Claude项目配置", ToolTypeClaude, ScopeProject, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDir := CreateMockConfigDir(t, tt.toolType, tt.scope)
			defer CleanupMockDir(t, mockDir)

			opts := &ScanOptions{
				ScanGlobal:   tt.scope == ScopeGlobal,
				ScanProjects: tt.scope == ScopeProject,
				IncludeFiles: true,
				MaxDepth:     1,
			}

			if tt.scope == ScopeProject {
				opts.ProjectPaths = []string{mockDir}
			}

			result, err := detector.DetectWithOptions(ctx, opts)
			assert.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

// TestDetectWithOptions_MultipleProjects 测试多项目扫描
func TestDetectWithOptions_MultipleProjects(t *testing.T) {
	ctx := context.Background()
	detector := NewToolDetector()

	// 创建多个模拟项目
	project1 := CreateMockProject(t, "project1", true, false)
	project2 := CreateMockProject(t, "project2", false, true)
	project3 := CreateMockProject(t, "project3", true, true)
	defer CleanupMockProjects(t)

	opts := &ScanOptions{
		ScanGlobal:   false,
		ScanProjects: true,
		ProjectPaths: []string{project1, project2, project3},
		IncludeFiles: true,
		MaxDepth:     1,
	}

	result, err := detector.DetectWithOptions(ctx, opts)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 验证所有项目都被记录
	assert.Len(t, result.Projects, 3, "应检测到3个项目")

	projectPaths := make(map[string]bool)
	for _, p := range result.Projects {
		projectPaths[p.Path] = true
	}

	assert.True(t, projectPaths[project1], "应包含 project1")
	assert.True(t, projectPaths[project2], "应包含 project2")
	assert.True(t, projectPaths[project3], "应包含 project3")
	assert.Len(t, result.ProjectInstallations, 4, "应为每个工具类型生成独立项目扫描对象")
}

// TestDetectWithOptions_InvalidPaths 测试无效路径处理
func TestDetectWithOptions_InvalidPaths(t *testing.T) {
	ctx := context.Background()
	detector := NewToolDetector()

	opts := &ScanOptions{
		ScanGlobal:   false,
		ScanProjects: true,
		ProjectPaths: []string{
			"",                           // 空路径
			"/nonexistent/path/12345",    // 不存在的路径
			"\\\\invalid\\network\\path", // 无效网络路径
		},
		IncludeFiles: true,
		MaxDepth:     1,
	}

	result, err := detector.DetectWithOptions(ctx, opts)
	// 无效路径不应导致崩溃，应优雅处理
	assert.NoError(t, err, "无效路径应优雅处理")
	assert.NotNil(t, result)
	assert.Empty(t, result.Projects, "无效路径应返回空项目列表")
}

// TestDetectTool_Single 测试单个工具检测
func TestDetectTool_Single(t *testing.T) {
	ctx := context.Background()
	detector := NewToolDetector()

	opts := &ScanOptions{
		ScanGlobal:   true,
		IncludeFiles: true,
		MaxDepth:     1,
	}

	// 测试检测 Codex
	codex, err := detector.DetectTool(ctx, ToolTypeCodex, opts)
	assert.NoError(t, err)
	assert.NotNil(t, codex)
	assert.Equal(t, ToolTypeCodex, codex.ToolType)

	// 测试检测 Claude
	claude, err := detector.DetectTool(ctx, ToolTypeClaude, opts)
	assert.NoError(t, err)
	assert.NotNil(t, claude)
	assert.Equal(t, ToolTypeClaude, claude.ToolType)
}

// TestDetectAll 测试检测所有工具
func TestDetectAll(t *testing.T) {
	ctx := context.Background()
	detector := NewToolDetector()

	result, err := detector.DetectAll(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 验证结果结构
	_ = result.Codex
	_ = result.Claude
}

// TestWalkConfigDir 测试配置目录遍历
func TestWalkConfigDir(t *testing.T) {
	detector := NewToolDetector()

	// 创建临时目录
	tempDir := filepath.Join(os.TempDir(), "ai-sync-walk-test")
	_ = os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	// 创建一些测试文件和目录
	_ = os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("test"), 0644)
	_ = os.MkdirAll(filepath.Join(tempDir, ".hidden"), 0755)

	err := detector.WalkConfigDir(tempDir)
	assert.NoError(t, err, "遍历不应返回错误")
}

// TestDefaultScanOptions 测试默认扫描选项
func TestDefaultScanOptions(t *testing.T) {
	opts := DefaultScanOptions()

	assert.NotNil(t, opts)
	assert.True(t, opts.ScanGlobal)
	assert.False(t, opts.ScanProjects)
	assert.True(t, opts.IncludeFiles)
	assert.Equal(t, 1, opts.MaxDepth)
}

// BenchmarkDetectWithOptions 性能基准测试
func BenchmarkDetectWithOptions(b *testing.B) {
	ctx := context.Background()
	detector := NewToolDetector()

	opts := &ScanOptions{
		ScanGlobal:   true,
		ScanProjects: false,
		IncludeFiles: true,
		MaxDepth:     1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = detector.DetectWithOptions(ctx, opts)
	}
}
