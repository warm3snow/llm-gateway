package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/pkg/cache"
)

// CacheMiddleware creates a Gin middleware that checks/updates cache.
func CacheMiddleware(c cache.Cache) gin.HandlerFunc {
	if c == nil {
		// No-op if cache is disabled
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(ctx *gin.Context) {
		// Only cache POST requests to chat/completions (non-streaming)
		if ctx.Request.Method != "POST" || !strings.HasSuffix(ctx.FullPath(), "chat/completions") {
			ctx.Next()
			return
		}

		// Skip streaming requests
		bodyBytes, err := ctx.GetRawData()
		if err != nil {
			ctx.Next()
			return
		}
		// Restore body for downstream handlers
		ctx.Request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

		var req map[string]interface{}
		if json.Unmarshal(bodyBytes, &req) == nil && req["stream"] == true {
			ctx.Next()
			return
		}

		// Generate cache key
		provider := ctx.GetString("provider")
		model := ""
		if m, ok := req["model"].(string); ok {
			model = m
		}
		cacheKey := generateCacheKey(provider, model, bodyBytes)

		// Try cache
		cached, err := c.Get(ctx.Request.Context(), cacheKey)
		if err == nil && cached != nil {
			if validateCachedResponseGuardrail(ctx, cached.ResponseText) {
				return
			}
			ctx.Header("x-cache", "HIT")
			ctx.Data(http.StatusOK, "application/json", []byte(cached.ResponseText))
			ctx.Abort()
			return
		}

		// Not cached — capture response
		ctx.Header("x-cache", "MISS")
		writer := &bodyCaptureWriter{ResponseWriter: ctx.Writer, body: &strings.Builder{}}
		ctx.Writer = writer

		ctx.Next()

		// Cache the response if status is 200
		if ctx.Writer.Status() == http.StatusOK && writer.body.Len() > 0 {
			entry := &cache.CacheEntry{
				Key:          cacheKey,
				RequestText:  string(bodyBytes),
				ResponseText: writer.body.String(),
				Provider:     provider,
				Model:        model,
				ExpiresAt:    time.Now().Add(5 * time.Minute),
			}
			_ = c.Set(ctx.Request.Context(), cacheKey, entry, 5*time.Minute)
		}
	}
}

// generateCacheKey creates a hash-based cache key.
func generateCacheKey(provider, model string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(provider + ":" + model + ":"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// bodyCaptureWriter captures the response body for caching.
type bodyCaptureWriter struct {
	gin.ResponseWriter
	body *strings.Builder
}

func (w *bodyCaptureWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyCaptureWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}
