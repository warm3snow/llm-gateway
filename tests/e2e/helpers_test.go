//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/handler"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/models"
	_ "github.com/warm3snow/llm-gateway/internal/provider/openai"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
	gatewaycache "github.com/warm3snow/llm-gateway/pkg/cache"
	"github.com/warm3snow/llm-gateway/pkg/proxy"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

type mockChatUpstream struct {
	server *httptest.Server
	calls  atomic.Int64
}

func newMockChatUpstream(t *testing.T, usagePrompt, usageCompletion int) *mockChatUpstream {
	t.Helper()
	upstream := &mockChatUpstream{}
	upstream.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream.calls.Add(1)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/chat/completions", r.URL.Path)
		var req struct {
			Model string `json:"model"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		require.NotEmpty(t, req.Model)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-e2e","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":` + strconv.Itoa(usagePrompt) + `,"completion_tokens":` + strconv.Itoa(usageCompletion) + `,"total_tokens":` + strconv.Itoa(usagePrompt+usageCompletion) + `}}`))
	}))
	t.Cleanup(upstream.server.Close)
	return upstream
}

func (u *mockChatUpstream) URL() string {
	return u.server.URL
}

func (u *mockChatUpstream) Calls() int64 {
	return u.calls.Load()
}

func newGatewayRouterWithCache(t *testing.T, cfg *config.Config, c gatewaycache.Cache) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Recovery())

	handler.NewAuthHandler(cfg).RegisterRoutes(router)
	jwtMiddleware := middleware.JWTAuth(cfg)
	handler.NewVirtualKeyHandler().RegisterRoutesWithAuth(router, jwtMiddleware)

	proxyHandler := proxy.NewProxyHandler(cfg, c)
	virtualKeyService := service.NewVirtualKeyService()
	v1 := router.Group("/v1")
	v1.Use(middleware.VirtualKeyAuth(cfg))
	v1.Use(middleware.UsageRecordMiddleware(cfg, virtualKeyService))
	v1.Use(middleware.CacheMiddleware(c, time.Minute, cfg.Gateway.DefaultProvider))
	v1.POST("/chat/completions", proxyHandler.HandleChatCompletion)
	return router
}

func setupGateway(t *testing.T, upstreamURL string, c gatewaycache.Cache) (*config.Config, *gin.Engine) {
	t.Helper()
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	require.NoError(t, database.Bootstrap(cfg.Security.AdminUser, cfg.Security.AdminPass))
	cfg.Gateway.DefaultProvider = "openai"
	cfg.Gateway.Providers = map[string]types.Options{
		"openai": {Provider: "openai", APIKey: "sk-upstream", CustomHost: upstreamURL, RequestTimeout: 5000},
	}
	return cfg, newGatewayRouterWithCache(t, cfg, c)
}

func seedModelPricing(t *testing.T, provider, model string, inputPrice, outputPrice, cacheReadPrice float64) {
	t.Helper()
	require.NoError(t, models.UpsertModelPricing(database.GetDB(), &models.ModelPricing{
		Provider:       provider,
		Model:          model,
		InputPrice:     inputPrice,
		OutputPrice:    outputPrice,
		CacheReadPrice: cacheReadPrice,
		Currency:       "USD",
		Source:         "e2e",
	}))
}

func loginAdmin(t *testing.T, router *gin.Engine, cfg *config.Config) string {
	t.Helper()
	login := testutil.DoJSON(t, router, http.MethodPost, "/api/v1/auth/login", gin.H{
		"username": cfg.Security.AdminUser,
		"password": cfg.Security.AdminPass,
	})
	require.Equal(t, http.StatusOK, login.Code, login.Body.String())
	var res struct {
		Token string `json:"token"`
	}
	testutil.DecodeJSON(t, login, &res)
	require.NotEmpty(t, res.Token)
	return res.Token
}

func createVirtualKey(t *testing.T, router *gin.Engine, token, path, name string, budget float64, providers []string) string {
	t.Helper()
	req := testutil.NewJSONRequest(t, http.MethodPost, path, gin.H{
		"name":         name,
		"budget_total": budget,
		"providers":    providers,
	})
	testutil.Authorize(req, token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var res struct {
		Key struct {
			Key string `json:"key"`
			ID  uint   `json:"id"`
		} `json:"key"`
	}
	testutil.DecodeJSON(t, w, &res)
	require.NotEmpty(t, res.Key.Key)
	return res.Key.Key
}

func doChatCompletion(t *testing.T, router *gin.Engine, cfg *config.Config, virtualKey string, extraHeaders map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := testutil.NewJSONRequest(t, http.MethodPost, "/v1/chat/completions", gin.H{
		"model":    "gpt-test",
		"messages": []gin.H{{"role": "user", "content": "ping"}},
	})
	req.Header.Set(cfg.Security.APIKeyHeader, virtualKey)
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}
