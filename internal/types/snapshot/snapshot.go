package snapshot

import "time"

// SnapshotPackage bundles a snapshot header with packaging metadata.
type SnapshotPackage struct {
	Snapshot    *SnapshotHeader `json:"snapshot" yaml:"snapshot"`           // 快照头信息
	ProjectPath string          `json:"project_path" yaml:"project_path"` // 项目路径
	CreatedAt   time.Time       `json:"created_at" yaml:"created_at"`     // 创建时间
	Size        int64           `json:"size" yaml:"size"`                 // 总大小（字节）
	FileCount   int             `json:"file_count" yaml:"file_count"`     // 文件数量
	Checksum    string          `json:"checksum" yaml:"checksum"`         // 校验和
}

// SnapshotInfo holds brief snapshot information for list display.
type SnapshotInfo struct {
	ID          uint      `json:"id" yaml:"id"`                     // 快照 ID
	Name        string    `json:"name" yaml:"name"`                 // 快照名称
	Description string    `json:"description" yaml:"description"`   // 快照描述
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`     // 创建时间
	Project     string    `json:"project" yaml:"project"`           // 关联的项目名称
	CommitHash  string    `json:"commit_hash" yaml:"commit_hash"`   // 提交哈希
	IsRemote    bool      `json:"is_remote" yaml:"is_remote"`       // 是否已推送到远端
}

// CreateSnapshotOptions defines parameters for creating a new snapshot.
type CreateSnapshotOptions struct {
	Message     string   `json:"message" yaml:"message"`               // 快照描述/提交消息
	Tools       []string `json:"tools" yaml:"tools"`                   // 包含的工具 [codex, claude]
	Name        string   `json:"name" yaml:"name"`                     // 快照名称（可选）
	Tags        []string `json:"tags" yaml:"tags"`                     // 标签（可选）
	ProjectName string   `json:"project_name" yaml:"project_name"`     // 项目名称（必填）
}
