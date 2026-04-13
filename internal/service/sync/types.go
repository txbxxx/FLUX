package sync

import (
	"flux/internal/models"
)

// SyncOptions 同步选项
type SyncOptions struct {
	// 远程仓库配置
	RemoteURL  string             // 仓库 URL
	Auth       *models.AuthConfig // 认证配置
	Branch     string             // 分支名
	RemoteName string             // 远程名称

	// 同步行为
	CreateCommit  bool   // 是否创建 Git 提交
	CommitMessage string // 提交消息（可选）
	ForcePush     bool   // 是否强制推送

	// 快照处理
	IncludeFiles bool // 是否包含文件内容
}

// PushResult 推送结果
type PushResult struct {
	Success      bool   `json:"success"`       // 是否成功
	SnapshotID   string `json:"snapshot_id"`   // 快照 ID
	CommitHash   string `json:"commit_hash"`   // Git 提交哈希
	Branch       string `json:"branch"`        // 分支名
	PushedAt     string `json:"pushed_at"`     // 推送时间
	FilesCount   int    `json:"files_count"`   // 文件数量
	ErrorMessage string `json:"error_message"` // 错误信息
}

// PullResult 拉取结果
type PullResult struct {
	Success      bool           `json:"success"`       // 是否成功
	Commits      []RemoteCommit `json:"commits"`       // 拉取的提交列表
	Branch       string         `json:"branch"`        // 分支名
	PulledAt     string         `json:"pulled_at"`     // 拉取时间
	NewSnapshots int            `json:"new_snapshots"` // 新快照数量
	ErrorMessage string         `json:"error_message"` // 错误信息
}

// RemoteCommit 远程提交信息
type RemoteCommit struct {
	Hash       string `json:"hash"`        // 提交哈希
	Message    string `json:"message"`     // 提交消息
	Author     string `json:"author"`      // 作者
	Date       string `json:"date"`        // 日期
	SnapshotID string `json:"snapshot_id"` // 关联的快照 ID
}

// SyncStatus 同步状态
type SyncStatus struct {
	RemoteURL       string               `json:"remote_url"`       // 远程仓库 URL
	Branch          string               `json:"branch"`           // 当前分支
	LastSync        string               `json:"last_sync"`        // 最后同步时间
	LocalAhead      int                  `json:"local_ahead"`      // 本地领先提交数
	LocalBehind     int                  `json:"local_behind"`     // 本地落后提交数
	RemoteSnapshots []RemoteSnapshotInfo `json:"remote_snapshots"` // 远程快照列表
}

// RemoteSnapshotInfo 远程快照信息
type RemoteSnapshotInfo struct {
	ID          string   `json:"id"`          // 快照 ID
	Name        string   `json:"name"`        // 快照名称
	Description string   `json:"description"` // 快照描述
	Message     string   `json:"message"`     // 提交消息
	CommitHash  string   `json:"commit_hash"` // Git 提交哈希
	Author      string   `json:"author"`      // 作者
	Date        string   `json:"date"`        // 日期
	Project     string   `json:"project"`     // 所属项目
	FileCount   int      `json:"file_count"`  // 文件数量
	Size        int64    `json:"size"`        // 快照大小
}

// SyncResult 同步结果
type SyncResult struct {
	Success      bool        `json:"success"`        // 是否成功
	Action       SyncAction  `json:"action"`         // 同步动作
	PushResult   *PushResult `json:"push,omitempty"` // 推送结果
	PullResult   *PullResult `json:"pull,omitempty"` // 拉取结果
	ErrorMessage string      `json:"error_message"`  // 错误信息
}

// SyncAction 同步动作
type SyncAction string

const (
	SyncActionPush  SyncAction = "push"  // 推送到远程
	SyncActionPull  SyncAction = "pull"  // 从远程拉取
	SyncActionClone SyncAction = "clone" // 克隆远程仓库
)

// SyncConflict 同步冲突
type SyncConflict struct {
	Type       ConflictType `json:"type"`        // 冲突类型
	Path       string       `json:"path"`        // 文件路径
	LocalHash  string       `json:"local_hash"`  // 本地哈希
	RemoteHash string       `json:"remote_hash"` // 远程哈希
	Reason     string       `json:"reason"`      // 冲突原因
}

// ConflictType 冲突类型
type ConflictType string

const (
	ConflictTypeModified ConflictType = "modified" // 文件被修改
	ConflictTypeDeleted  ConflictType = "deleted"  // 文件被删除
	ConflictTypeCreated  ConflictType = "created"  // 文件被创建
	ConflictTypeBinary   ConflictType = "binary"   // 二进制冲突
)
