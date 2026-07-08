//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/handler"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	_ "github.com/warm3snow/llm-gateway/internal/provider/openai"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
	gatewaycache "github.com/warm3snow/llm-gateway/pkg/cache"
	"github.com/warm3snow/llm-gateway/pkg/guardrail"
	"github.com/warm3snow/llm-gateway/pkg/proxy"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

type guardrailUpstream struct {
	server        *httptest.Server
	chatCalls     atomic.Int64
	completeCalls atomic.Int64
	embedCalls    atomic.Int64
	chatContent   string
}

func newGuardrailUpstream(t *testing.T, chatContent string) *guardrailUpstream {
	t.Helper()
	upstream := &guardrailUpstream{chatContent: chatContent}
	upstream.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/chat/completions":
			upstream.chatCalls.Add(1)
			_, _ = w.Write([]byte(`{"id":"chatcmpl-e2e","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":` + strconv.Quote(upstream.chatContent) + `},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`))
		case "/completions":
			upstream.completeCalls.Add(1)
			_, _ = w.Write([]byte(`{"id":"cmpl-e2e","object":"text_completion","choices":[{"index":0,"text":"pong","finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`))
		case "/embeddings":
			upstream.embedCalls.Add(1)
			_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","embedding":[0.1,0.2],"index":0}],"usage":{"prompt_tokens":1,"total_tokens":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.server.Close)
	return upstream
}

func newGuardrailRouter(t *testing.T, cfg *config.Config, c gatewaycache.Cache) *gin.Engine {
	t.Helper()
	manager, err := guardrail.NewManagerFromConfig(cfg.Gateway.GuardrailsEnabled, cfg.Gateway.DefaultConfig.Guardrails)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Recovery())

	handler.NewAuthHandler(cfg).RegisterRoutes(router)
	jwtMiddleware := middleware.JWTAuth(cfg)
	handler.NewVirtualKeyHandler().RegisterRoutesWithAuth(router, jwtMiddleware)

	proxyHandler := proxy.NewProxyHandler(cfg, c)
	proxyHandler.SetGuardrailManager(manager)
	virtualKeyService := service.NewVirtualKeyService()
	v1 := router.Group("/v1")
	v1.Use(middleware.VirtualKeyAuth(cfg))
	v1.Use(middleware.GuardrailMiddleware(manager))
	v1.Use(middleware.UsageRecordMiddleware(cfg, virtualKeyService))
	v1.Use(middleware.CacheMiddleware(c))
	v1.POST("/chat/completions", proxyHandler.HandleChatCompletion)
	v1.POST("/completions", proxyHandler.HandleCompletion)
	v1.POST("/embeddings", proxyHandler.HandleEmbedding)
	v1.POST("/chat/completions/stream", proxyHandler.HandleStreamRequest)
	v1.Any("/proxy/*path", proxyHandler.ProxyRequest)
	return router
}

func setupGuardrailGateway(t *testing.T, upstreamURL string, enabled bool, keywords ...string) (*config.Config, *gin.Engine) {
	t.Helper()
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	require.NoError(t, database.Bootstrap(cfg.Security.AdminUser, cfg.Security.AdminPass))
	cfg.Gateway.DefaultProvider = "openai"
	cfg.Gateway.GuardrailsEnabled = enabled
	cfg.Gateway.DefaultConfig = &types.Config{
		Guardrails: []types.GuardrailConfig{
			{
				Type: "keyword",
				Parameters: map[string]any{
					"keywords":      stringSliceToAny(keywords),
					"matchMode":     "contains",
					"caseSensitive": false,
				},
			},
		},
	}
	cfg.Gateway.Providers = map[string]types.Options{
		"openai": {Provider: "openai", APIKey: "sk-upstream", CustomHost: upstreamURL, RequestTimeout: 5000},
	}
	return cfg, newGuardrailRouter(t, cfg, nil)
}

func stringSliceToAny(values []string) []any {
	items := make([]any, 0, len(values))
	for _, value := range values {
		items = append(items, value)
	}
	return items
}

func createGuardrailVirtualKey(t *testing.T, router *gin.Engine, cfg *config.Config, name string) string {
	t.Helper()
	adminToken := loginAdmin(t, router, cfg)
	return createVirtualKey(t, router, adminToken, "/api/v1/virtual-keys", name, 100, []string{"openai"})
}

