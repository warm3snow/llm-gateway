package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
	"gorm.io/gorm"
)

// VirtualKeyHandler handles virtual key API requests
type VirtualKeyHandler struct {
	service *service.VirtualKeyService
	db      *gorm.DB
}

// NewVirtualKeyHandler creates a new VirtualKeyHandler
func NewVirtualKeyHandler() *VirtualKeyHandler {
	return &VirtualKeyHandler{
		service: service.NewVirtualKeyService(),
		db:      database.GetDB(),
	}
}

// RegisterRoutes registers virtual key routes (no JWT)
func (h *VirtualKeyHandler) RegisterRoutes(router *gin.Engine) {
	h.RegisterRoutesWithAuth(router, nil)
}

// RegisterRoutesWithAuth registers virtual key routes with JWT protection
func (h *VirtualKeyHandler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	vk := router.Group("/api/v1/virtual-keys")
	if jwtMiddleware != nil {
		vk.Use(jwtMiddleware)
	}
	{
		vk.POST("", h.Create)
		vk.GET("", h.List)
		vk.GET("/:id", h.Get)
		vk.PUT("/:id", h.Update)
		vk.DELETE("/:id", h.Delete)
		vk.POST("/:id/reset", h.Reset)
	}
}

// Create creates a new virtual key
// POST /api/v1/virtual-keys
// @Summary Create virtual key
// @Description Create a new virtual key for API access
// @Tags virtual-keys
// @Accept json
// @Produce json
// @Param request body models.VirtualKeyRequest true "Virtual key request"
// @Success 200 {object} map[string]interface{} "Virtual key created"
// @Failure 400 {object} types.ErrorResponse "Invalid request"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/virtual-keys [post]
func (h *VirtualKeyHandler) Create(c *gin.Context) {
	if currentRole(c) == models.RoleTenantUser {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "tenant_user cannot create virtual keys",
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

	// Resolve the owning tenant. tenant_admin -> own tenant; super_admin may
	// target a tenant via ?tenant_id=, otherwise the key lands in the default
	// tenant rather than becoming unscoped/orphaned.
	tenantID := middleware.EffectiveTenantID(c)
	if tenantID == 0 {
		tenantID = database.DefaultTenantID
	}

	fullKey, vk, err := h.service.Create(tenantID, &req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to create virtual key: %v", err),
			Type:    "internal_error",
		})
		return
	}

	resp := h.service.ToResponse(vk, fullKey)
	c.JSON(http.StatusOK, gin.H{
		"message": "Virtual key created successfully. Save the key now - it won't be shown again!",
		"key":     resp,
		"status":  "success",
	})
}

// List lists all virtual keys
// GET /api/v1/virtual-keys
// @Summary List virtual keys
// @Description Get all virtual keys
// @Tags virtual-keys
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "List of virtual keys"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/virtual-keys [get]
func (h *VirtualKeyHandler) List(c *gin.Context) {
	keys, err := h.service.List(middleware.EffectiveTenantID(c))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to list virtual keys: %v", err),
			Type:    "internal_error",
		})
		return
	}

	resp := make([]*models.VirtualKeyResponse, 0, len(keys))
	for _, vk := range keys {
		resp = append(resp, h.service.ToResponse(&vk, ""))
	}

	c.JSON(http.StatusOK, gin.H{
		"virtual_keys": resp,
		"status":       "success",
	})
}

// Get gets a virtual key by ID
// GET /api/v1/virtual-keys/:id
// @Summary Get virtual key
// @Description Get a virtual key by ID
// @Tags virtual-keys
// @Accept json
// @Produce json
// @Param id path int true "Virtual Key ID"
// @Success 200 {object} map[string]interface{} "Virtual key details"
// @Failure 400 {object} types.ErrorResponse "Invalid ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 404 {object} types.ErrorResponse "Not found"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/virtual-keys/{id} [get]
func (h *VirtualKeyHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Invalid ID",
			Type:    "invalid_request_error",
		})
		return
	}

	vk, err := h.service.GetByID(middleware.EffectiveTenantID(c), uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "virtual key not found" {
			status = http.StatusNotFound
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: err.Error(),
			Type:    "not_found",
		})
		return
	}

	resp := h.service.ToResponse(vk, "")
	c.JSON(http.StatusOK, gin.H{
		"virtual_key": resp,
		"status":      "success",
	})
}

