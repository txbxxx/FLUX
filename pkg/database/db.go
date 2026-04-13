package database

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ai-sync-manager/pkg/logger"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DB 数据库实例
type DB struct {
	conn   *gorm.DB
	path   string
	mu     sync.RWMutex
	closed bool
}

var (
	instance *DB
	mu       sync.Mutex
)

// InitDB 初始化数据库单例，并完成首次迁移。
// 为什么：使用 mutex 替代 sync.Once，首次失败后可重试。
func InitDB(dataDir string) (*DB, error) {
	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
		return instance, nil
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, "ai-sync-manager.db")
	db := &DB{
		path:   dbPath,
		closed: false,
	}

	conn, err := openGormDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.conn = conn

	if err := db.configure(); err != nil {
		return nil, fmt.Errorf("配置数据库失败: %w", err)
	}

	// 检查旧版本 schema（UUID string ID）并提示用户
	if err := db.checkLegacySchema(); err != nil {
		return nil, err
	}

	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	instance = db
	logger.Info("数据库初始化成功", zap.String("path", dbPath))
	return instance, nil
}

// InitDBWithConfig 使用外部配置初始化数据库单例。
// cfg 为 nil 时使用默认值。config 包通过 DatabaseConfig 接口解耦。
// 为什么：使用 mutex 替代 sync.Once，首次失败后可重试。
func InitDBWithConfig(dataDir string, cfg DatabaseConfig) (*DB, error) {
	if cfg == nil {
		return InitDB(dataDir)
	}

	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
		return instance, nil
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dataDir, cfg.GetFilename())
	db := &DB{
		path:   dbPath,
		closed: false,
	}

	conn, err := openGormDB(dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.conn = conn

	sqlDB, err := conn.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}

	if n := cfg.GetMaxOpenConns(); n > 0 {
		sqlDB.SetMaxOpenConns(n)
	}
	if n := cfg.GetMaxIdleConns(); n > 0 {
		sqlDB.SetMaxIdleConns(n)
	}
	if d := cfg.GetConnMaxLifetime(); d > 0 {
		sqlDB.SetConnMaxLifetime(d)
	}

	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	instance = db
	logger.Info("数据库初始化成功", zap.String("path", dbPath))
	return instance, nil
}

// DatabaseConfig 数据库配置接口（避免直接依赖 config 包）。
type DatabaseConfig interface {
	GetFilename() string
	GetMaxOpenConns() int
	GetMaxIdleConns() int
	GetConnMaxLifetime() time.Duration
}

func openGormDB(dbPath string) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
}

func (db *DB) configure() error {
	sqlDB, err := db.conn.DB()
	if err != nil {
		return err
	}

	sqlDB.SetMaxOpenConns(1) // SQLite 只能单写
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return nil
}

// GetDB 返回当前进程内的数据库单例。
func GetDB() *DB {
	return instance
}

// Close 关闭数据库连接
func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if db.closed {
		return nil
	}

	sqlDB, err := db.conn.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return err
	}

	db.closed = true
	logger.Info("数据库连接已关闭")
	return nil
}

// IsClosed 检查数据库是否已关闭
func (db *DB) IsClosed() bool {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.closed
}

// checkLegacySchema 检测旧版本 schema（UUID string ID）并自动迁移为 uint 自增 ID。
// 迁移策略：按 rowid 顺序复制数据，建立旧 text ID → 新 uint ID 映射，更新关联字段。
func (db *DB) checkLegacySchema() error {
	if !db.conn.Migrator().HasTable(&SnapshotRecord{}) {
		return nil // 表不存在，首次创建
	}

	var columnType string
	err := db.conn.Raw("SELECT typeof(id) FROM snapshots LIMIT 1").Scan(&columnType).Error
	if err != nil || columnType != "text" {
		return nil // 已经是整数类型或空表
	}

	logger.Info("检测到旧版本数据库 schema，开始自动迁移（UUID string → uint 自增）")
	if migrateErr := db.migrateLegacySchema(); migrateErr != nil {
		return fmt.Errorf("数据库迁移失败: %w\n数据库路径: %s\n请手动删除数据库文件后重试", migrateErr, db.path)
	}
	logger.Info("数据库 schema 迁移完成")
	return nil
}

