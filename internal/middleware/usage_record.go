package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/logstore"
	"github.com/warm3snow/llm-gateway/internal/metrics"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
)

// StreamUsageKey is the gin context key under which the streaming proxy
// handler stores the final usage block (set by pkg/proxy.streamResponse).
const StreamUsageKey = "stream_usage"

// streamUsage mirrors the struct used in pkg/proxy. Kept duplicated to avoid
// an import cycle (pkg/proxy already imports internal/middleware indirectly
// via the gin context only — but the struct lives here for the middleware's
// own use).
type streamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

var defaultPricingCache = newPricingCache(5*time.Minute, 30*time.Second)

// responseBodyWriter wraps gin.ResponseWriter to capture the response body and status code.
type responseBodyWriter struct {
	gin.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseBodyWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// UsageRecordMiddleware records a usage entry only for actual model
// invocations. It runs on the model-invocation route group, and persists a
// UsageRecord when either the upstream reported token usage OR the call failed
// (status >= 400). Requests that hit the route but produced neither tokens nor
// an error (e.g. GET /v1/models) are intentionally not recorded — they are not
// billable model invocations.
//
// virtualKeyService is optional; if non-nil, the computed cost is applied to
// the virtual key's budget_used column via TrackUsage, establishing the
// token→budget conversion for every recorded invocation.
func UsageRecordMiddleware(cfg *config.Config, virtualKeyService *service.VirtualKeyService) gin.HandlerFunc {
	return UsageRecordMiddlewareWithBudgetTracker(cfg, virtualKeyService, nil)
}

func UsageRecordMiddlewareWithBudgetTracker(cfg *config.Config, virtualKeyService *service.VirtualKeyService, budgetTracker *service.BudgetTracker) gin.HandlerFunc {
	defaultProvider := "openai"
	if cfg != nil && cfg.Gateway.DefaultProvider != "" {
		defaultProvider = cfg.Gateway.DefaultProvider
	}

	return func(c *gin.Context) {
		start := time.Now()

		// Read and save the request body BEFORE c.Next() so we can extract the model
		// (handlers will consume the body via ShouldBindJSON)
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			// Restore the body for downstream handlers
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		inputPreview := buildModelInputPreview(requestBody, modelInputPreviewMaxBytes)

		// Wrap the response writer to capture the body
		writer := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			status:         http.StatusOK,
		}
		c.Writer = writer
		preWorkElapsed := time.Since(start)

		c.Next()

		postWorkStart := time.Now()
		recordedMiddlewareMetrics := false
		recordMiddlewareMetrics := func() {
			if recordedMiddlewareMetrics {
				return
			}
			recordedMiddlewareMetrics = true
			endpointLabel := middlewareMetricEndpoint(c)
			metrics.RecordUsageMiddleware(endpointLabel, writer.status, preWorkElapsed+time.Since(postWorkStart))
			metrics.RecordUsageBodySizes(endpointLabel, len(requestBody), writer.body.Len())
		}

		// Extract provider: selected auto-mode provider > header > config default.
		provider := c.GetHeader("x-llm-provider")
		if selectedProvider, exists := c.Get("selected_provider"); exists {
			if value, ok := selectedProvider.(string); ok && value != "" {
				provider = value
			}
		}
		if provider == "" {
			provider = defaultProvider
		}

		// Extract model: selected auto-mode model > saved request body/header.
		model := extractModelFromBytes(requestBody, c)
		if selectedModel, exists := c.Get("selected_model"); exists {
			if value, ok := selectedModel.(string); ok && value != "" {
				model = value
			}
		}

		// Parse tokens: prefer the streaming usage stashed by pkg/proxy,
		// then fall back to parsing the captured (non-stream) response body.
		inputTokens, outputTokens := 0, 0
		if u, exists := c.Get(StreamUsageKey); exists {
			if su, ok := u.(streamUsage); ok {
				inputTokens = su.PromptTokens
				outputTokens = su.CompletionTokens
			} else if suPtr, ok := u.(*streamUsage); ok && suPtr != nil {
				inputTokens = suPtr.PromptTokens
				outputTokens = suPtr.CompletionTokens
			}
		}
		if inputTokens == 0 && outputTokens == 0 {
			inputTokens, outputTokens = parseUsage(writer.body.Bytes())
		}

		// Detect a cache hit for cost purposes: CacheMiddleware sets the
		// "x-cache" response header to HIT/MISS. Cache hits are still billable
		// (at the cache-read rate) and thus still recorded.
		cached := c.Writer.Header().Get("x-cache") == "HIT"

		cost := calculateCost(provider, model, inputTokens, outputTokens, cached)

		// Recording gate: only persist actual model invocations — those that
		// produced token usage, or failed upstream. Everything else that
		// merely hit the route (e.g. GET /v1/models) is skipped.
		status := writer.status
		if inputTokens == 0 && outputTokens == 0 && status < 400 {
			recordMiddlewareMetrics()
			return
		}

		// Get virtual key info from context (set by VirtualKeyAuth middleware)
		var virtualKeyID *uint
		var virtualKeyName string
		var virtualKeyCreatedByUserID *uint
		var virtualKeyCreatedByUsername string
		if id, exists := c.Get("virtual_key_id"); exists {
			if v, ok := id.(uint); ok {
				virtualKeyID = &v
			}
		}
		if name, exists := c.Get("virtual_key_name"); exists {
			if n, ok := name.(string); ok {
				virtualKeyName = n
			}
		}
		if id, exists := c.Get("virtual_key_created_by_user_id"); exists {
			if v, ok := id.(uint); ok {
				virtualKeyCreatedByUserID = &v
			}
		}
		if username, exists := c.Get("virtual_key_created_by_username"); exists {
			if u, ok := username.(string); ok {
				virtualKeyCreatedByUsername = u
			}
		}

		tenantID := GetUserTenantID(c)
		if tenantID == 0 {
			tenantID = database.DefaultTenantID
		}

		// Apply the virtual key's budget usage. Prefer the async tracker so request
		// latency is not coupled to the budget UPDATE; fall back to synchronous
		// TrackUsage if the queue is full or async tracking is not configured.
		if virtualKeyID != nil && cost > 0 {
			trackStart := time.Now()
			trackResult := trackVirtualKeyBudget(*virtualKeyID, cost, virtualKeyService, budgetTracker)
			metrics.RecordUsageTrackUsage(trackResult, time.Since(trackStart))
		}

		// Reuse the request-scoped trace ID (set by the Logger middleware) so
		// the persisted row correlates with the structured access logs.
		requestID := generateRequestID()
		if tid, ok := c.Get("trace_id"); ok {
			if s, ok := tid.(string); ok && s != "" {
				requestID = s
			}
		}

		// Capture an error message for failed calls (from gin errors, else the
		// response body which typically carries the upstream error JSON).
		errorMessage := ""
		if status >= 400 {
			if e := c.Errors.ByType(gin.ErrorTypePrivate).String(); e != "" {
				errorMessage = e
			} else {
				errorMessage = truncate(writer.body.String(), 2000)
			}
		}

		record := models.UsageRecord{
			TenantID:                    tenantID,
			RequestID:                   requestID,
			VirtualKeyID:                virtualKeyID,
			VirtualKeyName:              virtualKeyName,
			VirtualKeyCreatedByUserID:   virtualKeyCreatedByUserID,
			VirtualKeyCreatedByUsername: virtualKeyCreatedByUsername,
			Provider:                    provider,
			Model:                       model,
			Endpoint:                    c.Request.URL.Path,
			StatusCode:                  status,
			ErrorMessage:                errorMessage,
			ModelInputKind:              inputPreview.Kind,
			ModelInputPreview:           inputPreview.Preview,
			ModelInputTruncated:         inputPreview.Truncated,
			ModelInputBytes:             inputPreview.InputBytes,
			ModelInputPreviewBytes:      inputPreview.PreviewBytes,
			InputTokens:                 inputTokens,
			OutputTokens:                outputTokens,
			Cost:                        cost,
		}

		// Record real-time Prometheus metrics. Use the route template
		// (FullPath) as the endpoint label to keep cardinality bounded;
		// fall back to the raw path only when no route matched.
		endpointLabel := c.FullPath()
		if endpointLabel == "" {
			endpointLabel = "unmatched"
		}
		elapsed := time.Since(start)
		metrics.RecordRequest(provider, model, endpointLabel, status, cached, elapsed.Seconds(), inputTokens, outputTokens, cost)

		// Persist asynchronously via the buffered writer. This decouples the
		// request path from DB latency and bounds goroutine growth under load.
		logstore.Enqueue(&record)
		recordMiddlewareMetrics()
	}
}

