package models

import (
	"testing"
	"time"

	"ai-sync-manager/pkg/database"
	testify "github.com/stretchr/testify/assert"
)

// TestSnapshotDAO_Create_WritesFiles 验证 DAO Create 正确写入 snapshot_files。
// 这是复现 issue "snapshot_files 表为空" 的回归测试。
func TestSnapshotDAO_Create_WritesFiles(t *testing.T) {
	assert := testify.New(t)

	db, err := database.InitTestDB(t)
	assert.NoError(err, "创建测试数据库失败")

	dao := NewSnapshotDAO(db)

	snapshot := &Snapshot{
		ID:        "test-dao-files-001",
		Name:      "DAO Files Test",
		Message:   "testing file persistence",
		CreatedAt: time.Now(),
		Tools:     []string{"claude"},
		Metadata: SnapshotMetadata{
			OSVersion:   "test",
			AppVersion:  "0.0.1",
			ProjectPath: "/tmp/test",
		},
		Files: []SnapshotFile{
			{
				Path:         "config/settings.json",
				OriginalPath: "/home/user/.claude/settings.json",
				Size:         12,
				Hash:         "abc123",
				ModifiedAt:   time.Now(),
				Content:      []byte(`{"key":"val"}`),
				ToolType:     "claude",
				Category:     CategoryConfig,
				IsBinary:     false,
			},
			{
				Path:         "commands/test.md",
				OriginalPath: "/home/user/.claude/commands/test.md",
				Size:         5,
				Hash:         "def456",
				ModifiedAt:   time.Now(),
				Content:      []byte("hello"),
				ToolType:     "claude",
				Category:     CategoryCommands,
				IsBinary:     false,
			},
		},
		Tags: []string{"test"},
	}

	// 创建快照
	err = dao.Create(snapshot)
	assert.NoError(err, "Create 不应返回错误")

	// 验证：读回快照，检查文件是否存在
	loaded, err := dao.GetByID("test-dao-files-001")
	assert.NoError(err, "GetByID 不应返回错误")

	// 核心断言：文件数量必须匹配
	assert.Equal(len(snapshot.Files), len(loaded.Files),
		"读回的文件数必须等于写入的文件数")

	if len(loaded.Files) > 0 {
		// 验证文件内容
		assert.Equal("config/settings.json", loaded.Files[0].Path)
		assert.Equal([]byte(`{"key":"val"}`), loaded.Files[0].Content)

		assert.Equal("commands/test.md", loaded.Files[1].Path)
		assert.Equal([]byte("hello"), loaded.Files[1].Content)
	}

	t.Logf("写入 %d 个文件，读回 %d 个文件", len(snapshot.Files), len(loaded.Files))
}

// TestSnapshotDAO_Create_DoesNotDuplicateFiles 验证 GORM 级联不会导致文件重复。
func TestSnapshotDAO_Create_DoesNotDuplicateFiles(t *testing.T) {
	assert := testify.New(t)

	db, err := database.InitTestDB(t)
	assert.NoError(err)

	dao := NewSnapshotDAO(db)

	snapshot := &Snapshot{
		ID:        "test-no-dup-001",
		Name:      "No Dup Test",
		Message:   "test",
		CreatedAt: time.Now(),
		Tools:     []string{"claude"},
		Metadata:  SnapshotMetadata{OSVersion: "test"},
		Files: []SnapshotFile{
			{Path: "a.txt", OriginalPath: "/a.txt", Size: 5, Hash: "a", ModifiedAt: time.Now(), Content: []byte("aaa"), ToolType: "claude", Category: CategoryConfig},
		},
	}

	err = dao.Create(snapshot)
	assert.NoError(err)

	loaded, err := dao.GetByID("test-no-dup-001")
	assert.NoError(err)

	// 不应有重复
	assert.Equal(1, len(loaded.Files), "不应有重复文件记录，期望 1 个，实际 %d 个", len(loaded.Files))
}
