package git

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"flux/pkg/logger"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
	"go.uber.org/zap"
)

// GitClient Git 客户端
type GitClient struct {
	logger *zap.Logger
}

// NewGitClient 创建 Git 客户端
func NewGitClient() *GitClient {
	return &GitClient{
		logger: logger.L(),
	}
}

// Clone 基于 go-git 执行克隆，并在结束后回填基础仓库信息。
func (c *GitClient) Clone(ctx context.Context, opts *CloneOptions) (*RepositoryInfo, error) {
	if opts == nil {
		return nil, fmt.Errorf("克隆选项不能为空")
	}

	if opts.URL == "" {
		return nil, fmt.Errorf("仓库 URL 不能为空")
	}

	if opts.Path == "" {
		return nil, fmt.Errorf("目标路径不能为空")
	}

	logger.Info("开始克隆仓库",
		zap.String("url", opts.URL),
		zap.String("path", opts.Path),
		zap.String("branch", opts.Branch),
	)

	// 创建认证方法
	auth, err := NewAuthMethod(opts.Auth)
	if err != nil {
		logger.Error("创建认证方法失败", zap.Error(err))
		return nil, fmt.Errorf("创建认证方法失败: %w", err)
	}

	// 克隆选项
	cloneOpts := &git.CloneOptions{
		URL:      opts.URL,
		Progress: os.Stdout,
	}

	if auth != nil {
		cloneOpts.Auth = auth
	}

	if opts.Branch != "" {
		cloneOpts.ReferenceName = plumbing.ReferenceName("refs/heads/" + opts.Branch)
	}

	if opts.Depth > 0 {
		cloneOpts.Depth = opts.Depth
	}

	if opts.SingleBranch {
		cloneOpts.SingleBranch = true
	}

	// 执行克隆
	repo, err := git.PlainCloneContext(ctx, opts.Path, false, cloneOpts)
	if err != nil {
		logger.Error("克隆仓库失败", zap.Error(err))
		return nil, fmt.Errorf("克隆仓库失败: %w", err)
	}

	logger.Info("仓库克隆成功", zap.String("path", opts.Path))

	// 获取仓库信息
	return c.getRepositoryInfo(repo, opts.Path)
}

// Pull 只处理当前工作树所在分支，不在这里做额外分支切换。
func (c *GitClient) Pull(ctx context.Context, opts *PullOptions) (*OperationResult, error) {
	if opts == nil {
		return nil, fmt.Errorf("拉取选项不能为空")
	}

	if opts.Path == "" {
		return nil, fmt.Errorf("仓库路径不能为空")
	}

	logger.Info("开始拉取更新",
		zap.String("path", opts.Path),
		zap.String("remote", opts.RemoteName),
		zap.String("branch", opts.Branch),
	)

	// 打开仓库
	repo, err := git.PlainOpen(opts.Path)
	if err != nil {
		logger.Error("打开仓库失败", zap.Error(err))
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	// 获取工作树
	worktree, err := repo.Worktree()
	if err != nil {
		logger.Error("获取工作树失败", zap.Error(err))
		return nil, fmt.Errorf("获取工作树失败: %w", err)
	}

	// 创建认证方法
	auth, err := NewAuthMethod(opts.Auth)
	if err != nil {
		logger.Error("创建认证方法失败", zap.Error(err))
		return nil, fmt.Errorf("创建认证方法失败: %w", err)
	}

	// 拉取选项
	pullOpts := &git.PullOptions{
		Progress: os.Stdout,
		Force:    opts.Force,
	}

	if auth != nil {
		pullOpts.Auth = auth
	}

	if opts.RemoteName != "" {
		pullOpts.RemoteName = opts.RemoteName
	}

	if opts.Branch != "" {
		pullOpts.ReferenceName = plumbing.ReferenceName("refs/heads/" + opts.Branch)
	}

	// 执行拉取
	err = worktree.PullContext(ctx, pullOpts)
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			logger.Info("仓库已是最新")
			return &OperationResult{
				Success: true,
				Message: "仓库已是最新",
			}, nil
		}
		logger.Error("拉取失败", zap.Error(err))
		return nil, fmt.Errorf("拉取失败: %w", err)
	}

	logger.Info("拉取成功")

	return &OperationResult{
		Success: true,
		Message: "拉取成功",
	}, nil
}

