package setting

import (
	"errors"

	"ai-sync-manager/internal/models"
	typesCommon "ai-sync-manager/internal/types/common"
	typesSetting "ai-sync-manager/internal/types/setting"
)

// AISettingService AI 配置服务。
// 将 DAO 层的 models 类型转换为 UseCase 层的 types 类型，
// 避免上层直接依赖 models 包。
type AISettingService struct {
	dao *models.AISettingDAO
}

// NewAISettingService 创建 AI 配置服务。
func NewAISettingService(dao *models.AISettingDAO) *AISettingService {
	return &AISettingService{dao: dao}
}

// Create 创建 AI 配置。
func (s *AISettingService) Create(record *typesSetting.AISettingRecord) error {
	setting := &models.AISetting{
		ID:          record.ID,
		Name:        record.Name,
		Token:       record.Token,
		BaseURL:     record.BaseURL,
		OpusModel:   record.OpusModel,
		SonnetModel: record.SonnetModel,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
	if err := s.dao.Create(setting); err != nil {
		return s.wrapError(err)
	}
	return nil
}

// GetByName 按名称获取 AI 配置。
func (s *AISettingService) GetByName(name string) (*typesSetting.AISettingRecord, error) {
	setting, err := s.dao.GetByName(name)
	if err != nil {
		return nil, s.wrapError(err)
	}
	return toRecord(setting), nil
}

// List 列出所有 AI 配置。
func (s *AISettingService) List() ([]*typesSetting.AISettingRecord, error) {
	settings, err := s.dao.List()
	if err != nil {
		return nil, s.wrapError(err)
	}
	records := make([]*typesSetting.AISettingRecord, 0, len(settings))
	for _, s := range settings {
		records = append(records, toRecord(s))
	}
	return records, nil
}

// Delete 按名称删除 AI 配置。
func (s *AISettingService) Delete(name string) error {
	if err := s.dao.Delete(name); err != nil {
		return s.wrapError(err)
	}
	return nil
}

// toRecord converts a models.AISetting to a types-level AISettingRecord.
func toRecord(s *models.AISetting) *typesSetting.AISettingRecord {
	return &typesSetting.AISettingRecord{
		ID:          s.ID,
		Name:        s.Name,
		Token:       s.Token,
		BaseURL:     s.BaseURL,
		OpusModel:   s.OpusModel,
		SonnetModel: s.SonnetModel,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

// wrapError converts DAO-level errors to types-level equivalents.
func (s *AISettingService) wrapError(err error) error {
	if errors.Is(err, models.ErrRecordNotFound) {
		return typesCommon.ErrRecordNotFound
	}
	return err
}
