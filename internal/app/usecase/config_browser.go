package usecase

import (
	"context"
	"errors"
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
	Tool string // 工具类型名（codex / claude），必填
	Path string // 相对于配置根目录的路径，空则返回根目录列表
	Edit bool   // 是否进入编辑模式（仅文件有效）
}

// SaveConfigInput 是 SaveConfig 用例的输入参数。
type SaveConfigInput struct {
	Tool    string // 工具类型名（codex / claude），必填
	Path    string // 相对于配置根目录的文件路径，必填
	Content string // 要写入的文件内容
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
func (w *LocalWorkflow) GetConfig(_ context.Context, input GetConfigInput) (*GetConfigResult, error) {
	if w.accessor == nil {
		return nil, &UserError{
			Message:    "无法读取配置",
			Suggestion: "内部初始化异常，请重新启动程序",
			Err:        errors.New("missing config accessor"),
		}
	}

	toolType, err := parseToolType(input.Tool)
	if err != nil {
		return nil, &UserError{
			Message:    "读取配置失败",
			Suggestion: "工具名只支持 codex 或 claude",
			Err:        err,
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

// SaveConfig writes content to a configuration file for the specified tool.
//
// The target path must be a file, not a directory.
// Content is written atomically using a temporary file to prevent data corruption.
func (w *LocalWorkflow) SaveConfig(_ context.Context, input SaveConfigInput) error {
	if w.accessor == nil {
		return &UserError{
			Message:    "无法保存配置",
			Suggestion: "当前工作流未初始化配置访问能力",
			Err:        errors.New("missing config accessor"),
		}
	}

	toolType, err := parseToolType(input.Tool)
	if err != nil {
		return &UserError{
			Message:    "无法保存配置",
			Suggestion: "工具名只支持 codex 或 claude",
			Err:        err,
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
