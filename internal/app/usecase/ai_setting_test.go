package usecase

import (
	"encoding/json"
	"testing"
)

// TestParseCurrentSettingInfoToleratesNullBytes 验证 parseCurrentSettingInfo 能处理含空字节的内容。
// 为什么：get -e 编辑器可能在 settings.json 中引入空字节，getCurrentSettingInfo 在调用
// parseCurrentSettingInfo 前会做 sanitizeContent 清理，此测试验证端到端的容错链路。
func TestParseCurrentSettingInfoToleratesNullBytes(t *testing.T) {
	tests := []struct {
		name    string
		content string
		token   string
		baseURL string
	}{
		{
			name:    "正常JSON",
			content: `{"env":{"ANTHROPIC_AUTH_TOKEN":"my-token","ANTHROPIC_BASE_URL":"https://api.test.com"}}`,
			token:   "my-token",
			baseURL: "https://api.test.com",
		},
		{
			name:    "含空字节的JSON",
			content: "{\x00\"env\":{\"ANTHROPIC_AUTH_TOKEN\":\"to\x00ken\",\"ANTHROPIC_BASE_URL\":\"https://api\x00.test.com\"}}",
			token:   "token",
			baseURL: "https://api.test.com",
		},
		{
			name:    "含多种控制字符的JSON",
			content: "{\x01\x02\"env\":{\"ANTHROPIC_AUTH_TOKEN\":\"my\x03-token\x04\",\"ANTHROPIC_BASE_URL\":\"https://api\x05.test.com\"}}",
			token:   "my-token",
			baseURL: "https://api.test.com",
		},
		{
			name:    "含空字节和其他env字段的JSON",
			content: "{\"env\":{\"ANTHROPIC_AUTH_TOKEN\":\"abc\x00def\",\"ANTHROPIC_BASE_URL\":\"https://api.test.com\",\"OTHER\":\"val\"}}",
			token:   "abcdef",
			baseURL: "https://api.test.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 模拟 getCurrentSettingInfo 中的清理链路
			cleaned := []byte(sanitizeContent(tt.content))
			info, err := parseCurrentSettingInfo(cleaned)
			if err != nil {
				t.Fatalf("parseCurrentSettingInfo failed: %v", err)
			}
			if info == nil {
				t.Fatal("expected non-nil result")
			}
			if info.Token != tt.token {
				t.Errorf("Token = %q, want %q", info.Token, tt.token)
			}
			if info.BaseURL != tt.baseURL {
				t.Errorf("BaseURL = %q, want %q", info.BaseURL, tt.baseURL)
			}
		})
	}
}

// TestParseCurrentSettingInfoReturnsNilForMissingToken 验证缺少 token 时返回 nil。
func TestParseCurrentSettingInfoReturnsNilForMissingToken(t *testing.T) {
	content := `{"env":{"ANTHROPIC_BASE_URL":"https://api.test.com"}}`
	info, err := parseCurrentSettingInfo([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil for missing token, got %+v", info)
	}
}

// TestSanitizeThenJsonUnmarshal 验证 sanitizeContent + json.Unmarshal 的完整清理链路。
// 这是 SwitchAISetting 修复的核心逻辑：先清理再解析，防止静默跳过合并。
func TestSanitizeThenJsonUnmarshal(t *testing.T) {
	// 模拟被 get -e 损坏的 settings.json
	corrupted := "{\x00\x01\"env\":{\"" +
		"ANTHROPIC_AUTH_TOKEN\":\"old-tok\x00en\"," +
		"\"ANTHROPIC_BASE_URL\":\"https://old\x02.com\"}," +
		"\"language\":\"chinese\"," +
		"\"enabledPlugins\":{\"p1\":true}" +
		"\x03}"

	// 模拟 SwitchAISetting 第五步的清理链路
	cleaned := sanitizeContent(corrupted)

	var result map[string]any
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		t.Fatalf("清理后的内容应为合法 JSON，但解析失败: %v\ncleaned: %q", err, cleaned)
	}

	// 验证所有字段都被正确解析
	env, ok := result["env"].(map[string]any)
	if !ok {
		t.Fatal("env 字段不存在或类型错误")
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "old-token" {
		t.Errorf("Token = %v, want old-token", env["ANTHROPIC_AUTH_TOKEN"])
	}
	if result["language"] != "chinese" {
		t.Errorf("language = %v, want chinese", result["language"])
	}
}
