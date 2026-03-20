package git

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"ai-sync-manager/pkg/logger"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

// Clone 克隆仓库
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

// Pull 拉取更新
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

// Push 推送提交
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

	if opts.Branch != "" {
		refName := plumbing.ReferenceName("refs/heads/" + opts.Branch)
		pushOpts.RefSpecs = []config.RefSpec{
			config.RefSpec(refName + ":" + refName),
		}
	}

	if opts.Force {
		pushOpts.Force = true
	}

	// 执行推送
	err = repo.PushContext(ctx, pushOpts)
	if err != nil {
		logger.Error("推送失败", zap.Error(err))
		return nil, fmt.Errorf("推送失败: %w", err)
	}

	logger.Info("推送成功")

	return &OperationResult{
		Success: true,
		Message: "推送成功",
	}, nil
}

// GetStatus 获取仓库状态
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

// GetRepositoryInfo 获取仓库信息
func (c *GitClient) GetRepositoryInfo(path string) (*RepositoryInfo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("打开仓库失败: %w", err)
	}

	return c.getRepositoryInfo(repo, path)
}

// getRepositoryInfo 内部方法：获取仓库信息
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

// Commit 提交更改
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

// ListBranches 列出所有分支
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
				Name:    branchName,
				IsHead:  ref.Name() == head.Name(),
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

// IsRepository 检查路径是否为 Git 仓库
func IsRepository(path string) bool {
	_, err := git.PlainOpen(path)
	return err == nil
}

// InitRepository 初始化新仓库
func InitRepository(path string, bare bool) (*RepositoryInfo, error) {
	repo, err := git.PlainInit(path, bare)
	if err != nil {
		return nil, fmt.Errorf("初始化仓库失败: %w", err)
	}

	client := NewGitClient()
	return client.getRepositoryInfo(repo, path)
}

// AddRemote 添加远程仓库
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
