package tool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"flux/internal/models"
	typesScan "flux/internal/types/scan"
	"flux/pkg/database"
)

// RuleManager 负责统一处理规则持久化和路径规范化。
// 规则一旦入库，就要求使用稳定的绝对路径，避免后续比较时出现同一路径多种写法。
type RuleManager struct {
	store *RuleStore
}

// NewRuleManager 基于数据库初始化规则管理器。
func NewRuleManager(db *database.DB) *RuleManager {
	if db == nil {
		return &RuleManager{}
	}

	return &RuleManager{
		store: NewRuleStore(
			models.NewCustomSyncRuleDAO(db),
			models.NewRegisteredProjectDAO(db),
		),
	}
}

// Store 暴露只读仓储，供 resolver 复用同一套规则数据。
func (m *RuleManager) Store() *RuleStore {
	if m == nil {
		return nil
	}
	return m.store
}

// AddCustomRule adds a custom sync rule for a specific tool type.
//
// The path must be an absolute path to an existing file, not a directory.
// The path is normalized and stored with a generated ID and timestamp.
func (m *RuleManager) AddCustomRule(toolType ToolType, absolutePath string) error {
	if m == nil || m.store == nil || m.store.customRules == nil {
		return errors.New("规则存储未初始化")
	}

	normalizedPath, info, err := normalizeExistingPath(absolutePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("自定义规则只支持文件：%s", normalizedPath)
	}

	now := time.Now()
	return m.store.customRules.Create(&models.CustomSyncRule{
		ID:           0, // GORM 自动生成自增 ID
		ToolType:     toolType.String(),
		AbsolutePath: normalizedPath,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
}

// RemoveCustomRule removes a custom sync rule for a specific tool type.
//
// The path must be an absolute path and will be normalized before matching.
// Only exact path matches are removed.
func (m *RuleManager) RemoveCustomRule(toolType ToolType, absolutePath string) error {
	if m == nil || m.store == nil || m.store.customRules == nil {
		return errors.New("规则存储未初始化")
	}

	normalizedPath, err := normalizeStoredPath(absolutePath)
	if err != nil {
		return err
	}
	return m.store.customRules.DeleteByToolAndPath(toolType.String(), normalizedPath)
}

// AddProject registers a project for a specific tool type with a unique name.
//
// The project path must be an absolute path to an existing directory.
// The project name must not be empty and is used to identify the project.
func (m *RuleManager) AddProject(toolType ToolType, projectName, projectPath string) error {
	if m == nil || m.store == nil || m.store.projects == nil {
		return errors.New("规则存储未初始化")
	}

	normalizedPath, info, err := normalizeExistingPath(projectPath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("项目路径必须是目录：%s", normalizedPath)
	}

	name := strings.TrimSpace(projectName)
	if name == "" {
		return errors.New("项目名称不能为空")
	}

	now := time.Now()
	return m.store.projects.Create(&models.RegisteredProject{
		ID:          0, // GORM 自动生成自增 ID
		ToolType:    toolType.String(),
		ProjectName: name,
		ProjectPath: normalizedPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

// RemoveProject removes a registered project for a specific tool type.
//
// The project path must be an absolute path and will be normalized before matching.
// Only exact path matches are removed.
func (m *RuleManager) RemoveProject(toolType ToolType, projectPath string) error {
	if m == nil || m.store == nil || m.store.projects == nil {
		return errors.New("规则存储未初始化")
	}

	normalizedPath, err := normalizeStoredPath(projectPath)
	if err != nil {
		return err
	}
	return m.store.projects.DeleteByToolAndPath(toolType.String(), normalizedPath)
}

// ListCustomRules returns all custom sync rules for the specified tool type.
//
// If toolType is nil, returns rules for all tool types.
// Each rule includes the tool type and absolute path.
func (m *RuleManager) ListCustomRules(toolType *ToolType) ([]typesScan.CustomRuleRecord, error) {
	if m == nil || m.store == nil {
		return nil, nil
	}

	rules, err := m.collectCustomRules(toolType)
	if err != nil {
		return nil, err
	}

	items := make([]typesScan.CustomRuleRecord, 0, len(rules))
	for _, rule := range rules {
		items = append(items, typesScan.CustomRuleRecord{
			ToolType:     rule.ToolType,
			AbsolutePath: rule.AbsolutePath,
		})
	}
	return items, nil
}

// ListRegisteredProjects returns all registered projects for the specified tool type.
//
// If toolType is nil, returns projects for all tool types.
// Each project includes the tool type, name, and path.
func (m *RuleManager) ListRegisteredProjects(toolType *ToolType) ([]typesScan.RegisteredProjectRecord, error) {
	if m == nil || m.store == nil {
		return nil, nil
	}

	projects, err := m.collectProjects(toolType)
	if err != nil {
		return nil, err
	}

	items := make([]typesScan.RegisteredProjectRecord, 0, len(projects))
	for _, project := range projects {
		items = append(items, typesScan.RegisteredProjectRecord{
			ToolType:    project.ToolType,
			ProjectName: project.ProjectName,
			ProjectPath: project.ProjectPath,
		})
	}
	return items, nil
}

func (m *RuleManager) collectCustomRules(toolType *ToolType) ([]*models.CustomSyncRule, error) {
	if toolType != nil {
		return m.store.ListCustomRules(*toolType)
	}

	var result []*models.CustomSyncRule
	for _, current := range []ToolType{ToolTypeCodex, ToolTypeClaude} {
		rules, err := m.store.ListCustomRules(current)
		if err != nil {
			return nil, err
		}
		result = append(result, rules...)
	}
	return result, nil
}

func (m *RuleManager) collectProjects(toolType *ToolType) ([]*models.RegisteredProject, error) {
	if toolType != nil {
		return m.store.ListRegisteredProjects(*toolType)
	}

	var result []*models.RegisteredProject
	for _, current := range []ToolType{ToolTypeCodex, ToolTypeClaude} {
		projects, err := m.store.ListRegisteredProjects(current)
		if err != nil {
			return nil, err
		}
		result = append(result, projects...)
	}
	return result, nil
}

func normalizeStoredPath(path string) (string, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "." || cleanPath == "" {
		return "", errors.New("路径不能为空")
	}
	if !filepath.IsAbs(cleanPath) {
		return "", fmt.Errorf("路径必须是绝对路径：%s", cleanPath)
	}
	return cleanPath, nil
}

func normalizeExistingPath(path string) (string, os.FileInfo, error) {
	cleanPath, err := normalizeStoredPath(path)
	if err != nil {
		return "", nil, err
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil, fmt.Errorf("路径不存在：%s", cleanPath)
		}
		return "", nil, fmt.Errorf("读取路径失败: %w", err)
	}

	resolvedPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		resolvedPath = cleanPath
	}

	return filepath.Clean(resolvedPath), info, nil
}

// EnsureGlobalProjectsRegistered 确保全局项目已注册到数据库。
// 自动注册 codex-global 和 claude-global，使全局配置也能作为项目管理。
func (m *RuleManager) EnsureGlobalProjectsRegistered() error {
	if m == nil || m.store == nil || m.store.projects == nil {
		return nil // 数据库未初始化，跳过
	}

	// 为每个工具注册全局项目
	globalProjects := []struct {
		toolType ToolType
		name     string
	}{
		{ToolTypeCodex, "codex-global"},
		{ToolTypeClaude, "claude-global"},
	}

	for _, gp := range globalProjects {
		if err := m.ensureOneGlobalProject(gp.toolType, gp.name); err != nil {
			return fmt.Errorf("注册全局项目 %s 失败: %w", gp.name, err)
		}
	}

	return nil
}

// ensureOneGlobalProject 确保单个全局项目已注册。
func (m *RuleManager) ensureOneGlobalProject(toolType ToolType, projectName string) error {
	// 检查是否已存在
	existing, err := m.store.projects.GetByToolAndName(toolType.String(), projectName)
	if err == nil && existing != nil {
		return nil // 已存在，无需注册
	}

	// 不存在则注册
	globalPath := GetDefaultGlobalPath(toolType)
	if globalPath == "" {
		return fmt.Errorf("无法获取 %s 的全局路径", toolType)
	}

	now := time.Now()
	return m.store.projects.Create(&models.RegisteredProject{
		ID:          0, // GORM 自动生成自增 ID
		ToolType:    toolType.String(),
		ProjectName: projectName,
		ProjectPath: globalPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}