// migrateLegacySchema 将所有表的 ID 列从 string 转为 uint 自增。
func (db *DB) migrateLegacySchema() error {
	return db.conn.Transaction(func(tx *gorm.DB) error {
		// 第一步：读取旧 snapshot ID 映射（按 rowid 顺序保证新 ID 1,2,3... 对应）
		var oldSnapshotIDs []string
		if err := tx.Raw("SELECT id FROM snapshots ORDER BY rowid").Scan(&oldSnapshotIDs).Error; err != nil {
			return fmt.Errorf("读取旧快照 ID 失败: %w", err)
		}
		snapIDMap := make(map[string]uint)
		for i, oldID := range oldSnapshotIDs {
			snapIDMap[oldID] = uint(i + 1)
		}

		// 第二步：读取旧 sync_task ID 映射
		var oldTaskIDs []string
		_ = tx.Raw("SELECT id FROM sync_tasks ORDER BY rowid").Scan(&oldTaskIDs).Error
		taskIDMap := make(map[string]uint)
		for i, oldID := range oldTaskIDs {
			taskIDMap[oldID] = uint(i + 1)
		}

		// 第三步：迁移 snapshots 表
		if err := migrateTable(tx, "snapshots", `
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			message TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			tools TEXT NOT NULL,
			metadata TEXT,
			tags TEXT,
			commit_hash TEXT,
			file_count INTEGER DEFAULT 0,
			total_size INTEGER DEFAULT 0
		`, "name, description, message, created_at, tools, metadata, tags, commit_hash, file_count, total_size"); err != nil {
			return fmt.Errorf("迁移 snapshots 表失败: %w", err)
		}

		// 第四步：迁移 snapshot_files 表（更新 snapshot_id 为新 uint ID）
		if err := migrateTableWithSnapshotID(tx, "snapshot_files", `
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			snapshot_id INTEGER NOT NULL,
			path TEXT NOT NULL,
			original_path TEXT NOT NULL,
			size INTEGER NOT NULL,
			hash TEXT,
			modified_at DATETIME,
			content BLOB,
			tool_type TEXT NOT NULL,
			category TEXT NOT NULL,
			is_binary INTEGER DEFAULT 0
		`, snapIDMap); err != nil {
			return fmt.Errorf("迁移 snapshot_files 表失败: %w", err)
		}

		// 第五步：迁移 sync_tasks 表（更新 snapshot_id）
		if err := migrateSyncTasks(tx, snapIDMap); err != nil {
			return fmt.Errorf("迁移 sync_tasks 表失败: %w", err)
		}

		// 第六步：迁移 sync_history 表（更新 task_id）
		if err := migrateSyncHistory(tx, taskIDMap); err != nil {
			return fmt.Errorf("迁移 sync_history 表失败: %w", err)
		}

		// 第七步：迁移 backups 表（更新 snapshot_id）
		if err := migrateBackups(tx, snapIDMap); err != nil {
			return fmt.Errorf("迁移 backups 表失败: %w", err)
		}

		// 第八步：迁移无外键关联的表（直接复制）
		for _, table := range []struct {
			name    string
			schema  string
			columns string
		}{
			{
				"remote_configs",
				`id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, url TEXT NOT NULL,
				auth_type TEXT NOT NULL, auth_username TEXT, auth_password TEXT, auth_ssh_key TEXT,
				auth_passphrase TEXT, branch TEXT NOT NULL DEFAULT 'main', is_default INTEGER DEFAULT 0,
				created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL, last_synced DATETIME,
				status TEXT NOT NULL DEFAULT 'inactive'`,
				"name, url, auth_type, auth_username, auth_password, auth_ssh_key, auth_passphrase, branch, is_default, created_at, updated_at, last_synced, status",
			},
			{
				"custom_sync_rules",
				`id INTEGER PRIMARY KEY AUTOINCREMENT, tool_type TEXT NOT NULL, absolute_path TEXT NOT NULL,
				created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL`,
				"tool_type, absolute_path, created_at, updated_at",
			},
			{
				"registered_projects",
				`id INTEGER PRIMARY KEY AUTOINCREMENT, tool_type TEXT NOT NULL, project_name TEXT NOT NULL,
				project_path TEXT NOT NULL, created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL`,
				"tool_type, project_name, project_path, created_at, updated_at",
			},
			{
				"ai_settings",
				`id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, token TEXT NOT NULL,
				base_url TEXT, opus_model TEXT, sonnet_model TEXT,
				created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL`,
				"name, token, base_url, opus_model, sonnet_model, created_at, updated_at",
			},
			{
				"app_settings",
				`id INTEGER PRIMARY KEY AUTOINCREMENT, version TEXT NOT NULL, remote_config_id INTEGER,
				sync_auto_sync INTEGER DEFAULT 0, sync_interval INTEGER, sync_conflict_policy TEXT,
				sync_excludes TEXT, sync_includes TEXT, encryption_enabled INTEGER DEFAULT 0,
				encryption_algorithm TEXT, encryption_key_path TEXT, ui_theme TEXT, ui_language TEXT,
				ui_auto_start INTEGER DEFAULT 0, ui_minimize_to_tray INTEGER DEFAULT 0,
				notifications_enabled INTEGER DEFAULT 1, notifications_sync_success INTEGER DEFAULT 1,
				notifications_sync_failure INTEGER DEFAULT 1, notifications_conflict INTEGER DEFAULT 1,
				notifications_new_snapshot INTEGER DEFAULT 1, notifications_sound TEXT,
				created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL`,
				"version, remote_config_id, sync_auto_sync, sync_interval, sync_conflict_policy, sync_excludes, sync_includes, encryption_enabled, encryption_algorithm, encryption_key_path, ui_theme, ui_language, ui_auto_start, ui_minimize_to_tray, notifications_enabled, notifications_sync_success, notifications_sync_failure, notifications_conflict, notifications_new_snapshot, notifications_sound, created_at, updated_at",
			},
		} {
			if tx.Migrator().HasTable(table.name) {
				if err := migrateTable(tx, table.name, table.schema, table.columns); err != nil {
					return fmt.Errorf("迁移 %s 表失败: %w", table.name, err)
				}
			}
		}

		return nil
	})
}

