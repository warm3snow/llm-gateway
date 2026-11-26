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
//
// BudgetMonthly is the per-user monthly quota (USD) assigned by the tenant
// admin; the sum of member quotas should stay within the tenant's monthly
// budget. BudgetUsed is the month-to-date spend, reset each calendar month.
type User struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	TenantID      *uint      `gorm:"uniqueIndex:idx_tenant_username" json:"tenant_id,omitempty"` // nil for super_admin
	Username      string     `gorm:"size:100;uniqueIndex:idx_tenant_username;not null" json:"username"`
	Email         string     `gorm:"size:255;index" json:"email,omitempty"`
	PasswordHash  string     `gorm:"size:100;not null" json:"-"`
	Role          string     `gorm:"size:20;not null;default:'tenant_admin'" json:"role"`
	Status        string     `gorm:"size:20;default:'active'" json:"status"`             // active, disabled
	BudgetMonthly float64    `gorm:"type:decimal(12,2);default:0" json:"budget_monthly"` // 0 = unlimited
	BudgetUsed    float64    `gorm:"type:decimal(12,2);default:0" json:"budget_used"`
	RpmLimit      int        `gorm:"default:0" json:"rpm_limit"` // 0 = unlimited
	BudgetResetAt *time.Time `json:"budget_reset_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// TableName specifies the table name.
func (User) TableName() string {
	return "users"
}

// EmailDomain is the domain used for generated account emails.
const EmailDomain = "llmgw.org"

// DefaultUserEmail returns the tenant-scoped, globally unique email for a
// user created without an explicit address: name@<tenant-slug>.llmgw.org.
func DefaultUserEmail(username, tenantSlug string) string {
	slug := strings.ToLower(strings.TrimSpace(tenantSlug))
	return strings.ToLower(strings.TrimSpace(username)) + "@" + slug + "." + EmailDomain
}

// NormalizeEmail lowercases and trims an email address.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidEmail reports whether email looks like a usable address. Login and
// account creation are email-based, so the check is applied server-side.
func ValidEmail(email string) bool {
	email = NormalizeEmail(email)
	if len(email) < 5 || len(email) > 255 {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	local, domain := email[:at], email[at+1:]
	if strings.ContainsAny(local, " @") || !strings.Contains(domain, ".") {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return false
	}
	return true
}

// EmailLocalPart returns the part before "@" (the display username).
func EmailLocalPart(email string) string {
	email = NormalizeEmail(email)
	if at := strings.LastIndex(email, "@"); at > 0 {
		return email[:at]
	}
	return email
}

// UserRequest is used for creating a tenant user. Email is the login identity
// and must be globally unique; Username is optional and defaults to the email's
// local part.
type UserRequest struct {
	Username      string  `json:"username"`
	Email         string  `json:"email" binding:"required"`
	Password      string  `json:"password" binding:"required"`
	TenantID      uint    `json:"tenant_id" binding:"required"`
	Role          string  `json:"role"` // defaults to tenant_admin
	BudgetMonthly float64 `json:"budget_monthly"`
	RpmLimit      int     `json:"rpm_limit"`
}

// UserQuotaRequest updates a user's monthly quota.
type UserQuotaRequest struct {
	BudgetMonthly *float64 `json:"budget_monthly"`
	RpmLimit      *int     `json:"rpm_limit"`
}

// UserRoleRequest changes a tenant member's role. TenantID is optional and
// selects which membership to update when a user belongs to several tenants.
type UserRoleRequest struct {
	Role     string `json:"role" binding:"required"`
	TenantID uint   `json:"tenant_id"`
}
