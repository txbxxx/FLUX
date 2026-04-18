package cobra

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	spcobra "github.com/spf13/cobra"

	typesSync "flux/internal/types/sync"
)

// newSyncCommand creates the sync command group.
func newSyncCommand(deps Dependencies) *spcobra.Command {
	command := &spcobra.Command{
		Use:   "sync",
		Short: "同步配置到远端仓库",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			fmt.Fprintln(cmd.ErrOrStderr(), "请指定 sync 操作，例如: fl sync push")
			return errCommandHandled
		},
	}

	command.AddCommand(
		newSyncPushCommand(deps),
		newSyncPullCommand(deps),
		newSyncStatusCommand(deps),
	)

	return command
}

func newSyncPushCommand(deps Dependencies) *spcobra.Command {
	var project string
	var all bool

	command := &spcobra.Command{
		Use:   "push",
		Short: "推送配置到远端仓库",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			result, err := deps.Workflow.SyncPush(cmd.Context(), typesSync.SyncPushInput{
				Project: project,
				All:     all,
			})
			if err != nil {
				return err
			}

			printSyncPushResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVarP(&project, "project", "p", "", "项目名称")
	flags.BoolVar(&all, "all", false, "推送所有项目")

	return command
}

func newSyncPullCommand(deps Dependencies) *spcobra.Command {
	var project string
	var all bool
	var verbose bool

	command := &spcobra.Command{
		Use:   "pull",
		Short: "从远端拉取配置",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			result, err := deps.Workflow.SyncPull(cmd.Context(), typesSync.SyncPullInput{
				Project: project,
				All:     all,
			})
			if err != nil {
				return err
			}

			printSyncPullResult(cmd.OutOrStdout(), result, verbose)
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVarP(&project, "project", "p", "", "项目名称")
	flags.BoolVar(&all, "all", false, "拉取所有项目")
	flags.BoolVarP(&verbose, "verbose", "v", false, "显示完整的自动同步文件列表")

	return command
}

func newSyncStatusCommand(deps Dependencies) *spcobra.Command {
	var project string

	command := &spcobra.Command{
		Use:   "status",
		Short: "查看同步状态",
		RunE: func(cmd *spcobra.Command, _ []string) error {
			result, err := deps.Workflow.SyncStatus(cmd.Context(), typesSync.SyncStatusInput{
				Project: project,
			})
			if err != nil {
				return err
			}

			printSyncStatusResult(cmd.OutOrStdout(), result)
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVarP(&project, "project", "p", "", "项目名称")

	return command
}

func printSyncPushResult(w io.Writer, result *typesSync.SyncPushResult) {
	if result.Success {
		if result.FilesPushed == 0 && result.Message != "" {
			fmt.Fprintf(w, "无需推送\n\n")
			fmt.Fprintf(w, "  项目:   %s\n", result.Project)
			fmt.Fprintf(w, "  说明:   %s\n", result.Message)
			fmt.Fprintf(w, "  远端:   %s\n", result.RemoteURL)
		} else {
			fmt.Fprintf(w, "推送成功\n\n")
			fmt.Fprintf(w, "  项目:   %s\n", result.Project)
			fmt.Fprintf(w, "  文件:   %d 个已推送\n", result.FilesPushed)
			if result.CommitHash != "" {
				fmt.Fprintf(w, "  提交:   %s\n", result.CommitHash[:8])
			}
			fmt.Fprintf(w, "  远端:   %s\n", result.RemoteURL)
		}
	} else {
		fmt.Fprintf(w, "推送失败: %s\n", result.Error)
	}
}

func printSyncPullResult(w io.Writer, result *typesSync.SyncPullResult, verbose bool) {
	if result.Cancelled {
		fmt.Fprintf(w, "已取消拉取\n\n")
		fmt.Fprintf(w, "  项目:   %s\n", result.Project)
	} else if result.Success {
		fmt.Fprintf(w, "拉取成功\n\n")
		fmt.Fprintf(w, "  项目:   %s\n", result.Project)
		fmt.Fprintf(w, "  文件:   %d 个已更新\n", result.FilesUpdated)

		if len(result.AutoResolved) > 0 {
			printAutoResolvedSummary(w, result.AutoResolved, verbose)
		}
	} else if result.HasConflicts {
		fmt.Fprintf(w, "拉取完成，发现冲突\n\n")
		fmt.Fprintf(w, "  项目:   %s\n", result.Project)
		fmt.Fprintf(w, "  冲突:   %d 个文件需要处理\n\n", result.ConflictCount)
		for i, conflict := range result.Conflicts {
			fmt.Fprintf(w, "  [%d] %s\n", i+1, conflict.Path)
			fmt.Fprintf(w, "      类型: %s\n", conflict.ConflictType)
			fmt.Fprintf(w, "      本地: %s\n", conflict.LocalSummary)
			fmt.Fprintf(w, "      远端: %s\n", conflict.RemoteSummary)
		}
		fmt.Fprintln(w)
		fmt.Fprintln(w, "请手动解决冲突后重新执行 fl sync pull")
	} else {
		fmt.Fprintf(w, "拉取失败: %s\n", result.Error)
	}
}

// printAutoResolvedSummary prints auto-resolved files with directory-level aggregation.
// When verbose is true, prints every file individually; otherwise aggregates by top-level directory.
func printAutoResolvedSummary(w io.Writer, autoResolved []typesSync.AutoResolvedInfo, verbose bool) {
	if verbose {
		fmt.Fprintf(w, "\n  自动同步（无冲突）：\n")
		for _, ar := range autoResolved {
			if ar.Resolution == "remote_added" {
				fmt.Fprintf(w, "    - %s — 远端新增文件，已同步到本地\n", ar.Path)
			} else {
				fmt.Fprintf(w, "    - %s — 本地独有文件，已保留\n", ar.Path)
			}
		}
		return
	}

	// Aggregate by top-level directory and resolution type.
	type aggEntry struct {
		resolution string
		count      int
	}
	aggMap := make(map[string]*aggEntry) // key: "dir/|resolution"
	var rootFiles []typesSync.AutoResolvedInfo

	for _, ar := range autoResolved {
		dir := topDir(ar.Path)
		if dir == "" {
			rootFiles = append(rootFiles, ar)
			continue
		}
		key := dir + "|" + ar.Resolution
		if e, ok := aggMap[key]; ok {
			e.count++
		} else {
			aggMap[key] = &aggEntry{resolution: ar.Resolution, count: 1}
		}
	}

	// Group entries by directory for O(n) lookup instead of nested loop.
	type dirStats struct {
		remoteAdded int
		localOnly   int
	}
	byDir := make(map[string]*dirStats)
	var dirs []string
	dirSet := make(map[string]bool)

	for key, e := range aggMap {
		// key format: "dir/|resolution"
		sepIdx := strings.LastIndex(key, "|")
		d := key[:sepIdx]
		if !dirSet[d] {
			dirSet[d] = true
			dirs = append(dirs, d)
			byDir[d] = &dirStats{}
		}
		switch e.resolution {
		case "remote_added":
			byDir[d].remoteAdded = e.count
		case "local_only":
			byDir[d].localOnly = e.count
		}
	}
	sort.Strings(dirs)

	fmt.Fprintf(w, "\n  自动同步（无冲突）：\n")

	for _, d := range dirs {
		st := byDir[d]
		var parts []string
		if st.remoteAdded > 0 {
			parts = append(parts, fmt.Sprintf("%d 个远端新增", st.remoteAdded))
		}
		if st.localOnly > 0 {
			parts = append(parts, fmt.Sprintf("%d 个本地独有", st.localOnly))
		}
		fmt.Fprintf(w, "    %s — %s\n", d, strings.Join(parts, "，"))
	}

	// Print root-level files individually
	for _, ar := range rootFiles {
		if ar.Resolution == "remote_added" {
			fmt.Fprintf(w, "    %s — 远端新增文件，已同步到本地\n", ar.Path)
		} else {
			fmt.Fprintf(w, "    %s — 本地独有文件，已保留\n", ar.Path)
		}
	}

	fmt.Fprintf(w, "    (使用 --verbose 查看完整列表)\n")
}

// topDir extracts the top-level directory from a file path.
// "market/aaa.md" → "market/", "settings.json" → "".
func topDir(p string) string {
	p = filepath.ToSlash(p)
	idx := strings.Index(p, "/")
	if idx < 0 {
		return ""
	}
	return p[:idx+1]
}

func printSyncStatusResult(w io.Writer, result *typesSync.SyncStatusResult) {
	fmt.Fprintf(w, "同步状态\n\n")
	fmt.Fprintf(w, "  项目:   %s\n", result.Project)
	fmt.Fprintf(w, "  远端:   %s\n", result.RemoteURL)
	fmt.Fprintf(w, "  分支:   %s\n", result.Branch)
	if result.IsSynced {
		fmt.Fprintf(w, "  状态:   已同步\n")
	} else {
		fmt.Fprintf(w, "  状态:   未同步 (领先 %d / 落后 %d)\n", result.AheadCount, result.BehindCount)
	}
	if result.LastSynced != "" {
		fmt.Fprintf(w, "  上次:   %s\n", result.LastSynced)
	}
}
