package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-sync-manager/internal/database"
	"ai-sync-manager/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPackager_ValidateSnapshotForPush 测试快照验证
func TestPackager_ValidateSnapshotForPush(t *testing.T) {
	packager := NewPackager()

	// 有效快照
	validSnapshot := &models.Snapshot{
		ID: "test-valid-1",
		Files: []models.SnapshotFile{
			{
				Path:         "config.toml",
				OriginalPath: "/test/config.toml",
			},
		},
	}

	err := packager.ValidateSnapshotForPush(validSnapshot)
	assert.NoError(t, err)

	// 无效快照 - ID 为空
	invalidSnapshot1 := &models.Snapshot{
		ID:    "",
		Files: []models.SnapshotFile{{Path: "test"}},
	}

	err = packager.ValidateSnapshotForPush(invalidSnapshot1)
	assert.Error(t, err)

	// 无效快照 - 无文件
	invalidSnapshot2 := &models.Snapshot{
		ID:    "test-id",
		Files: []models.SnapshotFile{},
	}

	err = packager.ValidateSnapshotForPush(invalidSnapshot2)
	assert.Error(t, err)

	// 无效快照 - 文件路径为空
	invalidSnapshot3 := &models.Snapshot{
		ID: "test-id",
		Files: []models.SnapshotFile{
			{
				Path:         "",
				OriginalPath: "/test/test",
			},
		},
	}

	err = packager.ValidateSnapshotForPush(invalidSnapshot3)
	assert.Error(t, err)

	// 无效快照 - 原始路径为空
	invalidSnapshot4 := &models.Snapshot{
		ID: "test-id",
		Files: []models.SnapshotFile{
			{
				Path:         "test",
				OriginalPath: "",
			},
		},
	}

	err = packager.ValidateSnapshotForPush(invalidSnapshot4)
	assert.Error(t, err)
}

// TestPackager_CreateCommitMessage 测试创建提交消息
func TestPackager_CreateCommitMessage(t *testing.T) {
	packager := NewPackager()

	snapshot := &models.Snapshot{
		ID:          "test-id-123",
		Name:        "Test Snapshot",
		Description: "This is a test snapshot for testing",
		Tools:       []string{"codex", "cursor"},
		Tags:        []string{"test", "unit"},
		Files:       []models.SnapshotFile{{Path: "file1"}, {Path: "file2"}},
	}

	message := packager.CreateCommitMessage(snapshot)

	assert.Contains(t, message, "Snapshot: Test Snapshot")
	assert.Contains(t, message, "ID: test-id-123")
	assert.Contains(t, message, "This is a test snapshot for testing")
	// 注意: strings.Join 会用逗号+空格连接
	assert.Contains(t, message, "Tools: codex, cursor")
	assert.Contains(t, message, "Files: 2")
	assert.Contains(t, message, "Tags: test, unit")
}

// TestPackager_ParseSnapshotIDFromCommit 测试解析提交消息中的快照 ID
func TestPackager_ParseSnapshotIDFromCommit(t *testing.T) {
	packager := NewPackager()

	commitMessage := `Snapshot: Test Snapshot
ID: test-snapshot-id-123

This is a test snapshot

Tools: codex
Files: 5`

	snapshotID := packager.ParseSnapshotIDFromCommit(commitMessage)
	assert.Equal(t, "test-snapshot-id-123", snapshotID)

	// 测试没有 ID 的消息
	noIDMessage := "Just a regular commit message"
	snapshotID = packager.ParseSnapshotIDFromCommit(noIDMessage)
	assert.Equal(t, "", snapshotID)
}

// TestPackager_IsSnapshotCommit 测试检查是否为快照提交
func TestPackager_IsSnapshotCommit(t *testing.T) {
	packager := NewPackager()

	// 快照提交
	snapshotCommit1 := "Snapshot: My Snapshot\nID: abc-123"
	assert.True(t, packager.IsSnapshotCommit(snapshotCommit1))

	snapshotCommit2 := "Some message\nsnapshot_id: xyz-789"
	assert.True(t, packager.IsSnapshotCommit(snapshotCommit2))

	// 非快照提交
	regularCommit := "Just a regular commit message"
	assert.False(t, packager.IsSnapshotCommit(regularCommit))
}

