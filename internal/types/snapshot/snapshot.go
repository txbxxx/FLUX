package snapshot

import (
	"time"

	"ai-sync-manager/internal/models"
)

// SnapshotPackage bundles a snapshot with packaging metadata.
type SnapshotPackage struct {
	Snapshot    *models.Snapshot `json:"snapshot"`     // 主快照
	ProjectPath string           `json:"project_path"` // 项目路径
	CreatedAt   time.Time        `json:"created_at"`   // 创建时间
	Size        int64            `json:"size"`         // 总大小（字节）
	FileCount   int              `json:"file_count"`   // 文件数量
	Checksum    string           `json:"checksum"`     // 校验和
}

// SnapshotInfo holds brief snapshot information for list display.
type SnapshotInfo struct {
	ID          string    `json:"id"`          // 快照 ID
	Name        string    `json:"name"`        // 快照名称
	Description string    `json:"description"` // 快照描述
	CreatedAt   time.Time `json:"created_at"`  // 创建时间
	Tools       []string  `json:"tools"`       // 包含的工具
	CommitHash  string    `json:"commit_hash"` // 提交哈希
	IsRemote    bool      `json:"is_remote"`   // 是否已推送到远端
}

// CreateSnapshotOptions defines parameters for creating a new snapshot.
type CreateSnapshotOptions struct {
	Message     string   `json:"message"`      // 快照描述/提交消息
	Tools       []string `json:"tools"`        // 包含的工具 [codex, claude]
	Name        string   `json:"name"`         // 快照名称（可选）
	Tags        []string `json:"tags"`         // 标签（可选）
	ProjectName string   `json:"project_name"` // 项目名称（必填）
}
