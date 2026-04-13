package setting

import "time"

// AISettingRecord carries AI setting data across the Service-UseCase boundary.
// It decouples the UseCase layer from the models (database) layer.
type AISettingRecord struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Token       string    `json:"token"`
	BaseURL     string    `json:"base_url"`
	OpusModel   string    `json:"opus_model"`
	SonnetModel string    `json:"sonnet_model"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