// migrateTable 重建表为 uint 自增 ID，复制所有非 ID 列数据。
func migrateTable(tx *gorm.DB, tableName, schema, columns string) error {
	newName := tableName + "_new"
	// 创建新表
	if err := tx.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", newName, schema)).Error; err != nil {
		return err
	}
	// 复制数据（按 rowid 顺序保证新 ID 递增顺序一致）
	if err := tx.Exec(fmt.Sprintf("INSERT INTO %s (%s) SELECT %s FROM %s ORDER BY rowid", newName, columns, columns, tableName)).Error; err != nil {
		return err
	}
	// 替换旧表
	if err := tx.Exec(fmt.Sprintf("DROP TABLE %s", tableName)).Error; err != nil {
		return err
	}
	return tx.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", newName, tableName)).Error
}

// migrateTableWithSnapshotID 迁移 snapshot_files 表，将 string snapshot_id 映射为 uint。
func migrateTableWithSnapshotID(tx *gorm.DB, tableName, schema string, snapIDMap map[string]uint) error {
	newName := tableName + "_new"
	if err := tx.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", newName, schema)).Error; err != nil {
		return err
	}

	// 读取旧数据
	type oldFileRow struct {
		SnapshotID   string
		Path         string
		OriginalPath string
		Size         int64
		Hash         string
		ModifiedAt   time.Time
		Content      []byte
		ToolType     string
		Category     string
		IsBinary     bool
	}
	var rows []oldFileRow
	if err := tx.Raw("SELECT snapshot_id, path, original_path, size, hash, modified_at, content, tool_type, category, is_binary FROM "+tableName+" ORDER BY rowid").Scan(&rows).Error; err != nil {
		return err
	}

	for _, r := range rows {
		newSnapID, ok := snapIDMap[r.SnapshotID]
		if !ok {
			continue // 孤儿记录，跳过
		}
		if err := tx.Exec(
			fmt.Sprintf("INSERT INTO %s (snapshot_id, path, original_path, size, hash, modified_at, content, tool_type, category, is_binary) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", newName),
			newSnapID, r.Path, r.OriginalPath, r.Size, r.Hash, r.ModifiedAt, r.Content, r.ToolType, r.Category, r.IsBinary,
		).Error; err != nil {
			return err
		}
	}

	if err := tx.Exec(fmt.Sprintf("DROP TABLE %s", tableName)).Error; err != nil {
		return err
	}
	return tx.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", newName, tableName)).Error
}

