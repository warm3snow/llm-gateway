//go:build e2e

package e2e

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

func TestUsageRecordCapturesBillingFields(t *testing.T) {
	upstream := newMockChatUpstream(t, 3, 4)
	cfg, router := setupGateway(t, upstream.URL(), nil)
	seedModelPricing(t, "openai", "gpt-test", 10, 20, 0)
	adminToken := loginAdmin(t, router, cfg)
	virtualKey := createVirtualKey(t, router, adminToken, "/api/v1/virtual-keys", "usage-fields", 100, []string{"openai"})

	res := doChatCompletion(t, router, cfg, virtualKey, nil)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	var key models.VirtualKey
	require.NoError(t, database.GetDB().Where("name = ?", "usage-fields").First(&key).Error)
	var record models.UsageRecord
	require.NoError(t, database.GetDB().Where("virtual_key_id = ?", key.ID).First(&record).Error)

	assert.Equal(t, database.DefaultTenantID, record.TenantID)
	require.NotNil(t, record.VirtualKeyID)
	assert.Equal(t, key.ID, *record.VirtualKeyID)
	assert.Equal(t, "usage-fields", record.VirtualKeyName)
	assert.Equal(t, "openai", record.Provider)
	assert.Equal(t, "gpt-test", record.Model)
	assert.Equal(t, "/v1/chat/completions", record.Endpoint)
	assert.Equal(t, http.StatusOK, record.StatusCode)
	assert.Equal(t, 3, record.InputTokens)
	assert.Equal(t, 4, record.OutputTokens)
	assert.InDelta(t, 1.1, record.Cost, 0.000001)
	assert.InDelta(t, record.Cost, key.BudgetUsed, 0.000001)
}

func TestTenantScopedKeysAndUsageDoNotLeakAcrossTenants(t *testing.T) {
	upstream := newMockChatUpstream(t, 2, 2)
	cfg, router := setupGateway(t, upstream.URL(), nil)
	tenantOneID := database.DefaultTenantID
	tenantTwo := testutil.CreateTenant(t, 2, "tenant-two")
	tenantOneUser := testutil.CreateUser(t, &tenantOneID, "tenant-one-admin", "secret", models.RoleTenantAdmin)
	tenantTwoUser := testutil.CreateUser(t, &tenantTwo.ID, "tenant-two-admin", "secret", models.RoleTenantAdmin)
	testutil.CreateTenantMember(t, tenantOneUser.ID, tenantOneID, models.RoleTenantAdmin)
	testutil.CreateTenantMember(t, tenantTwoUser.ID, tenantTwo.ID, models.RoleTenantAdmin)
	tenantOneToken := testutil.SignJWT(t, cfg, testutil.TokenClaims{UserID: tenantOneUser.ID, Username: tenantOneUser.Username, Role: models.RoleTenantAdmin, TenantID: tenantOneID})
	tenantTwoToken := testutil.SignJWT(t, cfg, testutil.TokenClaims{UserID: tenantTwoUser.ID, Username: tenantTwoUser.Username, Role: models.RoleTenantAdmin, TenantID: tenantTwo.ID})

	tenantOneKey := createVirtualKey(t, router, tenantOneToken, "/api/v1/virtual-keys", "tenant-one-key", 100, []string{"openai"})
	tenantTwoKey := createVirtualKey(t, router, tenantTwoToken, "/api/v1/virtual-keys", "tenant-two-key", 100, []string{"openai"})

	listReq := testutil.NewJSONRequest(t, http.MethodGet, "/api/v1/virtual-keys", nil)
	testutil.Authorize(listReq, tenantOneToken)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)
	require.Equal(t, http.StatusOK, listRes.Code, listRes.Body.String())
	var listBody struct {
		VirtualKeys []struct {
			Name     string `json:"name"`
			TenantID uint   `json:"tenant_id"`
		} `json:"virtual_keys"`
	}
	testutil.DecodeJSON(t, listRes, &listBody)
	require.Len(t, listBody.VirtualKeys, 1)
	assert.Equal(t, "tenant-one-key", listBody.VirtualKeys[0].Name)
	assert.Equal(t, tenantOneID, listBody.VirtualKeys[0].TenantID)

	require.Equal(t, http.StatusOK, doChatCompletion(t, router, cfg, tenantOneKey, nil).Code)
	require.Equal(t, http.StatusOK, doChatCompletion(t, router, cfg, tenantTwoKey, nil).Code)

	var tenantOneUsageCount int64
	require.NoError(t, database.GetDB().Model(&models.UsageRecord{}).Where("tenant_id = ?", tenantOneID).Count(&tenantOneUsageCount).Error)
	assert.Equal(t, int64(1), tenantOneUsageCount)
	var tenantTwoUsageCount int64
	require.NoError(t, database.GetDB().Model(&models.UsageRecord{}).Where("tenant_id = ?", tenantTwo.ID).Count(&tenantTwoUsageCount).Error)
	assert.Equal(t, int64(1), tenantTwoUsageCount)
}
