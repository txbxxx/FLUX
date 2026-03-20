package models

import (
	"time"
)

// Snapshot 配置快照
type Snapshot struct {
	ID          string                 `json:"id" db:"id"`                                 // 快照唯一 ID
	Name        string                 `json:"name" db:"name"`                           // 快照名称
	Description string                 `json:"description" db:"description"`             // 快照描述
	Message     string                 `json:"message" db:"message"`                     // 提交消息
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`               // 创建时间
	Tools       []string               `json:"tools" db:"tools"`                         // 包含的工具列表 [codex, claude]
	Metadata    SnapshotMetadata       `json:"metadata" db:"metadata"`                   // 快照元数据
	Files       []SnapshotFile         `json:"files" db:"files"`                         // 包含的文件列表
	CommitHash  string                 `json:"commit_hash,omitempty" db:"commit_hash"`  // Git 提交哈希（如果已推送）
	Tags        []string               `json:"tags,omitempty" db:"tags"`                 // 标签
}

// SnapshotMetadata 快照元数据
type SnapshotMetadata struct {
	OSVersion   string            `json:"os_version,omitempty"`   // 操作系统版本
	AppVersion  string            `json:"app_version,omitempty"`  // 应用版本
	ProjectPath string            `json:"project_path,omitempty"` // 项目路径（如果是项目快照）
	Scope       SnapshotScope     `json:"scope"`                   // 快照范围
	Extra       map[string]string `json:"extra,omitempty"`         // 额外信息
}

// SnapshotScope 快照范围
type SnapshotScope string

const (
	ScopeGlobal  SnapshotScope = "global"  // 全局配置
	ScopeProject SnapshotScope = "project" // 项目配置
	ScopeBoth    SnapshotScope = "both"    // 全局+项目
)

// SnapshotFile 快照中的文件
type SnapshotFile struct {
	Path         string       `json:"path"`                    // 文件相对路径
	OriginalPath string       `json:"original_path"`            // 原始路径
	Size         int64        `json:"size"`                    // 文件大小
	Hash         string       `json:"hash,omitempty"`          // 文件哈希
	ModifiedAt   time.Time    `json:"modified_at"`             // 修改时间
	Content      []byte       `json:"-" db:"content"`           // 文件内容（不序列化到 JSON）
	ToolType     string       `json:"tool_type"`               // 所属工具
	Category     FileCategory `json:"category"`                // 文件类别
	IsBinary     bool         `json:"is_binary"`               // 是否为二进制文件
}

// FileCategory 文件类别
type FileCategory string

const (
	CategoryConfig     FileCategory = "config"     // 配置文件
	CategorySkills     FileCategory = "skills"     // 技能文件
	CategoryCommands   FileCategory = "commands"   // 命令文件
	CategoryPlugins    FileCategory = "plugins"    // 插件文件
	CategoryMCP        FileCategory = "mcp"        // MCP 配置
	CategoryAgents     FileCategory = "agents"     // Agent 文件
	CategoryRules      FileCategory = "rules"      // 规则文件
	CategoryDocs       FileCategory = "docs"       // 文档文件
	CategoryOutput     FileCategory = "output"     // 输出样式
	CategoryOther      FileCategory = "other"      // 其他文件
)

// SnapshotPackage 快照包（包含多个快照）
type SnapshotPackage struct {
	Snapshot    *Snapshot    `json:"snapshot"`     // 主快照
	ProjectPath string       `json:"project_path"` // 项目路径
	CreatedAt   time.Time    `json:"created_at"`  // 创建时间
	Size        int64        `json:"size"`        // 总大小（字节）
	FileCount   int          `json:"file_count"`  // 文件数量
	Checksum    string       `json:"checksum"`    // 校验和
}

// SnapshotInfo 快照简要信息
type SnapshotInfo struct {
	ID          string    `json:"id"`           // 快照 ID
	Name        string    `json:"name"`         // 快照名称
	Description string    `json:"description"`  // 快照描述
	CreatedAt   time.Time `json:"created_at"`   // 创建时间
	Tools       []string  `json:"tools"`        // 包含的工具
	CommitHash  string    `json:"commit_hash"`  // 提交哈希
	IsRemote    bool      `json:"is_remote"`    // 是否已推送到远端
}

// ApplyOptions 应用快照选项
type ApplyOptions struct {
	CreateBackup bool   `json:"create_backup"` // 是否创建备份
	BackupPath    string `json:"backup_path"`   // 备份路径
	Force         bool   `json:"force"`         // 是否强制覆盖
	DryRun        bool   `json:"dry_run"`       // 是否仅预览
}

// ApplyResult 应用结果
type ApplyResult struct {
	Success      bool             `json:"success"`       // 是否成功
	AppliedFiles []AppliedFile    `json:"applied_files"` // 应用的文件列表
	SkippedFiles []SkippedFile    `json:"skipped_files"` // 跳过的文件列表
	Errors       []ApplyError     `json:"errors"`        // 错误列表
	BackupPath   string           `json:"backup_path"`   // 备份路径
	Summary      ChangeSummary    `json:"summary"`       // 变更摘要
}

// AppliedFile 已应用的文件
type AppliedFile struct {
	Path         string `json:"path"`          // 文件路径
	OriginalPath string `json:"original_path"` // 原始路径
	Action       string `json:"action"`        // 操作类型（created/updated/replaced）
}

// SkippedFile 跳过的文件
type SkippedFile struct {
	Path    string `json:"path"`    // 文件路径
	Reason  string `json:"reason"`  // 跳过原因
}

// ApplyError 应用错误
type ApplyError struct {
	Path    string `json:"path"`    // 文件路径
	Message string `json:"message"` // 错误消息
}

// ChangeSummary 变更摘要
type ChangeSummary struct {
	TotalFiles    int            `json:"total_files"`    // 总文件数
	Created       int            `json:"created"`        // 新建文件数
	Updated       int            `json:"updated"`        // 更新文件数
	Deleted       int            `json:"deleted"`        // 删除文件数
	Skipped       int            `json:"skipped"`        // 跳过文件数
	FilesByTool   map[string]int `json:"files_by_tool"`  // 按工具分组的文件数
	FilesByCategory map[string]int `json:"files_by_category"` // 按类别分组的文件数
}

// CreateSnapshotOptions 创建快照选项
type CreateSnapshotOptions struct {
	Message    string   `json:"message"`     // 快照描述/提交消息
	Tools      []string `json:"tools"`       // 包含的工具 [codex, claude]
	Name       string   `json:"name"`        // 快照名称（可选）
	Tags       []string `json:"tags"`        // 标签（可选）
	ProjectPath string  `json:"project_path,omitempty"` // 项目路径（可选）
	Scope      SnapshotScope `json:"scope"` // 快照范围
}
