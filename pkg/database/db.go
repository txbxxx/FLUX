package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"flux/pkg/logger"

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

	dbPath := filepath.Join(dataDir, "flux.db")
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

	// 检查旧版本 schema（UUID string ID）并自动迁移
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
func (db *DB) checkLegacySchema() error {
	if !db.conn.Migrator().HasTable(&SnapshotRecord{}) {
		return nil
	}

	sqlDB, err := db.conn.DB()
	if err != nil {
		return nil
	}

	pragRows, err := sqlDB.Query("PRAGMA table_info(snapshots)")
	if err != nil {
		return nil
	}
	var idColumnType string
	for pragRows.Next() {
		var cid int
		var name, colType string
		var notnull, pk int
		var dfltValue interface{}
		pragRows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk)
		if name == "id" {
			idColumnType = colType
			break
		}
	}
	pragRows.Close()

	if idColumnType == "" {
		return nil
	}

	if !strings.Contains(strings.ToUpper(idColumnType), "TEXT") {
		return nil
	}

	logger.Info("检测到旧版本数据库 schema，开始自动迁移（UUID string → uint 自增）")
	if migrateErr := db.migrateLegacySchema(); migrateErr != nil {
		return fmt.Errorf("数据库迁移失败: %w\n数据库路径: %s\n请手动删除数据库文件后重试", migrateErr, db.path)
	}
	logger.Info("数据库 schema 迁移完成")
	return nil
}

