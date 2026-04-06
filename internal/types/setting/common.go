package setting

// AISwitchResult 表示配置切换的结果。
type AISwitchResult struct {
	PreviousName string `json:"previous_name"`
	NewName     string `json:"new_name"`
	BackupPath  string `json:"backup_path"` // 备份文件路径
}
