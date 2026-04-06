package sync

import (
	"time"

	"ai-sync-manager/internal/models"
)

// ConflictPolicy defines how file conflicts are handled.
type ConflictPolicy string

const (
	ConflictPolicyAsk       ConflictPolicy = "ask"       // 询问用户
	ConflictPolicySkip      ConflictPolicy = "skip"      // 跳过
	ConflictPolicyOverwrite ConflictPolicy = "overwrite" // 覆盖
	ConflictPolicyRename    ConflictPolicy = "rename"    // 重命名
	ConflictPolicyMerge     ConflictPolicy = "merge"     // 合并
)

// SyncResult summarizes the outcome of a sync operation.
type SyncResult struct {
	Success    bool          `json:"success"`     // 是否成功
	TaskID     string        `json:"task_id"`     // 任务 ID
	SnapshotID string        `json:"snapshot_id"` // 快照 ID
	Uploaded   int           `json:"uploaded"`    // 上传文件数
	Downloaded int           `json:"downloaded"`  // 下载文件数
	Skipped    int           `json:"skipped"`     // 跳过文件数
	Conflicts  []Conflict    `json:"conflicts"`   // 冲突列表
	Duration   time.Duration `json:"duration"`    // 执行时长
	Message    string        `json:"message"`     // 结果消息
}

// Conflict describes a file conflict between local and remote.
type Conflict struct {
	Path       string             `json:"path"`                 // 文件路径
	LocalHash  string             `json:"local_hash"`           // 本地文件哈希
	RemoteHash string             `json:"remote_hash"`          // 远程文件哈希
	LocalTime  time.Time          `json:"local_time"`           // 本地修改时间
	RemoteTime time.Time          `json:"remote_time"`          // 远程修改时间
	Reason     string             `json:"reason"`               // 冲突原因
	Resolution ConflictResolution `json:"resolution,omitempty"` // 解决方案
}

// ConflictResolution defines how a conflict was resolved.
type ConflictResolution string

const (
	ResolutionKeepLocal   ConflictResolution = "keep_local"   // 保留本地
	ResolutionKeepRemote  ConflictResolution = "keep_remote"  // 保留远程
	ResolutionKeepNewest  ConflictResolution = "keep_newest"  // 保留最新的
	ResolutionManualMerge ConflictResolution = "manual_merge" // 手动合并
)

// RemoteRepository holds information about a remote Git repository.
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

// RemoteSnapshot holds metadata for a snapshot stored on a remote.
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

// SyncScope defines what gets synchronized.
type SyncScope struct {
	Global     bool     `json:"global"`     // 是否同步全局配置
	Projects   []string `json:"projects"`   // 要同步的项目路径
	Categories []string `json:"categories"` // 要同步的文件类别
	Excludes   []string `json:"excludes"`   // 排除的文件/目录
}

// TaskProgress tracks the progress of a sync task.
type TaskProgress struct {
	Percentage int      `json:"percentage"` // 进度百分比 (0-100)
	Current    int      `json:"current"`    // 当前处理数量
	Total      int      `json:"total"`      // 总数量
	Message    string   `json:"message"`    // 进度消息
	Steps      []string `json:"steps"`      // 已完成的步骤
}

// TaskMetadata holds additional metadata for a sync task.
type TaskMetadata struct {
	Tools       []string `json:"tools,omitempty"`        // 涉及的工具
	ProjectPath string   `json:"project_path,omitempty"` // 项目路径
	Scope       string   `json:"scope,omitempty"`        // 任务范围
	RetryCount  int      `json:"retry_count,omitempty"`  // 重试次数
	MaxRetries  int      `json:"max_retries,omitempty"`  // 最大重试次数
}

// SyncHistory records a completed sync operation.
type SyncHistory struct {
	ID          string              `json:"id"`           // 记录 ID
	TaskID      string              `json:"task_id"`      // 任务 ID
	Type        models.SyncTaskType `json:"type"`         // 任务类型
	Status      models.SyncTaskStatus `json:"status"`     // 任务状态
	Direction   models.SyncDirection `json:"direction"`    // 同步方向
	StartedAt   time.Time           `json:"started_at"`   // 开始时间
	CompletedAt *time.Time          `json:"completed_at"` // 完成时间
	Duration    string              `json:"duration"`     // 执行时长
	Success     bool                `json:"success"`      // 是否成功
	Error       string              `json:"error"`        // 错误信息
}

// RepositoryTestResult holds the result of testing a remote repository connection.
type RepositoryTestResult struct {
	Success      bool            `json:"success"`       // 是否成功
	URL          string          `json:"url"`           // 仓库 URL
	IsAccessible bool            `json:"is_accessible"` // 是否可访问
	AuthType     models.AuthType `json:"auth_type"`     // 使用的认证类型
	Branches     []string        `json:"branches"`      // 可用分支
	Error        string          `json:"error"`         // 错误信息
	Latency      int64           `json:"latency"`       // 延迟（毫秒）
}