// migrateLegacySchema 将所有表的 ID 列从 string 转为 uint 自增。
// 迁移策略：
//  1. 读取所有旧数据到内存
//  2. 关闭旧数据库连接
//  3. 将旧数据库文件重命名为备份
//  4. 重新连接（此时新建空数据库）
//  5. GORM AutoMigrate 创建新 schema
//  6. 将内存中的数据以新格式重新写入
func (db *DB) migrateLegacySchema() error {
	// 第一步：读取所有旧数据
	sqlDB, err := db.conn.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	// 读取 snapshots，建立 ID 映射
	var oldSnapshots []struct {
		ID          string
		Name        string
		Description string
		Message     string
		CreatedAt   time.Time
		Tools       string
		Metadata    string
		Tags        string
		CommitHash  string
		FileCount   int
		TotalSize   int64
	}
	snapRows, err := sqlDB.Query("SELECT id, name, description, message, created_at, tools, metadata, tags, commit_hash, file_count, total_size FROM snapshots ORDER BY rowid")
	if err != nil {
		return fmt.Errorf("读取 snapshots 失败: %w", err)
	}
	snapIDMap := make(map[string]uint)
	i := 1
	for snapRows.Next() {
		var s struct {
			ID          string
			Name        string
			Description *string
			Message     string
			CreatedAt   time.Time
			Tools       string
			Metadata    *string
			Tags        *string
			CommitHash  *string
			FileCount   int
			TotalSize   int64
		}
		if err := snapRows.Scan(&s.ID, &s.Name, &s.Description, &s.Message, &s.CreatedAt, &s.Tools, &s.Metadata, &s.Tags, &s.CommitHash, &s.FileCount, &s.TotalSize); err != nil {
			snapRows.Close()
			return fmt.Errorf("扫描 snapshot 行失败: %w", err)
		}
		desc := ""
		meta := ""
		tgs := ""
		hash := ""
		if s.Description != nil {
			desc = *s.Description
		}
		if s.Metadata != nil {
			meta = *s.Metadata
		}
		if s.Tags != nil {
			tgs = *s.Tags
		}
		if s.CommitHash != nil {
			hash = *s.CommitHash
		}
		snapshots := struct {
			ID          string
			Name        string
			Description string
			Message     string
			CreatedAt   time.Time
			Tools       string
			Metadata    string
			Tags        string
			CommitHash  string
			FileCount   int
			TotalSize   int64
		}{s.ID, s.Name, desc, s.Message, s.CreatedAt, s.Tools, meta, tgs, hash, s.FileCount, s.TotalSize}
		oldSnapshots = append(oldSnapshots, snapshots)
		snapIDMap[s.ID] = uint(i)
		i++
	}
	snapRows.Close()

	// 读取 sync_tasks，建立 ID 映射
	var oldSyncTasks []struct {
		ID              string
		Type            string
		Status          string
		SnapshotID      *string
		Direction       *string
		CreatedAt       time.Time
		StartedAt       *time.Time
		CompletedAt     *time.Time
		ProgressCurrent int
		ProgressTotal   int
		ProgressMessage *string
		ErrorMsg        *string
		Metadata        *string
	}
	taskRows, err := sqlDB.Query("SELECT id, type, status, snapshot_id, direction, created_at, started_at, completed_at, progress_current, progress_total, progress_message, error_msg, metadata FROM sync_tasks ORDER BY rowid")
	if err == nil {
		taskIDMap := make(map[string]uint)
		j := 1
		for taskRows.Next() {
			var t struct {
				ID              string
				Type            string
				Status          string
				SnapshotID      *string
				Direction       *string
				CreatedAt       time.Time
				StartedAt       *time.Time
				CompletedAt     *time.Time
				ProgressCurrent int
				ProgressTotal   int
				ProgressMessage *string
				ErrorMsg        *string
				Metadata        *string
			}
			if err := taskRows.Scan(&t.ID, &t.Type, &t.Status, &t.SnapshotID, &t.Direction, &t.CreatedAt, &t.StartedAt, &t.CompletedAt, &t.ProgressCurrent, &t.ProgressTotal, &t.ProgressMessage, &t.ErrorMsg, &t.Metadata); err != nil {
				taskRows.Close()
				return fmt.Errorf("扫描 sync_task 行失败: %w", err)
			}
			oldSyncTasks = append(oldSyncTasks, t)
			taskIDMap[t.ID] = uint(j)
			j++
		}
		taskRows.Close()
	}

	// 读取 snapshot_files
	var oldSnapshotFiles []struct {
		SnapshotID   string
		Path         string
		OriginalPath string
		Size         int64
		Hash         *string
		ModifiedAt   *time.Time
		Content      []byte
		ToolType     string
		Category     string
		IsBinary     bool
		IsSymlink    bool
		LinkTarget   string
	}
	fileRows, err := sqlDB.Query("SELECT snapshot_id, path, original_path, size, hash, modified_at, content, tool_type, category, is_binary, COALESCE(is_symlink, 0) as is_symlink, COALESCE(link_target, '') as link_target FROM snapshot_files ORDER BY rowid")
	if err == nil {
		for fileRows.Next() {
			var f struct {
				SnapshotID   string
				Path         string
				OriginalPath string
				Size         int64
				Hash         *string
				ModifiedAt   *time.Time
				Content      []byte
				ToolType     string
				Category     string
				IsBinary     bool
				IsSymlink    bool
				LinkTarget   string
			}
			if err := fileRows.Scan(&f.SnapshotID, &f.Path, &f.OriginalPath, &f.Size, &f.Hash, &f.ModifiedAt, &f.Content, &f.ToolType, &f.Category, &f.IsBinary, &f.IsSymlink, &f.LinkTarget); err != nil {
				fileRows.Close()
				return fmt.Errorf("扫描 snapshot_file 行失败: %w", err)
			}
			oldSnapshotFiles = append(oldSnapshotFiles, f)
		}
		fileRows.Close()
	}

	// 读取 sync_history
	var oldSyncHistory []struct {
		TaskID      string
		Type        string
		Status      string
		Direction   *string
		StartedAt   time.Time
		CompletedAt *time.Time
		DurationMS  *int64
		Success     bool
		ErrorMsg    *string
	}
	histRows, err := sqlDB.Query("SELECT task_id, type, status, direction, started_at, completed_at, duration_ms, success, error_msg FROM sync_history ORDER BY rowid")
	if err == nil {
		for histRows.Next() {
			var h struct {
				TaskID      string
				Type        string
				Status      string
				Direction   *string
				StartedAt   time.Time
				CompletedAt *time.Time
				DurationMS  *int64
				Success     bool
				ErrorMsg    *string
			}
			if err := histRows.Scan(&h.TaskID, &h.Type, &h.Status, &h.Direction, &h.StartedAt, &h.CompletedAt, &h.DurationMS, &h.Success, &h.ErrorMsg); err != nil {
				histRows.Close()
				return fmt.Errorf("扫描 sync_history 行失败: %w", err)
			}
			oldSyncHistory = append(oldSyncHistory, h)
		}
		histRows.Close()
	}

	// 读取 backups
	var oldBackups []struct {
		CreatedAt   time.Time
		Path        string
		Size        int64
		FileCount   int
		SnapshotID  *string
		Description *string
	}
	backupRows, err := sqlDB.Query("SELECT created_at, path, size, file_count, snapshot_id, description FROM backups ORDER BY rowid")
	if err == nil {
		for backupRows.Next() {
			var b struct {
				CreatedAt   time.Time
				Path        string
				Size        int64
				FileCount   int
				SnapshotID  *string
				Description *string
			}
			if err := backupRows.Scan(&b.CreatedAt, &b.Path, &b.Size, &b.FileCount, &b.SnapshotID, &b.Description); err != nil {
				backupRows.Close()
				return fmt.Errorf("扫描 backup 行失败: %w", err)
			}
			oldBackups = append(oldBackups, b)
		}
		backupRows.Close()
	}

	// 读取无 ID 外键关联的表
	readSimpleTable := func(tableName string, columns []string) ([][]interface{}, error) {
		var rows [][]interface{}
		rs, err := sqlDB.Query(fmt.Sprintf("SELECT %s FROM %s ORDER BY rowid", strings.Join(columns, ", "), tableName))
		if err != nil {
			return nil, err
		}
		defer rs.Close()
		cols, _ := rs.Columns()
		for rs.Next() {
			vals := make([]interface{}, len(cols))
			for k := range vals {
				var v interface{}
				vals[k] = &v
			}
			if err := rs.Scan(vals...); err != nil {
				return nil, err
			}
			flat := make([]interface{}, len(vals))
			for k, v := range vals {
				if vp, ok := v.(*interface{}); ok {
					flat[k] = *vp
				} else {
					flat[k] = v
				}
			}
			rows = append(rows, flat)
		}
		return rows, nil
	}

	remoteConfigs, _ := readSimpleTable("remote_configs", []string{"name", "url", "auth_type", "auth_username", "auth_password", "auth_ssh_key", "auth_passphrase", "branch", "is_default", "created_at", "updated_at", "last_synced", "status"})
	customSyncRules, _ := readSimpleTable("custom_sync_rules", []string{"tool_type", "absolute_path", "created_at", "updated_at"})
	registeredProjects, _ := readSimpleTable("registered_projects", []string{"tool_type", "project_name", "project_path", "created_at", "updated_at"})
	aiSettings, _ := readSimpleTable("ai_settings", []string{"name", "token", "base_url", "opus_model", "sonnet_model", "created_at", "updated_at"})
	appSettings, _ := readSimpleTable("app_settings", []string{"version", "remote_config_id", "sync_auto_sync", "sync_interval", "sync_conflict_policy", "sync_excludes", "sync_includes", "encryption_enabled", "encryption_algorithm", "encryption_key_path", "ui_theme", "ui_language", "ui_auto_start", "ui_minimize_to_tray", "notifications_enabled", "notifications_sync_success", "notifications_sync_failure", "notifications_conflict", "notifications_new_snapshot", "notifications_sound", "created_at", "updated_at"})

	// 第二步：关闭旧连接，重命名旧文件
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("关闭旧数据库连接失败: %w", err)
	}
	backupPath := db.path + ".uuid-backup"
	if err := os.Rename(db.path, backupPath); err != nil {
		return fmt.Errorf("重命名旧数据库文件失败: %w\n请确保没有其他进程正在使用数据库", err)
	}

	// 第三步：用同样路径重新打开连接（此时创建全新空数据库）
	newConn, err := openGormDB(db.path)
	if err != nil {
		return fmt.Errorf("重新打开数据库失败: %w", err)
	}
	db.conn = newConn
	if err := db.configure(); err != nil {
		return fmt.Errorf("配置新数据库失败: %w", err)
	}

	// 第四步：GORM AutoMigrate 创建所有新表
	if err := db.migrate(); err != nil {
		return fmt.Errorf("创建新 schema 失败: %w", err)
	}

	// 第五步：将数据重新写入
	newDB, err := db.conn.DB()
	if err != nil {
		return fmt.Errorf("获取新数据库连接失败: %w", err)
	}

	// 写入 snapshots（自动生成新 ID）
	for _, s := range oldSnapshots {
		if _, err := newDB.Exec(
			"INSERT INTO snapshots (name, description, message, created_at, tools, metadata, tags, commit_hash, file_count, total_size) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			s.Name, s.Description, s.Message, s.CreatedAt, s.Tools, s.Metadata, s.Tags, s.CommitHash, s.FileCount, s.TotalSize,
		); err != nil {
			return fmt.Errorf("写入 snapshot 失败: %w", err)
		}
	}

	// 写入 sync_tasks（映射 snapshot_id，更换主键 ID）
	for _, t := range oldSyncTasks {
		var newSnapID *uint
		if t.SnapshotID != nil {
			if id, ok := snapIDMap[*t.SnapshotID]; ok {
				newSnapID = &id
			}
		}
		dir := ""
		if t.Direction != nil {
			dir = *t.Direction
		}
		progMsg := ""
		if t.ProgressMessage != nil {
			progMsg = *t.ProgressMessage
		}
		errMsg := ""
		if t.ErrorMsg != nil {
			errMsg = *t.ErrorMsg
		}
		meta := ""
		if t.Metadata != nil {
			meta = *t.Metadata
		}
		if _, err := newDB.Exec(
			"INSERT INTO sync_tasks (type, status, snapshot_id, direction, created_at, started_at, completed_at, progress_current, progress_total, progress_message, error_msg, metadata) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			t.Type, t.Status, newSnapID, dir, t.CreatedAt, t.StartedAt, t.CompletedAt, t.ProgressCurrent, t.ProgressTotal, progMsg, errMsg, meta,
		); err != nil {
			return fmt.Errorf("写入 sync_task 失败: %w", err)
		}
	}

	// 写入 snapshot_files（映射 snapshot_id）
	for _, f := range oldSnapshotFiles {
		newSnapID, ok := snapIDMap[f.SnapshotID]
		if !ok {
			continue
		}
		hash := ""
		if f.Hash != nil {
			hash = *f.Hash
		}
		if _, err := newDB.Exec(
			"INSERT INTO snapshot_files (snapshot_id, path, original_path, size, hash, modified_at, content, tool_type, category, is_binary, is_symlink, link_target) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			newSnapID, f.Path, f.OriginalPath, f.Size, hash, f.ModifiedAt, f.Content, f.ToolType, f.Category, f.IsBinary, f.IsSymlink, f.LinkTarget,
		); err != nil {
			return fmt.Errorf("写入 snapshot_file 失败: %w", err)
		}
	}

	// 写入 sync_history（需要重新建立 task_id 映射）
	// 先重建 task ID 映射（按插入顺序）
	taskIDMap := make(map[string]uint)
	var newTaskID uint = 1
	for _, t := range oldSyncTasks {
		taskIDMap[t.ID] = newTaskID
		newTaskID++
	}
	for _, h := range oldSyncHistory {
		newTaskID, ok := taskIDMap[h.TaskID]
		if !ok {
			continue
		}
		dir := ""
		if h.Direction != nil {
			dir = *h.Direction
		}
		errMsg := ""
		if h.ErrorMsg != nil {
			errMsg = *h.ErrorMsg
		}
		if _, err := newDB.Exec(
			"INSERT INTO sync_history (task_id, type, status, direction, started_at, completed_at, duration_ms, success, error_msg) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			newTaskID, h.Type, h.Status, dir, h.StartedAt, h.CompletedAt, h.DurationMS, h.Success, errMsg,
		); err != nil {
			return fmt.Errorf("写入 sync_history 失败: %w", err)
		}
	}

	// 写入 backups（映射 snapshot_id）
	for _, b := range oldBackups {
		var newSnapID *uint
		if b.SnapshotID != nil {
			if id, ok := snapIDMap[*b.SnapshotID]; ok {
				newSnapID = &id
			}
		}
		desc := ""
		if b.Description != nil {
			desc = *b.Description
		}
		if _, err := newDB.Exec(
			"INSERT INTO backups (created_at, path, size, file_count, snapshot_id, description) VALUES (?, ?, ?, ?, ?, ?)",
			b.CreatedAt, b.Path, b.Size, b.FileCount, newSnapID, desc,
		); err != nil {
			return fmt.Errorf("写入 backup 失败: %w", err)
		}
	}

	// 写入无外键关联的表
	insertSimple := func(tableName, columns string, rows [][]interface{}) error {
		if len(rows) == 0 {
			return nil
		}
		colList := strings.Split(columns, ", ")
		placeholders := strings.Repeat("?, ", len(colList))
		placeholders = placeholders[:len(placeholders)-2]
		for _, row := range rows {
			if _, err := newDB.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, columns, placeholders), row...); err != nil {
				return fmt.Errorf("写入 %s 失败: %w", tableName, err)
			}
		}
		return nil
	}
	if err := insertSimple("remote_configs", "name, url, auth_type, auth_username, auth_password, auth_ssh_key, auth_passphrase, branch, is_default, created_at, updated_at, last_synced, status", remoteConfigs); err != nil {
		return err
	}
	if err := insertSimple("custom_sync_rules", "tool_type, absolute_path, created_at, updated_at", customSyncRules); err != nil {
		return err
	}
	if err := insertSimple("registered_projects", "tool_type, project_name, project_path, created_at, updated_at", registeredProjects); err != nil {
		return err
	}
	if err := insertSimple("ai_settings", "name, token, base_url, opus_model, sonnet_model, models_json, created_at, updated_at", aiSettings); err != nil {
		return err
	}
	if err := insertSimple("app_settings", "version, remote_config_id, sync_auto_sync, sync_interval, sync_conflict_policy, sync_excludes, sync_includes, encryption_enabled, encryption_algorithm, encryption_key_path, ui_theme, ui_language, ui_auto_start, ui_minimize_to_tray, notifications_enabled, notifications_sync_success, notifications_sync_failure, notifications_conflict, notifications_new_snapshot, notifications_sound, created_at, updated_at", appSettings); err != nil {
		return err
	}

	logger.Info("数据库迁移成功，旧数据库已备份到: " + backupPath)
	return nil
}

