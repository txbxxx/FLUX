package models

import (
	"time"
)

// AppSettings 应用设置
type AppSettings struct {
	ID            string            `json:"id" db:"id"`                       // 设置 ID
	Version       string            `json:"version" db:"version"`             // 设置版本
	Remote        *RemoteConfig     `json:"remote,omitempty" db:"remote"`     // 远端配置
	Sync          *SyncConfig       `json:"sync,omitempty" db:"sync"`         // 同步配置
	Encryption    *EncryptionConfig `json:"encryption,omitempty" db:"encryption"` // 加密配置
	UI            UISettings        `json:"ui" db:"ui"`                       // UI 设置
	Notifications NotificationSettings `json:"notifications" db:"notifications"` // 通知设置
	CreatedAt     time.Time         `json:"created_at" db:"created_at"`       // 创建时间
	UpdatedAt     time.Time         `json:"updated_at" db:"updated_at"`       // 更新时间
}

// UISettings UI 设置
type UISettings struct {
	Theme        string `json:"theme"`                  // 主题（light/dark/auto）
	Language     string `json:"language"`               // 语言
	AutoStart    bool   `json:"auto_start"`             // 是否自动启动
	MinimizeToTray bool  `json:"minimize_to_tray"`       // 最小化到托盘
	ShowNotifications bool `json:"show_notifications"`  // 显示通知
}

// NotificationSettings 通知设置
type NotificationSettings struct {
	Enabled      bool     `json:"enabled"`        // 是否启用通知
	SyncSuccess  bool     `json:"sync_success"`   // 同步成功通知
	SyncFailure  bool     `json:"sync_failure"`   // 同步失败通知
	ConflictDetected bool  `json:"conflict_detected"` // 冲突检测通知
	NewSnapshot  bool     `json:"new_snapshot"`   // 新快照通知
	Sound        string   `json:"sound"`          // 提示音
}

// UserProfile 用户配置文件
type UserProfile struct {
	ID            string        `json:"id"`                   // 用户 ID
	Name          string        `json:"name"`                 // 用户名称
	Email         string        `json:"email"`                // 用户邮箱
	Avatar        string        `json:"avatar,omitempty"`     // 头像路径
	Preferences   UserPreferences `json:"preferences"`        // 用户偏好
	CreatedAt     time.Time     `json:"created_at"`           // 创建时间
	LastLoginAt   time.Time     `json:"last_login_at"`        // 最后登录时间
}

// UserPreferences 用户偏好
type UserPreferences struct {
	DefaultTools     []string `json:"default_tools"`      // 默认工具
	DefaultScope     string   `json:"default_scope"`      // 默认范围
	AutoBackup       bool     `json:"auto_backup"`        // 自动备份
	BackupInterval   int      `json:"backup_interval"`    // 备份间隔（小时）
	ConflictPolicy   string   `json:"conflict_policy"`    // 冲突策略
}

// ToolProfile 工具配置文件
type ToolProfile struct {
	ToolType      string          `json:"tool_type"`      // 工具类型（codex/claude）
	Name          string          `json:"name"`           // 配置名称
	Description   string          `json:"description"`    // 配置描述
	GlobalPath    string          `json:"global_path"`    // 全局配置路径
	ProjectPaths  []string        `json:"project_paths"`  // 项目路径列表
	SyncEnabled   bool            `json:"sync_enabled"`   // 是否启用同步
	Categories    []FileCategory  `json:"categories"`     // 同步的文件类别
	Excludes      []string        `json:"excludes"`       // 排除的文件
	CreatedAt     time.Time       `json:"created_at"`     // 创建时间
	UpdatedAt     time.Time       `json:"updated_at"`     // 更新时间
}

// SystemInfo 系统信息
type SystemInfo struct {
	OS           string `json:"os"`            // 操作系统
	Arch         string `json:"arch"`          // 架构
	AppVersion   string `json:"app_version"`   // 应用版本
	GoVersion    string `json:"go_version"`    // Go 版本
	WailsVersion string `json:"wails_version"` // Wails 版本
}

// HealthInfo 健康检查信息
type HealthInfo struct {
	Status      string             `json:"status"`       // 状态（ok/warning/error）
	Version     string             `json:"version"`     // 版本号
	Uptime      string             `json:"uptime"`      // 运行时间
	Checks      []HealthCheck      `json:"checks"`      // 健康检查项
	Environment map[string]string `json:"environment"` // 环境变量
}

// HealthCheck 单项健康检查
type HealthCheck struct {
	Name     string `json:"name"`     // 检查项名称
	Status   string `json:"status"`   // 状态（ok/warning/error）
	Message  string `json:"message"`  // 检查消息
	Duration int64  `json:"duration"` // 执行时长（毫秒）
}

