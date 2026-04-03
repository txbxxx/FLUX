package usecase

import (
	"context"
	"errors"
	"strings"

	"ai-sync-manager/internal/service/tool"
)

type ConfigTargetKind string

const (
	ConfigTargetDirectory ConfigTargetKind = "directory"
	ConfigTargetFile      ConfigTargetKind = "file"
)

type GetConfigInput struct {
	Tool string
	Path string
	Edit bool
}

type SaveConfigInput struct {
	Tool    string
	Path    string
	Content string
}

type ConfigEntry struct {
	Name         string
	RelativePath string
	IsDir        bool
}

type GetConfigResult struct {
	Tool         string
	RelativePath string
	AbsolutePath string
	Kind         ConfigTargetKind
	Entries      []ConfigEntry
	Content      string
	Editable     bool
}

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
