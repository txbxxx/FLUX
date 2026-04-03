package models

import (
	"fmt"
	"time"

	"ai-sync-manager/pkg/database"
)

// CustomSyncRule 表示用户登记的绝对路径文件规则。
type CustomSyncRule struct {
	ID           string    `json:"id" db:"id" gorm:"column:id;primaryKey"`
	ToolType     string    `json:"tool_type" db:"tool_type" gorm:"column:tool_type;not null"`
	AbsolutePath string    `json:"absolute_path" db:"absolute_path" gorm:"column:absolute_path;not null"`
	CreatedAt    time.Time `json:"created_at" db:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// CustomSyncRuleDAO 自定义同步规则数据访问对象。
type CustomSyncRuleDAO struct {
	db *database.DB
}

// NewCustomSyncRuleDAO 创建自定义同步规则 DAO。
func NewCustomSyncRuleDAO(db *database.DB) *CustomSyncRuleDAO {
	return &CustomSyncRuleDAO{db: db}
}

// Create 创建自定义同步规则。
func (dao *CustomSyncRuleDAO) Create(rule *CustomSyncRule) error {
	return dao.db.GetConn().Create(rule).Error
}

// ListByTool 按工具列出自定义同步规则。
func (dao *CustomSyncRuleDAO) ListByTool(toolType string) ([]*CustomSyncRule, error) {
	var rules []*CustomSyncRule
	err := dao.db.GetConn().
		Where("tool_type = ?", toolType).
		Order("created_at DESC").
		Find(&rules).Error
	if err != nil {
		return nil, err
	}

	return rules, nil
}

// DeleteByToolAndPath 删除指定工具与路径的自定义规则。
func (dao *CustomSyncRuleDAO) DeleteByToolAndPath(toolType, absolutePath string) error {
	result := dao.db.GetConn().
		Where("tool_type = ? AND absolute_path = ?", toolType, absolutePath).
		Delete(&CustomSyncRule{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("未找到自定义同步规则")
	}
	return nil
}

func (CustomSyncRule) TableName() string {
	return "custom_sync_rules"
}
