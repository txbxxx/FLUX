package setting

import "time"

// AISettingRecord carries AI setting data across the Service-UseCase boundary.
// It decouples the UseCase layer from the models (database) layer.
type AISettingRecord struct {
	ID          uint      `json:"id" yaml:"id"`
	Name        string    `json:"name" yaml:"name"`
	Token       string    `json:"token" yaml:"token"`
	BaseURL     string    `json:"base_url" yaml:"base_url"`
	OpusModel   string    `json:"opus_model" yaml:"opus_model"`
	SonnetModel string    `json:"sonnet_model" yaml:"sonnet_model"`
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"updated_at"`
}
