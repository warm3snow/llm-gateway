package cache

import (
	"context"
	"fmt"
	"time"
)

// Cache defines the interface for caching responses.
type Cache interface {
	Get(ctx context.Context, key string) (*CacheEntry, error)
	Set(ctx context.Context, key string, value *CacheEntry, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
}

// CacheEntry represents a cached response.
type CacheEntry struct {
	Key          string    `json:"key"`
	RequestText  string    `json:"request_text,omitempty"`
	ResponseText string    `json:"response_text,omitempty"`
	Provider     string    `json:"provider,omitempty"`
	Model        string    `json:"model,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Config holds cache configuration.
type Config struct {
	Type       string // "memory", "redis"
	RedisAddr  string
	RedisPass  string
	RedisDB    int
	MaxEntries int
	DefaultTTL time.Duration
}

// NewCache creates a new cache instance based on config.
func NewCache(cfg *Config) (Cache, error) {
	switch cfg.Type {
	case "redis":
		return newRedisCache(cfg)
	case "memory", "":
		return newMemoryCache(cfg.MaxEntries)
	default:
		return nil, fmt.Errorf("unsupported cache type: %s", cfg.Type)
	}
}
