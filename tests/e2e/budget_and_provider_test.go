//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
)

func TestVirtualKeyBudgetExhaustionBlocksFurtherRequests(t *testing.T) {
	upstream := newMockChatUpstream(t, 1, 0)
	cfg, router := setupGateway(t, upstream.URL(), nil)
	seedModelPricing(t, "openai", "gpt-test", 100, 0, 0)
	adminToken := loginAdmin(t, router, cfg)
	virtualKey := createVirtualKey(t, router, adminToken, "/api/v1/virtual-keys", "budget-key", 1, []string{"openai"})

	first := doChatCompletion(t, router, cfg, virtualKey, nil)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())

	var key models.VirtualKey
	require.NoError(t, database.GetDB().Where("name = ?", "budget-key").First(&key).Error)
	assert.Equal(t, float64(1), key.BudgetUsed)

	second := doChatCompletion(t, router, cfg, virtualKey, nil)
	assert.Equal(t, http.StatusForbidden, second.Code, second.Body.String())
	assert.Contains(t, second.Body.String(), "Budget exceeded")
	assert.Equal(t, int64(1), upstream.Calls())
}

func TestVirtualKeyProviderAllowlistRejectsDisallowedProvider(t *testing.T) {
	upstream := newMockChatUpstream(t, 1, 1)
	cfg, router := setupGateway(t, upstream.URL(), nil)
	adminToken := loginAdmin(t, router, cfg)
	virtualKey := createVirtualKey(t, router, adminToken, "/api/v1/virtual-keys", "anthropic-only", 100, []string{"anthropic"})

	res := doChatCompletion(t, router, cfg, virtualKey, map[string]string{"x-llm-provider": "openai"})

	assert.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Contains(t, res.Body.String(), "Provider is not allowed")
	assert.Equal(t, int64(0), upstream.Calls())
}
