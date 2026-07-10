package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// TestLoadConfig_Defaults 测试加载默认配置
func TestLoadConfig_Defaults(t *testing.T) {
	// 使用不存在的配置文件，应该返回默认配置
	configPath := "nonexistent.yaml"

	cfg, err := LoadConfig(configPath)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// 检查默认值
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 60*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, "openai", cfg.Gateway.DefaultProvider)
	assert.False(t, cfg.Cache.Enabled)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, 50, cfg.Database.MaxOpenConns)
	assert.Equal(t, 25, cfg.Database.MaxIdleConns)
	assert.Equal(t, 30*time.Minute, cfg.Database.ConnMaxLifetime)
	assert.True(t, cfg.Budget.AsyncEnabled)
	assert.Equal(t, 10000, cfg.Budget.QueueSize)
	assert.Equal(t, 500, cfg.Budget.BatchSize)
	assert.Equal(t, 250*time.Millisecond, cfg.Budget.FlushInterval)
	assert.Equal(t, 5*time.Second, cfg.Budget.FlushTimeout)
}

// TestLoadConfig_FromFile 测试从文件加载配置
func TestLoadConfig_FromFile(t *testing.T) {
	// 创建临时配置文件
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.yaml")

	configContent := `
server:
  host: "127.0.0.1"
  port: 9090
gateway:
  defaultProvider: "anthropic"
  maxRequestTimeout: 60000
cache:
  enabled: true
  type: "redis"
logging:
  level: "debug"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)

	cfg, err := LoadConfig(configPath)

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "anthropic", cfg.Gateway.DefaultProvider)
	assert.Equal(t, 60000, cfg.Gateway.MaxRequestTimeout)
	assert.True(t, cfg.Cache.Enabled)
	assert.Equal(t, "redis", cfg.Cache.Type)
	assert.Equal(t, "debug", cfg.Logging.Level)
}

// TestLoadConfig_InvalidPort 测试无效端口
func TestLoadConfig_NestedEnvironmentOverrides(t *testing.T) {
	t.Setenv("LLM_GATEWAY_DATABASE_DRIVER", "postgres")
	t.Setenv("LLM_GATEWAY_DATABASE_DSN", "host=postgres user=llm_gateway dbname=llm_gateway sslmode=disable")
	t.Setenv("LLM_GATEWAY_DATABASE_MAX_OPEN_CONNS", "12")
	t.Setenv("LLM_GATEWAY_DATABASE_MAX_IDLE_CONNS", "6")
	t.Setenv("LLM_GATEWAY_DATABASE_CONN_MAX_LIFETIME", "2m")
	t.Setenv("LLM_GATEWAY_BUDGET_ASYNC_ENABLED", "false")
	t.Setenv("LLM_GATEWAY_BUDGET_QUEUE_SIZE", "123")
	t.Setenv("LLM_GATEWAY_BUDGET_BATCH_SIZE", "45")
	t.Setenv("LLM_GATEWAY_BUDGET_FLUSH_INTERVAL", "750ms")
	t.Setenv("LLM_GATEWAY_BUDGET_FLUSH_TIMEOUT", "3s")
	t.Setenv("LLM_GATEWAY_CACHE_REDIS_ADDR", "redis:6379")

	cfg, err := LoadConfig("nonexistent.yaml")

	assert.NoError(t, err)
	assert.Equal(t, "postgres", cfg.Database.Driver)
	assert.Equal(t, "host=postgres user=llm_gateway dbname=llm_gateway sslmode=disable", cfg.Database.DSN)
	assert.Equal(t, 12, cfg.Database.MaxOpenConns)
	assert.Equal(t, 6, cfg.Database.MaxIdleConns)
	assert.Equal(t, 2*time.Minute, cfg.Database.ConnMaxLifetime)
	assert.False(t, cfg.Budget.AsyncEnabled)
	assert.Equal(t, 123, cfg.Budget.QueueSize)
	assert.Equal(t, 45, cfg.Budget.BatchSize)
	assert.Equal(t, 750*time.Millisecond, cfg.Budget.FlushInterval)
	assert.Equal(t, 3*time.Second, cfg.Budget.FlushTimeout)
	assert.Equal(t, "redis:6379", cfg.Cache.Redis.Addr)
}

func TestLoadConfig_InvalidPort(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid_config.yaml")

	configContent := `
server:
  port: 99999
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	assert.NoError(t, err)

	cfg, err := LoadConfig(configPath)

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "invalid server port")
}

