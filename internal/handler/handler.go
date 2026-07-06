package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// Handler 管理界面处理器
type Handler struct {
	Config      *config.Config
	providerSvc *service.ProviderConfigService
}

// NewHandler 创建处理器
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{Config: cfg, providerSvc: service.NewProviderConfigService()}
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
		api.PUT("/providers/:name", h.UpdateProvider)
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

type providerRequest struct {
	Name string `json:"name"`
	types.Options
}

func providerResponse(name string, opts types.Options, enabled bool) ProviderResponse {
	return ProviderResponse{
		Name:           name,
		Provider:       opts.Provider,
		APIKey:         maskAPIKey(opts.APIKey),
		CustomHost:     opts.CustomHost,
		Weight:         opts.Weight,
		Enabled:        enabled,
		RequestTimeout: opts.RequestTimeout,
	}
}

func (h *Handler) syncProviderInMemory(name string, opts types.Options) {
	h.Config.Gateway.ProvidersMu.Lock()
	defer h.Config.Gateway.ProvidersMu.Unlock()
	if h.Config.Gateway.Providers == nil {
		h.Config.Gateway.Providers = make(map[string]types.Options)
	}
	h.Config.Gateway.Providers[name] = opts
}

func (h *Handler) providerResponsesFromMemory() []ProviderResponse {
	h.Config.Gateway.ProvidersMu.RLock()
	defer h.Config.Gateway.ProvidersMu.RUnlock()
	providers := make([]ProviderResponse, 0, len(h.Config.Gateway.Providers))
	for name, opts := range h.Config.Gateway.Providers {
		providers = append(providers, providerResponse(name, opts, true))
	}
	return providers
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
	rows, err := h.providerSvc.List()
	if err != nil {
		if errors.Is(err, service.ErrProviderStoreUnavailable) {
			c.JSON(http.StatusOK, gin.H{
				"data":   h.providerResponsesFromMemory(),
				"status": "success",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to list providers: %v", err),
			"status": "error",
		})
		return
	}

	providers := make([]ProviderResponse, 0, len(rows))
	for i := range rows {
		opts, err := h.providerSvc.ToOptions(&rows[i])
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  fmt.Sprintf("Failed to decode provider '%s': %v", rows[i].Name, err),
				"status": "error",
			})
			return
		}
		providers = append(providers, providerResponse(rows[i].Name, opts, rows[i].Enabled))
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
	var req providerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  fmt.Sprintf("Invalid request: %v", err),
			"status": "error",
		})
		return
	}
	if req.Name == "" || req.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "name and provider are required",
			"status": "error",
		})
		return
	}

	opts := req.Options
	created, err := h.providerSvc.Create(req.Name, opts)
	if err != nil {
		if errors.Is(err, service.ErrProviderAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{
				"error":  fmt.Sprintf("Provider '%s' already exists", req.Name),
				"status": "error",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to create provider: %v", err),
			"status": "error",
		})
		return
	}

	runtimeOpts, err := h.providerSvc.ToOptions(created)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to decode provider: %v", err),
			"status": "error",
		})
		return
	}
	h.syncProviderInMemory(req.Name, runtimeOpts)

	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("Provider '%s' added successfully", req.Name),
		"status":   "success",
		"provider": providerResponse(req.Name, runtimeOpts, created.Enabled),
	})
}

// UpdateProvider updates an existing provider configuration.
// PUT /api/v1/admin/providers/:name
func (h *Handler) UpdateProvider(c *gin.Context) {
	name := c.Param("name")
	var req providerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  fmt.Sprintf("Invalid request: %v", err),
			"status": "error",
		})
		return
	}
	if req.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "provider is required",
			"status": "error",
		})
		return
	}

	opts := req.Options
	updated, err := h.providerSvc.Update(name, opts)
	if err != nil {
		if errors.Is(err, service.ErrProviderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":  fmt.Sprintf("Provider '%s' not found", name),
				"status": "error",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to update provider: %v", err),
			"status": "error",
		})
		return
	}

	// Empty apiKey means "keep existing" on update. Re-decode DB row so the
	// runtime cache gets the existing decrypted key instead of an empty key.
	runtimeOpts, err := h.providerSvc.ToOptions(updated)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to decode provider: %v", err),
			"status": "error",
		})
		return
	}
	h.syncProviderInMemory(name, runtimeOpts)

	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("Provider '%s' updated successfully", name),
		"status":   "success",
		"provider": providerResponse(name, runtimeOpts, updated.Enabled),
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

	if err := h.providerSvc.Delete(name); err != nil {
		if errors.Is(err, service.ErrProviderStoreUnavailable) {
			h.Config.Gateway.ProvidersMu.RLock()
			_, exists := h.Config.Gateway.Providers[name]
			h.Config.Gateway.ProvidersMu.RUnlock()
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{
					"error":  fmt.Sprintf("Provider '%s' not found", name),
					"status": "error",
				})
				return
			}
		} else if errors.Is(err, service.ErrProviderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":  fmt.Sprintf("Provider '%s' not found", name),
				"status": "error",
			})
			return
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":  fmt.Sprintf("Failed to remove provider: %v", err),
				"status": "error",
			})
			return
		}
	}

	h.Config.Gateway.ProvidersMu.Lock()
	delete(h.Config.Gateway.Providers, name)
	h.Config.Gateway.ProvidersMu.Unlock()

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
	// Legacy endpoint — superseded by /api/v1/stats/overview. Derive real
	// figures from usage records instead of returning canned numbers.
	overview, err := service.NewStatsService().GetOverview(middleware.EffectiveTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to compute stats: %v", err),
			"status": "error",
		})
		return
	}

	providersCount := overview.ActiveProviders
	if providersCount == 0 {
		h.Config.Gateway.ProvidersMu.RLock()
		providersCount = len(h.Config.Gateway.Providers)
		h.Config.Gateway.ProvidersMu.RUnlock()
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": gin.H{
			"totalRequests":      overview.TotalRequests,
			"successfulRequests": int64(float64(overview.TotalRequests) * overview.SuccessRate / 100),
			"failedRequests":     overview.TotalRequests - int64(float64(overview.TotalRequests)*overview.SuccessRate/100),
			"totalTokens":        overview.TotalTokens,
			"totalCost":          overview.TotalCost,
			"successRate":        overview.SuccessRate,
			"providersCount":     providersCount,
		},
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
