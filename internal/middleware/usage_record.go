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

		// Wrap the response writer to capture the body
		writer := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
			status:         http.StatusOK,
		}
		c.Writer = writer

		c.Next()

		// Extract provider: header > config default
		provider := c.GetHeader("x-llm-provider")
		if provider == "" {
			provider = defaultProvider
		}

		// Extract model from the saved request body
		model := extractModelFromBytes(requestBody, c)

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
			return
		}

		// Get virtual key info from context (set by VirtualKeyAuth middleware)
		var virtualKeyID *uint
		var virtualKeyName string
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

		// Decrement the virtual key's budget synchronously so budgets stay
		// consistent with the recorded cost.
		if virtualKeyService != nil && virtualKeyID != nil && cost > 0 {
			_ = virtualKeyService.TrackUsage(*virtualKeyID, cost)
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
			RequestID:      requestID,
			VirtualKeyID:   virtualKeyID,
			VirtualKeyName: virtualKeyName,
			Provider:       provider,
			Model:          model,
			Endpoint:       c.Request.URL.Path,
			StatusCode:     status,
			ErrorMessage:   errorMessage,
			InputTokens:    inputTokens,
			OutputTokens:   outputTokens,
			Cost:           cost,
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
	}
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
func calculateCost(provider, model string, inputTokens, outputTokens int, cached bool) float64 {
	db := database.GetDB()
	if db == nil {
		return 0
	}
	mp, err := models.GetModelPricing(db, provider, model)
	if err != nil || mp == nil {
		return 0
	}
	inputPrice := mp.InputPrice
	if cached && mp.CacheReadPrice > 0 {
		inputPrice = mp.CacheReadPrice
	}
	// prices are cents per token; convert to USD
	return (float64(inputTokens)*inputPrice + float64(outputTokens)*mp.OutputPrice) / 100.0
}
