package setting

import (
	"ai-sync-manager/internal/models"
)

// AISettingService AI 配置服务。
type AISettingService struct {
	dao *models.AISettingDAO
}

// NewAISettingService 创建 AI 配置服务。
func NewAISettingService(dao *models.AISettingDAO) *AISettingService {
	return &AISettingService{dao: dao}
}

// Create 创建 AI 配置。
func (s *AISettingService) Create(setting *models.AISetting) error {
	return s.dao.Create(setting)
}

// GetByName 按名称获取 AI 配置。
func (s *AISettingService) GetByName(name string) (*models.AISetting, error) {
	return s.dao.GetByName(name)
}

// List 列出所有 AI 配置。
func (s *AISettingService) List() ([]*models.AISetting, error) {
	return s.dao.List()
}

// Delete 按名称删除 AI 配置。
func (s *AISettingService) Delete(name string) error {
	return s.dao.Delete(name)
}