// GetConn 暴露 GORM 连接给 DAO 层使用。
func (db *DB) GetConn() *gorm.DB {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.conn
}

// migrate 完成建表、列迁移和建索引。
func (db *DB) migrate() error {
	return db.conn.Transaction(func(tx *gorm.DB) error {
		if err := db.createTables(tx); err != nil {
			return err
		}
		if err := db.alterTables(tx); err != nil {
			return err
		}
		if err := db.createIndexes(tx); err != nil {
			return err
		}
		return nil
	})
}

// alterTables 为已有表补充新列。
// 为什么：项目不使用 GORM AutoMigrate（避免 SQLite 上重建表的风险），
// 但 CREATE TABLE IF NOT EXISTS 不会给已存在的表加新列。
// 用 ALTER TABLE ADD COLUMN 安全地补列，SQLite 下列不存在时会报错，用 IF NOT EXISTS 语义的 try-add 模式处理。
func (db *DB) alterTables(tx *gorm.DB) error {
	// snapshot_files 新增 is_symlink 和 link_target 列（v0.x → v0.x+1）
	alterSQL := []string{
		"ALTER TABLE snapshot_files ADD COLUMN is_symlink INTEGER DEFAULT 0",
		"ALTER TABLE snapshot_files ADD COLUMN link_target TEXT DEFAULT ''",
		"ALTER TABLE ai_settings ADD COLUMN models_json TEXT DEFAULT ''",
	}
	for _, sql := range alterSQL {
		// SQLite 不支持 IF NOT EXISTS for ALTER TABLE ADD COLUMN，
		// 重复执行会报 "duplicate column name"，忽略此错误即可。
		if err := tx.Exec(sql).Error; err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
		}
	}
	return nil
}