// Update updates a virtual key
// PUT /api/v1/virtual-keys/:id
// @Summary Update virtual key
// @Description Update a virtual key by ID
// @Tags virtual-keys
// @Accept json
// @Produce json
// @Param id path int true "Virtual Key ID"
// @Param request body models.VirtualKeyRequest true "Virtual key request"
// @Success 200 {object} map[string]interface{} "Virtual key updated"
// @Failure 400 {object} types.ErrorResponse "Invalid request"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 404 {object} types.ErrorResponse "Not found"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/virtual-keys/{id} [put]
func (h *VirtualKeyHandler) Update(c *gin.Context) {
	if currentRole(c) == models.RoleTenantUser {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "tenant_user cannot update virtual keys",
			Type:    "authorization_error",
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

	var req models.VirtualKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}

	vk, err := h.service.Update(middleware.EffectiveTenantID(c), uint(id), &req)
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "virtual key not found" {
			status = http.StatusNotFound
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: err.Error(),
			Type:    "update_error",
		})
		return
	}

	resp := h.service.ToResponse(vk, "")
	c.JSON(http.StatusOK, gin.H{
		"message":     "Virtual key updated successfully",
		"virtual_key": resp,
		"status":      "success",
	})
}

// Delete deletes a virtual key
// DELETE /api/v1/virtual-keys/:id
// @Summary Delete virtual key
// @Description Delete a virtual key by ID
// @Tags virtual-keys
// @Accept json
// @Produce json
// @Param id path int true "Virtual Key ID"
// @Success 200 {object} map[string]interface{} "Virtual key deleted"
// @Failure 400 {object} types.ErrorResponse "Invalid ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 404 {object} types.ErrorResponse "Not found"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/virtual-keys/{id} [delete]
func (h *VirtualKeyHandler) Delete(c *gin.Context) {
	if currentRole(c) == models.RoleTenantUser {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "tenant_user cannot delete virtual keys",
			Type:    "authorization_error",
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

	if err := h.service.Delete(middleware.EffectiveTenantID(c), uint(id)); err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "virtual key not found" {
			status = http.StatusNotFound
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to delete virtual key: %v", err),
			Type:    "internal_error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Virtual key deleted successfully",
		"status":  "success",
	})
}

// Reset resets budget for a virtual key
// POST /api/v1/virtual-keys/:id/reset
// @Summary Reset virtual key budget
// @Description Reset budget usage for a virtual key
// @Tags virtual-keys
// @Accept json
// @Produce json
// @Param id path int true "Virtual Key ID"
// @Success 200 {object} map[string]interface{} "Virtual key reset"
// @Failure 400 {object} types.ErrorResponse "Invalid ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 404 {object} types.ErrorResponse "Not found"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/virtual-keys/{id}/reset [post]
func (h *VirtualKeyHandler) Reset(c *gin.Context) {
	if currentRole(c) == models.RoleTenantUser {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "tenant_user cannot reset virtual keys",
			Type:    "authorization_error",
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
	if err := h.db.Scopes(database.TenantScope(middleware.EffectiveTenantID(c))).First(&vk, uint(id)).Error; err != nil {
		status := http.StatusInternalServerError
		if err == gorm.ErrRecordNotFound {
			status = http.StatusNotFound
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: "Virtual key not found",
			Type:    "not_found",
		})
		return
	}

	// Reset budget usage
	vk.BudgetUsed = 0
	now := time.Now()
	vk.BudgetResetAt = &now

	if err := h.db.Save(&vk).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: fmt.Sprintf("Failed to reset virtual key: %v", err),
			Type:    "internal_error",
		})
		return
	}

	resp := h.service.ToResponse(&vk, "")
	c.JSON(http.StatusOK, gin.H{
		"message":     "Virtual key reset successfully",
		"virtual_key": resp,
		"status":      "success",
	})
}
