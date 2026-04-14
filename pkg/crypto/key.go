package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

// GenerateKey 生成 32 字节随机密钥并返回 Base64 编码
func GenerateKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("生成密钥失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// ValidateKey 验证密钥格式（32 字节，Base64 编码）
func ValidateKey(key string) error {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("密钥格式无效: %w", err)
	}
	if len(decoded) != 32 {
		return fmt.Errorf("密钥长度无效: 期望 32 字节，实际 %d 字节", len(decoded))
	}
	return nil
}

// HashKey 计算 SHA256 哈希并返回 Base64 编码
func HashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return base64.StdEncoding.EncodeToString(hash[:])
}

// DecodeKey 将 Base64 编码的密钥解码为 32 字节
func DecodeKey(key string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("密钥解码失败: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("密钥长度无效: 期望 32 字节，实际 %d 字节", len(decoded))
	}
	return decoded, nil
}

// SHA256Hash 计算内容的 SHA256 哈希并返回十六进制字符串
func SHA256Hash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
