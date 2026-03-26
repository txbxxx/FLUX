package tool

import "ai-sync-manager/internal/models"

// RuleStore 聚合自定义绝对文件规则与已注册项目。
type RuleStore struct {
	customRules *models.CustomSyncRuleDAO
	projects    *models.RegisteredProjectDAO
}

// NewRuleStore 创建规则仓储。
func NewRuleStore(customRules *models.CustomSyncRuleDAO, projects *models.RegisteredProjectDAO) *RuleStore {
	return &RuleStore{
		customRules: customRules,
		projects:    projects,
	}
}

// ListCustomRules 列出某工具下的绝对文件规则。
func (s *RuleStore) ListCustomRules(toolType ToolType) ([]*models.CustomSyncRule, error) {
	if s == nil || s.customRules == nil {
		return nil, nil
	}
	return s.customRules.ListByTool(toolType.String())
}

// ListRegisteredProjects 列出某工具下的已注册项目。
func (s *RuleStore) ListRegisteredProjects(toolType ToolType) ([]*models.RegisteredProject, error) {
	if s == nil || s.projects == nil {
		return nil, nil
	}
	return s.projects.ListByTool(toolType.String())
}
