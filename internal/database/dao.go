package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"ai-sync-manager/internal/models"
)

// SnapshotDAO 快照数据访问对象
type SnapshotDAO struct {
	db *DB
}

// NewSnapshotDAO 创建快照 DAO
func NewSnapshotDAO(db *DB) *SnapshotDAO {
	return &SnapshotDAO{db: db}
}

// Create 创建快照
func (dao *SnapshotDAO) Create(snapshot *models.Snapshot) error {
	conn := dao.db.GetConn()

	// 序列化工具列表
	toolsJSON, err := json.Marshal(snapshot.Tools)
	if err != nil {
		return fmt.Errorf("序列化工具列表失败: %w", err)
	}

	// 序列化标签
	tagsJSON, err := json.Marshal(snapshot.Tags)
	if err != nil {
		return fmt.Errorf("序列化标签失败: %w", err)
	}

	// 序列化元数据
	metadataJSON, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %w", err)
	}

	query := `
		INSERT INTO snapshots (id, name, description, message, created_at, tools, metadata, tags, commit_hash, file_count, total_size)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = conn.Exec(query,
		snapshot.ID,
		snapshot.Name,
		snapshot.Description,
		snapshot.Message,
		snapshot.CreatedAt.Unix(),
		string(toolsJSON),
		string(metadataJSON),
		string(tagsJSON),
		snapshot.CommitHash,
		len(snapshot.Files),
		calculateTotalSize(snapshot.Files),
	)

	return err
}

// GetByID 根据 ID 获取快照
func (dao *SnapshotDAO) GetByID(id string) (*models.Snapshot, error) {
	conn := dao.db.GetConn()

	query := `
		SELECT id, name, description, message, created_at, tools, metadata, tags, commit_hash, file_count, total_size
		FROM snapshots
		WHERE id = ?
	`

	row := conn.QueryRow(query, id)

	var (
		name, description, message, toolsJSON, metadataJSON, tagsJSON, commitHash string
		createdAt                                                            int64
		fileCount, totalSize                                                   int
	)

	err := row.Scan(
		&id, &name, &description, &message, &createdAt, &toolsJSON, &metadataJSON, &tagsJSON, &commitHash, &fileCount, &totalSize,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("快照不存在")
	}
	if err != nil {
		return nil, err
	}

	snapshot := &models.Snapshot{
		ID:          id,
		Name:        name,
		Description: description,
		Message:     message,
		CreatedAt:   time.Unix(createdAt, 0),
		CommitHash:  commitHash,
	}

	// 反序列化工具列表
	if err := json.Unmarshal([]byte(toolsJSON), &snapshot.Tools); err != nil {
		return nil, fmt.Errorf("反序列化工具列表失败: %w", err)
	}

	// 反序列化标签
	if err := json.Unmarshal([]byte(tagsJSON), &snapshot.Tags); err != nil {
		return nil, fmt.Errorf("反序列化标签失败: %w", err)
	}

	// 反序列化元数据
	if err := json.Unmarshal([]byte(metadataJSON), &snapshot.Metadata); err != nil {
		return nil, fmt.Errorf("反序列化元数据失败: %w", err)
	}

	return snapshot, nil
}

// List 列出所有快照
func (dao *SnapshotDAO) List(limit, offset int) ([]*models.Snapshot, error) {
	conn := dao.db.GetConn()

	query := `
		SELECT id, name, description, message, created_at, tools, metadata, tags, commit_hash, file_count, total_size
		FROM snapshots
		ORDER BY created_at DESC
	`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", offset)
	}

	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []*models.Snapshot
	for rows.Next() {
		var (
			id, name, description, message, toolsJSON, metadataJSON, tagsJSON, commitHash string
			createdAt                                                             int64
			fileCount, totalSize                                                   int
		)

		if err := rows.Scan(
			&id, &name, &description, &message, &createdAt, &toolsJSON, &metadataJSON, &tagsJSON, &commitHash, &fileCount, &totalSize,
		); err != nil {
			return nil, err
		}

		snapshot := &models.Snapshot{
			ID:          id,
			Name:        name,
			Description: description,
			Message:     message,
			CreatedAt:   time.Unix(createdAt, 0),
			CommitHash:  commitHash,
		}

		_ = json.Unmarshal([]byte(toolsJSON), &snapshot.Tools)
		_ = json.Unmarshal([]byte(tagsJSON), &snapshot.Tags)
		_ = json.Unmarshal([]byte(metadataJSON), &snapshot.Metadata)

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// Update 更新快照
func (dao *SnapshotDAO) Update(snapshot *models.Snapshot) error {
	conn := dao.db.GetConn()

	toolsJSON, _ := json.Marshal(snapshot.Tools)
	tagsJSON, _ := json.Marshal(snapshot.Tags)
	metadataJSON, _ := json.Marshal(snapshot.Metadata)

	query := `
		UPDATE snapshots
		SET name = ?, description = ?, message = ?, tools = ?, metadata = ?, tags = ?, commit_hash = ?
		WHERE id = ?
	`

	_, err := conn.Exec(query,
		snapshot.Name,
		snapshot.Description,
		snapshot.Message,
		string(toolsJSON),
		string(metadataJSON),
		string(tagsJSON),
		snapshot.CommitHash,
		snapshot.ID,
	)

	return err
}

// Delete 删除快照
func (dao *SnapshotDAO) Delete(id string) error {
	conn := dao.db.GetConn()

	_, err := conn.Exec("DELETE FROM snapshots WHERE id = ?", id)
	return err
}

// Count 统计快照数量
func (dao *SnapshotDAO) Count() (int, error) {
	conn := dao.db.GetConn()

	var count int
	err := conn.QueryRow("SELECT COUNT(*) FROM snapshots").Scan(&count)
	return count, err
}

// calculateTotalSize 计算文件总大小
func calculateTotalSize(files []models.SnapshotFile) int64 {
	var total int64
	for _, file := range files {
		total += file.Size
	}
	return total
}

// SyncTaskDAO 同步任务数据访问对象
type SyncTaskDAO struct {
	db *DB
}

// NewSyncTaskDAO 创建同步任务 DAO
func NewSyncTaskDAO(db *DB) *SyncTaskDAO {
	return &SyncTaskDAO{db: db}
}

// Create 创建同步任务
func (dao *SyncTaskDAO) Create(task *models.SyncTask) error {
	conn := dao.db.GetConn()

	metadataJSON, _ := json.Marshal(task.Metadata)

	query := `
		INSERT INTO sync_tasks (id, type, status, snapshot_id, direction, created_at, started_at, completed_at,
			progress_current, progress_total, progress_message, error_msg, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// 处理 snapshot_id：空字符串转为 NULL
	var snapshotID interface{} = nil
	if task.SnapshotID != "" {
		snapshotID = task.SnapshotID
	}

	_, err := conn.Exec(query,
		task.ID,
		string(task.Type),
		string(task.Status),
		snapshotID,
		string(task.Direction),
		task.CreatedAt.Unix(),
		timeToUnix(task.StartedAt),
		timeToUnix(task.CompletedAt),
		task.Progress.Current,
		task.Progress.Total,
		task.Progress.Message,
		task.Error,
		string(metadataJSON),
	)

	return err
}

