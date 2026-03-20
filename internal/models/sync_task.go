package models

import (
	"time"
)

// SyncTask 同步任务
type SyncTask struct {
	ID          string         `json:"id" db:"id"`                       // 任务唯一 ID
	Type        SyncTaskType   `json:"type" db:"type"`                   // 任务类型
	Status      SyncTaskStatus `json:"status" db:"status"`               // 任务状态
	SnapshotID  string         `json:"snapshot_id,omitempty" db:"snapshot_id"` // 关联快照 ID
	Direction   SyncDirection  `json:"direction" db:"direction"`         // 同步方向
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`       // 创建时间
	StartedAt   *time.Time     `json:"started_at,omitempty" db:"started_at"`   // 开始时间
	CompletedAt *time.Time     `json:"completed_at,omitempty" db:"completed_at"` // 完成时间
	Progress    TaskProgress   `json:"progress" db:"progress"`           // 进度信息
	Error       string         `json:"error,omitempty" db:"error"`       // 错误信息
	Metadata    TaskMetadata   `json:"metadata" db:"metadata"`           // 任务元数据
}

// SyncTaskType 任务类型
type SyncTaskType string

const (
	TaskTypePush   SyncTaskType = "push"   // 推送配置到远端
	TaskTypePull   SyncTaskType = "pull"   // 从远端拉取配置
	TaskTypeCreate SyncTaskType = "create" // 创建快照
	TaskTypeApply  SyncTaskType = "apply"  // 应用快照
	TaskTypeBackup SyncTaskType = "backup" // 备份配置
)

// SyncTaskStatus 任务状态
type SyncTaskStatus string

const (
	StatusPending   SyncTaskStatus = "pending"   // 等待执行
	StatusRunning   SyncTaskStatus = "running"   // 执行中
	StatusCompleted SyncTaskStatus = "completed" // 已完成
	StatusFailed    SyncTaskStatus = "failed"    // 失败
	StatusCancelled SyncTaskStatus = "cancelled" // 已取消
)

// SyncDirection 同步方向
type SyncDirection string

const (
	DirectionUpload   SyncDirection = "upload"   // 上传（本地 → 远端）
	DirectionDownload SyncDirection = "download" // 下载（远端 → 本地）
	DirectionBoth     SyncDirection = "both"     // 双向同步
)

// TaskProgress 任务进度
type TaskProgress struct {
	Percentage int     `json:"percentage"` // 进度百分比 (0-100)
	Current    int     `json:"current"`    // 当前处理数量
	Total      int     `json:"total"`      // 总数量
	Message    string  `json:"message"`    // 进度消息
	Steps      []string `json:"steps"`      // 已完成的步骤
}

// TaskMetadata 任务元数据
type TaskMetadata struct {
	Tools       []string `json:"tools,omitempty"`       // 涉及的工具
	ProjectPath string   `json:"project_path,omitempty"` // 项目路径
	Scope       string   `json:"scope,omitempty"`        // 任务范围
	RetryCount  int      `json:"retry_count,omitempty"`  // 重试次数
	MaxRetries  int      `json:"max_retries,omitempty"`  // 最大重试次数
}

// SyncConfig 同步配置
type SyncConfig struct {
	AutoSync      bool          `json:"auto_sync"`       // 是否自动同步
	SyncInterval  time.Duration `json:"sync_interval"`   // 同步间隔
	ConflictPolicy ConflictPolicy `json:"conflict_policy"` // 冲突策略
	Excludes      []string      `json:"excludes"`        // 排除的文件/目录
	Includes      []string      `json:"includes"`        // 包含的文件/目录
}

// ConflictPolicy 冲突策略
type ConflictPolicy string

const (
	ConflictPolicyAsk       ConflictPolicy = "ask"       // 询问用户
	ConflictPolicySkip      ConflictPolicy = "skip"      // 跳过
	ConflictPolicyOverwrite ConflictPolicy = "overwrite" // 覆盖
	ConflictPolicyRename    ConflictPolicy = "rename"    // 重命名
	ConflictPolicyMerge     ConflictPolicy = "merge"     // 合并
)

// SyncResult 同步结果
type SyncResult struct {
	Success      bool          `json:"success"`       // 是否成功
	TaskID       string        `json:"task_id"`       // 任务 ID
	SnapshotID   string        `json:"snapshot_id"`   // 快照 ID
	Uploaded     int           `json:"uploaded"`      // 上传文件数
	Downloaded   int           `json:"downloaded"`    // 下载文件数
	Skipped      int           `json:"skipped"`       // 跳过文件数
	Conflicts    []Conflict    `json:"conflicts"`     // 冲突列表
	Duration     time.Duration `json:"duration"`      // 执行时长
	Message      string        `json:"message"`       // 结果消息
}

// Conflict 文件冲突
type Conflict struct {
	Path         string       `json:"path"`           // 文件路径
	LocalHash    string       `json:"local_hash"`     // 本地文件哈希
	RemoteHash   string       `json:"remote_hash"`    // 远程文件哈希
	LocalTime    time.Time    `json:"local_time"`     // 本地修改时间
	RemoteTime   time.Time    `json:"remote_time"`    // 远程修改时间
	Reason       string       `json:"reason"`         // 冲突原因
	Resolution   ConflictResolution `json:"resolution,omitempty"` // 解决方案
}

// ConflictResolution 冲突解决方案
type ConflictResolution string

const (
	ResolutionKeepLocal    ConflictResolution = "keep_local"     // 保留本地
	ResolutionKeepRemote   ConflictResolution = "keep_remote"    // 保留远程
	ResolutionKeepNewest   ConflictResolution = "keep_newest"    // 保留最新的
	ResolutionManualMerge  ConflictResolution = "manual_merge"   // 手动合并
)

// BackupInfo 备份信息
type BackupInfo struct {
	ID          string    `json:"id"`           // 备份 ID
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	Path        string    `json:"path"`         // 备份路径
	Size        int64     `json:"size"`         // 备份大小
	FileCount   int       `json:"file_count"`   // 文件数量
	SnapshotID  string    `json:"snapshot_id"`  // 关联快照 ID
	Description string    `json:"description"`  // 备份描述
}

// BackupOptions 备份选项
type BackupOptions struct {
	Path        string   `json:"path"`         // 备份路径
	Description string   `json:"description"`  // 备份描述
	IncludeTools []string `json:"include_tools"` // 包含的工具
	Compress    bool     `json:"compress"`     // 是否压缩
}

// BackupResult 备份结果
type BackupResult struct {
	Success     bool   `json:"success"`      // 是否成功
	BackupID    string `json:"backup_id"`    // 备份 ID
	Path        string `json:"path"`         // 备份路径
	Size        int64  `json:"size"`         // 备份大小
	FileCount   int    `json:"file_count"`   // 文件数量
	Duration    string `json:"duration"`     // 执行时长
	Message     string `json:"message"`      // 结果消息
}
