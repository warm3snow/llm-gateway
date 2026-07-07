package service

import (
	"testing"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
)

func seedTenantUserTestData(t *testing.T) {
	t.Helper()
	db := database.GetDB()
	if err := db.Create(&models.Tenant{ID: 1, Name: "Default", Slug: "default", Status: "active"}).Error; err != nil {
		t.Fatalf("create tenant 1: %v", err)
	}
	if err := db.Create(&models.Tenant{ID: 2, Name: "Acme", Slug: "acme", Status: "active"}).Error; err != nil {
		t.Fatalf("create tenant 2: %v", err)
	}
}

func TestUserServiceListScopesByRole(t *testing.T) {
	setupTenantTestDB(t)
	seedTenantUserTestData(t)
	svc := NewUserService()

	if _, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "t1-admin", Password: "secret", TenantID: 1, Role: models.RoleTenantAdmin}); err != nil {
		t.Fatalf("create t1 admin: %v", err)
	}
	if _, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "t2-user", Password: "secret", TenantID: 2, Role: models.RoleTenantUser}); err != nil {
		t.Fatalf("create t2 user: %v", err)
	}

	all, err := svc.List(models.RoleSuperAdmin, 0, 0)
	if err != nil {
		t.Fatalf("super list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("super_admin should list all users, got %d", len(all))
	}

	t1, err := svc.List(models.RoleTenantAdmin, 1, 0)
	if err != nil {
		t.Fatalf("tenant admin list: %v", err)
	}
	if len(t1) != 1 || t1[0].Username != "t1-admin" {
		t.Fatalf("tenant_admin should only list own tenant users, got %+v", t1)
	}

	if _, err := svc.List(models.RoleTenantUser, 1, 0); err == nil {
		t.Fatal("tenant_user must not list users")
	}
}

func TestUserServiceCreateRoleRules(t *testing.T) {
	setupTenantTestDB(t)
	seedTenantUserTestData(t)
	svc := NewUserService()

	if _, err := svc.Create(models.RoleTenantAdmin, 1, &models.UserRequest{Username: "member", Password: "secret", TenantID: 2, Role: models.RoleTenantUser}); err != nil {
		t.Fatalf("tenant_admin should create tenant_user in own tenant: %v", err)
	}
	users, err := svc.List(models.RoleSuperAdmin, 0, 1)
	if err != nil {
		t.Fatalf("super list tenant 1: %v", err)
	}
	if len(users) != 1 || users[0].TenantID == nil || *users[0].TenantID != 1 || users[0].Role != models.RoleTenantUser {
		t.Fatalf("tenant_admin create should force own tenant tenant_user, got %+v", users)
	}

	if _, err := svc.Create(models.RoleTenantAdmin, 1, &models.UserRequest{Username: "admin2", Password: "secret", Role: models.RoleTenantAdmin}); err == nil {
		t.Fatal("tenant_admin must not create tenant_admin")
	}

	if _, err := svc.Create(models.RoleTenantUser, 1, &models.UserRequest{Username: "blocked", Password: "secret", Role: models.RoleTenantUser}); err == nil {
		t.Fatal("tenant_user must not create users")
	}
}

func TestUserServiceSetStatusRules(t *testing.T) {
	setupTenantTestDB(t)
	seedTenantUserTestData(t)
	svc := NewUserService()

	member, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "member", Password: "secret", TenantID: 1, Role: models.RoleTenantUser})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	admin, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "tenant-admin", Password: "secret", TenantID: 1, Role: models.RoleTenantAdmin})
	if err != nil {
		t.Fatalf("create tenant admin: %v", err)
	}
	other, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "other-member", Password: "secret", TenantID: 2, Role: models.RoleTenantUser})
	if err != nil {
		t.Fatalf("create other member: %v", err)
	}

	if err := svc.SetStatus(models.RoleTenantAdmin, 1, "tenant-admin", member.ID, "disabled"); err != nil {
		t.Fatalf("tenant_admin should disable own tenant_user: %v", err)
	}
	if err := svc.SetStatus(models.RoleTenantAdmin, 1, "tenant-admin", admin.ID, "disabled"); err == nil {
		t.Fatal("tenant_admin must not disable tenant_admin")
	}
	if err := svc.SetStatus(models.RoleTenantAdmin, 1, "tenant-admin", other.ID, "disabled"); err == nil {
		t.Fatal("tenant_admin must not disable users from another tenant")
	}
	if err := svc.SetStatus(models.RoleTenantUser, 1, "member", member.ID, "active"); err == nil {
		t.Fatal("tenant_user must not update user status")
	}
}
