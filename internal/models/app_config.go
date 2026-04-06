package models

import (
	"time"
)

// AppSettings 应用设置
type AppSettings struct {
	ID            string               `json:"id" db:"id"`                           // 设置 ID
	Version       string               `json:"version" db:"version"`                 // 设置版本
	Remote        *RemoteConfig        `json:"remote,omitempty" db:"remote"`         // 远端配置
	Sync          *SyncConfig          `json:"sync,omitempty" db:"sync"`             // 同步配置
	Encryption    *EncryptionConfig    `json:"encryption,omitempty" db:"encryption"` // 加密配置
	UI            UISettings           `json:"ui" db:"ui"`                           // UI 设置
	Notifications NotificationSettings `json:"notifications" db:"notifications"`     // 通知设置
	CreatedAt     time.Time            `json:"created_at" db:"created_at"`           // 创建时间
	UpdatedAt     time.Time            `json:"updated_at" db:"updated_at"`           // 更新时间
}

// UISettings UI 设置
type UISettings struct {
	Theme             string `json:"theme"`              // 主题（light/dark/auto）
	Language          string `json:"language"`           // 语言
	AutoStart         bool   `json:"auto_start"`         // 是否自动启动
	MinimizeToTray    bool   `json:"minimize_to_tray"`   // 最小化到托盘
	ShowNotifications bool   `json:"show_notifications"` // 显示通知
}

// NotificationSettings 通知设置
type NotificationSettings struct {
	Enabled          bool   `json:"enabled"`           // 是否启用通知
	SyncSuccess      bool   `json:"sync_success"`      // 同步成功通知
	SyncFailure      bool   `json:"sync_failure"`      // 同步失败通知
	ConflictDetected bool   `json:"conflict_detected"` // 冲突检测通知
	NewSnapshot      bool   `json:"new_snapshot"`      // 新快照通知
	Sound            string `json:"sound"`             // 提示音
}

// SyncConfig 同步配置（AppSettings JSON 列映射）
type SyncConfig struct {
	AutoSync   bool          `json:"auto_sync"`    // 是否自动同步
	SyncInterval time.Duration `json:"sync_interval"` // 同步间隔
	Excludes   []string      `json:"excludes"`     // 排除的文件/目录
	Includes   []string      `json:"includes"`     // 包含的文件/目录
}

// EncryptionConfig 加密配置（AppSettings JSON 列映射）
type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`     // 是否启用加密
	Algorithm string `json:"algorithm"`   // 加密算法（aes256-gcm 等）
	KeyPath   string `json:"key_path"`    // 密钥文件路径
	KeyEnvVar string `json:"key_env_var"` // 密钥环境变量名
}
