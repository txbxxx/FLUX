// Package crypto 加密服务类型定义
package crypto

// ConfigInput 加密配置输入
type ConfigInput struct {
	Enabled   bool   `json:"enabled"`     // 是否启用加密
	Algorithm string `json:"algorithm"`   // 加密算法
	KeyPath   string `json:"key_path"`    // 密钥文件路径
	KeyEnvVar string `json:"key_env_var"` // 密钥环境变量名
}

// ToModel 转换为模型
func (c *ConfigInput) ToModel() *EncryptionConfig {
	return &EncryptionConfig{
		Enabled:   c.Enabled,
		Algorithm: c.Algorithm,
		KeyPath:   c.KeyPath,
		KeyEnvVar: c.KeyEnvVar,
	}
}

// EncryptionConfig 加密配置（内部使用）
type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"`
	KeyPath   string `json:"key_path"`
	KeyEnvVar string `json:"key_env_var"`
}

// KeyInfo 密钥信息
type KeyInfo struct {
	HasKey     bool   `json:"has_key"`      // 是否有密钥
	KeyPath    string `json:"key_path"`     // 密钥路径
	EnvVarName string `json:"env_var_name"` // 环境变量名
}
