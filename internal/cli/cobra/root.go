package cobra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	spcobra "github.com/spf13/cobra"

	"ai-sync-manager/internal/app/usecase"
	"ai-sync-manager/internal/cli/output"
typesRemote "ai-sync-manager/internal/types/remote"
	typesSync "ai-sync-manager/internal/types/sync"
	typesSnapshot "ai-sync-manager/internal/types/snapshot"
)

var errCommandHandled = errors.New("command handled")

// Workflow 抽象 CLI 依赖的全部用例能力，便于命令层测试替换。
type Workflow interface {
	Scan(ctx context.Context, input usecase.ScanInput) (*usecase.ScanResult, error)
	AddCustomRule(ctx context.Context, input usecase.AddCustomRuleInput) error
	RemoveCustomRule(ctx context.Context, input usecase.RemoveCustomRuleInput) error
	AddProject(ctx context.Context, input usecase.AddProjectInput) error
	RemoveProject(ctx context.Context, input usecase.RemoveProjectInput) error
	ListScanRules(ctx context.Context, input usecase.ListScanRulesInput) (*usecase.ListScanRulesResult, error)
	CreateSnapshot(ctx context.Context, input usecase.CreateSnapshotInput) (*usecase.SnapshotSummary, error)
	ListSnapshots(ctx context.Context, input usecase.ListSnapshotsInput) (*usecase.ListSnapshotsResult, error)
	DeleteSnapshot(ctx context.Context, input usecase.DeleteSnapshotInput) error
	UpdateSnapshot(ctx context.Context, input usecase.UpdateSnapshotInput) (*typesSnapshot.UpdateSnapshotResult, error)
	RestoreSnapshot(ctx context.Context, input usecase.RestoreSnapshotInput) (*typesSnapshot.RestoreResult, error)
	DiffSnapshots(ctx context.Context, input usecase.DiffSnapshotsInput) (*typesSnapshot.DiffResult, error)
	GetConfig(ctx context.Context, input usecase.GetConfigInput) (*usecase.GetConfigResult, error)
	SaveConfig(ctx context.Context, input usecase.SaveConfigInput) error
		// 远端仓库管理
		AddRemote(ctx context.Context, input typesRemote.AddRemoteInput) (*typesRemote.AddRemoteResult, error)
		ListRemotes(ctx context.Context) (*typesRemote.ListRemotesResult, error)
		RemoveRemote(ctx context.Context, input typesRemote.RemoveRemoteInput) (*typesRemote.ListRemotesResult, error)
	CreateAISetting(ctx context.Context, input usecase.CreateAISettingInput) (*usecase.CreateAISettingResult, error)
	ListAISettings(ctx context.Context, input usecase.ListAISettingsInput) (*usecase.ListAISettingsResult, error)
	GetAISetting(ctx context.Context, input usecase.GetAISettingInput) (*usecase.GetAISettingResult, error)
	DeleteAISetting(ctx context.Context, input usecase.DeleteAISettingInput) error
	SwitchAISetting(ctx context.Context, input usecase.SwitchAISettingInput) (*usecase.SwitchAISettingResult, error)
	EditAISetting(ctx context.Context, input usecase.EditAISettingInput) (*usecase.EditAISettingResult, error)
	// 新增批量方法
	GetAISettingsBatch(ctx context.Context, input usecase.GetAISettingsBatchInput) (*usecase.GetAISettingsBatchResult, error)
	DeleteAISettingsBatch(ctx context.Context, input usecase.DeleteAISettingsBatchInput) (*usecase.DeleteAISettingsBatchResult, error)
		// 同步操作
		SyncPush(ctx context.Context, input typesSync.SyncPushInput) (*typesSync.SyncPushResult, error)
		SyncPull(ctx context.Context, input typesSync.SyncPullInput) (*typesSync.SyncPullResult, error)
		SyncStatus(ctx context.Context, input typesSync.SyncStatusInput) (*typesSync.SyncStatusResult, error)
}

// TUIRunner / EditorRunner 抽象终端交互能力，避免 cobra 直接依赖具体实现。
type TUIRunner interface {
	Run(ctx context.Context) error
}

type EditorRunner interface {
	Run(ctx context.Context, result *usecase.GetConfigResult, save func(string) error) error
}

// Dependencies 集中描述 root command 所需的全部外部依赖。
// 所有子命令共享同一份 Dependencies 实例，通过它访问业务逻辑和 I/O。
type Dependencies struct {
	Workflow Workflow        // 用例层接口，提供所有业务操作（扫描、快照、配置读写等）
	TUI      TUIRunner       // TUI 交互式终端界面启动器
	Editor   EditorRunner    // 外部编辑器启动器（如 vim / code）
	DataDir  string          // 应用数据目录路径（存放数据库、配置等）
	Out      io.Writer       // 标准输出写入器，nil 时回退到 io.Discard
	Err      io.Writer       // 标准错误写入器，nil 时回退到 io.Discard
	Context  context.Context // 外部上下文（用于取消控制和超时），nil 时使用 context.Background
}

