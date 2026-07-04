package config

import (
	"fmt"
	"os"
	"regexp"
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
	Logging  LoggingConfig  `mapstructure:"logging" json:"logging" yaml:"logging"`
	Security SecurityConfig `mapstructure:"security" json:"security" yaml:"security"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Driver   string `mapstructure:"driver" json:"driver" yaml:"driver"`
	DSN      string `mapstructure:"dsn" json:"dsn" yaml:"dsn"`
	LogLevel string `mapstructure:"logLevel" json:"logLevel" yaml:"logLevel"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host            string        `mapstructure:"host" json:"host" yaml:"host"`
	Port            int           `mapstructure:"port" json:"port" yaml:"port"`
	ReadTimeout     time.Duration `mapstructure:"readTimeout" json:"readTimeout" yaml:"readTimeout"`
	WriteTimeout    time.Duration `mapstructure:"writeTimeout" json:"writeTimeout" yaml:"writeTimeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdownTimeout" json:"shutdownTimeout" yaml:"shutdownTimeout"`
	Mode            string        `mapstructure:"mode" json:"mode" yaml:"mode"`
}

// GatewayConfig 网关配置
type GatewayConfig struct {
	DefaultProvider    string                   `mapstructure:"defaultProvider" json:"defaultProvider" yaml:"defaultProvider"`
	Providers          map[string]types.Options `mapstructure:"providers" json:"providers" yaml:"providers"`
	DefaultConfig      *types.Config            `mapstructure:"defaultConfig" json:"defaultConfig" yaml:"defaultConfig"`
	GuardrailsEnabled  bool                     `mapstructure:"guardrailsEnabled" json:"guardrailsEnabled" yaml:"guardrailsEnabled"`
	MaxRequestTimeout  int                      `mapstructure:"maxRequestTimeout" json:"maxRequestTimeout" yaml:"maxRequestTimeout"`
	SupportedProviders []string                 `mapstructure:"supportedProviders" json:"supportedProviders" yaml:"supportedProviders"`
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
	v.AutomaticEnv()

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

	v.SetDefault("gateway.defaultProvider", "openai")
	v.SetDefault("gateway.guardrailsEnabled", true)
	v.SetDefault("gateway.maxRequestTimeout", 120000)
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

	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "llm-gateway.db")
	v.SetDefault("database.logLevel", "warn")
}

// validateConfig 验证配置
func validateConfig(cfg *Config) error {
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", cfg.Server.Port)
	}

	if cfg.Gateway.MaxRequestTimeout < 0 {
		return fmt.Errorf("invalid max request timeout: %d", cfg.Gateway.MaxRequestTimeout)
	}

	return nil
}

// SaveConfig 保存配置到文件
func SaveConfig(cfg *Config, path string) error {
	v := viper.New()

	v.Set("server", cfg.Server)
	v.Set("gateway", cfg.Gateway)
	v.Set("cache", cfg.Cache)
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
