package setting

import (
	"ai-sync-manager/internal/models"
	typesSetting "ai-sync-manager/internal/types/setting"
)

// AISettingManager 实现 usecase.AISettingManager 接口。
// 它是对 AISettingDAO 的适配器，将 DAO 方法转换为 Manager 接口方法。
type AISettingManager struct {
	dao *models.AISettingDAO
}

// NewAISettingManager 创建 AISettingManager 实例。
func NewAISettingManager(dao *models.AISettingDAO) *AISettingManager {
	return &AISettingManager{dao: dao}
}

// Create 创建 AI 配置。
func (m *AISettingManager) Create(setting *typesSetting.AISettingRecord) error {
	model := &models.AISetting{
		ID:          setting.ID,
		Name:        setting.Name,
		Token:       setting.Token,
		BaseURL:     setting.BaseURL,
		OpusModel:   setting.OpusModel,
		SonnetModel: setting.SonnetModel,
		CreatedAt:   setting.CreatedAt,
		UpdatedAt:   setting.UpdatedAt,
	}
	return m.dao.Create(model)
}

// GetByName 按名称获取 AI 配置。
func (m *AISettingManager) GetByName(name string) (*typesSetting.AISettingRecord, error) {
	model, err := m.dao.GetByName(name)
	if err != nil {
		return nil, err
	}
	return &typesSetting.AISettingRecord{
		ID:          model.ID,
		Name:        model.Name,
		Token:       model.Token,
		BaseURL:     model.BaseURL,
		OpusModel:   model.OpusModel,
		SonnetModel: model.SonnetModel,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}, nil
}

// List 列出所有 AI 配置。
func (m *AISettingManager) List() ([]*typesSetting.AISettingRecord, error) {
	models, err := m.dao.List()
	if err != nil {
		return nil, err
	}
	records := make([]*typesSetting.AISettingRecord, 0, len(models))
	for _, model := range models {
		records = append(records, &typesSetting.AISettingRecord{
			ID:          model.ID,
			Name:        model.Name,
			Token:       model.Token,
			BaseURL:     model.BaseURL,
			OpusModel:   model.OpusModel,
			SonnetModel: model.SonnetModel,
			CreatedAt:   model.CreatedAt,
			UpdatedAt:   model.UpdatedAt,
		})
	}
	return records, nil
}

// ListPaginated 分页列出 AI 配置。
func (m *AISettingManager) ListPaginated(limit, offset int) ([]*typesSetting.AISettingRecord, int, error) {
	models, total, err := m.dao.ListPaginated(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	records := make([]*typesSetting.AISettingRecord, 0, len(models))
	for _, model := range models {
		records = append(records, &typesSetting.AISettingRecord{
			ID:          model.ID,
			Name:        model.Name,
			Token:       model.Token,
			BaseURL:     model.BaseURL,
			OpusModel:   model.OpusModel,
			SonnetModel: model.SonnetModel,
			CreatedAt:   model.CreatedAt,
			UpdatedAt:   model.UpdatedAt,
		})
	}
	return records, total, nil
}

// Delete 删除 AI 配置。
func (m *AISettingManager) Delete(name string) error {
	return m.dao.Delete(name)
}

// UpdateByName 按名称更新 AI 配置。
func (m *AISettingManager) UpdateByName(oldName string, record *typesSetting.AISettingRecord) error {
	model := &models.AISetting{
		ID:          record.ID,
		Name:        record.Name,
		Token:       record.Token,
		BaseURL:     record.BaseURL,
		OpusModel:   record.OpusModel,
		SonnetModel: record.SonnetModel,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
	return m.dao.UpdateByName(oldName, model)
}
