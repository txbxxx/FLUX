package tui

import (
	"testing"

	typesSetting "flux/internal/types/setting"

	"flux/internal/app/usecase"
)

// TestMaskTokenForEditor 测试token脱敏显示功能
func TestMaskTokenForEditor(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "空token返回占位符",
			token:    "",
			expected: "<unchanged>",
		},
		{
			name:     "短token返回全星号",
			token:    "abc",
			expected: "****",
		},
		{
			name:     "长token返回脱敏格式",
			token:    "4e5b12345678validQEXh",
			expected: "4e5b****QEXh",
		},
		{
			name:     "正好8字符token",
			token:    "12345678",
			expected: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskTokenForEditor(tt.token)
			if result != tt.expected {
				t.Errorf("maskTokenForEditor(%q) = %q, want %q", tt.token, result, tt.expected)
			}
		})
	}
}

// TestSettingEditorSaveChanges_UnchangedToken 测试保存时token保持不变的场景
func TestSettingEditorSaveChanges_UnchangedToken(t *testing.T) {
	// 创建原始配置（完整token）
	original := &usecase.GetAISettingResult{
		AISettingDetail: typesSetting.AISettingDetail{
			Name:  "test-config",
			Token: "4e5b12345678validQEXh", // 完整token
		},
	}

	saveCalled := false
	var savedToken string

	saveFunc := func(input *usecase.EditAISettingInput) error {
		saveCalled = true
		savedToken = input.Token
		return nil
	}

	// 创建编辑器模型
	model := newSettingEditorModel(original, saveFunc)

	// 模拟用户没有修改token字段（保持脱敏显示值）
	tokenInput := (*model.inputs[1])
	tokenInput.SetValue(maskTokenForEditor(original.Token)) // 设置为脱敏值

	// 调用保存
	_, _ = model.saveChanges()

	// 验证save被调用
	if !saveCalled {
		t.Fatal("save function was not called")
	}

	// 验证保存的是原始完整token，而不是脱敏值
	if savedToken != original.Token {
		t.Errorf("saved token = %q, want original token %q", savedToken, original.Token)
	}
}

// TestSettingEditorSaveChanges_UnchangedPlaceholder 测试保存时token为占位符的场景
func TestSettingEditorSaveChanges_UnchangedPlaceholder(t *testing.T) {
	original := &usecase.GetAISettingResult{
		AISettingDetail: typesSetting.AISettingDetail{
			Name:  "test-config",
			Token: "4e5b12345678validQEXh",
		},
	}

	var savedToken string
	saveFunc := func(input *usecase.EditAISettingInput) error {
		savedToken = input.Token
		return nil
	}

	model := newSettingEditorModel(original, saveFunc)

	// 模拟用户输入<unchanged>占位符
	tokenInput := (*model.inputs[1])
	tokenInput.SetValue("<unchanged>")

	_, _ = model.saveChanges()

	// 验证保存的是原始完整token
	if savedToken != original.Token {
		t.Errorf("saved token = %q, want original token %q", savedToken, original.Token)
	}
}

// TestSettingEditorSaveChanges_NewToken 测试保存时token被修改的场景
func TestSettingEditorSaveChanges_NewToken(t *testing.T) {
	original := &usecase.GetAISettingResult{
		AISettingDetail: typesSetting.AISettingDetail{
			Name:  "test-config",
			Token: "old-token-12345678",
		},
	}

	var savedToken string
	saveFunc := func(input *usecase.EditAISettingInput) error {
		savedToken = input.Token
		return nil
	}

	model := newSettingEditorModel(original, saveFunc)

	// 模拟用户输入新的token
	newToken := "new-token-abcdefgh"
	model.inputs[1].SetValue(newToken)

	// 调用保存
	_, _ = model.saveChanges()

	// 验证保存的是新token
	if savedToken != newToken {
		t.Errorf("saved token = %q, want new token %q", savedToken, newToken)
	}
}

// TestSettingEditorSaveChanges_ErrorHandling 测试保存失败时的错误处理
func TestSettingEditorSaveChanges_ErrorHandling(t *testing.T) {
	original := &usecase.GetAISettingResult{
		AISettingDetail: typesSetting.AISettingDetail{
			Name:  "test-config",
			Token: "test-token",
		},
	}

	expectedError := "保存失败"
	saveFunc := func(input *usecase.EditAISettingInput) error {
		return &testError{msg: expectedError}
	}

	model := newSettingEditorModel(original, saveFunc)

	// 调用保存
	result, _ := model.saveChanges()

	// 验证模型状态更新为错误消息
	newModel := result.(*SettingEditorModel)
	if newModel.statusMsg == "已保存" {
		t.Error("expected error status message, got '已保存'")
	}
}

// testError 用于测试的错误类型
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