// TestPackager_CreateIndex 测试创建快照索引
func TestPackager_CreateIndex(t *testing.T) {
	packager := NewPackager()

	snapshot := &models.Snapshot{
		ID:          "test-index-1",
		Name:        "Test Index",
		Description: "Test description",
		CreatedAt:   time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		Tools:       []string{"codex"},
		Files: []models.SnapshotFile{
			{Path: "config.toml"},
			{Path: "skills.yml"},
		},
	}

	index := packager.CreateIndex(snapshot)

	assert.Equal(t, "test-index-1", index.SnapshotID)
	assert.Len(t, index.FilePaths, 2)
	assert.Equal(t, "config.toml", index.FilePaths[0])
	assert.Equal(t, "skills.yml", index.FilePaths[1])
	assert.Equal(t, "Test Index", index.Metadata["name"])
	assert.Equal(t, "Test description", index.Metadata["description"])
	assert.Equal(t, "codex", index.Metadata["tools"])
}

// TestPackager_ExtractSnapshotID 测试提取快照 ID
func TestPackager_ExtractSnapshotID(t *testing.T) {
	packager := NewPackager()

	// UUID 路径 - 使用正斜杠因为代码检查 ".ai-sync/snapshots/"
	uuidPath := ".ai-sync/snapshots/550e8400-e29b-41d4-a716-446655440000/file.toml"
	id := packager.ExtractSnapshotID(uuidPath)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", id)

	// 非 UUID 路径
	nonUUIDPath := ".ai-sync/snapshots/config/file.toml"
	id = packager.ExtractSnapshotID(nonUUIDPath)
	assert.Equal(t, "", id)

	// 无效路径
	invalidPath := "random/path"
	id = packager.ExtractSnapshotID(invalidPath)
	assert.Equal(t, "", id)
}

// TestPackager_ReadSnapshotManifest 测试读取清单文件
func TestPackager_ReadSnapshotManifest(t *testing.T) {
	packager := NewPackager()
	tempDir := t.TempDir()

	// 创建测试清单文件
	manifestDir := filepath.Join(tempDir, ".ai-sync")
	require.NoError(t, os.MkdirAll(manifestDir, 0755))

	manifestPath := filepath.Join(manifestDir, "manifest.json")
	manifestContent := `{
  "snapshot-1": {
    "name": "Snapshot 1",
    "description": "First snapshot",
    "commit_hash": "abc123",
    "created_at": "2026-03-20T10:00:00Z"
  },
  "snapshot-2": {
    "name": "Snapshot 2",
    "description": "Second snapshot",
    "commit_hash": "def456",
    "created_at": "2026-03-20T11:00:00Z"
  }
}`
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0644))

	// 读取清单
	manifest, err := packager.ReadSnapshotManifest(tempDir)
	require.NoError(t, err)
	assert.NotNil(t, manifest)

	// 验证内容
	assert.Len(t, manifest, 2)

	snapshot1, ok1 := manifest["snapshot-1"].(map[string]interface{})
	require.True(t, ok1)
	assert.Equal(t, "Snapshot 1", snapshot1["name"])

	snapshot2, ok2 := manifest["snapshot-2"].(map[string]interface{})
	require.True(t, ok2)
	assert.Equal(t, "Snapshot 2", snapshot2["name"])
}

// TestPackager_ReadSnapshotManifest_NotFound 测试读取不存在的清单文件
func TestPackager_ReadSnapshotManifest_NotFound(t *testing.T) {
	packager := NewPackager()
	tempDir := t.TempDir()

	// 读取不存在的清单
	_, err := packager.ReadSnapshotManifest(tempDir)
	assert.Error(t, err)
}

// TestService_PushSnapshot 测试推送快照
func TestService_PushSnapshot(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	ctx := context.Background()
	snapshot := &models.Snapshot{
		ID:        "test-push-1",
		Name:      "Test Push",
		Message:   "Test message",
		CreatedAt: time.Now(),
		Tools:     []string{"codex"},
		Files: []models.SnapshotFile{
			{
				Path:         "config.toml",
				OriginalPath: "/test/config.toml",
				Size:         100,
				Hash:         "abc123",
				ModifiedAt:   time.Now(),
				Content:      []byte("test content"),
				ToolType:     "codex",
				Category:     models.CategoryConfig,
				IsBinary:     false,
			},
		},
		Metadata: models.SnapshotMetadata{
			Scope:       models.ScopeGlobal,
			ProjectPath: t.TempDir(),
		},
	}

	options := SyncOptions{
		RemoteURL:  "https://github.com/test/repo.git",
		Branch:     "main",
		RemoteName: "origin",
	}

	// 由于没有真实的 Git 仓库，这里主要测试验证逻辑
	result, err := service.PushSnapshot(ctx, snapshot, options)

	// 应该失败（因为没有真实仓库），但不应 panic
	assert.NotNil(t, result)
	// 可能会因为仓库操作失败
	_ = err
}

