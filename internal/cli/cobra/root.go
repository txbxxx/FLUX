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
	GetConfig(ctx context.Context, input usecase.GetConfigInput) (*usecase.GetConfigResult, error)
	SaveConfig(ctx context.Context, input usecase.SaveConfigInput) error
}

// TUIRunner / EditorRunner 抽象终端交互能力，避免 cobra 直接依赖具体实现。
type TUIRunner interface {
	Run(ctx context.Context) error
}

type EditorRunner interface {
	Run(ctx context.Context, result *usecase.GetConfigResult, save func(string) error) error
}

// Dependencies 集中描述 root command 所需依赖。
type Dependencies struct {
	Workflow Workflow
	TUI      TUIRunner
	Editor   EditorRunner
	DataDir  string
	Out      io.Writer
	Err      io.Writer
	Context  context.Context
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
	)

	return root
}

// Execute 负责执行命令，并把“已自行输出帮助”的情况转换为稳定退出码。
func Execute(deps Dependencies, args []string) int {
	cmd := NewRootCommand(deps)
	cmd.SetArgs(args)

	if err := cmd.ExecuteContext(commandContext(deps)); err != nil {
		if !errors.Is(err, errCommandHandled) {
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

// printScanResult 负责把扫描结果整理成面向终端的分组文本。
func printScanResult(w io.Writer, result *usecase.ScanResult) {
	for index, item := range result.Tools {
		if index > 0 {
			fmt.Fprintln(w)
		}

		fmt.Fprintln(w, displayScanSummaryTitle(item))
		fmt.Fprintf(w, "  检测结果: %s\n", item.ResultText)
		if strings.TrimSpace(item.Path) != "" {
			fmt.Fprintf(w, "  配置目录: %s\n", item.Path)
		}

		if item.ConfigCount > 0 {
			fmt.Fprintf(w, "  可同步项: %d 项\n", item.ConfigCount)
		}
		if strings.TrimSpace(item.Reason) != "" {
			fmt.Fprintf(w, "  原因: %s\n", item.Reason)
		}

		if len(item.Items) == 0 {
			continue
		}

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
			fmt.Fprintf(w, "\n  %s:\n", group)
			for _, configItem := range grouped[group] {
				fmt.Fprintf(w, "    - %s: %s\n", configItem.Label, configItem.RelativePath)
			}
		}
	}
}

// printScanRuleList 把默认规则、自定义规则和项目规则合并成稳定输出。
func printScanRuleList(w io.Writer, result *usecase.ListScanRulesResult) {
	if strings.TrimSpace(result.App) != "" {
		fmt.Fprintf(w, "%s 规则\n\n", displayToolName(result.App))
	}

	fmt.Fprintln(w, "默认全局规则:")
	for _, item := range result.DefaultGlobalRules {
		fmt.Fprintf(w, "  - %s\n", item.Path)
	}

	if len(result.RegisteredProjects) > 0 && len(result.ProjectRuleTemplates) > 0 {
		fmt.Fprintln(w, "\n已注册项目扫描模板:")
		for _, item := range result.ProjectRuleTemplates {
			fmt.Fprintf(w, "  - %s\n", item.Path)
		}
	}

	fmt.Fprintln(w, "\n自定义规则:")
	if len(result.CustomRules) == 0 {
		fmt.Fprintln(w, "  - 暂无")
	} else {
		for _, item := range result.CustomRules {
			fmt.Fprintf(w, "  - %s\n", item.Path)
		}
	}

	fmt.Fprintln(w, "\n已注册项目:")
	if len(result.RegisteredProjects) == 0 {
		fmt.Fprintln(w, "  - 暂无")
	} else {
		for _, item := range result.RegisteredProjects {
			fmt.Fprintf(w, "  - %s: %s\n", item.Name, item.Path)
		}
	}
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

	for _, item := range result.Items {
		fmt.Fprintf(w, "%s | %s | %s\n", item.ID, item.Name, item.Message)
	}
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
	fmt.Fprintf(w, "%s > %s\n\n", displayToolName(result.Tool), result.RelativePath)
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
	fmt.Fprintf(w, "%s > %s\n\n", displayToolName(result.Tool), result.RelativePath)
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