// NewRootCommand 组装顶层命令并注入所有子命令。
func NewRootCommand(deps Dependencies) *spcobra.Command {
	root := &spcobra.Command{
		Use:           "ai-sync",
		Short:         "AI tool config snapshot manager",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *spcobra.Command, _ []string) error {
			printUsage(cmd.ErrOrStderr())
			return errCommandHandled
		},
	}

	root.SetOut(outputWriter(deps.Out))
	root.SetErr(errorWriter(deps.Err))
	root.AddCommand(
		newScanCommand(deps),
		newSnapshotCommand(deps),
		newTUICommand(deps),
		newGetCommand(deps),
		newRemoteCommand(deps),
		newSyncCommand(deps),
		newSettingCommand(deps),
	)

	return root
}

// Execute 负责执行命令，并把"已自行输出帮助"的情况转换为稳定退出码。
func Execute(deps Dependencies, args []string) int {
	cmd := NewRootCommand(deps)
	cmd.SetArgs(args)

	if err := cmd.ExecuteContext(commandContext(deps)); err != nil {
		if !errors.Is(err, errCommandHandled) && !errors.Is(err, errDiffHasChanges) {
			printError(cmd.ErrOrStderr(), err)
		}
		return 1
	}

	return 0
}

// commandContext 优先复用外部上下文，便于测试和取消控制。
func commandContext(deps Dependencies) context.Context {
	if deps.Context != nil {
		return deps.Context
	}
	return context.Background()
}

// outputWriter / errorWriter 在未注入 writer 时回退到 io.Discard，避免 nil 判断散落各处。
func outputWriter(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

func errorWriter(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}

// printUsage 输出顶层简版帮助。
func printUsage(w io.Writer) {
	fmt.Fprintln(w, "请指定子命令，例如: ai-sync scan")
}

// printScanResult 负责把扫描结果渲染为统一表格。
// verbose 为 true 时，在表格下方追加每个项目的详细配置项列表。
func printScanResult(w io.Writer, result *usecase.ScanResult, verbose bool) {
	if len(result.Tools) == 0 {
		return
	}

	// 第一步：渲染精简表格
	tbl := &output.Table{
		Columns: []output.Column{
			{Title: "项目"},
			{Title: "类型"},
			{Title: "配置目录"},
			{Title: "状态"},
			{Title: "可同步项"},
		},
	}
	for _, item := range result.Tools {
		projectName := displayScanSummaryTitle(item)
		toolType := displayToolName(item.Tool)
		configCount := ""
		if item.ConfigCount > 0 {
			configCount = fmt.Sprintf("%d 项", item.ConfigCount)
		}
		tbl.Rows = append(tbl.Rows, output.Row{
			Cells: []string{projectName, toolType, item.Path, item.ResultText, configCount},
		})
	}
	fmt.Fprint(w, tbl.Render())

	// 第二步：verbose 模式追加详细配置项
	if verbose {
		for _, item := range result.Tools {
			if len(item.Items) == 0 {
				continue
			}
			fmt.Fprintln(w)
			fmt.Fprintf(w, "%s 详细配置:\n", displayScanSummaryTitle(item))

			grouped := map[string][]usecase.ToolConfigItem{}
			groupOrder := []string{}
			for _, configItem := range item.Items {
				if _, exists := grouped[configItem.Group]; !exists {
					groupOrder = append(groupOrder, configItem.Group)
				}
				grouped[configItem.Group] = append(grouped[configItem.Group], configItem)
			}
			preferredOrder := []string{"关键配置", "扩展内容", "其他内容"}
			slices.SortStableFunc(groupOrder, func(a, b string) int {
				return slices.Index(preferredOrder, a) - slices.Index(preferredOrder, b)
			})

			for _, group := range groupOrder {
				fmt.Fprintf(w, "  %s:\n", group)
				for _, configItem := range grouped[group] {
					fmt.Fprintf(w, "    - %s: %s\n", configItem.Label, configItem.RelativePath)
				}
			}
		}
	}
}

// printScanRuleList 把默认规则、自定义规则和项目规则用表格渲染输出。
func printScanRuleList(w io.Writer, result *usecase.ListScanRulesResult) {
	if strings.TrimSpace(result.App) != "" {
		fmt.Fprintf(w, "%s 规则\n\n", output.HeaderStyle.Render(displayToolName(result.App)))
	}

	// 第一步：默认全局规则
	fmt.Fprintln(w, output.HeaderStyle.Render("默认全局规则:"))
	tbl := &output.Table{
		Columns: []output.Column{{Title: "路径"}},
	}
	for _, item := range result.DefaultGlobalRules {
		tbl.Rows = append(tbl.Rows, output.Row{Cells: []string{item.Path}})
	}
	fmt.Fprint(w, tbl.Render())

	// 第二步：已注册项目扫描模板
	if len(result.RegisteredProjects) > 0 && len(result.ProjectRuleTemplates) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, output.HeaderStyle.Render("已注册项目扫描模板:"))
		tblTemplate := &output.Table{
			Columns: []output.Column{{Title: "路径"}},
		}
		for _, item := range result.ProjectRuleTemplates {
			tblTemplate.Rows = append(tblTemplate.Rows, output.Row{Cells: []string{item.Path}})
		}
		fmt.Fprint(w, tblTemplate.Render())
	}

	// 第三步：自定义规则
	fmt.Fprintln(w)
	fmt.Fprintln(w, output.HeaderStyle.Render("自定义规则:"))
	tblCustom := &output.Table{
		Columns: []output.Column{{Title: "路径"}},
	}
	if len(result.CustomRules) == 0 {
		tblCustom.Rows = append(tblCustom.Rows, output.Row{Cells: []string{"暂无"}})
	} else {
		for _, item := range result.CustomRules {
			tblCustom.Rows = append(tblCustom.Rows, output.Row{Cells: []string{item.Path}})
		}
	}
	fmt.Fprint(w, tblCustom.Render())

	// 第四步：已注册项目
	fmt.Fprintln(w)
	fmt.Fprintln(w, output.HeaderStyle.Render("已注册项目:"))
	tblProjects := &output.Table{
		Columns: []output.Column{{Title: "名称"}, {Title: "路径"}},
	}
	if len(result.RegisteredProjects) == 0 {
		tblProjects.Rows = append(tblProjects.Rows, output.Row{Cells: []string{"暂无", ""}})
	} else {
		for _, item := range result.RegisteredProjects {
			tblProjects.Rows = append(tblProjects.Rows, output.Row{Cells: []string{item.Name, item.Path}})
		}
	}
	fmt.Fprint(w, tblProjects.Render())
}

