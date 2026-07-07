package service

import (
	"path/filepath"
	"testing"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
)

// setupTenantTestDB spins up a throwaway sqlite database with the tenant-aware
// schema migrated, and returns it via the global database.DB used by services.
func setupTenantTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "isolation.db")
	if err := database.Connect(&database.Config{Driver: "sqlite", DSN: dsn, LogLevel: "silent"}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := database.Migrate(
		&models.Tenant{},
		&models.User{},
		&models.TenantMember{},
		&models.VirtualKey{},
		&models.UsageRecord{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
}

// TestVirtualKeyTenantIsolation is the guardrail against the primary risk of
// this feature: a query that forgets tenant scoping would leak another
// tenant's virtual keys. Tenant 1 and tenant 2 must never see each other's
// keys, while a super_admin (tenantID 0) sees both.
func TestVirtualKeyTenantIsolation(t *testing.T) {
	setupTenantTestDB(t)
	svc := NewVirtualKeyService()

	if _, _, err := svc.Create(1, &models.VirtualKeyRequest{Name: "t1-key"}, nil, ""); err != nil {
		t.Fatalf("create t1 key: %v", err)
	}
	if _, _, err := svc.Create(2, &models.VirtualKeyRequest{Name: "t2-key"}, nil, ""); err != nil {
		t.Fatalf("create t2 key: %v", err)
	}

	t1, err := svc.List(1)
	if err != nil {
		t.Fatalf("list t1: %v", err)
	}
	if len(t1) != 1 || t1[0].Name != "t1-key" {
		t.Fatalf("tenant 1 should see exactly its own key, got %+v", t1)
	}

	t2, err := svc.List(2)
	if err != nil {
		t.Fatalf("list t2: %v", err)
	}
	if len(t2) != 1 || t2[0].Name != "t2-key" {
		t.Fatalf("tenant 2 should see exactly its own key, got %+v", t2)
	}

	// super_admin (tenantID 0) sees everything.
	all, err := svc.List(0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("super_admin should see both keys, got %d", len(all))
	}

	// Cross-tenant GetByID must fail: tenant 2 cannot fetch tenant 1's key.
	t1Key := t1[0]
	if _, err := svc.GetByID(2, t1Key.ID); err == nil {
		t.Fatal("tenant 2 must not be able to read tenant 1's key by ID")
	}
	// Same-tenant GetByID works.
	if _, err := svc.GetByID(1, t1Key.ID); err != nil {
		t.Fatalf("tenant 1 should read its own key: %v", err)
	}

	// Cross-tenant Delete must not delete another tenant's key.
	if err := svc.Delete(2, t1Key.ID); err == nil {
		t.Fatal("tenant 2 must not be able to delete tenant 1's key")
	}
	if _, err := svc.GetByID(1, t1Key.ID); err != nil {
		t.Fatalf("tenant 1's key should still exist after cross-tenant delete attempt: %v", err)
	}
}

func TestVirtualKeyCreatorIsolation(t *testing.T) {
	setupTenantTestDB(t)
	svc := NewVirtualKeyService()

	userOneID := uint(10)
	userTwoID := uint(11)
	if _, _, err := svc.Create(1, &models.VirtualKeyRequest{Name: "u1-key"}, &userOneID, "alice"); err != nil {
		t.Fatalf("create user one key: %v", err)
	}
	if _, _, err := svc.Create(1, &models.VirtualKeyRequest{Name: "u2-key"}, &userTwoID, "bob"); err != nil {
		t.Fatalf("create user two key: %v", err)
	}
	if _, _, err := svc.Create(1, &models.VirtualKeyRequest{Name: "historical-key"}, nil, ""); err != nil {
		t.Fatalf("create historical key: %v", err)
	}

	aliceKeys, err := svc.ListByCreator(1, userOneID)
	if err != nil {
		t.Fatalf("list alice keys: %v", err)
	}
	if len(aliceKeys) != 1 || aliceKeys[0].Name != "u1-key" {
		t.Fatalf("alice should see only her user_id-owned key, got %+v", aliceKeys)
	}

	bobKeys, err := svc.ListByCreator(1, userTwoID)
	if err != nil {
		t.Fatalf("list bob keys: %v", err)
	}
	if len(bobKeys) != 1 || bobKeys[0].Name != "u2-key" {
		t.Fatalf("bob should see only own key, got %+v", bobKeys)
	}

	if _, err := svc.GetByIDAndCreator(1, aliceKeys[0].ID, userTwoID); err == nil {
		t.Fatal("bob must not read alice's key by ID")
	}
}
