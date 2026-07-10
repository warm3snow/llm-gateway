// Package metrics defines the Prometheus metrics exposed by the gateway and
// helpers for recording them. All metrics are registered on the default
// Prometheus registry and served at /metrics.
//
// Cardinality discipline: labels are restricted to bounded-cardinality
// dimensions (provider, model, endpoint, status_class, cached, error kind).
// High-cardinality values such as request_id, virtual key name, or client_ip
// are deliberately NOT used as labels — they belong in the usage_records table,
// not in the metrics system.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const namespace = "llmgw"

var (
	// RequestsTotal counts proxied requests.
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "requests_total",
		Help:      "Total number of LLM requests processed by the gateway.",
	}, []string{"provider", "model", "endpoint", "status_class", "cached"})

	// RequestDuration observes end-to-end request latency in seconds.
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "request_duration_seconds",
		Help:      "End-to-end request latency in seconds.",
		// Buckets tuned for LLM latencies (100ms .. ~120s).
		Buckets: []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 30, 60, 120},
	}, []string{"provider", "model", "endpoint", "status_class"})

	// TimeToFirstToken observes streaming TTFT in seconds.
	TimeToFirstToken = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "time_to_first_token_seconds",
		Help:      "Time from request start to first streamed token, in seconds.",
		Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 20, 30},
	}, []string{"provider", "model"})

	// TokensTotal counts tokens, split by kind (input/output).
	TokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "tokens_total",
		Help:      "Total tokens processed, labeled by kind (input/output).",
	}, []string{"provider", "model", "kind"})

	// CostUSDTotal accumulates estimated cost in USD.
	CostUSDTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "cost_usd_total",
		Help:      "Estimated cumulative cost in USD.",
	}, []string{"provider", "model"})

	// UpstreamErrorsTotal counts upstream failures by classified kind.
	UpstreamErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "upstream_errors_total",
		Help:      "Upstream provider errors classified by kind.",
	}, []string{"provider", "kind"})

	// RetriesTotal counts retry attempts by reason.
	RetriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "retries_total",
		Help:      "Retry attempts performed against upstream providers, by reason.",
	}, []string{"provider", "reason"})

	// RetryResultTotal counts terminal retry outcomes (success/exhausted).
	RetryResultTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "retry_result_total",
		Help:      "Terminal outcome of retryable calls (success/exhausted).",
	}, []string{"provider", "result"})

	// InFlightRequests tracks concurrently in-flight requests.
	InFlightRequests = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "inflight_requests",
		Help:      "Number of requests currently being processed.",
	}, []string{"endpoint"})

	// RecordsDropped exposes usage-record entries dropped by the async writer.
	RecordsDropped = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "usage_records_dropped_total",
		Help:      "Usage-record entries dropped due to a full async buffer.",
	})

	// VirtualKeyAuthDuration observes virtual key authentication middleware latency.
	VirtualKeyAuthDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "virtual_key_auth_duration_seconds",
		Help:      "Virtual key authentication middleware latency in seconds.",
		Buckets:   []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	}, []string{"endpoint", "result"})

	// VirtualKeyActiveKeysLoaded observes active virtual keys loaded during auth.
	VirtualKeyActiveKeysLoaded = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "virtual_key_active_keys_loaded",
		Help:      "Number of active virtual keys loaded during virtual key authentication.",
		Buckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
	}, []string{"endpoint", "result"})

	// UsageRecordMiddlewareDuration observes usage-record middleware latency.
	UsageRecordMiddlewareDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "usage_record_middleware_duration_seconds",
		Help:      "Usage-record middleware latency in seconds.",
		Buckets:   []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	}, []string{"endpoint", "status_class"})

	// UsageRecordCostLookupDuration observes model-pricing lookup latency.
	UsageRecordCostLookupDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "usage_record_cost_lookup_duration_seconds",
		Help:      "Usage-record model-pricing lookup latency in seconds.",
		Buckets:   []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	}, []string{"provider", "cached", "result"})

	// UsageRecordTrackUsageDuration observes virtual-key budget tracking latency.
	UsageRecordTrackUsageDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "usage_record_track_usage_duration_seconds",
		Help:      "Virtual-key budget tracking latency from usage recording in seconds.",
		Buckets:   []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	}, []string{"result"})

	// UsageRecordRequestBodyBytes observes request body sizes seen by usage recording.
	UsageRecordRequestBodyBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "usage_record_request_body_bytes",
		Help:      "Request body size captured by usage recording.",
		Buckets:   prometheus.ExponentialBuckets(128, 2, 16),
	}, []string{"endpoint"})

	// UsageRecordResponseBodyBytes observes response body sizes captured by usage recording.
	UsageRecordResponseBodyBytes = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "usage_record_response_body_bytes",
		Help:      "Response body size captured by usage recording.",
		Buckets:   prometheus.ExponentialBuckets(128, 2, 18),
	}, []string{"endpoint"})

	// LogstoreEnqueueDuration observes usage-record enqueue latency.
	LogstoreEnqueueDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "logstore_enqueue_duration_seconds",
		Help:      "Usage-record logstore enqueue latency in seconds.",
		Buckets:   []float64{0.000001, 0.000005, 0.00001, 0.000025, 0.00005, 0.0001, 0.00025, 0.0005, 0.001, 0.005, 0.01},
	}, []string{"result"})

	// BudgetTrackerEnqueueTotal counts async budget tracker enqueue outcomes.
	BudgetTrackerEnqueueTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "budget_tracker_enqueue_total",
		Help:      "Async budget tracker enqueue attempts by result.",
	}, []string{"result"})

	// BudgetTrackerQueueDepth exposes queued budget updates awaiting batching.
	BudgetTrackerQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "budget_tracker_queue_depth",
		Help:      "Number of queued async budget updates awaiting processing.",
	})

	// BudgetTrackerPendingCostUSD exposes total unflushed budget cost.
	BudgetTrackerPendingCostUSD = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "budget_tracker_pending_cost_usd",
		Help:      "Total cost currently pending async budget flush.",
	})

	// BudgetTrackerFlushDuration observes async budget flush latency.
	BudgetTrackerFlushDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "budget_tracker_flush_duration_seconds",
		Help:      "Async budget tracker flush latency in seconds.",
		Buckets:   []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
	}, []string{"result"})

	// BudgetTrackerFlushUpdates observes update count per async budget flush.
	BudgetTrackerFlushUpdates = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "budget_tracker_flush_updates",
		Help:      "Number of virtual-key budget updates in each async budget flush.",
		Buckets:   []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
	}, []string{"result"})
)

