package snapshot

// ApplyOptions controls how a snapshot is applied to the local filesystem.
type ApplyOptions struct {
	CreateBackup bool   `json:"create_backup"` // 是否创建备份
	BackupPath   string `json:"backup_path"`   // 备份路径
	Force        bool   `json:"force"`         // 是否强制覆盖
	DryRun       bool   `json:"dry_run"`       // 是否仅预览
}

// ApplyResult summarizes the outcome of applying a snapshot.
type ApplyResult struct {
	Success      bool          `json:"success"`       // 是否成功
	AppliedFiles []AppliedFile `json:"applied_files"` // 应用的文件列表
	SkippedFiles []SkippedFile `json:"skipped_files"` // 跳过的文件列表
	Errors       []ApplyError  `json:"errors"`        // 错误列表
	BackupPath   string        `json:"backup_path"`   // 备份路径
	Summary      ChangeSummary `json:"summary"`       // 变更摘要
}

// AppliedFile records a single file that was applied.
type AppliedFile struct {
	Path         string `json:"path"`          // 文件路径
	OriginalPath string `json:"original_path"` // 原始路径
	Action       string `json:"action"`        // 操作类型（created/updated/replaced）
}

// SkippedFile records a single file that was skipped during apply.
type SkippedFile struct {
	Path   string `json:"path"`   // 文件路径
	Reason string `json:"reason"` // 跳过原因
}

// ApplyError records an error encountered while applying a file.
type ApplyError struct {
	Path    string `json:"path"`    // 文件路径
	Message string `json:"message"` // 错误消息
}

// ChangeSummary aggregates file change statistics.
type ChangeSummary struct {
	TotalFiles      int            `json:"total_files"`       // 总文件数
	Created         int            `json:"created"`           // 新建文件数
	Updated         int            `json:"updated"`           // 更新文件数
	Deleted         int            `json:"deleted"`           // 删除文件数
	Skipped         int            `json:"skipped"`           // 跳过文件数
	FilesByTool     map[string]int `json:"files_by_tool"`     // 按工具分组的文件数
	FilesByCategory map[string]int `json:"files_by_category"` // 按类别分组的文件数
}

// BackupInfo holds metadata for a configuration backup.
type BackupInfo struct {
	ID          uint   `json:"id"`          // 备份 ID
	CreatedAt   string `json:"created_at"`  // 创建时间
	Path        string `json:"path"`        // 备份路径
	Size        int64  `json:"size"`        // 备份大小
	FileCount   int    `json:"file_count"`  // 文件数量
	SnapshotID  *uint  `json:"snapshot_id"` // 关联快照 ID
	Description string `json:"description"` // 备份描述
}

// BackupOptions configures how a backup is created.
type BackupOptions struct {
	Path         string   `json:"path"`          // 备份路径
	Description  string   `json:"description"`   // 备份描述
	IncludeTools []string `json:"include_tools"` // 包含的工具
	Compress     bool     `json:"compress"`      // 是否压缩
}

// BackupResult summarizes the outcome of a backup operation.
type BackupResult struct {
	Success   bool   `json:"success"`    // 是否成功
	BackupID  uint   `json:"backup_id"`  // 备份 ID
	Path      string `json:"path"`       // 备份路径
	Size      int64  `json:"size"`       // 备份大小
	FileCount int    `json:"file_count"` // 文件数量
	Duration  string `json:"duration"`   // 执行时长
	Message   string `json:"message"`    // 结果消息
}
