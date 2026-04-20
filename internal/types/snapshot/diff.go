package snapshot

// DiffFileChange represents a single file's diff result.
type DiffFileChange struct {
	Path     string     `json:"path" yaml:"path"`                 // 文件相对路径
	Status   FileStatus `json:"status" yaml:"status"`             // added / modified / deleted / unchanged
	IsBinary bool       `json:"is_binary" yaml:"is_binary"`       // 是否二进制文件
	OldSize  int64      `json:"old_size" yaml:"old_size"`         // 变更前大小（字节）
	NewSize  int64      `json:"new_size" yaml:"new_size"`         // 变更后大小（字节）
	AddLines int        `json:"add_lines" yaml:"add_lines"`       // 新增行数（二进制文件为 0）
	DelLines int        `json:"del_lines" yaml:"del_lines"`       // 删除行数（二进制文件为 0）
	ToolType string     `json:"tool_type" yaml:"tool_type"`       // 所属工具类型
	Hunks    []DiffHunk `json:"hunks" yaml:"hunks"`               // 差异块（仅 -v 模式填充）
}

// FileStatus represents the change status of a file.
type FileStatus string

const (
	FileAdded     FileStatus = "added"
	FileModified  FileStatus = "modified"
	FileDeleted   FileStatus = "deleted"
	FileUnchanged FileStatus = "unchanged"
)

// DiffHunk represents a contiguous block of changes.
type DiffHunk struct {
	OldStart int        `json:"old_start" yaml:"old_start"` // 旧文件起始行号
	OldCount int        `json:"old_count" yaml:"old_count"` // 旧文件行数
	NewStart int        `json:"new_start" yaml:"new_start"` // 新文件起始行号
	NewCount int        `json:"new_count" yaml:"new_count"` // 新文件行数
	Lines    []DiffLine `json:"lines" yaml:"lines"`         // 差异行（含上下文）
}

// DiffLine represents a single line in a diff hunk.
type DiffLine struct {
	Type      DiffLineType `json:"type" yaml:"type"`           // 行类型
	Content   string       `json:"content" yaml:"content"`     // 行内容
	OldLineNo int          `json:"old_line_no" yaml:"old_line_no"` // 旧文件行号（上下文和删除行有效）
	NewLineNo int          `json:"new_line_no" yaml:"new_line_no"` // 新文件行号（上下文和新增行有效）
}

// DiffLineType represents the type of a diff line.
type DiffLineType int

const (
	LineContext DiffLineType = iota // 上下文行
	LineAdded                       // 新增行
	LineDeleted                     // 删除行
)

// DiffResult is the complete result of a diff operation.
type DiffResult struct {
	SourceName    string           `json:"source_name" yaml:"source_name"`       // 源快照名称
	TargetName    string           `json:"target_name" yaml:"target_name"`       // 目标快照名称（或 "当前文件系统"）
	Files         []DiffFileChange `json:"files" yaml:"files"`                   // 变更文件列表
	Stats         DiffStats        `json:"stats" yaml:"stats"`                   // 变更统计
	HasDiff       bool             `json:"has_diff" yaml:"has_diff"`             // 是否存在差异
	Partial       bool             `json:"partial" yaml:"partial"`               // 降级模式：无法检测新增文件（rescanFilesystem 失败时为 true）
	PartialReason string           `json:"partial_reason" yaml:"partial_reason"` // 降级原因（供 CLI 展示）
}

// DiffStats holds aggregate statistics for a diff.
type DiffStats struct {
	TotalFiles    int `json:"total_files" yaml:"total_files"`     // 变更文件总数
	AddedFiles    int `json:"added_files" yaml:"added_files"`     // 新增文件数
	ModifiedFiles int `json:"modified_files" yaml:"modified_files"` // 修改文件数
	DeletedFiles  int `json:"deleted_files" yaml:"deleted_files"`   // 删除文件数
	AddLines      int `json:"add_lines" yaml:"add_lines"`           // 总新增行数
	DelLines      int `json:"del_lines" yaml:"del_lines"`           // 总删除行数
}
