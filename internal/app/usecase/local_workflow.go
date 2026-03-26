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

// ScanRuleManager 负责持久化规则的增删查。
// 这里单独抽接口，避免 detector 同时承担“扫描”和“规则管理”两类职责。
type ScanRuleManager interface {
	AddCustomRule(toolType tool.ToolType, absolutePath string) error
	RemoveCustomRule(toolType tool.ToolType, absolutePath string) error
	AddProject(toolType tool.ToolType, projectName, projectPath string) error
	RemoveProject(toolType tool.ToolType, projectPath string) error
	ListCustomRules(toolType *tool.ToolType) ([]models.CustomSyncRule, error)
	ListRegisteredProjects(toolType *tool.ToolType) ([]models.RegisteredProject, error)
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
	Scan(ctx context.Context, input ScanInput) (*ScanResult, error)
	AddCustomRule(ctx context.Context, input AddCustomRuleInput) error
	RemoveCustomRule(ctx context.Context, input RemoveCustomRuleInput) error
	AddProject(ctx context.Context, input AddProjectInput) error
	RemoveProject(ctx context.Context, input RemoveProjectInput) error
	ListScanRules(ctx context.Context, input ListScanRulesInput) (*ListScanRulesResult, error)
	CreateSnapshot(ctx context.Context, input CreateSnapshotInput) (*SnapshotSummary, error)
	ListSnapshots(ctx context.Context, input ListSnapshotsInput) (*ListSnapshotsResult, error)
	GetConfig(ctx context.Context, input GetConfigInput) (*GetConfigResult, error)
	SaveConfig(ctx context.Context, input SaveConfigInput) error
}

type LocalWorkflow struct {
	detector  Detector
	snapshots SnapshotManager
	accessor  ConfigAccessor
	rules     ScanRuleManager
}

type ToolSummary struct {
	Tool        string
	Scope       string
	ProjectName string
	Status      string
	Path        string
	ConfigCount int
	ResultText  string
	Reason      string
	Items       []ToolConfigItem
}

type ToolConfigItem struct {
	Group        string
	Label        string
	RelativePath string
}

type ScanResult struct {
	Tools []ToolSummary
}

type ScanInput struct {
	Apps []string
}

type AddCustomRuleInput struct {
	App          string
	AbsolutePath string
}

type RemoveCustomRuleInput struct {
	App          string
	AbsolutePath string
}

type AddProjectInput struct {
	App         string
	ProjectName string
	ProjectPath string
}

type RemoveProjectInput struct {
	App         string
	ProjectPath string
}

type ListScanRulesInput struct {
	App string
}

type RuleItem struct {
	Path string
	Kind string
}

type RegisteredProjectItem struct {
	Name string
	Path string
}

