package service

import (
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"gorm.io/gorm"
)

// AccessScope describes the authenticated caller's data visibility.
type AccessScope struct {
	TenantID uint
	Role     string
	UserID   *uint
}

func (s AccessScope) ApplyUsage(db *gorm.DB) *gorm.DB {
	db = db.Scopes(database.TenantScope(s.TenantID))
	if s.Role == models.RoleTenantUser {
		if s.UserID == nil {
			return db.Where("1 = 0")
		}
		db = db.Where("virtual_key_created_by_user_id = ?", *s.UserID)
	}
	return db
}

func (s AccessScope) ApplyVirtualKeys(db *gorm.DB) *gorm.DB {
	db = db.Scopes(database.TenantScope(s.TenantID))
	if s.Role == models.RoleTenantUser {
		if s.UserID == nil {
			return db.Where("1 = 0")
		}
		db = db.Where("created_by_user_id = ?", *s.UserID)
	}
	return db
}
