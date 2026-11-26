//go:build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

// createTenantWithBudget creates a tenant with a monthly budget via the admin API.
func createTenantWithBudget(t *testing.T, router *gin.Engine, token, name, slug string, budget float64) uint {
	t.Helper()
	req := testutil.NewJSONRequest(t, http.MethodPost, "/api/v1/tenants", gin.H{
		"name":           name,
		"slug":           slug,
		"budget_monthly": budget,
	})
	testutil.Authorize(req, token)
	w := testutil.DoJSONReq(t, router, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Tenant struct {
			ID uint `json:"id"`
		} `json:"tenant"`
	}
	testutil.DecodeJSON(t, w, &res)
	return res.Tenant.ID
}

// userEmail builds the unique login email used by these tests.
func userEmail(username string) string {
	return username + "@" + models.EmailDomain
}

// createUserInTenant creates a tenant_user with a monthly quota.
func createUserInTenant(t *testing.T, router *gin.Engine, token string, tenantID uint, username string, budget float64) uint {
	t.Helper()
	req := testutil.NewJSONRequest(t, http.MethodPost, "/api/v1/users", gin.H{
		"email":          userEmail(username),
		"password":       "password123",
		"role":           "tenant_user",
		"tenant_id":      tenantID,
		"budget_monthly": budget,
	})
	testutil.Authorize(req, token)
	w := testutil.DoJSONReq(t, router, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		User struct {
			ID uint `json:"id"`
		} `json:"user"`
	}
	testutil.DecodeJSON(t, w, &res)
	return res.User.ID
}

// loginUser logs in as a tenant user and returns a JWT.
func loginUser(t *testing.T, router *gin.Engine, username, password string) string {
	t.Helper()
	login := testutil.DoJSON(t, router, http.MethodPost, "/api/v1/auth/login", gin.H{
		"username": username,
		"password": password,
	})
	require.Equal(t, http.StatusOK, login.Code, login.Body.String())
	var res struct {
		Token string `json:"token"`
	}
	testutil.DecodeJSON(t, login, &res)
	require.NotEmpty(t, res.Token)
	return res.Token
}

// keyForUser issues a virtual key owned by the given user via the admin API.
// super_admin creates keys into a specific tenant via ?tenant_id=.
func keyForUser(t *testing.T, router *gin.Engine, token, name string, budget float64, tenantID, ownerID uint) string {
	t.Helper()
	path := "/api/v1/virtual-keys"
	if tenantID > 0 {
		path = fmt.Sprintf("%s?tenant_id=%d", path, tenantID)
	}
	req := testutil.NewJSONRequest(t, http.MethodPost, path, gin.H{
		"name":         name,
		"budget_total": budget,
		"user_id":      ownerID,
	})
	testutil.Authorize(req, token)
	w := testutil.DoJSONReq(t, router, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Key struct {
			Key string `json:"key"`
		} `json:"key"`
	}
	testutil.DecodeJSON(t, w, &res)
	require.NotEmpty(t, res.Key.Key)
	return res.Key.Key
}

func TestUserMonthlyQuotaBlocksRequests(t *testing.T) {
	upstream := newMockChatUpstream(t, 2, 0) // cost 0.0002 per call at 100 cents/token
	cfg, router := setupGateway(t, upstream.URL(), nil)
	seedModelPricing(t, "openai", "gpt-test", 100, 0, 0)
	adminToken := loginAdmin(t, router, cfg)

	tenantID := createTenantWithBudget(t, router, adminToken, "quota-tenant", "quota-tenant", 1000)
	userID := createUserInTenant(t, router, adminToken, tenantID, "quota-user", 2) // exactly one call (cost 2)
	token := loginUser(t, router, userEmail("quota-user"), "password123")
	_ = token
	key := keyForUser(t, router, adminToken, "quota-key", 0, tenantID, userID)

	first := doChatCompletion(t, router, cfg, key, nil)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	var user models.User
	require.NoError(t, database.GetDB().Where("username = ?", "quota-user").First(&user).Error)
	assert.InDelta(t, 2.0, user.BudgetUsed, 1e-9, "user budget_used should track spend")

	// Second call crosses the user's monthly quota (used 2 >= quota 2).
	second := doChatCompletion(t, router, cfg, key, nil)
	assert.Equal(t, http.StatusPaymentRequired, second.Code, second.Body.String())
	assert.Contains(t, second.Body.String(), "quota exceeded")
	assert.Equal(t, int64(1), upstream.Calls(), "second call must not reach upstream")
}