// Push 根据给定分支构造 refspec；未指定分支时交给 go-git 使用默认行为。
func (c *GitClient) Push(ctx context.Context, opts *PushOptions) (*OperationResult, error) {
	if opts == nil {
		return nil, fmt.Errorf("推送选项不能为空")
	}

	if opts.Path == "" {
		return nil, fmt.Errorf("仓库路径不能为空")
	}

	logger.Info("开始推送",
		zap.String("path", opts.Path),
		zap.String("remote", opts.RemoteName),
		zap.String("branch", opts.Branch),
		zap.Bool("force", opts.Force),
	)

	// 打开仓库
	repo, err := git.PlainOpen(opts.Path)
	if err != nil {
		logger.Error("打开仓库失败", zap.Error(err))
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	// 创建认证方法
	auth, err := NewAuthMethod(opts.Auth)
	if err != nil {
		logger.Error("创建认证方法失败", zap.Error(err))
		return nil, fmt.Errorf("创建认证方法失败: %w", err)
	}

	// 推送选项
	pushOpts := &git.PushOptions{
		Progress: os.Stdout,
	}

	if auth != nil {
		pushOpts.Auth = auth
	}

	if opts.RemoteName != "" {
		pushOpts.RemoteName = opts.RemoteName
	}

	// 诊断：检查本地分支状态
	localHead, _ := repo.Head()
	if localHead != nil {
		logger.Info("Push 前 HEAD 状态",
			zap.String("head_name", localHead.Name().String()),
			zap.String("head_hash", localHead.Hash().String()),
		)
	} else {
		logger.Warn("Push 前 HEAD 为空")
	}

	if opts.Branch != "" {
		refName := plumbing.ReferenceName("refs/heads/" + opts.Branch)
		// 检查该分支是否存在
		branchRef, branchErr := repo.Reference(refName, true)
		if branchErr != nil {
			logger.Warn("本地分支不存在，尝试用 HEAD 推送",
				zap.String("branch", opts.Branch),
				zap.Error(branchErr),
			)
			// 用 HEAD:refs/heads/branch 推送当前 HEAD 到目标分支
			pushOpts.RefSpecs = []config.RefSpec{
				config.RefSpec("HEAD:" + refName),
			}
		} else {
			logger.Info("本地分支存在",
				zap.String("branch", opts.Branch),
				zap.String("hash", branchRef.Hash().String()),
			)
			pushOpts.RefSpecs = []config.RefSpec{
				config.RefSpec(refName + ":" + refName),
			}
		}
	}

	if opts.Force {
		pushOpts.Force = true
	}

	// 执行推送
	err = repo.PushContext(ctx, pushOpts)
	if err != nil {
		// already up-to-date 不是错误，只是 Git 报告远端已是最新
		if err == git.NoErrAlreadyUpToDate {
			logger.Info("推送已是最新")
			return &OperationResult{
				Success: true,
				Message: "已是最新，无需推送",
			}, nil
		}
		logger.Error("推送失败", zap.Error(err))
		return nil, fmt.Errorf("推送失败: %w", err)
	}

	logger.Info("推送成功")

	return &OperationResult{
		Success: true,
		Message: "推送成功",
	}, nil
}

// GetStatus 汇总工作树变更和当前分支信息。
func (c *GitClient) GetStatus(opts *StatusOptions) (*RepositoryStatus, error) {
	if opts == nil {
		return nil, fmt.Errorf("状态选项不能为空")
	}

	if opts.Path == "" {
		return nil, fmt.Errorf("仓库路径不能为空")
	}

	// 打开仓库
	repo, err := git.PlainOpen(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	// 获取工作树
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("获取工作树失败: %w", err)
	}

	// 获取状态
	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("获取状态失败: %w", err)
	}

	// 获取当前分支
	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("获取 HEAD 失败: %w", err)
	}

	branch := strings.TrimPrefix(head.Name().String(), "refs/heads/")

	// 构建文件状态列表
	files := make([]FileStatus, 0, len(status))
	for path, fileStatus := range status {
		files = append(files, FileStatus{
			Path:     path,
			Worktree: string(fileStatus.Worktree),
			Staging:  string(fileStatus.Staging),
		})
	}

	return &RepositoryStatus{
		IsClean: status.IsClean(),
		Branch:  branch,
		Files:   files,
	}, nil
}

