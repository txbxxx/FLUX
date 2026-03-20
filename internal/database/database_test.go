package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ai-sync-manager/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDB_InitDB 测试数据库初始化
func TestDB_InitDB(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.False(t, db.IsClosed())

	// 清理由 Cleanup 自动处理
}

// TestDB_GetConn 测试获取连接
func TestDB_GetConn(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	conn := db.GetConn()
	assert.NotNil(t, conn)

	// 验证连接可用
	var result int
	err = conn.QueryRow("SELECT 1").Scan(&result)
	assert.NoError(t, err)
	assert.Equal(t, 1, result)
}

// TestDB_Transaction 测试事务
func TestDB_Transaction(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	// 测试成功事务
	err = db.Transaction(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO remote_configs (id, name, url, auth_type, branch, created_at, updated_at, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			"test-1", "Test", "https://github.com/test/repo.git", "none", "main", time.Now().Unix(), time.Now().Unix(), "inactive")
		if err != nil {
			return err
		}

		// 查询验证
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM remote_configs WHERE id = ?", "test-1").Scan(&count)
		if err != nil {
			return err
		}
		assert.Equal(t, 1, count)

		return nil
	})

	assert.NoError(t, err)
}

// TestDB_Transaction_Rollback 测试事务回滚
func TestDB_Transaction_Rollback(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	// 测试失败回滚
	expectedErr := assert.AnError
	err = db.Transaction(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO remote_configs (id, name, url, auth_type, branch, created_at, updated_at, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			"test-rollback", "Test", "https://github.com/test/repo.git", "none", "main", time.Now().Unix(), time.Now().Unix(), "inactive")
		if err != nil {
			return err
		}

		// 故意返回错误触发回滚
		return expectedErr
	})

	assert.Error(t, err)

	// 验证数据未插入
	conn := db.GetConn()
	var count int
	err = conn.QueryRow("SELECT COUNT(*) FROM remote_configs WHERE id = ?", "test-rollback").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestDB_GetDatabasePath 测试获取数据库路径
func TestDB_GetDatabasePath(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	path := db.GetDatabasePath()
	assert.Contains(t, path, "test.db")
	assert.FileExists(t, path)
}

// TestDB_GetDatabaseSize 测试获取数据库大小
func TestDB_GetDatabaseSize(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	size, err := db.GetDatabaseSize()
	assert.NoError(t, err)
	assert.Greater(t, size, int64(0))
}

// TestDB_BackupDatabase 测试备份数据库
func TestDB_BackupDatabase(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	backupPath := filepath.Join(os.TempDir(), "backup.db")
	err = db.BackupDatabase(backupPath)
	assert.NoError(t, err)

	// 验证备份文件存在
	info, err := os.Stat(backupPath)
	assert.NoError(t, err)
	assert.False(t, info.IsDir())
}

// TestDB_Vacuum 测试优化数据库
func TestDB_Vacuum(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	err = db.Vacuum()
	assert.NoError(t, err)
}

// TestDB_GetStats 测试获取统计信息
func TestDB_GetStats(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	// 插入测试数据
	conn := db.GetConn()
	now := time.Now()

	_, err = conn.Exec(
		"INSERT INTO snapshots (id, name, message, created_at, tools, metadata, tags) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"test-snap-1", "Test Snapshot", "Test message", now.Unix(), `["codex"]`, `{}`, `[]`,
	)
	require.NoError(t, err)

	stats, err := db.GetStats()
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.SnapshotCount)
	assert.Greater(t, stats.DatabaseSize, int64(0))
}

// SnapshotDAO_Create 测试创建快照
func TestSnapshotDAO_Create(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewSnapshotDAO(db)

	snapshot := &models.Snapshot{
		ID:        "snap-test-1",
		Name:      "Test Snapshot",
		Message:   "Test message",
		CreatedAt: time.Now(),
		Tools:     []string{"codex", "claude"},
		Metadata: models.SnapshotMetadata{
			Scope: models.ScopeGlobal,
		},
		Tags:      []string{"test"},
	}

	err = dao.Create(snapshot)
	assert.NoError(t, err)
}

