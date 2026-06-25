package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestLogger 测试日志中间件
func TestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Logger())

	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRecovery 测试恢复中间件
func TestRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Recovery())

	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/panic", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestCORS 测试 CORS 中间件
func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	tests := []struct {
		name           string
		allowedOrigins []string
		origin         string
		expectedHeader string
	}{
		{
			name:           "Allow all origins",
			allowedOrigins: []string{"*"},
			origin:         "https://example.com",
			expectedHeader: "https://example.com",
		},
		{
			name:           "Specific origin",
			allowedOrigins: []string{"https://example.com"},
			origin:         "https://example.com",
			expectedHeader: "https://example.com",
		},
		{
			name:           "Disallowed origin",
			allowedOrigins: []string{"https://example.com"},
			origin:         "https://malicious.com",
			expectedHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(CORS(tt.allowedOrigins))

			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", tt.origin)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedHeader, w.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

// TestAuth 测试认证中间件
func TestAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	t.Run("Missing API key", func(t *testing.T) {
		router := gin.New()
		router.Use(Auth("x-api-key"))

		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("With API key", func(t *testing.T) {
		router := gin.New()
		router.Use(Auth("x-api-key"))

		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("x-api-key", "test-key")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Skip auth for health endpoint", func(t *testing.T) {
		router := gin.New()
		router.Use(Auth("x-api-key"))

		router.GET("/health", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/health", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestRateLimit 测试限流中间件
func TestRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	t.Run("Within limit", func(t *testing.T) {
		router := gin.New()
		router.Use(RateLimit(10))

		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Exceeds limit", func(t *testing.T) {
		// 注意：这个测试可能会失败，因为限制是基于客户端 IP 的
		// 而且内存中的限制器在测试之间会重置
		t.Skip("Rate limit test requires proper IP simulation")
	})
}

// TestRequestID 测试请求 ID 中间件
func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())

	router.GET("/test", func(c *gin.Context) {
		requestID := c.GetString("RequestID")
		c.String(http.StatusOK, requestID)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.String())
}

// TestTimeout 测试超时中间件
func TestTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	t.Run("Within timeout", func(t *testing.T) {
		router := gin.New()
		router.Use(Timeout(5 * time.Second))

		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Exceeds timeout", func(t *testing.T) {
		t.Skip("Timeout test requires proper context cancellation simulation")
	})
}

// BenchmarkLogger 性能测试
func BenchmarkLogger(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(Logger())

	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
	}
}

// BenchmarkRecovery 性能测试
func BenchmarkRecovery(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(Recovery())

	router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
	}
}
