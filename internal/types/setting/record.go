package setting

import "time"

// AISettingRecord carries AI setting data across the Service-UseCase boundary.
// It decouples the UseCase layer from the models (database) layer.
type AISettingRecord struct {
	ID        uint                   `json:"id" yaml:"id"`
	Name      string                 `json:"name" yaml:"name"`
	Token     string                 `json:"token" yaml:"token"`
	BaseURL   string                 `json:"base_url" yaml:"base_url"`
	Models    ModelList              `json:"models" yaml:"models"`
	CreatedAt time.Time              `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time              `json:"updated_at" yaml:"updated_at"`
}
