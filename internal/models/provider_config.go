package models

import (
	"time"
)

// ProviderConfig represents a provider configuration stored in the database
type ProviderConfig struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	Name       string `gorm:"size:100;uniqueIndex;not null" json:"name"`
	ProviderType string `gorm:"size:50;not null;index" json:"provider_type"` // openai, anthropic, gemini, etc.
	APIKey     string `gorm:"type:text" json:"-"` // Encrypted API key, excluded from JSON
	APIKeyEncrypted string `gorm:"type:text" json:"api_key_encrypted,omitempty"` // Encrypted storage
	BaseURL    string `gorm:"size:255" json:"base_url"`
	Weight     int    `gorm:"default:1" json:"weight"`
	Enabled    bool   `gorm:"default:true;index" json:"enabled"`
	Config     string `gorm:"type:text" json:"config,omitempty"` // JSON for additional config (headers, etc.)
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	DeletedAt  *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name
func (ProviderConfig) TableName() string {
	return "provider_configs"
}

// ProviderConfigRequest is used for API requests
type ProviderConfigRequest struct {
	Name        string                 `json:"name" binding:"required"`
	ProviderType string                 `json:"provider_type" binding:"required"`
	APIKey      string                 `json:"api_key"`
	BaseURL     string                 `json:"base_url"`
	Weight      int                    `json:"weight"`
	Enabled     bool                   `json:"enabled"`
	Config      map[string]interface{} `json:"config"`
}

// ProviderConfigResponse is used for API responses
type ProviderConfigResponse struct {
	ID           uint                   `json:"id"`
	Name         string                 `json:"name"`
	ProviderType string                 `json:"provider_type"`
	APIKeyMasked string                 `json:"api_key_masked,omitempty"`
	BaseURL      string                 `json:"base_url"`
	Weight       int                    `json:"weight"`
	Enabled      bool                   `json:"enabled"`
	Config       map[string]interface{} `json:"config,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}
