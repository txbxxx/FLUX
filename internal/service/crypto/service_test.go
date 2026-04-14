package crypto

import (
	"os"
	"path/filepath"
	"testing"

	"flux/internal/models"
	"flux/internal/types/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestService_EncryptDecrypt 测试加密解密
func TestService_EncryptDecrypt(t *testing.T) {
	config := &models.EncryptionConfig{
		Enabled:   true,
		Algorithm: "aes256-gcm",
		KeyEnvVar: "", // 不使用环境变量
		KeyPath:   "", // 将自动生成
	}

	svc, err := NewService(config)
	require.NoError(t, err)
	require.True(t, svc.IsEnabled())

	// 测试加密
	plaintext := "Hello, World!"
	encryptResult, err := svc.Encrypt(plaintext)
	require.NoError(t, err)
	assert.True(t, encryptResult.Success)
	assert.NotEmpty(t, encryptResult.Data)
	assert.NotEqual(t, plaintext, encryptResult.Data)
	assert.Equal(t, "aes256-gcm", encryptResult.Algorithm)

	// 测试解密
	decryptResult, err := svc.Decrypt(encryptResult.Data)
	require.NoError(t, err)
	assert.True(t, decryptResult.Success)
	assert.Equal(t, plaintext, decryptResult.Data)
}

// TestService_Disabled 未启用加密
func TestService_Disabled(t *testing.T) {
	config := &models.EncryptionConfig{
		Enabled: false,
	}

	svc, err := NewService(config)
	require.NoError(t, err)
	assert.False(t, svc.IsEnabled())

	// 加密应该直接返回原文
	plaintext := "Hello, World!"
	encryptResult, err := svc.Encrypt(plaintext)
	require.NoError(t, err)
	assert.True(t, encryptResult.Success)
	assert.Equal(t, plaintext, encryptResult.Data)
	assert.Equal(t, "none", encryptResult.Algorithm)

	// 解密应该直接返回原文
	decryptResult, err := svc.Decrypt(plaintext)
	require.NoError(t, err)
	assert.True(t, decryptResult.Success)
	assert.Equal(t, plaintext, decryptResult.Data)
}

// TestService_EncryptPassword 测试密码加密
func TestService_EncryptPassword(t *testing.T) {
	config := &models.EncryptionConfig{
		Enabled:   true,
		Algorithm: "aes256-gcm",
	}

	svc, err := NewService(config)
	require.NoError(t, err)

	password := "my-secret-password-123"

	// 加密
	encrypted, err := svc.EncryptPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, password, encrypted)

	// 解密
	decrypted, err := svc.DecryptPassword(encrypted)
	require.NoError(t, err)
	assert.Equal(t, password, decrypted)
}

// TestService_EncryptToken 测试 Token 加密
func TestService_EncryptToken(t *testing.T) {
	config := &models.EncryptionConfig{
		Enabled:   true,
		Algorithm: "aes256-gcm",
	}

	svc, err := NewService(config)
	require.NoError(t, err)

	token := "ghp_test_token_12345"

	// 加密
	encrypted, err := svc.EncryptToken(token)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// 解密
	decrypted, err := svc.DecryptToken(encrypted)
	require.NoError(t, err)
	assert.Equal(t, token, decrypted)
}

// TestService_EncryptSSHKey 测试 SSH 密钥加密
func TestService_EncryptSSHKey(t *testing.T) {
	config := &models.EncryptionConfig{
		Enabled:   true,
		Algorithm: "aes256-gcm",
	}

	svc, err := NewService(config)
	require.NoError(t, err)

	sshKey := `-----BEGIN OPENSSH PRIVATE KEY-----
test ssh key content
-----END OPENSSH PRIVATE KEY-----`

	// 加密
	encrypted, err := svc.EncryptSSHKey(sshKey)
	require.NoError(t, err)
	assert.NotEmpty(t, encrypted)

	// 解密
	decrypted, err := svc.DecryptSSHKey(encrypted)
	require.NoError(t, err)
	assert.Equal(t, sshKey, decrypted)
}

