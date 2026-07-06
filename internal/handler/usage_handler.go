package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// UsageHandler handles usage-record API requests.
type UsageHandler struct {
	service *service.UsageService
}

// NewUsageHandler creates a new UsageHandler.
func NewUsageHandler() *UsageHandler {
	return &UsageHandler{
		service: service.NewUsageService(),
	}
}

// RegisterRoutes registers usage routes (no JWT).
func (h *UsageHandler) RegisterRoutes(router *gin.Engine) {
	h.RegisterRoutesWithAuth(router, nil)
}

// RegisterRoutesWithAuth registers usage routes with JWT protection.
func (h *UsageHandler) RegisterRoutesWithAuth(router *gin.Engine, jwtMiddleware gin.HandlerFunc) {
	usage := router.Group("/api/v1/usage")
	if jwtMiddleware != nil {
		usage.Use(jwtMiddleware)
	}
	{
		usage.GET("", h.GetRecords)
		usage.GET("/:id", h.GetRecord)
	}
}

// GetRecords returns paginated usage records.
// GET /api/v1/usage?provider=&model=&status_code=&start_date=&end_date=&limit=&offset=
// @Summary Get usage records
// @Description Get paginated model-invocation usage records with filters
// @Tags usage
// @Accept json
// @Produce json
// @Param provider query string false "Filter by provider"
// @Param model query string false "Filter by model"
// @Param status_code query int false "Filter by status code"
// @Param start_date query string false "Filter by start date (YYYY-MM-DD)"
// @Param end_date query string false "Filter by end date (YYYY-MM-DD)"
// @Param limit query int false "Limit results"
// @Param offset query int false "Offset for pagination"
// @Success 200 {object} map[string]interface{} "List of usage records"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/usage [get]
func (h *UsageHandler) GetRecords(c *gin.Context) {
	provider := c.Query("provider")
	model := c.Query("model")
	statusCode, _ := strconv.Atoi(c.Query("status_code"))

	var startDate, endDate time.Time
	if s := c.Query("start_date"); s != "" {
		startDate, _ = time.Parse("2006-01-02", s)
	}
	if s := c.Query("end_date"); s != "" {
		endDate, _ = time.Parse("2006-01-02", s)
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	records, total, err := h.service.GetRecords(middleware.EffectiveTenantID(c), provider, model, statusCode, startDate, endDate, limit, offset)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to get usage records",
			Type:    "internal_error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"status":  "success",
	})
}

// GetRecord returns a single usage record by ID.
// GET /api/v1/usage/:id
// @Summary Get usage record
// @Description Get a single usage record by ID
// @Tags usage
// @Accept json
// @Produce json
// @Param id path int true "Record ID"
// @Success 200 {object} map[string]interface{} "Record details"
// @Failure 400 {object} types.ErrorResponse "Invalid ID"
// @Failure 401 {object} types.ErrorResponse "Unauthorized"
// @Failure 404 {object} types.ErrorResponse "Not found"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Security BearerAuth
// @Router /api/v1/usage/{id} [get]
func (h *UsageHandler) GetRecord(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Invalid ID",
			Type:    "invalid_request_error",
		})
		return
	}

	record, err := h.service.GetRecordByID(middleware.EffectiveTenantID(c), uint(id))
	if err != nil {
		status := http.StatusInternalServerError
		if err.Error() == "record not found" {
			status = http.StatusNotFound
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{
			Message: "Usage record not found",
			Type:    "not_found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"record": record,
		"status": "success",
	})
}
