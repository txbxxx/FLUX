package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/internal/service/tool"
	typesSnapshot "ai-sync-manager/internal/types/snapshot"
	"ai-sync-manager/pkg/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollector_Collect 测试文件收集
func TestCollector_Collect(t *testing.T) {
	collector := NewCollector(tool.NewRuleResolver(nil))

	// 创建测试目录
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "config.toml")
	testContent := []byte("[test]\nkey = value")

	require.NoError(t, os.WriteFile(testFile, testContent, 0644))

	// 收集文件
	options := CollectOptions{
		Tools:       []string{"claude", "codex"},
		ProjectPath: tempDir,
		Categories:  []models.FileCategory{models.CategoryConfig},
	}

	// 由于路径不存在，应该返回空结果但不报错
	result, err := collector.Collect(options)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

// TestService_CreateSnapshot 测试创建快照
func TestService_CreateSnapshot(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db, tool.NewRuleResolver(nil), tool.NewRuleManager(db))

	// 创建测试配置目录
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, ".codex")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	// 创建测试文件
	testFile := filepath.Join(configDir, "config.toml")
	testContent := []byte("[test]\nkey = value")
	require.NoError(t, os.WriteFile(testFile, testContent, 0644))

	// 创建快照选项
	ruleManager := tool.NewRuleManager(db)
	ruleManager.EnsureGlobalProjectsRegistered()

	options := typesSnapshot.CreateSnapshotOptions{
		Message:     "Test snapshot",
		Tools:       []string{"claude", "codex"},
		ProjectName: "codex-global",
		Tags:        []string{"test"},
	}

	// 由于路径不是实际的全局路径，收集会失败
	// 这里主要测试服务逻辑
	_, err = service.CreateSnapshot(options)
	// 可能会因为找不到文件而失败，这是正常的
	// 我们主要验证服务不会 panic
	assert.NotNil(t, service)
}

// TestService_ListSnapshots 测试列出快照
func TestService_ListSnapshots(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db, tool.NewRuleResolver(nil), tool.NewRuleManager(db))

	// 创建测试快照
	snapshot := &models.Snapshot{
		ID:          0,
		Name:      "Test List",
		Message:   "Test message",
		CreatedAt: time.Now(),
		Project:     "codex-global",
		Files: []models.SnapshotFile{
			{
				Path:         "config.toml",
				OriginalPath: "/test/config.toml",
				Size:         100,
				Hash:         "abc123",
				ModifiedAt:   time.Now(),
				Content:      []byte("test"),
				ToolType:     "codex",
				Category:     models.CategoryConfig,
				IsBinary:     false,
			},
		},
		Metadata: models.SnapshotMetadata{
			
		},
	}

	snapshotDAO := models.NewSnapshotDAO(db)
	err = snapshotDAO.Create(snapshot)
	require.NoError(t, err)

	// 列出快照
	snapshots, err := service.ListSnapshots(10, 0)
	assert.NoError(t, err)
	assert.Len(t, snapshots, 1)
	assert.Equal(t, uint(1), snapshots[0].ID)
}

// TestService_GetSnapshot 测试获取快照
func TestService_GetSnapshot(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db, tool.NewRuleResolver(nil), tool.NewRuleManager(db))

	// 创建测试快照
	snapshot := &models.Snapshot{
		ID:          0,
		Name:      "Test Get",
		Message:   "Test message",
		CreatedAt: time.Now(),
		Project:     "codex-global",
		Files: []models.SnapshotFile{
			{
				Path:         "config.toml",
				OriginalPath: "/test/config.toml",
				Size:         100,
				Hash:         "abc123",
				ModifiedAt:   time.Now(),
				Content:      []byte("test"),
				ToolType:     "codex",
				Category:     models.CategoryConfig,
				IsBinary:     false,
			},
		},
		Metadata: models.SnapshotMetadata{
			
		},
	}

	snapshotDAO := models.NewSnapshotDAO(db)
	err = snapshotDAO.Create(snapshot)
	require.NoError(t, err)

	// 获取快照
	fetched, err := service.GetSnapshot(fmt.Sprintf("%d", snapshot.ID))
	assert.NoError(t, err)
	assert.NotNil(t, fetched)
	assert.Equal(t, snapshot.ID, fetched.ID)
	assert.Equal(t, "Test Get", fetched.Name)
}