// TestService_PullSnapshots 测试拉取快照
func TestService_PullSnapshots(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	ctx := context.Background()
	repoPath := t.TempDir()

	options := SyncOptions{
		RemoteURL:  "https://github.com/test/repo.git",
		Branch:     "main",
		RemoteName: "origin",
	}

	result, err := service.PullSnapshots(ctx, repoPath, options)

	// 应该返回结果（即使没有真实仓库）
	assert.NotNil(t, result)
	_ = err
}

// TestService_GetSyncStatus 测试获取同步状态
func TestService_GetSyncStatus(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	ctx := context.Background()
	repoPath := t.TempDir()

	options := SyncOptions{
		RemoteURL: "https://github.com/test/repo.git",
		Branch:    "main",
	}

	status, err := service.GetSyncStatus(ctx, repoPath, options)
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, options.RemoteURL, status.RemoteURL)
}

// TestService_ListRemoteSnapshots 测试列出远程快照
func TestService_ListRemoteSnapshots(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	ctx := context.Background()
	tempDir := t.TempDir()

	// 创建测试清单文件
	manifestDir := filepath.Join(tempDir, ".ai-sync")
	require.NoError(t, os.MkdirAll(manifestDir, 0755))

	manifestPath := filepath.Join(manifestDir, "manifest.json")
	manifestContent := `{
  "snap-1": {
    "name": "Snapshot 1",
    "description": "First snapshot",
    "message": "Test message 1",
    "commit_hash": "abc123",
    "created_at": "2026-03-20T10:00:00Z"
  }
}`
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifestContent), 0644))

	snapshots, err := service.ListRemoteSnapshots(ctx, tempDir)
	require.NoError(t, err)
	assert.Len(t, snapshots, 1)
	assert.Equal(t, "snap-1", snapshots[0].ID)
	assert.Equal(t, "Snapshot 1", snapshots[0].Name)
	assert.Equal(t, "abc123", snapshots[0].CommitHash)
}

// TestService_ListRemoteSnapshots_NoRepo 测试列出远程快照（仓库不存在）
func TestService_ListRemoteSnapshots_NoRepo(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	ctx := context.Background()
	nonExistentPath := filepath.Join(t.TempDir(), "non-existent")

	snapshots, err := service.ListRemoteSnapshots(ctx, nonExistentPath)
	require.NoError(t, err)
	assert.Empty(t, snapshots)
}

// TestService_DeleteLocalRepository 测试删除本地仓库
func TestService_DeleteLocalRepository(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	// 创建测试目录
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	// 删除目录
	err = service.DeleteLocalRepository(tempDir)
	assert.NoError(t, err)

	// 验证删除
	_, err = os.Stat(tempDir)
	assert.True(t, os.IsNotExist(err))
}

// TestService_ValidateRemoteConfig 测试验证远程配置
func TestService_ValidateRemoteConfig(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	ctx := context.Background()
	config := &models.RemoteConfig{
		URL:    "https://github.com/test/repo.git",
		Branch: "main",
	}

	result, err := service.ValidateRemoteConfig(ctx, config)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, config.URL, result.URL)
	assert.Equal(t, config.Branch, result.Branch)
}

// TestService_convertAuth 测试认证配置转换
func TestService_convertAuth(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	// 测试 nil 输入
	auth := service.convertAuth(nil)
	assert.Nil(t, auth)

	// 测试完整配置
	inputAuth := &models.AuthConfig{
		Type:       models.AuthTypeSSH,
		Username:   "testuser",
		Password:   "testpass",
		SSHKey:     "ssh-key-content",
		Passphrase: "key-passphrase",
	}

	resultAuth := service.convertAuth(inputAuth)
	assert.NotNil(t, resultAuth)
	assert.Equal(t, models.AuthTypeSSH, models.AuthType(resultAuth.Type))
	assert.Equal(t, "testuser", resultAuth.Username)
	assert.Equal(t, "testpass", resultAuth.Password)
	assert.Equal(t, "ssh-key-content", resultAuth.SSHKey)
	assert.Equal(t, "key-passphrase", resultAuth.Passphrase)
}

