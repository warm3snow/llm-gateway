package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// StatsHandler handles stats API requests
type StatsHandler struct {
	service *service.StatsService
	cfg     *config.Config
}

// NewStatsHandler creates a new StatsHandler
func NewStatsHandler(cfg *config.Config) *StatsHandler {
	return &StatsHandler{
		service: service.NewStatsService(),
		cfg:     cfg,
	}
}

// RegisterRoutes registers stats routes (no JWT)
func (h *StatsHandler) RegisterRoutes(router *gin.Engine) {
	h.RegisterRoutesWithAuth(router, nil)
}

// RegisterRoutesWithAuth registers stats routes with JWT protection
func (h *StatsHandler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	stats := router.Group("/api/v1/stats")
	if jwtMiddleware != nil {
		stats.Use(jwtMiddleware)
	}
	{
		stats.GET("/overview", h.GetOverview)
		stats.GET("/analytics", h.GetAnalytics)
		stats.GET("/hourly", h.GetHourly)
	}
}

// windowFromQuery parses an optional "hours" query param (default 24) and
// returns the [start, end] window.
func windowFromQuery(c *gin.Context) (time.Time, time.Time) {
	hours := 24
	if v := c.Query("hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hours = n
		}
	}
	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)
	return start, end
}

// GetHourly returns hourly-bucketed time series for the requested window.
// GET /api/v1/stats/hourly?hours=24
func (h *StatsHandler) GetHourly(c *gin.Context) {
	start, end := windowFromQuery(c)
	points, err := h.service.GetHourlyTimeSeries(start, end)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to get hourly stats",
			Type:    "internal_error",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"timeSeries": points})
}

// GetOverview returns dashboard overview stats
// GET /api/v1/stats/overview
// @Summary Get stats overview
// @Description Get dashboard overview statistics
// @Tags stats
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Stats overview"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/stats/overview [get]
func (h *StatsHandler) GetOverview(c *gin.Context) {
	overview, err := h.service.GetOverview()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to get stats overview",
			Type:    "internal_error",
		})
		return
	}

	// If no usage recorded yet, count configured providers
	if overview.ActiveProviders == 0 {
		overview.ActiveProviders = len(h.cfg.Gateway.Providers)
	}

	c.JSON(http.StatusOK, overview)
}

// GetAnalytics returns analytics data for the analytics page
// GET /api/v1/stats/analytics
func (h *StatsHandler) GetAnalytics(c *gin.Context) {
	data, err := h.service.GetAnalytics()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to get analytics data",
			Type:    "internal_error",
		})
		return
	}

	// If no usage recorded yet, count configured providers
	if data.ActiveProviders == 0 {
		data.ActiveProviders = len(h.cfg.Gateway.Providers)
	}

	c.JSON(http.StatusOK, data)
}