// createTables 创建表（如果不存在）。
// 为什么不用 AutoMigrate：GORM AutoMigrate 在检测到 schema 差异时会尝试重建表，
// 在 SQLite 上中间表命名可能冲突（如 __gorm_automatic_migration），且 NOT NULL 约束
// 重建时若处理不当会报 constraint failed。使用 CREATE TABLE IF NOT EXISTS 更安全。
func (db *DB) createTables(tx *gorm.DB) error {
	tables := []struct {
		name   string
		create string
	}{
		{
			"snapshots",
			`CREATE TABLE IF NOT EXISTS snapshots (
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
			)`,
		},
		{
			"snapshot_files",
			`CREATE TABLE IF NOT EXISTS snapshot_files (
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
				is_binary INTEGER DEFAULT 0,
				is_symlink INTEGER DEFAULT 0,
				link_target TEXT
			)`,
		},
		{
			"sync_tasks",
			`CREATE TABLE IF NOT EXISTS sync_tasks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				type TEXT NOT NULL,
				status TEXT NOT NULL,
				snapshot_id INTEGER,
				direction TEXT,
				created_at DATETIME NOT NULL,
				started_at DATETIME,
				completed_at DATETIME,
				progress_current INTEGER DEFAULT 0,
				progress_total INTEGER DEFAULT 0,
				progress_message TEXT,
				error_msg TEXT,
				metadata TEXT
			)`,
		},
		{
			"remote_configs",
			`CREATE TABLE IF NOT EXISTS remote_configs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE,
				url TEXT NOT NULL,
				auth_type TEXT NOT NULL,
				auth_username TEXT,
				auth_password TEXT,
				auth_ssh_key TEXT,
				auth_passphrase TEXT,
				branch TEXT NOT NULL DEFAULT 'main',
				is_default INTEGER DEFAULT 0,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL,
				last_synced DATETIME,
				status TEXT NOT NULL DEFAULT 'inactive'
			)`,
		},
		{
			"app_settings",
			`CREATE TABLE IF NOT EXISTS app_settings (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version TEXT NOT NULL,
				remote_config_id INTEGER,
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
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)`,
		},
		{
			"custom_sync_rules",
			`CREATE TABLE IF NOT EXISTS custom_sync_rules (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				tool_type TEXT NOT NULL,
				absolute_path TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)`,
		},
		{
			"registered_projects",
			`CREATE TABLE IF NOT EXISTS registered_projects (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				tool_type TEXT NOT NULL,
				project_name TEXT NOT NULL,
				project_path TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)`,
		},
		{
			"backups",
			`CREATE TABLE IF NOT EXISTS backups (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				created_at DATETIME NOT NULL,
				path TEXT NOT NULL,
				size INTEGER NOT NULL,
				file_count INTEGER NOT NULL,
				snapshot_id INTEGER,
				description TEXT
			)`,
		},
		{
			"sync_history",
			`CREATE TABLE IF NOT EXISTS sync_history (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				task_id INTEGER NOT NULL,
				type TEXT NOT NULL,
				status TEXT NOT NULL,
				direction TEXT,
				started_at DATETIME NOT NULL,
				completed_at DATETIME,
				duration_ms INTEGER,
				success INTEGER DEFAULT 0,
				error_msg TEXT
			)`,
		},
		{
			"ai_settings",
			`CREATE TABLE IF NOT EXISTS ai_settings (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE,
				token TEXT NOT NULL,
				base_url TEXT,
				opus_model TEXT,
				sonnet_model TEXT,
				models_json TEXT DEFAULT '',
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			)`,
		},
	}

	for _, t := range tables {
		if err := tx.Exec(t.create).Error; err != nil {
			return fmt.Errorf("创建表 %s 失败: %w", t.name, err)
		}
	}

	return nil
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
	IsSymlink    bool      `gorm:"column:is_symlink;default:false"`
	LinkTarget   string    `gorm:"column:link_target;type:TEXT"`
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
	ID          uint      `gorm:"column:id;primaryKey;autoIncrement"`
	Name        string    `gorm:"column:name;not null;uniqueIndex"`
	Token       string    `gorm:"column:token;not null"`
	BaseURL     string    `gorm:"column:base_url"`
	OpusModel   string    `gorm:"column:opus_model"`
	SonnetModel string    `gorm:"column:sonnet_model"`
		ModelsJSON  string    `gorm:"column:models_json"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;not null;autoUpdateTime"`
}

func (aiSettingRecord) TableName() string { return "ai_settings" }
