package setting

import (
	"context"
	"fmt"
	"strings"
	"time"

	"flux/internal/models"
	"flux/internal/types/setting"
)

// DAO 定义配置数据访问接口。
type DAO interface {
	GetByName(name string) (*models.AISetting, error)
	UpdateByName(oldName string, setting *models.AISetting) error
}

// Service 提供配置编辑的业务逻辑。
type Service struct {
	dao DAO
}

// NewService 创建配置服务。
func NewService(dao DAO) *Service {
	return &Service{dao: dao}
}

// EditInput 编辑输入。
type EditInput struct {
	Name        string
	NewName     string
	Token       string
	BaseURL     string
	OpusModel   string
	SonnetModel string
}

// EditOutput 编辑输出。
type EditOutput struct {
	ID        string
	Name      string
	UpdatedAt time.Time
	Changes   []setting.FieldChange
}

// Edit 编辑配置。
func (s *Service) Edit(ctx context.Context, input EditInput) (*EditOutput, error) {
	// 读取现有配置
	existing, err := s.dao.GetByName(input.Name)
	if err != nil {
		return nil, fmt.Errorf("配置不存在: %s", input.Name)
	}

	changes := make([]setting.FieldChange, 0)
	updated := &models.AISetting{
		ID:          existing.ID,
		Name:        existing.Name,
		Token:       existing.Token,
		BaseURL:     existing.BaseURL,
		OpusModel:   existing.OpusModel,
		SonnetModel: existing.SonnetModel,
	}

	// 处理名称变更
	newName := input.NewName
	if newName != "" {
		newName = strings.TrimSpace(newName)
		if newName != existing.Name {
			changes = append(changes, setting.FieldChange{
				Field:    "name",
				OldValue: existing.Name,
				NewValue: newName,
			})
			updated.Name = newName
		}
	}

	// 处理 Token 变更
	if input.Token != "" {
		changes = append(changes, setting.FieldChange{
			Field:    "token",
			OldValue: maskToken(existing.Token),
			NewValue: maskToken(input.Token),
		})
		updated.Token = input.Token
	}

	// 处理 BaseURL 变更
	if input.BaseURL != "" {
		changes = append(changes, setting.FieldChange{
			Field:    "base_url",
			OldValue: existing.BaseURL,
			NewValue: input.BaseURL,
		})
		updated.BaseURL = input.BaseURL
	}

	// 处理 OpusModel 变更
	if input.OpusModel != "" {
		changes = append(changes, setting.FieldChange{
			Field:    "opus_model",
			OldValue: existing.OpusModel,
			NewValue: input.OpusModel,
		})
		updated.OpusModel = input.OpusModel
	}

	// 处理 SonnetModel 变更
	if input.SonnetModel != "" {
		changes = append(changes, setting.FieldChange{
			Field:    "sonnet_model",
			OldValue: existing.SonnetModel,
			NewValue: input.SonnetModel,
		})
		updated.SonnetModel = input.SonnetModel
	}

	// 如果没有任何变更，直接返回
	if len(changes) == 0 {
		return &EditOutput{
			ID:        existing.ID,
			Name:      existing.Name,
			UpdatedAt: existing.UpdatedAt,
			Changes:   nil,
		}, nil
	}

	// 执行更新
	if err := s.dao.UpdateByName(input.Name, updated); err != nil {
		return nil, fmt.Errorf("更新配置失败: %w", err)
	}

	return &EditOutput{
		ID:        updated.ID,
		Name:      updated.Name,
		UpdatedAt: time.Now(),
		Changes:   changes,
	}, nil
}

// maskToken 脱敏 token。
func maskToken(token string) string {
	if len(token) > 8 {
		return token[:4] + "****" + token[len(token)-4:]
	}
	if len(token) > 0 {
		return "****"
	}
	return ""
}
