package models

import (
	"database/sql"
	"fmt"
	"time"

	"ai-sync-manager/pkg/database"
)

// RemoteConfig 远端仓库配置
type RemoteConfig struct {
	ID          string       `json:"id" db:"id"`                   // 配置 ID
	Name        string       `json:"name" db:"name"`               // 配置名称
	URL         string       `json:"url" db:"url"`                 // 仓库 URL
	Auth        AuthConfig   `json:"auth" db:"auth"`               // 认证配置
	Branch      string       `json:"branch" db:"branch"`           // 分支名
	IsDefault   bool         `json:"is_default" db:"is_default"`   // 是否为默认配置
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`   // 创建时间
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`   // 更新时间
	LastSynced  *time.Time   `json:"last_synced,omitempty" db:"last_synced"` // 最后同步时间
	Status      ConfigStatus `json:"status" db:"status"`           // 配置状态
}

// AuthConfig 认证配置
type AuthConfig struct {
	Type      AuthType `json:"type"`                 // 认证类型
	Username  string   `json:"username,omitempty"`   // 用户名
	Password  string   `json:"password,omitempty"`   // 密码/Token（加密）
	SSHKey    string   `json:"ssh_key,omitempty"`    // SSH 密钥（加密）
	Passphrase string  `json:"passphrase,omitempty"` // SSH 密钥密码（加密）
}

// AuthType 认证类型
type AuthType string

const (
	AuthTypeNone   AuthType = ""        // 无认证
	AuthTypeSSH    AuthType = "ssh"     // SSH 密钥
	AuthTypeToken  AuthType = "token"   // Token 认证
	AuthTypeBasic  AuthType = "basic"   // 用户名密码
)

// ConfigStatus 配置状态
type ConfigStatus string

const (
	StatusActive   ConfigStatus = "active"   // 活跃
	StatusInactive ConfigStatus = "inactive" // 未激活
	StatusError    ConfigStatus = "error"    // 错误
)

// RemoteRepository 远程仓库信息
type RemoteRepository struct {
	URL          string    `json:"url"`           // 仓库 URL
	Host         string    `json:"host"`          // 主机
	Owner        string    `json:"owner"`         // 所有者
	Name         string    `json:"name"`          // 仓库名
	Branch       string    `json:"branch"`        // 当前分支
	LastCommit   string    `json:"last_commit"`   // 最后提交哈希
	LastCommitAt time.Time `json:"last_commit_at"` // 最后提交时间
	IsAccessible bool      `json:"is_accessible"` // 是否可访问
}

// RemoteSnapshot 远端快照信息
type RemoteSnapshot struct {
	ID          string    `json:"id"`           // 快照 ID
	Name        string    `json:"name"`         // 快照名称
	Description string    `json:"description"`  // 快照描述
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	CommitHash  string    `json:"commit_hash"`  // 提交哈希
	Message     string    `json:"message"`      // 提交消息
	Tools       []string  `json:"tools"`        // 包含的工具
	IsLatest    bool      `json:"is_latest"`    // 是否为最新
}

// SyncScope 同步范围配置
type SyncScope struct {
	Global      bool     `json:"global"`       // 是否同步全局配置
	Projects    []string `json:"projects"`     // 要同步的项目路径
	Categories  []string `json:"categories"`   // 要同步的文件类别
	Excludes    []string `json:"excludes"`     // 排除的文件/目录
}

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Enabled    bool   `json:"enabled"`     // 是否启用加密
	Algorithm  string `json:"algorithm"`   // 加密算法（aes256-gcm 等）
	KeyPath    string `json:"key_path"`    // 密钥文件路径
	KeyEnvVar  string `json:"key_env_var"` // 密钥环境变量名
}

// SensitiveData 敏感数据类型
type SensitiveData struct {
	Type     SensitiveType `json:"type"`     // 数据类型
	Content  string        `json:"content"`  // 加密后的内容
	Original string        `json:"-"`        // 原始内容（不序列化）
}

// SensitiveType 敏感数据类型
type SensitiveType string

const (
	SensitiveTypeToken     SensitiveType = "token"     // Token
	SensitiveTypePassword  SensitiveType = "password"  // 密码
	SensitiveTypeSSHKey    SensitiveType = "ssh_key"   // SSH 密钥
	SensitiveTypeAPIKey    SensitiveType = "api_key"   // API 密钥
	SensitiveTypeSecret    SensitiveType = "secret"    // 其他密钥
)

