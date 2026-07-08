package database

import (
	"path/filepath"
	"testing"

	"github.com/warm3snow/llm-gateway/internal/models"
)

func setupBootstrapTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "bootstrap.db")
	if err := Connect(&Config{Driver: "sqlite", DSN: dsn, LogLevel: "silent"}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := Migrate(
		&models.Tenant{},
		&models.User{},
		&models.TenantMember{},
		&models.VirtualKey{},
		&models.UsageRecord{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
}

func TestBootstrapSeedsDefaultTenantAndSuperAdmin(t *testing.T) {
	setupBootstrapTestDB(t)

	if err := Bootstrap("root", "rootpass"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	var tenant models.Tenant
	if err := DB.First(&tenant, DefaultTenantID).Error; err != nil {
		t.Fatalf("query default tenant: %v", err)
	}
	if tenant.Name != "Default" || tenant.Slug != "default" || tenant.Status != "active" {
		t.Fatalf("unexpected default tenant: %+v", tenant)
	}

	var admin models.User
	if err := DB.Where("username = ?", "root").First(&admin).Error; err != nil {
		t.Fatalf("query admin: %v", err)
	}
	if admin.TenantID != nil || admin.Role != models.RoleSuperAdmin || admin.Status != "active" {
		t.Fatalf("unexpected super admin: %+v", admin)
	}

	var members int64
	if err := DB.Model(&models.TenantMember{}).Count(&members).Error; err != nil {
		t.Fatalf("count tenant members: %v", err)
	}
	if members != 0 {
		t.Fatalf("bootstrap should not backfill tenant members, got %d", members)
	}
}
