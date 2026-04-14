package usecase

import (
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
//  1. 查找 project 最新快照
//  2. 查找 project 的远端配置
//  3. 确保工作目录存在（不存在则 clone）
//  4. 从 SQLite 读文件 → 写入工作目录 → Git commit + push
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

	// 使用第一个活跃的配置（简化实现，后续可按 project 绑定）
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

	// 第三步：查找 project 最新快照
	if w.snapshots == nil {
		return nil, &UserError{
			Message:    "推送失败：快照服务不可用",
			Suggestion: "请重新启动程序",
		}
	}
	items, err := w.snapshots.ListSnapshots(0, 0)
	if err != nil {
		return nil, &UserError{
			Message:    "推送失败：无法读取快照列表",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	var targetID string
	for _, item := range items {
		if item.Project == projectName {
			targetID = fmt.Sprintf("%d", item.ID)
			break
		}
	}
	if targetID == "" {
		return nil, &UserError{
			Message:    "推送失败：项目 \"" + projectName + "\" 没有快照",
			Suggestion: "请先执行 fl snapshot create -p " + projectName + " 创建快照",
		}
	}

	snapshot, err := w.snapshots.GetSnapshot(targetID)
	if err != nil {
		return nil, &UserError{
			Message:    "推送失败：无法读取快照数据",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	// 第四步：确保工作目录存在
	dataDir := w.dataDir
	repoPath := filepath.Join(dataDir, "repos", projectName)

	gitClient := git.NewGitClient()
	if !git.IsRepository(repoPath) {
		// 尝试 clone 远端仓库
		cloneOpts := &git.CloneOptions{
			URL:    remoteConfig.URL,
			Path:   repoPath,
			Auth:   convertAuthFromModel(&remoteConfig.Auth),
			Branch: remoteConfig.Branch,
		}
		if _, cloneErr := gitClient.Clone(ctx, cloneOpts); cloneErr != nil {
			logger.Info("Clone 失败，尝试初始化新仓库",
				zap.String("repo_path", repoPath),
				zap.Error(cloneErr),
			)
			// clone 失败则初始化新仓库
			if _, initErr := git.InitRepository(repoPath, false); initErr != nil {
				return nil, &UserError{
					Message:    fmt.Sprintf("推送失败：无法创建工作目录（远端: %s）", remoteConfig.URL),
					Suggestion: "请检查磁盘空间和文件权限",
					Err:        initErr,
				}
			}
			_ = git.AddRemote(repoPath, "origin", remoteConfig.URL)
			logger.Info("新仓库初始化成功", zap.String("repo_path", repoPath))
		} else {
			logger.Info("Clone 成功", zap.String("repo_path", repoPath))
		}
	}

	// 确保分枝正确：clone 空仓库或 init 后，HEAD 可能指向 master 而非 main
	targetBranch := remoteConfig.Branch
	if targetBranch == "" {
		targetBranch = "main"
	}
	if branchErr := git.EnsureBranch(repoPath, targetBranch); branchErr != nil {
		logger.Warn("设置分枝失败，将使用默认分枝",
			zap.String("repo_path", repoPath),
			zap.String("branch", targetBranch),
			zap.Error(branchErr),
		)
	}

	// 第五步：从 SQLite 读文件 → 写入工作目录
	filesWritten := 0
	for _, file := range snapshot.Files {
		if len(file.Content) == 0 {
			continue
		}
		targetPath := filepath.Join(repoPath, projectName, file.Path)
		dir := filepath.Dir(targetPath)
		if mkErr := mkdirAll(dir); mkErr != nil {
			continue
		}
		if err := writeFile(targetPath, file.Content); err != nil {
			continue
		}
		filesWritten++
	}

	logger.Info("文件写入完成",
		zap.String("repo_path", repoPath),
		zap.Int("files_written", filesWritten),
	)

	// 诊断：写入后检查 git status
	statusResult, statusErr := gitClient.GetStatus(&git.StatusOptions{Path: repoPath})
	if statusErr != nil {
		logger.Warn("无法获取 git status",
			zap.String("repo_path", repoPath),
			zap.Error(statusErr),
		)
	} else {
		logger.Info("Git status 诊断",
			zap.Bool("is_clean", statusResult.IsClean),
			zap.String("branch", statusResult.Branch),
			zap.Int("changed_files", len(statusResult.Files)),
		)
		for i, f := range statusResult.Files {
			if i >= 5 {
				break
			}
			logger.Info("Git status 文件",
				zap.String("path", f.Path),
				zap.String("worktree", f.Worktree),
				zap.String("staging", f.Staging),
			)
		}
	}

	// 第六步：Git add + commit
	commitMsg := fmt.Sprintf("Snapshot: %s\nID: %d\nProject: %s\nFiles: %d",
		snapshot.Name, snapshot.ID, snapshot.Project, filesWritten)
	commitResult, err := gitClient.Commit(ctx, &git.CommitOptions{
		Path:    repoPath,
		Message: commitMsg,
		All:     true,
	})
	if err != nil {
		// commit 失败可能是因为没有变更
		// 为什么：go-git 返回 "cannot create empty commit: clean working tree"，而不是 "nothing to commit"
		errMsg := err.Error()
		if strings.Contains(errMsg, "nothing to commit") || strings.Contains(errMsg, "empty commit") {
			// 为什么需要区分 filesWritten：用户第一次推送时如果远端已有相同内容，
			// 会写入大量文件但 git 认为无变更，需要给出明确说明。
			msg := "远端仓库内容与本地快照一致，无需推送"
			if filesWritten == 0 {
				msg = "快照中没有需要推送的文件"
			} else {
				logger.Warn("文件已写入但 Git 未检测到变更",
					zap.String("project", projectName),
					zap.Int("files_written", filesWritten),
					zap.String("repo_path", repoPath),
					zap.Error(err),
				)
			}
			return &typesSync.SyncPushResult{
				Success:     true,
				Project:     projectName,
				FilesPushed: 0,
				RemoteURL:   remoteConfig.URL,
				Message:     msg,
			}, nil
		}
		return nil, &UserError{
			Message:    fmt.Sprintf("推送失败：Git commit 失败（远端: %s）", remoteConfig.URL),
			Suggestion: "请检查工作目录状态",
			Err:        err,
		}
	}

	// 第七步：Git push
	branch := remoteConfig.Branch
	if branch == "" {
		branch = "main"
	}
	pushResult, err := gitClient.Push(ctx, &git.PushOptions{
		Path:       repoPath,
		Auth:       convertAuthFromModel(&remoteConfig.Auth),
		RemoteName: "origin",
		Branch:     branch,
	})
	if err != nil {
		// push 失败不影响本地
		logger.Error("推送失败",
			zap.String("project", projectName),
			zap.String("remote_url", remoteConfig.URL),
			zap.String("branch", branch),
			zap.Error(err),
		)
		return nil, &UserError{
			Message:    fmt.Sprintf("推送失败：无法推送到远端仓库 \"%s\" (%s)", remoteConfig.Name, remoteConfig.URL),
			Suggestion: "请检查网络连接和仓库权限。本地数据未受影响。",
			Err:        err,
		}
	}
	// 诊断：push 后的仓库状态
	logger.Info("Push 返回结果",
		zap.String("project", projectName),
		zap.Bool("success", pushResult.Success),
		zap.String("message", pushResult.Message),
	)
	localHead, _ := gitClient.GetHeadHash(repoPath)
	logger.Info("本地 HEAD",
		zap.String("project", projectName),
		zap.String("local_head", localHead),
	)
	remoteHead, remoteHeadErr := gitClient.GetRemoteHeadHash(repoPath, "origin", branch)
	logger.Info("远端 HEAD",
		zap.String("project", projectName),
		zap.String("branch", branch),
		zap.String("remote_head", remoteHead),
		zap.Error(remoteHeadErr),
	)

	commitHash := ""
	if commitResult != nil {
		commitHash = commitResult.CommitHash
	}

	// 第八步：验证远端是否真的收到了提交
	logger.Info("验证远端提交",
		zap.String("project", projectName),
		zap.String("commit", commitHash),
		zap.String("remote_url", remoteConfig.URL),
		zap.String("branch", remoteConfig.Branch),
	)
	remoteHash, err := gitClient.GetRemoteHeadHash(repoPath, "origin", remoteConfig.Branch)
	if err != nil {
		logger.Warn("无法获取远端 HEAD，跳过验证",
			zap.String("project", projectName),
			zap.Error(err),
		)
	} else if commitHash != "" && remoteHash != commitHash {
		// 本地和远端不一致，push 可能失败
		logger.Error("远端提交验证失败",
			zap.String("project", projectName),
			zap.String("local_commit", commitHash),
			zap.String("remote_commit", remoteHash),
		)
		return nil, &UserError{
			Message:    fmt.Sprintf("推送失败：远端仓库未收到提交（本地: %s, 远端: %s）", commitHash[:8], remoteHash[:8]),
			Suggestion: "请检查远端仓库状态、分支配置和网络连接。本地数据未受影响。",
			Err:        err,
		}
	} else {
		logger.Info("推送验证成功",
			zap.String("project", projectName),
			zap.String("commit", commitHash),
			zap.Int("files_pushed", filesWritten),
		)
	}

	return &typesSync.SyncPushResult{
		Success:     true,
		Project:     projectName,
		FilesPushed: filesWritten,
		CommitHash:  commitHash,
		RemoteURL:   remoteConfig.URL,
	}, nil
}

// SyncPull pulls the latest configuration from remote and updates SQLite.
//
// 流程（含冲突检测）：
//  1. 获取远端配置
//  2. 确保 repo 存在，执行 git fetch（不 merge）
//  3. 对比本地 HEAD 和远端 HEAD 获取变更文件
//  4. 对每个变更文件做三方对比（本地快照 vs 远端版本）检测冲突 ··//  5. 无冲突 → hard reset 到远端版本 + 更新 SQLite
//  6. 有冲突 → 返回冲突列表，等待用户解决
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
	gitClient := git.NewGitClient()

	if !git.IsRepository(repoPath) {
		return nil, &UserError{
			Message:    "拉取失败：本地仓库不存在",
			Suggestion: "请先执行 fl sync push --project " + projectName + " 初始化仓库",
		}
	}

	// 第四步：记录本地 HEAD，然后 fetch 远端更新
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
