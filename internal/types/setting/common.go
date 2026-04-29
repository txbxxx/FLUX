package setting

// AISwitchResult 表示配置切换的结果。
type AISwitchResult struct {
	PreviousName string            `json:"previous_name" yaml:"previous_name"`
	NewName      string            `json:"new_name" yaml:"new_name"`
	BackupPath   string            `json:"backup_path" yaml:"backup_path"` // 备份文件路径
	EnvVars      map[string]string `json:"env_vars" yaml:"env_vars"`       // 写入的环境变量
}