// migrateSyncTasks 迁移 sync_tasks 表，映射 snapshot_id。
func migrateSyncTasks(tx *gorm.DB, snapIDMap map[string]uint) error {
	if !tx.Migrator().HasTable("sync_tasks") {
		return nil
	}
	if err := tx.Exec(`CREATE TABLE sync_tasks_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT NOT NULL, status TEXT NOT NULL,
		snapshot_id INTEGER, direction TEXT, created_at DATETIME NOT NULL,
		started_at DATETIME, completed_at DATETIME, progress_current INTEGER DEFAULT 0,
		progress_total INTEGER DEFAULT 0, progress_message TEXT, error_msg TEXT, metadata TEXT
	)`).Error; err != nil {
		return err
	}

	type oldTask struct {
		ID              string
		Type            string
		Status          string
		SnapshotID      *string
		Direction       string
		CreatedAt       time.Time
		StartedAt       *time.Time
		CompletedAt     *time.Time
		ProgressCurrent int
		ProgressTotal   int
		ProgressMessage string
		ErrorMsg        string
		Metadata        string
	}
	var rows []oldTask
	if err := tx.Raw("SELECT id, type, status, snapshot_id, direction, created_at, started_at, completed_at, progress_current, progress_total, progress_message, error_msg, metadata FROM sync_tasks ORDER BY rowid").Scan(&rows).Error; err != nil {
		return err
	}

	for _, r := range rows {
		var newSnapID *uint
		if r.SnapshotID != nil {
			if id, ok := snapIDMap[*r.SnapshotID]; ok {
				newSnapID = &id
			}
		}
		if err := tx.Exec(
			`INSERT INTO sync_tasks_new (type, status, snapshot_id, direction, created_at, started_at, completed_at, progress_current, progress_total, progress_message, error_msg, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			r.Type, r.Status, newSnapID, r.Direction, r.CreatedAt, r.StartedAt, r.CompletedAt,
			r.ProgressCurrent, r.ProgressTotal, r.ProgressMessage, r.ErrorMsg, r.Metadata,
		).Error; err != nil {
			return err
		}
	}

	if err := tx.Exec("DROP TABLE sync_tasks").Error; err != nil {
		return err
	}
	return tx.Exec("ALTER TABLE sync_tasks_new RENAME TO sync_tasks").Error
}

// migrateSyncHistory 迁移 sync_history 表，映射 task_id。
func migrateSyncHistory(tx *gorm.DB, taskIDMap map[string]uint) error {
	if !tx.Migrator().HasTable("sync_history") {
		return nil
	}
	if err := tx.Exec(`CREATE TABLE sync_history_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT, task_id INTEGER NOT NULL, type TEXT NOT NULL,
		status TEXT NOT NULL, direction TEXT, started_at DATETIME NOT NULL,
		completed_at DATETIME, duration_ms INTEGER, success INTEGER DEFAULT 0, error_msg TEXT
	)`).Error; err != nil {
		return err
	}

	type oldHistory struct {
		TaskID      string
		Type        string
		Status      string
		Direction   string
		StartedAt   time.Time
		CompletedAt *time.Time
		DurationMS  *int64
		Success     bool
		ErrorMsg    string
	}
	var rows []oldHistory
	if err := tx.Raw("SELECT task_id, type, status, direction, started_at, completed_at, duration_ms, success, error_msg FROM sync_history ORDER BY rowid").Scan(&rows).Error; err != nil {
		return err
	}

	for _, r := range rows {
		newTaskID, ok := taskIDMap[r.TaskID]
		if !ok {
			continue
		}
		if err := tx.Exec(
			`INSERT INTO sync_history_new (task_id, type, status, direction, started_at, completed_at, duration_ms, success, error_msg)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			newTaskID, r.Type, r.Status, r.Direction, r.StartedAt, r.CompletedAt, r.DurationMS, r.Success, r.ErrorMsg,
		).Error; err != nil {
			return err
		}
	}

	if err := tx.Exec("DROP TABLE sync_history").Error; err != nil {
		return err
	}
	return tx.Exec("ALTER TABLE sync_history_new RENAME TO sync_history").Error
}