// GetByID 根据 ID 获取同步任务
func (dao *SyncTaskDAO) GetByID(id string) (*models.SyncTask, error) {
	conn := dao.db.GetConn()

	query := `
		SELECT id, type, status, snapshot_id, direction, created_at, started_at, completed_at,
			progress_current, progress_total, progress_message, error_msg, metadata
		FROM sync_tasks
		WHERE id = ?
	`

	row := conn.QueryRow(query, id)

	var (
		taskType, status, direction, progressMessage, errorMsg, metadataJSON string
		snapshotID                                                             sql.NullString
		createdAt                                                              int64
		startedAt, completedAt                                                 sql.NullInt64
		progressCurrent, progressTotal                                           int
	)

	err := row.Scan(
		&id, &taskType, &status, &snapshotID, &direction, &createdAt, &startedAt, &completedAt,
		&progressCurrent, &progressTotal, &progressMessage, &errorMsg, &metadataJSON,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("任务不存在")
	}
	if err != nil {
		return nil, err
	}

	task := &models.SyncTask{
		ID:         id,
		Type:       models.SyncTaskType(taskType),
		Status:     models.SyncTaskStatus(status),
		SnapshotID: snapshotID.String,
		Direction:  models.SyncDirection(direction),
		CreatedAt:  time.Unix(createdAt, 0),
		Progress: models.TaskProgress{
			Current: progressCurrent,
			Total:   progressTotal,
			Message: progressMessage,
		},
		Error: errorMsg,
	}

	if startedAt.Valid {
		t := time.Unix(startedAt.Int64, 0)
		task.StartedAt = &t
	}

	if completedAt.Valid {
		t := time.Unix(completedAt.Int64, 0)
		task.CompletedAt = &t
	}

	_ = json.Unmarshal([]byte(metadataJSON), &task.Metadata)

	return task, nil
}

