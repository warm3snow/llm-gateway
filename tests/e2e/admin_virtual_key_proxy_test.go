//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/handler"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/models"
	_ "github.com/warm3snow/llm-gateway/internal/provider/openai"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
	"github.com/warm3snow/llm-gateway/pkg/proxy"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

func newGatewayRouter(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Recovery())

	handler.NewAuthHandler(cfg).RegisterRoutes(router)
	jwtMiddleware := middleware.JWTAuth(cfg)
	handler.NewVirtualKeyHandler().RegisterRoutesWithAuth(router, jwtMiddleware)

	proxyHandler := proxy.NewProxyHandler(cfg, nil)
	virtualKeyService := service.NewVirtualKeyService()
	v1 := router.Group("/v1")
	v1.Use(middleware.VirtualKeyAuth(cfg))
	v1.Use(middleware.UsageRecordMiddleware(cfg, virtualKeyService))
	v1.Use(middleware.CacheMiddleware(nil))
	v1.POST("/chat/completions", proxyHandler.HandleChatCompletion)
	return router
}

func TestAdminCreatesVirtualKeyAndProxiesChatCompletion(t *testing.T) {
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	require.NoError(t, database.Bootstrap(cfg.Security.AdminUser, cfg.Security.AdminPass))

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-upstream", r.Header.Get("Authorization"))
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "gpt-test", req.Model)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-e2e","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`))
	}))
	t.Cleanup(upstream.Close)

	cfg.Gateway.DefaultProvider = "openai"
	cfg.Gateway.Providers = map[string]types.Options{
		"openai": {Provider: "openai", APIKey: "sk-upstream", CustomHost: upstream.URL, RequestTimeout: 5000},
	}
	router := newGatewayRouter(t, cfg)

	login := testutil.DoJSON(t, router, http.MethodPost, "/api/v1/auth/login", gin.H{
		"username": cfg.Security.AdminUser,
		"password": cfg.Security.AdminPass,
	})
	require.Equal(t, http.StatusOK, login.Code, login.Body.String())
	var loginRes struct {
		Token string `json:"token"`
	}
	testutil.DecodeJSON(t, login, &loginRes)
	require.NotEmpty(t, loginRes.Token)

	createKeyReq := testutil.NewJSONRequest(t, http.MethodPost, "/api/v1/virtual-keys", gin.H{
		"name":         "e2e-key",
		"budget_total": 100,
		"providers":    []string{"openai"},
	})
	testutil.Authorize(createKeyReq, loginRes.Token)
	createKey := httptest.NewRecorder()
	router.ServeHTTP(createKey, createKeyReq)
	require.Equal(t, http.StatusOK, createKey.Code, createKey.Body.String())
	var createKeyRes struct {
		Key struct {
			Key string `json:"key"`
		} `json:"key"`
	}
	testutil.DecodeJSON(t, createKey, &createKeyRes)
	require.NotEmpty(t, createKeyRes.Key.Key)

	chatReq := testutil.NewJSONRequest(t, http.MethodPost, "/v1/chat/completions", gin.H{
		"model":    "gpt-test",
		"messages": []gin.H{{"role": "user", "content": "ping"}},
	})
	chatReq.Header.Set(cfg.Security.APIKeyHeader, createKeyRes.Key.Key)
	chat := httptest.NewRecorder()
	router.ServeHTTP(chat, chatReq)

	require.Equal(t, http.StatusOK, chat.Code, chat.Body.String())
	assert.JSONEq(t, `{"id":"chatcmpl-e2e","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":4,"total_tokens":7}}`, chat.Body.String())

	var usageCount int64
	require.NoError(t, database.GetDB().Model(&models.UsageRecord{}).Count(&usageCount).Error)
	assert.Equal(t, int64(1), usageCount)
}
