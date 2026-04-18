package usecase

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	pathpkg "path/filepath"
	"strings"
	"time"

	"flux/internal/models"
	"flux/internal/util/diffutil"
	typesSync "flux/internal/types/sync"
	"flux/pkg/git"
	"flux/pkg/logger"

	"go.uber.org/zap"
)

// SyncPush pushes the latest snapshot of a project to its configured remote repository.
//
// 流程：
//  1. 获取远端配置
//  2. 确保 repo 存在（不存在则询问是否 clone）
//  3. fetch 远端最新代码
//  4. 从 SQLite 读取该项目的最新快照
//  5. 把快照文件写入 repo 工作目录
//  6. git diff 展示快照 vs 远端 HEAD 的差异预览
//  7. 用户确认后才 commit + push
func (w *LocalWorkflow) SyncPush(ctx context.Context, input typesSync.SyncPushInput) (*typesSync.SyncPushResult, error) {
	// 第一步：参数校验
	projectName := strings.TrimSpace(input.Project)
	if projectName == "" && !input.All {
		return nil, &UserError{
			Message:    "推送失败：请指定项目名称",
			Suggestion: "使用 --project 指定项目，或 --all 推送所有项目",
		}
	}

	if w.remoteConfigs == nil {
		return nil, &UserError{
			Message:    "推送失败：远端配置服务不可用",
			Suggestion: "请先执行 fl remote add <url>",
		}
	}

	// 第二步：获取远端配置
	configs, err := w.remoteConfigs.ListRemoteConfigs()
	if err != nil || len(configs) == 0 {
		return nil, &UserError{
			Message:    "推送失败：未配置远端仓库",
			Suggestion: "请先执行 fl remote add <url> 添加远端仓库",
		}
	}

	var remoteConfig *models.RemoteConfig
	for _, cfg := range configs {
		if cfg.Status == models.StatusActive {
			remoteConfig = cfg
			break
		}
	}
	if remoteConfig == nil {
		return nil, &UserError{
			Message:    "推送失败：没有可用的活跃远端配置",
			Suggestion: "请先执行 fl remote add 添加一个有效的远端仓库",
		}
	}

	// 第三步：确保 repo 存在（不存在则询问是否 clone）
	dataDir := w.dataDir
	repoPath := pathpkg.Join(dataDir, "repos", projectName)

	continuePush, err := w.ensureRepoExists(
		ctx,
		repoPath,
		remoteConfig.URL,
		convertAuthFromModel(&remoteConfig.Auth),
		remoteConfig.Branch,
		"是否从远程拉取现有配置？",
	)
	if err != nil {
		return nil, err
	}
	if !continuePush {
		return &typesSync.SyncPushResult{
			Success:     true,
			Project:     projectName,
			FilesPushed: 0,
			Message:     "已取消推送",
		}, nil
	}
	fmt.Println("远端配置拉取成功")

	// 第四步：fetch 远端最新代码
	branch := remoteConfig.Branch
	if branch == "" {
		branch = "main"
	}
	gitClient := git.NewGitClient()
	fetchFailed := false
	fetchResult, fetchErr := gitClient.Fetch(ctx, &git.FetchOptions{
		Path:   repoPath,
		Auth:   convertAuthFromModel(&remoteConfig.Auth),
		Remote: "origin",
	})
	if fetchErr != nil && !strings.Contains(fetchErr.Error(), "already up-to-date") {
		logger.Warn("fetch 远端失败，跳过",
			zap.String("project", projectName),
			zap.Error(fetchErr),
		)
		fetchFailed = true
		_ = fetchResult
	}

	// 第五步：从 SQLite 读取该项目的最新快照
	if w.snapshots == nil {
		return nil, &UserError{
			Message:    "推送失败：快照服务不可用",
			Suggestion: "请重新启动程序",
		}
	}
	items, listErr := w.snapshots.ListSnapshots(0, 0)
	if listErr != nil {
		return nil, &UserError{
			Message:    "推送失败：无法读取快照列表",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        listErr,
		}
	}
	var snapshotID string
	for _, item := range items {
		if item.Project == projectName {
			snapshotID = fmt.Sprintf("%d", item.ID)
			break
		}
	}
	if snapshotID == "" {
		return nil, &UserError{
			Message:    "推送失败：项目 \"" + projectName + "\" 没有快照",
			Suggestion: "请先执行 fl snapshot create -p " + projectName + " 创建快照",
		}
	}
	snapshot, err := w.snapshots.GetSnapshot(snapshotID)
	if err != nil {
		return nil, &UserError{
			Message:    "推送失败：无法读取快照数据",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	// 第六步：把快照文件写入 repo 工作目录
	// 先清空项目子目录，确保只保留本次快照的内容
	projectSubDir := pathpkg.Join(repoPath, projectName)
	if exists, _ := pathExists(projectSubDir); exists {
		entries, readErr := os.ReadDir(projectSubDir)
		if readErr == nil {
			for _, entry := range entries {
				entryPath := pathpkg.Join(projectSubDir, entry.Name())
				if err := os.RemoveAll(entryPath); err != nil {
					logger.Warn("清理旧文件失败，跳过",
						zap.String("path", entryPath),
						zap.Error(err))
				}
			}
		}
	}
	filesWritten := 0
	for _, file := range snapshot.Files {
		if len(file.Content) == 0 {
			continue
		}
		targetPath := pathpkg.Join(repoPath, projectName, file.Path)
		dir := pathpkg.Dir(targetPath)
		if err := mkdirAll(dir); err != nil {
			logger.Warn("创建目录失败，跳过文件",
				zap.String("dir", dir),
				zap.String("file", targetPath),
				zap.Error(err))
			continue
		}
		if err := writeFile(targetPath, file.Content); err != nil {
			logger.Warn("写入文件失败，跳过",
				zap.String("file", targetPath),
				zap.Error(err))
			continue
		}
		filesWritten++
	}

	// 第七步：git diff 展示快照 vs 远端 HEAD 的差异预览
	fmt.Println()
	fmt.Printf("快照「%s」vs 远端 origin/%s 差异预览：\n", snapshot.Name, branch)
	if fetchFailed {
		fmt.Println("  [警告] fetch 失败，差异可能不完整（远端引用不可用）")
	}
	fmt.Println(strings.Repeat("-", 60))

	diffResult, diffErr := gitClient.Diff(&git.DiffOptions{
		Path:    repoPath,
		Against: "origin/" + branch,
	})

	added, modified, deleted := 0, 0, 0
	if diffErr == nil && diffResult != nil {
		added = diffResult.Added
		modified = diffResult.Modified
		deleted = diffResult.Deleted

		// 分类显示前几个文件
		showFile := func(label, path string) { fmt.Printf("  [%s] %s\n", label, path) }
		showMore := func(label string, n int) { fmt.Printf("  ... 等 %d 个%s文件\n", n, label) }

		// 粗略按比例分配显示：新增/修改/删除（使用分类 slices，不再依赖文件顺序）
		total := added + modified + deleted
		if total > 15 {
			showAdded := min(added, 5)
			showModified := min(modified, 5)
			showDeleted := min(deleted, 5)
			for i := 0; i < showAdded; i++ {
				showFile("新增", diffResult.AddedFiles[i])
			}
			for i := 0; i < showModified; i++ {
				showFile("修改", diffResult.ModifiedFiles[i])
			}
			for i := 0; i < showDeleted; i++ {
				showFile("删除", diffResult.DeletedFiles[i])
			}
			if added > showAdded {
				showMore("新增", added-showAdded)
			}
			if modified > showModified {
				showMore("修改", modified-showModified)
			}
			if deleted > showDeleted {
				showMore("删除", deleted-showDeleted)
			}
		} else {
			// 文件少，逐类输出（直接使用分类 slices，不依赖文件顺序）
			for _, path := range diffResult.AddedFiles {
				fmt.Printf("  [新增] %s\n", path)
			}
			for _, path := range diffResult.ModifiedFiles {
				fmt.Printf("  [修改] %s\n", path)
			}
			for _, path := range diffResult.DeletedFiles {
				fmt.Printf("  [删除] %s\n", path)
			}
		}
	} else {
		fmt.Println("  无文件变更（快照与远端一致）")
	}

	if added == 0 && modified == 0 && deleted == 0 && diffResult != nil {
		fmt.Println("  无文件变更")
	}
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println()

	// 第八步：询问用户确认
	fmt.Printf("快照「%s」：新增 %d，修改 %d，删除 %d\n",
		snapshot.Name, added, modified, deleted)
	fmt.Println("警告：以快照内容覆盖远端仓库，确认推送？[Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	confirmInput, _ := reader.ReadString('\n')
	confirmInput = strings.TrimSpace(strings.ToLower(confirmInput))
	if confirmInput != "" && confirmInput != "y" && confirmInput != "yes" {
		return &typesSync.SyncPushResult{
			Success:     true,
			Project:     projectName,
			FilesPushed: 0,
			Message:     "已取消推送",
		}, nil
	}
	fmt.Println()

	// 第九步：git add + commit
	commitMsg := fmt.Sprintf("Snapshot: %s\nID: %d\nProject: %s\nFiles: %d",
		snapshot.Name, snapshot.ID, snapshot.Project, filesWritten)
	commitResult, commitErr := gitClient.Commit(ctx, &git.CommitOptions{
		Path:    repoPath,
		Message: commitMsg,
		All:     true,
	})
	if commitErr != nil {
		errMsg := commitErr.Error()
		if strings.Contains(errMsg, "nothing to commit") || strings.Contains(errMsg, "empty commit") {
			return &typesSync.SyncPushResult{
				Success:     true,
				Project:     projectName,
				FilesPushed: 0,
				RemoteURL:   remoteConfig.URL,
				Message:     "快照与远端一致，无需推送",
			}, nil
		}
		return nil, &UserError{
			Message:    fmt.Sprintf("推送失败：Git commit 失败（远端: %s）", remoteConfig.URL),
			Suggestion: "请检查工作目录状态",
			Err:        commitErr,
		}
	}

	// 第十步：git push
	_, pushErr := gitClient.Push(ctx, &git.PushOptions{
		Path:       repoPath,
		Auth:       convertAuthFromModel(&remoteConfig.Auth),
		RemoteName: "origin",
		Branch:     branch,
	})
	if pushErr != nil {
		logger.Error("推送失败",
			zap.String("project", projectName),
			zap.String("remote_url", remoteConfig.URL),
			zap.Error(pushErr),
		)
		return nil, &UserError{
			Message:    fmt.Sprintf("推送失败：无法推送到远端仓库（%s）", remoteConfig.URL),
			Suggestion: "请检查网络连接和仓库权限",
			Err:        pushErr,
		}
	}

	commitHash := ""
	if commitResult != nil {
		commitHash = commitResult.CommitHash
	}

	return &typesSync.SyncPushResult{
		Success:     true,
		Project:     projectName,
		FilesPushed: filesWritten,
		CommitHash:  commitHash,
		RemoteURL:   remoteConfig.URL,
		Message:     "推送成功",
	}, nil
}

// SyncPull pulls the latest remote state for a project into the local repository.
//
// 流程：
//  1. 获取远端配置
//  2. 确保 repo 存在（不存在则询问是否 clone）
//  3. fetch 远端最新代码
//  4. 检测本地和远端的冲突（三方对比：本地快照 vs 本地 git vs 远端 git）
//  5. 有冲突则返回冲突信息，无冲突则 reset 到远端版本并更新 SQLite
func (w *LocalWorkflow) SyncPull(ctx context.Context, input typesSync.SyncPullInput) (*typesSync.SyncPullResult, error) {
	// 第一步：参数校验
	projectName := strings.TrimSpace(input.Project)
	if projectName == "" {
		return nil, &UserError{
			Message:    "拉取失败：请指定项目名称",
			Suggestion: "使用 --project 指定项目",
		}
	}

	if w.remoteConfigs == nil {
		return nil, &UserError{
			Message:    "拉取失败：远端配置服务不可用",
			Suggestion: "请先执行 fl remote add <url>",
		}
	}

	// 第二步：获取远端配置
	configs, err := w.remoteConfigs.ListRemoteConfigs()
	if err != nil || len(configs) == 0 {
		return nil, &UserError{
			Message:    "拉取失败：未配置远端仓库",
			Suggestion: "请先执行 fl remote add <url>",
		}
	}

	var remoteConfig *models.RemoteConfig
	for _, cfg := range configs {
		if cfg.Status == models.StatusActive {
			remoteConfig = cfg
			break
		}
	}
	if remoteConfig == nil {
		return nil, &UserError{
			Message:    "拉取失败：没有可用的活跃远端配置",
			Suggestion: "请先执行 fl remote add 添加一个有效的远端仓库",
		}
	}

	// 第三步：确保仓库存在
	dataDir := w.dataDir
	repoPath := pathpkg.Join(dataDir, "repos", projectName)

	continuePull, err := w.ensureRepoExists(
		ctx,
		repoPath,
		remoteConfig.URL,
		convertAuthFromModel(&remoteConfig.Auth),
		remoteConfig.Branch,
		"是否从远程拉取现有配置到本地？",
	)
	if err != nil {
		return nil, err
	}
	if !continuePull {
		return &typesSync.SyncPullResult{
			Success:   true,
			Cancelled: true,
			Project:   projectName,
		}, nil
	}
	fmt.Printf("远端配置已拉取到本地（%s）\n\n", repoPath)

	// 第四步：记录本地 HEAD，然后 fetch 远端更新
	gitClient := git.NewGitClient()
	localHash, _ := gitClient.GetHeadHash(repoPath)

	branch := remoteConfig.Branch
	if branch == "" {
		branch = "main"
	}

	_, err = gitClient.Fetch(ctx, &git.FetchOptions{
		Path:   repoPath,
		Auth:   convertAuthFromModel(&remoteConfig.Auth),
		Remote: "origin",
	})
	if err != nil {
		// fetch 失败可能是网络问题
		if strings.Contains(err.Error(), "already up-to-date") {
			return &typesSync.SyncPullResult{
				Success:      true,
				Project:      projectName,
				FilesUpdated: 0,
			}, nil
		}

		// 解析错误类型并返回具体信息
		errorMsg, suggestion := parseGitError(err.Error(), remoteConfig.URL)
		logger.Error("拉取远端更新失败",
			zap.String("project", projectName),
			zap.String("remote_url", remoteConfig.URL),
			zap.Error(err),
		)
		return nil, &UserError{
			Message:    errorMsg,
			Suggestion: suggestion,
			Err:        err,
		}
	}

	// 第五步：获取远端 HEAD 并对比变更
	projectPrefix := projectName + "/"

	// 获取本地最新快照（提前获取，供 git 无变更时快照对比使用）
	var localSnapshot *models.Snapshot
	if w.snapshots != nil {
		items, listErr := w.snapshots.ListSnapshots(0, 0)
		if listErr == nil {
			for _, item := range items {
				if item.Project == projectName {
					snap, getErr := w.snapshots.GetSnapshot(fmt.Sprintf("%d", item.ID))
					if getErr == nil {
						localSnapshot = snap
					}
					break
				}
			}
		}
	}

	remoteHash, _ := gitClient.GetRemoteHeadHash(repoPath, "origin", branch)
	if localHash == "" || remoteHash == "" || localHash == remoteHash {
		// git 层面没有变更，但需要检查本地快照是否与远端一致
		if localSnapshot == nil {
			// 没有本地快照，无冲突
			return &typesSync.SyncPullResult{
				Success:      true,
				Project:      projectName,
				FilesUpdated: 0,
			}, nil
		}

		// 比较本地快照与远端 HEAD（clone 后工作目录就是 remote HEAD）
		// 使用新的 classifySnapshotDifferences 函数，区分冲突和自动解决
		snapshotConflicts, autoResolved, cmpErr := w.classifySnapshotDifferences(
			ctx, repoPath, remoteHash, localSnapshot, projectPrefix)
		if cmpErr != nil {
			logger.Warn("快照对比失败", zap.Error(cmpErr))
		}

		if len(snapshotConflicts) > 0 {
			fmt.Println()
			fmt.Printf("检测到 %d 个冲突文件：\n", len(snapshotConflicts))
			showLimit := 10
			for i := 0; i < len(snapshotConflicts) && i < showLimit; i++ {
				c := snapshotConflicts[i]
				fmt.Printf("  - %s  (%s vs %s)\n", c.Path, c.LocalSummary, c.RemoteSummary)
			}
			if len(snapshotConflicts) > showLimit {
				fmt.Printf("  ... 等 %d 个文件\n", len(snapshotConflicts)-showLimit)
			}
			fmt.Println()

			// 逐文件交互解决
			var keepLocalFiles, useRemoteFiles []string
			cancelled, resolvedKeepLocal, resolvedUseRemote, err := w.resolveConflicts(ctx, snapshotConflicts, repoPath, remoteHash, localHash, localSnapshot, projectPrefix)
			if err != nil {
				return nil, err
			}
			if cancelled {
				return &typesSync.SyncPullResult{
					Success:      true,
					Cancelled:    true,
					Project:      projectName,
					FilesUpdated: 0,
				}, nil
			}
			keepLocalFiles = resolvedKeepLocal
			useRemoteFiles = resolvedUseRemote

			// 应用用户选择：使用远端的文件从工作目录读取，使用本地的文件从快照恢复
			projectBasePath, projectToolType := w.resolveProjectInfo(projectName)
			if len(useRemoteFiles) > 0 {
				fmt.Println("正在将远端配置同步到本地快照...")
				if err := w.applyRemoteToSnapshot(ctx, repoPath, remoteHash, localSnapshot, projectPrefix, projectBasePath, projectToolType); err != nil {
					logger.Warn("同步远端到快照失败", zap.Error(err))
				}
			}
			// 恢复用户选择保留本地的文件到工作目录
			for _, relativePath := range keepLocalFiles {
				if localSnapshot == nil {
					continue
				}
				for _, f := range localSnapshot.Files {
					if f.Path == relativePath {
						targetPath := pathpkg.Join(repoPath, projectPrefix, relativePath)
						dir := pathpkg.Dir(targetPath)
						if err := os.MkdirAll(dir, 0755); err != nil {
							logger.Warn("创建目录失败", zap.String("dir", dir), zap.Error(err))
							continue
						}
						if err := os.WriteFile(targetPath, f.Content, 0644); err != nil {
							logger.Warn("恢复本地文件失败", zap.String("file", relativePath), zap.Error(err))
						}
						break
					}
				}
			}
			fmt.Println("快照已更新。")
			return &typesSync.SyncPullResult{
				Success:       true,
				Project:       projectName,
				FilesUpdated:  len(useRemoteFiles) + len(keepLocalFiles),
				AutoResolved:  autoResolved,
			}, nil
		}

		return &typesSync.SyncPullResult{
			Success:       true,
			Project:       projectName,
			FilesUpdated:  0,
			AutoResolved:  autoResolved,
		}, nil
	}

	changedFiles, err := gitClient.GetChangedFiles(repoPath, localHash, remoteHash)
	if err != nil || len(changedFiles) == 0 {
		return &typesSync.SyncPullResult{
			Success:      true,
			Project:      projectName,
			FilesUpdated: 0,
		}, nil
	}

	// 第六步：冲突检测 — 对比本地快照和远端版本的差异
	var conflicts []typesSync.ConflictInfo
	var safeFiles []git.FileDiff

	for _, cf := range changedFiles {
		// 只处理项目子目录下的文件
		if !strings.HasPrefix(cf.Path, projectPrefix) {
			continue
		}

		relativePath := strings.TrimPrefix(cf.Path, projectPrefix)

		// 读取远端版本内容（从 fetch 后的远端 commit）
		remoteContent, remoteErr := gitClient.GetFileContent(ctx, repoPath, cf.Path, remoteHash)
		if remoteErr != nil {
			// 远端文件可能被删除
			if cf.Status == "deleted" {
				safeFiles = append(safeFiles, cf)
				continue
			}
			continue
		}

		// 如果没有本地快照，所有远端变更都是安全的
		if localSnapshot == nil {
			safeFiles = append(safeFiles, cf)
			continue
		}

		// 在本地快照中查找对应文件
		var localContent []byte
		for _, f := range localSnapshot.Files {
			if f.Path == relativePath {
				localContent = f.Content
				break
			}
		}

		// 本地快照中没有这个文件 → 远端新增，安全
		if localContent == nil {
			safeFiles = append(safeFiles, cf)
			continue
		}

		// 本地快照和远端内容相同 → 不是真正的变更（可能是重复 push），安全
		if string(localContent) == string(remoteContent) {
			continue
		}

		// 读取本地 git HEAD 版本（上次 push 的版本）
		localGitContent, localGitErr := gitClient.GetFileContent(ctx, repoPath, cf.Path, localHash)
		if localGitErr != nil {
			// 本地 git 中没有这个文件但快照有 → 本地有修改，远端也有修改 → 冲突
			conflicts = append(conflicts, typesSync.ConflictInfo{
				Path:          relativePath,
				ConflictType:  "both_modified",
				LocalSummary:  fmt.Sprintf("本地快照有 %d 字节", len(localContent)),
				RemoteSummary: fmt.Sprintf("远端更新 %d 字节", len(remoteContent)),
				LocalHash:     "",
				RemoteHash:    remoteHash,
				LocalContent:  localContent,
				RemoteContent: remoteContent,
				IsBinary:      diffutil.IsBinaryContent(localContent) || diffutil.IsBinaryContent(remoteContent),
			})
			continue
		}

		// 三方对比：如果本地快照内容 != 本地 git 内容 → 本地有修改
		// 且远端也有修改 → 冲突
		if string(localContent) != string(localGitContent) {
			conflicts = append(conflicts, typesSync.ConflictInfo{
				Path:          relativePath,
				ConflictType:  "both_modified",
				LocalSummary:  fmt.Sprintf("本地已修改 (%d 字节)", len(localContent)),
				RemoteSummary: fmt.Sprintf("远端已修改 (%d 字节)", len(remoteContent)),
				LocalHash:     localHash,
				RemoteHash:    remoteHash,
				LocalContent:  localContent,
				RemoteContent: remoteContent,
				IsBinary:      diffutil.IsBinaryContent(localContent) || diffutil.IsBinaryContent(remoteContent),
			})
		} else {
			// 本地没有修改，只有远端修改 → 安全合并
			safeFiles = append(safeFiles, cf)
		}
	}

	// 第七步：如果有冲突，进入交互解决
	var keepLocalFiles, useRemoteFiles []string
	if len(conflicts) > 0 {
		cancelled, resolvedKeepLocal, resolvedUseRemote, err := w.resolveConflicts(ctx, conflicts, repoPath, remoteHash, localHash, localSnapshot, projectPrefix)
		if err != nil {
			return nil, err
		}
		if cancelled {
			// 用户取消，保持本地快照不变
			return &typesSync.SyncPullResult{
				Success:      true,
				Cancelled:    true,
				Project:      projectName,
				FilesUpdated: 0,
			}, nil
		}
		keepLocalFiles = resolvedKeepLocal
		useRemoteFiles = resolvedUseRemote
	}

	// 第八步：无冲突 → 将本地分支 hard reset 到远端版本并更新 SQLite
	// 为什么用 ResetToRef 而不是 Pull：go-git 的 Pull 只支持 fast-forward 合并，
	// 当本地和远端有分叉历史时会报 "non-fast-forward update"。
	// 本地仓库只是同步中间层，已在第六步完成冲突检测，可以安全地 reset 到远端。
	_, err = gitClient.ResetToRef(repoPath, remoteHash)
	if err != nil {
		errorMsg, suggestion := parseGitError(err.Error(), remoteConfig.URL)
		logger.Error("Git reset 失败",
			zap.String("project", projectName),
			zap.String("remote_url", remoteConfig.URL),
			zap.String("remote_hash", remoteHash),
			zap.Error(err),
		)
		return nil, &UserError{
			Message:    errorMsg,
			Suggestion: suggestion,
			Err:        err,
		}
	}

	// 第八步（续）：如果用户选择了保留本地版本的文件，
	// 直接使用快照内容恢复工作目录（不经过磁盘读写）
	for _, relativePath := range keepLocalFiles {
		if localSnapshot == nil {
			continue
		}
		for _, f := range localSnapshot.Files {
			if f.Path == relativePath {
				targetPath := pathpkg.Join(repoPath, projectPrefix, relativePath)
				if err := os.WriteFile(targetPath, f.Content, 0644); err != nil {
					logger.Warn("恢复本地文件失败，跳过", zap.String("file", relativePath), zap.Error(err))
				}
				break
			}
		}
	}

	// 第九步：更新 SQLite 中所有变更的文件
	var updatedCount int
	// 构建需要更新的文件映射：path -> newContent
	fileUpdates := make(map[string][]byte)

	// 安全合并的文件：从工作目录读取（ResetToRef 后的远端内容）
	for _, sf := range safeFiles {
		if !strings.HasPrefix(sf.Path, projectPrefix) {
			continue
		}
		content, readErr := os.ReadFile(pathpkg.Join(repoPath, sf.Path))
		if readErr != nil {
			continue
		}
		relativePath := strings.TrimPrefix(sf.Path, projectPrefix)
		fileUpdates[relativePath] = content
	}

	// keep_local 文件：直接用快照内容，不需要读磁盘（#5 修复）
	for _, relativePath := range keepLocalFiles {
		if localSnapshot == nil {
			continue
		}
		for _, f := range localSnapshot.Files {
			if f.Path == relativePath {
				fileUpdates[relativePath] = f.Content
				break
			}
		}
	}

	// 选择"使用远端"或"跳过"的文件：从工作目录读取（ResetToRef 后已是远端版本）
	for _, relativePath := range useRemoteFiles {
		targetPath := pathpkg.Join(repoPath, projectPrefix, relativePath)
		content, readErr := os.ReadFile(targetPath)
		if readErr != nil {
			logger.Warn("读取远端文件失败，跳过", zap.String("file", relativePath), zap.Error(readErr))
			continue
		}
		fileUpdates[relativePath] = content
	}

	// 批量更新 localSnapshot 并写入 SQLite
	// 使用 updateSnapshotFile 确保新增文件也能被正确添加（#6 修复）
	projectBasePath, projectToolType := w.resolveProjectInfo(projectName)
	if localSnapshot != nil && len(fileUpdates) > 0 {
		for path, content := range fileUpdates {
			w.updateSnapshotFile(localSnapshot, path, content, projectBasePath, projectToolType)
		}
		if err := w.snapshots.UpdateSnapshot(localSnapshot); err != nil {
			logger.Error("更新快照失败", zap.Error(err))
		} else {
			updatedCount = len(fileUpdates)
		}
	}

	return &typesSync.SyncPullResult{
		Success:      true,
		Project:      projectName,
		FilesUpdated: updatedCount,
	}, nil
}

// resolveConflicts 交互式解决冲突。
// 返回：是否取消、保留本地的文件列表、使用远端的文件列表、错误。
func (w *LocalWorkflow) resolveConflicts(
	ctx context.Context,
	conflicts []typesSync.ConflictInfo,
	repoPath string,
	remoteHash string,
	localHash string,
	localSnapshot *models.Snapshot,
	projectPrefix string,
) (cancelled bool, keepLocalFiles []string, useRemoteFiles []string, err error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Printf("检测到 %d 个冲突文件：\n", len(conflicts))
	showLimit := 10
	for i := 0; i < len(conflicts) && i < showLimit; i++ {
		c := conflicts[i]
		// Show line-level diff stats if available
		if len(c.LocalContent) > 0 && len(c.RemoteContent) > 0 {
			addLines, delLines := diffutil.CountLineChanges(c.LocalContent, c.RemoteContent)
			fmt.Printf("  - %s  (%s, +%d/-%d 行  vs  %s)\n", c.Path, c.LocalSummary, addLines, delLines, c.RemoteSummary)
		} else {
			fmt.Printf("  - %s  (%s vs %s)\n", c.Path, c.LocalSummary, c.RemoteSummary)
		}
	}
	if len(conflicts) > showLimit {
		fmt.Printf("  ... 等 %d 个文件\n", len(conflicts)-showLimit)
	}
	fmt.Println()

	// 询问是否查看全部
	if len(conflicts) > showLimit {
		fmt.Print("查看全部？[y/N]: ")
		if input, _ := reader.ReadString('\n'); strings.TrimSpace(strings.ToLower(input)) == "y" {
			for _, c := range conflicts {
				fmt.Printf("  - %s  (%s vs %s)\n", c.Path, c.LocalSummary, c.RemoteSummary)
			}
			fmt.Println()
		}
	}

	fmt.Println("请选择处理方式：")
	fmt.Println("  [1] 使用本地版本（保留本地修改）")
	fmt.Println("  [2] 使用远端版本（用远端覆盖）")
	fmt.Println("  [3] 查看差异")
	fmt.Println("  [4] 取消本次拉取")
	fmt.Println()

	for _, conflict := range conflicts {
		relativePath := conflict.Path
		// 统一为正斜杠，避免跨平台路径不匹配
		relativePath = pathpkg.ToSlash(relativePath)

		for {
			// Show line stats in prompt if available
			if len(conflict.LocalContent) > 0 && len(conflict.RemoteContent) > 0 {
				addLines, delLines := diffutil.CountLineChanges(conflict.LocalContent, conflict.RemoteContent)
				fmt.Printf("%s [1-4](默认=2, +%d/-%d 行): ", relativePath, addLines, delLines)
			} else {
				fmt.Printf("%s [1-4](默认=2): ", relativePath)
			}
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			switch input {
			case "1", "local":
				// 保留本地：ResetToRef 后会被远端覆盖，第八步后会恢复到工作目录
				keepLocalFiles = append(keepLocalFiles, relativePath)
				fmt.Printf("  -> 保留本地版本\n")
			case "2", "remote", "":
				// 使用远端：由第九步统一从工作目录读取并更新 SQLite
				useRemoteFiles = append(useRemoteFiles, relativePath)
				fmt.Printf("  -> 使用远端版本\n")
			case "3", "diff":
				// 查看差异后重新提示
				w.showConflictDiff(conflict)
				fmt.Println()
				fmt.Println("  [1] 本地  [2] 远端  [3] 差异  [4] 取消")
				continue
			case "4", "cancel", "q":
				// 取消
				fmt.Println("  已取消拉取")
				return true, nil, nil, nil
			default:
				fmt.Println("  无效输入，请重新选择")
				continue
			}
			break
		}
	}
	return false, keepLocalFiles, useRemoteFiles, nil
}

// applyRemoteToSnapshot updates the local SQLite snapshot with remote HEAD content.
// It adds new files, updates existing ones, and removes deleted ones.
// clone 后工作目录就是 remote HEAD，直接遍历工作目录。
func (w *LocalWorkflow) applyRemoteToSnapshot(
	ctx context.Context,
	repoPath string,
	remoteHash string,
	localSnapshot *models.Snapshot,
	projectPrefix string,
	projectBasePath string,
	toolType string,
) error {
	if localSnapshot == nil {
		return fmt.Errorf("本地快照不存在")
	}

	// 跟踪工作目录中存在的所有文件路径，用于后续删除快照中已不存在的文件
	existingPaths := make(map[string]bool)

	// 遍历工作目录中的文件（clone 后工作目录就是 remote HEAD）
	worktreeDir := pathpkg.Join(repoPath, projectPrefix)
	entries, err := os.ReadDir(worktreeDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			if err := w.applyRemoteToSnapshotSub(ctx, repoPath, remoteHash, localSnapshot, projectPrefix, entry.Name(), 10, existingPaths, projectBasePath, toolType); err != nil {
				logger.Warn("遍历远端目录失败", zap.String("dir", entry.Name()), zap.Error(err))
			}
		} else {
			relativePath := pathpkg.ToSlash(entry.Name())
			existingPaths[relativePath] = true
			fullPath := pathpkg.Join(worktreeDir, entry.Name())
			content, readErr := os.ReadFile(fullPath)
			if readErr != nil {
				logger.Warn("读取远端文件失败", zap.String("file", fullPath), zap.Error(readErr))
				continue
			}
			w.updateSnapshotFile(localSnapshot, relativePath, content, projectBasePath, toolType)
		}
	}

	// 删除快照中工作目录不再存在的文件
	newFiles := make([]models.SnapshotFile, 0, len(localSnapshot.Files))
	for _, f := range localSnapshot.Files {
		normalized := pathpkg.ToSlash(f.Path)
		if existingPaths[normalized] {
			newFiles = append(newFiles, f)
		} else {
			logger.Info("远端已删除文件，从快照中移除", zap.String("path", normalized))
		}
	}
	localSnapshot.Files = newFiles

	return w.snapshots.UpdateSnapshot(localSnapshot)
}

// applyRemoteToSnapshotSub recursively processes subdirectories.
func (w *LocalWorkflow) applyRemoteToSnapshotSub(
	ctx context.Context,
	repoPath string,
	remoteHash string,
	localSnapshot *models.Snapshot,
	projectPrefix string,
	subPath string,
	depth int,
	existingPaths map[string]bool,
	projectBasePath string,
	toolType string,
) error {
	if depth <= 0 {
		return nil
	}
	fullPath := pathpkg.Join(repoPath, projectPrefix, subPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		relativePath := pathpkg.ToSlash(pathpkg.Join(subPath, entry.Name()))
		if entry.IsDir() {
			if err := w.applyRemoteToSnapshotSub(ctx, repoPath, remoteHash, localSnapshot, projectPrefix, relativePath, depth-1, existingPaths, projectBasePath, toolType); err != nil {
				logger.Warn("遍历远端目录失败", zap.String("dir", relativePath), zap.Error(err))
			}
		} else {
			existingPaths[relativePath] = true
			content, readErr := os.ReadFile(pathpkg.Join(fullPath, entry.Name()))
			if readErr != nil {
				logger.Warn("读取远端文件失败", zap.String("file", pathpkg.Join(fullPath, entry.Name())), zap.Error(readErr))
				continue
			}
			w.updateSnapshotFile(localSnapshot, relativePath, content, projectBasePath, toolType)
		}
	}
	return nil
}

// updateSnapshotFile updates or adds a file in the snapshot.
// 对于已有文件：只更新 Content、Size、Hash。
// 对于新增文件：补全所有字段，包括 OriginalPath（使用 projectBasePath 拼接相对路径）。
func (w *LocalWorkflow) updateSnapshotFile(snapshot *models.Snapshot, relativePath string, content []byte, projectBasePath string, toolType string) {
	// 统一路径为正斜杠，避免跨平台不匹配
	normalizedPath := pathpkg.ToSlash(relativePath)
	for i := range snapshot.Files {
		if pathpkg.ToSlash(snapshot.Files[i].Path) == normalizedPath {
			snapshot.Files[i].Content = content
			snapshot.Files[i].Size = int64(len(content))
			snapshot.Files[i].Hash = computeHash(content)
			return
		}
	}
	// 新增文件：使用 projectBasePath 拼接相对路径构建完整的 OriginalPath
	originalPath := normalizedPath
	if projectBasePath != "" {
		originalPath = pathpkg.Join(projectBasePath, normalizedPath)
	}
	isBinary := isBinaryContent(content)
	snapshot.Files = append(snapshot.Files, models.SnapshotFile{
		Path:         normalizedPath,
		OriginalPath: originalPath,
		Size:         int64(len(content)),
		Hash:         computeHash(content),
		Content:      content,
		ToolType:     toolType,
		Category:     categorizeFile(normalizedPath, isBinary),
		IsBinary:     isBinary,
		ModifiedAt:   time.Now(),
	})
}

// computeHash computes SHA256 hash of content and returns hex string.
func computeHash(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// isBinaryContent checks if content appears to be binary (contains null bytes).
func isBinaryContent(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	limit := 512
	if len(content) < limit {
		limit = len(content)
	}
	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// categorizeFile categorizes a file based on its path and binary status.
func categorizeFile(path string, isBinary bool) models.FileCategory {
	filename := pathpkg.Base(path)
	ext := strings.TrimPrefix(pathpkg.Ext(path), ".")

	switch strings.ToLower(filename) {
	case "skills.yml", "skills.yaml":
		return models.CategorySkills
	case "commands.yml", "commands.yaml":
		return models.CategoryCommands
	case "plugins.yml", "plugins.yaml":
		return models.CategoryPlugins
	case "agents.md", "agents.yml", "agents.yaml":
		return models.CategoryAgents
	case "rules.md", "rules.yml", "rules.yaml":
		return models.CategoryRules
	}

	switch strings.ToLower(ext) {
	case "md", "markdown":
		return models.CategoryDocs
	case "yml", "yaml", "json", "toml":
		return models.CategoryConfig
	}

	if strings.Contains(strings.ToLower(path), "mcp") {
		return models.CategoryMCP
	}

	if isBinary {
		return models.CategoryOther
	}

	return models.CategoryConfig
}

// resolveProjectInfo looks up a registered project and returns its base path and tool type.
func (w *LocalWorkflow) resolveProjectInfo(projectName string) (basePath string, toolType string) {
	if w.rules == nil {
		return "", ""
	}
	projects, _ := w.rules.ListRegisteredProjects(nil)
	for _, p := range projects {
		if p.ProjectName == projectName {
			return p.ProjectPath, p.ToolType
		}
	}
	return "", ""
}

// SyncStatus checks the sync status of a project.
func (w *LocalWorkflow) SyncStatus(ctx context.Context, input typesSync.SyncStatusInput) (*typesSync.SyncStatusResult, error) {
	return nil, &UserError{
		Message:    "同步状态查询正在开发中",
		Suggestion: "请等待下一版本更新",
	}
}

// convertAuthFromModel converts models.AuthConfig to git.GitAuthConfig.
func convertAuthFromModel(auth *models.AuthConfig) *git.GitAuthConfig {
	if auth == nil {
		return nil
	}
	return &git.GitAuthConfig{
		Type:       git.AuthType(auth.Type),
		Username:   auth.Username,
		Password:   auth.Password,
		SSHKey:     auth.SSHKey,
		Passphrase: auth.Passphrase,
	}
}

// mkdirAll creates a directory and all parent directories.
func mkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}

// writeFile writes data to a file.
func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// parseGitError 解析 Git 错误并返回具体的错误信息和建议
func parseGitError(errStr, remoteURL string) (errorMsg, suggestion string) {
	errStr = strings.ToLower(errStr)

	// 认证失败
	if strings.Contains(errStr, "authentication required") ||
		strings.Contains(errStr, "invalid credentials") ||
		strings.Contains(errStr, "permission denied") && strings.Contains(errStr, "publickey") {
		return "拉取失败：认证失败",
			"请检查用户名和密码/SSH密钥是否正确，或更新访问令牌"
	}

	// 网络连接失败
	if strings.Contains(errStr, "could not resolve") ||
		strings.Contains(errStr, "network") && strings.Contains(errStr, "unreachable") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") {
		return "拉取失败：网络连接失败",
			"请检查网络连接和仓库地址是否正确"
	}

	// 仓库不存在
	if strings.Contains(errStr, "repository not found") ||
		strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "does not appear to be a git repository") {
		return fmt.Sprintf("拉取失败：仓库不存在（%s）", remoteURL),
			"请检查仓库地址是否有误，或是否有访问权限"
	}

	// 权限不足
	if strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "403") ||
		strings.Contains(errStr, "forbidden") {
		return "拉取失败：权限不足",
			"请检查您是否有该仓库的读取权限"
	}

	// SSL/TLS 证书问题
	if strings.Contains(errStr, "ssl") ||
		strings.Contains(errStr, "certificate") ||
		strings.Contains(errStr, "tls") {
		return "拉取失败：SSL/TLS 证书验证失败",
			"请检查证书配置或尝试使用 HTTPS 协议"
	}

	// 默认返回原始错误
	return fmt.Sprintf("拉取失败：%s", errStr),
		"请查看日志了解详细错误信息"
}