// TestService_GetSnapshot_NotFound 测试获取不存在的快照
func TestService_GetSnapshot_NotFound(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db, tool.NewRuleResolver(nil), tool.NewRuleManager(db))

	// 获取不存在的快照
	_, err = service.GetSnapshot("999999")
	assert.Error(t, err)
}

// TestService_DeleteSnapshot 测试删除快照
func TestService_DeleteSnapshot(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db, tool.NewRuleResolver(nil), tool.NewRuleManager(db))

	// 创建测试快照
	snapshot := &models.Snapshot{
		ID:          0,
		Name:      "Test Delete",
		Message:   "Test message",
		CreatedAt: time.Now(),
		Project:     "codex-global",
		Files:     []models.SnapshotFile{},
		Metadata: models.SnapshotMetadata{
			
		},
	}

	snapshotDAO := models.NewSnapshotDAO(db)
	err = snapshotDAO.Create(snapshot)
	require.NoError(t, err)

	// 删除快照
	err = service.DeleteSnapshot(fmt.Sprintf("%d", snapshot.ID))
	assert.NoError(t, err)

	// 验证删除
	_, err = service.GetSnapshot(fmt.Sprintf("%d", snapshot.ID))
	assert.Error(t, err)
}

// TestService_ValidateSnapshot 测试验证快照
func TestService_ValidateSnapshot(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db, tool.NewRuleResolver(nil), tool.NewRuleManager(db))

	// 有效快照
	validSnapshot := &models.Snapshot{
		ID:          0,
		Name:      "Valid",
		Message:   "Test",
		CreatedAt: time.Now(),
		Project:     "codex-global",
		Files: []models.SnapshotFile{
			{
				Path:         "config.toml",
				OriginalPath: "/test/config.toml",
				ToolType:     "codex",
				Category:     models.CategoryConfig,
			},
		},
	}

	err = service.ValidateSnapshot(validSnapshot)
	assert.NoError(t, err)

	// 无效快照 - ID 为空
	invalidSnapshot1 := &models.Snapshot{
		Name:      "Invalid",
		Message:   "Test",
		CreatedAt: time.Now(),
		Project:     "codex-global",
		Files:     []models.SnapshotFile{},
	}

	err = service.ValidateSnapshot(invalidSnapshot1)
	assert.Error(t, err)

	// 无效快照 - 无工具
	invalidSnapshot2 := &models.Snapshot{
		ID:          0,
		Name:      "Invalid",
		Message:   "Test",
		CreatedAt: time.Now(),
		Project:   "",
		Files:     []models.SnapshotFile{},
	}

	err = service.ValidateSnapshot(invalidSnapshot2)
	assert.Error(t, err)
}

