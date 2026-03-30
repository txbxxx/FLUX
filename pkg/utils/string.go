package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// StringIsEmpty 判断字符串在 trim 后是否为空。
func StringIsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// StringNotEmpty 是 StringIsEmpty 的反义包装。
func StringNotEmpty(s string) bool {
	return !StringIsEmpty(s)
}

// StringTrimSpace 去除首尾空白字符。
func StringTrimSpace(s string) string {
	return strings.TrimSpace(s)
}

// StringContains 以不区分大小写的方式判断是否包含子串。
func StringContains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// StringSplit 分割字符串并过滤空白项。
func StringSplit(s, sep string) []string {
	if StringIsEmpty(s) {
		return []string{}
	}
	result := strings.Split(s, sep)
	// 过滤空字符串和纯空白项。
	var filtered []string
	for _, item := range result {
		if StringNotEmpty(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// StringJoin 在空切片时返回空字符串。
func StringJoin(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, sep)
}

// SanitizeFilename 清理文件名中的常见非法字符。
func SanitizeFilename(filename string) string {
	// Windows 不允许的字符: \ / : * ? " < > |
	reg := regexp.MustCompile(`[\\/:*?"<>|]`)
	sanitized := reg.ReplaceAllString(filename, "_")
	// 去除首尾空格和点
	sanitized = strings.TrimSpace(sanitized)
	sanitized = strings.Trim(sanitized, ".")
	return sanitized
}

// IsWindows 返回当前进程是否运行在 Windows。
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// NormalizePath 按当前平台统一路径分隔符风格。
func NormalizePath(path string) string {
	if IsWindows() {
		// 统一使用反斜杠
		return filepath.FromSlash(path)
	}
	// Unix 系统使用正斜杠
	return filepath.ToSlash(path)
}

// ExpandUserHome 展开用户目录路径。
// 当前支持 `~` 和 `%USERPROFILE%` 两种写法。
func ExpandUserHome(path string) string {
	if strings.HasPrefix(path, "~") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(homeDir, path[1:])
		}
	}

	// 处理 Windows 环境变量 %USERPROFILE%
	if strings.Contains(path, "%USERPROFILE%") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			return strings.ReplaceAll(path, "%USERPROFILE%", homeDir)
		}
	}

	return path
}