// TestService_convertToRemoteCommits 测试转换为远程提交格式
func TestService_convertToRemoteCommits(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	snapshots := []*models.Snapshot{
		{
			ID:         "snap-1",
			CommitHash: "abc123",
			Message:    "First snapshot",
			CreatedAt:  time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:         "snap-2",
			CommitHash: "def456",
			Message:    "Second snapshot",
			CreatedAt:  time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC),
		},
	}

	commits := service.convertToRemoteCommits(snapshots)

	assert.Len(t, commits, 2)
	assert.Equal(t, "abc123", commits[0].Hash)
	assert.Equal(t, "First snapshot", commits[0].Message)
	assert.Equal(t, "snap-1", commits[0].SnapshotID)
	assert.Equal(t, "2026-03-20T10:00:00Z", commits[0].Date)

	assert.Equal(t, "def456", commits[1].Hash)
	assert.Equal(t, "Second snapshot", commits[1].Message)
	assert.Equal(t, "snap-2", commits[1].SnapshotID)
}

// TestService_isSnapshotNew 测试检查快照是否为新
func TestService_isSnapshotNew(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	// 新快照
	isNew := service.isSnapshotNew("non-existent-id")
	assert.True(t, isNew)

	// 创建已存在的快照
	existingSnapshot := &models.Snapshot{
		ID:        "existing-id",
		Name:      "Existing",
		Message:   "Test",
		CreatedAt: time.Now(),
		Tools:     []string{"codex"},
		Files:     []models.SnapshotFile{},
		Metadata: models.SnapshotMetadata{
			Scope: models.ScopeGlobal,
		},
	}

	snapshotDAO := database.NewSnapshotDAO(db)
	err = snapshotDAO.Create(existingSnapshot)
	require.NoError(t, err)

	// 检查已存在的快照
	isNew = service.isSnapshotNew("existing-id")
	assert.False(t, isNew)
}

// TestService_saveRemoteSnapshot 测试保存远程快照
func TestService_saveRemoteSnapshot(t *testing.T) {
	db, err := database.InitTestDB(t)
	require.NoError(t, err)

	service := NewService(db)

	remoteSnapshot := &models.Snapshot{
		ID:        "remote-1",
		Name:      "Remote Snapshot",
		Message:   "From remote",
		CreatedAt: time.Now(),
		Tools:     []string{"cursor"},
		Files:     []models.SnapshotFile{},
		Metadata: models.SnapshotMetadata{
			Scope: models.ScopeProject,
		},
	}

	err = service.saveRemoteSnapshot(remoteSnapshot)
	assert.NoError(t, err)

	// 验证保存
	snapshotDAO := database.NewSnapshotDAO(db)
	fetched, err := snapshotDAO.GetByID("remote-1")
	require.NoError(t, err)
	assert.Equal(t, "Remote Snapshot", fetched.Name)
}

// TestGenerateSnapshotID 测试生成快照 ID
func TestGenerateSnapshotID(t *testing.T) {
	id1 := GenerateSnapshotID()
	id2 := GenerateSnapshotID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)

	// 验证 UUID 格式
	assert.Len(t, id1, 36)
	assert.Contains(t, id1, "-")
}

// TestSyncOptions 测试同步选项
func TestSyncOptions(t *testing.T) {
	options := SyncOptions{
		RemoteURL:    "https://github.com/test/repo.git",
		Branch:       "main",
		RemoteName:   "origin",
		CreateCommit: true,
		ForcePush:    false,
	}

	assert.Equal(t, "https://github.com/test/repo.git", options.RemoteURL)
	assert.Equal(t, "main", options.Branch)
	assert.Equal(t, "origin", options.RemoteName)
	assert.True(t, options.CreateCommit)
	assert.False(t, options.ForcePush)
}

// TestPushResult 测试推送结果
func TestPushResult(t *testing.T) {
	result := PushResult{
		Success:      true,
		SnapshotID:   "snap-1",
		CommitHash:   "abc123",
		Branch:       "main",
		PushedAt:     "2026-03-20T10:00:00Z",
		FilesCount:   5,
		ErrorMessage: "",
	}

	assert.True(t, result.Success)
	assert.Equal(t, "snap-1", result.SnapshotID)
	assert.Equal(t, "abc123", result.CommitHash)
	assert.Equal(t, "main", result.Branch)
	assert.Equal(t, 5, result.FilesCount)
}

// TestPullResult 测试拉取结果
func TestPullResult(t *testing.T) {
	result := PullResult{
		Success:      true,
		Branch:       "main",
		PulledAt:     "2026-03-20T10:00:00Z",
		NewSnapshots: 3,
		Commits: []RemoteCommit{
			{
				Hash:       "abc123",
				Message:    "Commit 1",
				SnapshotID: "snap-1",
			},
		},
		ErrorMessage: "",
	}

	assert.True(t, result.Success)
	assert.Equal(t, "main", result.Branch)
	assert.Equal(t, 3, result.NewSnapshots)
	assert.Len(t, result.Commits, 1)
}