// TestValidateConfig 测试配置验证
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "Valid config",
			config: &Config{
				Server: ServerConfig{
					Host:        "0.0.0.0",
					Port:        8080,
					ReadTimeout: 30 * time.Second,
				},
				Gateway: GatewayConfig{
					MaxRequestTimeout: 120000,
				},
			},
			expectError: false,
		},
		{
			name: "Invalid port - negative",
			config: &Config{
				Server: ServerConfig{
					Port: -1,
				},
			},
			expectError: true,
		},
		{
			name: "Invalid port - too large",
			config: &Config{
				Server: ServerConfig{
					Port: 70000,
				},
			},
			expectError: true,
		},
		{
			name: "Negative timeout",
			config: &Config{
				Server: ServerConfig{
					Port: 8080,
				},
				Gateway: GatewayConfig{
					MaxRequestTimeout: -1000,
				},
			},
			expectError: true,
		},
		{
			name: "Negative database max open connections",
			config: &Config{
				Server: ServerConfig{Port: 8080},
				Gateway: GatewayConfig{
					MaxRequestTimeout: 120000,
				},
				Database: DatabaseConfig{MaxOpenConns: -1},
			},
			expectError: true,
		},
		{
			name: "Negative budget queue size",
			config: &Config{
				Server:  ServerConfig{Port: 8080},
				Gateway: GatewayConfig{MaxRequestTimeout: 120000},
				Budget:  BudgetConfig{QueueSize: -1},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSetDefaults 测试默认值设置
func TestSetDefaults(t *testing.T) {
	v := viper.New()
	setDefaults(v)

	// 检查默认值
	assert.Equal(t, "0.0.0.0", v.GetString("server.host"))
	assert.Equal(t, 8080, v.GetInt("server.port"))
	assert.Equal(t, 60*time.Second, v.GetDuration("server.readTimeout"))
	assert.Equal(t, "openai", v.GetString("gateway.defaultProvider"))
	assert.False(t, v.GetBool("cache.enabled"))
	assert.Equal(t, "info", v.GetString("logging.level"))
	assert.Equal(t, 50, v.GetInt("database.maxOpenConns"))
	assert.Equal(t, 25, v.GetInt("database.maxIdleConns"))
	assert.Equal(t, 30*time.Minute, v.GetDuration("database.connMaxLifetime"))
	assert.True(t, v.GetBool("budget.asyncEnabled"))
	assert.Equal(t, 10000, v.GetInt("budget.queueSize"))
	assert.Equal(t, 500, v.GetInt("budget.batchSize"))
	assert.Equal(t, 250*time.Millisecond, v.GetDuration("budget.flushInterval"))
	assert.Equal(t, 5*time.Second, v.GetDuration("budget.flushTimeout"))
}

// TestSaveConfig 测试保存配置
func TestSaveConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "save_test.yaml")

	cfg := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 3000,
		},
		Gateway: GatewayConfig{
			DefaultProvider: "ollama",
		},
		Cache: CacheConfig{
			Enabled: true,
			Type:    "memory",
		},
	}

	err := SaveConfig(cfg, configPath)
	assert.NoError(t, err)

	// 验证文件已创建
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// 重新加载并验证
	loadedCfg, err := LoadConfig(configPath)
	assert.NoError(t, err)
	assert.Equal(t, "localhost", loadedCfg.Server.Host)
	assert.Equal(t, 3000, loadedCfg.Server.Port)
	assert.Equal(t, "ollama", loadedCfg.Gateway.DefaultProvider)
}

// TestGetConfigPath 测试获取配置路径
func TestGetConfigPath(t *testing.T) {
	// 测试环境变量
	t.Run("From environment variable", func(t *testing.T) {
		os.Setenv("LLM_GATEWAY_CONFIG_PATH", "/custom/path/config.yaml")
		defer os.Unsetenv("LLM_GATEWAY_CONFIG_PATH")

		path := GetConfigPath()
		assert.Equal(t, "/custom/path/config.yaml", path)
	})

	// 测试默认值
	t.Run("Default path", func(t *testing.T) {
		os.Unsetenv("LLM_GATEWAY_CONFIG_PATH")

		path := GetConfigPath()
		assert.Equal(t, "configs/config.yaml", path)
	})
}

// BenchmarkLoadConfig 性能测试
func BenchmarkLoadConfig(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LoadConfig("nonexistent.yaml")
	}
}
