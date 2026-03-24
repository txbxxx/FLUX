package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ai-sync-manager/pkg/logger"

	_ "modernc.org/sqlite"
	"go.uber.org/zap"
)

// DB 数据库实例
type DB struct {
	conn   *sql.DB
	path   string
	mu     sync.RWMutex
	closed bool
}

var (
	instance *DB
	once     sync.Once
)

// InitDB 初始化数据库
func InitDB(dataDir string) (*DB, error) {
	var initErr error
	once.Do(func() {
		// 确保数据目录存在
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			initErr = fmt.Errorf("创建数据目录失败: %w", err)
			return
		}

		dbPath := filepath.Join(dataDir, "ai-sync-manager.db")
		instance = &DB{
			path:   dbPath,
			closed: false,
		}

		// 打开数据库连接
		conn, err := sql.Open("sqlite", dbPath)
		if err != nil {
			initErr = fmt.Errorf("打开数据库失败: %w", err)
			return
		}

		instance.conn = conn

		// 设置连接池参数
		instance.conn.SetMaxOpenConns(1) // SQLite 只能单写
		instance.conn.SetMaxIdleConns(1)
		instance.conn.SetConnMaxLifetime(time.Hour)

		// 启用外键约束
		if _, err := instance.conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
			initErr = fmt.Errorf("启用外键约束失败: %w", err)
			return
		}

		// 运行迁移
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

// GetDB 获取数据库实例
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

	if err := db.conn.Close(); err != nil {
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

// GetConn 获取底层连接（仅供内部使用）
func (db *DB) GetConn() *sql.DB {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.conn
}

// migrate 执行数据库迁移
func (db *DB) migrate() error {
	conn := db.GetConn()

	// 在事务中执行迁移
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	// 创建所有表
	if err := db.createTables(tx); err != nil {
		return err
	}

	// 创建索引
	if err := db.createIndexes(tx); err != nil {
		return err
	}

	// 创建触发器
	if err := db.createTriggers(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	logger.Info("数据库迁移完成")
	return nil
}

// createTables 创建表
func (db *DB) createTables(tx *sql.Tx) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS snapshots (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			message TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			tools TEXT NOT NULL,
			metadata TEXT,
			tags TEXT,
			commit_hash TEXT,
			file_count INTEGER DEFAULT 0,
			total_size INTEGER DEFAULT 0
		)`,

		`CREATE TABLE IF NOT EXISTS snapshot_files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			snapshot_id TEXT NOT NULL,
			path TEXT NOT NULL,
			original_path TEXT NOT NULL,
			size INTEGER NOT NULL,
			hash TEXT,
			modified_at INTEGER NOT NULL,
			content BLOB,
			tool_type TEXT NOT NULL,
			category TEXT NOT NULL,
			is_binary INTEGER DEFAULT 0,
			FOREIGN KEY (snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS sync_tasks (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			snapshot_id TEXT,
			direction TEXT,
			created_at INTEGER NOT NULL,
			started_at INTEGER,
			completed_at INTEGER,
			progress_current INTEGER DEFAULT 0,
			progress_total INTEGER DEFAULT 0,
			progress_message TEXT,
			error_msg TEXT,
			metadata TEXT,
			FOREIGN KEY (snapshot_id) REFERENCES snapshots(id) ON DELETE SET NULL
		)`,

		`CREATE TABLE IF NOT EXISTS remote_configs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			url TEXT NOT NULL,
			auth_type TEXT NOT NULL,
			auth_username TEXT,
			auth_password TEXT,
			auth_ssh_key TEXT,
			auth_passphrase TEXT,
			branch TEXT NOT NULL DEFAULT 'main',
			is_default INTEGER DEFAULT 0,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			last_synced INTEGER,
			status TEXT NOT NULL DEFAULT 'inactive'
		)`,

		`CREATE TABLE IF NOT EXISTS app_settings (
			id TEXT PRIMARY KEY,
			version TEXT NOT NULL,
			remote_config_id TEXT,
			sync_auto_sync INTEGER DEFAULT 0,
			sync_interval INTEGER,
			sync_conflict_policy TEXT,
			sync_excludes TEXT,
			sync_includes TEXT,
			encryption_enabled INTEGER DEFAULT 0,
			encryption_algorithm TEXT,
			encryption_key_path TEXT,
			ui_theme TEXT,
			ui_language TEXT,
			ui_auto_start INTEGER DEFAULT 0,
			ui_minimize_to_tray INTEGER DEFAULT 0,
			notifications_enabled INTEGER DEFAULT 1,
			notifications_sync_success INTEGER DEFAULT 1,
			notifications_sync_failure INTEGER DEFAULT 1,
			notifications_conflict INTEGER DEFAULT 1,
			notifications_new_snapshot INTEGER DEFAULT 1,
			notifications_sound TEXT,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			FOREIGN KEY (remote_config_id) REFERENCES remote_configs(id) ON DELETE SET NULL
		)`,

		`CREATE TABLE IF NOT EXISTS backups (
			id TEXT PRIMARY KEY,
			created_at INTEGER NOT NULL,
			path TEXT NOT NULL,
			size INTEGER NOT NULL,
			file_count INTEGER NOT NULL,
			snapshot_id TEXT,
			description TEXT,
			FOREIGN KEY (snapshot_id) REFERENCES snapshots(id) ON DELETE SET NULL
		)`,

		`CREATE TABLE IF NOT EXISTS sync_history (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			direction TEXT,
			started_at INTEGER NOT NULL,
			completed_at INTEGER,
			duration_ms INTEGER,
			success INTEGER DEFAULT 0,
			error_msg TEXT,
			FOREIGN KEY (task_id) REFERENCES sync_tasks(id) ON DELETE CASCADE
		)`,
	}

	for _, sql := range tables {
		if _, err := tx.Exec(sql); err != nil {
			return fmt.Errorf("创建表失败: %w", err)
		}
	}

	return nil
}

// createIndexes 创建索引
func (db *DB) createIndexes(tx *sql.Tx) error {
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
	}

	for _, sql := range indexes {
		if _, err := tx.Exec(sql); err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}

	return nil
}

// createTriggers 创建触发器
func (db *DB) createTriggers(tx *sql.Tx) error {
	triggers := []string{
		// 自动更新 updated_at
		`CREATE TRIGGER IF NOT EXISTS update_remote_configs_updated_at
			AFTER UPDATE ON remote_configs
			FOR EACH ROW
			BEGIN
				UPDATE remote_configs SET updated_at = strftime('%s', 'now') WHERE id = NEW.id;
			END`,

		`CREATE TRIGGER IF NOT EXISTS update_app_settings_updated_at
			AFTER UPDATE ON app_settings
			FOR EACH ROW
			BEGIN
				UPDATE app_settings SET updated_at = strftime('%s', 'now') WHERE id = NEW.id;
			END`,

		// 删除快照时级联删除同步历史
		`CREATE TRIGGER IF NOT EXISTS cleanup_sync_history_on_snapshot_delete
			AFTER DELETE ON snapshots
			FOR EACH ROW
			BEGIN
				DELETE FROM sync_history WHERE task_id IN (SELECT id FROM sync_tasks WHERE snapshot_id = OLD.id);
			END`,
	}

	for _, sql := range triggers {
		if _, err := tx.Exec(sql); err != nil {
			return fmt.Errorf("创建触发器失败: %w", err)
		}
	}

	return nil
}

// Transaction 执行事务
func (db *DB) Transaction(fn func(*sql.Tx) error) error {
	conn := db.GetConn()
	tx, err := conn.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
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
	// 简化实现：直接复制文件
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
	conn := db.GetConn()
	_, err := conn.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("数据库优化失败: %w", err)
	}

	logger.Info("数据库优化完成")
	return nil
}

// InitTestDB 初始化测试数据库
func InitTestDB(t TestDBHelper) (*DB, error) {
	t.Helper()

	// 创建临时数据库
	tempDir := t.TempDir()
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db := &DB{
		path:   dbPath,
		closed: false,
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	db.conn = conn

	// 启用外键
	if _, err := db.conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, err
	}

	// 运行迁移
	if err := db.migrate(); err != nil {
		return nil, err
	}

	// 清理函数
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
