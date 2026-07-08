package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

func TestAuthAPILoginSingleTenantReturnsFinalToken(t *testing.T) {
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	tenant := testutil.CreateTenant(t, 1, "tenant-one")
	user := testutil.CreateUser(t, &tenant.ID, "tenant-admin", "secret", models.RoleTenantAdmin)
	testutil.CreateTenantMember(t, user.ID, tenant.ID, models.RoleTenantAdmin)
	router := newAPIRouter(t, cfg)

	w := testutil.DoJSON(t, router, http.MethodPost, "/api/v1/auth/login", gin.H{
		"username": "tenant-admin",
		"password": "secret",
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Status string `json:"status"`
		Token  string `json:"token"`
		Tenant struct {
			TenantID uint   `json:"tenant_id"`
			Role     string `json:"role"`
		} `json:"tenant"`
	}
	testutil.DecodeJSON(t, w, &res)
	assert.Equal(t, "success", res.Status)
	assert.NotEmpty(t, res.Token)
	assert.Equal(t, tenant.ID, res.Tenant.TenantID)
	assert.Equal(t, models.RoleTenantAdmin, res.Tenant.Role)
}

func TestAuthAPILoginMultipleTenantsRequiresSelection(t *testing.T) {
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	tenantOne := testutil.CreateTenant(t, 1, "tenant-one")
	tenantTwo := testutil.CreateTenant(t, 2, "tenant-two")
	user := testutil.CreateUser(t, &tenantOne.ID, "multi-user", "secret", models.RoleTenantAdmin)
	testutil.CreateTenantMember(t, user.ID, tenantOne.ID, models.RoleTenantAdmin)
	testutil.CreateTenantMember(t, user.ID, tenantTwo.ID, models.RoleTenantUser)
	router := newAPIRouter(t, cfg)

	w := testutil.DoJSON(t, router, http.MethodPost, "/api/v1/auth/login", gin.H{
		"username": "multi-user",
		"password": "secret",
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var loginRes struct {
		Status     string `json:"status"`
		LoginToken string `json:"login_token"`
		Tenants    []struct {
			TenantID uint   `json:"tenant_id"`
			Role     string `json:"role"`
		} `json:"tenants"`
	}
	testutil.DecodeJSON(t, w, &loginRes)
	assert.Equal(t, "tenant_selection_required", loginRes.Status)
	require.NotEmpty(t, loginRes.LoginToken)
	require.Len(t, loginRes.Tenants, 2)

	w = testutil.DoJSON(t, router, http.MethodPost, "/api/v1/auth/select-tenant", gin.H{
		"login_token": loginRes.LoginToken,
		"tenant_id":   tenantTwo.ID,
	})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var selectRes struct {
		Status string `json:"status"`
		Token  string `json:"token"`
		Tenant struct {
			TenantID uint   `json:"tenant_id"`
			Role     string `json:"role"`
		} `json:"tenant"`
	}
	testutil.DecodeJSON(t, w, &selectRes)
	assert.Equal(t, "success", selectRes.Status)
	assert.NotEmpty(t, selectRes.Token)
	assert.Equal(t, tenantTwo.ID, selectRes.Tenant.TenantID)
	assert.Equal(t, models.RoleTenantUser, selectRes.Tenant.Role)
}

func TestAuthAPIRejectsInvalidPassword(t *testing.T) {
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	tenant := testutil.CreateTenant(t, 1, "tenant-one")
	user := testutil.CreateUser(t, &tenant.ID, "tenant-admin", "secret", models.RoleTenantAdmin)
	testutil.CreateTenantMember(t, user.ID, tenant.ID, models.RoleTenantAdmin)
	router := newAPIRouter(t, cfg)

	w := testutil.DoJSON(t, router, http.MethodPost, "/api/v1/auth/login", gin.H{
		"username": "tenant-admin",
		"password": "wrong",
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}
