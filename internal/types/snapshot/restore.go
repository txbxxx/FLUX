package snapshot

// RestoreResult holds the complete result of a snapshot restore operation.
// It is returned by the UseCase layer and consumed by CLI for rendering.
type RestoreResult struct {
	SnapshotID    string        `json:"snapshot_id"`    // 快照 ID
	SnapshotName  string        `json:"snapshot_name"`  // 快照名称
	AppliedFiles  []AppliedFile `json:"applied_files"`  // 已恢复的文件
	SkippedFiles  []SkippedFile `json:"skipped_files"`  // 跳过的文件
	Errors        []ApplyError  `json:"errors"`         // 恢复失败的文件
	BackupPath    string        `json:"backup_path"`    // 备份目录路径
	TotalFiles    int           `json:"total_files"`    // 总文件数
	AppliedCount  int           `json:"applied_count"`  // 已恢复数
	SkippedCount  int           `json:"skipped_count"`  // 跳过数
	ErrorCount    int           `json:"error_count"`    // 失败数
	DryRun        bool          `json:"dry_run"`        // 是否为预览模式
}
