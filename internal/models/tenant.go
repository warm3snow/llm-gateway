package models

import (
	"encoding/json"
	"strings"
	"time"
)

// Tenant represents an isolated tenant (organization/team) in the system.
// All tenant-owned resources (virtual keys, usage records) are scoped by
// TenantID. Provider configurations remain global/shared across tenants.
type Tenant struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Name          string     `gorm:"size:100;not null" json:"name"`
	Slug          string     `gorm:"size:100;uniqueIndex;not null" json:"slug"`
	Status        string     `gorm:"size:20;default:'active'" json:"status"`             // active, disabled
	BudgetMonthly float64    `gorm:"type:decimal(12,2);default:0" json:"budget_monthly"` // USD per calendar month, 0 = unlimited
	BudgetUsed    float64    `gorm:"type:decimal(12,2);default:0" json:"budget_used"`    // month-to-date spend
	TpmLimit      int        `gorm:"default:0" json:"tpm_limit"`                         // 0 = unlimited
	RpmLimit      int        `gorm:"default:0" json:"rpm_limit"`                         // 0 = unlimited
	AllowedModels string     `gorm:"type:text" json:"allowed_models,omitempty"`          // JSON array; empty = all models
	AllowSelfKey  bool       `gorm:"default:false" json:"allow_self_key"`
	BudgetResetAt *time.Time `json:"budget_reset_at,omitempty"` // next monthly reset
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name.
func (Tenant) TableName() string {
	return "tenants"
}

// TenantRequest is used for creating/updating a tenant.
type TenantRequest struct {
	Name          string   `json:"name" binding:"required"`
	Slug          string   `json:"slug" binding:"required"`
	BudgetMonthly float64  `json:"budget_monthly"`
	TpmLimit      int      `json:"tpm_limit"`
	RpmLimit      int      `json:"rpm_limit"`
	AllowedModels []string `json:"allowed_models"`
	AllowSelfKey  bool     `json:"allow_self_key"`
}

// MarshalAllowedModels serializes the allowed-models whitelist to the JSON
// stored in the tenants.allowed_models column (empty slice -> "").
func MarshalAllowedModels(models []string) string {
	if len(models) == 0 {
		return ""
	}
	data, err := json.Marshal(models)
	if err != nil {
		return ""
	}
	return string(data)
}

// UnmarshalAllowedModels parses the stored whitelist; an empty value means
// all models are allowed.
func UnmarshalAllowedModels(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}
