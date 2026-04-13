package models

import (
	"time"

	"flux/pkg/database"
)

// AISetting 表示保存的 AI 配置。
type AISetting struct {
	ID          uint      `json:"id" db:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Name        string    `json:"name" db:"name" gorm:"column:name;not null;uniqueIndex"`
	Token       string    `json:"token" db:"token" gorm:"column:token;not null"`
	BaseURL     string    `json:"base_url" db:"base_url" gorm:"column:base_url"`
	OpusModel   string    `json:"opus_model" db:"opus_model" gorm:"column:opus_model"`
	SonnetModel string    `json:"sonnet_model" db:"sonnet_model" gorm:"column:sonnet_model"`
	CreatedAt   time.Time `json:"created_at" db:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// AISettingDAO AI 配置数据访问对象。
type AISettingDAO struct {
	db *database.DB
}

// NewAISettingDAO 创建 AI 配置 DAO。
func NewAISettingDAO(db *database.DB) *AISettingDAO {
	return &AISettingDAO{db: db}
}

// Create 创建 AI 配置。
func (dao *AISettingDAO) Create(setting *AISetting) error {
	return dao.db.GetConn().Create(setting).Error
}

// GetByName 按名称获取 AI 配置。
func (dao *AISettingDAO) GetByName(name string) (*AISetting, error) {
	var setting AISetting
	err := dao.db.GetConn().
		Where("name = ?", name).
		First(&setting).Error
	if err != nil {
		return nil, err
	}
	return &setting, nil
}

// List 列出所有 AI 配置。
func (dao *AISettingDAO) List() ([]*AISetting, error) {
	var settings []*AISetting
	err := dao.db.GetConn().
		Order("created_at DESC").
		Find(&settings).Error
	if err != nil {
		return nil, err
	}
	return settings, nil
}

// ListPaginated returns a page of AI settings ordered by creation time descending.
// Returns the settings for the requested page and the total count of all settings.
func (dao *AISettingDAO) ListPaginated(limit, offset int) ([]*AISetting, int, error) {
	var total int64
	if err := dao.db.GetConn().Model(&AISetting{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var settings []*AISetting
	query := dao.db.GetConn().Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&settings).Error; err != nil {
		return nil, 0, err
	}

	return settings, int(total), nil
}

// Delete 按名称删除 AI 配置。
func (dao *AISettingDAO) Delete(name string) error {
	result := dao.db.GetConn().
		Where("name = ?", name).
		Delete(&AISetting{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// UpdateByName 按名称更新 AI 配置。
// 如果新名称与原名称不同，会先检查新名称是否已存在。
func (dao *AISettingDAO) UpdateByName(oldName string, setting *AISetting) error {
	// 开启事务
	tx := dao.db.GetConn().Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer func() {
		if tx.Error != nil {
			tx.Rollback()
		}
	}()

	// 如果名称发生变化，检查新名称是否已存在
	if oldName != setting.Name {
		var existing int64
		if err := tx.Model(&AISetting{}).Where("name = ? AND id != ?", setting.Name, setting.ID).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return ErrDuplicateName
		}
	}

	// 执行更新
	result := tx.Model(&AISetting{}).Where("name = ?", oldName).Updates(setting)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRecordNotFound
	}

	return tx.Commit().Error
}

// ErrRecordNotFound 表示记录未找到的错误。
var ErrRecordNotFound = RecordNotFound("记录未找到")

// ErrDuplicateName 表示名称重复错误。
var ErrDuplicateName = DuplicateName("配置名称已存在")

// RecordNotFound 记录未找到错误类型。
type RecordNotFound string

func (e RecordNotFound) Error() string {
	return string(e)
}

// DuplicateName 名称重复错误类型。
type DuplicateName string

func (e DuplicateName) Error() string {
	return string(e)
}

func (AISetting) TableName() string {
	return "ai_settings"
}
