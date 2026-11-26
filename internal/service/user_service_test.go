package service

import (
	"testing"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"golang.org/x/crypto/bcrypt"
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

func TestUserServiceSetRoleRules(t *testing.T) {
	setupTenantTestDB(t)
	seedTenantUserTestData(t)
	svc := NewUserService()

	admin, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "t1-admin", Password: "secret", TenantID: 1, Role: models.RoleTenantAdmin})
	if err != nil {
		t.Fatalf("create tenant admin: %v", err)
	}
	member, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "t1-member", Password: "secret", TenantID: 1, Role: models.RoleTenantUser})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	otherAdmin, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "t2-admin", Password: "secret", TenantID: 2, Role: models.RoleTenantAdmin})
	if err != nil {
		t.Fatalf("create second tenant admin: %v", err)
	}

	if _, err := svc.SetRole(models.RoleTenantAdmin, admin.ID, member.ID, &models.UserRoleRequest{Role: models.RoleTenantAdmin}); err == nil {
		t.Fatal("tenant_admin must not change roles")
	}
	if _, err := svc.SetRole(models.RoleSuperAdmin, admin.ID, member.ID, &models.UserRoleRequest{Role: "super_admin"}); err == nil {
		t.Fatal("super_admin role must not be assignable")
	}
	if _, err := svc.SetRole(models.RoleSuperAdmin, admin.ID, admin.ID, &models.UserRoleRequest{Role: models.RoleTenantUser}); err == nil {
		t.Fatal("must not change own role")
	}

	updated, err := svc.SetRole(models.RoleSuperAdmin, 999, member.ID, &models.UserRoleRequest{Role: models.RoleTenantAdmin})
	if err != nil {
		t.Fatalf("promote tenant_user to tenant_admin: %v", err)
	}
	if updated.Role != models.RoleTenantAdmin {
		t.Fatalf("expected user role tenant_admin, got %q", updated.Role)
	}

	// Demoting back is allowed while another active tenant_admin remains.
	if _, err := svc.SetRole(models.RoleSuperAdmin, 999, member.ID, &models.UserRoleRequest{Role: models.RoleTenantUser}); err != nil {
		t.Fatalf("demote when another tenant_admin exists: %v", err)
	}

	// The only remaining tenant_admin cannot be demoted.
	if _, err := svc.SetRole(models.RoleSuperAdmin, 999, otherAdmin.ID, &models.UserRoleRequest{Role: models.RoleTenantUser}); err == nil {
		t.Fatal("must not demote the last active tenant_admin")
	}

	// Tenant hint selects the membership for multi-tenant users.
	if _, err := svc.SetRole(models.RoleSuperAdmin, 999, member.ID, &models.UserRoleRequest{Role: models.RoleTenantUser, TenantID: 2}); err == nil {
		t.Fatal("user is not a member of tenant 2")
	}

	var membership models.TenantMember
	if err := database.GetDB().Where("user_id = ? AND tenant_id = ?", member.ID, 1).First(&membership).Error; err != nil {
		t.Fatalf("load membership: %v", err)
	}
	if membership.Role != models.RoleTenantUser {
		t.Fatalf("expected membership role tenant_user, got %q", membership.Role)
	}
}

func TestUserServiceChangeOwnPassword(t *testing.T) {
	setupTenantTestDB(t)
	seedTenantUserTestData(t)
	svc := NewUserService()

	user, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "member", Password: "old-secret", TenantID: 1, Role: models.RoleTenantUser})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := svc.ChangeOwnPassword(user.ID, "wrong-secret", "new-secret"); err == nil {
		t.Fatal("wrong current password must fail")
	}
	if err := svc.ChangeOwnPassword(user.ID, "old-secret", "new-secret"); err != nil {
		t.Fatalf("change password: %v", err)
	}

	var updated models.User
	if err := database.GetDB().First(&updated, user.ID).Error; err != nil {
		t.Fatalf("load updated user: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("new-secret")) != nil {
		t.Fatal("new password hash does not match")
	}
}

func TestUserServiceCreatesIndependentUsersForSameUsernameAcrossTenants(t *testing.T) {
	setupTenantTestDB(t)
	seedTenantUserTestData(t)
	svc := NewUserService()

	first, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "user1", Password: "secret", TenantID: 1, Role: models.RoleTenantAdmin})
	if err != nil {
		t.Fatalf("create tenant 1 member: %v", err)
	}
	second, err := svc.Create(models.RoleSuperAdmin, 0, &models.UserRequest{Username: "user1", Password: "ignored", TenantID: 2, Role: models.RoleTenantUser})
	if err != nil {
		t.Fatalf("create tenant 2 member: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("expected separate users across tenants, got same id %d", first.ID)
	}
	if first.Email != "user1@default.llmgw.org" || second.Email != "user1@acme.llmgw.org" {
		t.Fatalf("expected tenant-scoped default emails, got %q and %q", first.Email, second.Email)
	}

	all, err := svc.List(models.RoleSuperAdmin, 0, 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected two tenant users in list, got %+v", all)
	}
}
