package models

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// NewSnapshot 创建一个带默认时间和基础元数据的快照对象。
func NewSnapshot(message string, projectName string) *Snapshot {
	return &Snapshot{
		ID:          0, // GORM 自动生成自增 ID
		Name:        generateSnapshotName(message),
		Description: message,
		Message:     message,
		CreatedAt:   time.Now(),
		Project:     projectName,
		Metadata: SnapshotMetadata{
			ProjectPath: projectName, // 使用项目名称标识来源
		},
		Files: []SnapshotFile{},
		Tags:  []string{},
	}
}


// generateSnapshotName 从消息生成展示用名称。
func generateSnapshotName(message string) string {
	// 这里仅做简单截断，避免名称过长影响列表展示。
	maxLen := 50
	if len(message) <= maxLen {
		return message
	}
	return message[:maxLen] + "..."
}

// ValidateSnapshot 校验快照的最基础输入合法性。
func ValidateSnapshot(snapshot *Snapshot) error {
	if snapshot == nil {
		return fmt.Errorf("快照不能为空")
	}

	if snapshot.Message == "" {
		return fmt.Errorf("提交消息不能为空")
	}

	if snapshot.Project == "" {
		return fmt.Errorf("必须指定项目名称")
	}

	return nil
}

// ValidateRemoteConfig 验证远端配置的最基础合法性。
func ValidateRemoteConfig(config *RemoteConfig) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	if config.URL == "" {
		return fmt.Errorf("仓库 URL 不能为空")
	}

	if !isValidGitURL(config.URL) {
		return fmt.Errorf("无效的 Git URL: %s", config.URL)
	}

	if config.Branch == "" {
		config.Branch = "main"
	}

	return nil
}

// isValidGitURL 用简单模式匹配判断 URL 是否像一个 Git 地址。
func isValidGitURL(url string) bool {
	patterns := []string{
		`^https?://`,
		`^ssh://`,
		`^git@`,
		`^git://`,
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, url); matched {
			return true
		}
	}

	return false
}

// ValidateSyncConfig 校验同步配置中的关键约束。
func ValidateSyncConfig(config *SyncConfig) error {
	if config == nil {
		return fmt.Errorf("同步配置不能为空")
	}

	if config.AutoSync && config.SyncInterval == 0 {
		return fmt.Errorf("自动同步时必须指定同步间隔")
	}

	if config.SyncInterval < time.Minute {
		return fmt.Errorf("同步间隔不能小于 1 分钟")
	}

	return nil
}

// CalculateChecksum 计算快照校验和。
// 当前实现是轻量占位版本，只基于文件数和总大小。
func CalculateChecksum(files []SnapshotFile) string {
	// 简化实现：使用文件数量和总大小作为校验和
	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}
	return fmt.Sprintf("%d-%d", len(files), totalSize)
}

// FilterFilesByCategory 按类别过滤文件。
func FilterFilesByCategory(files []SnapshotFile, categories []FileCategory) []SnapshotFile {
	if len(categories) == 0 {
		return files
	}

	result := []SnapshotFile{}
	categoryMap := make(map[FileCategory]bool)
	for _, cat := range categories {
		categoryMap[cat] = true
	}

	for _, file := range files {
		if categoryMap[file.Category] {
			result = append(result, file)
		}
	}

	return result
}

// FilterFilesByTool 按工具过滤文件。
func FilterFilesByTool(files []SnapshotFile, toolTypes []string) []SnapshotFile {
	if len(toolTypes) == 0 {
		return files
	}

	result := []SnapshotFile{}
	toolMap := make(map[string]bool)
	for _, tool := range toolTypes {
		toolMap[tool] = true
	}

	for _, file := range files {
		if toolMap[file.ToolType] {
			result = append(result, file)
		}
	}

	return result
}

// GetFileExtension 返回不带点号的扩展名。
func GetFileExtension(path string) string {
	ext := filepath.Ext(path)
	return strings.TrimPrefix(ext, ".")
}

// IsConfigFile 用扩展名和少量常见文件名判断是否像配置文件。
func IsConfigFile(path string) bool {
	configExts := []string{
		".toml", ".yaml", ".yml", ".json", ".xml", ".ini", ".conf",
	}
	ext := strings.ToLower(filepath.Ext(path))

	for _, configExt := range configExts {
		if ext == configExt {
			return true
		}
	}

	base := strings.ToLower(filepath.Base(path))
	return base == "config" || base == "settings"
}

// IsBinaryFile 用扩展名做轻量判断，适合作为启发式过滤。
func IsBinaryFile(path string) bool {
	binaryExts := []string{
		".exe", ".dll", ".so", ".dylib", ".bin", ".img", ".iso",
		".zip", ".tar", ".gz", ".7z", ".rar",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
	}
	ext := strings.ToLower(filepath.Ext(path))

	for _, binaryExt := range binaryExts {
		if ext == binaryExt {
			return true
		}
	}

	return false
}

// FormatDuration 把时长格式化为更适合展示的短文本。
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// FormatBytes 用二进制单位格式化字节数。
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// CloneSnapshot 对切片字段做深拷贝，避免调用方互相污染。
func CloneSnapshot(snapshot *Snapshot) *Snapshot {
	if snapshot == nil {
		return nil
	}

	clone := *snapshot
	clone.Files = make([]SnapshotFile, len(snapshot.Files))
	copy(clone.Files, snapshot.Files)
	clone.Tags = make([]string, len(snapshot.Tags))
	copy(clone.Tags, snapshot.Tags)

	// Project 是字符串，不需要深拷贝
	return &clone
}

// NewTaskProgress 根据当前值和总量创建进度对象。
func NewTaskProgress(current, total int, message string) TaskProgress {
	percentage := 0
	if total > 0 {
		percentage = int(float64(current) / float64(total) * 100)
	}

	return TaskProgress{
		Percentage: percentage,
		Current:    current,
		Total:      total,
		Message:    message,
		Steps:      []string{},
	}
}

// AddStep 追加一条进度步骤说明。
func (p *TaskProgress) AddStep(step string) {
	p.Steps = append(p.Steps, step)
}

// UpdateProgress 更新进度计数和展示消息。
func (p *TaskProgress) UpdateProgress(current, total int, message string) {
	p.Current = current
	p.Total = total
	if total > 0 {
		p.Percentage = int(float64(current) / float64(total) * 100)
	}
	if message != "" {
		p.Message = message
	}
}

// IsCompleted 判断任务是否到达完成状态。
func (p *TaskProgress) IsCompleted() bool {
	return p.Current >= p.Total && p.Total > 0
}
