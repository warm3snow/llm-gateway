package handler

import (
	"errors"
	"fmt"
	"net/http"
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

// RegisterRoutesWithAuth registers admin API routes (JWT-protected when jwtMiddleware is given).
func (h *Handler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
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

		// 健康检查
		api.GET("/health", h.HealthCheck)
	}
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
	if !middleware.IsSuperAdmin(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "Only super_admin can view configuration",
			Type:    "forbidden",
		})
		return
	}

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
	if !middleware.IsSuperAdmin(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "Only super_admin can update configuration",
			Type:    "forbidden",
		})
		return
	}

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

	// 更新配置：保留运行中的 ProvidersMu（其上可能有等待的锁）与 DB 来源的
	// Providers 映射，仅替换其余配置块。
	h.Config.Server = newConfig.Server
	h.Config.Cache = newConfig.Cache
	h.Config.Budget = newConfig.Budget
	h.Config.Database = newConfig.Database
	h.Config.Logging = newConfig.Logging
	h.Config.Security = newConfig.Security
	h.Config.Gateway.DefaultProvider = newConfig.Gateway.DefaultProvider
	h.Config.Gateway.MaxRequestTimeout = newConfig.Gateway.MaxRequestTimeout
	h.Config.Gateway.GuardrailsEnabled = newConfig.Gateway.GuardrailsEnabled
	h.Config.Gateway.DefaultConfig = newConfig.Gateway.DefaultConfig
	h.Config.Gateway.Redact = newConfig.Gateway.Redact

	// 保存到文件
	if err := config.SaveConfig(h.Config, config.GetConfigPath()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to save config: %v", err),
			"status": "error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Config updated successfully",
		"status":  "success",
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

func (h *Handler) providerResponsesFromMemory() []ProviderResponse {
	h.Config.Gateway.ProvidersMu.RLock()
	defer h.Config.Gateway.ProvidersMu.RUnlock()
	providers := make([]ProviderResponse, 0, len(h.Config.Gateway.Providers))
	for name, opts := range h.Config.Gateway.Providers {
		providers = append(providers, providerResponse(name, opts, true))
	}
	return providers
}

// syncProviderInMemory adds/updates one provider in the runtime cache the
// proxy reads; the DB remains the source of truth.
func (h *Handler) syncProviderInMemory(name string, opts types.Options) {
	h.Config.Gateway.ProvidersMu.Lock()
	defer h.Config.Gateway.ProvidersMu.Unlock()
	if h.Config.Gateway.Providers == nil {
		h.Config.Gateway.Providers = make(map[string]types.Options)
	}
	h.Config.Gateway.Providers[name] = opts
}

// GetProviders 获取所有 Provider
// GET /api/v1/admin/providers
// @Summary Get providers
// @Description List configured upstream providers (stored in the DB)
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
	if !middleware.IsSuperAdmin(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "Only super_admin can create providers",
			Type:    "authorization_error",
		})
		return
	}

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
	if !middleware.IsSuperAdmin(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "Only super_admin can update providers",
			Type:    "authorization_error",
		})
		return
	}

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
	if !middleware.IsSuperAdmin(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "Only super_admin can delete providers",
			Type:    "authorization_error",
		})
		return
	}

	name := c.Param("name")

	if err := h.providerSvc.Delete(name); err != nil {
		if errors.Is(err, service.ErrProviderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error":  fmt.Sprintf("Provider '%s' not found", name),
				"status": "error",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  fmt.Sprintf("Failed to remove provider: %v", err),
			"status": "error",
		})
		return
	}

	h.Config.Gateway.ProvidersMu.Lock()
	delete(h.Config.Gateway.Providers, name)
	h.Config.Gateway.ProvidersMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Provider '%s' removed successfully", name),
		"status":  "success",
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
