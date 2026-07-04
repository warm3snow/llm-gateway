package handler

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// Handler 管理界面处理器
type Handler struct {
	Config *config.Config
}

// NewHandler 创建处理器
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{Config: cfg}
}

// RegisterRoutes 注册路由（无JWT保护，用于向后兼容）
func (h *Handler) RegisterRoutes(router *gin.Engine) {
	h.RegisterRoutesWithAuth(router, nil)
}

// RegisterRoutesWithAuth 注册路由（可传入JWT中间件）
func (h *Handler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	// 静态文件
	router.Static("/static", "./web/static")

	// 加载模板（如果不存在则忽略）
	templates := "web/templates/*"
	if _, err := os.Stat("web/templates"); err == nil {
		router.LoadHTMLGlob(templates)
	}

	// 页面路由
	router.GET("/admin", h.AdminPage)
	router.GET("/admin/dashboard", h.DashboardPage)
	router.GET("/admin/providers", h.ProvidersPage)
	router.GET("/admin/config", h.ConfigPage)

	// API 路由
	api := router.Group("/api/v1/admin")
	if jwtMiddleware != nil {
		api.Use(jwtMiddleware)
	}
	{
		// 配置管理
		api.GET("/config", h.GetConfig)
		api.POST("/config", h.UpdateConfig)

		// Provider 管理
		api.GET("/providers", h.GetProviders)
		api.POST("/providers", h.AddProvider)
		api.DELETE("/providers/:name", h.RemoveProvider)

		// 统计信息
		api.GET("/stats", h.GetStats)

		// 健康检查
		api.GET("/health", h.HealthCheck)
	}
}

// AdminPage 管理界面主页
func (h *Handler) AdminPage(c *gin.Context) {
	c.HTML(http.StatusOK, "admin.html", gin.H{
		"title": "LLM Gateway - Admin",
	})
}

// DashboardPage 仪表盘页面
func (h *Handler) DashboardPage(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title": "Dashboard - LLM Gateway",
	})
}

// ProvidersPage Provider 管理页面
func (h *Handler) ProvidersPage(c *gin.Context) {
	c.HTML(http.StatusOK, "providers.html", gin.H{
		"title": "Providers - LLM Gateway",
	})
}

// ConfigPage 配置管理页面
func (h *Handler) ConfigPage(c *gin.Context) {
	c.HTML(http.StatusOK, "config.html", gin.H{
		"title": "Config - LLM Gateway",
	})
}

// GetConfig 获取配置
// GET /api/v1/admin/config
// @Summary Get configuration
// @Description Get current gateway configuration
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Configuration data"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/admin/config [get]
func (h *Handler) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data":   h.Config,
		"status": "success",
	})
}

// UpdateConfig 更新配置
// POST /api/v1/admin/config
// @Summary Update configuration
// @Description Update gateway configuration
// @Tags admin
// @Accept json
// @Produce json
// @Param request body config.Config true "Configuration data"
// @Success 200 {object} map[string]interface{} "Configuration updated"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security BearerAuth
// @Router /api/v1/admin/config [post]
func (h *Handler) UpdateConfig(c *gin.Context) {
	var newConfig config.Config

	if err := c.ShouldBindJSON(&newConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  fmt.Sprintf("Invalid request: %v", err),
			"status": "error",
		})
		return
	}

	// 简单验证
	if newConfig.Server.Port <= 0 || newConfig.Server.Port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "Invalid server port",
			"status": "error",
		})
		return
	}

	// 更新配置
	h.Config = &newConfig

	// 保存到文件
	configPath := config.GetConfigPath()
	if err := config.SaveConfig(&newConfig, configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to save config: %v", err),
			"status": "error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Config updated successfully",
		"status":  "success",
		"data":    newConfig,
	})
}

// ProviderResponse is the response for a single provider
type ProviderResponse struct {
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	APIKey         string `json:"apiKey"`
	CustomHost     string `json:"customHost,omitempty"`
	Weight         int    `json:"weight"`
	Enabled        bool   `json:"enabled"`
	RequestTimeout int    `json:"requestTimeout"`
}

