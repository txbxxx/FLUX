package errors

import (
	"fmt"
)

// ErrorCode 错误码类型
type ErrorCode string

const (
	// 通用错误码 (1xxx)
	ErrCodeUnknown      ErrorCode = "1000"
	ErrCodeInternal     ErrorCode = "1001"
	ErrCodeInvalidParam  ErrorCode = "1002"
	ErrCodeNotFound     ErrorCode = "1003"
	ErrCodeAlreadyExist ErrorCode = "1004"

	// 工具相关错误码 (2xxx)
	ErrCodeToolNotInstalled ErrorCode = "2001"
	ErrCodeToolDetectFailed ErrorCode = "2002"
	ErrCodeToolConfigNotFound ErrorCode = "2003"

	// Git 相关错误码 (3xxx)
	ErrCodeGitCloneFailed    ErrorCode = "3001"
	ErrCodeGitPullFailed     ErrorCode = "3002"
	ErrCodeGitPushFailed     ErrorCode = "3003"
	ErrCodeGitAuthFailed     ErrorCode = "3004"
	ErrCodeGitRepoNotFound   ErrorCode = "3005"

	// 快照相关错误码 (4xxx)
	ErrCodeSnapshotCreateFailed  ErrorCode = "4001"
	ErrCodeSnapshotApplyFailed   ErrorCode = "4002"
	ErrCodeSnapshotNotFound      ErrorCode = "4003"
	ErrCodeSnapshotCorrupted     ErrorCode = "4004"

	// 同步相关错误码 (5xxx)
	ErrCodeSyncConflict    ErrorCode = "5001"
	ErrCodeSyncMergeFailed ErrorCode = "5002"

	// 敏感信息相关错误码 (6xxx)
	ErrCodeSensitiveDataDetected ErrorCode = "6001"
)

// AppError 应用错误类型
type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
	Err     error     `json:"-"`
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 实现 errors.Unwrap 接口
func (e *AppError) Unwrap() error {
	return e.Err
}

// New 创建新的应用错误
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap 包装已有错误
func Wrap(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// WithDetails 添加错误详情
func (e *AppError) WithDetails(details string) *AppError {
	e.Details = details
	return e
}

// Is 判断错误是否匹配
func Is(err error, target ErrorCode) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == target
	}
	return false
}

// GetCode 获取错误码
func GetCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return ErrCodeUnknown
}

// 预定义常用错误

var (
	ErrNotFound     = New(ErrCodeNotFound, "资源不存在")
	ErrInvalidParam  = New(ErrCodeInvalidParam, "参数错误")
	ErrAlreadyExist = New(ErrCodeAlreadyExist, "资源已存在")

	ErrToolNotInstalled     = New(ErrCodeToolNotInstalled, "工具未安装")
	ErrToolDetectFailed     = New(ErrCodeToolDetectFailed, "工具检测失败")
	ErrToolConfigNotFound   = New(ErrCodeToolConfigNotFound, "工具配置未找到")

	ErrGitCloneFailed  = New(ErrCodeGitCloneFailed, "Git clone 失败")
	ErrGitPullFailed   = New(ErrCodeGitPullFailed, "Git pull 失败")
	ErrGitPushFailed   = New(ErrCodeGitPushFailed, "Git push 失败")
	ErrGitAuthFailed   = New(ErrCodeGitAuthFailed, "Git 认证失败")
	ErrGitRepoNotFound = New(ErrCodeGitRepoNotFound, "Git 仓库不存在")

	ErrSnapshotCreateFailed = New(ErrCodeSnapshotCreateFailed, "快照创建失败")
	ErrSnapshotApplyFailed  = New(ErrCodeSnapshotApplyFailed, "快照应用失败")
	ErrSnapshotNotFound     = New(ErrCodeSnapshotNotFound, "快照不存在")

	ErrSyncConflict    = New(ErrCodeSyncConflict, "同步冲突")
	ErrSensitiveDataDetected = New(ErrCodeSensitiveDataDetected, "检测到敏感数据")
)
