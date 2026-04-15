package usecase

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"flux/internal/models"
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
	repoPath := filepath.Join(dataDir, "repos", projectName)

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
	projectSubDir := filepath.Join(repoPath, projectName)
	if exists, _ := pathExists(projectSubDir); exists {
		entries, readErr := os.ReadDir(projectSubDir)
		if readErr == nil {
			for _, entry := range entries {
				entryPath := filepath.Join(projectSubDir, entry.Name())
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
		targetPath := filepath.Join(repoPath, projectName, file.Path)
		dir := filepath.Dir(targetPath)
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

		// 粗略按比例分配显示：新增/修改/删除
		total := added + modified + deleted
		if total > 15 {
			showAdded := min(added, 5)
			showModified := min(modified, 5)
			showDeleted := min(deleted, 5)
			for i := 0; i < showAdded && i < len(diffResult.Files); i++ {
				showFile("新增", diffResult.Files[i])
			}
			offset := showAdded
			for i := 0; i < showModified && offset+i < len(diffResult.Files); i++ {
				showFile("修改", diffResult.Files[offset+i])
			}
			offset += showModified
			for i := 0; i < showDeleted && offset+i < len(diffResult.Files); i++ {
				showFile("删除", diffResult.Files[offset+i])
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
			// 文件少，按顺序输出（新增在前，修改在中，删除在后）
			for i, path := range diffResult.Files {
				var label string
				if i < added {
					label = "新增"
				} else if i < added+modified {
					label = "修改"
				} else {
					label = "删除"
				}
				fmt.Printf("  [%s] %s\n", label, path)
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
	repoPath := filepath.Join(dataDir, "repos", projectName)

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
			Success: true,
			Project: projectName,
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
	remoteHash, _ := gitClient.GetRemoteHeadHash(repoPath, "origin", branch)
	if localHash == "" || remoteHash == "" || localHash == remoteHash {
		// 没有远端变更
		return &typesSync.SyncPullResult{
			Success:      true,
			Project:      projectName,
			FilesUpdated: 0,
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
	projectPrefix := projectName + "/"
	var conflicts []typesSync.ConflictInfo
	var safeFiles []git.FileDiff

	// 获取本地最新快照
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
			})
		} else {
			// 本地没有修改，只有远端修改 → 安全合并
			safeFiles = append(safeFiles, cf)
		}
	}

	// 第七步：如果有冲突，返回冲突信息（不自动合并）
	if len(conflicts) > 0 {
		return &typesSync.SyncPullResult{
			Success:       false,
			Project:       projectName,
			HasConflicts:  true,
			ConflictCount: len(conflicts),
			Conflicts:     conflicts,
		}, nil
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

	// 第九步：更新 SQLite 中安全合并的文件
	var updatedCount int
	for _, sf := range safeFiles {
		if !strings.HasPrefix(sf.Path, projectPrefix) {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(repoPath, sf.Path))
		if readErr != nil {
			continue
		}

		if localSnapshot != nil {
			relativePath := strings.TrimPrefix(sf.Path, projectPrefix)
			for i, f := range localSnapshot.Files {
				if f.Path == relativePath {
					localSnapshot.Files[i].Content = content
					_ = w.snapshots.UpdateSnapshot(localSnapshot)
					updatedCount++
					break
				}
			}
		}
	}

	return &typesSync.SyncPullResult{
		Success:      true,
		Project:      projectName,
		FilesUpdated: updatedCount,
	}, nil
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
