package models

import "time"

// ProviderHealth stores the latest health check result for a provider config.
type ProviderHealth struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ProviderName   string     `gorm:"size:100;uniqueIndex;not null" json:"provider_name"`
	ProviderType   string     `gorm:"size:50;index" json:"provider_type"`
	Healthy        bool       `gorm:"index" json:"healthy"`
	Status         string     `gorm:"size:20;index" json:"status"`
	LatencyMS      int64      `json:"latency_ms"`
	ErrorMessage   string     `gorm:"type:text" json:"error_message,omitempty"`
	LastCheckedAt  time.Time  `gorm:"index" json:"last_checked_at"`
	LastSuccessAt  *time.Time `json:"last_success_at,omitempty"`
	ConsecutiveErr int        `json:"consecutive_errors"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (ProviderHealth) TableName() string {
	return "provider_health"
}
