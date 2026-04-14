// Package crypto 提供加密和解密服务
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"flux/internal/models"
	"flux/internal/types/common"
	"flux/pkg/logger"
	"flux/pkg/utils"

	"go.uber.org/zap"
)

// Service 加密服务
type Service struct {
	config *models.EncryptionConfig
	key    []byte
}

// NewService 创建加密服务。
// 未启用加密时会返回一个“直通模式”服务，避免上层反复判空。
func NewService(config *models.EncryptionConfig) (*Service, error) {
	if config == nil || !config.Enabled {
		return &Service{
			config: &models.EncryptionConfig{Enabled: false},
		}, nil
	}

	svc := &Service{
		config: config,
	}

	// 初始化阶段就把密钥取齐，避免运行时首次加解密才暴露问题。
	key, err := svc.getEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("获取加密密钥失败: %w", err)
	}

	// 验证密钥长度
	if len(key) != 32 {
		return nil, fmt.Errorf("加密密钥长度必须为 32 字节（AES-256）")
	}

	svc.key = key

	logger.Info("加密服务初始化成功",
		zap.String("algorithm", config.Algorithm),
	)

	return svc, nil
}

// Encrypt 使用 AES-GCM 加密文本，并返回 Base64 编码结果。
func (s *Service) Encrypt(plaintext string) (*common.EncryptionResult, error) {
	if !s.config.Enabled {
		return &common.EncryptionResult{
			Success:   true,
			Data:      plaintext, // 未启用加密，直接返回原文
			Algorithm: "none",
		}, nil
	}

	if len(s.key) == 0 {
		return &common.EncryptionResult{
			Success: false,
			Error:   "加密密钥未初始化",
		}, fmt.Errorf("加密密钥未初始化")
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return &common.EncryptionResult{
			Success: false,
			Error:   fmt.Sprintf("创建 cipher 失败: %v", err),
		}, err
	}

	// 使用 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return &common.EncryptionResult{
			Success: false,
			Error:   fmt.Sprintf("创建 GCM 失败: %v", err),
		}, err
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return &common.EncryptionResult{
			Success: false,
			Error:   fmt.Sprintf("生成 nonce 失败: %v", err),
		}, err
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Base64 编码
	encrypted := base64.StdEncoding.EncodeToString(ciphertext)

	logger.Debug("数据加密成功",
		zap.Int("original_len", len(plaintext)),
		zap.Int("encrypted_len", len(encrypted)),
	)

	return &common.EncryptionResult{
		Success:   true,
		Data:      encrypted,
		Algorithm: s.config.Algorithm,
	}, nil
}

// Decrypt 解密由 Encrypt 产生的 Base64 密文。
func (s *Service) Decrypt(ciphertext string) (*common.DecryptionResult, error) {
	if !s.config.Enabled {
		return &common.DecryptionResult{
			Success: true,
			Data:    ciphertext, // 未启用加密，直接返回
		}, nil
	}

	if len(s.key) == 0 {
		return &common.DecryptionResult{
			Success: false,
			Error:   "解密密钥未初始化",
		}, nil
	}

	// Base64 解码
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return &common.DecryptionResult{
			Success: false,
			Error:   fmt.Sprintf("Base64 解码失败: %v", err),
		}, nil
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return &common.DecryptionResult{
			Success: false,
			Error:   fmt.Sprintf("创建 cipher 失败: %v", err),
		}, nil
	}

	// 使用 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return &common.DecryptionResult{
			Success: false,
			Error:   fmt.Sprintf("创建 GCM 失败: %v", err),
		}, nil
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return &common.DecryptionResult{
			Success: false,
			Error:   "密文长度不足",
		}, nil
	}

	// 分离 nonce 和密文
	nonce, cipherData := data[:nonceSize], data[nonceSize:]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return &common.DecryptionResult{
			Success: false,
			Error:   fmt.Sprintf("解密失败: %v", err),
		}, nil
	}

	logger.Debug("数据解密成功",
		zap.Int("cipher_len", len(ciphertext)),
		zap.Int("plain_len", len(plaintext)),
	)

	return &common.DecryptionResult{
		Success: true,
		Data:    string(plaintext),
	}, nil
}

// EncryptSensitiveData 把敏感值包装成带类型标识的结构。
func (s *Service) EncryptSensitiveData(dataType common.SensitiveType, value string) (*common.SensitiveData, error) {
	result, err := s.Encrypt(value)
	if err != nil {
		return nil, err
	}

	return &common.SensitiveData{
		Type:    dataType,
		Content: result.Data,
	}, nil
}

