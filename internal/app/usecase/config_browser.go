package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"ai-sync-manager/internal/service/tool"
)

// ConfigTargetKind 表示配置目标的类型：文件或目录。
type ConfigTargetKind string

const (
	ConfigTargetDirectory ConfigTargetKind = "directory" // 目录类型，可列出子条目
	ConfigTargetFile      ConfigTargetKind = "file"      // 文件类型，可读取/编辑内容
)

// GetConfigInput 是 GetConfig 用例的输入参数。
type GetConfigInput struct {
	Tool     string // 工具类型名（codex / claude）或项目名，必填
	Path     string // 相对于配置根目录的路径，空则返回根目录列表
	Edit     bool   // 是否进入编辑模式（仅文件有效）
	Snapshot string // 快照 ID，非空时浏览快照中的文件而非磁盘文件
}

// SaveConfigInput 是 SaveConfig 用例的输入参数。
type SaveConfigInput struct {
	Tool     string // 工具类型名（codex / claude）或项目名，必填
	Path     string // 相对于配置根目录的文件路径，必填
	Content  string // 要写入的文件内容
	Snapshot string // 快照 ID，非空时修改快照中的文件内容
}

// ConfigEntry 表示配置目录列表中的单个条目。
type ConfigEntry struct {
	Name         string // 文件或目录名（不含路径前缀）
	RelativePath string // 相对于配置根目录的路径
	IsDir        bool   // 是否为目录
}

// GetConfigResult 是 GetConfig 用例的返回值。
// 目录类型时 Entries 有值；文件类型时 Content 和 Editable 有值。
type GetConfigResult struct {
	Tool         string           // 工具类型名（codex / claude）
	RelativePath string           // 相对于配置根目录的路径
	AbsolutePath string           // 磁盘绝对路径
	Kind         ConfigTargetKind // 目标类型：directory 或 file
	Entries      []ConfigEntry    // 目录列表（Kind=directory 时有值）
	Content      string           // 文件内容（Kind=file 时有值）
	Editable     bool             // 是否可编辑（仅文件且路径在允许范围内时为 true）
}

// GetConfig retrieves configuration file or directory content for the specified tool.
//
// If path is empty, returns the root directory configuration entries.
// If path points to a directory, returns directory entries.
// If path points to a file, returns file content.
// When edit mode is enabled, validates that the target is a file.
// When snapshot is specified, reads from snapshot stored files instead of disk.
//
// 工具名解析策略：先尝试精确匹配 codex/claude，失败后按项目名前缀推导（如 claude-global → claude）。
func (w *LocalWorkflow) GetConfig(_ context.Context, input GetConfigInput) (*GetConfigResult, error) {
	// 快照模式：从快照中读取文件
	if strings.TrimSpace(input.Snapshot) != "" {
		return w.getSnapshotConfig(input)
	}

	if w.accessor == nil {
		return nil, &UserError{
			Message:    "无法读取配置",
			Suggestion: "内部初始化异常，请重新启动程序",
			Err:        errors.New("missing config accessor"),
		}
	}

	toolType, err := parseToolType(input.Tool)
	if err != nil {
		// 精确匹配失败，尝试项目名前缀推导。
		toolType = inferToolTypeFromProjectName(input.Tool)
		if toolType == "" {
			return nil, &UserError{
				Message:    "读取配置失败",
				Suggestion: "工具名只支持 codex 或 claude，项目名如 claude-global、codex-global",
				Err:        err,
			}
		}
	}

	target, err := w.accessor.Resolve(toolType, strings.TrimSpace(input.Path))
	if err != nil {
		return nil, &UserError{
			Message:    "读取配置失败",
			Suggestion: "请检查工具名和路径是否正确",
			Err:        err,
		}
	}

	if target.IsDir && input.Edit {
		return nil, &UserError{
			Message:    "目录无法编辑，请指定文件",
			Suggestion: "例如: ai-sync get claude settings.json --edit",
			Err:        errors.New("directory edit unsupported"),
		}
	}

	result := &GetConfigResult{
		Tool:         target.ToolType.String(),
		RelativePath: target.RelativePath,
		AbsolutePath: target.AbsolutePath,
	}

	if target.IsDir {
		entries, err := w.accessor.ListDir(target)
		if err != nil {
			return nil, &UserError{
				Message:    "无法打开配置目录",
				Suggestion: "请检查目录权限或路径后重试",
				Err:        err,
			}
		}

		result.Kind = ConfigTargetDirectory
		result.Entries = make([]ConfigEntry, 0, len(entries))
		for _, entry := range entries {
			result.Entries = append(result.Entries, ConfigEntry{
				Name:         entry.Name,
				RelativePath: entry.RelativePath,
				IsDir:        entry.IsDir,
			})
		}
		return result, nil
	}

	content, err := w.accessor.ReadFile(target)
	if err != nil {
		return nil, &UserError{
			Message:    "无法读取文件",
			Suggestion: "请检查文件内容和访问权限后重试",
			Err:        err,
		}
	}

	result.Kind = ConfigTargetFile
	result.Content = content
	result.Editable = true
	return result, nil
}

