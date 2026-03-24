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

type Workflow interface {
	Scan(ctx context.Context) (*usecase.ScanResult, error)
	CreateSnapshot(ctx context.Context, input usecase.CreateSnapshotInput) (*usecase.SnapshotSummary, error)
	ListSnapshots(ctx context.Context, input usecase.ListSnapshotsInput) (*usecase.ListSnapshotsResult, error)
	GetConfig(ctx context.Context, input usecase.GetConfigInput) (*usecase.GetConfigResult, error)
	SaveConfig(ctx context.Context, input usecase.SaveConfigInput) error
}

type TUIRunner interface {
	Run(ctx context.Context) error
}

type EditorRunner interface {
	Run(ctx context.Context, result *usecase.GetConfigResult, save func(string) error) error
}

type Dependencies struct {
	Workflow Workflow
	TUI      TUIRunner
	Editor   EditorRunner
	DataDir  string
	Out      io.Writer
	Err      io.Writer
	Context  context.Context
}

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

func commandContext(deps Dependencies) context.Context {
	if deps.Context != nil {
		return deps.Context
	}
	return context.Background()
}

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

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "用法: ai-sync <scan|get|snapshot|tui>")
}

func printScanResult(w io.Writer, result *usecase.ScanResult) {
	for index, item := range result.Tools {
		if index > 0 {
			fmt.Fprintln(w)
		}

		fmt.Fprintln(w, displayToolName(item.Tool))
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

func printCreatedSnapshot(w io.Writer, result *usecase.SnapshotSummary) {
	fmt.Fprintf(w, "created snapshot: %s\n", result.ID)
	fmt.Fprintf(w, "name: %s\n", result.Name)
	fmt.Fprintf(w, "files: %d\n", result.FileCount)
	fmt.Fprintf(w, "size: %d bytes\n", result.Size)
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

func filepathSeparator() rune {
	return filepath.Separator
}