func TestTenantMonthlyBudgetBlocksRequests(t *testing.T) {
	upstream := newMockChatUpstream(t, 3, 0) // cost 0.0003 per call
	cfg, router := setupGateway(t, upstream.URL(), nil)
	seedModelPricing(t, "openai", "gpt-test", 100, 0, 0)
	adminToken := loginAdmin(t, router, cfg)

	tenantID := createTenantWithBudget(t, router, adminToken, "tiny-tenant", "tiny-tenant", 3)
	// Unlimited user quota so the tenant budget is the binding constraint.
	userID := createUserInTenant(t, router, adminToken, tenantID, "tiny-user", 0)
	key := keyForUser(t, router, adminToken, "tiny-key", 0, tenantID, userID)

	first := doChatCompletion(t, router, cfg, key, nil)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	var tenant models.Tenant
	require.NoError(t, database.GetDB().First(&tenant, tenantID).Error)
	assert.InDelta(t, 3.0, tenant.BudgetUsed, 1e-9, "tenant budget_used should track spend")

	second := doChatCompletion(t, router, cfg, key, nil)
	assert.Equal(t, http.StatusPaymentRequired, second.Code, second.Body.String())
	assert.Contains(t, second.Body.String(), "budget exceeded")
	assert.Equal(t, int64(1), upstream.Calls())
}

