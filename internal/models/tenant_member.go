package models

import "time"

// TenantMember links a global user account to a tenant with a tenant-scoped role.
type TenantMember struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	TenantID  uint       `gorm:"uniqueIndex:idx_tenant_member;index;not null" json:"tenant_id"`
	UserID    uint       `gorm:"uniqueIndex:idx_tenant_member;index;not null" json:"user_id"`
	Role      string     `gorm:"size:20;not null;default:'tenant_user'" json:"role"`
	Status    string     `gorm:"size:20;default:'active'" json:"status"`
	Tenant    Tenant     `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	User      User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (TenantMember) TableName() string {
	return "tenant_members"
}

// TenantMembership is returned to clients when a user can choose a tenant.
type TenantMembership struct {
	TenantID uint   `json:"tenant_id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}
