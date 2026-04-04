package models

import (
	"errors"
	"fmt"
	"time"

	"ai-sync-manager/pkg/database"

	"gorm.io/gorm"
)

// RemoteConfig 远端仓库配置
type RemoteConfig struct {
	ID         string       `json:"id" db:"id"`                             // 配置 ID
	Name       string       `json:"name" db:"name"`                         // 配置名称
	URL        string       `json:"url" db:"url"`                           // 仓库 URL
	Auth       AuthConfig   `json:"auth" db:"auth"`                         // 认证配置
	Branch     string       `json:"branch" db:"branch"`                     // 分支名
	IsDefault  bool         `json:"is_default" db:"is_default"`             // 是否为默认配置
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`             // 创建时间
	UpdatedAt  time.Time    `json:"updated_at" db:"updated_at"`             // 更新时间
	LastSynced *time.Time   `json:"last_synced,omitempty" db:"last_synced"` // 最后同步时间
	Status     ConfigStatus `json:"status" db:"status"`                     // 配置状态
}

// AuthConfig 认证配置
type AuthConfig struct {
	Type       AuthType `json:"type"`                 // 认证类型
	Username   string   `json:"username,omitempty"`   // 用户名
	Password   string   `json:"password,omitempty"`   // 密码/Token（加密）
	SSHKey     string   `json:"ssh_key,omitempty"`    // SSH 密钥（加密）
	Passphrase string   `json:"passphrase,omitempty"` // SSH 密钥密码（加密）
}

// AuthType 认证类型
type AuthType string

const (
	AuthTypeNone  AuthType = ""      // 无认证
	AuthTypeSSH   AuthType = "ssh"   // SSH 密钥
	AuthTypeToken AuthType = "token" // Token 认证
	AuthTypeBasic AuthType = "basic" // 用户名密码
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
	URL          string    `json:"url"`            // 仓库 URL
	Host         string    `json:"host"`           // 主机
	Owner        string    `json:"owner"`          // 所有者
	Name         string    `json:"name"`           // 仓库名
	Branch       string    `json:"branch"`         // 当前分支
	LastCommit   string    `json:"last_commit"`    // 最后提交哈希
	LastCommitAt time.Time `json:"last_commit_at"` // 最后提交时间
	IsAccessible bool      `json:"is_accessible"`  // 是否可访问
}

// RemoteSnapshot 远端快照信息
type RemoteSnapshot struct {
	ID          string    `json:"id"`          // 快照 ID
	Name        string    `json:"name"`        // 快照名称
	Description string    `json:"description"` // 快照描述
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	CommitHash  string    `json:"commit_hash"` // 提交哈希
	Message     string    `json:"message"`     // 提交消息
	Tools       []string  `json:"tools"`       // 包含的工具
	IsLatest    bool      `json:"is_latest"`   // 是否为最新
}

// SyncScope 同步范围配置
type SyncScope struct {
	Global     bool     `json:"global"`     // 是否同步全局配置
	Projects   []string `json:"projects"`   // 要同步的项目路径
	Categories []string `json:"categories"` // 要同步的文件类别
	Excludes   []string `json:"excludes"`   // 排除的文件/目录
}

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`     // 是否启用加密
	Algorithm string `json:"algorithm"`   // 加密算法（aes256-gcm 等）
	KeyPath   string `json:"key_path"`    // 密钥文件路径
	KeyEnvVar string `json:"key_env_var"` // 密钥环境变量名
}

// SensitiveData 敏感数据类型
type SensitiveData struct {
	Type     SensitiveType `json:"type"`    // 数据类型
	Content  string        `json:"content"` // 加密后的内容
	Original string        `json:"-"`       // 原始内容（不序列化）
}

// SensitiveType 敏感数据类型
type SensitiveType string

const (
	SensitiveTypeToken    SensitiveType = "token"    // Token
	SensitiveTypePassword SensitiveType = "password" // 密码
	SensitiveTypeSSHKey   SensitiveType = "ssh_key"  // SSH 密钥
	SensitiveTypeAPIKey   SensitiveType = "api_key"  // API 密钥
	SensitiveTypeSecret   SensitiveType = "secret"   // 其他密钥
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
	Valid    bool     `json:"valid"`    // 是否有效
	Errors   []string `json:"errors"`   // 错误列表
	Warnings []string `json:"warnings"` // 警告列表
}

// RepositoryTestResult 仓库测试结果
type RepositoryTestResult struct {
	Success      bool     `json:"success"`       // 是否成功
	URL          string   `json:"url"`           // 仓库 URL
	IsAccessible bool     `json:"is_accessible"` // 是否可访问
	AuthType     AuthType `json:"auth_type"`     // 使用的认证类型
	Branches     []string `json:"branches"`      // 可用分支
	Error        string   `json:"error"`         // 错误信息
	Latency      int64    `json:"latency"`       // 延迟（毫秒）
}

