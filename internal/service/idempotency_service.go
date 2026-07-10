package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"gorm.io/gorm"
)

var (
	ErrIdempotencyConflict   = errors.New("idempotency key reused with different request")
	ErrIdempotencyInProgress = errors.New("idempotency request is still processing")
)

type IdempotencyService struct {
	db *gorm.DB
}

type IdempotencyBeginResult struct {
	Record *models.IdempotencyRecord
	Replay bool
}

func NewIdempotencyService(db ...*gorm.DB) *IdempotencyService {
	store := database.GetDB()
	if len(db) > 0 && db[0] != nil {
		store = db[0]
	}
	return &IdempotencyService{db: store}
}

func HashIdempotencyRequest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func (s *IdempotencyService) Begin(ctx context.Context, virtualKeyID uint, key, requestHash string, ttl time.Duration) (*IdempotencyBeginResult, error) {
	if s == nil || s.db == nil || virtualKeyID == 0 || key == "" || requestHash == "" {
		return nil, nil
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}

	now := time.Now()
	result, err := s.classifyExisting(ctx, virtualKeyID, key, requestHash, now)
	if err != nil || result != nil {
		return result, err
	}

	record := &models.IdempotencyRecord{
		VirtualKeyID:   virtualKeyID,
		IdempotencyKey: key,
		RequestHash:    requestHash,
		Status:         models.IdempotencyStatusProcessing,
		ExpiresAt:      now.Add(ttl),
	}
	if err := s.db.WithContext(ctx).Create(record).Error; err != nil {
		result, classifyErr := s.classifyExisting(ctx, virtualKeyID, key, requestHash, now)
		if classifyErr != nil || result != nil {
			return result, classifyErr
		}
		return nil, err
	}
	return &IdempotencyBeginResult{Record: record}, nil
}

func (s *IdempotencyService) Complete(ctx context.Context, id uint, statusCode int, contentType string, body []byte) error {
	if s == nil || s.db == nil || id == 0 {
		return nil
	}
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.IdempotencyRecord{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":                models.IdempotencyStatusCompleted,
		"response_status":       statusCode,
		"response_content_type": contentType,
		"response_body":         body,
		"completed_at":          &now,
	}).Error
}

func (s *IdempotencyService) classifyExisting(ctx context.Context, virtualKeyID uint, key, requestHash string, now time.Time) (*IdempotencyBeginResult, error) {
	var existing models.IdempotencyRecord
	err := s.db.WithContext(ctx).Where("virtual_key_id = ? AND idempotency_key = ?", virtualKeyID, key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !existing.ExpiresAt.IsZero() && existing.ExpiresAt.Before(now) {
		if err := s.db.WithContext(ctx).Delete(&existing).Error; err != nil {
			return nil, err
		}
		return nil, nil
	}
	if existing.RequestHash != requestHash {
		return nil, ErrIdempotencyConflict
	}
	if existing.Status == models.IdempotencyStatusCompleted {
		return &IdempotencyBeginResult{Record: &existing, Replay: true}, nil
	}
	return nil, ErrIdempotencyInProgress
}