// DecryptSensitiveData 解密并返回敏感数据原文。
func (s *Service) DecryptSensitiveData(data *common.SensitiveData) (string, error) {
	result, err := s.Decrypt(data.Content)
	if err != nil {
		return "", err
	}

	return result.Data, nil
}

// EncryptPassword / DecryptPassword / EncryptToken 等是按语义区分的薄包装。
func (s *Service) EncryptPassword(password string) (string, error) {
	result, err := s.Encrypt(password)
	if err != nil {
		return "", err
	}
	return result.Data, nil
}

// DecryptPassword 解密密码
func (s *Service) DecryptPassword(encrypted string) (string, error) {
	result, err := s.Decrypt(encrypted)
	if err != nil {
		return "", err
	}
	return result.Data, nil
}

// EncryptToken 加密 Token
func (s *Service) EncryptToken(token string) (string, error) {
	return s.EncryptPassword(token)
}

// DecryptToken 解密 Token
func (s *Service) DecryptToken(encrypted string) (string, error) {
	return s.DecryptPassword(encrypted)
}

// EncryptSSHKey 加密 SSH 密钥
func (s *Service) EncryptSSHKey(sshKey string) (string, error) {
	return s.EncryptPassword(sshKey)
}

// DecryptSSHKey 解密 SSH 密钥
func (s *Service) DecryptSSHKey(encrypted string) (string, error) {
	return s.DecryptPassword(encrypted)
}

// getEncryptionKey 按“环境变量 -> 密钥文件 -> 自动生成”的顺序取密钥。
func (s *Service) getEncryptionKey() ([]byte, error) {
	// 优先级：环境变量 > 密钥文件 > 生成默认密钥

	// 1. 尝试从环境变量获取
	if s.config.KeyEnvVar != "" {
		envKey := os.Getenv(s.config.KeyEnvVar)
		if envKey != "" {
			// 使用 SHA256 哈希环境变量值作为密钥
			hash := sha256.Sum256([]byte(envKey))
			logger.Debug("使用环境变量密钥",
				zap.String("env_var", s.config.KeyEnvVar),
			)
			return hash[:], nil
		}
	}

	// 2. 尝试从密钥文件获取
	if s.config.KeyPath != "" {
		key, err := s.loadKeyFromFile(s.config.KeyPath)
		if err == nil {
			logger.Debug("使用文件密钥",
				zap.String("key_path", s.config.KeyPath),
			)
			return key, nil
		}
		logger.Warn("加载密钥文件失败", zap.Error(err))
	}

	// 3. 生成并保存默认密钥
	defaultKeyPath := filepath.Join(getDataDir(), ".encryption_key")
	key, err := s.generateAndSaveKey(defaultKeyPath)
	if err == nil {
		logger.Info("生成默认加密密钥",
			zap.String("key_path", defaultKeyPath),
		)
		return key, nil
	}

	return nil, fmt.Errorf("无法获取加密密钥: %w", err)
}

// loadKeyFromFile 从文件加载密钥
func (s *Service) loadKeyFromFile(keyPath string) ([]byte, error) {
	// 扩展路径（支持 ~ 和 %USERPROFILE%）
	keyPath = utils.ExpandUserHome(keyPath)

	// 读取文件
	data, err := utils.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	// 如果是 Base64 编码，先解码
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err == nil {
		return decoded, nil
	}

	return data, nil
}

// generateAndSaveKey 生成并保存密钥
func (s *Service) generateAndSaveKey(keyPath string) ([]byte, error) {
	// 生成 32 字节随机密钥
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(keyPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("创建密钥目录失败: %w", err)
	}

	// 保存密钥（Base64 编码）
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(keyPath, []byte(encoded), 0600); err != nil {
		return nil, fmt.Errorf("保存密钥失败: %w", err)
	}

	return key, nil
}

// getDataDir 获取数据目录
func getDataDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".flux")
}

// IsEnabled 检查加密是否启用
func (s *Service) IsEnabled() bool {
	return s.config != nil && s.config.Enabled
}

// GetAlgorithm 获取加密算法
func (s *Service) GetAlgorithm() string {
	if s.config != nil {
		return s.config.Algorithm
	}
	return "none"
}

// ValidateKey 验证密钥是否有效（委托给 utils.ValidateKey）
func ValidateKey(key string) error {
	return utils.ValidateKey(key)
}

// GenerateKey 生成新的加密密钥（委托给 utils.GenerateKey）
func GenerateKey() (string, error) {
	return utils.GenerateKey()
}

// HashKey 哈希密钥（委托给 utils.HashKey）
func HashKey(key string) string {
	return utils.HashKey(key)
}
