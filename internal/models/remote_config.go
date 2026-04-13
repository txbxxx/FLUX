package models

import (
	"errors"
	"fmt"
	"time"

	"flux/pkg/database"

	"gorm.io/gorm"
)

// RemoteConfig 远端仓库配置
type RemoteConfig struct {
	ID         uint         `json:"id" db:"id" gorm:"column:id;primaryKey;autoIncrement"` // 配置 ID
	Name       string       `json:"name" db:"name"`                                       // 配置名称
	URL        string       `json:"url" db:"url"`                                         // 仓库 URL
	Auth       AuthConfig   `json:"auth" db:"auth"`                                       // 认证配置
	Branch     string       `json:"branch" db:"branch"`                                   // 分支名
	IsDefault  bool         `json:"is_default" db:"is_default"`                           // 是否为默认配置
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`                           // 创建时间
	UpdatedAt  time.Time    `json:"updated_at" db:"updated_at"`                           // 更新时间
	LastSynced *time.Time   `json:"last_synced,omitempty" db:"last_synced"`               // 最后同步时间
	Status     ConfigStatus `json:"status" db:"status"`                                   // 配置状态
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
func (dao *RemoteConfigDAO) SetDefault(id uint) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&remoteConfigRow{}).Where("1 = 1").Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&remoteConfigRow{}).Where("id = ?", id).Update("is_default", true).Error
	})
}

// Delete 删除远端配置
func (dao *RemoteConfigDAO) Delete(id uint) error {
	return dao.db.GetConn().Delete(&remoteConfigRow{}, "id = ?", id).Error
}

type remoteConfigRow struct {
	ID             uint       `gorm:"column:id;primaryKey"`
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
