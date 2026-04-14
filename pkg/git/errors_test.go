package git

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyError_Nil(t *testing.T) {
	assert.Nil(t, classifyError(nil, "操作"))
}

func TestClassifyError_Auth(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"authentication required", fmt.Errorf("authentication required")},
		{"invalid credentials", fmt.Errorf("invalid credentials for user")},
		{"unable to authenticate", fmt.Errorf("unable to authenticate")},
		{"ssh permission denied", fmt.Errorf("permission denied (publickey)")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := classifyError(tt.err, "克隆失败")
			require.NotNil(t, wrapped)

			var ge *GitError
			assert.True(t, errors.As(wrapped, &ge))
			assert.Equal(t, ErrTypeAuth, ge.Type)
			assert.Equal(t, "克隆失败", ge.Message)
		})
	}
}

func TestClassifyError_Network(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"could not resolve", fmt.Errorf("could not resolve host: github.com")},
		{"connection refused", fmt.Errorf("connection refused")},
		{"timeout", fmt.Errorf("dial tcp: timeout")},
		{"network unreachable", fmt.Errorf("network is unreachable")},
		{"dial tcp", fmt.Errorf("dial tcp 192.168.1.1:22: connect: connection refused")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := classifyError(tt.err, "拉取失败")
			var ge *GitError
			require.True(t, errors.As(wrapped, &ge))
			assert.Equal(t, ErrTypeNetwork, ge.Type)
		})
	}
}

func TestClassifyError_NotFound(t *testing.T) {
	err := fmt.Errorf("repository not found")
	wrapped := classifyError(err, "推送失败")
	var ge *GitError
	require.True(t, errors.As(wrapped, &ge))
	assert.Equal(t, ErrTypeNotFound, ge.Type)
}

func TestClassifyError_Permission(t *testing.T) {
	err := fmt.Errorf("permission denied: access denied")
	wrapped := classifyError(err, "推送失败")
	var ge *GitError
	require.True(t, errors.As(wrapped, &ge))
	assert.Equal(t, ErrTypePermission, ge.Type)
}

func TestClassifyError_SSL(t *testing.T) {
	err := fmt.Errorf("SSL certificate problem")
	wrapped := classifyError(err, "克隆失败")
	var ge *GitError
	require.True(t, errors.As(wrapped, &ge))
	assert.Equal(t, ErrTypeSSL, ge.Type)
}

func TestClassifyError_EmptyCommit(t *testing.T) {
	err := fmt.Errorf("empty commit")
	wrapped := classifyError(err, "提交失败")
	var ge *GitError
	require.True(t, errors.As(wrapped, &ge))
	assert.Equal(t, ErrTypeEmptyCommit, ge.Type)
}

func TestClassifyError_Unknown(t *testing.T) {
	err := fmt.Errorf("some unknown error")
	wrapped := classifyError(err, "操作失败")
	assert.NotNil(t, wrapped)

	var ge *GitError
	assert.False(t, errors.As(wrapped, &ge))
	assert.Contains(t, wrapped.Error(), "操作失败")
}

func TestGitError_Error(t *testing.T) {
	ge := &GitError{
		Type:    ErrTypeAuth,
		Message: "克隆失败",
		Err:     fmt.Errorf("authentication required"),
	}
	assert.Contains(t, ge.Error(), "克隆失败")
	assert.Contains(t, ge.Error(), "authentication required")

	geNoErr := &GitError{
		Type:    ErrTypeNetwork,
		Message: "拉取失败",
	}
	assert.Equal(t, "拉取失败", geNoErr.Error())
}

func TestGitError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	ge := &GitError{
		Type:    ErrTypeAuth,
		Message: "克隆失败",
		Err:     inner,
	}
	assert.Equal(t, inner, ge.Unwrap())
}

func TestIsGitError(t *testing.T) {
	err := classifyError(fmt.Errorf("authentication required"), "操作")
	assert.True(t, IsGitError(err, ErrTypeAuth))
	assert.False(t, IsGitError(err, ErrTypeNetwork))
}

func TestIsGitError_NilError(t *testing.T) {
	assert.False(t, IsGitError(nil, ErrTypeAuth))
}

func TestIsGitError_NonGitError(t *testing.T) {
	assert.False(t, IsGitError(fmt.Errorf("普通错误"), ErrTypeAuth))
}

func TestAsGitError(t *testing.T) {
	err := classifyError(fmt.Errorf("timeout"), "拉取失败")
	ge, ok := AsGitError(err)
	require.True(t, ok)
	assert.Equal(t, ErrTypeNetwork, ge.Type)

	_, ok = AsGitError(fmt.Errorf("普通错误"))
	assert.False(t, ok)
}
