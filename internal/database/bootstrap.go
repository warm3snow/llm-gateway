package database

import (
	"log"

	"github.com/warm3snow/llm-gateway/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// DefaultTenantID is the initial tenant created for new installations.
const DefaultTenantID uint = 1

// Bootstrap seeds the data required for multi-tenancy to work:
//  1. a default tenant (id=1),
//  2. a super_admin user derived from the config admin credentials.
//
// It is idempotent and safe to call on every startup.
func Bootstrap(adminUser, adminPass string) error {
	if DB == nil {
		return nil
	}

	if adminUser == "" {
		adminUser = "admin"
	}
	if adminPass == "" {
		adminPass = "admin123"
	}

	// 1. Default tenant.
	var tenant models.Tenant
	if err := DB.First(&tenant, DefaultTenantID).Error; err != nil {
		tenant = models.Tenant{
			ID:     DefaultTenantID,
			Name:   "Default",
			Slug:   "default",
			Status: "active",
		}
		if err := DB.Create(&tenant).Error; err != nil {
			return err
		}
		log.Printf("[BOOTSTRAP] Created default tenant (id=%d)", DefaultTenantID)
	}

	// 2. Super-admin user from config credentials.
	var count int64
	if err := DB.Model(&models.User{}).Where("username = ?", adminUser).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		admin := models.User{
			TenantID:     nil, // super_admin is not bound to a tenant
			Username:     adminUser,
			Email:        models.DefaultUserEmail(adminUser, "platform"),
			PasswordHash: string(hash),
			Role:         models.RoleSuperAdmin,
			Status:       "active",
		}
		if err := DB.Create(&admin).Error; err != nil {
			return err
		}
		log.Printf("[BOOTSTRAP] Created super_admin user %q", adminUser)
	}

	return nil
}
