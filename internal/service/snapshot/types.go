package snapshot

import (
	"ai-sync-manager/internal/models"
)

// CollectOptions 文件收集选项
type CollectOptions struct {
	Tools      []string                // 要收集的工具
	Scope      models.SnapshotScope    // 快照范围
	ProjectPath string                 // 项目路径
	Categories  []models.FileCategory  // 要包含的文件类别
	Excludes    []string               // 排除的文件模式
}

// CollectResult 文件收集结果
type CollectResult struct {
	Files    []models.SnapshotFile   // 收集的文件列表
	TotalSize int64                  // 总大小
	Errors   []CollectError          // 收集过程中的错误
}

// CollectError 收集错误
type CollectError struct {
	Path    string // 文件路径
	Message string // 错误消息
}

// ApplyContext 应用上下文
type ApplyContext struct {
	Options      models.ApplyOptions  // 应用选项
	BackupPath   string               // 备份路径
	ExistingFiles []string            // 已存在的文件列表
}

// ApplyContextItem 单个文件的应用上下文
type ApplyContextItem struct {
	File         models.SnapshotFile // 要应用的文件
	Exists       bool                // 文件是否已存在
	NeedsBackup  bool                // 是否需要备份
	ShouldApply  bool                // 是否应该应用
	Reason       string              // 跳过原因（如果不应用）
}

// ComparisonContext 比较上下文
type ComparisonContext struct {
	Source      *models.Snapshot     // 源快照
	Target      *models.Snapshot     // 目标快照（可选，为 nil 时与文件系统比较）
	ProjectPath string               // 项目路径
}

// FileComparison 文件比较结果
type FileComparison struct {
	Path       string            // 文件路径
	Status     FileChangeStatus  // 变更状态
	SourceHash string            // 源文件哈希
	TargetHash string            // 目标文件哈希
}

// FileChangeStatus 文件变更状态
type FileChangeStatus string

const (
	FileStatusCreated FileChangeStatus = "created"   // 新建
	FileStatusUpdated FileChangeStatus = "updated"   // 更新
	FileStatusDeleted FileChangeStatus = "deleted"   // 删除
	FileStatusSame    FileChangeStatus = "same"      // 无变化
	FileStatusConflict FileChangeStatus = "conflict" // 冲突
)
