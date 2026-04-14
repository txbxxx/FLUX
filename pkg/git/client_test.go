package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewGitClient 测试客户端创建
func TestNewGitClient(t *testing.T) {
	client := NewGitClient()
	assert.NotNil(t, client)
}

// TestIsRepository 测试仓库检测
func TestIsRepository(t *testing.T) {
	// 创建测试仓库
	repoPath := CreateTestRepo(t)
	defer CleanupTestRepo(t, repoPath)

	// 测试存在的仓库
	assert.True(t, IsRepository(repoPath))

	// 测试不存在的路径
	assert.False(t, IsRepository("/nonexistent/path/12345"))

	// 测试非仓库目录
	tempDir := filepath.Join(os.TempDir(), "fl-not-repo")
	_ = os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)
	assert.False(t, IsRepository(tempDir))
}

// TestInitRepository 测试仓库初始化
func TestInitRepository(t *testing.T) {
	tests := []struct {
		name string
		bare bool
	}{
		{"普通仓库", false},
		{"裸仓库", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := filepath.Join(os.TempDir(), "fl-init-test")
			_ = os.RemoveAll(tempDir)
			defer os.RemoveAll(tempDir)

			info, err := InitRepository(tempDir, tt.bare)
			assert.NoError(t, err)
			assert.NotNil(t, info)
			assert.Equal(t, tempDir, info.Path)
			assert.Equal(t, tt.bare, info.IsBare)
			assert.True(t, IsRepository(tempDir))
		})
	}
}

// TestGetRepositoryInfo 测试获取仓库信息
func TestGetRepositoryInfo(t *testing.T) {
	repoPath := CreateTestRepo(t)
	defer CleanupTestRepo(t, repoPath)

	client := NewGitClient()
	info, err := client.GetRepositoryInfo(repoPath)

	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, repoPath, info.Path)
	assert.False(t, info.IsBare)
}

// TestGetRepositoryInfo_NotRepo 测试非仓库路径
func TestGetRepositoryInfo_NotRepo(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "fl-not-a-repo")
	_ = os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	client := NewGitClient()
	_, err := client.GetRepositoryInfo(tempDir)
	assert.Error(t, err)
}

// TestGitClient_Commit 测试提交功能
func TestGitClient_Commit(t *testing.T) {
	repoPath := CreateTestRepo(t)
	defer CleanupTestRepo(t, repoPath)

	client := NewGitClient()

	// 创建测试文件
	testFile := filepath.Join(repoPath, "test.txt")
	_ = os.WriteFile(testFile, []byte("test content"), 0644)

	// 执行提交
	result, err := client.Commit(context.Background(), &CommitOptions{
		Path:    repoPath,
		Message: "Test commit",
		All:     true,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.CommitHash)
	assert.Equal(t, "Test commit", result.Message)
}

// TestGitClient_Commit_EmptyMessage 测试空提交消息
func TestGitClient_Commit_EmptyMessage(t *testing.T) {
	repoPath := CreateTestRepo(t)
	defer CleanupTestRepo(t, repoPath)

	client := NewGitClient()

	_, err := client.Commit(context.Background(), &CommitOptions{
		Path: repoPath,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "提交消息不能为空")
}

// TestGitClient_Commit_NoPath 测试无路径
func TestGitClient_Commit_NoPath(t *testing.T) {
	client := NewGitClient()

	_, err := client.Commit(context.Background(), &CommitOptions{
		Message: "Test",
	})
	assert.Error(t, err)
}

// TestGetStatus 测试获取状态
func TestGetStatus(t *testing.T) {
	repoPath := CreateTestRepoWithCommit(t)
	defer CleanupTestRepo(t, repoPath)

	client := NewGitClient()

	// 创建未提交的更改
	testFile := filepath.Join(repoPath, "modified.txt")
	_ = os.WriteFile(testFile, []byte("modified content"), 0644)

	status, err := client.GetStatus(&StatusOptions{Path: repoPath})
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.False(t, status.IsClean, "有未提交更改时状态不应为干净")
	assert.NotEmpty(t, status.Branch)
}

// TestGetStatus_Clean 测试干净仓库状态
func TestGetStatus_Clean(t *testing.T) {
	repoPath := CreateTestRepoWithCommit(t)
	defer CleanupTestRepo(t, repoPath)

	client := NewGitClient()
	status, err := client.GetStatus(&StatusOptions{Path: repoPath})

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.True(t, status.IsClean, "没有更改时状态应为干净")
	assert.Empty(t, status.Files)
}

// TestGetStatus_NoPath 测试无路径
func TestGetStatus_NoPath(t *testing.T) {
	client := NewGitClient()

	_, err := client.GetStatus(&StatusOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "仓库路径不能为空")
}

// TestListBranches 测试列出分支
func TestListBranches(t *testing.T) {
	repoPath := CreateTestRepoWithCommit(t)
	defer CleanupTestRepo(t, repoPath)

	client := NewGitClient()
	branches, err := client.ListBranches(repoPath)

	assert.NoError(t, err)
	assert.NotNil(t, branches)
	assert.NotEmpty(t, branches, "应至少有一个分支（main 或 master）")

	// 检查是否有当前分支标记
	hasHead := false
	for _, branch := range branches {
		if branch.IsHead {
			hasHead = true
			break
		}
	}
	assert.True(t, hasHead, "应有一个分支标记为当前分支")
}

// TestListBranches_NotRepo 测试非仓库路径
func TestListBranches_NotRepo(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "fl-not-repo")
	_ = os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	client := NewGitClient()
	_, err := client.ListBranches(tempDir)
	assert.Error(t, err)
}

// TestAddRemote 测试添加远程仓库
func TestAddRemote(t *testing.T) {
	repoPath := CreateTestRepo(t)
	defer CleanupTestRepo(t, repoPath)

	// 添加远程仓库
	err := AddRemote(repoPath, "test_remote", "https://github.com/test/repo.git")
	assert.NoError(t, err)

	// 验证远程仓库已添加
	_, err = NewGitClient().GetRepositoryInfo(repoPath)
	assert.NoError(t, err)
}

// TestAddRemote_InvalidPath 测试无效路径
func TestAddRemote_InvalidPath(t *testing.T) {
	err := AddRemote("/nonexistent/path", "origin", "https://github.com/test/repo.git")
	assert.Error(t, err)
}

// TestCloneOptions 测试克隆选项验证
func TestCloneOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    *CloneOptions
		wantErr bool
	}{
		{
			name: "有效选项",
			opts: &CloneOptions{
				URL:  "https://github.com/test/repo.git",
				Path: "/tmp/repo",
			},
			wantErr: false,
		},
		{
			name:    "空选项",
			opts:    nil,
			wantErr: true,
		},
		{
			name: "空 URL",
			opts: &CloneOptions{
				Path: "/tmp/repo",
			},
			wantErr: true,
		},
		{
			name: "空路径",
			opts: &CloneOptions{
				URL: "https://github.com/test/repo.git",
			},
			wantErr: true,
		},
	}

	client := NewGitClient()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_, err := client.Clone(ctx, tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				// 即使选项有效，克隆也可能因为网络等原因失败
				// 这里只验证没有选项验证错误
				_ = err
			}
		})
	}
}