func TestTenantModelWhitelistRejectsModel(t *testing.T) {
	upstream := newMockChatUpstream(t, 1, 0)
	cfg, router := setupGateway(t, upstream.URL(), nil)
	seedModelPricing(t, "openai", "gpt-test", 100, 0, 0)
	adminToken := loginAdmin(t, router, cfg)

	// Tenant allows only "allowed-model".
	req := testutil.NewJSONRequest(t, http.MethodPost, "/api/v1/tenants", gin.H{
		"name":           "wl-tenant",
		"slug":           "wl-tenant",
		"allowed_models": []string{"allowed-model"},
	})
	testutil.Authorize(req, adminToken)
	w := testutil.DoJSONReq(t, router, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	tenantID := uint(0)
	var tenant models.Tenant
	require.NoError(t, database.GetDB().Where("slug = ?", "wl-tenant").First(&tenant).Error)
	tenantID = tenant.ID

	userID := createUserInTenant(t, router, adminToken, tenantID, "wl-user", 0)
	key := keyForUser(t, router, adminToken, "wl-key", 0, tenantID, userID)

	resp := doChatCompletion(t, router, cfg, key, nil)
	assert.Equal(t, http.StatusForbidden, resp.Code, resp.Body.String())
	assert.Contains(t, resp.Body.String(), "not allowed")
	assert.Equal(t, int64(0), upstream.Calls())
}

func TestMemberQuotaExceedingTenantBudgetRejected(t *testing.T) {
	cfg, router := setupGateway(t, "http://127.0.0.1:1", nil)
	adminToken := loginAdmin(t, router, cfg)

	tenantID := createTenantWithBudget(t, router, adminToken, "sum-tenant", "sum-tenant", 10)
	// First member takes 8 of the 10 budget.
	createUserInTenant(t, router, adminToken, tenantID, "sum-user-a", 8)

	// Second member wants 5 -> total 13 > 10; must be rejected.
	req := testutil.NewJSONRequest(t, http.MethodPost, "/api/v1/users", gin.H{
		"email":          userEmail("sum-user-b"),
		"password":       "password123",
		"role":           "tenant_user",
		"tenant_id":      tenantID,
		"budget_monthly": 5,
	})
	testutil.Authorize(req, adminToken)
	w := testutil.DoJSONReq(t, router, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "exceeds tenant monthly budget")
}

func TestMeUsageAndSelfServiceKeys(t *testing.T) {
	upstream := newMockChatUpstream(t, 1, 0)
	cfg, router := setupGateway(t, upstream.URL(), nil)
	seedModelPricing(t, "openai", "gpt-test", 100, 0, 0)
	adminToken := loginAdmin(t, router, cfg)

	// Enable self-service keys on the default tenant.
	var defTenant models.Tenant
	require.NoError(t, database.GetDB().First(&defTenant, database.DefaultTenantID).Error)
	require.NoError(t, database.GetDB().Model(&models.Tenant{}).Where("id = ?", defTenant.ID).
		Update("allow_self_key", true).Error)

	// Create a tenant_user on the default tenant.
	req := testutil.NewJSONRequest(t, http.MethodPost, "/api/v1/users", gin.H{
		"email":    userEmail("self-user"),
		"password": "password123",
		"role":     "tenant_user",
		"tenant_id": defTenant.ID,
	})
	testutil.Authorize(req, adminToken)
	w := testutil.DoJSONReq(t, router, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	token := loginUser(t, router, userEmail("self-user"), "password123")

	// Self-service key creation.
	createReq := testutil.NewJSONRequest(t, http.MethodPost, "/api/v1/me/keys", gin.H{
		"name": "self-key",
	})
	testutil.Authorize(createReq, token)
	createRes := testutil.DoJSONReq(t, router, createReq)
	require.Equal(t, http.StatusOK, createRes.Code, createRes.Body.String())
	var keyRes struct {
		Key struct {
			Key string `json:"key"`
		} `json:"key"`
	}
	testutil.DecodeJSON(t, createRes, &keyRes)
	require.NotEmpty(t, keyRes.Key.Key)

	// The key works for proxying.
	call := doChatCompletion(t, router, cfg, keyRes.Key.Key, nil)
	require.Equal(t, http.StatusOK, call.Code, call.Body.String())

	// /me/usage reports the month-to-date spend.
	usageReq := testutil.NewJSONRequest(t, http.MethodGet, "/api/v1/me/usage", nil)
	testutil.Authorize(usageReq, token)
	usageRes := testutil.DoJSONReq(t, router, usageReq)
	require.Equal(t, http.StatusOK, usageRes.Code, usageRes.Body.String())
	assert.Contains(t, usageRes.Body.String(), `"budget_used"`)
	assert.Contains(t, usageRes.Body.String(), fmt.Sprintf("%q", "self-user"))

	// /me/keys lists only the caller's own key.
	listReq := testutil.NewJSONRequest(t, http.MethodGet, "/api/v1/me/keys", nil)
	testutil.Authorize(listReq, token)
	listRes := testutil.DoJSONReq(t, router, listReq)
	require.Equal(t, http.StatusOK, listRes.Code, listRes.Body.String())
	assert.Contains(t, listRes.Body.String(), "self-key")
}

func TestTenantCostReport(t *testing.T) {
	upstream := newMockChatUpstream(t, 2, 1)
	cfg, router := setupGateway(t, upstream.URL(), nil)
	seedModelPricing(t, "openai", "gpt-test", 100, 0, 0)
	adminToken := loginAdmin(t, router, cfg)

	virtualKey := createVirtualKey(t, router, adminToken, "/api/v1/virtual-keys", "cost-key", 100, nil)
	resp := doChatCompletion(t, router, cfg, virtualKey, nil)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var defTenant models.Tenant
	require.NoError(t, database.GetDB().First(&defTenant, database.DefaultTenantID).Error)

	req := testutil.NewJSONRequest(t, http.MethodGet, fmt.Sprintf("/api/v1/tenants/%d/cost?by=user", defTenant.ID), nil)
	testutil.Authorize(req, adminToken)
	w := testutil.DoJSONReq(t, router, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"total_cost"`)
	assert.NotContains(t, w.Body.String(), `"items":[]`, "expected at least one cost row")
}
