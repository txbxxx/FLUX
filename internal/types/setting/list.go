package setting

// AISettingListItem 表示配置列表中的单个条目。
type AISettingListItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	BaseURL     string `json:"base_url"`
	OpusModel   string `json:"opus_model"`
	SonnetModel string `json:"sonnet_model"`
	IsCurrent    bool   `json:"is_current"` // 是否为当前生效配置
}

// AISettingListResult 表示配置列表的查询结果。
type AISettingListResult struct {
	Items      []AISettingListItem `json:"items"`
	Total      int                `json:"total"`
	Current    string             `json:"current"` // 当前配置名称
}
