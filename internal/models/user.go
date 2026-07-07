package models

import (
	"strings"
	"time"
)

// User roles.
const (
	RoleSuperAdmin  = "super_admin"  // platform-level, manages all tenants
	RoleTenantAdmin = "tenant_admin" // manages a single tenant's resources
	RoleTenantUser  = "tenant_user"  // read-only user within a tenant
)

// User represents an admin-console user. A super_admin has TenantID == nil
// and can manage across all tenants; a tenant_admin belongs to exactly one
// tenant and only sees that tenant's resources.
type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TenantID     *uint      `gorm:"uniqueIndex:idx_tenant_username" json:"tenant_id,omitempty"` // nil for super_admin
	Username     string     `gorm:"size:100;uniqueIndex:idx_tenant_username;not null" json:"username"`
	Email        string     `gorm:"size:255;index" json:"email,omitempty"`
	PasswordHash string     `gorm:"size:100;not null" json:"-"`
	Role         string     `gorm:"size:20;not null;default:'tenant_admin'" json:"role"`
	Status       string     `gorm:"size:20;default:'active'" json:"status"` // active, disabled
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name.
func (User) TableName() string {
	return "users"
}

// DefaultUserEmail returns the default globally unique email for a tenant-created user.
func DefaultUserEmail(username, tenantSlug string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "@" + strings.ToLower(strings.TrimSpace(tenantSlug)) + ".llmgw"
}

// UserRequest is used for creating a tenant user.
type UserRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email"`
	Password string `json:"password" binding:"required"`
	TenantID uint   `json:"tenant_id" binding:"required"`
	Role     string `json:"role"` // defaults to tenant_admin
}
