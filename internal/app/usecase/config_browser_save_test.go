package usecase

import (
	"context"
	"strings"
	"testing"

	"flux/internal/service/tool"
)

// TestSaveConfigSanitizesContentBeforeWriting 验证 SaveConfig 在写入前清理了空字节和控制字符。
func TestSaveConfigSanitizesContentBeforeWriting(t *testing.T) {
	accessor := &stubConfigAccessor{
		target: &tool.ConfigTarget{
			ToolType:     tool.ToolTypeClaude,
			RelativePath: "settings.json",
			AbsolutePath: "/home/test/.claude/settings.json",
			IsDir:        false,
		},
	}

	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}, accessor)

	// 模拟编辑器引入空字节和控制字符的场景
	dirtyContent := "{\n  \"env\": {\n    \"TOKEN\": \"abc\x00def\",\n    \"URL\": \"https://api\x01.test.com\"\n  }\n}"

	err := workflow.SaveConfig(context.Background(), SaveConfigInput{
		Tool:    "claude",
		Path:    "settings.json",
		Content: dirtyContent,
	})
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// 验证写入的内容不含空字节和控制字符
	if strings.Contains(accessor.writeContent, "\x00") {
		t.Error("写入内容不应包含空字节")
	}
	if strings.Contains(accessor.writeContent, "\x01") {
		t.Error("写入内容不应包含控制字符")
	}
	// 验证有效内容保留
	if !strings.Contains(accessor.writeContent, "TOKEN") || !strings.Contains(accessor.writeContent, "abcdef") {
		t.Errorf("写入内容丢失了有效数据: %q", accessor.writeContent)
	}
}

// TestSaveConfigPreservesValidContent 验证 SaveConfig 不会损坏正常内容。
func TestSaveConfigPreservesValidContent(t *testing.T) {
	accessor := &stubConfigAccessor{
		target: &tool.ConfigTarget{
			ToolType:     tool.ToolTypeClaude,
			RelativePath: "settings.json",
			AbsolutePath: "/home/test/.claude/settings.json",
			IsDir:        false,
		},
	}

	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}, accessor)

	validContent := "{\n  \"env\": {\n    \"TOKEN\": \"valid-token\",\n    \"URL\": \"https://api.test.com\"\n  },\n  \"language\": \"chinese\"\n}"

	err := workflow.SaveConfig(context.Background(), SaveConfigInput{
		Tool:    "claude",
		Path:    "settings.json",
		Content: validContent,
	})
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	if accessor.writeContent != validContent {
		t.Errorf("正常内容不应被修改:\ngot:  %q\nwant: %q", accessor.writeContent, validContent)
	}
}
