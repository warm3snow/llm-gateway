package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/handler"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/pkg/proxy"
)

func main() {
	// 加载配置
	configPath := config.GetConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config: %v, using defaults", err)
		// 使用默认配置
		cfg = &config.Config{
			Server: config.ServerConfig{
				Host:         "0.0.0.0",
				Port:         8080,
				ReadTimeout:  60 * time.Second,
				WriteTimeout: 60 * time.Second,
			},
			Gateway: config.GatewayConfig{
				DefaultProvider:   "openai",
				GuardrailsEnabled: true,
			},
		}
	}

	// 设置 Gin 模式
	ginMode := "release"
	if cfg.Server.Mode != "" {
		ginMode = cfg.Server.Mode
	}
	gin.SetMode(ginMode)

	// 创建路由引擎
	router := gin.New()

	// 添加中间件
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

	// 创建代理处理器
	proxyHandler := proxy.NewProxyHandler(cfg)

	// API 路由
	v1 := router.Group("/v1")
	{
		// 聊天补全
		v1.POST("/chat/completions", proxyHandler.HandleChatCompletion)

		// 文本补全
		v1.POST("/completions", proxyHandler.HandleCompletion)

		// 嵌入
		v1.POST("/embeddings", proxyHandler.HandleEmbedding)

		// 模型列表
		v1.GET("/models", proxyHandler.HandleModels)

		// 图像生成
		v1.POST("/images/generations", proxyHandler.HandleImageGeneration)

		// 音频处理
		v1.POST("/audio/speech", proxyHandler.HandleAudioSpeech)
		v1.POST("/audio/transcriptions", proxyHandler.HandleAudioTranscription)
		v1.POST("/audio/translations", proxyHandler.HandleAudioTranslation)

		// 代理端点
		v1.Any("/proxy/*path", proxyHandler.ProxyRequest)

		// 流式请求
		v1.POST("/chat/completions/stream", proxyHandler.HandleStreamRequest)
	}

	// 管理界面路由
	adminHandler := handler.NewHandler(cfg)
	adminHandler.RegisterRoutes(router)

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

	log.Println("Server exited")
}

var startTime = time.Now()
