package testutil

import (
	"path/filepath"
	"testing"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func SetupSQLiteDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "llm-gateway-test.db")
	if err := database.Connect(&database.Config{Driver: "sqlite", DSN: dsn, LogLevel: "silent"}); err != nil {
		t.Fatalf("connect sqlite: %v", err)
	}
	if err := database.Migrate(
		&models.Tenant{},
		&models.User{},
		&models.TenantMember{},
		&models.VirtualKey{},
		&models.UsageRecord{},
		&models.IdempotencyRecord{},
		&models.AlertRule{},
		&models.AlertEvent{},
		&models.ProviderConfig{},
		&models.CacheEntry{},
		&models.ModelPricing{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
		database.DB = nil
	})
	return database.GetDB()
}

func CreateTenant(t *testing.T, id uint, name string) models.Tenant {
	t.Helper()
	tenant := models.Tenant{ID: id, Name: name, Slug: name, Status: "active"}
	if err := database.GetDB().Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return tenant
}

func CreateUser(t *testing.T, tenantID *uint, username, password, role string) models.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := models.User{
		TenantID:     tenantID,
		Username:     username,
		Email:        username + "@test.llmgw",
		PasswordHash: string(hash),
		Role:         role,
		Status:       "active",
	}
	if err := database.GetDB().Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func CreateTenantMember(t *testing.T, userID, tenantID uint, role string) models.TenantMember {
	t.Helper()
	member := models.TenantMember{UserID: userID, TenantID: tenantID, Role: role, Status: "active"}
	if err := database.GetDB().Create(&member).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	return member
}
