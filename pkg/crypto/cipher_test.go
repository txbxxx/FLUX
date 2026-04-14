package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt(t *testing.T) {
	// 生成测试密钥
	key, err := GenerateKey()
	require.NoError(t, err)

	decodedKey, err := DecodeKey(key)
	require.NoError(t, err)
	require.Len(t, decodedKey, 32)

	tests := []struct {
		name string
		data string
	}{
		{"简单文本", "hello world"},
		{"中文", "你好世界"},
		{"空字符串", ""},
		{"长文本", string(make([]byte, 1000))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 加密
			encrypted, err := EncryptString(tt.data, decodedKey)
			require.NoError(t, err)
			assert.NotEmpty(t, encrypted)
			assert.NotEqual(t, tt.data, encrypted)

			// 解密
			decrypted, err := DecryptString(encrypted, decodedKey)
			require.NoError(t, err)
			assert.Equal(t, tt.data, decrypted)
		})
	}
}

func TestEncryptDecrypt_Bytes(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)

	decodedKey, err := DecodeKey(key)
	require.NoError(t, err)

	data := []byte{1, 2, 3, 4, 5}

	// 加密
	encrypted, err := Encrypt(data, decodedKey)
	require.NoError(t, err)

	// 解密
	decrypted, err := Decrypt(encrypted, decodedKey)
	require.NoError(t, err)
	assert.Equal(t, data, decrypted)
}

func TestGenerateKey(t *testing.T) {
	key1, err := GenerateKey()
	require.NoError(t, err)
	assert.NotEmpty(t, key1)

	key2, err := GenerateKey()
	require.NoError(t, err)
	assert.NotEmpty(t, key2)

	// 每次生成的密钥应该不同
	assert.NotEqual(t, key1, key2)
}

func TestValidateKey(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)

	// 有效密钥
	err = ValidateKey(key)
	assert.NoError(t, err)

	// 无效密钥
	err = ValidateKey("invalid-key")
	assert.Error(t, err)

	err = ValidateKey("dG9vXNob3J0") // base64 解码后不足 32 字节
	assert.Error(t, err)
}

func TestDecodeKey(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)

	decoded, err := DecodeKey(key)
	require.NoError(t, err)
	assert.Len(t, decoded, 32)

	// 无效密钥
	_, err = DecodeKey("invalid")
	assert.Error(t, err)
}

func TestHashKey(t *testing.T) {
	key := "test-key-12345"

	hash1 := HashKey(key)
	hash2 := HashKey(key)

	// 相同输入产生相同哈希
	assert.Equal(t, hash1, hash2)

	// 不同输入产生不同哈希
	hash3 := HashKey("different-key")
	assert.NotEqual(t, hash1, hash3)
}

func TestSHA256Hash(t *testing.T) {
	data := []byte("hello world")

	hash := SHA256Hash(data)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64) // SHA256 十六进制是 64 字符

	// 相同输入产生相同哈希
	hash2 := SHA256Hash(data)
	assert.Equal(t, hash, hash2)

	// 不同输入产生不同哈希
	hash3 := SHA256Hash([]byte("different"))
	assert.NotEqual(t, hash, hash3)
}

func TestEncryptDecrypt_InvalidKeyLength(t *testing.T) {
	shortKey := make([]byte, 16) // 不足 32 字节
	data := []byte("test data")

	_, err := Encrypt(data, shortKey)
	assert.Error(t, err)

	_, err = Decrypt("encrypted", shortKey)
	assert.Error(t, err)
}

func TestDecrypt_InvalidCiphertext(t *testing.T) {
	key, err := GenerateKey()
	require.NoError(t, err)

	decodedKey, err := DecodeKey(key)
	require.NoError(t, err)

	// 无效的 Base64
	_, err = Decrypt("not-base64!", decodedKey)
	assert.Error(t, err)

	// 有效的 Base64 但解密失败
	_, err = Decrypt("dGVzdA==", decodedKey) // "test" in base64
	assert.Error(t, err)
}
