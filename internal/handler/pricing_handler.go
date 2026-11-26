package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// PricingHandler manages per-model token pricing (admin API).
type PricingHandler struct {
	service *service.PricingService
}

// NewPricingHandler creates a PricingHandler.
func NewPricingHandler() *PricingHandler {
	return &PricingHandler{service: service.NewPricingService(database.GetDB())}
}

// RegisterRoutesWithAuth registers pricing routes with JWT protection.
func (h *PricingHandler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	api := router.Group("/api/v1/admin/pricing")
	if jwtMiddleware != nil {
		api.Use(jwtMiddleware)
	}
	{
		api.GET("", h.List)
		api.POST("", h.Upsert)
		api.PUT("/:id", h.Update)
		api.DELETE("/:id", h.Delete)
		api.POST("/sync", h.Sync)
	}
}

func pricingAccessDenied(c *gin.Context) bool {
	if !middleware.IsSuperAdmin(c) {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "Only super_admin can manage pricing",
			Type:    "forbidden",
		})
		return true
	}
	return false
}

// List returns all pricing rows, optionally filtered by provider.
// GET /api/v1/admin/pricing?provider=openai
func (h *PricingHandler) List(c *gin.Context) {
	if pricingAccessDenied(c) {
		return
	}
	rows, err := models.GetAllModelPricings(database.GetDB())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to list pricing",
			Type:    "internal_error",
		})
		return
	}
	provider := c.Query("provider")
	out := make([]models.ModelPricing, 0, len(rows))
	for _, row := range rows {
		if provider != "" && row.Provider != provider {
			continue
		}
		out = append(out, row)
	}
	c.JSON(http.StatusOK, gin.H{"pricings": out, "status": "success"})
}

type pricingUpsertRequest struct {
	Provider        string  `json:"provider" binding:"required"`
	Model           string  `json:"model" binding:"required"`
	InputPrice      float64 `json:"input_price"`
	OutputPrice     float64 `json:"output_price"`
	CacheReadPrice  float64 `json:"cache_read_price"`
	CacheWritePrice float64 `json:"cache_write_price"`
	Currency        string  `json:"currency"`
}

// Upsert creates or updates a pricing row keyed on (provider, model).
// POST /api/v1/admin/pricing
func (h *PricingHandler) Upsert(c *gin.Context) {
	if pricingAccessDenied(c) {
		return
	}
	var req pricingUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}
	mp := &models.ModelPricing{
		Provider:        req.Provider,
		Model:           req.Model,
		InputPrice:      req.InputPrice,
		OutputPrice:     req.OutputPrice,
		CacheReadPrice:  req.CacheReadPrice,
		CacheWritePrice: req.CacheWritePrice,
		Currency:        orDefault(req.Currency, "USD"),
		Source:          "manual",
	}
	if err := models.UpsertModelPricing(database.GetDB(), mp); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to save pricing",
			Type:    "internal_error",
		})
		return
	}
	// Persisted rows may differ from what was just written (upsert merge);
	// reload so the response reflects the stored row.
	saved, _ := models.GetModelPricing(database.GetDB(), mp.Provider, mp.Model)
	c.JSON(http.StatusOK, gin.H{"pricing": saved, "status": "success"})
}

// Update updates a pricing row by ID.
// PUT /api/v1/admin/pricing/:id
func (h *PricingHandler) Update(c *gin.Context) {
	if pricingAccessDenied(c) {
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
	db := database.GetDB()
	var mp models.ModelPricing
	if err := db.First(&mp, uint(id)).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, types.ErrorResponse{
			Message: "Pricing not found",
			Type:    "not_found",
		})
		return
	}
	var req pricingUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: fmt.Sprintf("Invalid request: %v", err),
			Type:    "invalid_request_error",
		})
		return
	}
	mp.Provider = req.Provider
	mp.Model = req.Model
	mp.InputPrice = req.InputPrice
	mp.OutputPrice = req.OutputPrice
	mp.CacheReadPrice = req.CacheReadPrice
	mp.CacheWritePrice = req.CacheWritePrice
	mp.Currency = orDefault(req.Currency, "USD")
	mp.Source = "manual"
	if err := db.Save(&mp).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to update pricing",
			Type:    "internal_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pricing": mp, "status": "success"})
}

// Delete removes a pricing row by ID.
// DELETE /api/v1/admin/pricing/:id
func (h *PricingHandler) Delete(c *gin.Context) {
	if pricingAccessDenied(c) {
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
	if err := database.GetDB().Delete(&models.ModelPricing{}, uint(id)).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to delete pricing",
			Type:    "internal_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// Sync fetches pricing from Portkey and upserts rows.
// POST /api/v1/admin/pricing/sync  body: {"providers": ["openai", ...], "dry_run": false}
func (h *PricingHandler) Sync(c *gin.Context) {
	if pricingAccessDenied(c) {
		return
	}
	var req struct {
		Providers []string `json:"providers"`
		DryRun    bool     `json:"dry_run"`
	}
	_ = c.ShouldBindJSON(&req)

	count, err := h.service.FetchAndStore(req.Providers, req.DryRun)
	if err != nil && count == 0 {
		c.AbortWithStatusJSON(http.StatusBadGateway, types.ErrorResponse{
			Message: fmt.Sprintf("Pricing sync failed: %v", err),
			Type:    "upstream_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"count":   count,
		"message": pricingSyncMessage(count, err),
	})
}

func pricingSyncMessage(count int, err error) string {
	if err != nil {
		return fmt.Sprintf("Synced %d pricing rows; some providers failed: %v", count, err)
	}
	return fmt.Sprintf("Synced %d pricing rows", count)
}

func orDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