// showConflictDiff displays a unified diff for a conflict file.
func (w *LocalWorkflow) showConflictDiff(conflict typesSync.ConflictInfo) {
	fmt.Println()
	if conflict.IsBinary {
		fmt.Printf("  [二进制文件，大小: %d → %d 字节]\n", len(conflict.LocalContent), len(conflict.RemoteContent))
		return
	}

	diffResult := diffutil.ComputeConflictDiff(conflict.LocalContent, conflict.RemoteContent, conflict.Path)
	if diffResult == nil || len(diffResult.Files) == 0 {
		return
	}

	fileChange := diffResult.Files[0]
	fmt.Printf("  --- 本地 (%dB) → 远端 (%dB) ---\n", fileChange.OldSize, fileChange.NewSize)

	for _, hunk := range fileChange.Hunks {
		fmt.Printf("  @@ -%d,%d +%d,%d @@\n", hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount)
		for _, line := range hunk.Lines {
			switch line.Type {
			case 1: // LineAdded
				fmt.Printf("  \x1b[32m+%s\x1b[0m\n", line.Content)
			case 2: // LineDeleted
				fmt.Printf("  \x1b[31m-%s\x1b[0m\n", line.Content)
			default: // LineContext
				fmt.Printf("  %s\n", line.Content)
			}
		}
	}
}

