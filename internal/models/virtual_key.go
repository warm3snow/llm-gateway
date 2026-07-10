package models

import (
	"time"
)

// VirtualKey represents a virtual key in the system
type VirtualKey struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	TenantID          uint       `gorm:"index;not null;default:1" json:"tenant_id"`
	CreatedByUserID   *uint      `gorm:"index" json:"created_by_user_id,omitempty"`
	CreatedByUsername string     `gorm:"size:100" json:"created_by_username,omitempty"`
	Name              string     `gorm:"size:100;not null" json:"name"`
	KeyHash           string     `gorm:"size:64;uniqueIndex;not null" json:"-"`
	KeySalt           string     `gorm:"size:32;not null" json:"-"`
	HashedKey         string     `gorm:"size:64;not null" json:"hashed_key"` // Public hash for identification
	BudgetTotal       float64    `gorm:"type:decimal(10,2);default:0" json:"budget_total"`
	BudgetUsed        float64    `gorm:"type:decimal(10,2);default:0" json:"budget_used"`
	BudgetResetAt     *time.Time `json:"budget_reset_at,omitempty"`
	RateLimit         int        `gorm:"default:0" json:"rate_limit"`            // requests per minute, create default = 60, 0 = unlimited
	RateLimitWindow   int        `gorm:"default:60" json:"rate_limit_window"`    // window in seconds
	Providers         string     `gorm:"type:text" json:"providers,omitempty"`   // JSON array of allowed provider names
	Status            string     `gorm:"size:20;default:'active'" json:"status"` // active, inactive, expired
	Metadata          string     `gorm:"type:text" json:"metadata,omitempty"`    // JSON for additional data
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name
func (VirtualKey) TableName() string {
	return "virtual_keys"
}

// VirtualKeyRequest is used for API requests
type VirtualKeyRequest struct {
	Name            string   `json:"name" binding:"required"`
	BudgetTotal     float64  `json:"budget_total"`
	RateLimit       *int     `json:"rate_limit"`
	RateLimitWindow int      `json:"rate_limit_window"`
	Providers       []string `json:"providers"`
	Metadata        string   `json:"metadata"`
}

// VirtualKeyResponse is used for API responses (excludes sensitive fields)
type VirtualKeyResponse struct {
	ID                uint       `json:"id"`
	TenantID          uint       `json:"tenant_id"`
	CreatedByUserID   *uint      `json:"created_by_user_id,omitempty"`
	CreatedByUsername string     `json:"created_by_username,omitempty"`
	Name              string     `json:"name"`
	Key               string     `json:"key,omitempty"`   // Only shown once on creation
	KeyHashPrefix     string     `json:"key_hash_prefix"` // First 8 chars for identification
	BudgetTotal       float64    `json:"budget_total"`
	BudgetUsed        float64    `json:"budget_used"`
	BudgetRemaining   float64    `json:"budget_remaining"`
	BudgetResetAt     *time.Time `json:"budget_reset_at,omitempty"`
	RateLimit         int        `json:"rate_limit"`
	RateLimitWindow   int        `json:"rate_limit_window"`
	Providers         string     `json:"providers,omitempty"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
