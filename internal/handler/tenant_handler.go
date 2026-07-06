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

// TenantHandler handles platform-level tenant + user management. All routes
// are guarded so that only a super_admin may access them.
type TenantHandler struct {
	service *service.TenantService
}

// NewTenantHandler creates a new TenantHandler.
func NewTenantHandler() *TenantHandler {
	return &TenantHandler{service: service.NewTenantService()}
}

// requireSuperAdmin aborts the request unless the caller is a super_admin.
func requireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !middleware.IsSuperAdmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
				Message: "Super admin privileges required",
				Type:    "authorization_error",
			})
			return
		}
		c.Next()
	}
}

// RegisterRoutesWithAuth registers tenant routes behind JWT + super_admin guard.
func (h *TenantHandler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	grp := router.Group("/api/v1/tenants")
	if jwtMiddleware != nil {
		grp.Use(jwtMiddleware)
	}
	grp.Use(requireSuperAdmin())
	{
		grp.GET("", h.List)
		grp.POST("", h.Create)
		grp.PUT("/:id/status", h.SetStatus)
		grp.GET("/users", h.ListUsers)
		grp.POST("/users", h.CreateUser)
	}
}

// List returns all tenants.
func (h *TenantHandler) List(c *gin.Context) {
	tenants, err := h.service.List()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to list tenants",
			Type:    "internal_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tenants": tenants, "status": "success"})
}

// Create creates a new tenant.
func (h *TenantHandler) Create(c *gin.Context) {
	var req models.TenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}
	t, err := h.service.Create(&req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to create tenant: %v", err),
			Type:    "internal_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tenant": t, "status": "success"})
}

// SetStatus enables/disables a tenant.
func (h *TenantHandler) SetStatus(c *gin.Context) {
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
	if err := h.service.SetStatus(uint(id), body.Status); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "tenant not found" {
			status = http.StatusNotFound
		} else if err.Error() == "invalid status" {
			status = http.StatusBadRequest
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: err.Error(),
			Type:    "update_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Tenant status updated", "status": "success"})
}

// ListUsers returns users, optionally filtered by ?tenant_id=.
func (h *TenantHandler) ListUsers(c *gin.Context) {
	var tenantID uint
	if q := c.Query("tenant_id"); q != "" {
		if v, err := strconv.ParseUint(q, 10, 32); err == nil {
			tenantID = uint(v)
		}
	}
	users, err := h.service.ListUsers(tenantID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to list users",
			Type:    "internal_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users, "status": "success"})
}

// CreateUser creates a tenant_admin user for a tenant.
func (h *TenantHandler) CreateUser(c *gin.Context) {
	var req models.UserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}
	u, err := h.service.CreateUser(&req)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "tenant not found" {
			status = http.StatusBadRequest
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to create user: %v", err),
			Type:    "internal_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": u, "status": "success"})
}
