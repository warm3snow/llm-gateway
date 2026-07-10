package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/handler"
	"github.com/warm3snow/llm-gateway/internal/logging"
	"github.com/warm3snow/llm-gateway/internal/logstore"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/provider"
	_ "github.com/warm3snow/llm-gateway/internal/provider/anthropic"
	_ "github.com/warm3snow/llm-gateway/internal/provider/azure"
	_ "github.com/warm3snow/llm-gateway/internal/provider/cohere"
	_ "github.com/warm3snow/llm-gateway/internal/provider/deepseek"
	_ "github.com/warm3snow/llm-gateway/internal/provider/deterministic"
	_ "github.com/warm3snow/llm-gateway/internal/provider/gemini"
	_ "github.com/warm3snow/llm-gateway/internal/provider/glm"
	_ "github.com/warm3snow/llm-gateway/internal/provider/groq"
	_ "github.com/warm3snow/llm-gateway/internal/provider/kimi"
	_ "github.com/warm3snow/llm-gateway/internal/provider/mistral"
	_ "github.com/warm3snow/llm-gateway/internal/provider/ollama"
	_ "github.com/warm3snow/llm-gateway/internal/provider/openai"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
	"github.com/warm3snow/llm-gateway/pkg/cache"
	"github.com/warm3snow/llm-gateway/pkg/encryption"
	"github.com/warm3snow/llm-gateway/pkg/guardrail"
	"github.com/warm3snow/llm-gateway/pkg/proxy"
)

// startTime tracks when the server process started, for /health uptime.
var startTime = time.Now()

