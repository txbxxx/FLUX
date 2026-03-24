package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/internal/service/tool"
)

const DefaultListLimit = 20

type Detector interface {
	DetectWithOptions(ctx context.Context, opts *tool.ScanOptions) (*tool.ToolDetectionResult, error)
}

type SnapshotManager interface {
	CreateSnapshot(options models.CreateSnapshotOptions) (*models.SnapshotPackage, error)
	ListSnapshots(limit, offset int) ([]*models.Snapshot, error)
	CountSnapshots() (int, error)
}

type ConfigAccessor interface {
	Resolve(toolType tool.ToolType, relativePath string) (*tool.ConfigTarget, error)
	ListDir(target *tool.ConfigTarget) ([]tool.ConfigEntry, error)
	ReadFile(target *tool.ConfigTarget) (string, error)
	WriteFile(target *tool.ConfigTarget, content string) error
}

type Workflow interface {
	Scan(ctx context.Context) (*ScanResult, error)
	CreateSnapshot(ctx context.Context, input CreateSnapshotInput) (*SnapshotSummary, error)
	ListSnapshots(ctx context.Context, input ListSnapshotsInput) (*ListSnapshotsResult, error)
	GetConfig(ctx context.Context, input GetConfigInput) (*GetConfigResult, error)
	SaveConfig(ctx context.Context, input SaveConfigInput) error
}

type LocalWorkflow struct {
	detector  Detector
	snapshots SnapshotManager
	accessor  ConfigAccessor
}

type ToolSummary struct {
	Tool         string
	Status       string
	Path         string
	ConfigCount  int
	ProjectCount int
	ResultText   string
	Reason       string
	Items        []ToolConfigItem
}

type ToolConfigItem struct {
	Group        string
	Label        string
	RelativePath string
}

type ScanResult struct {
	Tools []ToolSummary
}

type CreateSnapshotInput struct {
	Tools       []string
	Message     string
	Name        string
	Scope       models.SnapshotScope
	ProjectPath string
}

type SnapshotSummary struct {
	ID        string
	Name      string
	Message   string
	CreatedAt time.Time
	Tools     []string
	FileCount int
	Size      int64
}

type ListSnapshotsInput struct {
	Limit  int
	Offset int
}

type ListSnapshotsResult struct {
	Total int
	Items []SnapshotSummary
}

type UserError struct {
	Message    string
	Suggestion string
	Err        error
}

func (e *UserError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Message
	}
	return e.Message + ": " + e.Err.Error()
}

