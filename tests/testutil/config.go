package testutil

import (
	"testing"
	"time"

	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/types"
)

func TestConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Server: config.ServerConfig{
			Host:         "127.0.0.1",
			Port:         8080,
			Mode:         "test",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		Gateway: config.GatewayConfig{
			DefaultProvider:    "openai",
			SupportedProviders: []string{"openai"},
			MaxRequestTimeout:  5000,
			Providers: map[string]types.Options{
				"openai": {
					Provider:   "openai",
					APIKey:     "sk-test",
					CustomHost: "http://127.0.0.1",
				},
			},
		},
		Cache: config.CacheConfig{
			Enabled:    false,
			Type:       "memory",
			DefaultTTL: time.Minute,
		},
		Database: config.DatabaseConfig{
			Driver:   "sqlite",
			LogLevel: "silent",
		},
		Security: config.SecurityConfig{
			APIKeyHeader:   "x-llm-gateway-api-key",
			AllowedOrigins: []string{"*"},
			AdminUser:      "admin",
			AdminPass:      "admin123",
			JWTSecret:      "test-secret",
		},
	}
}
