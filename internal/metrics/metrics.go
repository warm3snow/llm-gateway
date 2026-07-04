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
	RecordsDropped = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "usage_records_dropped_total",
		Help:      "Usage-record entries dropped due to a full async buffer.",
	})
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
