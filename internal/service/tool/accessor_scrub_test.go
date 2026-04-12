package tool

import (
	"os"
	"path/filepath"
	"testing"
)

// TestScrubNullBytesRemovesAllNullBytes 验证 scrubNullBytes 正确移除所有空字节。
func TestScrubNullBytesRemovesAllNullBytes(t *testing.T) {
	input := []byte("hello\x00world\x00")
	result := scrubNullBytes(input)
	if string(result) != "helloworld" {
		t.Fatalf("expected 'helloworld', got %q", string(result))
	}
}

// TestScrubNullBytesPreservesNormalContent 验证不含空字节的正常内容不受影响。
func TestScrubNullBytesPreservesNormalContent(t *testing.T) {
	input := []byte("{\"key\": \"value\"}\n")
	result := scrubNullBytes(input)
	if string(result) != string(input) {
		t.Fatalf("expected content unchanged, got %q", string(result))
	}
}

// TestScrubNullBytesHandlesEmptyInput 验证空输入返回空结果。
func TestScrubNullBytesHandlesEmptyInput(t *testing.T) {
	result := scrubNullBytes([]byte{})
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %q", string(result))
	}
}

// TestScrubNullBytesHandlesAllNullBytes 验证全空字节输入返回空结果。
func TestScrubNullBytesHandlesAllNullBytes(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00}
	result := scrubNullBytes(input)
	if len(result) != 0 {
		t.Fatalf("expected empty result for all-null input, got %q", string(result))
	}
}

// TestScrubNullBytesPreservesControlChars 验证空字节外的控制字符被保留。
func TestScrubNullBytesPreservesControlChars(t *testing.T) {
	input := []byte("line1\nline2\ttab\r\n")
	result := scrubNullBytes(input)
	if string(result) != string(input) {
		t.Fatalf("expected control chars preserved, got %q", string(result))
	}
}

// TestReadFileRecoversFromSparseNullBytes 验证 ReadFile 能自动恢复含零星空字节的配置文件。
// 为什么：实际场景中 settings.json 被损坏后嵌入了 20+ 个空字节，
// 用户执行 get -e 时应能正常读取并编辑，而不是直接报错"二进制文件"。
func TestReadFileRecoversFromSparseNullBytes(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	root := filepath.Join(homeDir, ".codex")
	path := filepath.Join(root, "skills", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// 模拟被损坏的 settings.json：JSON 内容中散布空字节
	content := []byte("{\"name\":\"test\x00\",\"value\":\"ok\"}\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	accessor := NewConfigAccessor()

	target, err := accessor.Resolve(ToolTypeCodex, filepath.Join("skills", "config.json"))
	if err != nil {
		t.Fatalf("expected file to resolve: %v", err)
	}

	result, err := accessor.ReadFile(target)
	if err != nil {
		t.Fatalf("expected ReadFile to recover from sparse null bytes: %v", err)
	}

	expected := "{\"name\":\"test\",\"value\":\"ok\"}\n"
	if result != expected {
		t.Fatalf("expected cleaned content %q, got %q", expected, result)
	}
}

// TestReadFileRecoversFromRealWorldCorruption 模拟实际 bug 场景：
// settings.json 中间嵌入约 23 个连续空字节。
func TestReadFileRecoversFromRealWorldCorruption(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	root := filepath.Join(homeDir, ".codex")
	path := filepath.Join(root, "skills", "settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	// 模拟真实损坏：JSON 前半段 + 23 个空字节 + JSON 后半段
	prefix := []byte("{\"api\":\"https://api.example.com\",\"opus_model\":\"opus-4\",")
	nulls := make([]byte, 23) // 23 个空字节
	suffix := []byte("\"sonnet_model\":\"sonnet-4\"}\n")

	content := make([]byte, 0, len(prefix)+len(nulls)+len(suffix))
	content = append(content, prefix...)
	content = append(content, nulls...)
	content = append(content, suffix...)

	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	accessor := NewConfigAccessor()

	target, err := accessor.Resolve(ToolTypeCodex, filepath.Join("skills", "settings.json"))
	if err != nil {
		t.Fatalf("expected file to resolve: %v", err)
	}

	result, err := accessor.ReadFile(target)
	if err != nil {
		t.Fatalf("expected ReadFile to recover from real-world corruption: %v", err)
	}

	expected := "{\"api\":\"https://api.example.com\",\"opus_model\":\"opus-4\",\"sonnet_model\":\"sonnet-4\"}\n"
	if result != expected {
		t.Fatalf("expected cleaned content %q, got %q", expected, result)
	}
}