// TestRemoteCommit 测试远程提交
func TestRemoteCommit(t *testing.T) {
	commit := RemoteCommit{
		Hash:       "abc123def456",
		Message:    "Test commit message",
		Author:     "Test Author",
		Date:       "2026-03-20T10:00:00Z",
		SnapshotID: "snap-1",
	}

	assert.Equal(t, "abc123def456", commit.Hash)
	assert.Equal(t, "Test commit message", commit.Message)
	assert.Equal(t, "Test Author", commit.Author)
	assert.Equal(t, "snap-1", commit.SnapshotID)
}

// TestSyncStatus 测试同步状态
func TestSyncStatus(t *testing.T) {
	status := SyncStatus{
		RemoteURL:   "https://github.com/test/repo.git",
		Branch:      "main",
		LastSync:    "2026-03-20T10:00:00Z",
		LocalAhead:  2,
		LocalBehind: 1,
		RemoteSnapshots: []RemoteSnapshotInfo{
			{
				ID:          "snap-1",
				Name:        "Snapshot 1",
				CommitHash:  "abc123",
				Author:      "Test Author",
				Date:        "2026-03-20T10:00:00Z",
				Tools:       []string{"codex"},
			},
		},
	}

	assert.Equal(t, "https://github.com/test/repo.git", status.RemoteURL)
	assert.Equal(t, "main", status.Branch)
	assert.Equal(t, 2, status.LocalAhead)
	assert.Equal(t, 1, status.LocalBehind)
	assert.Len(t, status.RemoteSnapshots, 1)
}

// TestRemoteSnapshotInfo 测试远程快照信息
func TestRemoteSnapshotInfo(t *testing.T) {
	info := RemoteSnapshotInfo{
		ID:          "snap-1",
		Name:        "Snapshot Name",
		Description: "Snapshot description",
		Message:     "Commit message",
		CommitHash:  "abc123",
		Author:      "Test Author",
		Date:        "2026-03-20T10:00:00Z",
		Tools:       []string{"codex", "cursor"},
		FileCount:   5,
		Size:        1024,
	}

	assert.Equal(t, "snap-1", info.ID)
	assert.Equal(t, "Snapshot Name", info.Name)
	assert.Len(t, info.Tools, 2)
	assert.Equal(t, 5, info.FileCount)
	assert.Equal(t, int64(1024), info.Size)
}

// TestSyncAction 测试同步动作
func TestSyncAction(t *testing.T) {
	assert.Equal(t, SyncAction("push"), SyncActionPush)
	assert.Equal(t, SyncAction("pull"), SyncActionPull)
	assert.Equal(t, SyncAction("clone"), SyncActionClone)
}

// TestConflictType 测试冲突类型
func TestConflictType(t *testing.T) {
	assert.Equal(t, ConflictType("modified"), ConflictTypeModified)
	assert.Equal(t, ConflictType("deleted"), ConflictTypeDeleted)
	assert.Equal(t, ConflictType("created"), ConflictTypeCreated)
	assert.Equal(t, ConflictType("binary"), ConflictTypeBinary)
}

// TestSyncConflict 测试同步冲突
func TestSyncConflict(t *testing.T) {
	conflict := SyncConflict{
		Type:       ConflictTypeModified,
		Path:       "config.toml",
		LocalHash:  "abc123",
		RemoteHash: "def456",
		Reason:     "File modified on both sides",
	}

	assert.Equal(t, ConflictTypeModified, conflict.Type)
	assert.Equal(t, "config.toml", conflict.Path)
	assert.Equal(t, "abc123", conflict.LocalHash)
	assert.Equal(t, "def456", conflict.RemoteHash)
	assert.Equal(t, "File modified on both sides", conflict.Reason)
}

// TestSyncResult 测试同步结果
func TestSyncResult(t *testing.T) {
	pushResult := &PushResult{
		Success:    true,
		SnapshotID: "snap-1",
		CommitHash: "abc123",
	}

	result := SyncResult{
		Success:     true,
		Action:      SyncActionPush,
		PushResult:  pushResult,
		ErrorMessage: "",
	}

	assert.True(t, result.Success)
	assert.Equal(t, SyncActionPush, result.Action)
	assert.NotNil(t, result.PushResult)
	assert.Equal(t, "snap-1", result.PushResult.SnapshotID)
	assert.Nil(t, result.PullResult)
}
