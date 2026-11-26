package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// UserHandler handles admin-console user management.
type UserHandler struct {
	service *service.UserService
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler() *UserHandler {
	return &UserHandler{service: service.NewUserService()}
}

// RegisterRoutesWithAuth registers user routes behind JWT auth.
func (h *UserHandler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	grp := router.Group("/api/v1/users")
	if jwtMiddleware != nil {
		grp.Use(jwtMiddleware)
	}
	{
		grp.GET("", h.List)
		grp.POST("", h.Create)
		grp.PUT("/me/password", h.ChangeOwnPassword)
		grp.PUT("/:id/quota", h.UpdateQuota)
		grp.GET("/:id/cost", h.Cost)
		grp.PUT("/:id/status", h.SetStatus)
		grp.PUT("/:id/role", h.SetRole)
	}
}

// List returns visible users for the caller.
func (h *UserHandler) List(c *gin.Context) {
	var requestedTenantID uint
	if q := c.Query("tenant_id"); q != "" {
		if v, err := strconv.ParseUint(q, 10, 32); err == nil {
			requestedTenantID = uint(v)
		}
	}
	users, err := h.service.List(currentRole(c), middleware.GetUserTenantID(c), requestedTenantID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: err.Error(),
			Type:    "authorization_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users, "status": "success"})
}

// Create creates a tenant user.
func (h *UserHandler) Create(c *gin.Context) {
	var req models.UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}
	u, err := h.service.Create(currentRole(c), middleware.GetUserTenantID(c), &req)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "user management privileges required" {
			status = http.StatusForbidden
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: err.Error(),
			Type:    "user_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u, "status": "success"})
}

// ChangeOwnPassword lets any authenticated user update their own password.
func (h *UserHandler) ChangeOwnPassword(c *gin.Context) {
	userID := currentUserID(c)
	if userID == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid user token",
			Type:    "authentication_error",
		})
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "current_password and new_password are required",
			Type:    "invalid_request_error",
		})
		return
	}
	if err := h.service.ChangeOwnPassword(*userID, body.CurrentPassword, body.NewPassword); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "user not found" {
			status = http.StatusNotFound
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: err.Error(),
			Type:    "user_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password updated", "status": "success"})
}

// UpdateQuota updates a member's monthly quota / rpm limit.
func (h *UserHandler) UpdateQuota(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Invalid ID",
			Type:    "invalid_request_error",
		})
		return
	}
	var req models.UserQuotaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}
	u, err := h.service.UpdateQuota(currentRole(c), middleware.GetUserTenantID(c), uint(id), &req)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "user not found" {
			status = http.StatusNotFound
		} else if err.Error() == "user management privileges required" {
			status = http.StatusForbidden
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: err.Error(),
			Type:    "user_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u, "status": "success"})
}

// Cost returns the member's month-to-date spend broken down by model.
func (h *UserHandler) Cost(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Invalid ID",
			Type:    "invalid_request_error",
		})
		return
	}
	// tenant_admin may only view members of their own tenant.
	role := currentRole(c)
	if role == models.RoleTenantAdmin {
		var member models.TenantMember
		if err := h.service.DB().Where("user_id = ? AND tenant_id = ?", id, middleware.GetUserTenantID(c)).First(&member).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
				Message: "user not in your tenant",
				Type:    "authorization_error",
			})
			return
		}
	}
	result, err := h.service.UserCost(uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "user not found" {
			status = http.StatusNotFound
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: err.Error(),
			Type:    "internal_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":         "success",
		"user_id":        result.UserID,
		"total_requests": result.TotalReq,
		"total_cost":     result.TotalCost,
		"items":          result.Items,
	})
}

// SetStatus enables/disables a user.
func (h *UserHandler) SetStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Invalid ID",
			Type:    "invalid_request_error",
		})
		return
	}
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "status is required",
			Type:    "invalid_request_error",
		})
		return
	}
	if err := h.service.SetStatus(currentRole(c), middleware.GetUserTenantID(c), currentUsername(c), uint(id), body.Status); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "user not found" {
			status = http.StatusNotFound
		} else if err.Error() == "user management privileges required" {
			status = http.StatusForbidden
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: err.Error(),
			Type:    "user_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User status updated", "status": "success"})
}

// SetRole changes a tenant member's role (super_admin only).
func (h *UserHandler) SetRole(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Invalid ID",
			Type:    "invalid_request_error",
		})
		return
	}
	var req models.UserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "role is required",
			Type:    "invalid_request_error",
		})
		return
	}
	actorID := currentUserID(c)
	if actorID == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid user token",
			Type:    "authentication_error",
		})
		return
	}
	u, err := h.service.SetRole(currentRole(c), *actorID, uint(id), &req)
	if err != nil {
		status := http.StatusBadRequest
		switch err.Error() {
		case "user not found":
			status = http.StatusNotFound
		case "user management privileges required":
			status = http.StatusForbidden
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: err.Error(),
			Type:    "user_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u, "status": "success"})
}

func currentRole(c *gin.Context) string {
	role, _ := c.Get("role")
	r, _ := role.(string)
	return r
}

func currentUsername(c *gin.Context) string {
	username, _ := c.Get("username")
	u, _ := username.(string)
	return u
}

func currentUserID(c *gin.Context) *uint {
	userID, exists := c.Get("user_id")
	if !exists {
		return nil
	}
	id, ok := userID.(uint)
	if !ok {
		return nil
	}
	return &id
}
