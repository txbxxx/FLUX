package usecase

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"flux/internal/models"
	"flux/internal/service/tool"
	typesRemote "flux/internal/types/remote"
	typesScan "flux/internal/types/scan"
	typesSnapshot "flux/internal/types/snapshot"
	typesSync "flux/internal/types/sync"
)

// DefaultListLimit 快照列表默认分页大小。
const DefaultListLimit = 20

// Detector 工具检测接口，负责扫描本机 AI 工具的安装状态和配置文件。
// 实现类为 tool.ToolDetector，通过 DetectWithOptions 统一返回全局和项目两类检测结果。
type Detector interface {
	DetectWithOptions(ctx context.Context, opts *tool.ScanOptions) (*tool.ToolDetectionResult, error)
}

// ScanRuleManager 负责持久化规则的增删查。
// 这里单独抽接口，避免 detector 同时承担"扫描"和"规则管理"两类职责。
type ScanRuleManager interface {
	AddCustomRule(toolType tool.ToolType, absolutePath string) error
	RemoveCustomRule(toolType tool.ToolType, absolutePath string) error
	AddProject(toolType tool.ToolType, projectName, projectPath string) error
	RemoveProject(toolType tool.ToolType, projectPath string) error
	ListCustomRules(toolType *tool.ToolType) ([]typesScan.CustomRuleRecord, error)
	ListRegisteredProjects(toolType *tool.ToolType) ([]typesScan.RegisteredProjectRecord, error)
}

// SnapshotManager 快照持久化接口，屏蔽底层数据库细节。
type SnapshotManager interface {
	CreateSnapshot(options typesSnapshot.CreateSnapshotOptions) (*typesSnapshot.SnapshotPackage, error)
	ListSnapshots(limit, offset int) ([]*typesSnapshot.SnapshotListItem, error)
	CountSnapshots() (int, error)
	DeleteSnapshot(id string) error
	GetSnapshot(id string) (*models.Snapshot, error)
	UpdateSnapshot(snapshot *models.Snapshot) error
	RestoreSnapshot(id string, files []string, options typesSnapshot.ApplyOptions) (*typesSnapshot.RestoreResult, error)
	DiffSnapshots(sourceID, targetID string, verbose bool, tool string, pathPattern string, context int) (*typesSnapshot.DiffResult, error)
}

// ConfigAccessor 配置文件访问接口，支持目录浏览和文件读写。
type ConfigAccessor interface {
	Resolve(toolType tool.ToolType, relativePath string) (*tool.ConfigTarget, error)
	ListDir(target *tool.ConfigTarget) ([]tool.ConfigEntry, error)
	ReadFile(target *tool.ConfigTarget) (string, error)
	WriteFile(target *tool.ConfigTarget, content string) error
}

// Workflow 是用例层的顶层接口，聚合了扫描、规则管理、快照、配置浏览等所有用例。
// CLI 和 TUI 层只依赖此接口，不直接依赖具体实现。
type Workflow interface {
	Scan(ctx context.Context, input ScanInput) (*ScanResult, error)
	AddCustomRule(ctx context.Context, input AddCustomRuleInput) error
	RemoveCustomRule(ctx context.Context, input RemoveCustomRuleInput) error
	AddProject(ctx context.Context, input AddProjectInput) error
	RemoveProject(ctx context.Context, input RemoveProjectInput) error
	ListScanRules(ctx context.Context, input ListScanRulesInput) (*ListScanRulesResult, error)
	CreateSnapshot(ctx context.Context, input CreateSnapshotInput) (*SnapshotSummary, error)
	ListSnapshots(ctx context.Context, input ListSnapshotsInput) (*ListSnapshotsResult, error)
	DeleteSnapshot(ctx context.Context, input DeleteSnapshotInput) error
	UpdateSnapshot(ctx context.Context, input UpdateSnapshotInput) (*typesSnapshot.UpdateSnapshotResult, error)
	RestoreSnapshot(ctx context.Context, input RestoreSnapshotInput) (*typesSnapshot.RestoreResult, error)
	DiffSnapshots(ctx context.Context, input DiffSnapshotsInput) (*typesSnapshot.DiffResult, error)
	GetConfig(ctx context.Context, input GetConfigInput) (*GetConfigResult, error)
	SaveConfig(ctx context.Context, input SaveConfigInput) error
	// 远端仓库管理
	AddRemote(ctx context.Context, input typesRemote.AddRemoteInput) (*typesRemote.AddRemoteResult, error)
	ListRemotes(ctx context.Context) (*typesRemote.ListRemotesResult, error)
	RemoveRemote(ctx context.Context, input typesRemote.RemoveRemoteInput) (*typesRemote.ListRemotesResult, error)
	// AI setting 相关方法
	CreateAISetting(ctx context.Context, input CreateAISettingInput) (*CreateAISettingResult, error)
	ListAISettings(ctx context.Context, input ListAISettingsInput) (*ListAISettingsResult, error)
	GetAISetting(ctx context.Context, input GetAISettingInput) (*GetAISettingResult, error)
	DeleteAISetting(ctx context.Context, input DeleteAISettingInput) error
	SwitchAISetting(ctx context.Context, input SwitchAISettingInput) (*SwitchAISettingResult, error)
	EditAISetting(ctx context.Context, input EditAISettingInput) (*EditAISettingResult, error)
	// 新增批量方法
	GetAISettingsBatch(ctx context.Context, input GetAISettingsBatchInput) (*GetAISettingsBatchResult, error)
	DeleteAISettingsBatch(ctx context.Context, input DeleteAISettingsBatchInput) (*DeleteAISettingsBatchResult, error)
	// 同步操作
	SyncPush(ctx context.Context, input typesSync.SyncPushInput) (*typesSync.SyncPushResult, error)
	SyncPull(ctx context.Context, input typesSync.SyncPullInput) (*typesSync.SyncPullResult, error)
	SyncStatus(ctx context.Context, input typesSync.SyncStatusInput) (*typesSync.SyncStatusResult, error)
	// 历史版本
	SnapshotHistory(ctx context.Context, input SnapshotHistoryInput) (*typesSnapshot.HistoryResult, error)
	RestoreFromHistory(ctx context.Context, input RestoreFromHistoryInput) (*typesSnapshot.RestoreResult, error)
}

