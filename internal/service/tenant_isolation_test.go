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

	if _, _, err := svc.Create(1, &models.VirtualKeyRequest{Name: "t1-key"}); err != nil {
		t.Fatalf("create t1 key: %v", err)
	}
	if _, _, err := svc.Create(2, &models.VirtualKeyRequest{Name: "t2-key"}); err != nil {
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