func doGuardrailRequest(t *testing.T, router *gin.Engine, cfg *config.Config, virtualKey, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewJSONRequest(t, http.MethodPost, path, body)
	req.Header.Set(cfg.Security.APIKeyHeader, virtualKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestGuardrailEnabledBlocksChatCompletionRequestAndSkipsProvider(t *testing.T) {
	upstream := newGuardrailUpstream(t, "pong")
	cfg, router := setupGuardrailGateway(t, upstream.server.URL, true, "blocked")
	virtualKey := createGuardrailVirtualKey(t, router, cfg, "guardrail-chat-block")

	res := doGuardrailRequest(t, router, cfg, virtualKey, "/v1/chat/completions", gin.H{
		"model":    "gpt-test",
		"messages": []gin.H{{"role": "user", "content": "this contains blocked text"}},
	})

	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Contains(t, res.Body.String(), "guardrail_error")
	assert.Equal(t, int64(0), upstream.chatCalls.Load())
}

func TestGuardrailDisabledAllowsBlockedChatCompletionRequest(t *testing.T) {
	upstream := newGuardrailUpstream(t, "pong")
	cfg, router := setupGuardrailGateway(t, upstream.server.URL, false, "blocked")
	virtualKey := createGuardrailVirtualKey(t, router, cfg, "guardrail-disabled")

	res := doGuardrailRequest(t, router, cfg, virtualKey, "/v1/chat/completions", gin.H{
		"model":    "gpt-test",
		"messages": []gin.H{{"role": "user", "content": "this contains blocked text"}},
	})

	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	assert.Equal(t, int64(1), upstream.chatCalls.Load())
}

func TestGuardrailBlocksChatCompletionResponse(t *testing.T) {
	upstream := newGuardrailUpstream(t, "blocked response")
	cfg, router := setupGuardrailGateway(t, upstream.server.URL, true, "blocked response")
	virtualKey := createGuardrailVirtualKey(t, router, cfg, "guardrail-response-block")

	res := doGuardrailRequest(t, router, cfg, virtualKey, "/v1/chat/completions", gin.H{
		"model":    "gpt-test",
		"messages": []gin.H{{"role": "user", "content": "safe prompt"}},
	})

	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Contains(t, res.Body.String(), "guardrail_error")
	assert.NotContains(t, res.Body.String(), "blocked response")
	assert.Equal(t, int64(1), upstream.chatCalls.Load())
}

func TestGuardrailBlocksProxyChatCompletionRequestAndSkipsProvider(t *testing.T) {
	upstream := newGuardrailUpstream(t, "pong")
	cfg, router := setupGuardrailGateway(t, upstream.server.URL, true, "blocked")
	virtualKey := createGuardrailVirtualKey(t, router, cfg, "guardrail-proxy-request-block")

	res := doGuardrailRequest(t, router, cfg, virtualKey, "/v1/proxy/chat/completions", gin.H{
		"model":    "gpt-test",
		"messages": []gin.H{{"role": "user", "content": "this contains blocked text"}},
	})

	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Contains(t, res.Body.String(), "guardrail_error")
	assert.Equal(t, int64(0), upstream.chatCalls.Load())
}

func TestGuardrailBlocksProxyChatCompletionResponse(t *testing.T) {
	upstream := newGuardrailUpstream(t, "blocked response")
	cfg, router := setupGuardrailGateway(t, upstream.server.URL, true, "blocked response")
	virtualKey := createGuardrailVirtualKey(t, router, cfg, "guardrail-proxy-response-block")

	res := doGuardrailRequest(t, router, cfg, virtualKey, "/v1/proxy/chat/completions", gin.H{
		"model":    "gpt-test",
		"messages": []gin.H{{"role": "user", "content": "safe prompt"}},
	})

	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Contains(t, res.Body.String(), "guardrail_error")
	assert.NotContains(t, res.Body.String(), "blocked response")
	assert.Equal(t, int64(1), upstream.chatCalls.Load())
}

func TestGuardrailBlocksDoubleSlashProxyChatCompletionRequestAndSkipsProvider(t *testing.T) {
	upstream := newGuardrailUpstream(t, "pong")
	cfg, router := setupGuardrailGateway(t, upstream.server.URL, true, "blocked")
	virtualKey := createGuardrailVirtualKey(t, router, cfg, "guardrail-proxy-double-slash-request-block")

	res := doGuardrailRequest(t, router, cfg, virtualKey, "/v1/proxy//chat/completions", gin.H{
		"model":    "gpt-test",
		"messages": []gin.H{{"role": "user", "content": "this contains blocked text"}},
	})

	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Contains(t, res.Body.String(), "guardrail_error")
	assert.Equal(t, int64(0), upstream.chatCalls.Load())
}

func TestGuardrailBlocksDoubleSlashProxyChatCompletionResponse(t *testing.T) {
	upstream := newGuardrailUpstream(t, "blocked response")
	cfg, router := setupGuardrailGateway(t, upstream.server.URL, true, "blocked response")
	virtualKey := createGuardrailVirtualKey(t, router, cfg, "guardrail-proxy-double-slash-response-block")

	res := doGuardrailRequest(t, router, cfg, virtualKey, "/v1/proxy//chat/completions", gin.H{
		"model":    "gpt-test",
		"messages": []gin.H{{"role": "user", "content": "safe prompt"}},
	})

	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Contains(t, res.Body.String(), "guardrail_error")
	assert.NotContains(t, res.Body.String(), "blocked response")
	assert.Equal(t, int64(1), upstream.chatCalls.Load())
}

func TestGuardrailBlocksCompletionRequestAndSkipsProvider(t *testing.T) {
	upstream := newGuardrailUpstream(t, "pong")
	cfg, router := setupGuardrailGateway(t, upstream.server.URL, true, "blocked")
	virtualKey := createGuardrailVirtualKey(t, router, cfg, "guardrail-completion-block")

	res := doGuardrailRequest(t, router, cfg, virtualKey, "/v1/completions", gin.H{
		"model":  "gpt-test",
		"prompt": "blocked prompt",
	})

	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Equal(t, int64(0), upstream.completeCalls.Load())
}

func TestGuardrailBlocksEmbeddingRequestAndSkipsProvider(t *testing.T) {
	upstream := newGuardrailUpstream(t, "pong")
	cfg, router := setupGuardrailGateway(t, upstream.server.URL, true, "blocked")
	virtualKey := createGuardrailVirtualKey(t, router, cfg, "guardrail-embedding-block")

	res := doGuardrailRequest(t, router, cfg, virtualKey, "/v1/embeddings", gin.H{
		"model": "text-embedding-test",
		"input": "blocked input",
	})

	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Equal(t, int64(0), upstream.embedCalls.Load())
}

func TestGuardrailValidatesCachedChatCompletionResponse(t *testing.T) {
	upstream := newGuardrailUpstream(t, "pong")
	cacheInstance, err := gatewaycache.NewCache(&gatewaycache.Config{Type: "memory", MaxEntries: 10, DefaultTTL: time.Minute})
	require.NoError(t, err)

	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	require.NoError(t, database.Bootstrap(cfg.Security.AdminUser, cfg.Security.AdminPass))
	cfg.Gateway.DefaultProvider = "openai"
	cfg.Gateway.GuardrailsEnabled = true
	cfg.Gateway.DefaultConfig = &types.Config{Guardrails: []types.GuardrailConfig{{
		Type: "keyword",
		Parameters: map[string]any{
			"keywords":      []any{"cached-blocked"},
			"matchMode":     "contains",
			"caseSensitive": false,
		},
	}}}
	cfg.Gateway.Providers = map[string]types.Options{
		"openai": {Provider: "openai", APIKey: "sk-upstream", CustomHost: upstream.server.URL, RequestTimeout: 5000},
	}
	router := newGuardrailRouter(t, cfg, cacheInstance)
	virtualKey := createGuardrailVirtualKey(t, router, cfg, "guardrail-cache-block")

	body := gin.H{"model": "gpt-test", "messages": []gin.H{{"role": "user", "content": "safe prompt"}}}
	var requestBody bytes.Buffer
	require.NoError(t, json.NewEncoder(&requestBody).Encode(body))
	bodyBytes := requestBody.Bytes()
	cacheKey := guardrailE2ECacheKey("", "gpt-test", bodyBytes)
	require.NoError(t, cacheInstance.Set(context.Background(), cacheKey, &gatewaycache.CacheEntry{
		Key:          cacheKey,
		RequestText:  string(bodyBytes),
		ResponseText: `{"id":"chatcmpl-e2e","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"cached-blocked response"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`,
		Provider:     "",
		Model:        "gpt-test",
		ExpiresAt:    time.Now().Add(time.Minute),
	}, time.Minute))

	res := doGuardrailRequest(t, router, cfg, virtualKey, "/v1/chat/completions", body)

	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	assert.Contains(t, res.Body.String(), "guardrail_error")
	assert.Equal(t, int64(0), upstream.chatCalls.Load())
}

func guardrailE2ECacheKey(provider, model string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(provider + ":" + model + ":"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