// LocalWorkflow 是 Workflow 的本地实现，编排本地扫描、快照、配置浏览等流程。
// 它不直接访问数据库，而是通过 Detector、SnapshotManager、ScanRuleManager 等接口协调工作。
type LocalWorkflow struct {
	detector         Detector               // 工具检测器，负责扫描本机配置
	snapshots        SnapshotManager        // 快照管理器，负责快照的持久化
	accessor         ConfigAccessor         // 配置访问器，负责文件的读写浏览
	rules            ScanRuleManager        // 规则管理器，负责持久化规则和项目的增删查（可选依赖）
	aiSettingManager AISettingManager       // AI 配置管理器（可选依赖）
	remoteConfigs    RemoteConfigManager    // 远端配置管理器（可选依赖）
	remoteTester     RemoteConnectionTester // 远端连通性测试（可选依赖）
	dataDir          string                 // 应用数据目录（用于 repos 路径）
}

// --- 数据结构定义 ---
// 以下结构体用于在各层之间传递数据，不包含业务逻辑。

// ToolSummary 是 scan 结果中单个工具/项目的摘要，供 CLI/TUI 渲染。
type ToolSummary struct {
	Tool        string           // 工具类型：codex 或 claude
	Scope       string           // 作用域：global 或 project
	ProjectName string           // 项目名（全局项目为 codex-global / claude-global）
	Status      string           // 安装状态：installed / partial / not_installed
	Path        string           // 配置目录的绝对路径
	ConfigCount int              // 命中的配置文件数量
	ResultText  string           // 面向用户的状态描述（如"可同步"、"不可同步"）
	Reason      string           // 不可同步时的原因说明
	Items       []ToolConfigItem // 具体命中的配置文件列表
}

// ToolConfigItem 描述单个命中的配置文件，用于 scan 结果的分组展示。
type ToolConfigItem struct {
	Group        string // 分组名：关键配置、扩展内容、其他内容
	Label        string // 展示标签：主配置、技能目录等
	RelativePath string // 相对于项目根目录的路径
}

// ScanResult 是 Scan 用例的返回值，包含所有匹配的扫描摘要。
type ScanResult struct {
	Tools []ToolSummary
}

// ScanInput 是 Scan 用例的输入参数。
type ScanInput struct {
	Apps []string // 过滤条件：按工具名或项目名过滤，空则返回全部
}

// AddCustomRuleInput 是添加自定义规则的输入。
type AddCustomRuleInput struct {
	App          string // 工具类型：codex 或 claude
	AbsolutePath string // 要添加的配置文件绝对路径
}

// RemoveCustomRuleInput 是删除自定义规则的输入。
type RemoveCustomRuleInput struct {
	App          string
	AbsolutePath string
}

// AddProjectInput 是注册项目的输入。
type AddProjectInput struct {
	App         string // 工具类型
	ProjectName string // 项目名称
	ProjectPath string // 项目根目录绝对路径
}

// RemoveProjectInput 是移除注册项目的输入。
type RemoveProjectInput struct {
	App         string
	ProjectPath string
}

// ListScanRulesInput 是查看扫描规则的输入。
type ListScanRulesInput struct {
	App string // 工具类型或项目名，空则返回全部
}

// RuleItem 描述一条扫描规则。
type RuleItem struct {
	Path string // 规则路径（相对路径）
	Kind string // 类型：file 或 dir
}

// RegisteredProjectItem 描述一个已注册的项目。
type RegisteredProjectItem struct {
	Name string // 项目名
	Path string // 项目路径
}

// ListScanRulesResult 是查看扫描规则的返回值。
type ListScanRulesResult struct {
	App                  string                  // 匹配到的工具类型
	DefaultGlobalRules   []RuleItem              // 内置全局规则
	ProjectRuleTemplates []RuleItem              // 内置项目规则模板
	CustomRules          []RuleItem              // 用户自定义规则
	RegisteredProjects   []RegisteredProjectItem // 已注册项目列表
}

// CreateSnapshotInput 是创建快照的输入参数。
type CreateSnapshotInput struct {
	Tools       []string // 要备份的工具列表（可从项目名自动推导，无需用户手动指定）
	Message     string   // 快照说明（必填）
	Name        string   // 快照名称（可选）
	ProjectName string   // 项目名称（必填）
}