// SnapshotDAO_GetByID 测试获取快照
func TestSnapshotDAO_GetByID(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewSnapshotDAO(db)

	// 先创建
	snapshot := &models.Snapshot{
		ID:        "snap-get-1",
		Name:      "Get Test",
		Message:   "Test message",
		CreatedAt: time.Now(),
		Tools:     []string{"codex"},
	}

	err = dao.Create(snapshot)
	require.NoError(t, err)

	// 再获取
	fetched, err := dao.GetByID("snap-get-1")
	assert.NoError(t, err)
	assert.NotNil(t, fetched)
	assert.Equal(t, "Get Test", fetched.Name)
	assert.Equal(t, "Test message", fetched.Message)
	assert.Equal(t, []string{"codex"}, fetched.Tools)
}

// SnapshotDAO_GetByID_NotFound 测试获取不存在的快照
func TestSnapshotDAO_GetByID_NotFound(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewSnapshotDAO(db)

	snapshot, err := dao.GetByID("non-existent")
	assert.Error(t, err)
	assert.Nil(t, snapshot)
	assert.Contains(t, err.Error(), "不存在")
}

// SnapshotDAO_List 测试列出快照
func TestSnapshotDAO_List(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewSnapshotDAO(db)

	// 创建多个快照
	snapshots := []*models.Snapshot{
		{
			ID:        "snap-list-1",
			Name:      "Snapshot 1",
			Message:   "Message 1",
			CreatedAt: time.Now(),
			Tools:     []string{"codex"},
		},
		{
			ID:        "snap-list-2",
			Name:      "Snapshot 2",
			Message:   "Message 2",
			CreatedAt: time.Now().Add(time.Hour),
			Tools:     []string{"claude"},
		},
	}

	for _, snap := range snapshots {
		err = dao.Create(snap)
		assert.NoError(t, err)
	}

	// 列出所有
	list, err := dao.List(0, 0)
	assert.NoError(t, err)
	assert.Len(t, list, 2)

	// 验证按创建时间倒序
	assert.Equal(t, "Snapshot 2", list[0].Name)
	assert.Equal(t, "Snapshot 1", list[1].Name)
}

// SnapshotDAO_Update 测试更新快照
func TestSnapshotDAO_Update(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewSnapshotDAO(db)

	snapshot := &models.Snapshot{
		ID:        "snap-update-1",
		Name:      "Original Name",
		Message:   "Original Message",
		CreatedAt: time.Now(),
		Tools:     []string{"codex"},
	}

	err = dao.Create(snapshot)
	require.NoError(t, err)

	// 更新
	snapshot.Name = "Updated Name"
	snapshot.Message = "Updated Message"
	snapshot.Tools = []string{"codex", "claude"}

	err = dao.Update(snapshot)
	assert.NoError(t, err)

	// 验证更新
	fetched, err := dao.GetByID("snap-update-1")
	assert.NoError(t, err)
	assert.Equal(t, "Updated Name", fetched.Name)
	assert.Equal(t, "Updated Message", fetched.Message)
	assert.Len(t, fetched.Tools, 2)
}

// SnapshotDAO_Delete 测试删除快照
func TestSnapshotDAO_Delete(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewSnapshotDAO(db)

	snapshot := &models.Snapshot{
		ID:        "snap-delete-1",
		Name:      "Delete Test",
		Message:   "Test",
		CreatedAt: time.Now(),
		Tools:     []string{"codex"},
	}

	err = dao.Create(snapshot)
	require.NoError(t, err)

	// 删除
	err = dao.Delete("snap-delete-1")
	assert.NoError(t, err)

	// 验证已删除
	_, err = dao.GetByID("snap-delete-1")
	assert.Error(t, err)
}