// 其余 print* 函数都是 cobra 层的纯展示逻辑，不承载业务判断。
func printCreatedSnapshot(w io.Writer, result *usecase.SnapshotSummary) {
	fmt.Fprintf(w, "快照已创建: %s\n", result.ID)
	fmt.Fprintf(w, "名称: %s\n", result.Name)
	fmt.Fprintf(w, "文件数: %d\n", result.FileCount)
	fmt.Fprintf(w, "大小: %d 字节\n", result.Size)
}

func printSnapshotList(w io.Writer, result *usecase.ListSnapshotsResult) {
	if len(result.Items) == 0 {
		fmt.Fprintln(w, "暂无本地快照")
		return
	}

	tbl := &output.Table{
		Columns: []output.Column{
			{Title: "ID"},
			{Title: "名称"},
			{Title: "项目"},
			{Title: "说明"},
			{Title: "文件数"},
			{Title: "创建时间"},
		},
		Footer: fmt.Sprintf("共 %d 条快照", result.Total),
	}
	for _, item := range result.Items {
		tbl.Rows = append(tbl.Rows, output.Row{
			Cells: []string{
				item.ID,
				item.Name,
				item.Project,
				item.Message,
				fmt.Sprintf("%d", item.FileCount),
				item.CreatedAt.Format("2006-01-02 15:04"),
			},
		})
	}
	fmt.Fprint(w, tbl.Render())
}

func printError(w io.Writer, err error) {
	var userErr *usecase.UserError
	if errors.As(err, &userErr) {
		fmt.Fprintln(w, userErr.Message)
		if strings.TrimSpace(userErr.Suggestion) != "" {
			fmt.Fprintln(w, userErr.Suggestion)
		}
		return
	}

	fmt.Fprintln(w, err.Error())
}

func printConfigEntries(w io.Writer, result *usecase.GetConfigResult) {
	fmt.Fprintf(w, "%s\n\n", result.AbsolutePath)
	fmt.Fprintln(w, "类型    名称                 路径")
	for _, entry := range result.Entries {
		entryType := "文件"
		path := entry.RelativePath
		if entry.IsDir {
			entryType = "目录"
			if !strings.HasSuffix(path, "/") && !strings.HasSuffix(path, "\\") {
				path += string(filepathSeparator())
			}
		}
		fmt.Fprintf(w, "%s    %s    %s\n", entryType, entry.Name, path)
	}
}

func printConfigFile(w io.Writer, result *usecase.GetConfigResult) {
	fmt.Fprintf(w, "%s\n\n", result.AbsolutePath)
	fmt.Fprintln(w, result.Content)
}

// displayToolName 统一工具名的人类可读展示。
func displayToolName(name string) string {
	switch name {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	default:
		return strings.Title(name)
	}
}

// displayScanSummaryTitle 根据全局/项目作用域生成结果标题。
func displayScanSummaryTitle(item usecase.ToolSummary) string {
	if item.Scope == "project" {
		projectName := strings.TrimSpace(item.ProjectName)
		if projectName == "" && strings.TrimSpace(item.Path) != "" {
			projectName = filepath.Base(item.Path)
		}
		if projectName == "" {
			projectName = "未命名项目"
		}
		return fmt.Sprintf("%s（%s 项目）", projectName, displayToolName(item.Tool))
	}
	return displayToolName(item.Tool) + "（全局）"
}

// filepathSeparator 包一层便于测试时统一处理平台差异。
func filepathSeparator() rune {
	return filepath.Separator
}