// SnapshotSummary 是创建快照的返回值摘要。
type SnapshotSummary struct {
	ID        uint      // 快照唯一 ID
	Name      string    // 快照名称
	Message   string    // 快照说明
	CreatedAt time.Time // 创建时间
	Project   string    // 关联的项目名称
	FileCount int       // 收集的文件数量
	Size      int64     // 总文件大小（字节）
}

// ListSnapshotsInput 是查询快照列表的分页参数。
type ListSnapshotsInput struct {
	Limit  int // 每页条数，<=0 时使用 DefaultListLimit
	Offset int // 偏移量，<0 时归零
}

// ListSnapshotsResult 是快照列表的返回值。
type ListSnapshotsResult struct {
	Total int               // 快照总数（用于分页计算）
	Items []SnapshotSummary // 当前页的快照列表
}

// DeleteSnapshotInput 是删除快照的输入参数。
type DeleteSnapshotInput struct {
	IDOrName string // 快照 ID 或名称
}

// UpdateSnapshotInput 更新快照的输入参数。
type UpdateSnapshotInput struct {
	IDOrName string // 快照 ID 或名称
	Message  string // 更新说明（可选，默认保留原 message）
}

// DiffSnapshotsInput is the input for comparing snapshots.
type DiffSnapshotsInput struct {
	SourceID    string // 源快照 ID 或名称
	TargetID    string // 目标快照 ID 或名称（空则对比文件系统）
	Verbose     bool   // 是否显示内容级 diff
	SideBySide  bool   // 是否并排显示
	Tool        string // 按工具类型过滤（空则不过滤）
	PathPattern string // 按路径模式过滤（空则不过滤）
	Context     int    // 上下文行数（默认 5）
}

// RestoreSnapshotInput is the input for restoring a snapshot to disk.
type RestoreSnapshotInput struct {
	IDOrName  string   // 快照 ID 或名称
	Files     []string // 指定要恢复的文件路径（空=全部恢复）
	DryRun    bool     // 预览模式：只展示变更摘要，不备份不写入
	Force     bool     // 跳过用户确认步骤，但仍自动备份
	BackupDir string   // 备份基础目录（由 CLI 层从 DataDir 传入）
}

// UserError 是面向用户的错误类型，包含可展示的 Message 和引导性的 Suggestion。
// CLI/TUI 层捕获此错误后可直接将 Message 和 Suggestion 展示给用户，
// 而 Err 字段保留原始错误供日志记录，不暴露给用户。
type UserError struct {
	Message    string // 面向用户的错误描述
	Suggestion string // 修复建议
	Err        error  // 原始错误（可为 nil）
}

// Error 实现 error 接口。当 Err 不为 nil 时附加原始错误信息，
// 方便日志中同时看到用户描述和技术细节。
func (e *UserError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Message
	}
	return e.Message + ": " + e.Err.Error()
}

// Unwrap 支持 errors.Is/As 链式查找，让调用方可以用 errors.As 提取 UserError。
func (e *UserError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewLocalWorkflow 创建 LocalWorkflow 实例。
// detector 和 snapshots 是必需依赖，accessor 是可选依赖（用变长参数避免每加一个可选依赖就改签名）。
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
// 这样调用方可以按需注入：NewLocalWorkflow(detector, snapshots).WithScanRuleManager(rules)
func (w *LocalWorkflow) WithScanRuleManager(rules ScanRuleManager) *LocalWorkflow {
	w.rules = rules
	return w
}

// Scan 扫描本机所有已注册项目，返回每个项目的配置状态摘要。
//
// 统一项目模型：所有可同步的对象都是"项目"，包括自动注册的全局项目
// （如 claude-global 指向 ~/.claude，codex-global 指向 ~/.codex）。
// 不再区分"全局"和"项目"两种展示形态，scan list 中只有项目条目。
//
// 输出顺序：先 Codex 项目，再 Claude 项目，保持展示的一致性。
func (w *LocalWorkflow) Scan(ctx context.Context, input ScanInput) (*ScanResult, error) {
	// 第一阶段：调用 Detector 获取原始检测结果。
	// 同时开启全局和项目扫描（ScanGlobal=true 用于内部兼容，ScanProjects=true 用于实际数据采集）。
	result, err := w.detector.DetectWithOptions(ctx, &tool.ScanOptions{
		ScanGlobal:   true,
		ScanProjects: true,
		IncludeFiles: true,
		MaxDepth:     1,
	})
	if err != nil {
		return nil, &UserError{
			Message:    "扫描失败",
			Suggestion: "请确认 ~/.codex 或 ~/.claude 目录存在且有访问权限",
			Err:        err,
		}
	}

	// 第二阶段：将 detector 返回的 ProjectInstallations 转换为展示摘要。
	// 按 Codex → Claude 的顺序排列，确保输出稳定可预测。
	items := make([]ToolSummary, 0, len(result.ProjectInstallations))
	for _, projectInstallation := range result.ProjectInstallations {
		if projectInstallation == nil || projectInstallation.ToolType != tool.ToolTypeCodex {
			continue
		}
		items = append(items, buildProjectSummary(projectInstallation))
	}
	for _, projectInstallation := range result.ProjectInstallations {
		if projectInstallation == nil || projectInstallation.ToolType != tool.ToolTypeClaude {
			continue
		}
		items = append(items, buildProjectSummary(projectInstallation))
	}

	// 第三阶段：如果用户指定了过滤条件（如 "claude" 或项目名），
	// 则只返回匹配的条目；否则返回全部。
	filteredItems, err := filterScanSummaries(items, input.Apps)
	if err != nil {
		return nil, err
	}

	return &ScanResult{Tools: filteredItems}, nil
}

// AddCustomRule 添加一条自定义扫描规则到指定工具。
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

// RemoveCustomRule 移除一条自定义扫描规则。
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

// AddProject 注册一个新项目到指定工具下。
func (w *LocalWorkflow) AddProject(_ context.Context, input AddProjectInput) error {
	toolType, err := resolveToolType(input.App)
	if err != nil {
		return &UserError{Message: "添加项目失败：工具名无效", Suggestion: "工具名只支持 codex 或 claude", Err: err}
	}
	if w.rules == nil {
		return newRulesUnavailableError()
	}
	if strings.TrimSpace(input.ProjectName) == "" {
		return &UserError{Message: "添加项目失败：项目名不能为空", Suggestion: "请提供项目名（用于标识此项目）"}
	}
	if strings.TrimSpace(input.ProjectPath) == "" {
		return &UserError{Message: "添加项目失败：项目路径不能为空", Suggestion: "请提供项目根目录的绝对路径"}
	}
	if err := w.rules.AddProject(toolType, strings.TrimSpace(input.ProjectName), strings.TrimSpace(input.ProjectPath)); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "必须是目录") {
			return &UserError{Message: "添加项目失败：" + errMsg, Suggestion: "请确保路径指向一个存在的目录"}
		}
		if strings.Contains(errMsg, "不存在") {
			return &UserError{Message: "添加项目失败：" + errMsg, Suggestion: "请确保路径指向一个存在的目录"}
		}
		if strings.Contains(errMsg, "已存在") {
			return &UserError{Message: "添加项目失败：" + errMsg, Suggestion: "请使用其他项目名，或先删除已存在的同名项目"}
		}
		return &UserError{Message: "添加项目失败：" + errMsg, Suggestion: "请检查项目路径是否有效后重试", Err: err}
	}
	return nil
}

