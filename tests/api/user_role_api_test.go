package api

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

func loginToken(t *testing.T, router *gin.Engine, username, password string) string {
	t.Helper()
	w := testutil.DoJSON(t, router, http.MethodPost, "/api/v1/auth/login", gin.H{
		"username": username,
		"password": password,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Token string `json:"token"`
	}
	testutil.DecodeJSON(t, w, &res)
	require.NotEmpty(t, res.Token)
	return res.Token
}

func TestSuperAdminCanChangeTenantUserRole(t *testing.T) {
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	require.NoError(t, database.Bootstrap(cfg.Security.AdminUser, cfg.Security.AdminPass))

	tenant := testutil.CreateTenant(t, 2, "tenant-one")
	admin := testutil.CreateUser(t, &tenant.ID, "tenant-admin", "secret", models.RoleTenantAdmin)
	testutil.CreateTenantMember(t, admin.ID, tenant.ID, models.RoleTenantAdmin)
	member := testutil.CreateUser(t, &tenant.ID, "member", "secret", models.RoleTenantUser)
	testutil.CreateTenantMember(t, member.ID, tenant.ID, models.RoleTenantUser)

	router := newAPIRouter(t, cfg)
	superToken := loginToken(t, router, testutil.AdminEmail(cfg.Security.AdminUser), cfg.Security.AdminPass)
	// Issue the member's token before the promotion: the role change must
	// take effect without requiring a re-login.
	memberToken := loginToken(t, router, member.Email, "secret")

	req := testutil.NewJSONRequest(t, http.MethodPut, "/api/v1/users/"+itoa(member.ID)+"/role", gin.H{
		"role": models.RoleTenantAdmin,
	})
	testutil.Authorize(req, superToken)
	w := testutil.DoJSONReq(t, router, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var res struct {
		User struct {
			Role string `json:"role"`
		} `json:"user"`
	}
	testutil.DecodeJSON(t, w, &res)
	assert.Equal(t, models.RoleTenantAdmin, res.User.Role)

	var membership models.TenantMember
	require.NoError(t, database.GetDB().Where("user_id = ? AND tenant_id = ?", member.ID, tenant.ID).First(&membership).Error)
	assert.Equal(t, models.RoleTenantAdmin, membership.Role, "membership role should be updated")

	// The promoted user can now manage the tenant's resources.
	req = testutil.NewJSONRequest(t, http.MethodGet, "/api/v1/users", nil)
	testutil.Authorize(req, memberToken)
	w = testutil.DoJSONReq(t, router, req)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// tenant_admin must not change roles.
	req = testutil.NewJSONRequest(t, http.MethodPut, "/api/v1/users/"+itoa(admin.ID)+"/role", gin.H{
		"role": models.RoleTenantUser,
	})
	testutil.Authorize(req, memberToken)
	w = testutil.DoJSONReq(t, router, req)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestChangeRoleRejectsInvalidRole(t *testing.T) {
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	require.NoError(t, database.Bootstrap(cfg.Security.AdminUser, cfg.Security.AdminPass))

	tenant := testutil.CreateTenant(t, 2, "tenant-one")
	member := testutil.CreateUser(t, &tenant.ID, "member", "secret", models.RoleTenantUser)
	testutil.CreateTenantMember(t, member.ID, tenant.ID, models.RoleTenantUser)

	router := newAPIRouter(t, cfg)
	superToken := loginToken(t, router, testutil.AdminEmail(cfg.Security.AdminUser), cfg.Security.AdminPass)

	req := testutil.NewJSONRequest(t, http.MethodPut, "/api/v1/users/"+itoa(member.ID)+"/role", gin.H{
		"role": models.RoleSuperAdmin,
	})
	testutil.Authorize(req, superToken)
	w := testutil.DoJSONReq(t, router, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func itoa(v uint) string {
	return strconv.FormatUint(uint64(v), 10)
}
