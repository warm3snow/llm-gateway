package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// RedactHandler manages the sensitive-information masking configuration.
// Changes apply live and are persisted to configs/config.yaml.
type RedactHandler struct {
	cfg *config.Config
}

// NewRedactHandler creates a RedactHandler.
func NewRedactHandler(cfg *config.Config) *RedactHandler {
	return &RedactHandler{cfg: cfg}
}

// RegisterRoutesWithAuth registers redact routes with JWT protection.
func (h *RedactHandler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	api := router.Group("/api/v1/admin/redact")
	if jwtMiddleware != nil {
		api.Use(jwtMiddleware)
	}
	{
		api.GET("", h.Get)
		api.PUT("", h.Update)
		api.POST("/preview", h.Preview)
	}
}

func redactAccessDenied(c *gin.Context) bool {
	if !middleware.IsSuperAdmin(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "Only super_admin can manage masking settings",
			Type:    "forbidden",
		})
		return true
	}
	return false
}

// Get returns the current masking configuration.
// GET /api/v1/admin/redact
func (h *RedactHandler) Get(c *gin.Context) {
	if redactAccessDenied(c) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": service.RedactConfig(), "status": "success"})
}

// Update installs a new masking configuration: validated, applied live and
// persisted to the config file.
// PUT /api/v1/admin/redact
func (h *RedactHandler) Update(c *gin.Context) {
	if redactAccessDenied(c) {
		return
	}
	var cfg config.RedactConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Invalid request: " + err.Error(),
			Type:    "invalid_request_error",
		})
		return
	}
	if err := service.ApplyRedactConfig(cfg); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Invalid masking rule: " + err.Error(),
			Type:    "invalid_request_error",
		})
		return
	}
	h.cfg.Gateway.Redact = cfg
	if err := config.SaveConfig(h.cfg, config.GetConfigPath()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Masking applied but not persisted: " + err.Error(),
			"status": "error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cfg, "status": "success"})
}

// Preview shows what a sample text would become under the candidate (or
// current) configuration, without storing anything.
// POST /api/v1/admin/redact/preview  body: {"text": "...", "config": {...optional}}
func (h *RedactHandler) Preview(c *gin.Context) {
	if redactAccessDenied(c) {
		return
	}
	var req struct {
		Text   string               `json:"text" binding:"required"`
		Config *config.RedactConfig `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Invalid request: " + err.Error(),
			Type:    "invalid_request_error",
		})
		return
	}

	opts, enabled := service.RedactOptions()
	cfg := service.RedactConfig()
	if req.Config != nil {
		cfg = *req.Config
		var err error
		opts, err = service.RedactOptionsFor(cfg)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
				Message: "Invalid masking rule: " + err.Error(),
				Type:    "invalid_request_error",
			})
			return
		}
	}

	masked := service.RedactEngine().Preview(req.Text, opts)
	c.JSON(http.StatusOK, gin.H{
		"masked":  masked,
		"enabled": enabled,
		"status":  "success",
	})
}