// TestService_EncryptSensitiveData 测试敏感数据加密
func TestService_EncryptSensitiveData(t *testing.T) {
	config := &models.EncryptionConfig{
		Enabled:   true,
		Algorithm: "aes256-gcm",
	}

	svc, err := NewService(config)
	require.NoError(t, err)

	// 加密 Token
	sensitiveData, err := svc.EncryptSensitiveData(
		common.SensitiveTypeToken,
		"test-token-value",
	)
	require.NoError(t, err)
	assert.Equal(t, common.SensitiveTypeToken, sensitiveData.Type)
	assert.NotEmpty(t, sensitiveData.Content)

	// 解密
	decrypted, err := svc.DecryptSensitiveData(sensitiveData)
	require.NoError(t, err)
	assert.Equal(t, "test-token-value", decrypted)
}

// TestService_DecryptInvalidData 测试解密无效数据
func TestService_DecryptInvalidData(t *testing.T) {
	config := &models.EncryptionConfig{
		Enabled:   true,
		Algorithm: "aes256-gcm",
	}

	svc, err := NewService(config)
	require.NoError(t, err)

	// 测试无效 Base64
	result, err := svc.Decrypt("invalid-base64!!!")
	require.NoError(t, err) // 不应该返回错误
	assert.False(t, result.Success)
	assert.NotEmpty(t, result.Error)

	// 测试有效 Base64 但无效密文
	result, err = svc.Decrypt("dGVzdA==") // "test" in base64
	require.NoError(t, err)
	assert.False(t, result.Success)
}

// TestService_KeyFromFile 测试从文件加载密钥
func TestService_KeyFromFile(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, ".test-key")

	// 生成一个密钥并保存
	key, err := GenerateKey()
	require.NoError(t, err)

	err = os.WriteFile(keyPath, []byte(key), 0600)
	require.NoError(t, err)

	// 使用密钥文件创建服务
	config := &models.EncryptionConfig{
		Enabled:   true,
		Algorithm: "aes256-gcm",
		KeyPath:   keyPath,
	}

	svc, err := NewService(config)
	require.NoError(t, err)

	// 测试加密解密
	plaintext := "test-data-from-file-key"
	encrypted, err := svc.EncryptPassword(plaintext)
	require.NoError(t, err)

	decrypted, err := svc.DecryptPassword(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestService_KeyFromEnvVar 测试从环境变量获取密钥
func TestService_KeyFromEnvVar(t *testing.T) {
	// 设置环境变量
	testKey := "test-env-key-for-encryption"
	t.Setenv("TEST_ENCRYPTION_KEY", testKey)
	defer os.Unsetenv("TEST_ENCRYPTION_KEY")

	config := &models.EncryptionConfig{
		Enabled:   true,
		Algorithm: "aes256-gcm",
		KeyEnvVar: "TEST_ENCRYPTION_KEY",
	}

	svc, err := NewService(config)
	require.NoError(t, err)

	// 测试加密解密
	plaintext := "test-data-from-env-key"
	encrypted, err := svc.EncryptPassword(plaintext)
	require.NoError(t, err)

	decrypted, err := svc.DecryptPassword(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

// TestGenerateKey 测试密钥生成
func TestGenerateKey(t *testing.T) {
	key1, err := GenerateKey()
	require.NoError(t, err)
	assert.NotEmpty(t, key1)
	// Base64 编码的 32 字节应该是 44 字符
	assert.Len(t, key1, 44)

	// 再次生成应该不同
	key2, err := GenerateKey()
	require.NoError(t, err)
	assert.NotEmpty(t, key2)
	assert.NotEqual(t, key1, key2)
}

// TestValidateKey 测试密钥验证
func TestValidateKey(t *testing.T) {
	// 生成有效密钥
	validKey, err := GenerateKey()
	require.NoError(t, err)

	// 验证有效密钥
	err = ValidateKey(validKey)
	assert.NoError(t, err)

	// 验证无效 Base64
	err = ValidateKey("invalid-base64!!!")
	assert.Error(t, err)

	// 验证长度不足
	shortKey := "dGVzdA==" // "test" in base64
	err = ValidateKey(shortKey)
	assert.Error(t, err)
}

// TestHashKey 测试密钥哈希
func TestHashKey(t *testing.T) {
	key := "test-encryption-key"

	hash1 := HashKey(key)
	assert.NotEmpty(t, hash1)
	assert.NotEqual(t, key, hash1)

	// 相同的密钥应该产生相同的哈希
	hash2 := HashKey(key)
	assert.Equal(t, hash1, hash2)

	// 不同的密钥应该产生不同的哈希
	hash3 := HashKey("different-key")
	assert.NotEqual(t, hash1, hash3)
}
