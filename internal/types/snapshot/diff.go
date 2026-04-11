package snapshot

// DiffFileChange represents a single file's diff result.
type DiffFileChange struct {
	Path     string     // 文件相对路径
	Status   FileStatus // added / modified / deleted / unchanged
	IsBinary bool       // 是否二进制文件
	OldSize  int64      // 变更前大小（字节）
	NewSize  int64      // 变更后大小（字节）
	AddLines int        // 新增行数（二进制文件为 0）
	DelLines int        // 删除行数（二进制文件为 0）
	ToolType string     // 所属工具类型
	Hunks    []DiffHunk // 差异块（仅 -v 模式填充）
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
	OldStart int        // 旧文件起始行号
	OldCount int        // 旧文件行数
	NewStart int        // 新文件起始行号
	NewCount int        // 新文件行数
	Lines    []DiffLine // 差异行（含上下文）
}

// DiffLine represents a single line in a diff hunk.
type DiffLine struct {
	Type      DiffLineType // 行类型
	Content   string       // 行内容
	OldLineNo int          // 旧文件行号（上下文和删除行有效）
	NewLineNo int          // 新文件行号（上下文和新增行有效）
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
	SourceName string           // 源快照名称
	TargetName string           // 目标快照名称（或 "当前文件系统"）
	Files      []DiffFileChange // 变更文件列表
	Stats      DiffStats        // 变更统计
	HasDiff    bool             // 是否存在差异
}

// DiffStats holds aggregate statistics for a diff.
type DiffStats struct {
	TotalFiles    int // 变更文件总数
	AddedFiles    int // 新增文件数
	ModifiedFiles int // 修改文件数
	DeletedFiles  int // 删除文件数
	AddLines      int // 总新增行数
	DelLines      int // 总删除行数
}
