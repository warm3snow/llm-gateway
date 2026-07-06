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

// GetRecords returns paginated usage records filtered by provider/model/status
// and a created_at date window.
func (s *UsageService) GetRecords(tenantID uint, provider, model string, statusCode int, startDate, endDate time.Time, limit, offset int) ([]models.UsageRecord, int64, error) {
	var records []models.UsageRecord
	var total int64

	query := s.db.Model(&models.UsageRecord{}).Scopes(database.TenantScope(tenantID))

	if provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if model != "" {
		query = query.Where("model = ?", model)
	}
	if statusCode > 0 {
		query = query.Where("status_code = ?", statusCode)
	}
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

// GetRecordByID returns a single usage record by ID, scoped to tenantID
// (0 = any tenant).
func (s *UsageService) GetRecordByID(tenantID, id uint) (*models.UsageRecord, error) {
	var record models.UsageRecord
	if err := s.db.Scopes(database.TenantScope(tenantID)).First(&record, id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}