// GetRepositoryInfo 对外暴露只读仓库概览。
func (c *GitClient) GetRepositoryInfo(path string) (*RepositoryInfo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	return c.getRepositoryInfo(repo, path)
}

// getRepositoryInfo 统一提取远程、HEAD 和裸仓库等基础信息。
func (c *GitClient) getRepositoryInfo(repo *git.Repository, path string) (*RepositoryInfo, error) {
	info := &RepositoryInfo{
		Path: path,
	}

	// 获取远程配置
	remotes, err := repo.Remotes()
	if err == nil && len(remotes) > 0 {
		// 获取 origin 的 URL
		for _, remote := range remotes {
			if remote.Config().Name == "origin" || len(remote.Config().URLs) > 0 {
				if len(remote.Config().URLs) > 0 {
					info.RemoteURL = remote.Config().URLs[0]
					break
				}
			}
		}
	}

	// 获取 HEAD
	head, err := repo.Head()
	if err == nil {
		info.Head = head.Name().String()
		info.Branch = strings.TrimPrefix(head.Name().String(), "refs/heads/")
		info.CommitHash = head.Hash().String()
	}

	// 检查是否为裸仓库（通过检查工作树是否存在）
	_, worktreeErr := repo.Worktree()
	info.IsBare = worktreeErr != nil

	return info, nil
}

// Commit 当前支持“全部加入后提交”的简单模式，满足快照仓库场景。
func (c *GitClient) Commit(ctx context.Context, opts *CommitOptions) (*CommitResult, error) {
	if opts == nil {
		return nil, fmt.Errorf("提交选项不能为空")
	}

	if opts.Path == "" {
		return nil, fmt.Errorf("仓库路径不能为空")
	}

	if opts.Message == "" {
		return nil, fmt.Errorf("提交消息不能为空")
	}

	logger.Info("开始提交",
		zap.String("path", opts.Path),
		zap.String("message", opts.Message),
	)

	// 打开仓库
	repo, err := git.PlainOpen(opts.Path)
	if err != nil {
		logger.Error("打开仓库失败", zap.Error(err))
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	// 获取工作树
	worktree, err := repo.Worktree()
	if err != nil {
		logger.Error("获取工作树失败", zap.Error(err))
		return nil, fmt.Errorf("获取工作树失败: %w", err)
	}

	// 添加所有更改（如果需要）
	if opts.All {
		_, err = worktree.Add(".")
		if err != nil {
			logger.Error("添加文件失败", zap.Error(err))
			return nil, fmt.Errorf("添加文件失败: %w", err)
		}
	}

	// 执行提交
	hash, err := worktree.Commit(opts.Message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  opts.Author,
			Email: "",
			When:  time.Now(),
		},
	})
	if err != nil {
		// clean working tree is not a real error
		if strings.Contains(err.Error(), "empty commit") || strings.Contains(err.Error(), "nothing to commit") {
			logger.Debug("no changes to commit")
			return nil, fmt.Errorf("提交失败: %w", err)
		}
		logger.Error("提交失败", zap.Error(err))
		return nil, fmt.Errorf("提交失败: %w", err)
	}

	// 获取提交信息
	commit, err := repo.CommitObject(hash)
	if err != nil {
		logger.Warn("获取提交对象失败", zap.Error(err))
	} else {
		logger.Info("提交成功",
			zap.String("hash", hash.String()),
			zap.String("message", commit.Message),
		)
	}

	return &CommitResult{
		Success:    true,
		CommitHash: hash.String(),
		Message:    opts.Message,
		Time:       time.Now(),
	}, nil
}

