package metrics

import (
	"database/sql"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_model/go"
)

func TestBottleneckMetricHelpersCreateSeries(t *testing.T) {
	RecordVirtualKeyAuth("/v1/chat/completions", "success", 7, 10*time.Millisecond)
	RecordUsageMiddleware("/v1/chat/completions", 200, 20*time.Millisecond)
	RecordUsageCostLookup("openai", "gpt-test", false, "success", time.Millisecond)
	RecordUsageTrackUsage("success", time.Millisecond)
	RecordUsageBodySizes("/v1/chat/completions", 123, 456)
	RecordLogstoreEnqueue("enqueued", time.Millisecond)
	RecordBudgetTrackerEnqueue("queued", 3, 1.25)
	RecordBudgetTrackerFlush("success", 2, time.Millisecond, 1, 0.5)

	collectors := []prometheus.Collector{
		VirtualKeyAuthDuration,
		VirtualKeyActiveKeysLoaded,
		UsageRecordMiddlewareDuration,
		UsageRecordCostLookupDuration,
		UsageRecordTrackUsageDuration,
		UsageRecordRequestBodyBytes,
		UsageRecordResponseBodyBytes,
		LogstoreEnqueueDuration,
		BudgetTrackerEnqueueTotal,
		BudgetTrackerQueueDepth,
		BudgetTrackerPendingCostUSD,
		BudgetTrackerFlushDuration,
		BudgetTrackerFlushUpdates,
	}
	for _, collector := range collectors {
		if count := collectCount(collector); count == 0 {
			t.Fatalf("expected collector to expose at least one metric")
		}
	}
}

func TestDatabasePoolCollectorExposesSQLStats(t *testing.T) {
	collector := newDatabasePoolCollector()
	collector.Set("postgres", func() sql.DBStats {
		return sql.DBStats{
			MaxOpenConnections: 40,
			OpenConnections:    31,
			InUse:              25,
			Idle:               6,
			WaitCount:          12,
			WaitDuration:       150 * time.Millisecond,
			MaxIdleClosed:      3,
			MaxLifetimeClosed:  2,
		}
	})

	metrics := collectMetrics(collector)
	assertMetricValue(t, metrics, "llmgw_database_pool_max_open_connections", 40)
	assertMetricValue(t, metrics, "llmgw_database_pool_open_connections", 31)
	assertMetricValue(t, metrics, "llmgw_database_pool_in_use_connections", 25)
	assertMetricValue(t, metrics, "llmgw_database_pool_idle_connections", 6)
	assertMetricValue(t, metrics, "llmgw_database_pool_wait_count_total", 12)
	assertMetricValue(t, metrics, "llmgw_database_pool_wait_duration_seconds_total", 0.15)
	assertMetricValue(t, metrics, "llmgw_database_pool_max_idle_closed_total", 3)
	assertMetricValue(t, metrics, "llmgw_database_pool_max_lifetime_closed_total", 2)
}

func collectCount(collector prometheus.Collector) int {
	return len(collectMetrics(collector))
}

func collectMetrics(collector prometheus.Collector) []*io_prometheus_client.MetricFamily {
	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		panic(err)
	}
	metrics, err := registry.Gather()
	if err != nil {
		panic(err)
	}
	return metrics
}

func assertMetricValue(t *testing.T, families []*io_prometheus_client.MetricFamily, name string, want float64) {
	t.Helper()
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		if len(family.Metric) != 1 {
			t.Fatalf("metric %s series count = %d, want 1", name, len(family.Metric))
		}
		metric := family.Metric[0]
		if len(metric.Label) != 1 || metric.Label[0].GetName() != "driver" || metric.Label[0].GetValue() != "postgres" {
			t.Fatalf("metric %s labels = %+v", name, metric.Label)
		}
		var got float64
		if metric.Gauge != nil {
			got = metric.Gauge.GetValue()
		} else if metric.Counter != nil {
			got = metric.Counter.GetValue()
		} else {
			t.Fatalf("metric %s has neither gauge nor counter value", name)
		}
		if got != want {
			t.Fatalf("metric %s = %v, want %v", name, got, want)
		}
		return
	}
	t.Fatalf("metric %s not found", name)
}