// EncryptionResult 加密结果
type EncryptionResult struct {
	Success   bool   `json:"success"`   // 是否成功
	Data      string `json:"data"`      // 加密后的数据
	Algorithm string `json:"algorithm"` // 使用的算法
	Error     string `json:"error"`     // 错误信息
}

// DecryptionResult 解密结果
type DecryptionResult struct {
	Success bool   `json:"success"` // 是否成功
	Data    string `json:"data"`    // 解密后的数据
	Error   string `json:"error"`   // 错误信息
}

// ConfigValidationResult 配置验证结果
type ConfigValidationResult struct {
	Valid    bool     `json:"valid"`     // 是否有效
	Errors   []string `json:"errors"`    // 错误列表
	Warnings []string `json:"warnings"`  // 警告列表
}

// RepositoryTestResult 仓库测试结果
type RepositoryTestResult struct {
	Success      bool      `json:"success"`       // 是否成功
	URL          string    `json:"url"`           // 仓库 URL
	IsAccessible bool      `json:"is_accessible"` // 是否可访问
	AuthType     AuthType  `json:"auth_type"`     // 使用的认证类型
	Branches      []string  `json:"branches"`      // 可用分支
	Error        string    `json:"error"`         // 错误信息
	Latency      int64     `json:"latency"`       // 延迟（毫秒）
}

// SyncHistory 同步历史记录
type SyncHistory struct {
	ID          string       `json:"id"`           // 记录 ID
	TaskID      string       `json:"task_id"`      // 任务 ID
	Type        SyncTaskType `json:"type"`         // 任务类型
	Status      SyncTaskStatus `json:"status"`     // 任务状态
	Direction   SyncDirection `json:"direction"`   // 同步方向
	StartedAt   time.Time    `json:"started_at"`   // 开始时间
	CompletedAt *time.Time   `json:"completed_at"` // 完成时间
	Duration    string       `json:"duration"`     // 执行时长
	Success     bool         `json:"success"`      // 是否成功
	Error       string       `json:"error"`        // 错误信息
}

// RemoteConfigDAO 远端配置数据访问对象
type RemoteConfigDAO struct {
	db *database.DB
}

// NewRemoteConfigDAO 创建远端配置 DAO
func NewRemoteConfigDAO(db *database.DB) *RemoteConfigDAO {
	return &RemoteConfigDAO{db: db}
}

// Create 创建远端配置
func (dao *RemoteConfigDAO) Create(config *RemoteConfig) error {
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
func (dao *RemoteConfigDAO) GetDefault() (*RemoteConfig, error) {
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
		isDefault   int
		createdAt, updatedAt int64
		lastSynced  sql.NullInt64
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

	config := &RemoteConfig{
		ID:        id,
		Name:      name,
		URL:       url,
		Branch:    branch,
		IsDefault: intToBool(isDefault),
		CreatedAt: time.Unix(createdAt, 0),
		UpdatedAt: time.Unix(updatedAt, 0),
		Status:    ConfigStatus(status),
		Auth: AuthConfig{
			Type:       AuthType(authType),
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
func (dao *RemoteConfigDAO) List() ([]*RemoteConfig, error) {
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

	var configs []*RemoteConfig
	for rows.Next() {
		var (
			id, name, url, authType, username, password, sshKey, passphrase, branch, status string
			isDefault   int
			createdAt, updatedAt int64
			lastSynced  sql.NullInt64
		)

		if err := rows.Scan(
			&id, &name, &url, &authType, &username, &password, &sshKey, &passphrase,
				&branch, &isDefault, &createdAt, &updatedAt, &lastSynced, &status,
		); err != nil {
			return nil, err
		}

		config := &RemoteConfig{
			ID:        id,
			Name:      name,
			URL:       url,
			Branch:    branch,
			IsDefault: intToBool(isDefault),
			CreatedAt: time.Unix(createdAt, 0),
			UpdatedAt: time.Unix(updatedAt, 0),
			Status:    ConfigStatus(status),
			Auth: AuthConfig{
				Type:       AuthType(authType),
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
func (dao *RemoteConfigDAO) Update(config *RemoteConfig) error {
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
