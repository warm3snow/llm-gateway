package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/provider"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProviderHealthService struct {
	db          *gorm.DB
	providerSvc *ProviderConfigService
	factory     *provider.ProviderFactory
}

func NewProviderHealthService(factory *provider.ProviderFactory) *ProviderHealthService {
	if factory == nil {
		factory = provider.GetGlobalFactory()
	}
	return &ProviderHealthService{db: database.GetDB(), providerSvc: NewProviderConfigService(), factory: factory}
}

func (s *ProviderHealthService) List() ([]models.ProviderHealth, error) {
	var rows []models.ProviderHealth
	if s == nil || s.db == nil {
		return rows, nil
	}
	if err := s.db.Order("provider_name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *ProviderHealthService) CheckAll(ctx context.Context) error {
	if s == nil || s.db == nil || s.providerSvc == nil {
		return nil
	}
	configs, err := s.providerSvc.List()
	if err != nil {
		return err
	}
	for i := range configs {
		if !configs[i].Enabled {
			continue
		}
		if err := s.CheckProvider(ctx, &configs[i]); err != nil && ctx.Err() != nil {
			return err
		}
	}
	return nil
}

func (s *ProviderHealthService) CheckProvider(ctx context.Context, cfg *models.ProviderConfig) error {
	started := time.Now()
	health := models.ProviderHealth{ProviderName: cfg.Name, ProviderType: cfg.ProviderType, LastCheckedAt: started}
	opts, err := s.providerSvc.ToOptions(cfg)
	if err == nil {
		prov, createErr := s.factory.Create(opts.Provider, &opts)
		if createErr != nil {
			err = createErr
		} else {
			resp, modelsErr := prov.Models(ctx, &opts)
			if modelsErr != nil {
				err = modelsErr
			} else {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
				if resp.StatusCode >= 400 {
					err = fmt.Errorf("provider returned status %d", resp.StatusCode)
				}
			}
		}
	}

	health.LatencyMS = time.Since(started).Milliseconds()
	if err == nil {
		health.Healthy = true
		health.Status = "healthy"
		health.LastSuccessAt = &started
	} else {
		health.Healthy = false
		health.Status = "down"
		health.ErrorMessage = truncateHealthError(err.Error())
	}

	return s.upsertHealth(ctx, &health)
}

func (s *ProviderHealthService) upsertHealth(ctx context.Context, health *models.ProviderHealth) error {
	assignments := map[string]interface{}{
		"provider_type":   health.ProviderType,
		"healthy":         health.Healthy,
		"status":          health.Status,
		"latency_ms":      health.LatencyMS,
		"error_message":   health.ErrorMessage,
		"last_checked_at": health.LastCheckedAt,
		"last_success_at": health.LastSuccessAt,
		"updated_at":      time.Now(),
		"consecutive_err": gorm.Expr("CASE WHEN ? THEN 0 ELSE consecutive_err + 1 END", health.Healthy),
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "provider_name"}},
		DoUpdates: clause.Assignments(assignments),
	}).Create(health).Error
}

func (s *ProviderHealthService) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		_ = s.CheckAll(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.CheckAll(ctx)
			}
		}
	}()
}

func truncateHealthError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 500 {
		return message
	}
	return message[:500]
}