// RemoveProject 移除一个已注册的项目。
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
//
// 当 input.App 为工具名（如 "claude"）时，返回该工具的所有规则和项目；
// 当 input.App 为项目名（如 "demo"）时，先通过数据库查找项目，再返回该项目所属工具的规则；
// 当 input.App 为空时，返回所有工具的规则。
func (w *LocalWorkflow) ListScanRules(_ context.Context, input ListScanRulesInput) (*ListScanRulesResult, error) {
	var selectedTool *tool.ToolType
	var filteredProjects []typesScan.RegisteredProjectRecord

	// 第一阶段：解析 input.App，确定要查询的工具类型。
	// 优先按工具名匹配（codex/claude），匹配失败则按项目名查找。
	if strings.TrimSpace(input.App) != "" {
		if toolType, err := resolveToolType(input.App); err == nil {
			selectedTool = &toolType
		} else {
			// 不是工具名，尝试按项目名查找
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

	// 第二阶段：加载内置规则定义（全局规则 + 项目规则模板）。
	result := &ListScanRulesResult{}
	if selectedTool != nil {
		result.App = selectedTool.String()
		result.DefaultGlobalRules = mapRuleDefinitions(tool.DefaultGlobalRules(*selectedTool))
		result.ProjectRuleTemplates = mapRuleDefinitions(tool.ProjectRuleTemplates(*selectedTool))
	} else {
		// 未指定工具时，合并所有工具的规则
		for _, toolType := range []tool.ToolType{tool.ToolTypeCodex, tool.ToolTypeClaude} {
			result.DefaultGlobalRules = append(result.DefaultGlobalRules, mapRuleDefinitions(tool.DefaultGlobalRules(toolType))...)
			result.ProjectRuleTemplates = append(result.ProjectRuleTemplates, mapRuleDefinitions(tool.ProjectRuleTemplates(toolType))...)
		}
	}

	// 第三阶段：加载用户持久化的数据（自定义规则 + 已注册项目）。
	// 如果没有规则管理器（如数据库未初始化），跳过此阶段。
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
	// 如果用户按项目名查询，只展示匹配的项目
	if len(filteredProjects) > 0 {
		projects = filteredProjects
	}
	for _, project := range projects {
		result.RegisteredProjects = append(result.RegisteredProjects, RegisteredProjectItem{Name: project.ProjectName, Path: project.ProjectPath})
	}

	return result, nil
}

// findRegisteredProjectFilter 在已注册项目中查找匹配 filter 的项目。
// 匹配规则：项目名精确匹配（不区分大小写）或项目路径的 basename 匹配。
func (w *LocalWorkflow) findRegisteredProjectFilter(filter string) (typesScan.RegisteredProjectRecord, tool.ToolType, error) {
	projects, err := w.rules.ListRegisteredProjects(nil)
	if err != nil {
		return typesScan.RegisteredProjectRecord{}, "", &UserError{
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
			return typesScan.RegisteredProjectRecord{}, "", err
		}
		return project, projectToolType, nil
	}

	return typesScan.RegisteredProjectRecord{}, "", &UserError{
		Message:    "未找到匹配的应用或项目：" + filter,
		Suggestion: "请使用 codex、claude 或已注册项目名重试",
	}
}

// matchesRegisteredProjectFilter 判断项目是否匹配过滤条件。
// 支持项目名匹配和路径 basename 匹配两种方式，均不区分大小写。
func matchesRegisteredProjectFilter(project typesScan.RegisteredProjectRecord, filter string) bool {
	if strings.EqualFold(project.ProjectName, filter) {
		return true
	}
	if strings.TrimSpace(project.ProjectPath) != "" && strings.EqualFold(filepath.Base(project.ProjectPath), filter) {
		return true
	}
	return false
}

// CreateSnapshot 创建本地配置快照。
//
// 工具类型推导策略（用户无需显式指定 -t 参数）：
//  1. 如果用户传了 -t（input.Tools 不为空），直接使用
//  2. 如果用户只传了 -p，从项目名自动推导：
//     a. 项目名前缀匹配（如 claude-global → claude，codex-global → codex）
//     b. 查数据库中已注册项目的 ToolType 字段
//  3. 都没有则报错，提示用户指定
//
// 这样用户只需要 `fl snapshot create -m 说明 -p claude` 即可，
// 不需要再手动传 `-t claude`。
func (w *LocalWorkflow) CreateSnapshot(_ context.Context, input CreateSnapshotInput) (*SnapshotSummary, error) {
	// 第一步：自动推导工具类型（当用户未指定 -t 时）
	if len(input.Tools) == 0 && strings.TrimSpace(input.ProjectName) != "" {
		inferredTools := w.inferToolsFromProject(strings.TrimSpace(input.ProjectName))
		if len(inferredTools) > 0 {
			input.Tools = inferredTools
		}
	}

	// 第二步：参数校验
	if len(input.Tools) == 0 {
		return nil, &UserError{
			Message:    "创建快照失败：无法确定工具类型",
			Suggestion: "请通过 -t 指定工具（如 codex 或 claude），或确保 -p 项目名已注册",
			Err:        errors.New("empty tools"),
		}
	}
	// 名称为空时自动生成
	trimmedName := strings.TrimSpace(input.Name)
	if trimmedName == "" {
		trimmedName = fmt.Sprintf("snapshot-%s", time.Now().Format("20060102-150405"))
	}
	// 名称唯一性校验：遍历已有快照检查是否重名。
	existingSnapshots, listErr := w.snapshots.ListSnapshots(0, 0)
	if listErr == nil {
		for _, snap := range existingSnapshots {
			if snap.Name == trimmedName {
				return nil, &UserError{
					Message:    "创建快照失败：名称 \"" + trimmedName + "\" 已存在",
					Suggestion: "请使用 snapshot list 查看已有快照，指定一个不同的名称",
					Err:        errors.New("duplicate name"),
				}
			}
		}
	}
	if strings.TrimSpace(input.Message) == "" {
		return nil, &UserError{
			Message:    "创建快照失败：message 不能为空",
			Suggestion: "请通过 --message 或 TUI 输入本次快照说明",
			Err:        errors.New("empty message"),
		}
	}
	if strings.TrimSpace(input.ProjectName) == "" {
		return nil, &UserError{
			Message:    "创建快照失败：必须指定项目名称",
			Suggestion: "请使用 --project 参数指定项目（如 codex-global、claude-global 或用户注册的项目）",
			Err:        errors.New("empty project name"),
		}
	}

	// 第三步：调用 SnapshotManager 创建快照
	pkg, err := w.snapshots.CreateSnapshot(typesSnapshot.CreateSnapshotOptions{
		Message:     strings.TrimSpace(input.Message),
		Tools:       input.Tools,
		Name:        strings.TrimSpace(input.Name),
		ProjectName: strings.TrimSpace(input.ProjectName),
	})
	if err != nil {
		// 对"未找到配置文件"这种常见错误给出更明确的建议
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

	// 第四步：将服务层返回的快照包转换为展示摘要
	snapshot := pkg.Snapshot
	if snapshot == nil {
		snapshot = &typesSnapshot.SnapshotHeader{}
	}

	return &SnapshotSummary{
		ID:        snapshot.ID,
		Name:      snapshot.Name,
		Message:   snapshot.Message,
		CreatedAt: snapshot.CreatedAt,
		Project:   snapshot.Project,
		FileCount: pkg.FileCount,
		Size:      pkg.Size,
	}, nil
}

// buildProjectSummary 将 detector 返回的 ToolInstallation 转换为面向用户的 ToolSummary。
// 根据安装状态生成 ResultText 和 Reason，方便 CLI/TUI 直接展示。
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

	// 根据状态生成用户友好的描述文本
	switch installation.Status {
	case tool.StatusInstalled:
		summary.ResultText = "可同步"
	case tool.StatusPartial:
		// Partial：配置目录存在，但按规则模板未找到可同步的文件
		summary.ResultText = "暂不可同步"
		summary.Reason = "已注册项目下未识别到可同步的配置文件"
	case tool.StatusNotInstalled:
		// NotInstalled：注册的项目路径在磁盘上不存在（可能被删除或移动）
		summary.ResultText = "不可同步"
		summary.Reason = "未找到已注册项目路径 " + installation.ProjectPath
	default:
		summary.ResultText = "未知"
	}

	return summary
}

// mapRuleDefinitions 将 service 层的 SyncRuleDefinition 转换为用例层的 RuleItem。
// 这个转换将 IsDir 布尔值转为 "file"/"dir" 字符串，方便 CLI 展示。
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

// filterScanSummaries 按用户指定的过滤条件筛选扫描结果。
//
// 过滤逻辑：
//   - 每个过滤词按"工具名 > 项目名 > 路径 basename"的优先级匹配
//   - 同一条结果只出现一次（通过 composite key 去重）
//   - 如果某个过滤词没有任何匹配，返回错误提示用户可用的选项
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

			// 用 scope+tool+projectName+path 组合去重，
			// 防止 "claude" 同时匹配工具名和项目名时出现重复条目。
			key := item.Scope + "|" + item.Tool + "|" + item.ProjectName + "|" + item.Path
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, item)
		}

		// 如果过滤词完全没有匹配，提前返回错误。
		// 这样用户能立即知道输错了什么，而不是看到空列表。
		if !matched {
			return nil, &UserError{
				Message:    "未找到 \"" + strings.TrimSpace(rawFilter) + "\"，支持的应用: codex、claude",
				Suggestion: "可输入 codex 或 claude 查看对应工具，或使用已注册的项目名",
			}
		}
	}

	return result, nil
}

