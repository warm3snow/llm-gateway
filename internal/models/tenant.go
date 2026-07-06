package models

import (
	"time"
)

// Tenant represents an isolated tenant (organization/team) in the system.
// All tenant-owned resources (virtual keys, usage records) are scoped by
// TenantID. Provider configurations remain global/shared across tenants.
type Tenant struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Name      string     `gorm:"size:100;not null" json:"name"`
	Slug      string     `gorm:"size:100;uniqueIndex;not null" json:"slug"`
	Status    string     `gorm:"size:20;default:'active'" json:"status"` // active, disabled
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name.
func (Tenant) TableName() string {
	return "tenants"
}

// TenantRequest is used for creating/updating a tenant.
type TenantRequest struct {
	Name string `json:"name" binding:"required"`
	Slug string `json:"slug" binding:"required"`
}