// SnapshotDAO_Count 测试统计快照数量
func TestSnapshotDAO_Count(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewSnapshotDAO(db)

	// 初始为 0
	count, err := dao.Count()
	assert.NoError(t, err)
	assert.Equal(t, 0, count)

	// 添加数据
	for i := 1; i <= 3; i++ {
		snapshot := &models.Snapshot{
			ID:        fmt.Sprintf("snap-count-%d", i),
			Name:      fmt.Sprintf("Snapshot %d", i),
			Message:   "Test",
			CreatedAt: time.Now(),
			Tools:     []string{"codex"},
		}
		err = dao.Create(snapshot)
		assert.NoError(t, err)
	}

	count, err = dao.Count()
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

// SyncTaskDAO_Create 测试创建同步任务
func TestSyncTaskDAO_Create(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewSyncTaskDAO(db)

	task := &models.SyncTask{
		ID:        "task-1",
		Type:      models.TaskTypePush,
		Status:    models.StatusPending,
		Direction: models.DirectionUpload,
		CreatedAt: time.Now(),
		Progress: models.TaskProgress{
			Current: 0,
			Total:   100,
			Message: "Pending",
		},
		Metadata: models.TaskMetadata{
			Tools: []string{"codex"},
		},
	}

	err = dao.Create(task)
	assert.NoError(t, err)
}

// SyncTaskDAO_GetByID 测试获取同步任务
func TestSyncTaskDAO_GetByID(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewSyncTaskDAO(db)

	task := &models.SyncTask{
		ID:        "task-get-1",
		Type:      models.TaskTypePull,
		Status:    models.StatusRunning,
		CreatedAt: time.Now(),
		Progress: models.TaskProgress{
			Current: 50,
			Total:   100,
			Message: "Running",
		},
	}

	err = dao.Create(task)
	require.NoError(t, err)

	fetched, err := dao.GetByID("task-get-1")
	assert.NoError(t, err)
	assert.NotNil(t, fetched)
	assert.Equal(t, models.TaskTypePull, fetched.Type)
	assert.Equal(t, models.StatusRunning, fetched.Status)
	assert.Equal(t, 50, fetched.Progress.Current)
}

// SyncTaskDAO_Update 测试更新同步任务
func TestSyncTaskDAO_Update(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewSyncTaskDAO(db)

	task := &models.SyncTask{
		ID:        "task-update-1",
		Type:      models.TaskTypePush,
		Status:    models.StatusPending,
		CreatedAt: time.Now(),
		Progress: models.TaskProgress{
			Current: 0,
			Total:   100,
		},
	}

	err = dao.Create(task)
	require.NoError(t, err)

	// 更新状态
	task.Status = models.StatusCompleted
	task.Progress.Current = 100
	task.Progress.Message = "Completed"

	now := time.Now()
	task.CompletedAt = &now

	err = dao.Update(task)
	assert.NoError(t, err)

	// 验证更新
	fetched, err := dao.GetByID("task-update-1")
	assert.NoError(t, err)
	assert.Equal(t, models.StatusCompleted, fetched.Status)
	assert.Equal(t, 100, fetched.Progress.Current)
	assert.NotNil(t, fetched.CompletedAt)
}

// RemoteConfigDAO_Create 测试创建远端配置
func TestRemoteConfigDAO_Create(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewRemoteConfigDAO(db)

	config := &models.RemoteConfig{
		ID:      "remote-1",
		Name:    "Test Config",
		URL:     "https://github.com/test/repo.git",
		Branch:  "main",
		Auth: models.AuthConfig{
			Type: models.AuthTypeToken,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    models.StatusInactive,
	}

	err = dao.Create(config)
	assert.NoError(t, err)
}

// RemoteConfigDAO_GetDefault 测试获取默认配置
func TestRemoteConfigDAO_GetDefault(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewRemoteConfigDAO(db)

	// 创建非默认配置
	config := &models.RemoteConfig{
		ID:      "remote-default",
		Name:    "Default Config",
		URL:     "https://github.com/test/repo.git",
		Branch:  "main",
		Auth:    models.AuthConfig{Type: models.AuthTypeNone},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    models.StatusActive,
		IsDefault: true,
	}

	err = dao.Create(config)
	require.NoError(t, err)

	// 获取默认配置
	defaultConfig, err := dao.GetDefault()
	assert.NoError(t, err)
	assert.NotNil(t, defaultConfig)
	assert.Equal(t, "Default Config", defaultConfig.Name)
	assert.True(t, defaultConfig.IsDefault)
}

// RemoteConfigDAO_GetDefault_None 测试无默认配置
func TestRemoteConfigDAO_GetDefault_None(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewRemoteConfigDAO(db)

	config, err := dao.GetDefault()
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), "未找到默认配置")
}

