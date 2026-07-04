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
}

// computeSummary runs the aggregate queries over usage records that back both
// GetOverview and GetAnalytics.
func (s *StatsService) computeSummary() summary {
	var sum summary

	s.db.Model(&models.UsageRecord{}).Count(&sum.TotalRequests)

	var successCount int64
	s.db.Model(&models.UsageRecord{}).Where("status_code >= 200 AND status_code < 300").Count(&successCount)
	if sum.TotalRequests > 0 {
		sum.SuccessRate = float64(successCount) / float64(sum.TotalRequests) * 100
	}

	var tokens struct {
		TotalInput  int64 `gorm:"column:total_input_tokens"`
		TotalOutput int64 `gorm:"column:total_output_tokens"`
	}
	s.db.Model(&models.UsageRecord{}).Select("SUM(input_tokens) as total_input_tokens, SUM(output_tokens) as total_output_tokens").Scan(&tokens)
	sum.TotalTokens = tokens.TotalInput + tokens.TotalOutput

	s.db.Model(&models.UsageRecord{}).Select("COALESCE(SUM(cost), 0)").Scan(&sum.TotalCost)

	var providerCount int64
	s.db.Model(&models.UsageRecord{}).Distinct("provider").Count(&providerCount)
	sum.ActiveProviders = int(providerCount)

	s.db.Model(&models.VirtualKey{}).Where("status = ?", "active").Count(&sum.ActiveVirtualKeys)

	return sum
}

// GetOverview returns stats for the dashboard
func (s *StatsService) GetOverview() (*OverviewStats, error) {
	sum := s.computeSummary()
	return &OverviewStats{
		TotalRequests:     sum.TotalRequests,
		TotalTokens:       sum.TotalTokens,
		TotalCost:         sum.TotalCost,
		ActiveProviders:   sum.ActiveProviders,
		ActiveVirtualKeys: sum.ActiveVirtualKeys,
		SuccessRate:       sum.SuccessRate,
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
	Model string `json:"model"`
	Count int64  `json:"count"`
}

// TopProvider represents a provider ranking entry
type TopProvider struct {
	Provider string `json:"provider"`
	Count    int64  `json:"count"`
}

// AnalyticsData combines all analytics data for the frontend
type AnalyticsData struct {
	TotalRequests     int64             `json:"totalRequests"`
	TotalTokens       int64             `json:"totalTokens"`
	TotalCost         float64           `json:"totalCost"`
	ActiveProviders   int               `json:"activeProviders"`
	ActiveVirtualKeys int64             `json:"activeVirtualKeys"`
	SuccessRate       float64           `json:"successRate"`
	TimeSeries        []TimeSeriesPoint `json:"timeSeries"`
	TopModels         []TopModel        `json:"topModels"`
	TopProviders      []TopProvider     `json:"topProviders"`
	MaxCount          int64             `json:"maxCount"`
}

// GetAnalytics returns combined analytics data
func (s *StatsService) GetAnalytics() (*AnalyticsData, error) {
	sum := s.computeSummary()
	data := &AnalyticsData{
		TotalRequests:     sum.TotalRequests,
		TotalTokens:       sum.TotalTokens,
		TotalCost:         sum.TotalCost,
		ActiveProviders:   sum.ActiveProviders,
		ActiveVirtualKeys: sum.ActiveVirtualKeys,
		SuccessRate:       sum.SuccessRate,
	}

	// Time series (last 7 days)
	end := time.Now()
	start := end.AddDate(0, 0, -7)
	data.TimeSeries = []TimeSeriesPoint{}

	var timePoints []struct {
		Timestamp string  `json:"timestamp"`
		Count     int64   `json:"count"`
		Cost      float64 `json:"cost"`
	}
	s.db.Model(&models.UsageRecord{}).
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

	// Top models
	data.TopModels = []TopModel{}
	var topModels []struct {
		Model string `json:"model"`
		Count int64  `json:"count"`
	}
	s.db.Model(&models.UsageRecord{}).
		Select("model, COUNT(*) as count").
		Where("model != ''").
		Group("model").
		Order("count DESC").
		Limit(10).
		Scan(&topModels)
	for _, m := range topModels {
		data.TopModels = append(data.TopModels, TopModel{Model: m.Model, Count: m.Count})
	}

	// Top providers
	data.TopProviders = []TopProvider{}
	var topProviders []struct {
		Provider string `json:"provider"`
		Count    int64  `json:"count"`
	}
	s.db.Model(&models.UsageRecord{}).
		Select("provider, COUNT(*) as count").
		Group("provider").
		Order("count DESC").
		Limit(10).
		Scan(&topProviders)
	for _, p := range topProviders {
		data.TopProviders = append(data.TopProviders, TopProvider{Provider: p.Provider, Count: p.Count})
	}

	return data, nil
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
func (s *StatsService) GetHourlyTimeSeries(start, end time.Time) ([]TimeSeriesPoint, error) {
	return s.timeSeries(s.hourGroupExpr(), start, end)
}

// timeSeries returns invocation counts and cost grouped by groupExpr over the
// optional [start, end] window.
func (s *StatsService) timeSeries(groupExpr string, start, end time.Time) ([]TimeSeriesPoint, error) {
	points := []TimeSeriesPoint{}

	var rows []struct {
		Timestamp string  `gorm:"column:timestamp"`
		Count     int64   `gorm:"column:count"`
		Cost      float64 `gorm:"column:cost"`
	}

	query := s.db.Model(&models.UsageRecord{}).
		Select(groupExpr + " as timestamp, COUNT(*) as count, COALESCE(SUM(cost), 0) as cost").
		Group(groupExpr).
		Order("timestamp ASC")

	if !start.IsZero() {
		query = query.Where("created_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("created_at <= ?", end)
	}

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
