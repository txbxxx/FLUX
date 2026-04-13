package setting

import "time"

// AISettingDetail 表示保存的 AI 配置详情。
type AISettingDetail struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Token       string    `json:"token"`
	BaseURL     string    `json:"base_url"`
	OpusModel   string    `json:"opus_model"`
	SonnetModel string    `json:"sonnet_model"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AISettingCreateResult 表示创建配置的结果。
type AISettingCreateResult struct {
	ID uint `json:"id"`
}