// StatusClass maps an HTTP status code to a low-cardinality class label.
func StatusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	default:
		return "other"
	}
}

// ClassifyStatus maps an upstream HTTP status code to an error kind label.
// Returns "" for success (2xx) responses.
func ClassifyStatus(code int) string {
	switch {
	case code >= 200 && code < 300:
		return ""
	case code == http.StatusTooManyRequests:
		return "rate_limited"
	case code == http.StatusUnauthorized, code == http.StatusForbidden:
		return "auth"
	case code == http.StatusNotFound:
		return "not_found"
	case code == http.StatusRequestTimeout, code == http.StatusGatewayTimeout:
		return "timeout"
	case code >= 500:
		return "upstream_5xx"
	case code >= 400:
		return "client_4xx"
	default:
		return "other"
	}
}

// LabelOrUnknown normalizes an empty label value to "unknown", keeping
// provider/model label cardinality bounded and free of empty strings.
func LabelOrUnknown(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}

// RecordVirtualKeyAuth records virtual key authentication bottleneck metrics.
func RecordVirtualKeyAuth(endpoint, result string, activeKeys int, elapsed time.Duration) {
	endpoint = LabelOrUnknown(endpoint)
	result = LabelOrUnknown(result)
	VirtualKeyAuthDuration.WithLabelValues(endpoint, result).Observe(elapsed.Seconds())
	VirtualKeyActiveKeysLoaded.WithLabelValues(endpoint, result).Observe(float64(activeKeys))
}

