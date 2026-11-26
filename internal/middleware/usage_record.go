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
	"github.com/warm3snow/llm-gateway/internal/types"
)

// StreamUsageKey is the gin context key under which the streaming proxy
// handler stores the final usage block (set by pkg/proxy.streamResponse).
const StreamUsageKey = "stream_usage"

// TriesKey is the gin context key under which the proxy handler stores how
// many upstream attempts (primary + fallbacks) the request used.
const TriesKey = "upstream_tries"

// RedactionsKey is the gin context key under which the masking middleware
// stores how many values it replaced in the request.
const RedactionsKey = "redactions_count"

func streamUsageToTokens(v any) usageTokens {
	switch su := v.(type) {
	case types.Usage:
		return usageTokens{Input: su.PromptTokens, Output: su.CompletionTokens, Total: su.TotalTokens, CacheRead: su.CacheRead, CacheWrite: su.CacheWrite, Reasoning: su.Reasoning}
	case *types.Usage:
		if su == nil {
			return usageTokens{}
		}
		return usageTokens{Input: su.PromptTokens, Output: su.CompletionTokens, Total: su.TotalTokens, CacheRead: su.CacheRead, CacheWrite: su.CacheWrite, Reasoning: su.Reasoning}
	}
	return usageTokens{}
}

func triesFromContext(c *gin.Context) int {
	if v, ok := c.Get(TriesKey); ok {
		if n, ok := v.(int); ok && n > 0 {
			return n
		}
	}
	return 1
}

func redactionsFromContext(c *gin.Context) int {
	if v, ok := c.Get(RedactionsKey); ok {
		if n, ok := v.(int); ok && n > 0 {
			return n
		}
	}
	return 0
}

// usageTokens is one call's token counts as the usage middleware needs them.
// Input and Output exclude cache-read/cache-write prompt tokens; Reasoning is
// a subset of Output.
type usageTokens struct {
	Input      int
	Output     int
	Total      int
	CacheRead  int
	CacheWrite int
	Reasoning  int
}

