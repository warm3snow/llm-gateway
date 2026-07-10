package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
)

type AlertHandler struct {
	service *service.AlertService
}

func NewAlertHandler() *AlertHandler {
	return &AlertHandler{service: service.NewAlertService()}
}

func (h *AlertHandler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	alerts := router.Group("/api/v1/alerts")
	if jwtMiddleware != nil {
		alerts.Use(jwtMiddleware)
	}
	alerts.GET("/rules", h.ListRules)
	alerts.POST("/rules", h.CreateRule)
	alerts.GET("/events", h.ListEvents)
}

func (h *AlertHandler) CreateRule(c *gin.Context) {
	if currentRole(c) == models.RoleTenantUser {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{Message: "Only admins can create alert rules", Type: "forbidden"})
		return
	}
	var req models.AlertRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{Message: err.Error(), Type: "invalid_request_error"})
		return
	}
	tenantID := middleware.EffectiveTenantID(c)
	if tenantID == 0 {
		tenantID = database.DefaultTenantID
	}
	rule, err := h.service.CreateRule(tenantID, &req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{Message: "Failed to create alert rule", Type: "internal_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rule": rule, "status": "success"})
}

func (h *AlertHandler) ListRules(c *gin.Context) {
	virtualKeyID, _ := strconv.ParseUint(c.Query("virtual_key_id"), 10, 64)
	rules, err := h.service.ListRules(middleware.EffectiveTenantID(c), uint(virtualKeyID))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{Message: "Failed to list alert rules", Type: "internal_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules, "status": "success"})
}

func (h *AlertHandler) ListEvents(c *gin.Context) {
	virtualKeyID, _ := strconv.ParseUint(c.Query("virtual_key_id"), 10, 64)
	activeOnly := c.Query("active") == "true"
	events, err := h.service.ListEvents(middleware.EffectiveTenantID(c), uint(virtualKeyID), activeOnly)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{Message: "Failed to list alert events", Type: "internal_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events, "status": "success"})
}