type ListScanRulesResult struct {
	App                  string
	DefaultGlobalRules   []RuleItem
	ProjectRuleTemplates []RuleItem
	CustomRules          []RuleItem
	RegisteredProjects   []RegisteredProjectItem
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

// WithScanRuleManager 以链式方式补充规则管理依赖，保持现有构造器兼容。
func (w *LocalWorkflow) WithScanRuleManager(rules ScanRuleManager) *LocalWorkflow {
	w.rules = rules
	return w
}

func (w *LocalWorkflow) Scan(ctx context.Context, input ScanInput) (*ScanResult, error) {
	result, err := w.detector.DetectWithOptions(ctx, &tool.ScanOptions{
		ScanGlobal:   true,
		ScanProjects: true,
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

	items := make([]ToolSummary, 0, 2+len(result.ProjectInstallations))
	items = append(items, buildToolSummary(result.Codex, tool.ToolTypeCodex))
	for _, projectInstallation := range result.ProjectInstallations {
		if projectInstallation == nil || projectInstallation.ToolType != tool.ToolTypeCodex {
			continue
		}
		items = append(items, buildProjectSummary(projectInstallation))
	}
	items = append(items, buildToolSummary(result.Claude, tool.ToolTypeClaude))
	for _, projectInstallation := range result.ProjectInstallations {
		if projectInstallation == nil || projectInstallation.ToolType != tool.ToolTypeClaude {
			continue
		}
		items = append(items, buildProjectSummary(projectInstallation))
	}

	filteredItems, err := filterScanSummaries(items, input.Apps)
	if err != nil {
		return nil, err
	}

	return &ScanResult{Tools: filteredItems}, nil
}

func (w *LocalWorkflow) AddCustomRule(_ context.Context, input AddCustomRuleInput) error {
	toolType, err := resolveToolType(input.App)
	if err != nil {
		return err
	}
	if w.rules == nil {
		return newRulesUnavailableError()
	}
	if strings.TrimSpace(input.AbsolutePath) == "" {
		return &UserError{Message: "添加规则失败：路径不能为空", Suggestion: "请提供绝对路径文件"}
	}
	if err := w.rules.AddCustomRule(toolType, strings.TrimSpace(input.AbsolutePath)); err != nil {
		return &UserError{Message: "添加规则失败", Suggestion: "请检查工具名与路径后重试", Err: err}
	}
	return nil
}

func (w *LocalWorkflow) RemoveCustomRule(_ context.Context, input RemoveCustomRuleInput) error {
	toolType, err := resolveToolType(input.App)
	if err != nil {
		return err
	}
	if w.rules == nil {
		return newRulesUnavailableError()
	}
	if err := w.rules.RemoveCustomRule(toolType, strings.TrimSpace(input.AbsolutePath)); err != nil {
		return &UserError{Message: "删除规则失败", Suggestion: "请检查工具名与路径后重试", Err: err}
	}
	return nil
}

func (w *LocalWorkflow) AddProject(_ context.Context, input AddProjectInput) error {
	toolType, err := resolveToolType(input.App)
	if err != nil {
		return err
	}
	if w.rules == nil {
		return newRulesUnavailableError()
	}
	if strings.TrimSpace(input.ProjectName) == "" || strings.TrimSpace(input.ProjectPath) == "" {
		return &UserError{Message: "添加项目失败：项目名和路径不能为空", Suggestion: "请提供项目名和绝对路径"}
	}
	if err := w.rules.AddProject(toolType, strings.TrimSpace(input.ProjectName), strings.TrimSpace(input.ProjectPath)); err != nil {
		return &UserError{Message: "添加项目失败", Suggestion: "请检查工具名、项目名和路径后重试", Err: err}
	}
	return nil
}

func (w *LocalWorkflow) RemoveProject(_ context.Context, input RemoveProjectInput) error {
	toolType, err := resolveToolType(input.App)
	if err != nil {
		return err
	}
	if w.rules == nil {
		return newRulesUnavailableError()
	}
	if err := w.rules.RemoveProject(toolType, strings.TrimSpace(input.ProjectPath)); err != nil {
		return &UserError{Message: "删除项目失败", Suggestion: "请检查工具名与项目路径后重试", Err: err}
	}
	return nil
}

// ListScanRules 同时返回内置默认规则和用户持久化的数据，供 CLI/TUI 共用。
func (w *LocalWorkflow) ListScanRules(_ context.Context, input ListScanRulesInput) (*ListScanRulesResult, error) {
	var selectedTool *tool.ToolType
	var filteredProjects []models.RegisteredProject
	if strings.TrimSpace(input.App) != "" {
		if toolType, err := resolveToolType(input.App); err == nil {
			selectedTool = &toolType
		} else {
			if w.rules == nil {
				return nil, err
			}
			project, projectToolType, projectErr := w.findRegisteredProjectFilter(strings.TrimSpace(input.App))
			if projectErr != nil {
				return nil, projectErr
			}
			selectedTool = &projectToolType
			filteredProjects = append(filteredProjects, project)
		}
	}

	result := &ListScanRulesResult{}
	if selectedTool != nil {
		result.App = selectedTool.String()
		result.DefaultGlobalRules = mapRuleDefinitions(tool.DefaultGlobalRules(*selectedTool))
		result.ProjectRuleTemplates = mapRuleDefinitions(tool.ProjectRuleTemplates(*selectedTool))
	} else {
		for _, toolType := range []tool.ToolType{tool.ToolTypeCodex, tool.ToolTypeClaude} {
			result.DefaultGlobalRules = append(result.DefaultGlobalRules, mapRuleDefinitions(tool.DefaultGlobalRules(toolType))...)
			result.ProjectRuleTemplates = append(result.ProjectRuleTemplates, mapRuleDefinitions(tool.ProjectRuleTemplates(toolType))...)
		}
	}

	if w.rules == nil {
		return result, nil
	}

	customRules, err := w.rules.ListCustomRules(selectedTool)
	if err != nil {
		return nil, &UserError{Message: "读取规则失败", Suggestion: "请检查本地规则数据后重试", Err: err}
	}
	for _, rule := range customRules {
		result.CustomRules = append(result.CustomRules, RuleItem{Path: rule.AbsolutePath, Kind: "file"})
	}

	projects, err := w.rules.ListRegisteredProjects(selectedTool)
	if err != nil {
		return nil, &UserError{Message: "读取项目失败", Suggestion: "请检查本地规则数据后重试", Err: err}
	}
	if len(filteredProjects) > 0 {
		projects = filteredProjects
	}
	for _, project := range projects {
		result.RegisteredProjects = append(result.RegisteredProjects, RegisteredProjectItem{Name: project.ProjectName, Path: project.ProjectPath})
	}

	return result, nil
}

func (w *LocalWorkflow) findRegisteredProjectFilter(filter string) (models.RegisteredProject, tool.ToolType, error) {
	projects, err := w.rules.ListRegisteredProjects(nil)
	if err != nil {
		return models.RegisteredProject{}, "", &UserError{
			Message:    "读取项目失败",
			Suggestion: "请检查本地规则数据后重试",
			Err:        err,
		}
	}

	for _, project := range projects {
		if !matchesRegisteredProjectFilter(project, filter) {
			continue
		}
		projectToolType, err := resolveToolType(project.ToolType)
		if err != nil {
			return models.RegisteredProject{}, "", err
		}
		return project, projectToolType, nil
	}

	return models.RegisteredProject{}, "", &UserError{
		Message:    "未找到匹配的应用或项目：" + filter,
		Suggestion: "请使用 codex、claude 或已注册项目名重试",
	}
}

func matchesRegisteredProjectFilter(project models.RegisteredProject, filter string) bool {
	if strings.EqualFold(project.ProjectName, filter) {
		return true
	}
	if strings.TrimSpace(project.ProjectPath) != "" && strings.EqualFold(filepath.Base(project.ProjectPath), filter) {
		return true
	}
	return false
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
			Scope:    tool.ScopeGlobal,
			Status:   tool.StatusNotInstalled,
		}
	}

	summary := ToolSummary{
		Tool:        installation.ToolType.String(),
		Scope:       string(installation.Scope),
		Status:      string(installation.Status),
		Path:        installation.GlobalPath,
		ConfigCount: len(installation.ConfigFiles),
		Items:       mapToolConfigItems(installation),
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

func buildProjectSummary(installation *tool.ToolInstallation) ToolSummary {
	summary := ToolSummary{
		Tool:        installation.ToolType.String(),
		Scope:       string(tool.ScopeProject),
		ProjectName: installation.ProjectName,
		Status:      string(installation.Status),
		Path:        installation.ProjectPath,
		ConfigCount: len(installation.ConfigFiles),
		Items:       mapToolConfigItems(installation),
	}

	switch installation.Status {
	case tool.StatusInstalled:
		summary.ResultText = "可同步"
	case tool.StatusPartial:
		summary.ResultText = "暂不可同步"
		summary.Reason = "已注册项目下未识别到可同步的配置文件"
	case tool.StatusNotInstalled:
		summary.ResultText = "不可同步"
		summary.Reason = "未找到已注册项目路径 " + installation.ProjectPath
	default:
		summary.ResultText = "未知"
	}

	return summary
}

func mapRuleDefinitions(definitions []tool.SyncRuleDefinition) []RuleItem {
	items := make([]RuleItem, 0, len(definitions))
	for _, definition := range definitions {
		kind := "file"
		if definition.IsDir {
			kind = "dir"
		}
		items = append(items, RuleItem{
			Path: definition.Path,
			Kind: kind,
		})
	}
	return items
}

func filterScanSummaries(items []ToolSummary, filters []string) ([]ToolSummary, error) {
	if len(filters) == 0 {
		return items, nil
	}

	result := make([]ToolSummary, 0, len(items))
	seen := map[string]struct{}{}
	for _, rawFilter := range filters {
		filter := strings.TrimSpace(strings.ToLower(rawFilter))
		if filter == "" {
			continue
		}

		matched := false
		for _, item := range items {
			if !matchesScanFilter(item, filter) {
				continue
			}
			matched = true

			key := item.Scope + "|" + item.Tool + "|" + item.ProjectName + "|" + item.Path
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, item)
		}

		if !matched {
			return nil, &UserError{
				Message:    "未找到匹配的应用或项目：" + strings.TrimSpace(rawFilter),
				Suggestion: "请使用 codex、claude 或已注册项目名重试",
			}
		}
	}

	return result, nil
}

func matchesScanFilter(item ToolSummary, filter string) bool {
	if strings.EqualFold(item.Tool, filter) {
		return true
	}
	if item.Scope != string(tool.ScopeProject) {
		return false
	}
	if strings.EqualFold(item.ProjectName, filter) {
		return true
	}
	if strings.TrimSpace(item.Path) != "" && strings.EqualFold(filepath.Base(item.Path), filter) {
		return true
	}
	return false
}

func resolveToolType(app string) (tool.ToolType, error) {
	switch strings.TrimSpace(strings.ToLower(app)) {
	case "codex":
		return tool.ToolTypeCodex, nil
	case "claude":
		return tool.ToolTypeClaude, nil
	default:
		return "", &UserError{
			Message:    "不支持的应用：" + strings.TrimSpace(app),
			Suggestion: "当前只支持 codex 或 claude",
		}
	}
}

func newRulesUnavailableError() error {
	return &UserError{
		Message:    "规则管理未初始化",
		Suggestion: "请先在 runtime 中接入规则存储后再使用该命令",
	}
}

func mapToolConfigItems(installation *tool.ToolInstallation) []ToolConfigItem {
	if installation == nil || len(installation.ConfigFiles) == 0 {
		return nil
	}

	rootPath := installation.GlobalPath
	if installation.Scope == tool.ScopeProject {
		// 项目扫描对象应以项目根目录为基准回显相对路径，
		// 否则会让用户误以为这些文件属于全局配置目录。
		rootPath = installation.ProjectPath
	}
	items := make([]ToolConfigItem, 0, len(installation.ConfigFiles))
	for _, file := range installation.ConfigFiles {
		group, label := describeToolConfig(installation.ToolType, file)
		relativePath := file.Name
		if rootPath != "" {
			if rel, err := filepath.Rel(rootPath, file.Path); err == nil && strings.TrimSpace(rel) != "" && rel != "." {
				if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					relativePath = rel
				}
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
		case ".codex":
			return "关键配置", "项目配置目录"
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