func (e *UserError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewLocalWorkflow(detector Detector, snapshots SnapshotManager, accessors ...ConfigAccessor) *LocalWorkflow {
	var accessor ConfigAccessor
	if len(accessors) > 0 {
		accessor = accessors[0]
	}

	return &LocalWorkflow{
		detector:  detector,
		snapshots: snapshots,
		accessor:  accessor,
	}
}

func (w *LocalWorkflow) Scan(ctx context.Context) (*ScanResult, error) {
	result, err := w.detector.DetectWithOptions(ctx, &tool.ScanOptions{
		ScanGlobal:   true,
		ScanProjects: false,
		IncludeFiles: true,
		MaxDepth:     1,
	})
	if err != nil {
		return nil, &UserError{
			Message:    "扫描工具失败",
			Suggestion: "请检查本地配置目录权限后重试",
			Err:        err,
		}
	}

	items := []ToolSummary{
		buildToolSummary(result.Codex, tool.ToolTypeCodex),
		buildToolSummary(result.Claude, tool.ToolTypeClaude),
	}

	return &ScanResult{Tools: items}, nil
}

func (w *LocalWorkflow) CreateSnapshot(_ context.Context, input CreateSnapshotInput) (*SnapshotSummary, error) {
	if len(input.Tools) == 0 {
		return nil, &UserError{
			Message:    "创建快照失败：工具列表不能为空",
			Suggestion: "请至少指定一个工具，例如 codex 或 claude",
			Err:        errors.New("empty tools"),
		}
	}
	if strings.TrimSpace(input.Message) == "" {
		return nil, &UserError{
			Message:    "创建快照失败：message 不能为空",
			Suggestion: "请通过 --message 或 TUI 输入本次快照说明",
			Err:        errors.New("empty message"),
		}
	}

	scope := input.Scope
	if scope == "" {
		scope = models.ScopeGlobal
	}

	pkg, err := w.snapshots.CreateSnapshot(models.CreateSnapshotOptions{
		Message:     strings.TrimSpace(input.Message),
		Tools:       input.Tools,
		Name:        strings.TrimSpace(input.Name),
		ProjectPath: strings.TrimSpace(input.ProjectPath),
		Scope:       scope,
	})
	if err != nil {
		if strings.Contains(err.Error(), "未找到任何配置文件") {
			return nil, &UserError{
				Message:    "创建快照失败：未找到任何配置文件",
				Suggestion: "请先确认所选工具的配置目录中存在可备份文件",
				Err:        err,
			}
		}

		return nil, &UserError{
			Message:    "创建快照失败",
			Suggestion: "请检查工具选择和本地配置目录后重试",
			Err:        err,
		}
	}

	snapshot := pkg.Snapshot
	if snapshot == nil {
		snapshot = &models.Snapshot{}
	}

	return &SnapshotSummary{
		ID:        snapshot.ID,
		Name:      snapshot.Name,
		Message:   snapshot.Message,
		CreatedAt: snapshot.CreatedAt,
		Tools:     snapshot.Tools,
		FileCount: pkg.FileCount,
		Size:      pkg.Size,
	}, nil
}

func buildToolSummary(installation *tool.ToolInstallation, toolType tool.ToolType) ToolSummary {
	if installation == nil {
		installation = &tool.ToolInstallation{
			ToolType: toolType,
			Status:   tool.StatusNotInstalled,
		}
	}

	summary := ToolSummary{
		Tool:         installation.ToolType.String(),
		Status:       string(installation.Status),
		Path:         installation.GlobalPath,
		ConfigCount:  len(installation.ConfigFiles),
		ProjectCount: len(installation.ProjectPaths),
		Items:        mapToolConfigItems(installation),
	}

	switch installation.Status {
	case tool.StatusInstalled:
		summary.ResultText = "可同步"
	case tool.StatusPartial:
		summary.ResultText = "暂不可同步"
		summary.Reason = "找到了配置目录，但未识别到可同步的配置文件"
	case tool.StatusNotInstalled:
		summary.ResultText = "不可同步"
		summary.Reason = "未找到配置目录 " + tool.GetDefaultGlobalPath(toolType)
		if summary.Path == "" {
			summary.Path = tool.GetDefaultGlobalPath(toolType)
		}
	default:
		summary.ResultText = "未知"
	}

	return summary
}

func mapToolConfigItems(installation *tool.ToolInstallation) []ToolConfigItem {
	if installation == nil || len(installation.ConfigFiles) == 0 {
		return nil
	}

	rootPath := installation.GlobalPath
	items := make([]ToolConfigItem, 0, len(installation.ConfigFiles))
	for _, file := range installation.ConfigFiles {
		group, label := describeToolConfig(installation.ToolType, file)
		relativePath := file.Name
		if rootPath != "" {
			if rel, err := filepath.Rel(rootPath, file.Path); err == nil && strings.TrimSpace(rel) != "" && rel != "." {
				relativePath = rel
			}
		}
		if file.IsDir && !strings.HasSuffix(relativePath, string(filepath.Separator)) {
			relativePath += string(filepath.Separator)
		}

		items = append(items, ToolConfigItem{
			Group:        group,
			Label:        label,
			RelativePath: relativePath,
		})
	}

	return items
}

func describeToolConfig(toolType tool.ToolType, file tool.ConfigFile) (string, string) {
	switch toolType {
	case tool.ToolTypeCodex:
		switch file.Name {
		case "config.toml":
			return "关键配置", "主配置"
		case "AGENTS.md":
			return "关键配置", "代理规则"
		case "skills":
			return "扩展内容", "技能目录"
		case "superpowers":
			return "扩展内容", "超能力目录"
		case "rules":
			return "扩展内容", "规则目录"
		}
	case tool.ToolTypeClaude:
		switch file.Name {
		case "settings.json":
			return "关键配置", "主配置"
		case "CLAUDE.md":
			return "关键配置", "说明文件"
		case "commands":
			return "扩展内容", "命令目录"
		case "skills":
			return "扩展内容", "技能目录"
		case "plugins":
			return "扩展内容", "插件目录"
		case "output-styles":
			return "扩展内容", "输出样式目录"
		case ".claude":
			return "关键配置", "项目配置目录"
		}
	}

	switch file.Category {
	case tool.CategoryAgents:
		return "关键配置", "代理规则"
	case tool.CategoryConfigFile:
		return "关键配置", file.Name
	default:
		return "其他内容", file.Name
	}
}

func (w *LocalWorkflow) ListSnapshots(_ context.Context, input ListSnapshotsInput) (*ListSnapshotsResult, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}

	snapshots, err := w.snapshots.ListSnapshots(limit, offset)
	if err != nil {
		return nil, &UserError{
			Message:    "读取快照列表失败",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	total, err := w.snapshots.CountSnapshots()
	if err != nil {
		return nil, &UserError{
			Message:    "读取快照统计失败",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	items := make([]SnapshotSummary, 0, len(snapshots))
	for _, snapshot := range snapshots {
		items = append(items, SnapshotSummary{
			ID:        snapshot.ID,
			Name:      snapshot.Name,
			Message:   snapshot.Message,
			CreatedAt: snapshot.CreatedAt,
			Tools:     snapshot.Tools,
			FileCount: len(snapshot.Files),
		})
	}

	return &ListSnapshotsResult{
		Total: total,
		Items: items,
	}, nil
}