// Update 更新同步任务
func (dao *SyncTaskDAO) Update(task *models.SyncTask) error {
	conn := dao.db.GetConn()

	metadataJSON, _ := json.Marshal(task.Metadata)

	query := `
		UPDATE sync_tasks
		SET status = ?, started_at = ?, completed_at = ?, progress_current = ?, progress_total = ?,
			progress_message = ?, error_msg = ?, metadata = ?
		WHERE id = ?
	`

	_, err := conn.Exec(query,
		string(task.Status),
		timeToUnixPtr(task.StartedAt),
		timeToUnixPtr(task.CompletedAt),
		task.Progress.Current,
		task.Progress.Total,
		task.Progress.Message,
		task.Error,
		string(metadataJSON),
		task.ID,
	)

	return err
}

// List 列出同步任务
func (dao *SyncTaskDAO) List(limit, offset int, status models.SyncTaskStatus) ([]*models.SyncTask, error) {
	conn := dao.db.GetConn()

	query := `
		SELECT id, type, status, snapshot_id, direction, created_at, started_at, completed_at,
			progress_current, progress_total, progress_message, error_msg, metadata
		FROM sync_tasks
	`

	if status != "" {
		query += " WHERE status = '" + string(status) + "'"
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	if offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", offset)
	}

	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.SyncTask
	for rows.Next() {
		var (
			id, taskType, taskStatus, direction, progressMessage, errorMsg, metadataJSON string
			snapshotID                                                                 sql.NullString
			createdAt                                                                  int64
			startedAt, completedAt                                                     sql.NullInt64
			progressCurrent, progressTotal                                                int
		)

		if err := rows.Scan(
			&id, &taskType, &taskStatus, &snapshotID, &direction, &createdAt, &startedAt, &completedAt,
			&progressCurrent, &progressTotal, &progressMessage, &errorMsg, &metadataJSON,
		); err != nil {
			return nil, err
		}

		task := &models.SyncTask{
			ID:         id,
			Type:       models.SyncTaskType(taskType),
			Status:     models.SyncTaskStatus(taskStatus),
			SnapshotID: snapshotID.String,
			Direction:  models.SyncDirection(direction),
			CreatedAt:  time.Unix(createdAt, 0),
			Progress: models.TaskProgress{
				Current: progressCurrent,
				Total:   progressTotal,
				Message: progressMessage,
			},
			Error: errorMsg,
		}

		if startedAt.Valid {
			t := time.Unix(startedAt.Int64, 0)
			task.StartedAt = &t
		}

		if completedAt.Valid {
			t := time.Unix(completedAt.Int64, 0)
			task.CompletedAt = &t
		}

		_ = json.Unmarshal([]byte(metadataJSON), &task.Metadata)

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// timeToUnix 将时间转换为 Unix 时间戳
func timeToUnix(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix()
}

// timeToUnixPtr 将时间指针转换为 Unix 时间戳
func timeToUnixPtr(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.Unix(), Valid: true}
}

// RemoteConfigDAO 远端配置数据访问对象
type RemoteConfigDAO struct {
	db *DB
}

// NewRemoteConfigDAO 创建远端配置 DAO
func NewRemoteConfigDAO(db *DB) *RemoteConfigDAO {
	return &RemoteConfigDAO{db: db}
}

// Create 创建远端配置
func (dao *RemoteConfigDAO) Create(config *models.RemoteConfig) error {
	conn := dao.db.GetConn()

	query := `
		INSERT INTO remote_configs (id, name, url, auth_type, auth_username, auth_password,
			auth_ssh_key, auth_passphrase, branch, is_default, created_at, updated_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := conn.Exec(query,
		config.ID,
		config.Name,
		config.URL,
		string(config.Auth.Type),
		config.Auth.Username,
		config.Auth.Password,
		config.Auth.SSHKey,
		config.Auth.Passphrase,
		config.Branch,
		boolToInt(config.IsDefault),
		config.CreatedAt.Unix(),
		config.UpdatedAt.Unix(),
		string(config.Status),
	)

	return err
}

// GetDefault 获取默认配置
func (dao *RemoteConfigDAO) GetDefault() (*models.RemoteConfig, error) {
	conn := dao.db.GetConn()

	query := `
		SELECT id, name, url, auth_type, auth_username, auth_password, auth_ssh_key, auth_passphrase,
			branch, is_default, created_at, updated_at, last_synced, status
		FROM remote_configs
		WHERE is_default = 1
		LIMIT 1
	`

	row := conn.QueryRow(query)

	var (
		id, name, url, authType, username, password, sshKey, passphrase, branch, status string
		isDefault                                                           int
		createdAt, updatedAt                                                 int64
		lastSynced                                                          sql.NullInt64
	)

	err := row.Scan(
		&id, &name, &url, &authType, &username, &password, &sshKey, &passphrase,
			&branch, &isDefault, &createdAt, &updatedAt, &lastSynced, &status,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("未找到默认配置")
	}
	if err != nil {
		return nil, err
	}

	config := &models.RemoteConfig{
		ID:        id,
		Name:      name,
		URL:       url,
		Branch:    branch,
		IsDefault: intToBool(isDefault),
		CreatedAt: time.Unix(createdAt, 0),
		UpdatedAt: time.Unix(updatedAt, 0),
		Status:    models.ConfigStatus(status),
		Auth: models.AuthConfig{
			Type:       models.AuthType(authType),
			Username:   username,
			Password:   password,
			SSHKey:     sshKey,
			Passphrase: passphrase,
		},
	}

	if lastSynced.Valid {
		t := time.Unix(lastSynced.Int64, 0)
		config.LastSynced = &t
	}

	return config, nil
}

// List 列出所有远端配置
func (dao *RemoteConfigDAO) List() ([]*models.RemoteConfig, error) {
	conn := dao.db.GetConn()

	query := `
		SELECT id, name, url, auth_type, auth_username, auth_password, auth_ssh_key, auth_passphrase,
			branch, is_default, created_at, updated_at, last_synced, status
		FROM remote_configs
		ORDER BY created_at DESC
	`

	rows, err := conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*models.RemoteConfig
	for rows.Next() {
		var (
			id, name, url, authType, username, password, sshKey, passphrase, branch, status string
			isDefault                                                           int
			createdAt, updatedAt                                                 int64
			lastSynced                                                          sql.NullInt64
		)

		if err := rows.Scan(
			&id, &name, &url, &authType, &username, &password, &sshKey, &passphrase,
			&branch, &isDefault, &createdAt, &updatedAt, &lastSynced, &status,
		); err != nil {
			return nil, err
		}

		config := &models.RemoteConfig{
			ID:        id,
			Name:      name,
			URL:       url,
			Branch:    branch,
			IsDefault: intToBool(isDefault),
			CreatedAt: time.Unix(createdAt, 0),
			UpdatedAt: time.Unix(updatedAt, 0),
			Status:    models.ConfigStatus(status),
			Auth: models.AuthConfig{
				Type:       models.AuthType(authType),
				Username:   username,
				Password:   password,
				SSHKey:     sshKey,
				Passphrase: passphrase,
			},
		}

		if lastSynced.Valid {
			t := time.Unix(lastSynced.Int64, 0)
			config.LastSynced = &t
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// Update 更新远端配置
func (dao *RemoteConfigDAO) Update(config *models.RemoteConfig) error {
	conn := dao.db.GetConn()

	query := `
		UPDATE remote_configs
		SET name = ?, url = ?, auth_type = ?, auth_username = ?, auth_password = ?,
			auth_ssh_key = ?, auth_passphrase = ?, branch = ?, updated_at = ?, status = ?
		WHERE id = ?
	`

	now := time.Now()
	config.UpdatedAt = now

	_, err := conn.Exec(query,
		config.Name,
		config.URL,
		string(config.Auth.Type),
		config.Auth.Username,
		config.Auth.Password,
		config.Auth.SSHKey,
		config.Auth.Passphrase,
		config.Branch,
		now.Unix(),
		string(config.Status),
		config.ID,
	)

	return err
}

// SetDefault 设置默认配置
func (dao *RemoteConfigDAO) SetDefault(id string) error {
	conn := dao.db.GetConn()

	// 取消所有默认
	_, err := conn.Exec("UPDATE remote_configs SET is_default = 0")
	if err != nil {
		return err
	}

	// 设置新的默认
	_, err = conn.Exec("UPDATE remote_configs SET is_default = 1 WHERE id = ?", id)
	return err
}

// Delete 删除远端配置
func (dao *RemoteConfigDAO) Delete(id string) error {
	conn := dao.db.GetConn()

	_, err := conn.Exec("DELETE FROM remote_configs WHERE id = ?", id)
	return err
}

// 辅助函数
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func intToBool(i int) bool {
	return i != 0
}
