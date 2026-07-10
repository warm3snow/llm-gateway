package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
	"github.com/warm3snow/llm-gateway/internal/types"
)

var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandEnvInString replaces ${VAR} placeholders with environment variable values.
func expandEnvInString(s string) string {
	return envVarRegex.ReplaceAllStringFunc(s, func(match string) string {
		name := match[2 : len(match)-1] // strip ${ and }
		if val, ok := os.LookupEnv(name); ok {
			return val
		}
		return match // leave unchanged if env var not set
	})
}

// expandEnvInMap recursively expands ${VAR} placeholders in map values.
func expandEnvInMap(m map[string]interface{}) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			m[k] = expandEnvInString(val)
		case map[string]interface{}:
			expandEnvInMap(val)
		}
	}
}

// Config 全局配置
type Config struct {
	Server   ServerConfig   `mapstructure:"server" json:"server" yaml:"server"`
	Gateway  GatewayConfig  `mapstructure:"gateway" json:"gateway" yaml:"gateway"`
	Database DatabaseConfig `mapstructure:"database" json:"database" yaml:"database"`
	Cache    CacheConfig    `mapstructure:"cache" json:"cache" yaml:"cache"`
	Budget   BudgetConfig   `mapstructure:"budget" json:"budget" yaml:"budget"`
	Logging  LoggingConfig  `mapstructure:"logging" json:"logging" yaml:"logging"`
	Security SecurityConfig `mapstructure:"security" json:"security" yaml:"security"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver          string        `mapstructure:"driver" json:"driver" yaml:"driver"`
	DSN             string        `mapstructure:"dsn" json:"dsn" yaml:"dsn"`
	LogLevel        string        `mapstructure:"logLevel" json:"logLevel" yaml:"logLevel"`
	MaxOpenConns    int           `mapstructure:"maxOpenConns" json:"maxOpenConns" yaml:"maxOpenConns"`
	MaxIdleConns    int           `mapstructure:"maxIdleConns" json:"maxIdleConns" yaml:"maxIdleConns"`
	ConnMaxLifetime time.Duration `mapstructure:"connMaxLifetime" json:"connMaxLifetime" yaml:"connMaxLifetime"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host            string        `mapstructure:"host" json:"host" yaml:"host"`
	Port            int           `mapstructure:"port" json:"port" yaml:"port"`
	ReadTimeout     time.Duration `mapstructure:"readTimeout" json:"readTimeout" yaml:"readTimeout"`
	WriteTimeout    time.Duration `mapstructure:"writeTimeout" json:"writeTimeout" yaml:"writeTimeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdownTimeout" json:"shutdownTimeout" yaml:"shutdownTimeout"`
	Mode            string        `mapstructure:"mode" json:"mode" yaml:"mode"`
	PprofEnabled    bool          `mapstructure:"pprofEnabled" json:"pprofEnabled" yaml:"pprofEnabled"`
}

// GatewayConfig 网关配置
type GatewayConfig struct {
	ProvidersMu        sync.RWMutex             `mapstructure:"-" json:"-" yaml:"-"`
	DefaultProvider    string                   `mapstructure:"defaultProvider" json:"defaultProvider" yaml:"defaultProvider"`
	Providers          map[string]types.Options `mapstructure:"providers" json:"providers" yaml:"providers"`
	DefaultConfig      *types.Config            `mapstructure:"defaultConfig" json:"defaultConfig" yaml:"defaultConfig"`
	GuardrailsEnabled  bool                     `mapstructure:"guardrailsEnabled" json:"guardrailsEnabled" yaml:"guardrailsEnabled"`
	MaxRequestTimeout  int                      `mapstructure:"maxRequestTimeout" json:"maxRequestTimeout" yaml:"maxRequestTimeout"`
	SupportedProviders []string                 `mapstructure:"supportedProviders" json:"supportedProviders" yaml:"supportedProviders"`
	AutoMode           AutoModeConfig           `mapstructure:"autoMode" json:"autoMode" yaml:"autoMode"`
}