// matchesScanFilter 判断单个扫描条目是否匹配过滤词。
// 匹配优先级：工具名 > 项目名 > 路径 basename，均不区分大小写。
func matchesScanFilter(item ToolSummary, filter string) bool {
	// 优先匹配工具名（codex / claude）
	if strings.EqualFold(item.Tool, filter) {
		return true
	}
	// 以下匹配只适用于项目作用域的条目
	if item.Scope != string(tool.ScopeProject) {
		return false
	}
	// 匹配项目名（如 demo、flux）
	if strings.EqualFold(item.ProjectName, filter) {
		return true
	}
	// 匹配路径 basename（如 /workspace/demo 中的 demo）
	if strings.TrimSpace(item.Path) != "" && strings.EqualFold(filepath.Base(item.Path), filter) {
		return true
	}
	return false
}

// resolveToolType 将用户输入的工具名转换为内部 ToolType 枚举。
// 输入不区分大小写，前后空格会被忽略。
func resolveToolType(app string) (tool.ToolType, error) {
	switch strings.TrimSpace(strings.ToLower(app)) {
	case "codex":
		return tool.ToolTypeCodex, nil
	case "claude":
		return tool.ToolTypeClaude, nil
	default:
		return "", &UserError{
			Message:    "不支持的应用 \"" + strings.TrimSpace(app) + "\"，目前支持: codex、claude",
			Suggestion: "请输入 codex 或 claude 作为应用名",
		}
	}
}

