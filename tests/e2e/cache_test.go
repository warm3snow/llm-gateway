//go:build e2e

package e2e

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gatewaycache "github.com/warm3snow/llm-gateway/pkg/cache"
)

func TestIdenticalChatCompletionUsesCacheAfterInitialMiss(t *testing.T) {
	upstream := newMockChatUpstream(t, 3, 4)
	c, err := gatewaycache.NewCache(&gatewaycache.Config{Type: "memory", MaxEntries: 100, DefaultTTL: time.Minute})
	require.NoError(t, err)
	cfg, router := setupGateway(t, upstream.URL(), c)
	adminToken := loginAdmin(t, router, cfg)
	virtualKey := createVirtualKey(t, router, adminToken, "/api/v1/virtual-keys", "cache-key", 100, []string{"openai"})

	first := doChatCompletion(t, router, cfg, virtualKey, nil)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	assert.Equal(t, "MISS", first.Header().Get("x-cache"))
	assert.Equal(t, int64(1), upstream.Calls())

	second := doChatCompletion(t, router, cfg, virtualKey, nil)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	assert.Equal(t, "HIT", second.Header().Get("x-cache"))
	assert.JSONEq(t, first.Body.String(), second.Body.String())
	assert.Equal(t, int64(1), upstream.Calls())
}
