package models

import (
	"fmt"
	"time"

	"ai-sync-manager/pkg/database"
)

// RegisteredProject 表示用户登记参与扫描的项目。
type RegisteredProject struct {
	ID          string    `json:"id" db:"id" gorm:"column:id;primaryKey"`
	ToolType    string    `json:"tool_type" db:"tool_type" gorm:"column:tool_type;not null"`
	ProjectName string    `json:"project_name" db:"project_name" gorm:"column:project_name;not null"`
	ProjectPath string    `json:"project_path" db:"project_path" gorm:"column:project_path;not null"`
	CreatedAt   time.Time `json:"created_at" db:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// RegisteredProjectDAO 已注册项目数据访问对象。
type RegisteredProjectDAO struct {
	db *database.DB
}

// NewRegisteredProjectDAO 创建已注册项目 DAO。
func NewRegisteredProjectDAO(db *database.DB) *RegisteredProjectDAO {
	return &RegisteredProjectDAO{db: db}
}

// Create 创建已注册项目。
func (dao *RegisteredProjectDAO) Create(project *RegisteredProject) error {
	return dao.db.GetConn().Create(project).Error
}

// ListByTool 按工具列出已注册项目。
func (dao *RegisteredProjectDAO) ListByTool(toolType string) ([]*RegisteredProject, error) {
	var projects []*RegisteredProject
	err := dao.db.GetConn().
		Where("tool_type = ?", toolType).
		Order("created_at DESC").
		Find(&projects).Error
	if err != nil {
		return nil, err
	}

	return projects, nil
}

// GetByToolAndName 根据工具类型和项目名查找已注册项目。
func (dao *RegisteredProjectDAO) GetByToolAndName(toolType, projectName string) (*RegisteredProject, error) {
	var project RegisteredProject
	err := dao.db.GetConn().
		Where("tool_type = ? AND project_name = ?", toolType, projectName).
		First(&project).Error
	if err != nil {
		return nil, err
	}
	return &project, nil
}

// DeleteByToolAndPath 删除指定工具与路径的已注册项目。
func (dao *RegisteredProjectDAO) DeleteByToolAndPath(toolType, projectPath string) error {
	result := dao.db.GetConn().
		Where("tool_type = ? AND project_path = ?", toolType, projectPath).
		Delete(&RegisteredProject{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("未找到已注册项目")
	}

	return nil
}

func (RegisteredProject) TableName() string {
	return "registered_projects"
}