// migrateBackups 迁移 backups 表，映射 snapshot_id。
func migrateBackups(tx *gorm.DB, snapIDMap map[string]uint) error {
	if !tx.Migrator().HasTable("backups") {
		return nil
	}
	if err := tx.Exec(`CREATE TABLE backups_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT, created_at DATETIME NOT NULL, path TEXT NOT NULL,
		size INTEGER NOT NULL, file_count INTEGER NOT NULL, snapshot_id INTEGER, description TEXT
	)`).Error; err != nil {
		return err
	}

	type oldBackup struct {
		CreatedAt   time.Time
		Path        string
		Size        int64
		FileCount   int
		SnapshotID  *string
		Description string
	}
	var rows []oldBackup
	if err := tx.Raw("SELECT created_at, path, size, file_count, snapshot_id, description FROM backups ORDER BY rowid").Scan(&rows).Error; err != nil {
		return err
	}

	for _, r := range rows {
		var newSnapID *uint
		if r.SnapshotID != nil {
			if id, ok := snapIDMap[*r.SnapshotID]; ok {
				newSnapID = &id
			}
		}
		if err := tx.Exec(
			`INSERT INTO backups_new (created_at, path, size, file_count, snapshot_id, description)
			VALUES (?, ?, ?, ?, ?, ?)`,
			r.CreatedAt, r.Path, r.Size, r.FileCount, newSnapID, r.Description,
		).Error; err != nil {
			return err
		}
	}

	if err := tx.Exec("DROP TABLE backups").Error; err != nil {
		return err
	}
	return tx.Exec("ALTER TABLE backups_new RENAME TO backups").Error
}

// GetConn 暴露 GORM 连接给 DAO 层使用。
func (db *DB) GetConn() *gorm.DB {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.conn
}

// migrate 完成建表和建索引。
func (db *DB) migrate() error {
	return db.conn.Transaction(func(tx *gorm.DB) error {
		if err := db.createTables(tx); err != nil {
			return err
		}
		if err := db.createIndexes(tx); err != nil {
			return err
		}
		return nil
	})
}

// createTables 使用 GORM AutoMigrate 创建表结构。
func (db *DB) createTables(tx *gorm.DB) error {
	return tx.AutoMigrate(
		&SnapshotRecord{},
		&SnapshotFileRecord{},
		&syncTaskRecord{},
		&remoteConfigRecord{},
		&appSettingsRecord{},
		&customSyncRuleRecord{},
		&registeredProjectRecord{},
		&backupRecord{},
		&syncHistoryRecord{},
		&aiSettingRecord{},
	)
}

