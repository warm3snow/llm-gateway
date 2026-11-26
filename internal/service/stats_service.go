package service

import (
	"time"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"gorm.io/gorm"
)

// StatsService handles statistics queries, derived from usage records.
type StatsService struct {
	db *gorm.DB
}

// NewStatsService creates a new StatsService
func NewStatsService() *StatsService {
	return &StatsService{
		db: database.GetDB(),
	}
}

// OverviewStats matches the frontend DashboardStats interface
type OverviewStats struct {
	TotalRequests     int64   `json:"totalRequests"`
	TotalTokens       int64   `json:"totalTokens"`
	TotalCost         float64 `json:"totalCost"`
	ActiveProviders   int     `json:"activeProviders"`
	ActiveVirtualKeys int64   `json:"activeVirtualKeys"`
	SuccessRate       float64 `json:"successRate"`
	CacheReadTokens   int64   `json:"cacheReadTokens"`
	CacheWriteTokens  int64   `json:"cacheWriteTokens"`
	ReasoningTokens   int64   `json:"reasoningTokens"`
	TotalRedactions   int64   `json:"totalRedactions"`
}

// summary holds the aggregate stats shared by the overview and analytics
// endpoints.
type summary struct {
	TotalRequests     int64
	TotalTokens       int64
	TotalCost         float64
	ActiveProviders   int
	ActiveVirtualKeys int64
	SuccessRate       float64
	CacheReadTokens   int64
	CacheWriteTokens  int64
	ReasoningTokens   int64
	TotalRedactions   int64
}

// computeSummary runs the aggregate queries over usage records that back both
// GetOverview and GetAnalytics, scoped to the caller.
func (s *StatsService) computeSummary(scope AccessScope) summary {
	var sum summary

	// Guard against an uninitialized DB (e.g. in unit tests that don't spin up
	// a database) — return zero-valued stats rather than panicking.
	if s.db == nil {
		return sum
	}

	usageQuery := func() *gorm.DB {
		return scope.ApplyUsage(s.db.Model(&models.UsageRecord{}))
	}

	usageQuery().Count(&sum.TotalRequests)

	var successCount int64
	usageQuery().Where("status_code >= 200 AND status_code < 300").Count(&successCount)
	if sum.TotalRequests > 0 {
		sum.SuccessRate = float64(successCount) / float64(sum.TotalRequests) * 100
	}

	var tokens struct {
		TotalInput      int64 `gorm:"column:total_input_tokens"`
		TotalOutput     int64 `gorm:"column:total_output_tokens"`
		TotalCacheRead  int64 `gorm:"column:total_cache_read_tokens"`
		TotalCacheWrite int64 `gorm:"column:total_cache_write_tokens"`
		TotalReasoning  int64 `gorm:"column:total_reasoning_tokens"`
	}
	usageQuery().Select(
		"SUM(input_tokens) as total_input_tokens, SUM(output_tokens) as total_output_tokens, " +
			"COALESCE(SUM(cache_read_tokens), 0) as total_cache_read_tokens, " +
			"COALESCE(SUM(cache_write_tokens), 0) as total_cache_write_tokens, " +
			"COALESCE(SUM(reasoning_tokens), 0) as total_reasoning_tokens",
	).Scan(&tokens)
	sum.TotalTokens = tokens.TotalInput + tokens.TotalOutput
	sum.CacheReadTokens = tokens.TotalCacheRead
	sum.CacheWriteTokens = tokens.TotalCacheWrite
	sum.ReasoningTokens = tokens.TotalReasoning

	usageQuery().Select("COALESCE(SUM(cost), 0)").Scan(&sum.TotalCost)
	usageQuery().Select("COALESCE(SUM(redactions), 0)").Scan(&sum.TotalRedactions)

	var providerCount int64
	usageQuery().Distinct("provider").Count(&providerCount)
	sum.ActiveProviders = int(providerCount)

	scope.ApplyVirtualKeys(s.db.Model(&models.VirtualKey{})).Where("status = ?", "active").Count(&sum.ActiveVirtualKeys)

	return sum
}

// GetOverview returns stats for the dashboard, scoped to the caller.
func (s *StatsService) GetOverview(scope AccessScope) (*OverviewStats, error) {
	sum := s.computeSummary(scope)
	return &OverviewStats{
		TotalRequests:     sum.TotalRequests,
		TotalTokens:       sum.TotalTokens,
		TotalCost:         sum.TotalCost,
		ActiveProviders:   sum.ActiveProviders,
		ActiveVirtualKeys: sum.ActiveVirtualKeys,
		SuccessRate:       sum.SuccessRate,
		CacheReadTokens:   sum.CacheReadTokens,
		CacheWriteTokens:  sum.CacheWriteTokens,
		ReasoningTokens:   sum.ReasoningTokens,
		TotalRedactions:   sum.TotalRedactions,
	}, nil
}

