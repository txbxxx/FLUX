package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"ai-sync-manager/pkg/database"

	"gorm.io/gorm"
)

// SyncTask 同步任务
type SyncTask struct {
	ID          string         `json:"id" db:"id"`                               // 任务唯一 ID
	Type        SyncTaskType   `json:"type" db:"type"`                           // 任务类型
	Status      SyncTaskStatus `json:"status" db:"status"`                       // 任务状态
	SnapshotID  string         `json:"snapshot_id,omitempty" db:"snapshot_id"`   // 关联快照 ID
	Direction   SyncDirection  `json:"direction" db:"direction"`                 // 同步方向
	CreatedAt   time.Time      `json:"created_at" db:"created_at"`               // 创建时间
	StartedAt   *time.Time     `json:"started_at,omitempty" db:"started_at"`     // 开始时间
	CompletedAt *time.Time     `json:"completed_at,omitempty" db:"completed_at"` // 完成时间
	Progress    TaskProgress   `json:"progress" db:"progress"`                   // 进度信息
	Error       string         `json:"error,omitempty" db:"error"`               // 错误信息
	Metadata    TaskMetadata   `json:"metadata" db:"metadata"`                   // 任务元数据
}

// SyncTaskType 任务类型
type SyncTaskType string

const (
	TaskTypePush   SyncTaskType = "push"   // 推送配置到远端
	TaskTypePull   SyncTaskType = "pull"   // 从远端拉取配置
	TaskTypeCreate SyncTaskType = "create" // 创建快照
	TaskTypeApply  SyncTaskType = "apply"  // 应用快照
	TaskTypeBackup SyncTaskType = "backup" // 备份配置
)

// SyncTaskStatus 任务状态
type SyncTaskStatus string

const (
	StatusPending   SyncTaskStatus = "pending"   // 等待执行
	StatusRunning   SyncTaskStatus = "running"   // 执行中
	StatusCompleted SyncTaskStatus = "completed" // 已完成
	StatusFailed    SyncTaskStatus = "failed"    // 失败
	StatusCancelled SyncTaskStatus = "cancelled" // 已取消
)

// SyncDirection 同步方向
type SyncDirection string

const (
	DirectionUpload   SyncDirection = "upload"   // 上传（本地 → 远端）
	DirectionDownload SyncDirection = "download" // 下载（远端 → 本地）
	DirectionBoth     SyncDirection = "both"     // 双向同步
)

// TaskProgress 任务进度
type TaskProgress struct {
	Percentage int      `json:"percentage"` // 进度百分比 (0-100)
	Current    int      `json:"current"`    // 当前处理数量
	Total      int      `json:"total"`      // 总数量
	Message    string   `json:"message"`    // 进度消息
	Steps      []string `json:"steps"`      // 已完成的步骤
}

// TaskMetadata 任务元数据
type TaskMetadata struct {
	Tools       []string `json:"tools,omitempty"`        // 涉及的工具
	ProjectPath string   `json:"project_path,omitempty"` // 项目路径
	Scope       string   `json:"scope,omitempty"`        // 任务范围
	RetryCount  int      `json:"retry_count,omitempty"`  // 重试次数
	MaxRetries  int      `json:"max_retries,omitempty"`  // 最大重试次数
}

// SyncTaskDAO 同步任务数据访问对象
type SyncTaskDAO struct {
	db *database.DB
}

// NewSyncTaskDAO 创建同步任务 DAO
func NewSyncTaskDAO(db *database.DB) *SyncTaskDAO {
	return &SyncTaskDAO{db: db}
}

// Create 创建同步任务
func (dao *SyncTaskDAO) Create(task *SyncTask) error {
	row, err := syncTaskToRow(task)
	if err != nil {
		return err
	}
	return dao.db.GetConn().Create(&row).Error
}

// GetByID 根据 ID 获取同步任务
func (dao *SyncTaskDAO) GetByID(id string) (*SyncTask, error) {
	var row syncTaskRow
	err := dao.db.GetConn().First(&row, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("任务不存在")
	}
	if err != nil {
		return nil, err
	}
	task, err := row.toModel()
	if err != nil {
		return nil, err
	}
	return task, nil
}

