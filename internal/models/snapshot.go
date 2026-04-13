package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"flux/pkg/database"

	"gorm.io/gorm"
)

// Snapshot 配置快照
type Snapshot struct {
	ID          uint             `json:"id" db:"id" gorm:"column:id;primaryKey;autoIncrement"` // 快照唯一 ID
	Name        string           `json:"name" db:"name"`                                       // 快照名称
	Description string           `json:"description" db:"description"`                         // 快照描述
	Message     string           `json:"message" db:"message"`                                 // 提交消息
	CreatedAt   time.Time        `json:"created_at" db:"created_at"`                           // 创建时间
	Project     string           `json:"project" db:"tools"`                                   // 关联的项目名称（db tag 保留 "tools" 复用现有列，不改变表结构）
	Metadata    SnapshotMetadata `json:"metadata" db:"metadata"`                               // 快照元数据
	Files       []SnapshotFile   `json:"files" db:"files"`                                     // 包含的文件列表
	CommitHash  string           `json:"commit_hash,omitempty" db:"commit_hash"`               // Git 提交哈希（如果已推送）
	Tags        []string         `json:"tags,omitempty" db:"tags"`                             // 标签
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

// Create writes the snapshot record and its associated files in a single transaction.
// 分批插入文件以避免 SQLite SQL 变量数量限制（每批 500 个文件）。
func (dao *SnapshotDAO) Create(snapshot *Snapshot) error {
	row := snapshotToRow(snapshot)
	fileRows := snapshotFilesToRows(snapshot.ID, snapshot.Files)

	return dao.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		snapshot.ID = row.ID
		for i := range fileRows {
			fileRows[i].SnapshotID = row.ID
		}
		if len(fileRows) > 0 {
			// 分批插入以避免 SQLite SQL 变量数量限制
			const batchSize = 500
			for i := 0; i < len(fileRows); i += batchSize {
				end := i + batchSize
				if end > len(fileRows) {
					end = len(fileRows)
				}
				batch := fileRows[i:end]
				if err := tx.Omit("id").Create(&batch).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// GetByID retrieves a snapshot by ID with its files loaded via explicit query.
func (dao *SnapshotDAO) GetByID(id uint) (*Snapshot, error) {
	var row snapshotRow
	err := dao.db.GetConn().First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("快照不存在")
	}
	if err != nil {
		return nil, err
	}

	var fileRows []snapshotFileRow
	if err := dao.db.GetConn().Where("snapshot_id = ?", id).Find(&fileRows).Error; err != nil {
		return nil, err
	}

	snapshot, err := snapshotRowToModel(row, fileRows)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

// List returns snapshots ordered by creation time descending, with files loaded in batch.
func (dao *SnapshotDAO) List(limit, offset int) ([]*Snapshot, error) {
	query := dao.db.GetConn().Model(&snapshotRow{}).Order("created_at DESC")
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

	if len(rows) == 0 {
		return []*Snapshot{}, nil
	}

	// Batch-load files for all snapshots.
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	var allFileRows []snapshotFileRow
	if err := dao.db.GetConn().Where("snapshot_id IN ?", ids).Find(&allFileRows).Error; err != nil {
		return nil, err
	}

	filesBySnapshot := make(map[uint][]snapshotFileRow)
	for _, f := range allFileRows {
		filesBySnapshot[f.SnapshotID] = append(filesBySnapshot[f.SnapshotID], f)
	}

	snapshots := make([]*Snapshot, 0, len(rows))
	for _, row := range rows {
		snapshot, err := snapshotRowToModel(row, filesBySnapshot[row.ID])
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

// Update updates mutable fields of the snapshot record.
func (dao *SnapshotDAO) Update(snapshot *Snapshot) error {
	row := snapshotToRow(snapshot)

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

// UpdateWithFiles updates the snapshot record and replaces all associated files in a transaction.
// 为什么：编辑快照中的文件需要同时更新 snapshot_files 记录。
func (dao *SnapshotDAO) UpdateWithFiles(snapshot *Snapshot) error {
	row := snapshotToRow(snapshot)
	fileRows := snapshotFilesToRows(snapshot.ID, snapshot.Files)

	return dao.db.Transaction(func(tx *gorm.DB) error {
		// 先删旧文件再插入新文件。
		if err := tx.Where("snapshot_id = ?", snapshot.ID).Delete(&snapshotFileRow{}).Error; err != nil {
			return err
		}
		if len(fileRows) > 0 {
			if err := tx.Omit("id").Create(&fileRows).Error; err != nil {
				return err
			}
		}
		// 更新快照记录字段。
		return tx.Model(&snapshotRow{}).
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
	})
}

// Delete deletes the snapshot and its associated files in a single transaction.
// 为什么：没有外键约束，必须由应用层级联清理 snapshot_files，否则会留下孤立记录。
func (dao *SnapshotDAO) Delete(id uint) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		// 先删文件再删快照记录，避免残留。
		if err := tx.Where("snapshot_id = ?", id).Delete(&snapshotFileRow{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&snapshotRow{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
}

// Count 返回当前快照总数。
func (dao *SnapshotDAO) Count() (int, error) {
	var count int64
	err := dao.db.GetConn().Model(&snapshotRow{}).Count(&count).Error
	return int(count), err
}

// ListByToolType returns snapshots whose JSON tools array contains the specified tool.
// Uses SQL LIKE to filter at the database level instead of loading all snapshots into memory.
func (dao *SnapshotDAO) ListByToolType(toolType string, limit, offset int) ([]*Snapshot, error) {
	query := dao.db.GetConn().Model(&snapshotRow{}).
		Where("tools LIKE ?", fmt.Sprintf(`%%"%s"%%`, toolType)).
		Order("created_at DESC")
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

	return dao.loadSnapshotsWithFiles(rows)
}

// ListByTag returns snapshots whose JSON tags array contains the specified tag.
// Uses SQL LIKE to filter at the database level instead of loading all snapshots into memory.
func (dao *SnapshotDAO) ListByTag(tag string, limit, offset int) ([]*Snapshot, error) {
	query := dao.db.GetConn().Model(&snapshotRow{}).
		Where("tags LIKE ?", fmt.Sprintf(`%%"%s"%%`, tag)).
		Order("created_at DESC")
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

	return dao.loadSnapshotsWithFiles(rows)
}

// loadSnapshotsWithFiles batch-loads file rows for the given snapshot rows and converts to domain models.
func (dao *SnapshotDAO) loadSnapshotsWithFiles(rows []snapshotRow) ([]*Snapshot, error) {
	if len(rows) == 0 {
		return []*Snapshot{}, nil
	}

	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}

	var allFileRows []snapshotFileRow
	if err := dao.db.GetConn().Where("snapshot_id IN ?", ids).Find(&allFileRows).Error; err != nil {
		return nil, err
	}

	filesBySnapshot := make(map[uint][]snapshotFileRow)
	for _, f := range allFileRows {
		filesBySnapshot[f.SnapshotID] = append(filesBySnapshot[f.SnapshotID], f)
	}

	snapshots := make([]*Snapshot, 0, len(rows))
	for _, row := range rows {
		snapshot, err := snapshotRowToModel(row, filesBySnapshot[row.ID])
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

// snapshotRow aliases the canonical GORM record defined in pkg/database,
// eliminating the previous duplication between database/db.go and this file.
type snapshotRow = database.SnapshotRecord

// snapshotFileRow aliases the canonical GORM record defined in pkg/database.
type snapshotFileRow = database.SnapshotFileRecord

// snapshotToRow converts a Snapshot domain model to a snapshotRow for DB persistence.
func snapshotToRow(snapshot *Snapshot) snapshotRow {
	projectJSON, _ := json.Marshal(snapshot.Project)
	tagsJSON, _ := json.Marshal(snapshot.Tags)
	metadataJSON, _ := json.Marshal(snapshot.Metadata)

	return snapshotRow{
		ID:          snapshot.ID,
		Name:        snapshot.Name,
		Description: snapshot.Description,
		Message:     snapshot.Message,
		CreatedAt:   snapshot.CreatedAt,
		Tools:       string(projectJSON),
		Metadata:    string(metadataJSON),
		CommitHash:  snapshot.CommitHash,
		Tags:        string(tagsJSON),
		FileCount:   len(snapshot.Files),
		TotalSize:   calculateTotalSize(snapshot.Files),
	}
}

// snapshotFilesToRows converts SnapshotFile domain models to snapshotFileRow slices for DB persistence.
func snapshotFilesToRows(snapshotID uint, files []SnapshotFile) []snapshotFileRow {
	rows := make([]snapshotFileRow, 0, len(files))
	for _, file := range files {
		rows = append(rows, snapshotFileRow{
			SnapshotID:   snapshotID,
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
	return rows
}

// snapshotRowToModel converts a snapshotRow and its file rows into a Snapshot domain model.
//
// 向后兼容性处理：如果读取到旧格式的 tools JSON 数组，且 project 为空，则取第一个工具作为 project 名称。
func snapshotRowToModel(row snapshotRow, fileRows []snapshotFileRow) (*Snapshot, error) {
	snapshot := &Snapshot{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Message:     row.Message,
		CreatedAt:   row.CreatedAt,
		CommitHash:  row.CommitHash,
		Files:       make([]SnapshotFile, 0, len(fileRows)),
	}

	if row.Tools != "" {
		// 先尝试解析为新项目格式：单个 project 字符串
		var project string
		if err := json.Unmarshal([]byte(row.Tools), &project); err == nil {
			snapshot.Project = project
		} else {
			// 如果失败，尝试解析为旧格式：tools 数组
			var oldTools []string
			if err2 := json.Unmarshal([]byte(row.Tools), &oldTools); err2 == nil && len(oldTools) > 0 {
				// 向后兼容：取第一个工具作为项目名
				snapshot.Project = oldTools[0]
			} else {
				return nil, fmt.Errorf("反序列化项目信息失败 (tried both new and old formats): %w, %w", err, err2)
			}
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

	for _, file := range fileRows {
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