func trackVirtualKeyBudget(virtualKeyID uint, cost float64, virtualKeyService *service.VirtualKeyService, budgetTracker *service.BudgetTracker) string {
	if budgetTracker != nil {
		if err := budgetTracker.Enqueue(virtualKeyID, cost); err == nil {
			return "queued"
		}
		if virtualKeyService == nil {
			return "error"
		}
		if err := virtualKeyService.TrackUsage(virtualKeyID, cost); err != nil {
			return "fallback_error"
		}
		return "fallback_success"
	}
	if virtualKeyService != nil {
		if err := virtualKeyService.TrackUsage(virtualKeyID, cost); err != nil {
			return "error"
		}
	}
	return "success"
}

// truncate caps a string to n bytes to bound stored error message size.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// extractModelFromBytes extracts the model name from the request body bytes.
// Returns empty string if no model field is found (e.g. GET /v1/models).
func extractModelFromBytes(bodyBytes []byte, c *gin.Context) string {
	// Try to parse as JSON and extract "model" field
	if len(bodyBytes) > 0 {
		var body map[string]interface{}
		if json.Unmarshal(bodyBytes, &body) == nil {
			if m, ok := body["model"].(string); ok {
				return m
			}
		}
	}

	// Fallback: try from header
	if m := c.GetHeader("x-llm-model"); m != "" {
		return m
	}

	return ""
}

