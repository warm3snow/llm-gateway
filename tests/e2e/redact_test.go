//go:build e2e

package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/handler"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
	"github.com/warm3snow/llm-gateway/pkg/proxy"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

type redactUpstream struct {
	lastBody atomic.Value
}

func newRedactUpstream(t *testing.T) (*redactUpstream, *httptest.Server) {
	t.Helper()
	u := &redactUpstream{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		u.lastBody.Store(string(body))
		w.Header().Set("Content-Type", "application/json")
		// Echo the last prompt back as the completion so the client can
		// assert the real value came back restored.
		var req struct {
			Messages []struct {
				Content interface{} `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &req)
		echo := ""
		if len(req.Messages) > 0 {
			if s, ok := req.Messages[len(req.Messages)-1].Content.(string); ok {
				echo = s
			}
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"id": "chatcmpl-redact", "object": "chat.completion", "model": "gpt-test",
			"choices": []map[string]interface{}{
				{"index": 0, "message": map[string]string{"role": "assistant", "content": echo}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 2, "completion_tokens": 2, "total_tokens": 4},
		})
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	return u, srv
}

func newRedactRouter(t *testing.T, cfg *config.Config) *gin.Engine {
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
	v1.Use(middleware.RedactMiddleware())
	v1.Use(middleware.UsageRecordMiddleware(cfg, virtualKeyService))
	v1.POST("/chat/completions", proxyHandler.HandleChatCompletion)
	return router
}

func setupRedactGateway(t *testing.T, enabled bool) (*config.Config, *gin.Engine, *redactUpstream) {
	t.Helper()
	cfg := testutil.TestConfig(t)
	testutil.SetupSQLiteDB(t)
	require.NoError(t, database.Bootstrap(cfg.Security.AdminUser, cfg.Security.AdminPass))
	cfg.Gateway.DefaultProvider = "openai"
	upstream, srv := newRedactUpstream(t)
	cfg.Gateway.Providers = map[string]types.Options{
		"openai": {Provider: "openai", APIKey: "sk-upstream", CustomHost: srv.URL, RequestTimeout: 5000},
	}
	router := newRedactRouter(t, cfg)
	require.NoError(t, service.ApplyRedactConfig(config.RedactConfig{
		Enabled: enabled,
		Secrets: true,
		Personal: false,
	}))
	return cfg, router, upstream
}

func TestRedactMasksRequestAndRestoresResponse(t *testing.T) {
	cfg, router, upstream := setupRedactGateway(t, true)
	adminToken := loginAdmin(t, router, cfg)
	virtualKey := createVirtualKey(t, router, adminToken, "/api/v1/virtual-keys", "redact-key", 100, []string{"openai"})

	secret := "sk-abc123XYZdef456ghi789"
	req := testutil.NewJSONRequest(t, http.MethodPost, "/v1/chat/completions", map[string]interface{}{
		"model": "gpt-test",
		"messages": []map[string]string{
			{"role": "user", "content": "my key is " + secret + " please keep it"},
		},
	})
	req.Header.Set(cfg.Security.APIKeyHeader, virtualKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// The provider saw a placeholder, not the value.
	sent, _ := upstream.lastBody.Load().(string)
	require.NotEmpty(t, sent)
	assert.NotContains(t, sent, secret)
	assert.Contains(t, sent, "{{API_KEY_")

	// The client got the real value back.
	assert.Contains(t, w.Body.String(), secret)
	assert.NotContains(t, w.Body.String(), "{{API_KEY_")

	// The usage record counted the redaction.
	var record models.UsageRecord
	require.NoError(t, database.GetDB().Where("endpoint = ?", "/v1/chat/completions").Order("id DESC").First(&record).Error)
	assert.Equal(t, 1, record.Redactions)
}

func TestRedactDisabledLeavesValuesInPlace(t *testing.T) {
	cfg, router, upstream := setupRedactGateway(t, false)
	adminToken := loginAdmin(t, router, cfg)
	virtualKey := createVirtualKey(t, router, adminToken, "/api/v1/virtual-keys", "redact-off-key", 100, []string{"openai"})

	secret := "sk-abc123XYZdef456ghi789"
	req := testutil.NewJSONRequest(t, http.MethodPost, "/v1/chat/completions", map[string]interface{}{
		"model": "gpt-test",
		"messages": []map[string]string{
			{"role": "user", "content": "my key is " + secret},
		},
	})
	req.Header.Set(cfg.Security.APIKeyHeader, virtualKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	loaded, _ := upstream.lastBody.Load().(string)
	assert.Contains(t, loaded, secret)
	assert.Contains(t, w.Body.String(), secret)
}
