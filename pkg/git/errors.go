package git

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorType 分类标识 git 操作中可能出现的各类错误。
type ErrorType int

const (
	ErrTypeAuth           ErrorType = iota // 认证失败
	ErrTypeNetwork                         // 网络问题
	ErrTypeNotFound                        // 仓库不存在
	ErrTypePermission                      // 权限不足
	ErrTypeSSL                             // SSL/TLS 证书
	ErrTypeEmptyCommit                     // 空提交（无变更）
)

// GitError 自定义错误，携带分类信息，便于调用方按类型处理。
type GitError struct {
	Type    ErrorType
	Message string
	Err     error
}

// Error 实现 error 接口。
func (e *GitError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap 返回被包装的原始错误，支持 errors.As/Is 链式查找。
func (e *GitError) Unwrap() error {
	return e.Err
}

// IsGitError 检查 err 链中是否存在指定类型的 GitError。
func IsGitError(err error, t ErrorType) bool {
	var ge *GitError
	if errors.As(err, &ge) {
		return ge.Type == t
	}
	return false
}

// AsGitError 提取 GitError，成功返回 true。
func AsGitError(err error) (*GitError, bool) {
	var ge *GitError
	if errors.As(err, &ge) {
		return ge, true
	}
	return nil, false
}

// classifyError 将底层 go-git 错误分类为 GitError。
// 所有字符串匹配集中在此函数，调用方无需关心底层错误文本。
func classifyError(err error, opMsg string) error {
	if err == nil {
		return nil
	}

	lower := strings.ToLower(err.Error())

	// 第一步：认证失败
	if containsAny(lower,
		"authentication required",
		"invalid credentials",
		"unable to authenticate",
	) || (strings.Contains(lower, "permission denied") && strings.Contains(lower, "publickey")) {
		return &GitError{Type: ErrTypeAuth, Message: opMsg, Err: err}
	}

	// 第二步：网络连接失败
	if containsAny(lower,
		"could not resolve",
		"connection refused",
		"timeout",
		"network is unreachable",
		"dial tcp",
	) {
		return &GitError{Type: ErrTypeNetwork, Message: opMsg, Err: err}
	}

	// 第三步：仓库不存在
	if containsAny(lower,
		"repository not found",
		"does not appear to be a git repository",
	) {
		return &GitError{Type: ErrTypeNotFound, Message: opMsg, Err: err}
	}

	// 第四步：权限不足（非 SSH 的 permission denied）
	if containsAny(lower, "permission denied", "403", "forbidden") {
		return &GitError{Type: ErrTypePermission, Message: opMsg, Err: err}
	}

	// 第五步：SSL/TLS 证书问题
	if containsAny(lower, "ssl", "certificate", "tls: ") {
		return &GitError{Type: ErrTypeSSL, Message: opMsg, Err: err}
	}

	// 第六步：空提交
	if containsAny(lower, "empty commit", "nothing to commit") {
		return &GitError{Type: ErrTypeEmptyCommit, Message: opMsg, Err: err}
	}

	// 未匹配到已知模式，用 opMsg 包装原始错误返回
	return fmt.Errorf("%s: %w", opMsg, err)
}

// containsAny 判断 s 是否包含 subs 中任一子串（s 应已转小写）。
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
