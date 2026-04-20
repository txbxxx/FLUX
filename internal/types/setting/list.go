package setting

// AISettingListItem 表示配置列表中的单个条目。
type AISettingListItem struct {
	ID          uint   `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	BaseURL     string `json:"base_url" yaml:"base_url"`
	OpusModel   string `json:"opus_model" yaml:"opus_model"`
	SonnetModel string `json:"sonnet_model" yaml:"sonnet_model"`
	IsCurrent   bool   `json:"is_current" yaml:"is_current"` // 是否为当前生效配置
}

// AISettingListResult 表示配置列表的查询结果。
type AISettingListResult struct {
	Items   []AISettingListItem `json:"items" yaml:"items"`
	Total   int                 `json:"total" yaml:"total"`
	Current string              `json:"current" yaml:"current"` // 当前配置名称
}
