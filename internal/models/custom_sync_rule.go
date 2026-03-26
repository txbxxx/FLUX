package models

import (
	"fmt"
	"time"

	"ai-sync-manager/pkg/database"
)

// CustomSyncRule 表示用户登记的绝对路径文件规则。
type CustomSyncRule struct {
	ID           string    `json:"id" db:"id"`
	ToolType     string    `json:"tool_type" db:"tool_type"`
	AbsolutePath string    `json:"absolute_path" db:"absolute_path"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
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
	conn := dao.db.GetConn()

	query := `
		INSERT INTO custom_sync_rules (id, tool_type, absolute_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := conn.Exec(
		query,
		rule.ID,
		rule.ToolType,
		rule.AbsolutePath,
		rule.CreatedAt.Unix(),
		rule.UpdatedAt.Unix(),
	)
	return err
}

// ListByTool 按工具列出自定义同步规则。
func (dao *CustomSyncRuleDAO) ListByTool(toolType string) ([]*CustomSyncRule, error) {
	conn := dao.db.GetConn()

	query := `
		SELECT id, tool_type, absolute_path, created_at, updated_at
		FROM custom_sync_rules
		WHERE tool_type = ?
		ORDER BY created_at DESC
	`

	rows, err := conn.Query(query, toolType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*CustomSyncRule
	for rows.Next() {
		var (
			id, loadedToolType, absolutePath string
			createdAt, updatedAt             int64
		)

		if err := rows.Scan(&id, &loadedToolType, &absolutePath, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		rules = append(rules, &CustomSyncRule{
			ID:           id,
			ToolType:     loadedToolType,
			AbsolutePath: absolutePath,
			CreatedAt:    time.Unix(createdAt, 0),
			UpdatedAt:    time.Unix(updatedAt, 0),
		})
	}

	return rules, nil
}

// DeleteByToolAndPath 删除指定工具与路径的自定义规则。
func (dao *CustomSyncRuleDAO) DeleteByToolAndPath(toolType, absolutePath string) error {
	conn := dao.db.GetConn()

	result, err := conn.Exec(
		"DELETE FROM custom_sync_rules WHERE tool_type = ? AND absolute_path = ?",
		toolType,
		absolutePath,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("未找到自定义同步规则")
	}

	return nil
}