// TimeSeriesPoint is a data point for charts
type TimeSeriesPoint struct {
	Date  string  `json:"date"`
	Count int64   `json:"count"`
	Cost  float64 `json:"cost"`
}

// TopModel represents a model ranking entry
type TopModel struct {
	Model  string  `json:"model"`
	Count  int64   `json:"count"`
	Tokens int64   `json:"tokens"`
	Cost   float64 `json:"cost"`
}

// TopProvider represents a provider ranking entry
type TopProvider struct {
	Provider string  `json:"provider"`
	Count    int64   `json:"count"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

// PeriodSummary is a period-window summary: today / 7d / 30d / all.
type PeriodSummary struct {
	Period           string            `json:"period"`
	Since            string            `json:"since"`
	TotalRequests    int64             `json:"totalRequests"`
	Errors           int64             `json:"errors"`
	InputTokens      int64             `json:"inputTokens"`
	OutputTokens     int64             `json:"outputTokens"`
	CacheReadTokens  int64             `json:"cacheReadTokens"`
	CacheWriteTokens int64             `json:"cacheWriteTokens"`
	ReasoningTokens  int64             `json:"reasoningTokens"`
	Cost             float64           `json:"cost"`
	TotalRedactions  int64             `json:"totalRedactions"`
	Series           []TimeSeriesPoint `json:"series"`
	TopModels        []TopModel        `json:"topModels"`
	TopProviders     []TopProvider     `json:"topProviders"`
}

// AnalyticsData combines all analytics data for the frontend
type AnalyticsData struct {
	TotalRequests     int64             `json:"totalRequests"`
	TotalTokens       int64             `json:"totalTokens"`
	TotalCost         float64           `json:"totalCost"`
	ActiveProviders   int               `json:"activeProviders"`
	ActiveVirtualKeys int64             `json:"activeVirtualKeys"`
	SuccessRate       float64           `json:"successRate"`
	CacheReadTokens   int64             `json:"cacheReadTokens"`
	CacheWriteTokens  int64             `json:"cacheWriteTokens"`
	ReasoningTokens   int64             `json:"reasoningTokens"`
	TotalRedactions   int64             `json:"totalRedactions"`
	TimeSeries        []TimeSeriesPoint `json:"timeSeries"`
	TopModels         []TopModel        `json:"topModels"`
	TopProviders      []TopProvider     `json:"topProviders"`
	MaxCount          int64             `json:"maxCount"`
}

// GetAnalytics returns combined analytics data, scoped to the caller.
func (s *StatsService) GetAnalytics(scope AccessScope) (*AnalyticsData, error) {
	sum := s.computeSummary(scope)
	data := &AnalyticsData{
		TotalRequests:     sum.TotalRequests,
		TotalTokens:       sum.TotalTokens,
		TotalCost:         sum.TotalCost,
		ActiveProviders:   sum.ActiveProviders,
		ActiveVirtualKeys: sum.ActiveVirtualKeys,
		SuccessRate:       sum.SuccessRate,
		CacheReadTokens:   sum.CacheReadTokens,
		CacheWriteTokens:  sum.CacheWriteTokens,
		ReasoningTokens:   sum.ReasoningTokens,
		TotalRedactions:   sum.TotalRedactions,
	}

	data.TimeSeries = []TimeSeriesPoint{}
	data.TopModels = []TopModel{}
	data.TopProviders = []TopProvider{}
	if s.db == nil {
		return data, nil
	}

	usageQuery := func() *gorm.DB {
		return scope.ApplyUsage(s.db.Model(&models.UsageRecord{}))
	}

	// Time series (last 7 days)
	end := time.Now()
	start := end.AddDate(0, 0, -7)

	var timePoints []struct {
		Timestamp string  `json:"timestamp"`
		Count     int64   `json:"count"`
		Cost      float64 `json:"cost"`
	}
	usageQuery().
		Select("DATE(created_at) as timestamp, COUNT(*) as count, COALESCE(SUM(cost), 0) as cost").
		Where("created_at >= ? AND created_at <= ?", start, end).
		Group("DATE(created_at)").
		Order("timestamp ASC").
		Scan(&timePoints)

	for _, p := range timePoints {
		data.TimeSeries = append(data.TimeSeries, TimeSeriesPoint{
			Date:  p.Timestamp,
			Count: p.Count,
			Cost:  p.Cost,
		})
		if p.Count > data.MaxCount {
			data.MaxCount = p.Count
		}
	}

	// Top models with tokens and cost
	var topModels []struct {
		Model  string  `gorm:"column:model"`
		Count  int64   `gorm:"column:count"`
		Tokens int64   `gorm:"column:tokens"`
		Cost   float64 `gorm:"column:cost"`
	}
	usageQuery().
		Select("model, COUNT(*) as count, COALESCE(SUM(input_tokens + output_tokens + cache_read_tokens + cache_write_tokens), 0) as tokens, COALESCE(SUM(cost), 0) as cost").
		Where("model != ''").
		Group("model").
		Order("count DESC").
		Limit(10).
		Scan(&topModels)
	for _, m := range topModels {
		data.TopModels = append(data.TopModels, TopModel{Model: m.Model, Count: m.Count, Tokens: m.Tokens, Cost: m.Cost})
	}

	// Top providers with tokens and cost
	var topProviders []struct {
		Provider string  `gorm:"column:provider"`
		Count    int64   `gorm:"column:count"`
		Tokens   int64   `gorm:"column:tokens"`
		Cost     float64 `gorm:"column:cost"`
	}
	usageQuery().
		Select("provider, COUNT(*) as count, COALESCE(SUM(input_tokens + output_tokens + cache_read_tokens + cache_write_tokens), 0) as tokens, COALESCE(SUM(cost), 0) as cost").
		Group("provider").
		Order("count DESC").
		Limit(10).
		Scan(&topProviders)
	for _, p := range topProviders {
		data.TopProviders = append(data.TopProviders, TopProvider{Provider: p.Provider, Count: p.Count, Tokens: p.Tokens, Cost: p.Cost})
	}

	return data, nil
}

// periodWindow maps a period name to [start, end]; end is zero for "all"
// (unbounded). Unknown names default to "all".
func periodWindow(period string, now time.Time) (string, time.Time, time.Time) {
	start := time.Time{}
	switch period {
	case "today":
		y, m, d := now.Date()
		start = time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	case "7d":
		start = now.AddDate(0, 0, -7)
	case "30d":
		start = now.AddDate(0, 0, -30)
	default:
		period = "all"
	}
	return period, start, time.Time{}
}

// GetPeriodSummary returns a period-window summary (today/7d/30d/all) with a
// daily series and top models/providers, scoped to the caller.
func (s *StatsService) GetPeriodSummary(scope AccessScope, period string) (*PeriodSummary, error) {
	now := time.Now()
	period, start, _ := periodWindow(period, now)
	out := &PeriodSummary{
		Period:       period,
		Series:       []TimeSeriesPoint{},
		TopModels:    []TopModel{},
		TopProviders: []TopProvider{},
	}
	if s.db == nil {
		return out, nil
	}

	usageQuery := func() *gorm.DB {
		q := scope.ApplyUsage(s.db.Model(&models.UsageRecord{}))
		if !start.IsZero() {
			q = q.Where("created_at >= ?", start)
		}
		return q
	}

	var agg struct {
		TotalRequests    int64   `gorm:"column:total_requests"`
		Errors           int64   `gorm:"column:errors"`
		InputTokens      int64   `gorm:"column:input_tokens"`
		OutputTokens     int64   `gorm:"column:output_tokens"`
		CacheReadTokens  int64   `gorm:"column:cache_read_tokens"`
		CacheWriteTokens int64   `gorm:"column:cache_write_tokens"`
		ReasoningTokens  int64   `gorm:"column:reasoning_tokens"`
		Cost             float64 `gorm:"column:cost"`
		Redactions       int64   `gorm:"column:redactions"`
	}
	usageQuery().Select(
		"COUNT(*) as total_requests, " +
			"COALESCE(SUM(CASE WHEN status_code >= 400 THEN 1 ELSE 0 END), 0) as errors, " +
			"COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, " +
			"COALESCE(SUM(cache_read_tokens), 0) as cache_read_tokens, COALESCE(SUM(cache_write_tokens), 0) as cache_write_tokens, " +
			"COALESCE(SUM(reasoning_tokens), 0) as reasoning_tokens, " +
			"COALESCE(SUM(cost), 0) as cost, COALESCE(SUM(redactions), 0) as redactions",
	).Scan(&agg)
	out.TotalRequests = agg.TotalRequests
	out.Errors = agg.Errors
	out.InputTokens = agg.InputTokens
	out.OutputTokens = agg.OutputTokens
	out.CacheReadTokens = agg.CacheReadTokens
	out.CacheWriteTokens = agg.CacheWriteTokens
	out.ReasoningTokens = agg.ReasoningTokens
	out.Cost = agg.Cost
	out.TotalRedactions = agg.Redactions
	if !start.IsZero() {
		out.Since = start.Format(time.RFC3339)
	}

	// Daily series over the window.
	groupExpr := "DATE(created_at)"
	if s.db.Dialector != nil && s.db.Dialector.Name() == "postgres" {
		groupExpr = "to_char(date_trunc('day', created_at), 'YYYY-MM-DD')"
	}
	var rows []struct {
		Timestamp string  `gorm:"column:timestamp"`
		Count     int64   `gorm:"column:count"`
		Cost      float64 `gorm:"column:cost"`
	}
	usageQuery().
		Select(groupExpr + " as timestamp, COUNT(*) as count, COALESCE(SUM(cost), 0) as cost").
		Group(groupExpr).
		Order("timestamp ASC").
		Scan(&rows)
	for _, r := range rows {
		out.Series = append(out.Series, TimeSeriesPoint{Date: r.Timestamp, Count: r.Count, Cost: r.Cost})
	}

	var topModels []struct {
		Model string  `gorm:"column:model"`
		Count int64   `gorm:"column:count"`
		Cost  float64 `gorm:"column:cost"`
	}
	usageQuery().
		Select("model, COUNT(*) as count, COALESCE(SUM(cost), 0) as cost").
		Where("model != ''").
		Group("model").Order("cost DESC").Limit(10).Scan(&topModels)
	for _, m := range topModels {
		out.TopModels = append(out.TopModels, TopModel{Model: m.Model, Count: m.Count, Cost: m.Cost})
	}

	var topProviders []struct {
		Provider string  `gorm:"column:provider"`
		Count    int64   `gorm:"column:count"`
		Cost     float64 `gorm:"column:cost"`
	}
	usageQuery().
		Select("provider, COUNT(*) as count, COALESCE(SUM(cost), 0) as cost").
		Group("provider").Order("cost DESC").Limit(10).Scan(&topProviders)
	for _, p := range topProviders {
		out.TopProviders = append(out.TopProviders, TopProvider{Provider: p.Provider, Count: p.Count, Cost: p.Cost})
	}

	return out, nil
}

// hourGroupExpr returns the DB-specific SQL expression that truncates
// created_at to the hour. SQLite and PostgreSQL differ here, so we branch on
// the active dialect.
func (s *StatsService) hourGroupExpr() string {
	if s.db != nil && s.db.Dialector != nil && s.db.Dialector.Name() == "postgres" {
		return "to_char(date_trunc('hour', created_at), 'YYYY-MM-DD HH24:00')"
	}
	// SQLite (and compatible) strftime.
	return "strftime('%Y-%m-%d %H:00', created_at)"
}

// GetHourlyTimeSeries returns invocation counts and cost bucketed by hour over
// [start, end], for finer-grained dashboards and alerting.
func (s *StatsService) GetHourlyTimeSeries(scope AccessScope, start, end time.Time) ([]TimeSeriesPoint, error) {
	return s.GetHourlyTimeSeriesWithStatus(scope, start, end, "", 0)
}

// GetHourlyTimeSeriesWithStatus returns hourly invocation counts and cost with
// either an exact status_code filter or a status family filter: success/error.
func (s *StatsService) GetHourlyTimeSeriesWithStatus(scope AccessScope, start, end time.Time, status string, statusCode int) ([]TimeSeriesPoint, error) {
	return s.timeSeries(scope, s.hourGroupExpr(), start, end, status, statusCode)
}

// timeSeries returns invocation counts and cost grouped by groupExpr over the
// optional [start, end] window, scoped to the caller.
func (s *StatsService) timeSeries(scope AccessScope, groupExpr string, start, end time.Time, status string, statusCode int) ([]TimeSeriesPoint, error) {
	points := []TimeSeriesPoint{}
	if s.db == nil {
		return points, nil
	}

	var rows []struct {
		Timestamp string  `gorm:"column:timestamp"`
		Count     int64   `gorm:"column:count"`
		Cost      float64 `gorm:"column:cost"`
	}

	query := scope.ApplyUsage(s.db.Model(&models.UsageRecord{})).
		Select(groupExpr + " as timestamp, COUNT(*) as count, COALESCE(SUM(cost), 0) as cost").
		Group(groupExpr).
		Order("timestamp ASC")

	if !start.IsZero() {
		query = query.Where("created_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("created_at <= ?", end)
	}
	query = applyUsageStatusFilter(query, status, statusCode)

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, r := range rows {
		points = append(points, TimeSeriesPoint{
			Date:  r.Timestamp,
			Count: r.Count,
			Cost:  r.Cost,
		})
	}
	return points, nil
}
