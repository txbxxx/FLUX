package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ai-sync-manager/pkg/database"

	"gorm.io/gorm"
)

// Snapshot 配置快照
type Snapshot struct {
	ID          string           `json:"id" db:"id"`                             // 快照唯一 ID
	Name        string           `json:"name" db:"name"`                         // 快照名称
	Description string           `json:"description" db:"description"`           // 快照描述
	Message     string           `json:"message" db:"message"`                   // 提交消息
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`             // 创建时间
	Tools       []string         `json:"tools" db:"tools"`                       // 包含的工具列表 [codex, claude]
	Metadata    SnapshotMetadata `json:"metadata" db:"metadata"`                 // 快照元数据
	Files       []SnapshotFile   `json:"files" db:"files"`                       // 包含的文件列表
	CommitHash  string           `json:"commit_hash,omitempty" db:"commit_hash"` // Git 提交哈希（如果已推送）
	Tags        []string         `json:"tags,omitempty" db:"tags"`               // 标签
}

// SnapshotMetadata 快照元数据
type SnapshotMetadata struct {
	OSVersion   string            `json:"os_version,omitempty"`   // 操作系统版本
	AppVersion  string            `json:"app_version,omitempty"`  // 应用版本
	ProjectPath string            `json:"project_path,omitempty"` // 项目路径（如果是项目快照）
	Scope       SnapshotScope     `json:"scope"`                  // 快照范围
	Extra       map[string]string `json:"extra,omitempty"`        // 额外信息
}

// SnapshotScope 快照范围
type SnapshotScope string

const (
	ScopeGlobal  SnapshotScope = "global"  // 全局配置
	ScopeProject SnapshotScope = "project" // 项目配置
	ScopeBoth    SnapshotScope = "both"    // 全局+项目
)

// SnapshotFile 快照中的文件
type SnapshotFile struct {
	Path         string       `json:"path"`           // 文件相对路径
	OriginalPath string       `json:"original_path"`  // 原始路径
	Size         int64        `json:"size"`           // 文件大小
	Hash         string       `json:"hash,omitempty"` // 文件哈希
	ModifiedAt   time.Time    `json:"modified_at"`    // 修改时间
	Content      []byte       `json:"-" db:"content"` // 文件内容（不序列化到 JSON）
	ToolType     string       `json:"tool_type"`      // 所属工具
	Category     FileCategory `json:"category"`       // 文件类别
	IsBinary     bool         `json:"is_binary"`      // 是否为二进制文件
}

// FileCategory 文件类别
type FileCategory string

const (
	CategoryConfig   FileCategory = "config"   // 配置文件
	CategorySkills   FileCategory = "skills"   // 技能文件
	CategoryCommands FileCategory = "commands" // 命令文件
	CategoryPlugins  FileCategory = "plugins"  // 插件文件
	CategoryMCP      FileCategory = "mcp"      // MCP 配置
	CategoryAgents   FileCategory = "agents"   // Agent 文件
	CategoryRules    FileCategory = "rules"    // 规则文件
	CategoryDocs     FileCategory = "docs"     // 文档文件
	CategoryOutput   FileCategory = "output"   // 输出样式
	CategoryOther    FileCategory = "other"    // 其他文件
)

// SnapshotDAO 快照数据访问对象
type SnapshotDAO struct {
	db *database.DB
}

// NewSnapshotDAO 创建快照 DAO。
func NewSnapshotDAO(db *database.DB) *SnapshotDAO {
	return &SnapshotDAO{db: db}
}

// Create 写入快照主记录和关联文件。
func (dao *SnapshotDAO) Create(snapshot *Snapshot) error {
	row, err := snapshotToRow(snapshot)
	if err != nil {
		return err
	}

	return dao.db.Transaction(func(tx *gorm.DB) error {
		// Omit("Files"): 禁止 GORM 级联写入关联文件，避免与下方手动插入重复。
		if err := tx.Omit("Files").Create(&row).Error; err != nil {
			return err
		}
		if len(row.Files) > 0 {
			if err := tx.Omit("id").Create(&row.Files).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetByID 根据 ID 获取快照，并反序列化 JSON 字段。
func (dao *SnapshotDAO) GetByID(id string) (*Snapshot, error) {
	var row snapshotRow
	err := dao.db.GetConn().
		Preload("Files").
		First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("快照不存在")
	}
	if err != nil {
		return nil, err
	}

	snapshot, err := row.toModel()
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

// List 按创建时间倒序返回快照列表。
func (dao *SnapshotDAO) List(limit, offset int) ([]*Snapshot, error) {
	query := dao.db.GetConn().Model(&snapshotRow{}).Preload("Files").Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	var rows []snapshotRow
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}

	snapshots := make([]*Snapshot, 0, len(rows))
	for _, row := range rows {
		snapshot, err := row.toModel()
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

// Update 更新快照主记录中的可变字段。
func (dao *SnapshotDAO) Update(snapshot *Snapshot) error {
	row, err := snapshotToRow(snapshot)
	if err != nil {
		return err
	}

	return dao.db.GetConn().
		Model(&snapshotRow{}).
		Where("id = ?", snapshot.ID).
		Updates(map[string]interface{}{
			"name":        row.Name,
			"description": row.Description,
			"message":     row.Message,
			"tools":       row.Tools,
			"metadata":    row.Metadata,
			"tags":        row.Tags,
			"commit_hash": row.CommitHash,
			"file_count":  row.FileCount,
			"total_size":  row.TotalSize,
		}).Error
}

// Delete 删除快照记录。
func (dao *SnapshotDAO) Delete(id string) error {
	return dao.db.GetConn().Delete(&snapshotRow{}, "id = ?", id).Error
}

// Count 返回当前快照总数。
func (dao *SnapshotDAO) Count() (int, error) {
	var count int64
	err := dao.db.GetConn().Model(&snapshotRow{}).Count(&count).Error
	return int(count), err
}

type snapshotRow struct {
	ID          string            `gorm:"column:id;primaryKey"`
	Name        string            `gorm:"column:name"`
	Description string            `gorm:"column:description"`
	Message     string            `gorm:"column:message"`
	CreatedAt   time.Time         `gorm:"column:created_at"`
	Tools       string            `gorm:"column:tools"`
	Metadata    string            `gorm:"column:metadata"`
	CommitHash  string            `gorm:"column:commit_hash"`
	Tags        string            `gorm:"column:tags"`
	FileCount   int               `gorm:"column:file_count"`
	TotalSize   int64             `gorm:"column:total_size"`
	Files       []snapshotFileRow `gorm:"foreignKey:SnapshotID;references:ID"`
}

func (snapshotRow) TableName() string {
	return "snapshots"
}

type snapshotFileRow struct {
	ID           uint      `gorm:"column:id;primaryKey;autoIncrement"`
	SnapshotID   string    `gorm:"column:snapshot_id"`
	Path         string    `gorm:"column:path"`
	OriginalPath string    `gorm:"column:original_path"`
	Size         int64     `gorm:"column:size"`
	Hash         string    `gorm:"column:hash"`
	ModifiedAt   time.Time `gorm:"column:modified_at"`
	Content      []byte    `gorm:"column:content"`
	ToolType     string    `gorm:"column:tool_type"`
	Category     string    `gorm:"column:category"`
	IsBinary     bool      `gorm:"column:is_binary"`
}

func (snapshotFileRow) TableName() string {
	return "snapshot_files"
}

func snapshotToRow(snapshot *Snapshot) (snapshotRow, error) {
	toolsJSON, err := json.Marshal(snapshot.Tools)
	if err != nil {
		return snapshotRow{}, fmt.Errorf("序列化工具列表失败: %w", err)
	}
	tagsJSON, err := json.Marshal(snapshot.Tags)
	if err != nil {
		return snapshotRow{}, fmt.Errorf("序列化标签失败: %w", err)
	}
	metadataJSON, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return snapshotRow{}, fmt.Errorf("序列化元数据失败: %w", err)
	}

	files := make([]snapshotFileRow, 0, len(snapshot.Files))
	for _, file := range snapshot.Files {
		files = append(files, snapshotFileRow{
			SnapshotID:   snapshot.ID,
			Path:         file.Path,
			OriginalPath: file.OriginalPath,
			Size:         file.Size,
			Hash:         file.Hash,
			ModifiedAt:   file.ModifiedAt,
			Content:      file.Content,
			ToolType:     file.ToolType,
			Category:     string(file.Category),
			IsBinary:     file.IsBinary,
		})
	}

	return snapshotRow{
		ID:          snapshot.ID,
		Name:        snapshot.Name,
		Description: snapshot.Description,
		Message:     snapshot.Message,
		CreatedAt:   snapshot.CreatedAt,
		Tools:       string(toolsJSON),
		Metadata:    string(metadataJSON),
		CommitHash:  snapshot.CommitHash,
		Tags:        string(tagsJSON),
		FileCount:   len(snapshot.Files),
		TotalSize:   calculateTotalSize(snapshot.Files),
		Files:       files,
	}, nil
}

func (row snapshotRow) toModel() (*Snapshot, error) {
	snapshot := &Snapshot{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Message:     row.Message,
		CreatedAt:   row.CreatedAt,
		CommitHash:  row.CommitHash,
		Files:       make([]SnapshotFile, 0, len(row.Files)),
	}

	if row.Tools != "" {
		if err := json.Unmarshal([]byte(row.Tools), &snapshot.Tools); err != nil {
			return nil, fmt.Errorf("反序列化工具列表失败: %w", err)
		}
	}
	if row.Tags != "" {
		if err := json.Unmarshal([]byte(row.Tags), &snapshot.Tags); err != nil {
			return nil, fmt.Errorf("反序列化标签失败: %w", err)
		}
	}
	if row.Metadata != "" {
		if err := json.Unmarshal([]byte(row.Metadata), &snapshot.Metadata); err != nil {
			return nil, fmt.Errorf("反序列化元数据失败: %w", err)
		}
	}

	for _, file := range row.Files {
		snapshot.Files = append(snapshot.Files, SnapshotFile{
			Path:         file.Path,
			OriginalPath: file.OriginalPath,
			Size:         file.Size,
			Hash:         file.Hash,
			ModifiedAt:   file.ModifiedAt,
			Content:      file.Content,
			ToolType:     file.ToolType,
			Category:     FileCategory(file.Category),
			IsBinary:     file.IsBinary,
		})
	}

	return snapshot, nil
}

// calculateTotalSize 汇总快照文件总大小。
func calculateTotalSize(files []SnapshotFile) int64 {
	var total int64
	for _, file := range files {
		total += file.Size
	}
	return total
}
