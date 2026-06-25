package proxy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/warm3snow/llm-gateway/internal/config"
)

// TestNewProxyHandler 测试创建 ProxyHandler
func TestNewProxyHandler(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewProxyHandler(cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.Config)
	assert.NotNil(t, handler.ProviderFactory)
	assert.NotNil(t, handler.Retryer)
}

// TestNewProxyHandlerWithConfig 测试使用自定义配置创建 ProxyHandler
func TestNewProxyHandlerWithConfig(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewProxyHandler(cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.Config)
}

// TestProxyHandler_Structure 测试 ProxyHandler 结构
func TestProxyHandler_Structure(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewProxyHandler(cfg)

	// 验证结构字段
	assert.NotNil(t, handler.Config)
	assert.NotNil(t, handler.ProviderFactory)
	assert.NotNil(t, handler.Retryer)

	// 验证 Retryer 配置
	assert.NotNil(t, handler.Retryer.Config)
	assert.Equal(t, 3, handler.Retryer.Config.MaxRetries)
	assert.Equal(t, 1*time.Second, handler.Retryer.Config.BackoffMin)
	assert.Equal(t, 30*time.Second, handler.Retryer.Config.BackoffMax)
}