// Update 更新同步任务
func (dao *SyncTaskDAO) Update(task *SyncTask) error {
	row, err := syncTaskToRow(task)
	if err != nil {
		return err
	}

	return dao.db.GetConn().
		Model(&syncTaskRow{}).
		Where("id = ?", task.ID).
		Updates(map[string]interface{}{
			"status":           row.Status,
			"started_at":       row.StartedAt,
			"completed_at":     row.CompletedAt,
			"progress_current": row.ProgressCurrent,
			"progress_total":   row.ProgressTotal,
			"progress_message": row.ProgressMessage,
			"error_msg":        row.ErrorMsg,
			"metadata":         row.Metadata,
		}).Error
}

// List 列出同步任务
func (dao *SyncTaskDAO) List(limit, offset int, status SyncTaskStatus) ([]*SyncTask, error) {
	query := dao.db.GetConn().Model(&syncTaskRow{})
	if status != "" {
		query = query.Where("status = ?", string(status))
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	var rows []syncTaskRow
	if err := query.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}

	tasks := make([]*SyncTask, 0, len(rows))
	for _, row := range rows {
		task, err := row.toModel()
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

type syncTaskRow struct {
	ID              string     `gorm:"column:id;primaryKey"`
	Type            string     `gorm:"column:type"`
	Status          string     `gorm:"column:status"`
	SnapshotID      *string    `gorm:"column:snapshot_id"`
	Direction       string     `gorm:"column:direction"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	StartedAt       *time.Time `gorm:"column:started_at"`
	CompletedAt     *time.Time `gorm:"column:completed_at"`
	ProgressCurrent int        `gorm:"column:progress_current"`
	ProgressTotal   int        `gorm:"column:progress_total"`
	ProgressMessage string     `gorm:"column:progress_message"`
	ErrorMsg        string     `gorm:"column:error_msg"`
	Metadata        string     `gorm:"column:metadata"`
}

func (syncTaskRow) TableName() string {
	return "sync_tasks"
}

func syncTaskToRow(task *SyncTask) (syncTaskRow, error) {
	metadataJSON, err := json.Marshal(task.Metadata)
	if err != nil {
		return syncTaskRow{}, fmt.Errorf("序列化任务元数据失败: %w", err)
	}

	var snapshotID *string
	if task.SnapshotID != "" {
		snapshotID = &task.SnapshotID
	}

	return syncTaskRow{
		ID:              task.ID,
		Type:            string(task.Type),
		Status:          string(task.Status),
		SnapshotID:      snapshotID,
		Direction:       string(task.Direction),
		CreatedAt:       task.CreatedAt,
		StartedAt:       task.StartedAt,
		CompletedAt:     task.CompletedAt,
		ProgressCurrent: task.Progress.Current,
		ProgressTotal:   task.Progress.Total,
		ProgressMessage: task.Progress.Message,
		ErrorMsg:        task.Error,
		Metadata:        string(metadataJSON),
	}, nil
}

func (row syncTaskRow) toModel() (*SyncTask, error) {
	task := &SyncTask{
		ID:          row.ID,
		Type:        SyncTaskType(row.Type),
		Status:      SyncTaskStatus(row.Status),
		Direction:   SyncDirection(row.Direction),
		CreatedAt:   row.CreatedAt,
		StartedAt:   row.StartedAt,
		CompletedAt: row.CompletedAt,
		Progress: TaskProgress{
			Current: row.ProgressCurrent,
			Total:   row.ProgressTotal,
			Message: row.ProgressMessage,
		},
		Error: row.ErrorMsg,
	}
	if row.SnapshotID != nil {
		task.SnapshotID = *row.SnapshotID
	}

	if row.Metadata != "" {
		if err := json.Unmarshal([]byte(row.Metadata), &task.Metadata); err != nil {
			return nil, fmt.Errorf("反序列化任务元数据失败: %w", err)
		}
	}
	return task, nil
}