// createIndexes 创建索引。
func (db *DB) createIndexes(tx *gorm.DB) error {
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_snapshot_files_snapshot_id ON snapshot_files(snapshot_id)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_files_tool_type ON snapshot_files(tool_type)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshot_files_category ON snapshot_files(category)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_tasks_snapshot_id ON sync_tasks(snapshot_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_tasks_status ON sync_tasks(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_tasks_created_at ON sync_tasks(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_backups_snapshot_id ON backups(snapshot_id)`,
		`CREATE INDEX IF NOT EXISTS idx_backups_created_at ON backups(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_history_task_id ON sync_history(task_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_history_started_at ON sync_history(started_at)`,
		`CREATE INDEX IF NOT EXISTS idx_remote_configs_is_default ON remote_configs(is_default)`,
		`CREATE INDEX IF NOT EXISTS idx_custom_sync_rules_tool_type ON custom_sync_rules(tool_type)`,
		`CREATE INDEX IF NOT EXISTS idx_registered_projects_tool_type ON registered_projects(tool_type)`,
	}

	for _, stmt := range indexes {
		if err := tx.Exec(stmt).Error; err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}

	return nil
}

// Transaction 执行 GORM 事务。
func (db *DB) Transaction(fn func(*gorm.DB) error) error {
	return db.conn.Transaction(fn)
}

// GetDatabasePath 获取数据库文件路径
func (db *DB) GetDatabasePath() string {
	return db.path
}

// GetDatabaseSize 获取数据库文件大小
func (db *DB) GetDatabaseSize() (int64, error) {
	info, err := os.Stat(db.path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// BackupDatabase 备份数据库
func (db *DB) BackupDatabase(backupPath string) error {
	input, err := os.ReadFile(db.path)
	if err != nil {
		return fmt.Errorf("读取数据库文件失败: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return fmt.Errorf("写入备份文件失败: %w", err)
	}

	logger.Info("数据库备份完成", zap.String("backup_path", backupPath))
	return nil
}

// Vacuum 优化数据库
func (db *DB) Vacuum() error {
	if err := db.conn.Exec("VACUUM").Error; err != nil {
		return fmt.Errorf("数据库优化失败: %w", err)
	}

	logger.Info("数据库优化完成")
	return nil
}

// InitTestDB 初始化测试数据库
func InitTestDB(t TestDBHelper) (*DB, error) {
	t.Helper()

	tempDir := t.TempDir()
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db := &DB{
		path:   dbPath,
		closed: false,
	}

	conn, err := openGormDB(dbPath)
	if err != nil {
		return nil, err
	}
	db.conn = conn

	if err := db.configure(); err != nil {
		return nil, err
	}
	if err := db.migrate(); err != nil {
		return nil, err
	}

	t.Cleanup(func() {
		_ = db.Close()
		_ = os.RemoveAll(tempDir)
	})

	return db, nil
}

// TestDBHelper 测试数据库辅助接口
type TestDBHelper interface {
	testing.TB
	TempDir() string
	Cleanup(func())
}

// SnapshotRecord is the GORM model for the "snapshots" table, used for migration and CRUD.
type SnapshotRecord struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement"`
	Name        string    `gorm:"column:name;not null"`
	Description string    `gorm:"column:description"`
	Message     string    `gorm:"column:message;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	Tools       string    `gorm:"column:tools;not null"`
	Metadata    string    `gorm:"column:metadata"`
	Tags        string    `gorm:"column:tags"`
	CommitHash  string    `gorm:"column:commit_hash"`
	FileCount   int       `gorm:"column:file_count;default:0"`
	TotalSize   int64     `gorm:"column:total_size;default:0"`
}

func (SnapshotRecord) TableName() string { return "snapshots" }

// SnapshotFileRecord is the GORM model for the "snapshot_files" table, used for migration and CRUD.
type SnapshotFileRecord struct {
	ID           uint      `gorm:"column:id;primaryKey;autoIncrement"`
	SnapshotID   uint      `gorm:"column:snapshot_id;not null"`
	Path         string    `gorm:"column:path;not null"`
	OriginalPath string    `gorm:"column:original_path;not null"`
	Size         int64     `gorm:"column:size;not null"`
	Hash         string    `gorm:"column:hash"`
	ModifiedAt   time.Time `gorm:"column:modified_at"`
	Content      []byte    `gorm:"column:content"`
	ToolType     string    `gorm:"column:tool_type;not null"`
	Category     string    `gorm:"column:category;not null"`
	IsBinary     bool      `gorm:"column:is_binary;default:false"`
}

func (SnapshotFileRecord) TableName() string { return "snapshot_files" }

type syncTaskRecord struct {
	ID              uint       `gorm:"column:id;primaryKey;autoIncrement"`
	Type            string     `gorm:"column:type;not null"`
	Status          string     `gorm:"column:status;not null"`
	SnapshotID      *uint      `gorm:"column:snapshot_id"`
	Direction       string     `gorm:"column:direction"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null"`
	StartedAt       *time.Time `gorm:"column:started_at"`
	CompletedAt     *time.Time `gorm:"column:completed_at"`
	ProgressCurrent int        `gorm:"column:progress_current;default:0"`
	ProgressTotal   int        `gorm:"column:progress_total;default:0"`
	ProgressMessage string     `gorm:"column:progress_message"`
	ErrorMsg        string     `gorm:"column:error_msg"`
	Metadata        string     `gorm:"column:metadata"`
}

func (syncTaskRecord) TableName() string { return "sync_tasks" }

type remoteConfigRecord struct {
	ID             uint       `gorm:"column:id;primaryKey;autoIncrement"`
	Name           string     `gorm:"column:name;not null;unique"`
	URL            string     `gorm:"column:url;not null"`
	AuthType       string     `gorm:"column:auth_type;not null"`
	AuthUsername   string     `gorm:"column:auth_username"`
	AuthPassword   string     `gorm:"column:auth_password"`
	AuthSSHKey     string     `gorm:"column:auth_ssh_key"`
	AuthPassphrase string     `gorm:"column:auth_passphrase"`
	Branch         string     `gorm:"column:branch;not null;default:main"`
	IsDefault      bool       `gorm:"column:is_default;default:false"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null;autoUpdateTime"`
	LastSynced     *time.Time `gorm:"column:last_synced"`
	Status         string     `gorm:"column:status;not null;default:inactive"`
}

func (remoteConfigRecord) TableName() string { return "remote_configs" }

type appSettingsRecord struct {
	ID                       uint      `gorm:"column:id;primaryKey;autoIncrement"`
	Version                  string    `gorm:"column:version;not null"`
	RemoteConfigID           *uint     `gorm:"column:remote_config_id"`
	SyncAutoSync             bool      `gorm:"column:sync_auto_sync;default:false"`
	SyncInterval             *int      `gorm:"column:sync_interval"`
	SyncConflictPolicy       string    `gorm:"column:sync_conflict_policy"`
	SyncExcludes             string    `gorm:"column:sync_excludes"`
	SyncIncludes             string    `gorm:"column:sync_includes"`
	EncryptionEnabled        bool      `gorm:"column:encryption_enabled;default:false"`
	EncryptionAlgorithm      string    `gorm:"column:encryption_algorithm"`
	EncryptionKeyPath        string    `gorm:"column:encryption_key_path"`
	UITheme                  string    `gorm:"column:ui_theme"`
	UILanguage               string    `gorm:"column:ui_language"`
	UIAutoStart              bool      `gorm:"column:ui_auto_start;default:false"`
	UIMinimizeToTray         bool      `gorm:"column:ui_minimize_to_tray;default:false"`
	NotificationsEnabled     bool      `gorm:"column:notifications_enabled;default:true"`
	NotificationsSyncSuccess bool      `gorm:"column:notifications_sync_success;default:true"`
	NotificationsSyncFailure bool      `gorm:"column:notifications_sync_failure;default:true"`
	NotificationsConflict    bool      `gorm:"column:notifications_conflict;default:true"`
	NotificationsNewSnapshot bool      `gorm:"column:notifications_new_snapshot;default:true"`
	NotificationsSound       string    `gorm:"column:notifications_sound"`
	CreatedAt                time.Time `gorm:"column:created_at;not null"`
	UpdatedAt                time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (appSettingsRecord) TableName() string { return "app_settings" }

type customSyncRuleRecord struct {
	ID           uint      `gorm:"column:id;primaryKey;autoIncrement"`
	ToolType     string    `gorm:"column:tool_type;not null;uniqueIndex:idx_custom_sync_rules_tool_path"`
	AbsolutePath string    `gorm:"column:absolute_path;not null;uniqueIndex:idx_custom_sync_rules_tool_path"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (customSyncRuleRecord) TableName() string { return "custom_sync_rules" }

type registeredProjectRecord struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement"`
	ToolType    string    `gorm:"column:tool_type;not null;uniqueIndex:idx_registered_projects_tool_path"`
	ProjectName string    `gorm:"column:project_name;not null"`
	ProjectPath string    `gorm:"column:project_path;not null;uniqueIndex:idx_registered_projects_tool_path"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (registeredProjectRecord) TableName() string { return "registered_projects" }

type backupRecord struct {
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	Path        string    `gorm:"column:path;not null"`
	Size        int64     `gorm:"column:size;not null"`
	FileCount   int       `gorm:"column:file_count;not null"`
	SnapshotID  *uint     `gorm:"column:snapshot_id"`
	Description string    `gorm:"column:description"`
}

func (backupRecord) TableName() string { return "backups" }

type syncHistoryRecord struct {
	ID          uint       `gorm:"column:id;primaryKey;autoIncrement"`
	TaskID      uint       `gorm:"column:task_id;not null"`
	Type        string     `gorm:"column:type;not null"`
	Status      string     `gorm:"column:status;not null"`
	Direction   string     `gorm:"column:direction"`
	StartedAt   time.Time  `gorm:"column:started_at;not null"`
	CompletedAt *time.Time `gorm:"column:completed_at"`
	DurationMS  *int64     `gorm:"column:duration_ms"`
	Success     bool       `gorm:"column:success;default:false"`
	ErrorMsg    string     `gorm:"column:error_msg"`
}

func (syncHistoryRecord) TableName() string { return "sync_history" }

type aiSettingRecord struct {
	ID        uint      `gorm:"column:id;primaryKey;autoIncrement"`
	Name      string    `gorm:"column:name;not null;uniqueIndex"`
	Token     string    `gorm:"column:token;not null"`
	BaseURL   string    `gorm:"column:base_url"`
	OpusModel string    `gorm:"column:opus_model"`
	SonnetModel string `gorm:"column:sonnet_model"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (aiSettingRecord) TableName() string { return "ai_settings" }
