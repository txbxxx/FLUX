package setting

import "time"

// EditAISettingInput 编辑 AI 配置的输入。
type EditAISettingInput struct {
	// Name 是要编辑的配置名称（必填）
	Name string
	// NewName 是新名称（可选，留空表示不修改）
	NewName string
	// Token 是新 Token（可选，留空表示不修改）
	Token string
	// BaseURL 是新 API 地址（可选，留空表示不修改）
	BaseURL string
	// Models 是新模型列表（可选，留空表示不修改）
	Models []string
	// UseEditor 是否使用编辑器模式
	UseEditor bool
	// Format 编辑器模式文件格式：yaml 或 json（默认 yaml）
	Format string
}

// EditAISettingResult 编辑配置的返回值。
type EditAISettingResult struct {
	ID        string       // 配置 ID
	Name      string       // 配置名称
	UpdatedAt time.Time    // 更新时间
	Changes   []FieldChange // 变更字段列表
	IsCurrent bool         // 是否为当前生效配置
}

// FieldChange 表示单个字段的变更。
type FieldChange struct {
	Field    string // 字段名
	OldValue string // 旧值
	NewValue string // 新值
}

// EditorTemplate 编辑器模式模板内容。
type EditorTemplate struct {
	Name    string   `json:"name" yaml:"name"`
	Token   string   `json:"token" yaml:"token"`
	BaseURL string   `json:"base_url" yaml:"base_url"`
	Models  []string `json:"models" yaml:"models"`
}