// ListBranches 同时返回本地和远程分支，并标记当前 HEAD。
func (c *GitClient) ListBranches(path string) ([]BranchInfo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	branches := []BranchInfo{}

	// 获取本地分支
	refs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("获取引用失败: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("获取 HEAD 失败: %w", err)
	}

	err = refs.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().String()
		if strings.HasPrefix(name, "refs/heads/") {
			branchName := strings.TrimPrefix(name, "refs/heads/")
			branches = append(branches, BranchInfo{
				Name:     branchName,
				IsHead:   ref.Name() == head.Name(),
				IsRemote: false,
			})
		} else if strings.HasPrefix(name, "refs/remotes/") && !strings.HasSuffix(name, "/HEAD") {
			branchName := strings.TrimPrefix(name, "refs/remotes/")
			branches = append(branches, BranchInfo{
				Name:     branchName,
				IsRemote: true,
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return branches, nil
}

// IsRepository 用于轻量判断路径是否已经是 Git 仓库。
func IsRepository(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

// InitRepository 初始化新仓库，并复用 GitClient 的信息提取逻辑。
// 默认 HEAD 指向 main 分支（go-git 默认是 master，现代仓库用 main）。
func InitRepository(path string, bare bool) (*RepositoryInfo, error) {
	repo, err := git.PlainInit(path, bare)
	if err != nil {
		return nil, fmt.Errorf("初始化仓库失败: %w", err)
	}

	if err := EnsureBranch(path, "main"); err != nil {
		return nil, err
	}

	client := NewGitClient()
	return client.getRepositoryInfo(repo, path)
}

// EnsureBranch ensures the repository's HEAD points to the specified branch.
// 为什么：go-git 的 PlainInit 和空仓库的 Clone 都默认用 master 分支，
// 但现代仓库约定用 main。调用方应在 clone/init 后调用此函数。
func EnsureBranch(path, branch string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("打开仓库失败: %w", err)
	}

	head, err := repo.Head()
	if err != nil && err != plumbing.ErrReferenceNotFound {
		// 新仓库没有 HEAD 是正常的
		return nil
	}

	if head != nil {
		expected := plumbing.ReferenceName("refs/heads/" + branch)
		if head.Name() == expected {
			return nil // 已正确
		}
	}

	if err := repo.Storer.SetReference(
		plumbing.NewSymbolicReference("HEAD", plumbing.ReferenceName("refs/heads/"+branch)),
	); err != nil {
		return fmt.Errorf("设置分支 %s 失败: %w", branch, err)
	}

	logger.Info("已设置仓库分支", zap.String("path", path), zap.String("branch", branch))
	return nil
}

// AddRemote 为现有仓库添加远端配置。
func AddRemote(path, name, url string) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("打开仓库失败: %w", err)
	}

	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: name,
		URLs: []string{url},
	})

	if err != nil {
		return fmt.Errorf("添加远程仓库失败: %w", err)
	}

	return nil
}

// Fetch pulls remote changes without merging, similar to git fetch.
func (c *GitClient) Fetch(ctx context.Context, opts *FetchOptions) (*OperationResult, error) {
	if opts == nil {
		return nil, fmt.Errorf("fetch 选项不能为空")
	}

	repo, err := git.PlainOpen(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	auth, err := NewAuthMethod(opts.Auth)
	if err != nil {
		return nil, fmt.Errorf("创建认证方法失败: %w", err)
	}

	remoteName := opts.Remote
	if remoteName == "" {
		remoteName = "origin"
	}

	fetchOpts := &git.FetchOptions{
		RemoteName: remoteName,
		Auth:       auth,
	}

	if err := repo.Fetch(fetchOpts); err != nil {
		if err == git.NoErrAlreadyUpToDate {
			return &OperationResult{Success: true, Message: "已是最新"}, nil
		}
		return nil, fmt.Errorf("fetch 失败: %w", err)
	}

	return &OperationResult{Success: true, Message: "fetch 成功"}, nil
}

// ResetToRef hard-resets the worktree and branch to the given commit hash.
// 为什么：go-git 的 Pull 只支持 fast-forward 合并，当本地和远端有分叉历史时会报
// "non-fast-forward update"。对于配置同步场景，本地仓库只是同步中间层，
// 在应用层已完成冲突检测后，可以安全地 hard reset 到远端版本。
func (c *GitClient) ResetToRef(path string, hash string) (*OperationResult, error) {
	if path == "" {
		return nil, fmt.Errorf("仓库路径不能为空")
	}
	if hash == "" {
		return nil, fmt.Errorf("目标 commit hash 不能为空")
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("获取工作树失败: %w", err)
	}

	targetHash := plumbing.NewHash(hash)
	if err := worktree.Reset(&git.ResetOptions{
		Commit: targetHash,
		Mode:   git.HardReset,
	}); err != nil {
		logger.Error("Hard reset 失败",
			zap.String("path", path),
			zap.String("target_hash", hash),
			zap.Error(err),
		)
		return nil, fmt.Errorf("重置到远端版本失败: %w", err)
	}

	logger.Info("已重置到远端版本",
		zap.String("path", path),
		zap.String("hash", hash),
	)
	return &OperationResult{
		Success: true,
		Message: "已重置到远端版本",
	}, nil
}

// GetHeadHash returns the current HEAD commit hash.
func (c *GitClient) GetHeadHash(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("打开仓库失败: %w", err)
	}

	ref, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("获取 HEAD 失败: %w", err)
	}

	return ref.Hash().String(), nil
}

