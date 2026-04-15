package git

import (
	"time"
)

// AuthType 认证类型
type AuthType string

const (
	AuthTypeNone  AuthType = ""      // 无认证（公开仓库）
	AuthTypeSSH   AuthType = "ssh"   // SSH 密钥认证
	AuthTypeToken AuthType = "token" // Token 认证（GitHub PAT、GitLab Token 等）
	AuthTypeBasic AuthType = "basic" // 用户名密码认证
)

// GitAuthConfig Git 认证配置
type GitAuthConfig struct {
	Type       AuthType `json:"type"`                 // 认证类型
	Username   string   `json:"username,omitempty"`   // 用户名（Basic Auth）
	Password   string   `json:"password,omitempty"`   // 密码或 Token（加密存储）
	SSHKey     string   `json:"ssh_key,omitempty"`    // SSH 私钥路径或内容（加密存储）
	Passphrase string   `json:"passphrase,omitempty"` // SSH 私钥密码（可选）
}

// GitRemoteConfig Git 远端仓库配置
type GitRemoteConfig struct {
	URL        string        `json:"url"`                   // 仓库 URL
	Auth       GitAuthConfig `json:"auth,omitempty"`        // 认证配置
	Branch     string        `json:"branch,omitempty"`      // 分支名，默认 "main"
	RemoteName string        `json:"remote_name,omitempty"` // 远端名称，默认 "origin"
}

// RepositoryInfo 仓库信息
type RepositoryInfo struct {
	Path       string `json:"path"`                  // 仓库本地路径
	RemoteURL  string `json:"remote_url,omitempty"`  // 远端 URL
	Branch     string `json:"branch,omitempty"`      // 当前分支
	CommitHash string `json:"commit_hash,omitempty"` // 当前提交哈希
	IsBare     bool   `json:"is_bare"`               // 是否为裸仓库
	Head       string `json:"head,omitempty"`        // HEAD 引用
}

// CloneOptions 克隆选项
type CloneOptions struct {
	URL          string         `json:"url"`                     // 仓库 URL
	Path         string         `json:"path"`                    // 本地路径
	Auth         *GitAuthConfig `json:"auth,omitempty"`          // 认证配置
	Branch       string         `json:"branch,omitempty"`        // 要克隆的分支
	Depth        int            `json:"depth,omitempty"`         // 克隆深度（0 表示完整克隆）
	SingleBranch bool           `json:"single_branch,omitempty"` // 是否只克隆单个分支
}

// PullOptions 拉取选项
type PullOptions struct {
	Path       string         `json:"path"`                  // 仓库路径
	Auth       *GitAuthConfig `json:"auth,omitempty"`        // 认证配置
	RemoteName string         `json:"remote_name,omitempty"` // 远端名称
	Branch     string         `json:"branch,omitempty"`      // 要拉取的分支
	Force      bool           `json:"force,omitempty"`       // 是否强制拉取
}

// PushOptions 推送选项
type PushOptions struct {
	Path       string         `json:"path"`                  // 仓库路径
	Auth       *GitAuthConfig `json:"auth,omitempty"`        // 认证配置
	RemoteName string         `json:"remote_name,omitempty"` // 远端名称
	Branch     string         `json:"branch,omitempty"`      // 要推送的分支
	Force      bool           `json:"force,omitempty"`       // 是否强制推送
}

// StatusOptions 状态查询选项
type StatusOptions struct {
	Path string `json:"path"` // 仓库路径
}

// FileStatus 文件状态
type FileStatus struct {
	Path     string `json:"path"`               // 文件路径
	Worktree string `json:"worktree,omitempty"` // 工作区状态（modified, added, deleted, etc.）
	Staging  string `json:"staging,omitempty"`  // 暂存区状态
}

// RepositoryStatus 仓库状态
type RepositoryStatus struct {
	IsClean bool         `json:"is_clean"` // 是否干净（无未提交更改）
	Branch  string       `json:"branch"`   // 当前分支
	Files   []FileStatus `json:"files"`    // 有更改的文件
	Ahead   int          `json:"ahead"`    // 领先提交数
	Behind  int          `json:"behind"`   // 落后提交数
}

// CommitOptions 提交选项
type CommitOptions struct {
	Path    string `json:"path"`             // 仓库路径
	Message string `json:"message"`          // 提交消息
	Author  string `json:"author,omitempty"` // 作者信息（可选）
	All     bool   `json:"all,omitempty"`    // 是否添加所有更改
}

// CommitResult 提交结果
type CommitResult struct {
	Success    bool      `json:"success"`               // 是否成功
	CommitHash string    `json:"commit_hash,omitempty"` // 提交哈希
	Message    string    `json:"message,omitempty"`     // 提交消息
	Time       time.Time `json:"time,omitempty"`        // 提交时间
}

// BranchInfo 分支信息
type BranchInfo struct {
	Name     string `json:"name"`                // 分支名称
	IsHead   bool   `json:"is_head,omitempty"`   // 是否为当前分支
	IsRemote bool   `json:"is_remote,omitempty"` // 是否为远程分支
}

// OperationResult 操作结果
type OperationResult struct {
	Success bool   `json:"success"`         // 是否成功
	Message string `json:"message"`         // 结果消息
	Error   string `json:"error,omitempty"` // 错误信息（如果失败）
}

// FetchOptions fetch 选项
type FetchOptions struct {
	Path   string         `json:"path"`            // 仓库路径
	Auth   *GitAuthConfig `json:"auth,omitempty"`  // 认证配置
	Remote string         `json:"remote,omitempty"` // 远端名称（默认 origin）
}

// LogOptions 日志查询选项
type LogOptions struct {
	Path     string    `json:"path"`               // 仓库路径
	FilePath string    `json:"file_path,omitempty"` // 文件路径过滤（可选）
	Since    time.Time `json:"since,omitempty"`    // 起始时间（可选）
	Until    time.Time `json:"until,omitempty"`    // 截止时间（可选）
	Limit    int       `json:"limit,omitempty"`    // 最大返回数量（默认 50）
}

// CommitInfo 提交信息
type CommitInfo struct {
	Hash    string    `json:"hash"`    // 提交哈希
	Message string    `json:"message"` // 提交消息
	Author  string    `json:"author"`  // 作者
	Date    time.Time `json:"date"`    // 提交时间
}

// FileDiff 文件差异描述
type FileDiff struct {
	Path    string `json:"path"`              // 文件路径
	Status  string `json:"status"`            // added / modified / deleted
	OldHash string `json:"old_hash"`           // 变更前哈希
	NewHash string `json:"new_hash"`           // 变更后哈希
}

// DiffOptions git diff 选项
type DiffOptions struct {
	Path    string `json:"path"`             // 仓库路径
	Against string `json:"against,omitempty"` // 对比基准（默认为 origin/<branch>）
}

// DiffResult git diff 结果
type DiffResult struct {
	Added    int      `json:"added"`              // 新增文件数
	Modified int      `json:"modified"`           // 修改文件数
	Deleted  int      `json:"deleted"`            // 删除文件数
	Files    []string `json:"files,omitempty"`    // 变更文件路径
}
