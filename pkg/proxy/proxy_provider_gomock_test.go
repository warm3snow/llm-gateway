package proxy

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/provider"
	"github.com/warm3snow/llm-gateway/internal/provider/mockprovider"
	"github.com/warm3snow/llm-gateway/internal/types"
	"go.uber.org/mock/gomock"
)

func newProxyTestHandler(t *testing.T, prov provider.Provider) *ProxyHandler {
	t.Helper()
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			DefaultProvider:   "mock",
			MaxRequestTimeout: 5000,
			Providers: map[string]types.Options{
				"mock": {Provider: "mock", APIKey: "sk-test"},
			},
		},
	}
	factory := provider.NewProviderFactory()
	factory.Register("mock", func(opts *types.Options) (provider.Provider, error) {
		return prov, nil
	})
	handler := NewProxyHandler(cfg, nil)
	handler.ProviderFactory = factory
	return handler
}

func TestProxyHandlerChatCompletionUsesInjectedProviderFactory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	mockProvider := mockprovider.NewMockProvider(ctrl)
	handler := newProxyTestHandler(t, mockProvider)

	mockProvider.EXPECT().
		ChatCompletion(gomock.Any(), gomock.AssignableToTypeOf(&types.ChatCompletionRequest{}), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
			assert.Equal(t, "gpt-test", req.Model)
			assert.Equal(t, "mock", opts.Provider)
			return &http.Response{
				StatusCode: http.StatusCreated,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-test","object":"chat.completion"}`)),
			}, nil
		})

	router := gin.New()
	router.POST("/v1/chat/completions", handler.HandleChatCompletion)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.JSONEq(t, `{"id":"chatcmpl-test","object":"chat.completion"}`, w.Body.String())
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestProxyHandlerChatCompletionReturnsProviderError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := gomock.NewController(t)
	mockProvider := mockprovider.NewMockProvider(ctrl)
	handler := newProxyTestHandler(t, mockProvider)

	mockProvider.EXPECT().
		ChatCompletion(gomock.Any(), gomock.AssignableToTypeOf(&types.ChatCompletionRequest{}), gomock.Any()).
		Return(nil, errors.New("upstream unavailable"))

	router := gin.New()
	router.POST("/v1/chat/completions", handler.HandleChatCompletion)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-test","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "upstream unavailable")
	assert.Contains(t, w.Body.String(), "request_error")
}
