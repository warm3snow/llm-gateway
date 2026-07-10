package service

import (
	"time"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"gorm.io/gorm"
)

// UsageService handles usage-record queries.
type UsageService struct {
	db *gorm.DB
}

// NewUsageService creates a new UsageService.
func NewUsageService() *UsageService {
	return &UsageService{
		db: database.GetDB(),
	}
}

// GetRecords returns paginated usage records filtered by access scope,
// provider/model/exact status code, and a created_at date window.
func (s *UsageService) GetRecords(scope AccessScope, provider, model string, statusCode int, startDate, endDate time.Time, limit, offset int) ([]models.UsageRecord, int64, error) {
	return s.GetRecordsWithStatus(scope, provider, model, "", statusCode, startDate, endDate, limit, offset)
}

// GetRecordsWithStatus returns paginated usage records with either an exact
// status_code filter or a status family filter: success (<400) / error (>=400).
func (s *UsageService) GetRecordsWithStatus(scope AccessScope, provider, model, status string, statusCode int, startDate, endDate time.Time, limit, offset int) ([]models.UsageRecord, int64, error) {
	var records []models.UsageRecord
	var total int64

	query := scope.ApplyUsage(s.db.Model(&models.UsageRecord{}))

	if provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if model != "" {
		query = query.Where("model = ?", model)
	}
	query = applyUsageStatusFilter(query, status, statusCode)
	if !startDate.IsZero() {
		query = query.Where("created_at >= ?", startDate)
	}
	if !endDate.IsZero() {
		query = query.Where("created_at <= ?", endDate)
	}

	query.Count(&total)

	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	err := query.Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error
	if err != nil {
		return nil, 0, err
	}

	return records, total, nil
}

// GetRecordByID returns a single usage record by ID, scoped to the caller.
func (s *UsageService) GetRecordByID(scope AccessScope, id uint) (*models.UsageRecord, error) {
	var record models.UsageRecord
	if err := scope.ApplyUsage(s.db.Model(&models.UsageRecord{})).First(&record, id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}
