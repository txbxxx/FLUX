package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewSnapshot 测试创建快照
func TestNewSnapshot(t *testing.T) {
	message := "Test snapshot"
	projectName := "codex-global"

	snapshot := NewSnapshot(message, projectName)

	assert.NotNil(t, snapshot)
	assert.Equal(t, uint(0), snapshot.ID) // NewSnapshot sets ID to 0, auto-filled by GORM on Create
	assert.Equal(t, message, snapshot.Message)
	assert.Equal(t, projectName, snapshot.Project)
	assert.Equal(t, projectName, snapshot.Metadata.ProjectPath)
	assert.False(t, snapshot.CreatedAt.IsZero())
}

// TestValidateSnapshot 测试验证快照
func TestValidateSnapshot(t *testing.T) {
	tests := []struct {
		name     string
		snapshot *Snapshot
		wantErr  bool
	}{
		{
			name: "有效快照",
			snapshot: &Snapshot{
				Message: "Valid snapshot",
				Project: "codex-global",
			},
			wantErr: false,
		},
		{
			name:     "空快照",
			snapshot: nil,
			wantErr:  true,
		},
		{
			name: "空消息",
			snapshot: &Snapshot{
				Project: "codex-global",
			},
			wantErr: true,
		},
		{
			name: "空项目",
			snapshot: &Snapshot{
				Message: "Test",
				Project: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSnapshot(tt.snapshot)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateRemoteConfig 测试验证远端配置
func TestValidateRemoteConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *RemoteConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			config: &RemoteConfig{
				URL:    "https://github.com/user/repo.git",
				Branch: "main",
			},
			wantErr: false,
		},
		{
			name:    "空配置",
			config:  nil,
			wantErr: true,
		},
		{
			name: "空 URL",
			config: &RemoteConfig{
				Branch: "main",
			},
			wantErr: true,
		},
		{
			name: "无效 URL",
			config: &RemoteConfig{
				URL:    "not-a-valid-url",
				Branch: "main",
			},
			wantErr: true,
		},
		{
			name: "空分支（应使用默认）",
			config: &RemoteConfig{
				URL: "https://github.com/user/repo.git",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRemoteConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateSyncConfig 测试验证同步配置
func TestValidateSyncConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *SyncConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			config: &SyncConfig{
				AutoSync:     false,
				SyncInterval: time.Minute * 5,
			},
			wantErr: false,
		},
		{
			name:    "空配置",
			config:  nil,
			wantErr: true,
		},
		{
			name: "自动同步但无间隔",
			config: &SyncConfig{
				AutoSync:     true,
				SyncInterval: 0,
			},
			wantErr: true,
		},
		{
			name: "间隔过短",
			config: &SyncConfig{
				AutoSync:     true,
				SyncInterval: time.Second * 30,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSyncConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestFilterFilesByCategory 测试按类别过滤文件
func TestFilterFilesByCategory(t *testing.T) {
	files := []SnapshotFile{
		{Path: "config.toml", Category: CategoryConfig},
		{Path: "skill.js", Category: CategorySkills},
		{Path: "command.sh", Category: CategoryCommands},
	}

	// 测试过滤单个类别
	result := FilterFilesByCategory(files, []FileCategory{CategoryConfig})
	assert.Len(t, result, 1)
	assert.Equal(t, "config.toml", result[0].Path)

	// 测试过滤多个类别
	result = FilterFilesByCategory(files, []FileCategory{CategoryConfig, CategorySkills})
	assert.Len(t, result, 2)

	// 测试空类别（应返回所有）
	result = FilterFilesByCategory(files, []FileCategory{})
	assert.Len(t, result, 3)
}

// TestFilterFilesByTool 测试按工具过滤文件
func TestFilterFilesByTool(t *testing.T) {
	files := []SnapshotFile{
		{Path: "codex/config.toml", ToolType: "codex"},
		{Path: "claude/config.json", ToolType: "claude"},
		{Path: "shared/readme.md", ToolType: "codex"},
	}

	// 测试过滤单个工具
	result := FilterFilesByTool(files, []string{"codex"})
	assert.Len(t, result, 2)

	// 测试过滤多个工具
	result = FilterFilesByTool(files, []string{"codex", "claude"})
	assert.Len(t, result, 3)

	// 测试空工具（应返回所有）
	result = FilterFilesByTool(files, []string{})
	assert.Len(t, result, 3)
}

// TestGetFileExtension 测试获取文件扩展名
func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"config.toml", "toml"},
		{"script.js", "js"},
		{"README.md", "md"},
		{".gitignore", "gitignore"},
		{"noextension", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := GetFileExtension(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsConfigFile 测试判断配置文件
func TestIsConfigFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"config.toml", true},
		{"settings.yaml", true},
		{"config.json", true},
		{"app.ini", true},
		{"README.md", false},
		{"script.js", false},
		{"config", true},
		{"settings", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsConfigFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsBinaryFile 测试判断二进制文件
func TestIsBinaryFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"app.exe", true},
		{"lib.dll", true},
		{"archive.zip", true},
		{"document.pdf", true},
		{"config.toml", false},
		{"README.md", false},
		{"script.js", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsBinaryFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFormatDuration 测试格式化时长
func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"毫秒", 500 * time.Millisecond, "500ms"},
		{"秒", 1500 * time.Millisecond, "1.5s"},
		{"分钟", 90 * time.Second, "1.5m"},
		{"小时", 5400 * time.Second, "1.5h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDuration(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFormatBytes 测试格式化字节数
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"字节", 500, "500 B"},
		{"千字节", 1536, "1.5 KiB"},
		{"兆字节", 1572864, "1.5 MiB"},
		{"吉字节", 1610612736, "1.5 GiB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCloneSnapshot 测试克隆快照
func TestCloneSnapshot(t *testing.T) {
	original := &Snapshot{
		ID:          0,
		Name:      "Test Snapshot",
		Message:   "Test message",
		Project:   "codex-global",
		Tags:      []string{"backup", "test"},
		CreatedAt: time.Now(),
	}

	clone := CloneSnapshot(original)

	assert.NotNil(t, clone)
	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.Name, clone.Name)
	assert.Equal(t, original.Message, clone.Message)
	assert.Equal(t, original.Project, clone.Project)
	assert.Equal(t, original.Tags, clone.Tags)

	// 验证是深拷贝
	clone.Tags = append(clone.Tags, "new-tag")
	assert.Len(t, original.Tags, 2)
	assert.Len(t, clone.Tags, 3)
}

// TestMergeSummary 测试合并变更摘要
// 已迁移到 types/snapshot 包
// func TestMergeSummary(t *testing.T) { ... }

// TestNewTaskProgress 测试创建任务进度
func TestNewTaskProgress(t *testing.T) {
	progress := NewTaskProgress(5, 10, "Processing")

	assert.Equal(t, 50, progress.Percentage)
	assert.Equal(t, 5, progress.Current)
	assert.Equal(t, 10, progress.Total)
	assert.Equal(t, "Processing", progress.Message)
}

// TestTaskProgress_AddStep 测试添加步骤
func TestTaskProgress_AddStep(t *testing.T) {
	progress := NewTaskProgress(0, 2, "")

	progress.AddStep("Step 1")
	progress.AddStep("Step 2")

	assert.Len(t, progress.Steps, 2)
	assert.Equal(t, "Step 1", progress.Steps[0])
	assert.Equal(t, "Step 2", progress.Steps[1])
}

// TestTaskProgress_UpdateProgress 测试更新进度
func TestTaskProgress_UpdateProgress(t *testing.T) {
	progress := NewTaskProgress(0, 10, "")

	progress.UpdateProgress(5, 10, "Halfway")

	assert.Equal(t, 50, progress.Percentage)
	assert.Equal(t, 5, progress.Current)
	assert.Equal(t, "Halfway", progress.Message)
}

// TestTaskProgress_IsCompleted 测试检查完成状态
func TestTaskProgress_IsCompleted(t *testing.T) {
	progress := NewTaskProgress(5, 10, "")

	assert.False(t, progress.IsCompleted())

	progress.Current = 10
	assert.True(t, progress.IsCompleted())
}

// Table驱动测试：文件类别
func TestFileCategory_Values(t *testing.T) {
	tests := []struct {
		name     string
		category FileCategory
		expected string
	}{
		{"配置", CategoryConfig, "config"},
		{"技能", CategorySkills, "skills"},
		{"命令", CategoryCommands, "commands"},
		{"插件", CategoryPlugins, "plugins"},
		{"MCP", CategoryMCP, "mcp"},
		{"代理", CategoryAgents, "agents"},
		{"规则", CategoryRules, "rules"},
		{"文档", CategoryDocs, "docs"},
		{"输出", CategoryOutput, "output"},
		{"其他", CategoryOther, "other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.category))
		})
	}
}

// Table驱动测试：同步方向
func TestSyncDirection_Values(t *testing.T) {
	tests := []struct {
		name      string
		direction SyncDirection
		expected  string
	}{
		{"上传", DirectionUpload, "upload"},
		{"下载", DirectionDownload, "download"},
		{"双向", DirectionBoth, "both"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.direction))
		})
	}
}

// Table驱动测试：任务状态
func TestSyncTaskStatus_Values(t *testing.T) {
	tests := []struct {
		name     string
		status   SyncTaskStatus
		expected string
	}{
		{"等待", StatusPending, "pending"},
		{"运行中", StatusRunning, "running"},
		{"已完成", StatusCompleted, "completed"},
		{"失败", StatusFailed, "failed"},
		{"已取消", StatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}
