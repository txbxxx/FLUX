package config

import "time"

// UserProfile holds user profile information.
type UserProfile struct {
	ID          string          `json:"id"`               // 用户 ID
	Name        string          `json:"name"`             // 用户名称
	Email       string          `json:"email"`            // 用户邮箱
	Avatar      string          `json:"avatar,omitempty"` // 头像路径
	Preferences UserPreferences `json:"preferences"`      // 用户偏好
	CreatedAt   time.Time       `json:"created_at"`       // 创建时间
	LastLoginAt time.Time       `json:"last_login_at"`    // 最后登录时间
}

// UserPreferences holds user preference settings.
type UserPreferences struct {
	DefaultTools   []string `json:"default_tools"`   // 默认工具
	DefaultScope   string   `json:"default_scope"`   // 默认范围
	AutoBackup     bool     `json:"auto_backup"`     // 自动备份
	BackupInterval int      `json:"backup_interval"` // 备份间隔（小时）
	ConflictPolicy string   `json:"conflict_policy"` // 冲突策略
}

// SystemInfo holds system environment information.
type SystemInfo struct {
	OS           string `json:"os"`            // 操作系统
	Arch         string `json:"arch"`          // 架构
	AppVersion   string `json:"app_version"`   // 应用版本
	GoVersion    string `json:"go_version"`    // Go 版本
	WailsVersion string `json:"wails_version"` // Wails 版本
}

// HealthInfo holds application health check results.
type HealthInfo struct {
	Status      string            `json:"status"`      // 状态（ok/warning/error）
	Version     string            `json:"version"`     // 版本号
	Uptime      string            `json:"uptime"`      // 运行时间
	Checks      []HealthCheck     `json:"checks"`      // 健康检查项
	Environment map[string]string `json:"environment"` // 环境变量
}

// HealthCheck represents a single health check item.
type HealthCheck struct {
	Name     string `json:"name"`     // 检查项名称
	Status   string `json:"status"`   // 状态（ok/warning/error）
	Message  string `json:"message"`  // 检查消息
	Duration int64  `json:"duration"` // 执行时长（毫秒）
}

// LogEntry represents a single log entry.
type LogEntry struct {
	ID        string            `json:"id"`                 // 日志 ID
	Level     string            `json:"level"`              // 日志级别
	Message   string            `json:"message"`            // 日志消息
	Timestamp time.Time         `json:"timestamp"`          // 时间戳
	Source    string            `json:"source"`             // 来源
	Metadata  map[string]string `json:"metadata,omitempty"` // 元数据
}

// Metrics holds application-level metrics.
type Metrics struct {
	TotalSnapshots  int       `json:"total_snapshots"`   // 总快照数
	TotalSyncTasks  int       `json:"total_sync_tasks"`  // 总同步任务数
	SuccessfulSyncs int       `json:"successful_syncs"`  // 成功同步数
	FailedSyncs     int       `json:"failed_syncs"`      // 失败同步数
	TotalBackupSize int64     `json:"total_backup_size"` // 总备份大小
	LastSyncTime    time.Time `json:"last_sync_time"`    // 最后同步时间
	AvgSyncDuration int64     `json:"avg_sync_duration"` // 平均同步时长（毫秒）
}