// AutoModeConfig controls dynamic provider/model selection for chat requests.
type AutoModeConfig struct {
	Enabled                     bool    `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	CostWeight                  float64 `mapstructure:"costWeight" json:"costWeight" yaml:"costWeight"`
	ConcurrencyWeight           float64 `mapstructure:"concurrencyWeight" json:"concurrencyWeight" yaml:"concurrencyWeight"`
	RecentUsageWeight           float64 `mapstructure:"recentUsageWeight" json:"recentUsageWeight" yaml:"recentUsageWeight"`
	ErrorWeight                 float64 `mapstructure:"errorWeight" json:"errorWeight" yaml:"errorWeight"`
	ProviderWeightPenaltyWeight float64 `mapstructure:"providerWeightPenaltyWeight" json:"providerWeightPenaltyWeight" yaml:"providerWeightPenaltyWeight"`
	RecentWindowSeconds         int     `mapstructure:"recentWindowSeconds" json:"recentWindowSeconds" yaml:"recentWindowSeconds"`
	DefaultMaxConcurrency       int     `mapstructure:"defaultMaxConcurrency" json:"defaultMaxConcurrency" yaml:"defaultMaxConcurrency"`
	DefaultOutputTokens         int     `mapstructure:"defaultOutputTokens" json:"defaultOutputTokens" yaml:"defaultOutputTokens"`
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled    bool          `mapstructure:"enabled" json:"enabled" yaml:"enabled"`
	Type       string        `mapstructure:"type" json:"type" yaml:"type"`
	DefaultTTL time.Duration `mapstructure:"defaultTTL" json:"defaultTTL" yaml:"defaultTTL"`
	Redis      RedisConfig   `mapstructure:"redis" json:"redis" yaml:"redis"`
}

// RedisConfig Redis 配置
type RedisConfig struct {
	Addr     string `mapstructure:"addr" json:"addr" yaml:"addr"`
	Password string `mapstructure:"password" json:"password" yaml:"password"`
	DB       int    `mapstructure:"db" json:"db" yaml:"db"`
}

// BudgetConfig controls virtual-key budget accounting.
type BudgetConfig struct {
	AsyncEnabled  bool          `mapstructure:"asyncEnabled" json:"asyncEnabled" yaml:"asyncEnabled"`
	QueueSize     int           `mapstructure:"queueSize" json:"queueSize" yaml:"queueSize"`
	BatchSize     int           `mapstructure:"batchSize" json:"batchSize" yaml:"batchSize"`
	FlushInterval time.Duration `mapstructure:"flushInterval" json:"flushInterval" yaml:"flushInterval"`
	FlushTimeout  time.Duration `mapstructure:"flushTimeout" json:"flushTimeout" yaml:"flushTimeout"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `mapstructure:"level" json:"level" yaml:"level"`
	Format     string `mapstructure:"format" json:"format" yaml:"format"`
	OutputPath string `mapstructure:"outputPath" json:"outputPath" yaml:"outputPath"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	APIKeyHeader     string   `mapstructure:"apiKeyHeader" json:"apiKeyHeader" yaml:"apiKeyHeader"`
	AllowedOrigins   []string `mapstructure:"allowedOrigins" json:"allowedOrigins" yaml:"allowedOrigins"`
	RateLimitEnabled bool     `mapstructure:"rateLimitEnabled" json:"rateLimitEnabled" yaml:"rateLimitEnabled"`
	RateLimit        int      `mapstructure:"rateLimit" json:"rateLimit" yaml:"rateLimit"`
	AdminUser        string   `mapstructure:"adminUser" json:"adminUser" yaml:"adminUser"`
	AdminPass        string   `mapstructure:"adminPass" json:"adminPass" yaml:"adminPass"`
	JWTSecret        string   `mapstructure:"jwtSecret" json:"jwtSecret" yaml:"jwtSecret"`
	// EncryptionKey is a 64-char hex string (32 bytes) used to encrypt sensitive
	// data (e.g. provider API keys) at rest. If empty, a deterministic key is
	// derived from JWTSecret at startup. Set this explicitly in production so
	// stored ciphertext remains decryptable across restarts and secret rotations.
	EncryptionKey string `mapstructure:"encryptionKey" json:"encryptionKey" yaml:"encryptionKey"`
}

// LoadConfig 加载配置
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// 设置默认值
	setDefaults(v)

	// 配置文件
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.AddConfigPath("/etc/llm-gateway")
	}

	// 环境变量
	v.SetEnvPrefix("LLM_GATEWAY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	_ = v.BindEnv("database.maxOpenConns", "LLM_GATEWAY_DATABASE_MAX_OPEN_CONNS")
	_ = v.BindEnv("database.maxIdleConns", "LLM_GATEWAY_DATABASE_MAX_IDLE_CONNS")
	_ = v.BindEnv("database.connMaxLifetime", "LLM_GATEWAY_DATABASE_CONN_MAX_LIFETIME")
	_ = v.BindEnv("budget.asyncEnabled", "LLM_GATEWAY_BUDGET_ASYNC_ENABLED")
	_ = v.BindEnv("budget.queueSize", "LLM_GATEWAY_BUDGET_QUEUE_SIZE")
	_ = v.BindEnv("budget.batchSize", "LLM_GATEWAY_BUDGET_BATCH_SIZE")
	_ = v.BindEnv("budget.flushInterval", "LLM_GATEWAY_BUDGET_FLUSH_INTERVAL")
	_ = v.BindEnv("budget.flushTimeout", "LLM_GATEWAY_BUDGET_FLUSH_TIMEOUT")

	// 读取配置
	if err := v.ReadInConfig(); err != nil {
		// 任何错误都使用默认值
		fmt.Println("Config file not found or failed to read, using defaults and environment variables:", err)
	}

	// 解析配置
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand ${ENV_VAR} placeholders in provider API keys and other string fields
	for name, opts := range cfg.Gateway.Providers {
		if opts.APIKey != "" {
			opts.APIKey = expandEnvInString(opts.APIKey)
		}
		if opts.CustomHost != "" {
			opts.CustomHost = expandEnvInString(opts.CustomHost)
		}
		cfg.Gateway.Providers[name] = opts
	}

	// 验证配置
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// setDefaults 设置默认配置
func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.readTimeout", 60*time.Second)
	v.SetDefault("server.writeTimeout", 60*time.Second)
	v.SetDefault("server.shutdownTimeout", 30*time.Second)
	v.SetDefault("server.mode", "release")
	v.SetDefault("server.pprofEnabled", false)

	v.SetDefault("gateway.defaultProvider", "openai")
	v.SetDefault("gateway.guardrailsEnabled", true)
	v.SetDefault("gateway.maxRequestTimeout", 120000)
	v.SetDefault("gateway.autoMode.enabled", true)
	v.SetDefault("gateway.autoMode.costWeight", 0.50)
	v.SetDefault("gateway.autoMode.concurrencyWeight", 0.25)
	v.SetDefault("gateway.autoMode.recentUsageWeight", 0.15)
	v.SetDefault("gateway.autoMode.errorWeight", 0.05)
	v.SetDefault("gateway.autoMode.providerWeightPenaltyWeight", 0.05)
	v.SetDefault("gateway.autoMode.recentWindowSeconds", 300)
	v.SetDefault("gateway.autoMode.defaultMaxConcurrency", 20)
	v.SetDefault("gateway.autoMode.defaultOutputTokens", 1024)
	v.SetDefault("gateway.supportedProviders", []string{
		"openai", "anthropic", "google", "azure-openai", "cohere",
		"mistral-ai", "together-ai", "ollama", "groq", "deepseek",
	})

	v.SetDefault("cache.enabled", false)
	v.SetDefault("cache.type", "memory")
	v.SetDefault("cache.defaultTTL", 5*time.Minute)
	v.SetDefault("cache.redis.addr", "localhost:6379")
	v.SetDefault("cache.redis.password", "")
	v.SetDefault("cache.redis.db", 0)

	v.SetDefault("budget.asyncEnabled", true)
	v.SetDefault("budget.queueSize", 10000)
	v.SetDefault("budget.batchSize", 500)
	v.SetDefault("budget.flushInterval", 250*time.Millisecond)
	v.SetDefault("budget.flushTimeout", 5*time.Second)

	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.outputPath", "stdout")

	v.SetDefault("security.apiKeyHeader", "x-llm-gateway-api-key")
	v.SetDefault("security.allowedOrigins", []string{"*"})
	v.SetDefault("security.rateLimitEnabled", false)
	v.SetDefault("security.rateLimit", 100)
	v.SetDefault("security.adminUser", "admin")
	v.SetDefault("security.adminPass", "admin123")
	v.SetDefault("security.jwtSecret", "llm-gateway-secret-change-in-production")
	v.SetDefault("security.encryptionKey", "")

	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "llm-gateway.db")
	v.SetDefault("database.logLevel", "warn")
	v.SetDefault("database.maxOpenConns", 50)
	v.SetDefault("database.maxIdleConns", 25)
	v.SetDefault("database.connMaxLifetime", 30*time.Minute)
}

// validateConfig 验证配置
func validateConfig(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Gateway.MaxRequestTimeout < 0 {
		return fmt.Errorf("invalid max request timeout: %d", cfg.Gateway.MaxRequestTimeout)
	}

	if cfg.Database.MaxOpenConns < 0 {
		return fmt.Errorf("invalid database maxOpenConns: %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns < 0 {
		return fmt.Errorf("invalid database maxIdleConns: %d", cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetime < 0 {
		return fmt.Errorf("invalid database connMaxLifetime: %s", cfg.Database.ConnMaxLifetime)
	}
	if cfg.Budget.QueueSize < 0 {
		return fmt.Errorf("invalid budget queueSize: %d", cfg.Budget.QueueSize)
	}
	if cfg.Budget.BatchSize < 0 {
		return fmt.Errorf("invalid budget batchSize: %d", cfg.Budget.BatchSize)
	}
	if cfg.Budget.FlushInterval < 0 {
		return fmt.Errorf("invalid budget flushInterval: %s", cfg.Budget.FlushInterval)
	}
	if cfg.Budget.FlushTimeout < 0 {
		return fmt.Errorf("invalid budget flushTimeout: %s", cfg.Budget.FlushTimeout)
	}

	return nil
}

// SaveConfig 保存配置到文件
func SaveConfig(cfg *Config, path string) error {
	v := viper.New()

	v.Set("server", cfg.Server)
	v.Set("gateway", cfg.Gateway)
	v.Set("cache", cfg.Cache)
	v.Set("budget", cfg.Budget)
	v.Set("database", cfg.Database)
	v.Set("logging", cfg.Logging)
	v.Set("security", cfg.Security)

	if err := v.WriteConfigAs(path); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// GetConfigPath 获取配置文件路径
func GetConfigPath() string {
	// 优先级: 环境变量 > 默认路径
	if path := os.Getenv("LLM_GATEWAY_CONFIG_PATH"); path != "" {
		return path
	}
	return "configs/config.yaml"
}
