package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
)

type ProviderHealthHandler struct {
	service *service.ProviderHealthService
}

func NewProviderHealthHandler(service *service.ProviderHealthService) *ProviderHealthHandler {
	return &ProviderHealthHandler{service: service}
}

func (h *ProviderHealthHandler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	api := router.Group("/api/v1/admin/providers")
	if jwtMiddleware != nil {
		api.Use(jwtMiddleware)
	}
	api.GET("/health", h.List)
}

func (h *ProviderHealthHandler) List(c *gin.Context) {
	if !middleware.IsSuperAdmin(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{Message: "Only super_admin can view provider health", Type: "forbidden"})
		return
	}
	rows, err := h.service.List()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{Message: "Failed to list provider health", Type: "internal_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"providers": rows, "status": "success"})
}
