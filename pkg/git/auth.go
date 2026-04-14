package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"

	"flux/pkg/logger"

	"go.uber.org/zap"
)

// NewAuthMethod 创建认证方法
func NewAuthMethod(auth *GitAuthConfig) (transport.AuthMethod, error) {
	if auth == nil || auth.Type == AuthTypeNone {
		return nil, nil
	}

	switch auth.Type {
	case AuthTypeToken:
		return &http.BasicAuth{
			Username: "token", // 某些 Git 服务需要用户名（如 GitHub 用 token）
			Password: auth.Password,
		}, nil

	case AuthTypeBasic:
		return &http.BasicAuth{
			Username: auth.Username,
			Password: auth.Password,
		}, nil

	case AuthTypeSSH:
		return newSSHAuth(auth)

	default:
		return nil, fmt.Errorf("不支持的认证类型: %s", auth.Type)
	}
}

// newSSHAuth 创建 SSH 认证
func newSSHAuth(auth *GitAuthConfig) (transport.AuthMethod, error) {
	// 尝试多种方式获取 SSH 密钥
	var keyFiles []string

	// 1. 如果配置中指定了 SSH 密钥路径
	if auth.SSHKey != "" && filepath.IsAbs(auth.SSHKey) {
		keyPath := auth.SSHKey
		// 自动修正：用户可能指定了 .pub（公钥），需要使用对应的私钥
		if strings.HasSuffix(keyPath, ".pub") {
			privateKey := strings.TrimSuffix(keyPath, ".pub")
			if _, err := os.Stat(privateKey); err == nil {
				logger.Warn("配置的是公钥，已自动切换到私钥",
					zap.String("public_key", keyPath),
					zap.String("private_key", privateKey),
				)
				keyPath = privateKey
			}
		}
		if _, err := os.Stat(keyPath); err == nil {
			keyFiles = append(keyFiles, keyPath)
		}
	}

	// 2. 尝试默认的 SSH 密钥位置
	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultKeys := []string{
			"id_ed25519",
			"id_ed25519_sk",
			"id_rsa",
			"id_ecdsa",
			"id_ecdsa_sk",
			"id_dsa",
		}
		sshDir := filepath.Join(homeDir, ".ssh")

		for _, key := range defaultKeys {
			keyPath := filepath.Join(sshDir, key)
			if _, err := os.Stat(keyPath); err == nil {
				keyFiles = append(keyFiles, keyPath)
			}
		}
	}

	// 3. 如果配置中提供了密钥内容，创建临时文件
	if auth.SSHKey != "" && !filepath.IsAbs(auth.SSHKey) && strings.Contains(auth.SSHKey, "BEGIN") {
		// 密钥内容，创建临时文件
		tmpFile, err := os.CreateTemp("", "ssh-key-*.pem")
		if err == nil {
			defer tmpFile.Close()
			if _, err := tmpFile.WriteString(auth.SSHKey); err == nil {
				keyFiles = append([]string{tmpFile.Name()}, keyFiles...)
			}
		}
	}

	if len(keyFiles) == 0 {
		return nil, fmt.Errorf("未找到 SSH 私钥")
	}

	// 尝试使用不同的私钥文件
	for _, keyFile := range keyFiles {
		signer, err := loadSSHSigner(keyFile, auth.Passphrase)
		if err != nil {
			continue
		}
		return &gitssh.PublicKeys{
			User:   "git",
			Signer: signer,
			HostKeyCallbackHelper: gitssh.HostKeyCallbackHelper{
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			},
		}, nil
	}

	return nil, fmt.Errorf("所有 SSH 密钥尝试失败，请检查密钥格式和密码")
}