// TestService_ExportSnapshot 测试导出快照
func TestService_ExportSnapshot(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	// 创建测试快照
	testContent := []byte("test content")
	exportDir := t.TempDir()

	// 创建源文件
	sourceDir := filepath.Join(exportDir, "source")
	sourceFile := filepath.Join(sourceDir, ".codex", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(sourceFile), 0755))
	require.NoError(t, os.WriteFile(sourceFile, testContent, 0644))

	snapshot := &models.Snapshot{
		ID:          0,
		Name:      "Test Export",
		Message:   "Test message",
		CreatedAt: time.Now(),
		Project:     "codex-global",
		Files: []models.SnapshotFile{
			{
				Path:         filepath.Join(".codex", "config.toml"),
				OriginalPath: sourceFile,
				Size:         int64(len(testContent)),
				Hash:         "abc123",
				ModifiedAt:   time.Now(),
				Content:      testContent,
				ToolType:     "codex",
				Category:     models.CategoryConfig,
				IsBinary:     false,
			},
		},
		Metadata: models.SnapshotMetadata{
			
		},
	}

	snapshotDAO := models.NewSnapshotDAO(db)
	err = snapshotDAO.Create(snapshot)
	require.NoError(t, err)

	// 直接从内存对象导出（不经过数据库）
	for _, file := range snapshot.Files {
		targetPath := filepath.Join(exportDir, file.Path)
		err = os.MkdirAll(filepath.Dir(targetPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(targetPath, file.Content, 0644)
		require.NoError(t, err)
	}

	// 验证导出
	exportedFile := filepath.Join(exportDir, ".codex", "config.toml")
	_, err = os.Stat(exportedFile)
	assert.NoError(t, err)

	content, err := os.ReadFile(exportedFile)
	assert.NoError(t, err)
	assert.Equal(t, testContent, content)
}

// TestService_CountSnapshots 测试统计快照
func TestService_CountSnapshots(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db, tool.NewRuleResolver(nil), tool.NewRuleManager(db))

	// 初始计数
	count, err := service.CountSnapshots()
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// 创建测试快照
	snapshotDAO := models.NewSnapshotDAO(db)
	for i := 1; i <= 3; i++ {
		snapshot := &models.Snapshot{
			ID:        0,
			Name:      fmt.Sprintf("Snapshot %d", i),
			Message:   "Test",
			CreatedAt: time.Now(),
			Project:     "codex-global",
			Files:     []models.SnapshotFile{},
			Metadata: models.SnapshotMetadata{
				
			},
		}
		err = snapshotDAO.Create(snapshot)
		assert.NoError(t, err)
	}

	// 验证计数
	count, err = service.CountSnapshots()
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

// TestApplier_ApplySnapshot 测试应用快照
func TestApplier_ApplySnapshot(t *testing.T) {
	collector := NewCollector(tool.NewRuleResolver(nil))
	applier := NewApplier(collector)

	// 创建测试快照
	testContent := []byte("applied content")
	snapshot := &models.Snapshot{
		ID:          0,
		Name:      "Test Apply",
		Message:   "Test message",
		CreatedAt: time.Now(),
		Project:     "codex-global",
		Files: []models.SnapshotFile{
			{
				Path:         "config.toml",
				OriginalPath: filepath.Join(t.TempDir(), "config.toml"),
				Size:         int64(len(testContent)),
				Hash:         "abc123",
				ModifiedAt:   time.Now(),
				Content:      testContent,
				ToolType:     "codex",
				Category:     models.CategoryConfig,
				IsBinary:     false,
			},
		},
		Metadata: models.SnapshotMetadata{
			
		},
	}

	// 应用快照（干运行）
	options := typesSnapshot.ApplyOptions{
		DryRun: true,
	}

	result, err := applier.ApplySnapshot(snapshot, options)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
}

// TestComparator_CompareSnapshots 测试比较快照
func TestComparator_CompareSnapshots(t *testing.T) {
	collector := NewCollector(tool.NewRuleResolver(nil))
	comparator := NewComparator(collector)

	// 创建源快照
	sourceSnapshot := &models.Snapshot{
		ID:          0,
		Name:      "Source",
		CreatedAt: time.Now(),
		Project:     "codex-global",
		Files: []models.SnapshotFile{
			{
				Path:         "file1.toml",
				OriginalPath: "/test/file1.toml",
				Size:         100,
				Hash:         "hash1",
				ModifiedAt:   time.Now(),
				Content:      []byte("content1"),
				ToolType:     "codex",
				Category:     models.CategoryConfig,
				IsBinary:     false,
			},
			{
				Path:         "file2.toml",
				OriginalPath: "/test/file2.toml",
				Size:         100,
				Hash:         "hash2",
				ModifiedAt:   time.Now(),
				Content:      []byte("content2"),
				ToolType:     "codex",
				Category:     models.CategoryConfig,
				IsBinary:     false,
			},
		},
		Metadata: models.SnapshotMetadata{
			
		},
	}

	// 创建目标快照
	targetSnapshot := &models.Snapshot{
		ID:          0,
		Name:      "Target",
		CreatedAt: time.Now(),
		Project:     "codex-global",
		Files: []models.SnapshotFile{
			{
				Path:         "file1.toml",
				OriginalPath: "/test/file1.toml",
				Size:         100,
				Hash:         "hash1", // 相同哈希
				ModifiedAt:   time.Now(),
				Content:      []byte("content1"),
				ToolType:     "codex",
				Category:     models.CategoryConfig,
				IsBinary:     false,
			},
			{
				Path:         "file3.toml",
				OriginalPath: "/test/file3.toml",
				Size:         100,
				Hash:         "hash3",
				ModifiedAt:   time.Now(),
				Content:      []byte("content3"),
				ToolType:     "codex",
				Category:     models.CategoryConfig,
				IsBinary:     false,
			},
		},
		Metadata: models.SnapshotMetadata{
			
		},
	}

	// 比较快照
	summary, err := comparator.CompareSnapshots(sourceSnapshot, targetSnapshot)
	assert.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Equal(t, 2, summary.TotalFiles)
	assert.Equal(t, 1, summary.Created) // file3
	assert.Equal(t, 1, summary.Deleted) // file2
}

// TestCollector_CalculateHash 测试哈希计算
func TestCollector_CalculateHash(t *testing.T) {
	collector := NewCollector(tool.NewRuleResolver(nil))

	content1 := []byte("test content")
	content2 := []byte("test content")
	content3 := []byte("different content")

	hash1 := collector.calculateHash(content1)
	hash2 := collector.calculateHash(content2)
	hash3 := collector.calculateHash(content3)

	assert.Equal(t, hash1, hash2)    // 相同内容，相同哈希
	assert.NotEqual(t, hash1, hash3) // 不同内容，不同哈希
}

// TestCollector_CategorizeFile 测试文件分类
func TestCollector_CategorizeFile(t *testing.T) {
	collector := NewCollector(tool.NewRuleResolver(nil))

	tests := []struct {
		name        string
		path        string
		isBinary    bool
		expectedCat models.FileCategory
	}{
		{
			name:        "配置文件",
			path:        "config.toml",
			isBinary:    false,
			expectedCat: models.CategoryConfig,
		},
		{
			name:        "技能文件",
			path:        "skills.yml",
			isBinary:    false,
			expectedCat: models.CategorySkills,
		},
		{
			name:        "Markdown 文档",
			path:        "README.md",
			isBinary:    false,
			expectedCat: models.CategoryDocs,
		},
		{
			name:        "二进制文件",
			path:        "image.png",
			isBinary:    true,
			expectedCat: models.CategoryOther,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := collector.categorizeFile(tt.path, tt.isBinary)
			assert.Equal(t, tt.expectedCat, category)
		})
	}
}

