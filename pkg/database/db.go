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
	once     sync.Once
)

// InitDB 初始化数据库单例，并完成首次迁移。
func InitDB(dataDir string) (*DB, error) {
	var initErr error
	once.Do(func() {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			initErr = fmt.Errorf("创建数据目录失败: %w", err)
			return
		}

		dbPath := filepath.Join(dataDir, "ai-sync-manager.db")
		instance = &DB{
			path:   dbPath,
			closed: false,
		}

		conn, err := openGormDB(dbPath)
		if err != nil {
			initErr = fmt.Errorf("打开数据库失败: %w", err)
			return
		}
		instance.conn = conn

		if err := instance.configure(); err != nil {
			initErr = fmt.Errorf("配置数据库失败: %w", err)
			return
		}

		if err := instance.migrate(); err != nil {
			initErr = fmt.Errorf("数据库迁移失败: %w", err)
			return
		}

		logger.Info("数据库初始化成功", zap.String("path", dbPath))
	})

	if initErr != nil {
		return nil, initErr
	}

	return instance, nil
}

// InitDBWithConfig 使用外部配置初始化数据库单例。
// cfg 为 nil 时使用默认值。config 包通过 DatabaseConfig 接口解耦。
func InitDBWithConfig(dataDir string, cfg DatabaseConfig) (*DB, error) {
	if cfg == nil {
		return InitDB(dataDir)
	}

	var initErr error
	once.Do(func() {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			initErr = fmt.Errorf("创建数据目录失败: %w", err)
			return
		}

		dbPath := filepath.Join(dataDir, cfg.GetFilename())
		instance = &DB{
			path:   dbPath,
			closed: false,
		}

		conn, err := openGormDB(dbPath)
		if err != nil {
			initErr = fmt.Errorf("打开数据库失败: %w", err)
			return
		}
		instance.conn = conn

		sqlDB, err := conn.DB()
		if err != nil {
			initErr = fmt.Errorf("获取底层 sql.DB 失败: %w", err)
			return
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

		if err := conn.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
			initErr = fmt.Errorf("设置 PRAGMA 失败: %w", err)
			return
		}

		if err := instance.migrate(); err != nil {
			initErr = fmt.Errorf("数据库迁移失败: %w", err)
			return
		}

		logger.Info("数据库初始化成功", zap.String("path", dbPath))
	})

	if initErr != nil {
		return nil, initErr
	}

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

	return db.conn.Exec("PRAGMA foreign_keys = ON").Error
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

// GetConn 暴露 GORM 连接给 DAO 层使用。
func (db *DB) GetConn() *gorm.DB {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.conn
}

// migrate 完成建表、建索引和建触发器。
func (db *DB) migrate() error {
	return db.conn.Transaction(func(tx *gorm.DB) error {
		if err := db.createTables(tx); err != nil {
			return err
		}
		if err := db.createIndexes(tx); err != nil {
			return err
		}
		if err := db.createTriggers(tx); err != nil {
			return err
		}
		return nil
	})
}

// createTables 使用 GORM AutoMigrate 创建表结构。
func (db *DB) createTables(tx *gorm.DB) error {
	return tx.AutoMigrate(
		&snapshotRecord{},
		&snapshotFileRecord{},
		&syncTaskRecord{},
		&remoteConfigRecord{},
		&appSettingsRecord{},
		&customSyncRuleRecord{},
		&registeredProjectRecord{},
		&backupRecord{},
		&syncHistoryRecord{},
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

// createTriggers 创建触发器。
func (db *DB) createTriggers(tx *gorm.DB) error {
	triggers := []string{
		`CREATE TRIGGER IF NOT EXISTS update_remote_configs_updated_at
			AFTER UPDATE ON remote_configs
			FOR EACH ROW
			BEGIN
				UPDATE remote_configs SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_app_settings_updated_at
			AFTER UPDATE ON app_settings
			FOR EACH ROW
			BEGIN
				UPDATE app_settings SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_custom_sync_rules_updated_at
			AFTER UPDATE ON custom_sync_rules
			FOR EACH ROW
			BEGIN
				UPDATE custom_sync_rules SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_registered_projects_updated_at
			AFTER UPDATE ON registered_projects
			FOR EACH ROW
			BEGIN
				UPDATE registered_projects SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS cleanup_sync_history_on_snapshot_delete
			AFTER DELETE ON snapshots
			FOR EACH ROW
			BEGIN
				DELETE FROM sync_history WHERE task_id IN (SELECT id FROM sync_tasks WHERE snapshot_id = OLD.id);
			END`,
	}

	for _, stmt := range triggers {
		if err := tx.Exec(stmt).Error; err != nil {
			return fmt.Errorf("创建触发器失败: %w", err)
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

type snapshotRecord struct {
	ID          string    `gorm:"column:id;primaryKey"`
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

func (snapshotRecord) TableName() string { return "snapshots" }

type snapshotFileRecord struct {
	ID           uint      `gorm:"column:id;primaryKey;autoIncrement"`
	SnapshotID   string    `gorm:"column:snapshot_id;not null"`
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

func (snapshotFileRecord) TableName() string { return "snapshot_files" }

type syncTaskRecord struct {
	ID              string     `gorm:"column:id;primaryKey"`
	Type            string     `gorm:"column:type;not null"`
	Status          string     `gorm:"column:status;not null"`
	SnapshotID      *string    `gorm:"column:snapshot_id"`
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
	ID             string     `gorm:"column:id;primaryKey"`
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
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null"`
	LastSynced     *time.Time `gorm:"column:last_synced"`
	Status         string     `gorm:"column:status;not null;default:inactive"`
}

func (remoteConfigRecord) TableName() string { return "remote_configs" }

type appSettingsRecord struct {
	ID                       string    `gorm:"column:id;primaryKey"`
	Version                  string    `gorm:"column:version;not null"`
	RemoteConfigID           *string   `gorm:"column:remote_config_id"`
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
	UpdatedAt                time.Time `gorm:"column:updated_at;not null"`
}

func (appSettingsRecord) TableName() string { return "app_settings" }

type customSyncRuleRecord struct {
	ID           string    `gorm:"column:id;primaryKey"`
	ToolType     string    `gorm:"column:tool_type;not null;uniqueIndex:idx_custom_sync_rules_tool_path"`
	AbsolutePath string    `gorm:"column:absolute_path;not null;uniqueIndex:idx_custom_sync_rules_tool_path"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (customSyncRuleRecord) TableName() string { return "custom_sync_rules" }

type registeredProjectRecord struct {
	ID          string    `gorm:"column:id;primaryKey"`
	ToolType    string    `gorm:"column:tool_type;not null;uniqueIndex:idx_registered_projects_tool_path"`
	ProjectName string    `gorm:"column:project_name;not null"`
	ProjectPath string    `gorm:"column:project_path;not null;uniqueIndex:idx_registered_projects_tool_path"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null"`
}

func (registeredProjectRecord) TableName() string { return "registered_projects" }

type backupRecord struct {
	ID          string    `gorm:"column:id;primaryKey"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	Path        string    `gorm:"column:path;not null"`
	Size        int64     `gorm:"column:size;not null"`
	FileCount   int       `gorm:"column:file_count;not null"`
	SnapshotID  *string   `gorm:"column:snapshot_id"`
	Description string    `gorm:"column:description"`
}

func (backupRecord) TableName() string { return "backups" }

type syncHistoryRecord struct {
	ID          string     `gorm:"column:id;primaryKey"`
	TaskID      string     `gorm:"column:task_id;not null"`
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