// LogEntry 日志条目
type LogEntry struct {
	ID        string    `json:"id"`         // 日志 ID
	Level     string    `json:"level"`      // 日志级别
	Message   string    `json:"message"`    // 日志消息
	Timestamp time.Time `json:"timestamp"`  // 时间戳
	Source    string    `json:"source"`     // 来源
	Metadata  map[string]string `json:"metadata,omitempty"` // 元数据
}

// LogLevel 日志级别
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// Metrics 应用指标
type Metrics struct {
	TotalSnapshots      int `json:"total_snapshots"`      // 总快照数
	TotalSyncTasks      int `json:"total_sync_tasks"`     // 总同步任务数
	SuccessfulSyncs     int `json:"successful_syncs"`     // 成功同步数
	FailedSyncs         int `json:"failed_syncs"`         // 失败同步数
	TotalBackupSize     int64 `json:"total_backup_size"`  // 总备份大小
	LastSyncTime        time.Time `json:"last_sync_time"`  // 最后同步时间
	AvgSyncDuration     int64 `json:"avg_sync_duration"`  // 平均同步时长（毫秒）
}

// ToolStatistics 工具统计
type ToolStatistics struct {
	ToolType       string    `json:"tool_type"`       // 工具类型
	IsInstalled    bool      `json:"is_installed"`    // 是否已安装
	GlobalFiles    int       `json:"global_files"`    // 全局文件数
	ProjectFiles   int       `json:"project_files"`   // 项目文件数
	LastSynced     time.Time `json:"last_synced"`     // 最后同步时间
	SyncCount      int       `json:"sync_count"`      // 同步次数
	ConflictCount  int       `json:"conflict_count"`  // 冲突次数
}

// ProjectStatistics 项目统计
type ProjectStatistics struct {
	Path         string    `json:"path"`          // 项目路径
	HasCodex     bool      `json:"has_codex"`     // 是否有 Codex 配置
	HasClaude    bool      `json:"has_claude"`    // 是否有 Claude 配置
	FileCount    int       `json:"file_count"`    // 文件数量
	LastBackup   time.Time `json:"last_backup"`   // 最后备份时间
	SyncCount    int       `json:"sync_count"`    // 同步次数
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Code    string `json:"code"`    // 错误码
	Message string `json:"message"` // 错误消息
	Details string `json:"details"` // 详细信息
}

// Response 统一响应格式
type Response struct {
	Success bool        `json:"success"` // 是否成功
	Data    interface{} `json:"data"`    // 响应数据
	Error   *ErrorDetail `json:"error"`   // 错误信息
}

// PageRequest 分页请求
type PageRequest struct {
	Page     int    `json:"page"`     // 页码（从 1 开始）
	PageSize int    `json:"page_size"` // 每页大小
	SortBy   string `json:"sort_by"`  // 排序字段
	SortDesc bool   `json:"sort_desc"` // 是否降序
}

// PageResponse 分页响应
type PageResponse struct {
	Total     int64 `json:"total"`      // 总记录数
	Page      int   `json:"page"`       // 当前页码
	PageSize  int   `json:"page_size"`  // 每页大小
	PageCount int   `json:"page_count"` // 总页数
	HasNext   bool  `json:"has_next"`   // 是否有下一页
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query      string    `json:"query"`       // 搜索关键词
	Type       string    `json:"type"`        // 搜索类型（snapshot/task/backup）
	StartDate  time.Time `json:"start_date"`  // 开始日期
	EndDate    time.Time `json:"end_date"`    // 结束日期
	Tags       []string  `json:"tags"`        // 标签过滤
	Tools      []string  `json:"tools"`       // 工具过滤
	Page       PageRequest `json:"page"`      // 分页
}

// SearchResult 搜索结果
type SearchResult struct {
	Type      string         `json:"type"`      // 结果类型
	Count     int            `json:"count"`     // 结果数量
	Results   []SearchItem   `json:"results"`   // 结果列表
	Page      PageResponse   `json:"page"`      // 分页信息
}

// SearchItem 搜索项
type SearchItem struct {
	ID          string    `json:"id"`          // 项目 ID
	Type        string    `json:"type"`        // 类型
	Title       string    `json:"title"`       // 标题
	Description string    `json:"description"` // 描述
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	Relevance   float64   `json:"relevance"`   // 相关度
}

// DatabaseStats 数据库统计信息
type DatabaseStats struct {
	SnapshotCount      int64  `json:"snapshot_count"`
	FileCount          int64  `json:"file_count"`
	TaskCount          int64  `json:"task_count"`
	RemoteConfigCount  int64  `json:"remote_config_count"`
	BackupCount        int64  `json:"backup_count"`
	HistoryCount       int64  `json:"history_count"`
	DatabaseSize       int64  `json:"database_size"`
}
