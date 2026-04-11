package usecase

import (
	"testing"
)

// TestSanitizeContent 测试文件内容清理功能
func TestSanitizeContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "普通文本不修改",
			input:    "Hello, World!\n这是中文",
			expected: "Hello, World!\n这是中文",
		},
		{
			name:     "保留制表符",
			input:    "Name\tValue\nAlice\t100",
			expected: "Name\tValue\nAlice\t100",
		},
		{
			name:     "保留换行符",
			input:    "Line1\nLine2\r\nLine3",
			expected: "Line1\nLine2\r\nLine3",
		},
		{
			name:     "移除ASCII控制字符",
			input:    "Hello\x00\x01\x02World",
			expected: "HelloWorld",
		},
		{
			name:     "保留可打印Unicode字符",
			input:    "Hello 世界 🌍",
			expected: "Hello 世界 🌍",
		},
		{
			name:     "JSON文件内容",
			input:    "{\n  \"name\": \"test\",\n  \"value\": 123\n}",
			expected: "{\n  \"name\": \"test\",\n  \"value\": 123\n}",
		},
		{
			name:     "移除字符串控制符（0-31，除了\\t,\\n,\\r）",
			input:    "\x01\x02\x03\x04\x05\x06\x07\x08\x0B\x0C\x0E\x0F\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1A\x1B\x1C\x1D\x1E\x1F",
			expected: "",
		},
		{
			name:     "保留所有可打印ASCII字符",
			input:    " !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~",
			expected: " !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeContent(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeContent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestSanitizeContent_PreservesLineEndings 测试保留各种换行符格式
func TestSanitizeContent_PreservesLineEndings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Unix换行符",
			input:    "Line1\nLine2\n",
			expected: "Line1\nLine2\n",
		},
		{
			name:     "Windows换行符",
			input:    "Line1\r\nLine2\r\n",
			expected: "Line1\r\nLine2\r\n",
		},
		{
			name:     "混合换行符",
			input:    "Line1\nLine2\r\nLine3\n",
			expected: "Line1\nLine2\r\nLine3\n",
		},
		{
			name:     "制表符和换行符",
			input:    "Col1\tCol2\tCol3\nVal1\tVal2\tVal3\n",
			expected: "Col1\tCol2\tCol3\nVal1\tVal2\tVal3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeContent(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeContent(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
