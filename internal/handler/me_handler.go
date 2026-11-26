package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// MeHandler serves the authenticated user's self-service endpoints:
// month-to-date usage and (when the tenant allows it) self-service API keys.
type MeHandler struct {
	vkService *service.VirtualKeyService
}

// NewMeHandler creates a MeHandler.
func NewMeHandler() *MeHandler {
	return &MeHandler{vkService: service.NewVirtualKeyService()}
}

// RegisterRoutesWithAuth registers /api/v1/me routes behind JWT auth.
func (h *MeHandler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	grp := router.Group("/api/v1/me")
	if jwtMiddleware != nil {
		grp.Use(jwtMiddleware)
	}
	{
		grp.GET("/usage", h.Usage)
		grp.GET("/keys", h.ListKeys)
		grp.POST("/keys", h.CreateKey)
		grp.DELETE("/keys/:id", h.DeleteKey)
	}
}

// MeUsageItem aggregates a single model's month-to-date usage for the caller.
type MeUsageItem struct {
	Model        string  `json:"model"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	Cost         float64 `json:"cost"`
}

// Usage returns the caller's month-to-date spend and quota.
func (h *MeHandler) Usage(c *gin.Context) {
	userID := currentUserID(c)
	if userID == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid user token",
			Type:    "authentication_error",
		})
		return
	}

	var user models.User
	if err := database.GetDB().First(&user, *userID).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, types.ErrorResponse{
			Message: "user not found",
			Type:    "not_found_error",
		})
		return
	}

	monthStart := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	var rows []MeUsageItem
	var totalReq, inTok, outTok int64
	var totalCost float64
	if err := database.GetDB().Table("usage_records").
		Select("model, COUNT(*) AS requests, COALESCE(SUM(input_tokens),0) AS input_tokens, COALESCE(SUM(output_tokens),0) AS output_tokens, COALESCE(SUM(cost),0) AS cost").
		Where("user_id = ? AND created_at >= ?", *userID, monthStart).
		Group("model").
		Order("cost DESC").
		Scan(&rows).Error; err == nil {
		for _, r := range rows {
			totalReq += r.Requests
			totalCost += r.Cost
			inTok += r.InputTokens
			outTok += r.OutputTokens
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"username":       user.Username,
		"role":           user.Role,
		"budget_monthly": user.BudgetMonthly,
		"budget_used":    user.BudgetUsed,
		"rpm_limit":      user.RpmLimit,
		"total_requests": totalReq,
		"total_cost":     totalCost,
		"input_tokens":   inTok,
		"output_tokens":  outTok,
		"items":          rows,
	})
}

// ListKeys returns API keys owned by the caller.
func (h *MeHandler) ListKeys(c *gin.Context) {
	userID := currentUserID(c)
	if userID == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid user token",
			Type:    "authentication_error",
		})
		return
	}
	var keys []models.VirtualKey
	if err := database.GetDB().Where("user_id = ? AND deleted_at IS NULL", *userID).Order("id DESC").Find(&keys).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "failed to list keys",
			Type:    "internal_error",
		})
		return
	}
	resp := make([]*models.VirtualKeyResponse, 0, len(keys))
	for i := range keys {
		resp = append(resp, h.vkService.ToResponse(&keys[i], ""))
	}
	c.JSON(http.StatusOK, gin.H{"virtual_keys": resp, "status": "success"})
}

// CreateKey issues a key owned by the caller when the tenant allows
// self-service keys.
func (h *MeHandler) CreateKey(c *gin.Context) {
	userID := currentUserID(c)
	if userID == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid user token",
			Type:    "authentication_error",
		})
		return
	}
	var user models.User
	if err := database.GetDB().First(&user, *userID).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, types.ErrorResponse{
			Message: "user not found",
			Type:    "not_found_error",
		})
		return
	}
	if user.TenantID == nil {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "super_admin issues keys via the admin API",
			Type:    "authorization_error",
		})
		return
	}
	var tenant models.Tenant
	if err := database.GetDB().First(&tenant, *user.TenantID).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "tenant not found",
			Type:    "authorization_error",
		})
		return
	}
	if !tenant.AllowSelfKey {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "self-service keys are disabled for this tenant; ask your tenant admin",
			Type:    "authorization_error",
		})
		return
	}

	var req models.VirtualKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}
	// Keys created via /me are always owned by the caller, within their own
	// tenant, and bound to the user's monthly quota (no extra budget grant).
	req.UserID = userID
	fullKey, vk, err := h.vkService.Create(*user.TenantID, &req, userID, user.Username)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("failed to create key: %v", err),
			Type:    "internal_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": h.vkService.ToResponse(vk, fullKey), "status": "success"})
}

// DeleteKey revokes one of the caller's own keys.
func (h *MeHandler) DeleteKey(c *gin.Context) {
	userID := currentUserID(c)
	if userID == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid user token",
			Type:    "authentication_error",
		})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Invalid ID",
			Type:    "invalid_request_error",
		})
		return
	}
	var vk models.VirtualKey
	if err := database.GetDB().Where("id = ? AND user_id = ?", id, *userID).First(&vk).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, types.ErrorResponse{
			Message: "key not found",
			Type:    "not_found_error",
		})
		return
	}
	if err := h.vkService.Delete(vk.TenantID, vk.ID); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "failed to revoke key",
			Type:    "internal_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "key revoked", "status": "success"})
}