// RemoteConfigDAO_SetDefault 测试设置默认配置
func TestRemoteConfigDAO_SetDefault(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewRemoteConfigDAO(db)

	// 创建两个配置
	config1 := &models.RemoteConfig{
		ID:        "remote-set-1",
		Name:      "Config 1",
		URL:       "https://github.com/test/repo1.git",
			Branch:    "main",
		Auth:      models.AuthConfig{Type: models.AuthTypeNone},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    models.StatusActive,
		IsDefault: false,
	}

	config2 := &models.RemoteConfig{
		ID:        "remote-set-2",
		Name:      "Config 2",
		URL:       "https://github.com/test/repo2.git",
		Branch:    "main",
		Auth:      models.AuthConfig{Type: models.AuthTypeNone},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    models.StatusActive,
		IsDefault: true,
	}

	_ = dao.Create(config1)
	_ = dao.Create(config2)

	// 设置 config1 为默认
	err = dao.SetDefault("remote-set-1")
	assert.NoError(t, err)

	// 验证
	defaultConfig, err := dao.GetDefault()
	assert.NoError(t, err)
	assert.Equal(t, "Config 1", defaultConfig.Name)
	assert.True(t, defaultConfig.IsDefault)
}

// RemoteConfigDAO_List 测试列出远端配置
func TestRemoteConfigDAO_List(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewRemoteConfigDAO(db)

	// 创建多个配置
	configs := []*models.RemoteConfig{
		{
			ID:        "remote-list-1",
			Name:      "Config 1",
			URL:       "https://github.com/test/repo1.git",
			Branch:    "main",
			Auth:      models.AuthConfig{Type: models.AuthTypeNone},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Status:    models.StatusActive,
		},
		{
			ID:        "remote-list-2",
			Name:      "Config 2",
			URL:       "https://github.com/test/repo2.git",
			Branch:    "develop",
			Auth:      models.AuthConfig{Type: models.AuthTypeSSH},
			CreatedAt: time.Now().Add(-time.Hour),
			UpdatedAt: time.Now().Add(-time.Hour),
			Status:    models.StatusActive,
		},
	}

	for _, config := range configs {
		_ = dao.Create(config)
	}

	// 列出所有
	list, err := dao.List()
	assert.NoError(t, err)
	assert.Len(t, list, 2)

	// 验证按创建时间倒序
	assert.Equal(t, "Config 1", list[0].Name)
	assert.Equal(t, "Config 2", list[1].Name)
}

// RemoteConfigDAO_Update 测试更新远端配置
func TestRemoteConfigDAO_Update(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewRemoteConfigDAO(db)

	config := &models.RemoteConfig{
		ID:      "remote-update-1",
		Name:    "Original Name",
		URL:     "https://github.com/test/repo.git",
		Branch:  "main",
		Auth:    models.AuthConfig{Type: models.AuthTypeNone},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:  models.StatusInactive,
	}

	_ = dao.Create(config)

	// 更新
	config.Name = "Updated Name"
	config.Branch = "develop"
	config.Status = models.StatusActive

	err = dao.Update(config)
	assert.NoError(t, err)

	// 验证 UpdatedAt 被更新
	assert.True(t, config.UpdatedAt.After(config.CreatedAt))
}

// RemoteConfigDAO_Delete 测试删除远端配置
func TestRemoteConfigDAO_Delete(t *testing.T) {
	db, err := InitTestDB(t)
	require.NoError(t, err)
	defer db.Close()

	dao := NewRemoteConfigDAO(db)

	config := &models.RemoteConfig{
		ID:        "remote-delete-1",
		Name:      "Delete Test",
		URL:       "https://github.com/test/repo.git",
		Branch:    "main",
		Auth:      models.AuthConfig{Type: models.AuthTypeNone},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    models.StatusActive,
	}

	_ = dao.Create(config)

	err = dao.Delete("remote-delete-1")
	assert.NoError(t, err)

	// 验证已删除
	list, err := dao.List()
	assert.NoError(t, err)
	assert.Len(t, list, 0)
}

// Table驱动测试：bool 转换
func TestBoolConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected int
	}{
		{"true", true, 1},
		{"false", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boolToInt(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIntToBool(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected bool
	}{
		{"1", 1, true},
		{"0", 0, false},
		{"2", 2, true}, // 非 0/1 都为 true
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intToBool(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
