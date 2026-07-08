package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

func doAuthorizedJSON(t *testing.T, router *gin.Engine, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewJSONRequest(t, method, path, body)
	testutil.Authorize(req, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestVirtualKeysAPICreateAndList(t *testing.T) {
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	tenant := testutil.CreateTenant(t, 1, "tenant-one")
	user := testutil.CreateUser(t, &tenant.ID, "tenant-admin", "secret", models.RoleTenantAdmin)
	testutil.CreateTenantMember(t, user.ID, tenant.ID, models.RoleTenantAdmin)
	router := newAPIRouter(t, cfg)
	token := testutil.SignJWT(t, cfg, testutil.TokenClaims{UserID: user.ID, Username: user.Username, Role: models.RoleTenantAdmin, TenantID: tenant.ID})

	w := doAuthorizedJSON(t, router, http.MethodPost, "/api/v1/virtual-keys", gin.H{
		"name":         "api-key",
		"budget_total": 25,
		"providers":    []string{"openai"},
	}, token)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var createRes struct {
		Status string `json:"status"`
		Key    struct {
			ID          uint    `json:"id"`
			TenantID    uint    `json:"tenant_id"`
			Name        string  `json:"name"`
			Key         string  `json:"key"`
			BudgetTotal float64 `json:"budget_total"`
		} `json:"key"`
	}
	testutil.DecodeJSON(t, w, &createRes)
	assert.Equal(t, "success", createRes.Status)
	assert.NotZero(t, createRes.Key.ID)
	assert.Equal(t, tenant.ID, createRes.Key.TenantID)
	assert.Equal(t, "api-key", createRes.Key.Name)
	assert.NotEmpty(t, createRes.Key.Key)
	assert.Equal(t, float64(25), createRes.Key.BudgetTotal)

	w = doAuthorizedJSON(t, router, http.MethodGet, "/api/v1/virtual-keys", nil, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var listRes struct {
		Status      string `json:"status"`
		VirtualKeys []struct {
			ID       uint   `json:"id"`
			TenantID uint   `json:"tenant_id"`
			Name     string `json:"name"`
			Key      string `json:"key"`
		} `json:"virtual_keys"`
	}
	testutil.DecodeJSON(t, w, &listRes)
	assert.Equal(t, "success", listRes.Status)
	require.Len(t, listRes.VirtualKeys, 1)
	assert.Equal(t, createRes.Key.ID, listRes.VirtualKeys[0].ID)
	assert.Empty(t, listRes.VirtualKeys[0].Key)
}

func TestVirtualKeysAPIRequiresJWT(t *testing.T) {
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	testutil.CreateTenant(t, 1, "tenant-one")
	router := newAPIRouter(t, cfg)

	w := testutil.DoJSON(t, router, http.MethodGet, "/api/v1/virtual-keys", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}

func TestVirtualKeysAPITenantUserCannotUpdate(t *testing.T) {
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	tenant := testutil.CreateTenant(t, 1, "tenant-one")
	user := testutil.CreateUser(t, &tenant.ID, "tenant-user", "secret", models.RoleTenantUser)
	testutil.CreateTenantMember(t, user.ID, tenant.ID, models.RoleTenantUser)
	router := newAPIRouter(t, cfg)
	token := testutil.SignJWT(t, cfg, testutil.TokenClaims{UserID: user.ID, Username: user.Username, Role: models.RoleTenantUser, TenantID: tenant.ID})

	w := doAuthorizedJSON(t, router, http.MethodPost, "/api/v1/virtual-keys", gin.H{"name": "own-key"}, token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var createRes struct {
		Key struct {
			ID uint `json:"id"`
		} `json:"key"`
	}
	testutil.DecodeJSON(t, w, &createRes)

	w = doAuthorizedJSON(t, router, http.MethodPut, "/api/v1/virtual-keys/"+strconv.FormatUint(uint64(createRes.Key.ID), 10), gin.H{"name": "renamed"}, token)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}