// classifySnapshotDifferences categorizes differences between local snapshot and remote.
// Returns conflicts and auto-resolved files.
func (w *LocalWorkflow) classifySnapshotDifferences(
	ctx context.Context,
	repoPath string,
	remoteHash string,
	localSnapshot *models.Snapshot,
	projectPrefix string,
) (conflicts []typesSync.ConflictInfo, autoResolved []typesSync.AutoResolvedInfo, err error) {
	conflicts = []typesSync.ConflictInfo{}
	autoResolved = []typesSync.AutoResolvedInfo{}

	err = w.classifySnapshotDifferencesSub(ctx, repoPath, remoteHash, localSnapshot, projectPrefix, "", &conflicts, &autoResolved, 10)
	return conflicts, autoResolved, err
}

// classifySnapshotDifferencesSub recursively processes subdirectories.
func (w *LocalWorkflow) classifySnapshotDifferencesSub(
	ctx context.Context,
	repoPath string,
	remoteHash string,
	localSnapshot *models.Snapshot,
	projectPrefix string,
	subPath string,
	conflicts *[]typesSync.ConflictInfo,
	autoResolved *[]typesSync.AutoResolvedInfo,
	depth int,
) error {
	if depth <= 0 {
		return nil
	}
	fullPath := pathpkg.Join(repoPath, projectPrefix, subPath)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return err
	}

	// Build a map of files in local snapshot for quick lookup
	snapshotFileMap := make(map[string][]byte)
	if localSnapshot != nil {
		for _, f := range localSnapshot.Files {
			snapshotFileMap[pathpkg.ToSlash(f.Path)] = f.Content
		}
	}

	// Track which snapshot files have been processed
	processedPaths := make(map[string]bool)

	for _, entry := range entries {
		relativePath := entry.Name()
		if subPath != "" {
			relativePath = pathpkg.ToSlash(pathpkg.Join(subPath, entry.Name()))
		}

		// 统一为正斜杠，避免跨平台路径不匹配
		normalizedRelativePath := pathpkg.ToSlash(relativePath)

		if entry.IsDir() {
			if err := w.classifySnapshotDifferencesSub(ctx, repoPath, remoteHash, localSnapshot, projectPrefix, normalizedRelativePath, conflicts, autoResolved, depth-1); err != nil {
				logger.Warn("遍历远端目录失败", zap.String("dir", normalizedRelativePath), zap.Error(err))
			}
			continue
		}

		processedPaths[normalizedRelativePath] = true
		fullFilePath := pathpkg.Join(fullPath, entry.Name())
		remoteContent, readErr := os.ReadFile(fullFilePath)
		if readErr != nil {
			continue
		}

		localContent, existsInSnapshot := snapshotFileMap[normalizedRelativePath]

		if !existsInSnapshot {
			// 远端新增文件 → 自动解决
			*autoResolved = append(*autoResolved, typesSync.AutoResolvedInfo{
				Path:       normalizedRelativePath,
				Resolution: "remote_added",
				Summary:    fmt.Sprintf("远端新增文件（%d 字节），本地不存在此文件，已自动同步到本地快照", len(remoteContent)),
			})
		} else if string(localContent) != string(remoteContent) {
			// 双方都有但内容不同 → 冲突
			*conflicts = append(*conflicts, typesSync.ConflictInfo{
				Path:          normalizedRelativePath,
				ConflictType:  "both_modified",
				LocalSummary:  fmt.Sprintf("本地已修改 (%d 字节)", len(localContent)),
				RemoteSummary: fmt.Sprintf("远端已修改 (%d 字节)", len(remoteContent)),
				LocalHash:     "",
				RemoteHash:    remoteHash,
				LocalContent:  localContent,
				RemoteContent: remoteContent,
				IsBinary:      diffutil.IsBinaryContent(localContent) || diffutil.IsBinaryContent(remoteContent),
			})
		}
	}

	// Check for files in snapshot that don’t exist remotely (local-only files).
	// Only check files belonging to the current subPath to avoid duplicating
	// root-level files in every recursive call.
	if localSnapshot != nil {
		prefix := subPath
		if prefix != "" {
			prefix = prefix + "/"
		}
		for _, f := range localSnapshot.Files {
			normalizedPath := pathpkg.ToSlash(f.Path)
			// Skip files outside the current subPath
			if subPath != "" && !strings.HasPrefix(normalizedPath, prefix) {
				continue
			}
			if !processedPaths[normalizedPath] {
				// 本地快照有但远端没有 → 自动解决
				*autoResolved = append(*autoResolved, typesSync.AutoResolvedInfo{
					Path:       normalizedPath,
					Resolution: "local_only",
					Summary:    fmt.Sprintf("本地快照有文件 %s，远端已无此文件，保留本地版本不变", normalizedPath),
				})
			}
		}
	}

	return nil
}

// pathExists 检查指定路径是否存在（文件或目录）。
func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// ensureRepoExists 确保本地仓库存在，不存在时询问用户是否从远程 clone。
// 返回是否继续执行（用户取消则返回 false）。
func (w *LocalWorkflow) ensureRepoExists(ctx context.Context, repoPath string, remoteURL string, auth *git.GitAuthConfig, branch string, promptMsg string) (bool, error) {
	if git.IsRepository(repoPath) {
		return true, nil
	}

	fmt.Printf("本地仓库不存在（%s）\n", repoPath)
	fmt.Printf("%s[Y/n]: ", promptMsg)

	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm != "" && confirm != "y" && confirm != "yes" {
		return false, nil
	}

	gitClient := git.NewGitClient()
	_, cloneErr := gitClient.Clone(ctx, &git.CloneOptions{
		URL:    remoteURL,
		Path:   repoPath,
		Auth:   auth,
		Branch: branch,
	})
	if cloneErr != nil {
		return false, &UserError{
			Message:    fmt.Sprintf("拉取远端仓库失败（远端: %s）", remoteURL),
			Suggestion: "请检查网络连接和仓库地址",
			Err:        cloneErr,
		}
	}
	return true, nil
}