// RecordUsageMiddleware records usage-record middleware latency.
func RecordUsageMiddleware(endpoint string, statusCode int, elapsed time.Duration) {
	UsageRecordMiddlewareDuration.WithLabelValues(LabelOrUnknown(endpoint), StatusClass(statusCode)).Observe(elapsed.Seconds())
}

// RecordUsageCostLookup records model-pricing lookup latency.
func RecordUsageCostLookup(provider, _ string, cached bool, result string, elapsed time.Duration) {
	UsageRecordCostLookupDuration.WithLabelValues(
		LabelOrUnknown(provider),
		strconv.FormatBool(cached),
		LabelOrUnknown(result),
	).Observe(elapsed.Seconds())
}

// RecordUsageTrackUsage records budget tracking latency.
func RecordUsageTrackUsage(result string, elapsed time.Duration) {
	UsageRecordTrackUsageDuration.WithLabelValues(LabelOrUnknown(result)).Observe(elapsed.Seconds())
}

// RecordUsageBodySizes records request and response body sizes captured by usage recording.
func RecordUsageBodySizes(endpoint string, requestBytes, responseBytes int) {
	endpoint = LabelOrUnknown(endpoint)
	UsageRecordRequestBodyBytes.WithLabelValues(endpoint).Observe(float64(requestBytes))
	UsageRecordResponseBodyBytes.WithLabelValues(endpoint).Observe(float64(responseBytes))
}

// RecordLogstoreEnqueue records usage-record enqueue latency.
func RecordLogstoreEnqueue(result string, elapsed time.Duration) {
	LogstoreEnqueueDuration.WithLabelValues(LabelOrUnknown(result)).Observe(elapsed.Seconds())
}

// RecordBudgetTrackerEnqueue records async budget enqueue outcomes.
func RecordBudgetTrackerEnqueue(result string, queueDepth int, pendingCost float64) {
	BudgetTrackerEnqueueTotal.WithLabelValues(LabelOrUnknown(result)).Inc()
	BudgetTrackerQueueDepth.Set(float64(queueDepth))
	BudgetTrackerPendingCostUSD.Set(pendingCost)
}

// RecordBudgetTrackerFlush records async budget flush outcomes.
func RecordBudgetTrackerFlush(result string, updates int, elapsed time.Duration, queueDepth int, pendingCost float64) {
	result = LabelOrUnknown(result)
	BudgetTrackerFlushDuration.WithLabelValues(result).Observe(elapsed.Seconds())
	BudgetTrackerFlushUpdates.WithLabelValues(result).Observe(float64(updates))
	BudgetTrackerQueueDepth.Set(float64(queueDepth))
	BudgetTrackerPendingCostUSD.Set(pendingCost)
}

// RecordBudgetTrackerState records async budget queue/pending state without counting an event.
func RecordBudgetTrackerState(queueDepth int, pendingCost float64) {
	BudgetTrackerQueueDepth.Set(float64(queueDepth))
	BudgetTrackerPendingCostUSD.Set(pendingCost)
}

// RecordRequest records the standard per-request metrics. latencySeconds is the
// total handling time; tokens and cost may be zero when unavailable.
func RecordRequest(provider, model, endpoint string, statusCode int, cached bool, latencySeconds float64, inputTokens, outputTokens int, costUSD float64) {
	p := LabelOrUnknown(provider)
	m := LabelOrUnknown(model)
	sc := StatusClass(statusCode)
	cachedLabel := strconv.FormatBool(cached)

	RequestsTotal.WithLabelValues(p, m, endpoint, sc, cachedLabel).Inc()
	RequestDuration.WithLabelValues(p, m, endpoint, sc).Observe(latencySeconds)

	if inputTokens > 0 {
		TokensTotal.WithLabelValues(p, m, "input").Add(float64(inputTokens))
	}
	if outputTokens > 0 {
		TokensTotal.WithLabelValues(p, m, "output").Add(float64(outputTokens))
	}
	if costUSD > 0 {
		CostUSDTotal.WithLabelValues(p, m).Add(costUSD)
	}

	if kind := ClassifyStatus(statusCode); kind != "" {
		UpstreamErrorsTotal.WithLabelValues(p, kind).Inc()
	}
}
