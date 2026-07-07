package database

import (
	"log"

	"github.com/warm3snow/llm-gateway/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// DefaultTenantID is the tenant that legacy (pre-multi-tenant) resources are
// assigned to during migration.
const DefaultTenantID uint = 1

// Bootstrap seeds the data required for multi-tenancy to work:
//  1. a default tenant (id=1) that owns all pre-existing resources,
//  2. backfill of any virtual_keys / usage_records that predate the
//     tenant_id column (tenant_id = 0) onto the default tenant,
//  3. a super_admin user derived from the config admin credentials.
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

	// 2. Backfill legacy rows with tenant_id = 0.
	if err := DB.Model(&models.VirtualKey{}).Where("tenant_id = ?", 0).
		Update("tenant_id", DefaultTenantID).Error; err != nil {
		return err
	}
	if err := DB.Model(&models.UsageRecord{}).Where("tenant_id = ?", 0).
		Update("tenant_id", DefaultTenantID).Error; err != nil {
		return err
	}

	// 3. Super-admin user from config credentials.
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
			PasswordHash: string(hash),
			Role:         models.RoleSuperAdmin,
			Status:       "active",
		}
		if err := DB.Create(&admin).Error; err != nil {
			return err
		}
		log.Printf("[BOOTSTRAP] Created super_admin user %q", adminUser)
	}

	return backfillTenantMembers()
}

func backfillTenantMembers() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var users []models.User
		if err := tx.Where("tenant_id IS NOT NULL").Order("id ASC").Find(&users).Error; err != nil {
			return err
		}

		canonicalByUsername := make(map[string]models.User)
		for _, user := range users {
			if user.TenantID == nil || user.Role == models.RoleSuperAdmin {
				continue
			}

			canonical, exists := canonicalByUsername[user.Username]
			if !exists {
				canonical = user
				canonicalByUsername[user.Username] = user
			}

			role := user.Role
			if role == "" {
				role = models.RoleTenantUser
			}
			status := user.Status
			if status == "" {
				status = "active"
			}

			member := models.TenantMember{
				TenantID: *user.TenantID,
				UserID:   canonical.ID,
			}
			if err := tx.Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).
				Assign(models.TenantMember{Role: role, Status: status}).
				FirstOrCreate(&member).Error; err != nil {
				return err
			}

			if user.ID == canonical.ID {
				continue
			}

			if err := tx.Model(&models.VirtualKey{}).
				Where("created_by_user_id = ?", user.ID).
				Update("created_by_user_id", canonical.ID).Error; err != nil {
				return err
			}
			if err := tx.Delete(&models.User{}, user.ID).Error; err != nil {
				return err
			}
		}

		return nil
	})
}