// inferToolsFromProject 根据项目名推导工具类型列表。
//
// 推导策略（按优先级）：
//  1. 前缀匹配：项目名以 "codex" 或 "claude" 开头时直接推导。
//     适用于系统自动注册的全局项目（codex-global、claude-global），
//     也适用于用户按约定命名的项目（如 claude-my-config）。
//  2. 数据库查找：在已注册项目中按项目名精确匹配，取其 ToolType 字段。
//     适用于用户自定义项目名（如 demo、my-project）。
//
// 返回 nil 表示无法推导，调用方应要求用户显式指定工具类型。
func (w *LocalWorkflow) inferToolsFromProject(projectName string) []string {
	// 策略 1：快速前缀匹配，无需查数据库
	for _, prefix := range []string{"codex", "claude"} {
		if strings.HasPrefix(strings.ToLower(projectName), prefix) {
			return []string{prefix}
		}
	}

	// 策略 2：查数据库中已注册项目
	if w.rules != nil {
		projects, err := w.rules.ListRegisteredProjects(nil)
		if err == nil {
			for _, p := range projects {
				if p.ProjectName == projectName {
					return []string{p.ToolType}
				}
			}
		}
	}

	return nil
}

// newRulesUnavailableError 返回规则管理不可用的标准错误。
// 当数据库未初始化或依赖注入不完整时使用。
func newRulesUnavailableError() error {
	return &UserError{
		Message:    "规则管理功能暂不可用",
		Suggestion: "请重新启动程序，或检查数据目录是否正常",
	}
}

