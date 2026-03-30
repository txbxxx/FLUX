package utils

import (
	"os"
	"path/filepath"
	"strings"
)

// FileExists 判断路径是否存在且是普通文件。
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists 判断路径是否存在且是目录。
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// EnsureDir 确保目录存在，不存在时递归创建。
func EnsureDir(dir string) error {
	if DirExists(dir) {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

// GetFileSize 返回文件大小。
func GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// IsFile 是 FileExists 的语义别名，便于调用端表达意图。
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// IsDir 是 DirExists 的语义别名，便于调用端表达意图。
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// GetFileExt 返回包含点号的文件扩展名。
func GetFileExt(path string) string {
	return filepath.Ext(path)
}

// GetFileNameWithoutExt 返回不带扩展名的基础文件名。
func GetFileNameWithoutExt(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(path)
	return strings.TrimSuffix(base, ext)
}

// ReadFile 直接代理 os.ReadFile，保留统一工具入口。
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile 写文件前会先确保父目录存在。
func WriteFile(path string, data []byte) error {
	// 确保目录存在
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// JoinPath 在空输入时返回空字符串，而不是 "."。
func JoinPath(parts ...string) string {
	if len(parts) == 0 {
		return ""
	}
	return filepath.Join(parts...)
}

// AbsPath 返回路径的绝对表示。
func AbsPath(path string) (string, error) {
	return filepath.Abs(path)
}
