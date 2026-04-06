package common

import "time"

// ErrorDetail holds structured error information.
type ErrorDetail struct {
	Code    string `json:"code"`    // 错误码
	Message string `json:"message"` // 错误消息
	Details string `json:"details"` // 详细信息
}

// Response is the unified API response wrapper.
type Response struct {
	Success bool         `json:"success"` // 是否成功
	Data    any          `json:"data"`    // 响应数据
	Error   *ErrorDetail `json:"error"`   // 错误信息
}

// PageRequest holds pagination parameters.
type PageRequest struct {
	Page     int    `json:"page"`      // 页码（从 1 开始）
	PageSize int    `json:"page_size"` // 每页大小
	SortBy   string `json:"sort_by"`   // 排序字段
	SortDesc bool   `json:"sort_desc"` // 是否降序
}

// PageResponse holds pagination metadata.
type PageResponse struct {
	Total     int64 `json:"total"`      // 总记录数
	Page      int   `json:"page"`       // 当前页码
	PageSize  int   `json:"page_size"`  // 每页大小
	PageCount int   `json:"page_count"` // 总页数
	HasNext   bool  `json:"has_next"`   // 是否有下一页
}

// SearchRequest holds search parameters.
type SearchRequest struct {
	Query     string      `json:"query"`      // 搜索关键词
	Type      string      `json:"type"`       // 搜索类型（snapshot/task/backup）
	StartDate time.Time   `json:"start_date"` // 开始日期
	EndDate   time.Time   `json:"end_date"`   // 结束日期
	Tags      []string    `json:"tags"`       // 标签过滤
	Tools     []string    `json:"tools"`      // 工具过滤
	Page      PageRequest `json:"page"`       // 分页
}

// SearchResult holds search output.
type SearchResult struct {
	Type    string       `json:"type"`    // 结果类型
	Count   int          `json:"count"`   // 结果数量
	Results []SearchItem `json:"results"` // 结果列表
	Page    PageResponse `json:"page"`    // 分页信息
}

// SearchItem is a single search result entry.
type SearchItem struct {
	ID          string    `json:"id"`          // 项目 ID
	Type        string    `json:"type"`        // 类型
	Title       string    `json:"title"`       // 标题
	Description string    `json:"description"` // 描述
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	Relevance   float64   `json:"relevance"`   // 相关度
}

// DatabaseStats holds database-level statistics.
type DatabaseStats struct {
	SnapshotCount     int64 `json:"snapshot_count"`
	FileCount         int64 `json:"file_count"`
	TaskCount         int64 `json:"task_count"`
	RemoteConfigCount int64 `json:"remote_config_count"`
	BackupCount       int64 `json:"backup_count"`
	HistoryCount      int64 `json:"history_count"`
	DatabaseSize      int64 `json:"database_size"`
}

// SensitiveType classifies sensitive data.
type SensitiveType string

const (
	SensitiveTypeToken    SensitiveType = "token"    // Token
	SensitiveTypePassword SensitiveType = "password" // 密码
	SensitiveTypeSSHKey   SensitiveType = "ssh_key"  // SSH 密钥
	SensitiveTypeAPIKey   SensitiveType = "api_key"  // API 密钥
	SensitiveTypeSecret   SensitiveType = "secret"   // 其他密钥
)

// SensitiveData holds encrypted sensitive content.
type SensitiveData struct {
	Type     SensitiveType `json:"type"`    // 数据类型
	Content  string        `json:"content"` // 加密后的内容
	Original string        `json:"-"`       // 原始内容（不序列化）
}

// EncryptionResult holds the outcome of an encryption operation.
type EncryptionResult struct {
	Success   bool   `json:"success"`   // 是否成功
	Data      string `json:"data"`      // 加密后的数据
	Algorithm string `json:"algorithm"` // 使用的算法
	Error     string `json:"error"`     // 错误信息
}

// DecryptionResult holds the outcome of a decryption operation.
type DecryptionResult struct {
	Success bool   `json:"success"` // 是否成功
	Data    string `json:"data"`    // 解密后的数据
	Error   string `json:"error"`   // 错误信息
}

// ConfigValidationResult holds the outcome of a configuration validation.
type ConfigValidationResult struct {
	Valid    bool     `json:"valid"`    // 是否有效
	Errors   []string `json:"errors"`   // 错误列表
	Warnings []string `json:"warnings"` // 警告列表
}

// LogLevel defines log severity levels.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)