// mapToolConfigItems 将 detector 返回的 ConfigFile 列表转换为展示用的 ToolConfigItem。
// 关键逻辑：将绝对路径转为相对于项目根目录的相对路径，避免在展示中暴露用户主目录等敏感信息。
func mapToolConfigItems(installation *tool.ToolInstallation) []ToolConfigItem {
	if installation == nil || len(installation.ConfigFiles) == 0 {
		return nil
	}

	// 确定相对路径的基准目录：
	// - 全局扫描结果以全局配置目录为基准
	// - 项目扫描结果以项目根目录为基准
	// 这样用户看到的路径都是相对于各自根目录的，不会产生误导。
	rootPath := installation.GlobalPath
	if installation.Scope == tool.ScopeProject {
		rootPath = installation.ProjectPath
	}

	items := make([]ToolConfigItem, 0, len(installation.ConfigFiles))
	for _, file := range installation.ConfigFiles {
		group, label := describeToolConfig(installation.ToolType, file)

		// 计算相对路径：尝试将绝对路径转为相对于 rootPath 的路径。
		// 如果转换失败或结果超出 rootPath（如 ../xxx），保留原始文件名。
		relativePath := file.Name
		if rootPath != "" {
			if rel, err := filepath.Rel(rootPath, file.Path); err == nil && strings.TrimSpace(rel) != "" && rel != "." {
				if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					relativePath = rel
				}
			}
		}

		// 目录类型的路径追加路径分隔符，让用户一眼识别是目录
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

// describeToolConfig 根据工具类型和文件特征，返回面向用户的分组名和标签。
// 例如 Codex 的 config.toml → ("关键配置", "主配置")，Claude 的 commands/ → ("扩展内容", "命令目录")。
// 未被识别的文件按 Category 回退到通用标签。
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

	// 兜底：按 Category 返回通用标签
	switch file.Category {
	case tool.CategoryAgents:
		return "关键配置", "代理规则"
	case tool.CategoryConfigFile:
		return "关键配置", file.Name
	default:
		return "其他内容", file.Name
	}
}

