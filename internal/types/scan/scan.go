package scan

import (
	"time"

	"flux/internal/models"
)

// ToolStatistics holds statistics for a single tool.
type ToolStatistics struct {
	ToolType      string    `json:"tool_type"`      // 工具类型
	IsInstalled   bool      `json:"is_installed"`   // 是否已安装
	GlobalFiles   int       `json:"global_files"`   // 全局文件数
	ProjectFiles  int       `json:"project_files"`  // 项目文件数
	LastSynced    time.Time `json:"last_synced"`    // 最后同步时间
	SyncCount     int       `json:"sync_count"`     // 同步次数
	ConflictCount int       `json:"conflict_count"` // 冲突次数
}

// ProjectStatistics holds statistics for a registered project.
type ProjectStatistics struct {
	Path       string    `json:"path"`        // 项目路径
	HasCodex   bool      `json:"has_codex"`   // 是否有 Codex 配置
	HasClaude  bool      `json:"has_claude"`  // 是否有 Claude 配置
	FileCount  int       `json:"file_count"`  // 文件数量
	LastBackup time.Time `json:"last_backup"` // 最后备份时间
	SyncCount  int       `json:"sync_count"`  // 同步次数
}

// ToolProfile describes the sync profile for a specific tool.
type ToolProfile struct {
	ToolType     string               `json:"tool_type"`     // 工具类型（codex/claude）
	Name         string               `json:"name"`          // 配置名称
	Description  string               `json:"description"`   // 配置描述
	GlobalPath   string               `json:"global_path"`   // 全局配置路径
	ProjectPaths []string             `json:"project_paths"` // 项目路径列表
	SyncEnabled  bool                 `json:"sync_enabled"`  // 是否启用同步
	Categories   []models.FileCategory `json:"categories"`    // 同步的文件类别
	Excludes     []string             `json:"excludes"`      // 排除的文件
	CreatedAt    time.Time            `json:"created_at"`    // 创建时间
	UpdatedAt    time.Time            `json:"updated_at"`    // 更新时间
}
