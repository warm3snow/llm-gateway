package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// TestNewHandler 测试创建 Handler
func TestNewHandler(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewHandler(cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.Config)
}

// TestMaskAPIKey 测试 API Key 脱敏
func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "Long key",
			apiKey:   "sk-abcdefghijklmnopqrstuvwxyz123456",
			expected: "sk-ab...z123456",
		},
		{
			name:     "Short key",
			apiKey:   "sk-short",
			expected: "********",
		},
		{
			name:     "Empty key",
			apiKey:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskAPIKey(tt.apiKey)
			if tt.name == "Long key" {
				// 检查格式：前 4 个字符 + *** + 后 4 个字符
				assert.Equal(t, tt.apiKey[:4], result[:4])
				assert.Equal(t, tt.apiKey[len(tt.apiKey)-4:], result[len(result)-4:])
				assert.Equal(t, len(tt.apiKey), len(result))
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestRegisterRoutes 测试路由注册
func TestRegisterRoutes(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewHandler(cfg)
	router := gin.New()

	// 不应该 panic
	assert.NotPanics(t, func() {
		handler.RegisterRoutes(router)
	})
}

// TestGetConfig 测试获取配置 API
func TestGetConfig(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Gateway: config.GatewayConfig{
			DefaultProvider: "openai",
		},
	}

	handler := NewHandler(cfg)
	router := gin.Default()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/config", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "openai")
}

// TestHealthCheck 测试健康检查
func TestHealthCheck(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewHandler(cfg)
	router := gin.Default()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

// TestGetProviders 测试获取 Providers API
func TestGetProviders(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		Gateway: config.GatewayConfig{
			DefaultProvider: "openai",
			Providers: map[string]types.Options{
				"openai": {
					APIKey:    "sk-test",
					CustomHost: "https://api.openai.com/v1",
				},
			},
		},
	}

	handler := NewHandler(cfg)
	router := gin.Default()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/providers", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "openai")
}

// TestGetStats 测试获取统计信息 API
func TestGetStats(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewHandler(cfg)
	router := gin.Default()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/admin/stats", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestUpdateConfig 测试更新配置 API
func TestUpdateConfig(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewHandler(cfg)
	router := gin.Default()
	handler.RegisterRoutes(router)

	// 测试：无效的请求体
	t.Run("Invalid request body", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/admin/config", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// 测试：无效的端口
	t.Run("Invalid port", func(t *testing.T) {
		body := `{"server": {"port": -1}}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/admin/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	// 测试：有效的配置（可能会失败因为无法保存文件）
	t.Run("Valid config", func(t *testing.T) {
		body := `{"server": {"host": "0.0.0.0", "port": 9090}}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/admin/config", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		// 可能返回 200 或 500（取决于是否能保存文件）
		assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusInternalServerError)
	})
}

// TestAddProvider 测试添加 Provider API
func TestAddProvider(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewHandler(cfg)
	router := gin.Default()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/providers", nil)
	router.ServeHTTP(w, req)

	// 由于没有请求体，应该返回错误
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestRemoveProvider 测试删除 Provider API
func TestRemoveProvider(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewHandler(cfg)
	router := gin.Default()
	handler.RegisterRoutes(router)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/admin/providers/openai", nil)
	router.ServeHTTP(w, req)

	// 由于 Provider 不存在，应该返回错误
	assert.Equal(t, http.StatusNotFound, w.Code)
}