// ListSnapshots 分页查询本地快照列表。
// limit <= 0 时使用 DefaultListLimit，offset < 0 时归零。
func (w *LocalWorkflow) ListSnapshots(_ context.Context, input ListSnapshotsInput) (*ListSnapshotsResult, error) {
	// 规范化分页参数
	limit := input.Limit
	if limit <= 0 {
		limit = DefaultListLimit
	}
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}

	// 查询当前页数据
	snapshots, err := w.snapshots.ListSnapshots(limit, offset)
	if err != nil {
		return nil, &UserError{
			Message:    "无法读取快照列表",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	// 查询总数（用于前端分页计算）
	total, err := w.snapshots.CountSnapshots()
	if err != nil {
		return nil, &UserError{
			Message:    "无法读取快照统计",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	// 转换为展示摘要
	items := make([]SnapshotSummary, 0, len(snapshots))
	for _, snapshot := range snapshots {
		items = append(items, SnapshotSummary{
			ID:        snapshot.ID,
			Name:      snapshot.Name,
			Message:   snapshot.Message,
			CreatedAt: snapshot.CreatedAt,
			Project:   snapshot.Project,
			FileCount: snapshot.FileCount,
		})
	}

	return &ListSnapshotsResult{
		Total: total,
		Items: items,
	}, nil
}

// DeleteSnapshot 删除指定快照。
// 支持通过完整 ID 或快照名称删除。使用名称时会自动查找匹配的快照。
func (w *LocalWorkflow) DeleteSnapshot(_ context.Context, input DeleteSnapshotInput) error {
	if strings.TrimSpace(input.IDOrName) == "" {
		return &UserError{
			Message:    "删除快照失败：请指定快照 ID 或名称",
			Suggestion: "使用 snapshot list 查看快照列表",
		}
	}

	id := strings.TrimSpace(input.IDOrName)

	// 如果不是数字 ID 格式，按名称查找对应的 ID。
	if !isNumericID(id) {
		snapshots, err := w.snapshots.ListSnapshots(0, 0)
		if err != nil {
			return &UserError{
				Message:    "删除快照失败：无法查找快照",
				Suggestion: "请检查本地数据库是否可访问",
				Err:        err,
			}
		}

		found := false
		for _, snap := range snapshots {
			if snap.Name == id {
				id = fmt.Sprintf("%d", snap.ID)
				found = true
				break
			}
		}
		if !found {
			return &UserError{
				Message:    "删除快照失败：未找到名称为 \"" + id + "\" 的快照",
				Suggestion: "使用 snapshot list 查看所有快照",
			}
		}
	}

	// 验证快照存在。
	if _, err := w.snapshots.GetSnapshot(id); err != nil {
		return &UserError{
			Message:    "删除快照失败：快照不存在",
			Suggestion: "请检查快照 ID 是否正确",
			Err:        err,
		}
	}

	if err := w.snapshots.DeleteSnapshot(id); err != nil {
		return &UserError{
			Message:    "删除快照失败",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	return nil
}

// isNumericID 检查字符串是否为数字 ID 格式。
func isNumericID(s string) bool {
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}

// RestoreSnapshot restores a snapshot's files to their original paths on disk.
//
// 流程：
//  1. 通过 ID 或名称查找快照
//  2. 构造备份路径
//  3. 调用 Service.RestoreSnapshot 执行恢复
//  4. 将底层技术性错误翻译为用户可读错误
func (w *LocalWorkflow) RestoreSnapshot(_ context.Context, input RestoreSnapshotInput) (*typesSnapshot.RestoreResult, error) {
	// 第一步：参数校验
	if strings.TrimSpace(input.IDOrName) == "" {
		return nil, &UserError{
			Message:    "恢复快照失败：请指定快照 ID 或名称",
			Suggestion: "使用 snapshot list 查看快照列表",
		}
	}

	// 第二步：解析 ID（支持名称查找，复用 DeleteSnapshot 的模式）
	id := strings.TrimSpace(input.IDOrName)
	if !isNumericID(id) {
		snapshots, err := w.snapshots.ListSnapshots(0, 0)
		if err != nil {
			return nil, &UserError{
				Message:    "恢复快照失败：无法查找快照",
				Suggestion: "请检查本地数据库是否可访问",
				Err:        err,
			}
		}

		found := false
		for _, snap := range snapshots {
			if snap.Name == id {
				id = fmt.Sprintf("%d", snap.ID)
				found = true
				break
			}
		}
		if !found {
			return nil, &UserError{
				Message:    "恢复快照失败：未找到名称为 \"" + id + "\" 的快照",
				Suggestion: "使用 snapshot list 查看所有快照",
			}
		}
	}

	// 第三步：构造备份路径（仅非 dry-run 时需要）
	backupPath := ""
	if !input.DryRun && strings.TrimSpace(input.BackupDir) != "" {
		backupPath = filepath.Join(input.BackupDir, "backup", time.Now().Format("20060102-150405"))
	}

	// 第四步：调用 Service 执行恢复
	result, err := w.snapshots.RestoreSnapshot(id, input.Files, typesSnapshot.ApplyOptions{
		CreateBackup: !input.DryRun,
		BackupPath:   backupPath,
		Force:        false,
		DryRun:       input.DryRun,
	})
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "获取快照失败") {
			return nil, &UserError{
				Message:    "恢复快照失败：快照不存在",
				Suggestion: "请检查快照 ID 或名称是否正确",
				Err:        err,
			}
		}
		if strings.Contains(errMsg, "不在快照中") {
			return nil, &UserError{
				Message:    errMsg,
				Suggestion: "请检查文件路径是否正确",
				Err:        err,
			}
		}
		return nil, &UserError{
			Message:    "恢复快照失败",
			Suggestion: "请检查本地数据库和文件系统权限",
			Err:        err,
		}
	}

	return result, nil
}

// DiffSnapshots compares two snapshots (or a snapshot with the filesystem)
// and returns structured diff results.
func (w *LocalWorkflow) DiffSnapshots(_ context.Context, input DiffSnapshotsInput) (*typesSnapshot.DiffResult, error) {
	// 第一步：参数校验
	if strings.TrimSpace(input.SourceID) == "" {
		return nil, &UserError{
			Message:    "对比快照失败：请指定源快照 ID 或名称",
			Suggestion: "使用 snapshot list 查看快照列表",
		}
	}

	// 第二步：解析源快照 ID（支持名称查找）
	sourceID, err := w.resolveSnapshotID(input.SourceID)
	if err != nil {
		return nil, &UserError{
			Message:    "对比快照失败：源快照 [" + input.SourceID + "] " + extractErrorReason(err),
			Suggestion: "请使用 snapshot list 查看可用快照，确认源快照存在",
			Err:        err,
		}
	}

	// 第三步：解析目标快照 ID（如果提供）
	var targetID string
	if input.TargetID != "" {
		targetID, err = w.resolveSnapshotID(input.TargetID)
		if err != nil {
			return nil, &UserError{
				Message:    "对比快照失败：目标快照 [" + input.TargetID + "] " + extractErrorReason(err),
				Suggestion: "请使用 snapshot list 查看可用快照，确认目标快照存在",
				Err:        err,
			}
		}
	}

	// 第四步：调用 Service
	result, err := w.snapshots.DiffSnapshots(sourceID, targetID, input.Verbose, input.Tool, input.PathPattern, input.Context)
	if err != nil {
		return nil, &UserError{
			Message:    "对比快照失败",
			Suggestion: "请检查快照是否存在",
			Err:        err,
		}
	}

	return result, nil
}

// resolveSnapshotID resolves a snapshot ID or name to a UUID.
// If the input is already in UUID format, it is returned as-is.
// Otherwise, the snapshot list is searched for a matching name.
func (w *LocalWorkflow) resolveSnapshotID(idOrName string) (string, error) {
	id := strings.TrimSpace(idOrName)
	if isNumericID(id) {
		return id, nil
	}

	snapshots, err := w.snapshots.ListSnapshots(0, 0)
	if err != nil {
		return "", &UserError{
			Message:    "无法查找快照",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	for _, snap := range snapshots {
		if snap.Name == id {
			return fmt.Sprintf("%d", snap.ID), nil
		}
	}

	return "", &UserError{
		Message:    "未找到名称为 \"" + id + "\" 的快照",
		Suggestion: "使用 snapshot list 查看所有快照",
	}
}

// SnapshotHistoryInput is the input for viewing snapshot history.
type SnapshotHistoryInput struct {
	IDOrName string // Snapshot ID or name
	Limit    int    // Max entries to return (0 = default 20)
}

// RestoreFromHistoryInput is the input for restoring a snapshot from a specific history version.
type RestoreFromHistoryInput struct {
	IDOrName   string   // Snapshot ID or name
	CommitHash string   // Git commit hash to restore from
	Files      []string // Specific files to restore (empty = all)
	DryRun     bool     // Preview mode
	Force      bool     // Skip confirmation
	BackupDir  string   // Backup base directory
}

// extractErrorReason 从 error 中提取简短的错误原因描述。
func extractErrorReason(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// 移除 "flux/internal/app/usecase." 前缀
	if strings.HasPrefix(msg, "未找到名称为") {
		return msg
	}
	return "未找到"
}