func parseToolType(value string) (tool.ToolType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(tool.ToolTypeCodex):
		return tool.ToolTypeCodex, nil
	case string(tool.ToolTypeClaude):
		return tool.ToolTypeClaude, nil
	default:
		return "", errors.New("unsupported tool")
	}
}

// inferToolTypeFromProjectName 从项目名推导工具类型。
// 支持前缀匹配：claude-global → claude，codex-global → codex。
func inferToolTypeFromProjectName(name string) tool.ToolType {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range []string{"codex", "claude"} {
		if strings.HasPrefix(lower, prefix) {
			return tool.ToolType(prefix)
		}
	}
	return ""
}

// SaveConfig writes content to a configuration file for the specified tool.
//
// The target path must be a file, not a directory.
// Content is written atomically using a temporary file to prevent data corruption.
// When snapshot is specified, updates the file content within the snapshot record.
func (w *LocalWorkflow) SaveConfig(_ context.Context, input SaveConfigInput) error {
	// 快照模式：修改快照中的文件内容
	if strings.TrimSpace(input.Snapshot) != "" {
		return w.saveSnapshotConfig(input)
	}

	if w.accessor == nil {
		return &UserError{
			Message:    "无法保存配置",
			Suggestion: "当前工作流未初始化配置访问能力",
			Err:        errors.New("missing config accessor"),
		}
	}

	toolType, err := parseToolType(input.Tool)
	if err != nil {
		// 精确匹配失败，尝试项目名前缀推导。
		toolType = inferToolTypeFromProjectName(input.Tool)
		if toolType == "" {
			return &UserError{
				Message:    "无法保存配置",
				Suggestion: "工具名只支持 codex 或 claude",
				Err:        err,
			}
		}
	}

	target, err := w.accessor.Resolve(toolType, strings.TrimSpace(input.Path))
	if err != nil {
		return &UserError{
			Message:    "无法保存配置",
			Suggestion: "请检查工具名和路径是否正确",
			Err:        err,
		}
	}
	if target.IsDir {
		return &UserError{
			Message:    "无法保存：目标是目录而非文件",
			Suggestion: "请指定具体文件路径后重试",
			Err:        errors.New("directory save unsupported"),
		}
	}

	if err := w.accessor.WriteFile(target, input.Content); err != nil {
		return &UserError{
			Message:    "无法保存配置",
			Suggestion: "请检查文件是否被占用、是否有写入权限",
			Err:        err,
		}
	}

	return nil
}

