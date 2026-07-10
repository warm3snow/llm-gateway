package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/pkg/cache"
)

type fakeResponseCache struct {
	entries map[string]*cache.CacheEntry
	ttls    []time.Duration
}

func newFakeResponseCache() *fakeResponseCache {
	return &fakeResponseCache{entries: map[string]*cache.CacheEntry{}}
}

func (f *fakeResponseCache) Get(ctx context.Context, key string) (*cache.CacheEntry, error) {
	return f.entries[key], nil
}

func (f *fakeResponseCache) Set(ctx context.Context, key string, value *cache.CacheEntry, ttl time.Duration) error {
	f.entries[key] = value
	f.ttls = append(f.ttls, ttl)
	return nil
}

func (f *fakeResponseCache) Delete(ctx context.Context, key string) error {
	delete(f.entries, key)
	return nil
}

func (f *fakeResponseCache) Clear(ctx context.Context) error {
	f.entries = map[string]*cache.CacheEntry{}
	return nil
}

func TestCacheMiddlewareUsesExplicitProviderAndConfiguredTTL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := newFakeResponseCache()
	router := gin.New()
	router.Use(CacheMiddleware(fake, 42*time.Second, "ollama"))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"provider": c.GetHeader("x-llm-provider")})
	})

	body := `{"model":"gpt-test","messages":[{"role":"user","content":"hi"}]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", stringsReader(body))
	req.Header.Set("x-llm-provider", "openai")
	router.ServeHTTP(w, req)
	if w.Header().Get("x-cache") != "MISS" {
		t.Fatalf("first request cache header = %q, want MISS", w.Header().Get("x-cache"))
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", stringsReader(body))
	req.Header.Set("x-llm-provider", "anthropic")
	router.ServeHTTP(w, req)
	if w.Header().Get("x-cache") != "MISS" {
		t.Fatalf("different provider cache header = %q, want MISS", w.Header().Get("x-cache"))
	}
	if len(fake.entries) != 2 {
		t.Fatalf("cache entries = %d, want 2 provider-scoped entries", len(fake.entries))
	}
	for _, ttl := range fake.ttls {
		if ttl != 42*time.Second {
			t.Fatalf("ttl = %s, want 42s", ttl)
		}
	}
}

func TestCacheMiddlewareSkipsAutoModeWithoutExplicitProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := newFakeResponseCache()
	router := gin.New()
	router.Use(CacheMiddleware(fake, time.Minute, "ollama"))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", stringsReader(`{"model":"auto","messages":[{"role":"user","content":"hi"}]}`))
	router.ServeHTTP(w, req)

	if w.Header().Get("x-cache") != "" {
		t.Fatalf("auto-mode cache header = %q, want empty", w.Header().Get("x-cache"))
	}
	if len(fake.entries) != 0 {
		t.Fatalf("cache entries = %d, want 0", len(fake.entries))
	}
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }
