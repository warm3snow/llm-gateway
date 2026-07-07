package database

import (
	"path/filepath"
	"testing"

	"github.com/warm3snow/llm-gateway/internal/models"
	"golang.org/x/crypto/bcrypt"
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

func TestBootstrapBackfillsTenantMembersWithoutMergingDuplicateUsers(t *testing.T) {
	setupBootstrapTestDB(t)

	if err := DB.Create(&models.Tenant{ID: 1, Name: "Tenant One", Slug: "tenant-one", Status: "active"}).Error; err != nil {
		t.Fatalf("create tenant 1: %v", err)
	}
	if err := DB.Create(&models.Tenant{ID: 2, Name: "Tenant Two", Slug: "tenant-two", Status: "active"}).Error; err != nil {
		t.Fatalf("create tenant 2: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	tenantOneID := uint(1)
	tenantTwoID := uint(2)
	first := models.User{TenantID: &tenantOneID, Username: "user1", PasswordHash: string(hash), Role: models.RoleTenantAdmin, Status: "active"}
	duplicate := models.User{TenantID: &tenantTwoID, Username: "user1", PasswordHash: string(hash), Role: models.RoleTenantUser, Status: "active"}
	if err := DB.Omit("Email").Create(&first).Error; err != nil {
		t.Fatalf("create first user: %v", err)
	}
	if err := DB.Omit("Email").Create(&duplicate).Error; err != nil {
		t.Fatalf("create duplicate user: %v", err)
	}
	if err := DB.Create(&models.VirtualKey{TenantID: 2, CreatedByUserID: &duplicate.ID, CreatedByUsername: "user1", Name: "dup-key", KeyHash: "hash", KeySalt: "salt", HashedKey: "hashed", Status: "active"}).Error; err != nil {
		t.Fatalf("create virtual key: %v", err)
	}

	if err := Bootstrap("root", "rootpass"); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	var users []models.User
	if err := DB.Where("username = ?", "user1").Order("id ASC").Find(&users).Error; err != nil {
		t.Fatalf("query users: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected duplicate usernames to remain separate users, got %+v", users)
	}
	if users[0].Email != "user1@tenant-one.llmgw" || users[1].Email != "user1@tenant-two.llmgw" {
		t.Fatalf("expected default tenant-scoped emails, got %+v", users)
	}

	var members []models.TenantMember
	if err := DB.Order("tenant_id ASC").Find(&members).Error; err != nil {
		t.Fatalf("query members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected two tenant memberships, got %+v", members)
	}
	if members[0].TenantID != 1 || members[0].UserID != first.ID || members[0].Role != models.RoleTenantAdmin {
		t.Fatalf("unexpected tenant one membership: %+v", members[0])
	}
	if members[1].TenantID != 2 || members[1].UserID != duplicate.ID || members[1].Role != models.RoleTenantUser {
		t.Fatalf("unexpected tenant two membership: %+v", members[1])
	}

	var key models.VirtualKey
	if err := DB.Where("name = ?", "dup-key").First(&key).Error; err != nil {
		t.Fatalf("query virtual key: %v", err)
	}
	if key.CreatedByUserID == nil || *key.CreatedByUserID != duplicate.ID {
		t.Fatalf("expected virtual key creator to keep duplicate user id, got %+v", key.CreatedByUserID)
	}
}