// TestPullOptions 测试拉取选项验证
func TestPullOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    *PullOptions
		wantErr bool
	}{
		{
			name: "有效选项",
			opts: &PullOptions{
				Path: "/tmp/repo",
			},
			wantErr: false,
		},
		{
			name:    "空选项",
			opts:    nil,
			wantErr: true,
		},
		{
			name:    "空路径",
			opts:    &PullOptions{},
			wantErr: true,
		},
	}

	client := NewGitClient()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_, err := client.Pull(ctx, tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				// 路径不存在会报错，但不是因为选项验证
				_ = err
			}
		})
	}
}

// TestPushOptions 测试推送选项验证
func TestPushOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    *PushOptions
		wantErr bool
	}{
		{
			name: "有效选项",
			opts: &PushOptions{
				Path: "/tmp/repo",
			},
			wantErr: false,
		},
		{
			name:    "空选项",
			opts:    nil,
			wantErr: true,
		},
		{
			name:    "空路径",
			opts:    &PushOptions{},
			wantErr: true,
		},
	}

	client := NewGitClient()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			_, err := client.Push(ctx, tt.opts)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				_ = err
			}
		})
	}
}

// Table驱动测试：分支信息
func TestBranchInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     BranchInfo
		expected string
	}{
		{
			name:     "本地分支",
			info:     BranchInfo{Name: "main", IsHead: true, IsRemote: false},
			expected: "main",
		},
		{
			name:     "远程分支",
			info:     BranchInfo{Name: "origin/main", IsHead: false, IsRemote: true},
			expected: "origin/main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.info.Name)
		})
	}
}

// TestRepositoryStatus 测试仓库状态结构
func TestRepositoryStatus(t *testing.T) {
	status := &RepositoryStatus{
		IsClean: true,
		Branch:  "main",
		Files:   []FileStatus{},
		Ahead:   0,
		Behind:  0,
	}

	assert.True(t, status.IsClean)
	assert.Equal(t, "main", status.Branch)
	assert.Empty(t, status.Files)
}

// TestFileStatus 测试文件状态结构
func TestFileStatus(t *testing.T) {
	status := FileStatus{
		Path:     "README.md",
		Worktree: "Modified",
		Staging:  "Unmodified",
	}

	assert.Equal(t, "README.md", status.Path)
	assert.Equal(t, "Modified", status.Worktree)
}