func (u usageTokens) empty() bool {
	return u.Input == 0 && u.Output == 0 && u.CacheRead == 0 && u.CacheWrite == 0
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
		tokens := usageTokens{}
		if u, exists := c.Get(StreamUsageKey); exists {
			tokens = streamUsageToTokens(u)
		}
		if tokens.empty() {
			tokens = parseUsage(writer.body.Bytes())
		}

		// Detect a cache hit for metrics purposes: CacheMiddleware sets the
		// "x-cache" response header to HIT/MISS.
		cached := c.Writer.Header().Get("x-cache") == "HIT"

		cost := calculateCost(provider, model, tokens.Input, tokens.Output, tokens.CacheRead, tokens.CacheWrite)

		// Recording gate: only persist actual model invocations — those that
		// produced token usage, or failed upstream. Everything else that
		// merely hit the route (e.g. GET /v1/models) is skipped.
		status := writer.status
		if tokens.empty() && status < 400 {
			recordMiddlewareMetrics()
			return
		}

		// Get virtual key info from context (set by VirtualKeyAuth middleware)
		var virtualKeyID *uint
		var virtualKeyName string
		var virtualKeyUserID *uint
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
		if id, exists := c.Get("virtual_key_user_id"); exists {
			if v, ok := id.(*uint); ok {
				virtualKeyUserID = v
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
			UserID:                      virtualKeyUserID,
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
			InputTokens:                 tokens.Input,
			OutputTokens:                tokens.Output,
			CacheReadTokens:             tokens.CacheRead,
			CacheWriteTokens:            tokens.CacheWrite,
			ReasoningTokens:             tokens.Reasoning,
			Cost:                        cost,
			LatencyMS:                   time.Since(start).Milliseconds(),
			Tries:                       triesFromContext(c),
			Redactions:                  redactionsFromContext(c),
		}

		// Record real-time Prometheus metrics. Use the route template
		// (FullPath) as the endpoint label to keep cardinality bounded;
		// fall back to the raw path only when no route matched.
		endpointLabel := c.FullPath()
		if endpointLabel == "" {
			endpointLabel = "unmatched"
		}
		elapsed := time.Since(start)
		metrics.RecordRequest(provider, model, endpointLabel, status, cached, elapsed.Seconds(), tokens.Input+tokens.CacheRead+tokens.CacheWrite, tokens.Output, cost)

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

// parseUsage extracts token counts from a response body. It understands the
// OpenAI Chat Completions shape (including prompt_tokens_details /
// completion_tokens_details), the Anthropic Messages shape, and the Gemini
// generateContent shape (usageMetadata). Non-JSON bodies parse as zero.
func parseUsage(body []byte) usageTokens {
	var resp struct {
		// OpenAI chat.completion
		Usage *struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
			PromptDetails    *struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CompletionDetails *struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
		// Anthropic messages: usage at the top level
		InputTokens  *int `json:"input_tokens"`
		OutputTokens *int `json:"output_tokens"`
		CacheRead    *int `json:"cache_read_input_tokens"`
		CacheWrite   *int `json:"cache_creation_input_tokens"`
		// Gemini generateContent: usageMetadata at the top level
		UsageMetadata *struct {
			PromptTokenCount        int `json:"promptTokenCount"`
			CandidatesTokenCount    int `json:"candidatesTokenCount"`
			TotalTokenCount         int `json:"totalTokenCount"`
			CachedContentTokenCount int `json:"cachedContentTokenCount"`
			ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return usageTokens{}
	}

	if u := resp.Usage; u != nil {
		var cacheRead, reasoning int
		if u.PromptDetails != nil {
			cacheRead = u.PromptDetails.CachedTokens
		}
		if u.CompletionDetails != nil {
			reasoning = u.CompletionDetails.ReasoningTokens
		}
		out := usageTokens{
			Input:     u.PromptTokens,
			Output:    u.CompletionTokens,
			Total:     u.TotalTokens,
			CacheRead: cacheRead,
			Reasoning: reasoning,
		}
		if out.Total == 0 {
			out.Total = out.Input + out.Output
		}
		return out
	}

	if resp.InputTokens != nil || resp.OutputTokens != nil {
		in, out := derefInt(resp.InputTokens), derefInt(resp.OutputTokens)
		return usageTokens{
			Input:      in,
			Output:     out,
			Total:      in + out,
			CacheRead:  derefInt(resp.CacheRead),
			CacheWrite: derefInt(resp.CacheWrite),
		}
	}

	if resp.UsageMetadata != nil {
		m := resp.UsageMetadata
		in := m.PromptTokenCount
		out := m.CandidatesTokenCount
		if m.CachedContentTokenCount > 0 && m.CachedContentTokenCount <= in {
			in -= m.CachedContentTokenCount
		}
		return usageTokens{
			Input:     in,
			Output:    out,
			Total:     m.TotalTokenCount,
			CacheRead: m.CachedContentTokenCount,
			Reasoning: m.ThoughtsTokenCount,
		}
	}

	return usageTokens{}
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
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

func calculateCost(provider, model string, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) float64 {
	start := time.Now()
	result := "success"
	defer func() {
		metrics.RecordUsageCostLookup(provider, model, cacheReadTokens > 0, result, time.Since(start))
	}()

	db := database.GetDB()
	if db == nil {
		result = "no_db"
		return 0
	}
	defaultPricingCache.bind(db)

	if mp, ok := defaultPricingCache.get(provider, model); ok {
		if mp == nil {
			result = "missing_pricing_cached"
			return 0
		}
		result = "cache_hit"
		return calculateCostFromPricing(mp, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
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
	return calculateCostFromPricing(mp, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
}

// calculateCostFromPricing prices a call:
//
//	prompt tokens beyond cache reads/writes at InputPrice,
//	cache-read tokens at CacheReadPrice (InputPrice when unset),
//	cache-write tokens at CacheWritePrice (InputPrice when unset),
//	output tokens at OutputPrice.
//
// Prices are cents per token; the result is USD.
func calculateCostFromPricing(mp *models.ModelPricing, inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens int) float64 {
	inputPrice := mp.InputPrice
	cacheReadPrice := mp.CacheReadPrice
	if cacheReadPrice <= 0 {
		cacheReadPrice = inputPrice
	}
	cacheWritePrice := mp.CacheWritePrice
	if cacheWritePrice <= 0 {
		cacheWritePrice = inputPrice
	}
	plainInput := inputTokens - cacheReadTokens - cacheWriteTokens
	if plainInput < 0 {
		plainInput = 0
	}
	cost := float64(plainInput)*inputPrice +
		float64(cacheReadTokens)*cacheReadPrice +
		float64(cacheWriteTokens)*cacheWritePrice +
		float64(outputTokens)*mp.OutputPrice
	return cost / 100.0
}
