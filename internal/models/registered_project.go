package models

import (
	"fmt"
	"time"

	"ai-sync-manager/pkg/database"
)

// RegisteredProject 表示用户登记参与扫描的项目。
type RegisteredProject struct {
	ID          string    `json:"id" db:"id"`
	ToolType    string    `json:"tool_type" db:"tool_type"`
	ProjectName string    `json:"project_name" db:"project_name"`
	ProjectPath string    `json:"project_path" db:"project_path"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
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
	conn := dao.db.GetConn()

	query := `
		INSERT INTO registered_projects (id, tool_type, project_name, project_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := conn.Exec(
		query,
		project.ID,
		project.ToolType,
		project.ProjectName,
		project.ProjectPath,
		project.CreatedAt.Unix(),
		project.UpdatedAt.Unix(),
	)
	return err
}

// ListByTool 按工具列出已注册项目。
func (dao *RegisteredProjectDAO) ListByTool(toolType string) ([]*RegisteredProject, error) {
	conn := dao.db.GetConn()

	query := `
		SELECT id, tool_type, project_name, project_path, created_at, updated_at
		FROM registered_projects
		WHERE tool_type = ?
		ORDER BY created_at DESC
	`

	rows, err := conn.Query(query, toolType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*RegisteredProject
	for rows.Next() {
		var (
			id, loadedToolType, projectName, projectPath string
			createdAt, updatedAt                         int64
		)

		if err := rows.Scan(&id, &loadedToolType, &projectName, &projectPath, &createdAt, &updatedAt); err != nil {
			return nil, err
		}

		projects = append(projects, &RegisteredProject{
			ID:          id,
			ToolType:    loadedToolType,
			ProjectName: projectName,
			ProjectPath: projectPath,
			CreatedAt:   time.Unix(createdAt, 0),
			UpdatedAt:   time.Unix(updatedAt, 0),
		})
	}

	return projects, nil
}

// DeleteByToolAndPath 删除指定工具与路径的已注册项目。
func (dao *RegisteredProjectDAO) DeleteByToolAndPath(toolType, projectPath string) error {
	conn := dao.db.GetConn()

	result, err := conn.Exec(
		"DELETE FROM registered_projects WHERE tool_type = ? AND project_path = ?",
		toolType,
		projectPath,
	)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("未找到已注册项目")
	}

	return nil
}