// @title LLM Gateway API
// @version 1.0
// @description Unified API gateway for multiple LLM providers with virtual key management, response caching, and admin dashboard.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// @securityDefinitions.apikey VirtualKeyAuth
// @in header
// @name x-llm-gateway-api-key
// @description Virtual key for API access

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the LLM Gateway HTTP server (default)",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return persistentLoad()
		},
		RunE:         runServe,
		SilenceUsage: true,
	}
	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg := loadedConfig

	// Configure structured logging. JSON unless the operator explicitly
	// selects a "text"/"console" format (handy for local development).
	jsonLogs := cfg.Logging.Format != "text" && cfg.Logging.Format != "console"
	logging.Init(cfg.Logging.Level, jsonLogs)

	// Initialize the encryption key BEFORE any encrypt/decrypt (provider seeding
	// below). A deterministic key is required so ciphertext stored in the DB
	// remains decryptable across restarts. Prefer an explicit 64-hex key; fall
	// back to a key derived from JWTSecret so single-node dev setups still work.
	encKey := cfg.Security.EncryptionKey
	if encKey == "" {
		sum := sha256.Sum256([]byte(cfg.Security.JWTSecret))
		encKey = hex.EncodeToString(sum[:])
		log.Printf("[SECURITY] WARN: security.encryptionKey not set; deriving encryption key from jwtSecret. Set security.encryptionKey (64 hex chars) in production.")
	}
	if err := encryption.InitEncryptionKey(encKey); err != nil {
		return fmt.Errorf("failed to init encryption key: %w", err)
	}

	// Run auto-migration
	migrateErr := database.Migrate(
		&models.Tenant{},
		&models.User{},
		&models.TenantMember{},
		&models.VirtualKey{},
		&models.UsageRecord{},
		&models.ProviderConfig{},
		&models.CacheEntry{},
		&models.ModelPricing{},
	)
	if migrateErr != nil {
		log.Printf("Warning: Failed to run migration: %v", migrateErr)
	}
	defer database.Close()

	// Seed the default tenant and ensure the config admin exists as a
	// super_admin. Safe to run on every startup.
	if err := database.Bootstrap(cfg.Security.AdminUser, cfg.Security.AdminPass); err != nil {
		log.Printf("Warning: Failed to bootstrap tenants/users: %v", err)
	}

	// Seed providers from config.yaml into the DB (WARN+skip on name conflict),
	// then reload the full provider set from the DB into cfg.Gateway.Providers.
	// The DB is the source of truth at runtime; the in-memory map is a cache the
	// proxy reads live (proxyHandler and adminHandler share this cfg pointer).
	seedProvidersFromConfig(cfg)

	// Start the asynchronous usage-record writer. Draining happens during
	// graceful shutdown below so buffered records are flushed before exit.
	logstore.Init(logstore.Options{})

	// 初始化缓存
	var cacheInstance cache.Cache
	if cfg.Cache.Enabled {
		cacheCfg := &cache.Config{
			Type:       cfg.Cache.Type,
			RedisAddr:  cfg.Cache.Redis.Addr,
			RedisPass:  cfg.Cache.Redis.Password,
			RedisDB:    cfg.Cache.Redis.DB,
			MaxEntries: 1000,
			DefaultTTL: cfg.Cache.DefaultTTL,
		}
		var cacheErr error
		cacheInstance, cacheErr = cache.NewCache(cacheCfg)
		if cacheErr != nil {
			return fmt.Errorf("failed to initialize enabled cache: %w", cacheErr)
		}
		log.Printf("[CACHE] Initialized %s cache", cfg.Cache.Type)
	}

	// 设置 Gin 模式
	ginMode := "release"
	if cfg.Server.Mode != "" {
		ginMode = cfg.Server.Mode
	}
	gin.SetMode(ginMode)

	// 创建路由引擎
	router := gin.New()

	// 添加 Swagger 文档
	setupSwagger(router)

	// 添加全局中间件
	router.Use(middleware.Logger())
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS(cfg.Security.AllowedOrigins))

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"version": "1.0.0",
			"uptime":  time.Since(startTime).String(),
		})
	})

	// Prometheus 指标端点
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	if cfg.Server.PprofEnabled {
		registerPprofRoutes(router)
		log.Printf("[PPROF] enabled at /debug/pprof")
	}

	// 日志输出已注册的提供商
	log.Printf("[PROVIDER] Registered providers: %v", provider.GetGlobalFactory().ListProviders())

	// 创建 Guardrail 管理器
	var guardrailConfigs []types.GuardrailConfig
	if cfg.Gateway.DefaultConfig != nil {
		guardrailConfigs = cfg.Gateway.DefaultConfig.Guardrails
	}
	guardrailManager, err := guardrail.NewManagerFromConfig(cfg.Gateway.GuardrailsEnabled, guardrailConfigs)
	if err != nil {
		return fmt.Errorf("failed to initialize guardrails: %w", err)
	}

	// 创建代理处理器
	proxyHandler := proxy.NewProxyHandler(cfg, cacheInstance)
	proxyHandler.SetGuardrailManager(guardrailManager)

	// VirtualKeyService for budget tracking on usage records.
	virtualKeyService := service.NewVirtualKeyService()
	var budgetTracker *service.BudgetTracker
	if cfg.Budget.AsyncEnabled {
		budgetTracker = service.NewBudgetTracker(database.GetDB(), service.BudgetTrackerOptions{
			QueueSize:     cfg.Budget.QueueSize,
			BatchSize:     cfg.Budget.BatchSize,
			FlushInterval: cfg.Budget.FlushInterval,
			FlushTimeout:  cfg.Budget.FlushTimeout,
		})
		budgetTracker.Start()
		log.Printf("[BUDGET] async tracker enabled queue_size=%d batch_size=%d flush_interval=%s", cfg.Budget.QueueSize, cfg.Budget.BatchSize, cfg.Budget.FlushInterval)
	}

	// API 路由（需要虚拟密钥认证 + 缓存 + 用量记录）
	v1 := router.Group("/v1")
	v1.Use(middleware.VirtualKeyAuthWithBudgetTracker(cfg, budgetTracker))
	v1.Use(middleware.GuardrailMiddleware(guardrailManager))
	v1.Use(middleware.UsageRecordMiddlewareWithBudgetTracker(cfg, virtualKeyService, budgetTracker))
	v1.Use(middleware.CacheMiddleware(cacheInstance, cfg.Cache.DefaultTTL, cfg.Gateway.DefaultProvider))
	{
		v1.POST("/chat/completions", proxyHandler.HandleChatCompletion)
		v1.POST("/completions", proxyHandler.HandleCompletion)
		v1.POST("/embeddings", proxyHandler.HandleEmbedding)
		v1.GET("/models", proxyHandler.HandleModels)
		v1.POST("/images/generations", proxyHandler.HandleImageGeneration)
		v1.POST("/audio/speech", proxyHandler.HandleAudioSpeech)
		v1.POST("/audio/transcriptions", proxyHandler.HandleAudioTranscription)
		v1.POST("/audio/translations", proxyHandler.HandleAudioTranslation)
		v1.Any("/proxy/*path", proxyHandler.ProxyRequest)
		v1.POST("/chat/completions/stream", proxyHandler.HandleStreamRequest)
	}

	// 认证路由（不需要JWT）
	authHandler := handler.NewAuthHandler(cfg)
	authHandler.RegisterRoutes(router)

	// JWT 中间件（用于保护管理接口）
	jwtMiddleware := middleware.JWTAuth(cfg)

	// 管理界面路由（需要JWT保护）
	adminHandler := handler.NewHandler(cfg)
	adminHandler.RegisterRoutesWithAuth(router, jwtMiddleware)

	// 统计路由（需要JWT保护）
	statsHandler := handler.NewStatsHandler(cfg)
	statsHandler.RegisterRoutesWithAuth(router, jwtMiddleware)

	// 用量记录路由（需要JWT保护）
	usageHandler := handler.NewUsageHandler()
	usageHandler.RegisterRoutesWithAuth(router, jwtMiddleware)

	// 虚拟密钥路由（需要JWT保护）
	vkHandler := handler.NewVirtualKeyHandler()
	vkHandler.RegisterRoutesWithAuth(router, jwtMiddleware)

	// 租户管理路由（需要JWT + super_admin）
	tenantHandler := handler.NewTenantHandler()
	tenantHandler.RegisterRoutesWithAuth(router, jwtMiddleware)

	// 用户管理路由（需要JWT，按角色授权）
	userHandler := handler.NewUserHandler()
	userHandler.RegisterRoutesWithAuth(router, jwtMiddleware)

	// 创建 HTTP 服务器
	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 启动服务器（非阻塞）
	go func() {
		log.Printf("Starting LLM Gateway on %s:%d", cfg.Server.Host, cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Flush accepted budget usage and buffered usage records before DB close.
	if budgetTracker != nil {
		if err := budgetTracker.Shutdown(ctx); err != nil {
			log.Printf("[BUDGET] WARN: async tracker shutdown incomplete: %v", err)
		}
	}
	logstore.ShutdownDefault(ctx)

	log.Println("Server exited")
	return nil
}

func seedProvidersFromConfig(cfg *config.Config) {
	svc := service.NewProviderConfigService()

	for name, opts := range cfg.Gateway.Providers {
		if _, err := svc.Create(name, opts); err != nil {
			if errors.Is(err, service.ErrProviderAlreadyExists) {
				log.Printf("[PROVIDER] WARN: provider %q already exists in DB, skipping config.yaml seed", name)
				continue
			}
			log.Printf("[PROVIDER] WARN: failed to seed provider %q into DB: %v", name, err)
		} else {
			log.Printf("[PROVIDER] seeded provider %q from config.yaml into DB", name)
		}
	}

	// DB rows win after startup; the proxy reads this map as its runtime cache.
	rows, err := svc.List()
	if err != nil {
		log.Printf("[PROVIDER] WARN: failed to load providers from DB: %v", err)
		return
	}
	m := make(map[string]types.Options, len(rows))
	for i := range rows {
		opts, err := svc.ToOptions(&rows[i])
		if err != nil {
			log.Printf("[PROVIDER] WARN: failed to decode provider %q from DB: %v", rows[i].Name, err)
			continue
		}
		m[rows[i].Name] = opts
	}
	cfg.Gateway.ProvidersMu.Lock()
	cfg.Gateway.Providers = m
	cfg.Gateway.ProvidersMu.Unlock()
	log.Printf("[PROVIDER] loaded %d providers from DB", len(m))
}

func registerPprofRoutes(router *gin.Engine) {
	router.GET("/debug/pprof/", gin.WrapF(pprof.Index))
	router.GET("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	router.GET("/debug/pprof/profile", gin.WrapF(pprof.Profile))
	router.POST("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	router.GET("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	router.GET("/debug/pprof/trace", gin.WrapF(pprof.Trace))
	router.GET("/debug/pprof/allocs", gin.WrapH(pprof.Handler("allocs")))
	router.GET("/debug/pprof/block", gin.WrapH(pprof.Handler("block")))
	router.GET("/debug/pprof/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	router.GET("/debug/pprof/heap", gin.WrapH(pprof.Handler("heap")))
	router.GET("/debug/pprof/mutex", gin.WrapH(pprof.Handler("mutex")))
	router.GET("/debug/pprof/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
}

func setupSwagger(router *gin.Engine) {
	// 读取 swagger.json 文件内容
	swaggerJSON, err := os.ReadFile("docs/swagger.json")
	if err != nil {
		log.Printf("Warning: Failed to read swagger.json: %v", err)
		// 尝试绝对路径
		swaggerJSON, err = os.ReadFile("/Users/hxy/go/src/github.com/warm3snow/llm-gateway/docs/swagger.json")
		if err != nil {
			log.Printf("Error: Failed to read swagger.json with absolute path: %v", err)
			swaggerJSON = []byte("{}")
		} else {
			log.Printf("Successfully read swagger.json with absolute path")
		}
	} else {
		log.Printf("Successfully read swagger.json")
	}

	// 提供自定义的 swagger.json 文件（包含示例）
	router.GET("/custom-swagger/doc.json", func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
		c.String(200, string(swaggerJSON))
	})

	// Swagger UI - 指向自定义文件
	url := ginSwagger.URL("/custom-swagger/doc.json")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))
}