// GetRemoteHeadHash returns the remote tracking branch's HEAD hash.
// This is useful for comparing local and remote state after a fetch.
func (c *GitClient) GetRemoteHeadHash(path, remoteName, branch string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", fmt.Errorf("打开仓库失败: %w", err)
	}

	refName := plumbing.ReferenceName("refs/remotes/" + remoteName + "/" + branch)
	ref, err := repo.Reference(refName, true)
	if err != nil {
		return "", fmt.Errorf("获取远端引用失败: %w", err)
	}

	return ref.Hash().String(), nil
}

// Log returns commit history from the repository.
func (c *GitClient) Log(ctx context.Context, opts *LogOptions) ([]CommitInfo, error) {
	if opts == nil {
		return nil, fmt.Errorf("log 选项不能为空")
	}

	repo, err := git.PlainOpen(opts.Path)
	if err != nil {
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	commitIter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取日志失败: %w", err)
	}
	defer commitIter.Close()

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	var commits []CommitInfo
	for i := 0; i < limit; i++ {
		commit, err := commitIter.Next()
		if err != nil {
			break
		}

		commits = append(commits, CommitInfo{
			Hash:    commit.Hash.String(),
			Message: commit.Message,
			Author:  commit.Author.Name,
			Date:    commit.Author.When,
		})
	}

	return commits, nil
}

// GetFileContent reads a file's content at a specific commit reference.
func (c *GitClient) GetFileContent(ctx context.Context, path, filePath, ref string) ([]byte, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	var hash plumbing.Hash
	if ref != "" {
		hash = plumbing.NewHash(ref)
	} else {
		h, err := repo.Head()
		if err != nil {
			return nil, fmt.Errorf("获取 HEAD 失败: %w", err)
		}
		hash = h.Hash()
	}

	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("获取 commit 失败: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("获取 tree 失败: %w", err)
	}

	file, err := tree.File(filePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件失败: %w", err)
	}

	content, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}

	return []byte(content), nil
}

// GetChangedFiles returns the list of files changed between two commits.
func (c *GitClient) GetChangedFiles(path, beforeHash, afterHash string) ([]FileDiff, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	beforeTree, err := c.getCommitTree(repo, beforeHash)
	if err != nil {
		return nil, err
	}

	afterTree, err := c.getCommitTree(repo, afterHash)
	if err != nil {
		return nil, err
	}

	if beforeTree == nil && afterTree == nil {
		return nil, nil
	}

	// Use go-git's diff to find changes
	changes, err := object.DiffTree(beforeTree, afterTree)
	if err != nil {
		return nil, fmt.Errorf("对比 tree 失败: %w", err)
	}

	var diffs []FileDiff
	for _, change := range changes {
		diff := FileDiff{
			Path: change.To.Name,
		}

		action, err := change.Action()
		if err != nil {
			continue
		}

		switch action {
		case merkletrie.Insert:
			diff.Status = "added"
		case merkletrie.Modify:
			diff.Status = "modified"
		case merkletrie.Delete:
			diff.Status = "deleted"
			diff.Path = change.From.Name
		}

		diffs = append(diffs, diff)
	}

	return diffs, nil
}

// getCommitTree returns the tree object for a given commit hash.
func (c *GitClient) getCommitTree(repo *git.Repository, hashStr string) (*object.Tree, error) {
	if hashStr == "" {
		return nil, nil
	}

	hash := plumbing.NewHash(hashStr)
	commit, err := repo.CommitObject(hash)
	if err != nil {
		return nil, fmt.Errorf("获取 commit %s 失败: %w", hashStr[:8], err)
	}

	return commit.Tree()
}
