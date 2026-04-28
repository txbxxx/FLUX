package setting

import (
	"flux/internal/models"
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