// SyncHistory 同步历史记录
type SyncHistory struct {
	ID          string         `json:"id"`           // 记录 ID
	TaskID      string         `json:"task_id"`      // 任务 ID
	Type        SyncTaskType   `json:"type"`         // 任务类型
	Status      SyncTaskStatus `json:"status"`       // 任务状态
	Direction   SyncDirection  `json:"direction"`    // 同步方向
	StartedAt   time.Time      `json:"started_at"`   // 开始时间
	CompletedAt *time.Time     `json:"completed_at"` // 完成时间
	Duration    string         `json:"duration"`     // 执行时长
	Success     bool           `json:"success"`      // 是否成功
	Error       string         `json:"error"`        // 错误信息
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
	row := remoteConfigToRow(config)
	return dao.db.GetConn().Create(&row).Error
}

// GetDefault 获取默认配置
func (dao *RemoteConfigDAO) GetDefault() (*RemoteConfig, error) {
	var row remoteConfigRow
	err := dao.db.GetConn().
		Where("is_default = ?", true).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("未找到默认配置")
	}
	if err != nil {
		return nil, err
	}
	config := row.toModel()
	return &config, nil
}

// List 列出所有远端配置
func (dao *RemoteConfigDAO) List() ([]*RemoteConfig, error) {
	var rows []remoteConfigRow
	if err := dao.db.GetConn().Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}

	configs := make([]*RemoteConfig, 0, len(rows))
	for _, row := range rows {
		config := row.toModel()
		configs = append(configs, &config)
	}
	return configs, nil
}

// Update 更新远端配置
func (dao *RemoteConfigDAO) Update(config *RemoteConfig) error {
	row := remoteConfigToRow(config)
	row.UpdatedAt = time.Now()

	return dao.db.GetConn().
		Model(&remoteConfigRow{}).
		Where("id = ?", config.ID).
		Updates(map[string]interface{}{
			"name":            row.Name,
			"url":             row.URL,
			"auth_type":       row.AuthType,
			"auth_username":   row.AuthUsername,
			"auth_password":   row.AuthPassword,
			"auth_ssh_key":    row.AuthSSHKey,
			"auth_passphrase": row.AuthPassphrase,
			"branch":          row.Branch,
			"updated_at":      row.UpdatedAt,
			"status":          row.Status,
			"last_synced":     row.LastSynced,
		}).Error
}

// SetDefault 设置默认配置
func (dao *RemoteConfigDAO) SetDefault(id string) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&remoteConfigRow{}).Where("1 = 1").Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&remoteConfigRow{}).Where("id = ?", id).Update("is_default", true).Error
	})
}

// Delete 删除远端配置
func (dao *RemoteConfigDAO) Delete(id string) error {
	return dao.db.GetConn().Delete(&remoteConfigRow{}, "id = ?", id).Error
}

type remoteConfigRow struct {
	ID             string     `gorm:"column:id;primaryKey"`
	Name           string     `gorm:"column:name"`
	URL            string     `gorm:"column:url"`
	AuthType       string     `gorm:"column:auth_type"`
	AuthUsername   string     `gorm:"column:auth_username"`
	AuthPassword   string     `gorm:"column:auth_password"`
	AuthSSHKey     string     `gorm:"column:auth_ssh_key"`
	AuthPassphrase string     `gorm:"column:auth_passphrase"`
	Branch         string     `gorm:"column:branch"`
	IsDefault      bool       `gorm:"column:is_default"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	LastSynced     *time.Time `gorm:"column:last_synced"`
	Status         string     `gorm:"column:status"`
}

func (remoteConfigRow) TableName() string {
	return "remote_configs"
}

func remoteConfigToRow(config *RemoteConfig) remoteConfigRow {
	return remoteConfigRow{
		ID:             config.ID,
		Name:           config.Name,
		URL:            config.URL,
		AuthType:       string(config.Auth.Type),
		AuthUsername:   config.Auth.Username,
		AuthPassword:   config.Auth.Password,
		AuthSSHKey:     config.Auth.SSHKey,
		AuthPassphrase: config.Auth.Passphrase,
		Branch:         config.Branch,
		IsDefault:      config.IsDefault,
		CreatedAt:      config.CreatedAt,
		UpdatedAt:      config.UpdatedAt,
		LastSynced:     config.LastSynced,
		Status:         string(config.Status),
	}
}

func (row remoteConfigRow) toModel() RemoteConfig {
	return RemoteConfig{
		ID:         row.ID,
		Name:       row.Name,
		URL:        row.URL,
		Branch:     row.Branch,
		IsDefault:  row.IsDefault,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
		LastSynced: row.LastSynced,
		Status:     ConfigStatus(row.Status),
		Auth: AuthConfig{
			Type:       AuthType(row.AuthType),
			Username:   row.AuthUsername,
			Password:   row.AuthPassword,
			SSHKey:     row.AuthSSHKey,
			Passphrase: row.AuthPassphrase,
		},
	}
}