// parseUsage extracts token counts from an OpenAI-compatible response body.
func parseUsage(body []byte) (inputTokens, outputTokens int) {
	var resp struct {
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Usage == nil {
		return 0, 0
	}
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens
}

// calculateCost computes cost (USD) for a request using the model_pricings
// table. Prices are stored as cents per token, matching Portkey's convention.
// Falls back to the provider's "default" row when the exact model isn't found.
// Returns 0 if no pricing is available — recording 0 cost is preferable to
// guessing with stale hardcoded rates.
//
// When cached is true and the model has a non-zero cache_read_price, input
// tokens are billed at the (cheaper) cache-read rate. If cache_read_price is
// unset (0), we fall back to the normal input price rather than billing input
// tokens for free.
func middlewareMetricEndpoint(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return "unknown"
	}
	if fullPath := c.FullPath(); fullPath != "" {
		return fullPath
	}
	if c.Request.URL.Path != "" {
		return c.Request.URL.Path
	}
	return "unknown"
}

func calculateCost(provider, model string, inputTokens, outputTokens int, cached bool) float64 {
	start := time.Now()
	result := "success"
	defer func() {
		metrics.RecordUsageCostLookup(provider, model, cached, result, time.Since(start))
	}()

	if mp, ok := defaultPricingCache.get(provider, model); ok {
		if mp == nil {
			result = "missing_pricing_cached"
			return 0
		}
		result = "cache_hit"
		return calculateCostFromPricing(mp, inputTokens, outputTokens, cached)
	}

	db := database.GetDB()
	if db == nil {
		result = "no_db"
		defaultPricingCache.put(provider, model, nil)
		return 0
	}
	mp, err := models.GetModelPricing(db, provider, model)
	if err != nil {
		result = "lookup_error"
		return 0
	}
	if mp == nil {
		result = "missing_pricing"
		defaultPricingCache.put(provider, model, nil)
		return 0
	}
	defaultPricingCache.put(provider, model, mp)
	result = "loaded"
	return calculateCostFromPricing(mp, inputTokens, outputTokens, cached)
}

func calculateCostFromPricing(mp *models.ModelPricing, inputTokens, outputTokens int, cached bool) float64 {
	inputPrice := mp.InputPrice
	if cached && mp.CacheReadPrice > 0 {
		inputPrice = mp.CacheReadPrice
	}
	// prices are cents per token; convert to USD
	return (float64(inputTokens)*inputPrice + float64(outputTokens)*mp.OutputPrice) / 100.0
}