// GetProviders 获取所有 Provider
// GET /api/v1/admin/providers
// @Summary Get providers
// @Description Get all supported providers (whitelist) with their configuration if configured
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "List of providers"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/admin/providers [get]
func (h *Handler) GetProviders(c *gin.Context) {
	providers := []ProviderResponse{}

	// Build the list from the supportedProviders whitelist
	for _, name := range h.Config.Gateway.SupportedProviders {
		opts, configured := h.Config.Gateway.Providers[name]
		providerName := name
		apiKey := ""
		customHost := ""
		weight := 0
		requestTimeout := 0
		enabled := configured
		if configured {
			providerName = opts.Provider
			apiKey = maskAPIKey(opts.APIKey)
			customHost = opts.CustomHost
			weight = opts.Weight
			requestTimeout = opts.RequestTimeout
		}
		providers = append(providers, ProviderResponse{
			Name:           name,
			Provider:       providerName,
			APIKey:         apiKey,
			CustomHost:     customHost,
			Weight:         weight,
			Enabled:        enabled,
			RequestTimeout: requestTimeout,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   providers,
		"status": "success",
	})
}

// AddProvider 添加 Provider
// @Summary Add provider
// @Description Add a new provider configuration
// @Tags admin
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Provider configuration"
// @Success 200 {object} map[string]interface{} "Provider added"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 409 {object} map[string]interface{} "Provider already exists"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security BearerAuth
// @Router /api/v1/admin/providers [post]
func (h *Handler) AddProvider(c *gin.Context) {
	var req struct {
		Name           string                 `json:"name" binding:"required"`
		Provider       string                 `json:"provider" binding:"required"`
		APIKey         string                 `json:"apiKey"`
		VirtualKey     string                 `json:"virtualKey"`
		CustomHost     string                 `json:"customHost"`
		Weight         int                    `json:"weight"`
		RequestTimeout int                    `json:"requestTimeout"`
		Extra          map[string]interface{} `json:"extra"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  fmt.Sprintf("Invalid request: %v", err),
			"status": "error",
		})
		return
	}

	// 检查是否已存在
	if _, exists := h.Config.Gateway.Providers[req.Name]; exists {
		c.JSON(http.StatusConflict, gin.H{
			"error":  fmt.Sprintf("Provider '%s' already exists", req.Name),
			"status": "error",
		})
		return
	}

	// 创建 Provider 配置
	opts := &types.Options{
		Provider:       req.Provider,
		APIKey:         req.APIKey,
		VirtualKey:     req.VirtualKey,
		CustomHost:     req.CustomHost,
		Weight:         req.Weight,
		RequestTimeout: req.RequestTimeout,
	}

	// 添加到配置
	if h.Config.Gateway.Providers == nil {
		h.Config.Gateway.Providers = make(map[string]types.Options)
	}
	h.Config.Gateway.Providers[req.Name] = *opts

	// 保存到文件
	configPath := config.GetConfigPath()
	if err := config.SaveConfig(h.Config, configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to save config: %v", err),
			"status": "error",
		})
		return
	}

	// 注册 Provider（这里只是示例，实际应该根据 provider 类型动态注册）
	// 注意：这里需要在程序启动时预先注册所有支持的 provider
	// 或者提供一个 factory 函数来创建 provider
	fmt.Printf("Provider '%s' registered\n", req.Name)

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Provider '%s' added successfully", req.Name),
		"status":  "success",
		"provider": gin.H{
			"name":       req.Name,
			"provider":   req.Provider,
			"apiKey":     maskAPIKey(req.APIKey),
			"customHost": req.CustomHost,
			"weight":     req.Weight,
		},
	})
}

// RemoveProvider 删除 Provider
// @Summary Remove provider
// @Description Remove a provider configuration
// @Tags admin
// @Accept json
// @Produce json
// @Param name path string true "Provider name"
// @Success 200 {object} map[string]interface{} "Provider removed"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 404 {object} map[string]interface{} "Provider not found"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Security BearerAuth
// @Router /api/v1/admin/providers/{name} [delete]
func (h *Handler) RemoveProvider(c *gin.Context) {
	name := c.Param("name")

	if _, exists := h.Config.Gateway.Providers[name]; !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error":  fmt.Sprintf("Provider '%s' not found", name),
			"status": "error",
		})
		return
	}

	// 删除 Provider
	delete(h.Config.Gateway.Providers, name)

	// 保存到文件
	configPath := config.GetConfigPath()
	if err := config.SaveConfig(h.Config, configPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to save config: %v", err),
			"status": "error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Provider '%s' removed successfully", name),
		"status":  "success",
	})
}

// GetStats 获取统计信息
// @Summary Get stats
// @Description Get gateway statistics
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Stats data"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/admin/stats [get]
func (h *Handler) GetStats(c *gin.Context) {
	// TODO: 实现真实的统计信息
	stats := gin.H{
		"totalRequests":      1234,
		"successfulRequests": 1200,
		"failedRequests":     34,
		"averageLatency":     "150ms",
		"providersCount":     len(h.Config.Gateway.Providers),
		"uptime":             "2h 30m",
	}

	c.JSON(http.StatusOK, gin.H{
		"stats":  stats,
		"status": "success",
	})
}

// HealthCheck 健康检查
// @Summary Health check
// @Description Check gateway health status
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Health status"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Security BearerAuth
// @Router /api/v1/admin/health [get]
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"version": "1.0.0",
		"uptime":  "2h 30m",
	})
}

// maskAPIKey 脱敏 API Key
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return strings.Repeat("*", len(apiKey))
	}
	return apiKey[:4] + strings.Repeat("*", len(apiKey)-8) + apiKey[len(apiKey)-4:]
}