// getSnapshotConfig 从快照中读取文件内容或列出文件列表。
func (w *LocalWorkflow) getSnapshotConfig(input GetConfigInput) (*GetConfigResult, error) {
	snapshotID := strings.TrimSpace(input.Snapshot)

	// 支持通过名称查找快照：如果不是 UUID 格式，按名称查找对应的 ID。
	if !isUUIDFormat(snapshotID) {
		snapshots, listErr := w.snapshots.ListSnapshots(0, 0)
		if listErr != nil {
			return nil, &UserError{
				Message:    "读取快照失败",
				Suggestion: "请检查快照名称或 ID 是否正确，使用 snapshot list 查看所有快照",
				Err:        listErr,
			}
		}
		found := false
		for _, snap := range snapshots {
			if snap.Name == snapshotID {
				snapshotID = snap.ID
				found = true
				break
			}
		}
		if !found {
			return nil, &UserError{
				Message:    "未找到名称为 \"" + snapshotID + "\" 的快照",
				Suggestion: "使用 snapshot list 查看所有快照的名称和 ID",
			}
		}
	}

	snapshot, err := w.snapshots.GetSnapshot(snapshotID)
	if err != nil {
		return nil, &UserError{
			Message:    "读取快照失败",
			Suggestion: "请检查快照 ID 是否正确，使用 snapshot list 查看所有快照",
			Err:        err,
		}
	}

	relativePath := strings.TrimSpace(input.Path)

	// 无路径时，列出快照中所有文件
	if relativePath == "" {
		result := &GetConfigResult{
			Tool:         input.Tool,
			RelativePath: "",
			AbsolutePath: "[快照] " + snapshot.Name,
			Kind:         ConfigTargetDirectory,
		}
		for _, file := range snapshot.Files {
			result.Entries = append(result.Entries, ConfigEntry{
				Name:         filepath.Base(file.Path),
				RelativePath: file.Path,
				IsDir:        false,
			})
		}
		return result, nil
	}

	// 查找匹配的文件
	for _, file := range snapshot.Files {
		if file.Path == relativePath {
			result := &GetConfigResult{
				Tool:         input.Tool,
				RelativePath: file.Path,
				AbsolutePath: "[快照] " + snapshot.Name + "/" + file.Path,
				Kind:         ConfigTargetFile,
				Content:      string(file.Content),
				Editable:     true,
			}
			return result, nil
		}
	}

	return nil, &UserError{
		Message:    "快照中未找到文件: " + relativePath,
		Suggestion: "省略路径可查看快照中的所有文件列表",
	}
}

// saveSnapshotConfig 更新快照中指定文件的内容。
func (w *LocalWorkflow) saveSnapshotConfig(input SaveConfigInput) error {
	snapshotID := strings.TrimSpace(input.Snapshot)
	relativePath := strings.TrimSpace(input.Path)

	if relativePath == "" {
		return &UserError{
			Message:    "保存快照文件失败：路径不能为空",
			Suggestion: "请指定要修改的文件路径",
		}
	}

	// 支持通过名称查找快照：如果不是 UUID 格式，按名称查找对应的 ID。
	if !isUUIDFormat(snapshotID) {
		snapshots, listErr := w.snapshots.ListSnapshots(0, 0)
		if listErr != nil {
			return &UserError{
				Message:    "读取快照失败",
				Suggestion: "请检查快照名称或 ID 是否正确，使用 snapshot list 查看所有快照",
				Err:        listErr,
			}
		}
		found := false
		for _, snap := range snapshots {
			if snap.Name == snapshotID {
				snapshotID = snap.ID
				found = true
				break
			}
		}
		if !found {
			return &UserError{
				Message:    "未找到名称为 \"" + snapshotID + "\" 的快照",
				Suggestion: "使用 snapshot list 查看所有快照的名称和 ID",
			}
		}
	}

	snapshot, err := w.snapshots.GetSnapshot(snapshotID)
	if err != nil {
		return &UserError{
			Message:    "保存快照文件失败",
			Suggestion: "请检查快照 ID 是否正确",
			Err:        err,
		}
	}

	// 查找并更新文件内容
	found := false
	for i, file := range snapshot.Files {
		if file.Path == relativePath {
			snapshot.Files[i].Content = []byte(input.Content)
			found = true
			break
		}
	}

	if !found {
		return &UserError{
			Message:    "快照中未找到文件: " + relativePath,
			Suggestion: "省略路径可查看快照中的所有文件列表",
		}
	}

	// 通过 DAO 更新整个快照记录
	if err := w.snapshots.UpdateSnapshot(snapshot); err != nil {
		return &UserError{
			Message:    "保存快照文件失败",
			Suggestion: "请检查数据库是否可访问",
			Err:        err,
		}
	}

	return nil
}
