package metrics

import (
	"database/sql"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type databasePoolCollector struct {
	mu      sync.RWMutex
	driver  string
	statsFn func() sql.DBStats

	maxOpenConnections *prometheus.Desc
	openConnections    *prometheus.Desc
	inUseConnections   *prometheus.Desc
	idleConnections    *prometheus.Desc
	waitCount          *prometheus.Desc
	waitDuration       *prometheus.Desc
	maxIdleClosed      *prometheus.Desc
	maxLifetimeClosed  *prometheus.Desc
}

var defaultDatabasePoolCollector = newDatabasePoolCollector()

func init() {
	prometheus.MustRegister(defaultDatabasePoolCollector)
}

func ObserveDatabasePool(driver string, db *sql.DB) {
	if db == nil {
		defaultDatabasePoolCollector.Clear()
		return
	}
	defaultDatabasePoolCollector.Set(driver, db.Stats)
}

func newDatabasePoolCollector() *databasePoolCollector {
	labels := []string{"driver"}
	return &databasePoolCollector{
		maxOpenConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "database_pool", "max_open_connections"),
			"Configured maximum number of open database connections.",
			labels,
			nil,
		),
		openConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "database_pool", "open_connections"),
			"Number of established database connections, both in use and idle.",
			labels,
			nil,
		),
		inUseConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "database_pool", "in_use_connections"),
			"Number of database connections currently in use.",
			labels,
			nil,
		),
		idleConnections: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "database_pool", "idle_connections"),
			"Number of idle database connections.",
			labels,
			nil,
		),
		waitCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "database_pool", "wait_count_total"),
			"Total number of times database callers waited for a connection.",
			labels,
			nil,
		),
		waitDuration: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "database_pool", "wait_duration_seconds_total"),
			"Total time database callers waited for a connection, in seconds.",
			labels,
			nil,
		),
		maxIdleClosed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "database_pool", "max_idle_closed_total"),
			"Total number of database connections closed due to SetMaxIdleConns.",
			labels,
			nil,
		),
		maxLifetimeClosed: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, "database_pool", "max_lifetime_closed_total"),
			"Total number of database connections closed due to SetConnMaxLifetime.",
			labels,
			nil,
		),
	}
}

func (c *databasePoolCollector) Set(driver string, statsFn func() sql.DBStats) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.driver = LabelOrUnknown(driver)
	c.statsFn = statsFn
}

func (c *databasePoolCollector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.driver = ""
	c.statsFn = nil
}

func (c *databasePoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.maxOpenConnections
	ch <- c.openConnections
	ch <- c.inUseConnections
	ch <- c.idleConnections
	ch <- c.waitCount
	ch <- c.waitDuration
	ch <- c.maxIdleClosed
	ch <- c.maxLifetimeClosed
}

func (c *databasePoolCollector) Collect(ch chan<- prometheus.Metric) {
	driver, statsFn := c.snapshot()
	if statsFn == nil {
		return
	}
	stats := statsFn()

	ch <- prometheus.MustNewConstMetric(c.maxOpenConnections, prometheus.GaugeValue, float64(stats.MaxOpenConnections), driver)
	ch <- prometheus.MustNewConstMetric(c.openConnections, prometheus.GaugeValue, float64(stats.OpenConnections), driver)
	ch <- prometheus.MustNewConstMetric(c.inUseConnections, prometheus.GaugeValue, float64(stats.InUse), driver)
	ch <- prometheus.MustNewConstMetric(c.idleConnections, prometheus.GaugeValue, float64(stats.Idle), driver)
	ch <- prometheus.MustNewConstMetric(c.waitCount, prometheus.CounterValue, float64(stats.WaitCount), driver)
	ch <- prometheus.MustNewConstMetric(c.waitDuration, prometheus.CounterValue, stats.WaitDuration.Seconds(), driver)
	ch <- prometheus.MustNewConstMetric(c.maxIdleClosed, prometheus.CounterValue, float64(stats.MaxIdleClosed), driver)
	ch <- prometheus.MustNewConstMetric(c.maxLifetimeClosed, prometheus.CounterValue, float64(stats.MaxLifetimeClosed), driver)
}

func (c *databasePoolCollector) snapshot() (string, func() sql.DBStats) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.driver, c.statsFn
}