// loadSSHSigner loads an SSH private key and returns a signer.
func loadSSHSigner(keyFile, passphrase string) (ssh.Signer, error) {
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("读取密钥文件失败: %w", err)
	}

	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
		if err != nil {
			return nil, err
		}
		return signer, nil
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		// 密钥可能需要密码
		if strings.Contains(err.Error(), "password protected") ||
			strings.Contains(err.Error(), "encrypted") ||
			strings.Contains(err.Error(), "passphrase") {
			return nil, fmt.Errorf("密钥需要密码，请配置 --password 参数")
		}
		return nil, err
	}
	return signer, nil
}

// ValidateAuthConfig 验证认证配置
func ValidateAuthConfig(auth *GitAuthConfig) error {
	if auth == nil || auth.Type == AuthTypeNone {
		return nil
	}

	switch auth.Type {
	case AuthTypeToken:
		if auth.Password == "" {
			return fmt.Errorf("Token 认证需要提供 token")
		}

	case AuthTypeBasic:
		if auth.Username == "" || auth.Password == "" {
			return fmt.Errorf("Basic 认证需要提供用户名和密码")
		}

	case AuthTypeSSH:
		// SSH 密钥会在实际使用时验证

	default:
		return fmt.Errorf("不支持的认证类型: %s", auth.Type)
	}

	return nil
}

// ParseGitURL 解析 Git URL 以确定认证类型
func ParseGitURL(url string) (string, AuthType, error) {
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return url, AuthTypeToken, nil
	}

	if strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://") {
		return url, AuthTypeSSH, nil
	}

	if strings.HasPrefix(url, "git://") {
		return url, AuthTypeNone, nil
	}

	// 无法确定类型
	return url, AuthTypeNone, fmt.Errorf("无法从 URL 确定认证类型: %s", url)
}

// GetDefaultBranch 获取默认分支名
func GetDefaultBranch() string {
	// 现代 Git 默认使用 main
	return "main"
}

// IsGitURL 检查是否为有效的 Git URL
func IsGitURL(url string) bool {
	if url == "" {
		return false
	}

	prefixes := []string{
		"https://",
		"http://",
		"git@",
		"ssh://",
		"git://",
		"file://",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}

	// 检查是否为本地路径
	if strings.HasPrefix(url, "/") || strings.HasPrefix(url, "./") || strings.HasPrefix(url, "../") {
		return true
	}

	return false
}

// ExtractRepoInfo 从 URL 中提取仓库信息
type RepoInfo struct {
	Host     string // 主机（github.com, gitlab.com 等）
	Owner    string // 所有者
	RepoName string // 仓库名
}

// ExtractRepoInfo 从 Git URL 中提取仓库信息
func ExtractRepoInfo(url string) (*RepoInfo, error) {
	info := &RepoInfo{}

	// 处理 HTTPS URL
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		// 移除协议和可能的认证信息
		url = strings.TrimPrefix(url, "https://")
		url = strings.TrimPrefix(url, "http://")

		// 移除认证信息
		if idx := strings.Index(url, "@"); idx != -1 {
			url = url[idx+1:]
		}

		// 移除 .git 后缀
		url = strings.TrimSuffix(url, ".git")

		// 分割路径
		parts := strings.Split(url, "/")
		if len(parts) >= 2 {
			info.Host = parts[0]
			info.Owner = parts[1]
			if len(parts) >= 3 {
				info.RepoName = parts[2]
			}
		}
	}

	// 处理 SSH URL (git@github.com:owner/repo.git)
	if strings.HasPrefix(url, "git@") {
		url = strings.TrimPrefix(url, "git@")
		url = strings.TrimSuffix(url, ".git")

		// 分割主机和路径
		if idx := strings.Index(url, ":"); idx != -1 {
			info.Host = url[:idx]
			pathParts := strings.Split(url[idx+1:], "/")
			if len(pathParts) >= 2 {
				info.Owner = pathParts[0]
				info.RepoName = pathParts[1]
			}
		}
	}

	if info.Host == "" {
		return nil, fmt.Errorf("无法解析仓库 URL: %s", url)
	}

	return info, nil
}