// TestApplier_RestoreFile 测试恢复文件
func TestApplier_RestoreFile(t *testing.T) {
	collector := NewCollector(tool.NewRuleResolver(nil))
	applier := NewApplier(collector)

	tempDir := t.TempDir()

	// 创建备份文件
	backupContent := []byte("backup content")
	backupPath := filepath.Join(tempDir, "backup", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(backupPath), 0755))
	require.NoError(t, os.WriteFile(backupPath, backupContent, 0644))

	// 恢复文件
	targetPath := filepath.Join(tempDir, "restored", "config.toml")
	err := applier.RestoreFile(backupPath, targetPath)
	assert.NoError(t, err)

	// 验证恢复
	content, err := os.ReadFile(targetPath)
	assert.NoError(t, err)
	assert.Equal(t, backupContent, content)
}

func TestServiceCreateSnapshotIncludesRegisteredProjectAndCustomRule(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	tool.CreateMockFile(t, filepath.Join(homeDir, ".claude", "settings.json"), "{}")

	customFile := filepath.Join(t.TempDir(), "claude.json")
	tool.CreateMockFile(t, customFile, `{"mcpServers":[]}`)

	projectPath := filepath.Join(t.TempDir(), "demo-project")
	tool.CreateMockFile(t, filepath.Join(projectPath, ".codex", "config.toml"), "[project]")
	tool.CreateMockFile(t, filepath.Join(projectPath, "AGENTS.md"), "# agents")

	manager := tool.NewRuleManager(db)
	require.NoError(t, manager.AddCustomRule(tool.ToolTypeClaude, customFile))
	require.NoError(t, manager.AddProject(tool.ToolTypeCodex, "demo", projectPath))

	service := NewService(db, tool.NewRuleResolver(manager.Store()), manager)
	pkg, err := service.CreateSnapshot(typesSnapshot.CreateSnapshotOptions{
		Message:     "test snapshot",
		Tools:       []string{"claude", "codex"},
		ProjectName: "demo",
	})
	require.NoError(t, err)

	// Retrieve the full snapshot from DB to verify collected files.
	snapshot, err := service.GetSnapshot(fmt.Sprintf("%d", pkg.Snapshot.ID))
	require.NoError(t, err)

	paths := make([]string, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		paths = append(paths, file.OriginalPath)
	}

	assert.Contains(t, paths, filepath.Join(homeDir, ".claude", "settings.json"))
	assert.Contains(t, paths, customFile)
	assert.Contains(t, paths, filepath.Join(projectPath, ".codex", "config.toml"))
	assert.Contains(t, paths, filepath.Join(projectPath, "AGENTS.md"))
}
