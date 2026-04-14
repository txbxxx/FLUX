package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateAuthConfig_None 测试无认证配置验证
func TestValidateAuthConfig_None(t *testing.T) {
	err := ValidateAuthConfig(nil)
	assert.NoError(t, err)

	err = ValidateAuthConfig(&GitAuthConfig{Type: AuthTypeNone})
	assert.NoError(t, err)
}

// TestValidateAuthConfig_Token 测试 Token 认证配置验证
func TestValidateAuthConfig_Token(t *testing.T) {
	tests := []struct {
		name    string
		auth    *GitAuthConfig
		wantErr bool
	}{
		{
			name: "有效 Token 配置",
			auth: &GitAuthConfig{
				Type:     AuthTypeToken,
				Password: "ghp_test_token",
			},
			wantErr: false,
		},
		{
			name: "缺少 Token",
			auth: &GitAuthConfig{
				Type: AuthTypeToken,
			},
			wantErr: true,
		},
		{
			name: "空 Token",
			auth: &GitAuthConfig{
				Type:     AuthTypeToken,
				Password: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAuthConfig(tt.auth)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateAuthConfig_Basic 测试 Basic 认证配置验证
func TestValidateAuthConfig_Basic(t *testing.T) {
	tests := []struct {
		name    string
		auth    *GitAuthConfig
		wantErr bool
	}{
		{
			name: "有效 Basic 配置",
			auth: &GitAuthConfig{
				Type:     AuthTypeBasic,
				Username: "testuser",
				Password: "testpass",
			},
			wantErr: false,
		},
		{
			name: "缺少用户名",
			auth: &GitAuthConfig{
				Type:     AuthTypeBasic,
				Password: "testpass",
			},
			wantErr: true,
		},
		{
			name: "缺少密码",
			auth: &GitAuthConfig{
				Type:     AuthTypeBasic,
				Username: "testuser",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAuthConfig(tt.auth)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateAuthConfig_SSH 测试 SSH 认证配置验证
func TestValidateAuthConfig_SSH(t *testing.T) {
	// SSH 配置验证不会失败，因为密钥会在使用时验证
	auth := &GitAuthConfig{
		Type:   AuthTypeSSH,
		SSHKey: "/path/to/key",
	}

	err := ValidateAuthConfig(auth)
	assert.NoError(t, err)
}

// TestValidateAuthConfig_Unknown 测试未知认证类型
func TestValidateAuthConfig_Unknown(t *testing.T) {
	auth := &GitAuthConfig{
		Type: AuthType("unknown"),
	}

	err := ValidateAuthConfig(auth)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不支持的认证类型")
}

// TestParseGitURL 测试 Git URL 解析
func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		expectedType AuthType
		wantErr      bool
	}{
		{
			name:         "HTTPS URL",
			url:          "https://github.com/user/repo.git",
			expectedType: AuthTypeToken,
			wantErr:      false,
		},
		{
			name:         "HTTP URL",
			url:          "http://gitlab.com/user/repo.git",
			expectedType: AuthTypeToken,
			wantErr:      false,
		},
		{
			name:         "SSH URL (git@)",
			url:          "git@github.com:user/repo.git",
			expectedType: AuthTypeSSH,
			wantErr:      false,
		},
		{
			name:         "SSH URL (ssh://)",
			url:          "ssh://git@github.com/user/repo.git",
			expectedType: AuthTypeSSH,
			wantErr:      false,
		},
		{
			name:         "Git 协议",
			url:          "git://github.com/user/repo.git",
			expectedType: AuthTypeNone,
			wantErr:      false,
		},
		{
			name:         "本地路径",
			url:          "/path/to/repo",
			expectedType: AuthTypeNone,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, authType, err := ParseGitURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedType, authType)
			}
		})
	}
}

// TestIsGitURL 测试 Git URL 验证
func TestIsGitURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"HTTPS URL", "https://github.com/user/repo.git", true},
		{"HTTP URL", "http://gitlab.com/user/repo.git", true},
		{"SSH URL", "git@github.com:user/repo.git", true},
		{"SSH with protocol", "ssh://git@github.com/user/repo.git", true},
		{"Git protocol", "git://github.com/user/repo.git", true},
		{"File protocol", "file:///path/to/repo", true},
		{"绝对路径", "/path/to/repo", true},
		{"相对路径", "./repo", true},
		{"父目录路径", "../repo", true},
		{"空字符串", "", false},
		{"无效 URL", "not-a-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsGitURL(tt.url)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestExtractRepoInfo 测试仓库信息提取
func TestExtractRepoInfo(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		expectedHost  string
		expectedOwner string
		expectedRepo  string
		wantErr       bool
	}{
		{
			name:          "GitHub HTTPS",
			url:           "https://github.com/user/repo.git",
			expectedHost:  "github.com",
			expectedOwner: "user",
			expectedRepo:  "repo",
			wantErr:       false,
		},
		{
			name:          "GitLab HTTPS",
			url:           "https://gitlab.com/group/project.git",
			expectedHost:  "gitlab.com",
			expectedOwner: "group",
			expectedRepo:  "project",
			wantErr:       false,
		},
		{
			name:          "GitHub SSH",
			url:           "git@github.com:user/repo.git",
			expectedHost:  "github.com",
			expectedOwner: "user",
			expectedRepo:  "repo",
			wantErr:       false,
		},
		{
			name:          "无 .git 后缀",
			url:           "https://github.com/user/repo",
			expectedHost:  "github.com",
			expectedOwner: "user",
			expectedRepo:  "repo",
			wantErr:       false,
		},
		{
			name:         "无效 URL",
			url:          "not-a-valid-url",
			expectedHost: "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ExtractRepoInfo(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedHost, info.Host)
				assert.Equal(t, tt.expectedOwner, info.Owner)
				assert.Equal(t, tt.expectedRepo, info.RepoName)
			}
		})
	}
}

// TestGetDefaultBranch 测试默认分支获取
func TestGetDefaultBranch(t *testing.T) {
	branch := GetDefaultBranch()
	assert.Equal(t, "main", branch)
}

// Table驱动测试：认证类型枚举
func TestAuthType_Values(t *testing.T) {
	tests := []struct {
		name     string
		authType AuthType
		expected string
	}{
		{"无认证", AuthTypeNone, ""},
		{"SSH", AuthTypeSSH, "ssh"},
		{"Token", AuthTypeToken, "token"},
		{"Basic", AuthTypeBasic, "basic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.authType))
		})
	}
}
