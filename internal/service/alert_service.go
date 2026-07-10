package service

import (
	"fmt"
	"time"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"gorm.io/gorm"
)

type AlertService struct {
	db *gorm.DB
}

func NewAlertService(db ...*gorm.DB) *AlertService {
	store := database.GetDB()
	if len(db) > 0 && db[0] != nil {
		store = db[0]
	}
	return &AlertService{db: store}
}

func (s *AlertService) CreateRule(tenantID uint, req *models.AlertRuleRequest) (*models.AlertRule, error) {
	if req.WindowHours <= 0 {
		req.WindowHours = 24
	}
	if req.Name == "" {
		req.Name = req.RuleType
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule := &models.AlertRule{TenantID: tenantID, VirtualKeyID: req.VirtualKeyID, Name: req.Name, RuleType: req.RuleType, Threshold: req.Threshold, WindowHours: req.WindowHours, Enabled: enabled}
	if err := s.db.Create(rule).Error; err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *AlertService) ListRules(tenantID, virtualKeyID uint) ([]models.AlertRule, error) {
	var rules []models.AlertRule
	query := s.db.Scopes(database.TenantScope(tenantID)).Order("created_at DESC")
	if virtualKeyID != 0 {
		query = query.Where("virtual_key_id = ?", virtualKeyID)
	}
	return rules, query.Find(&rules).Error
}

func (s *AlertService) ListEvents(tenantID, virtualKeyID uint, activeOnly bool) ([]models.AlertEvent, error) {
	var events []models.AlertEvent
	query := s.db.Scopes(database.TenantScope(tenantID)).Order("fired_at DESC")
	if virtualKeyID != 0 {
		query = query.Where("virtual_key_id = ?", virtualKeyID)
	}
	if activeOnly {
		query = query.Where("status = ?", models.AlertStatusFired)
	}
	return events, query.Find(&events).Error
}

func (s *AlertService) CheckVirtualKey(virtualKeyID uint) error {
	if s == nil || s.db == nil || virtualKeyID == 0 {
		return nil
	}
	var key models.VirtualKey
	if err := s.db.First(&key, virtualKeyID).Error; err != nil {
		return err
	}
	var rules []models.AlertRule
	if err := s.db.Where("virtual_key_id = ? AND enabled = ?", virtualKeyID, true).Find(&rules).Error; err != nil {
		return err
	}
	for i := range rules {
		value, triggered, err := s.evaluateRule(&key, &rules[i])
		if err != nil {
			return err
		}
		if triggered {
			if err := s.fireEvent(&rules[i], value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *AlertService) evaluateRule(key *models.VirtualKey, rule *models.AlertRule) (float64, bool, error) {
	switch rule.RuleType {
	case models.AlertRuleBudgetPercent:
		if key.BudgetTotal <= 0 {
			return 0, false, nil
		}
		value := key.BudgetUsed / key.BudgetTotal * 100
		return value, value >= rule.Threshold, nil
	case models.AlertRuleCostWindow:
		windowHours := rule.WindowHours
		if windowHours <= 0 {
			windowHours = 24
		}
		var value float64
		start := time.Now().Add(-time.Duration(windowHours) * time.Hour)
		if err := s.db.Model(&models.UsageRecord{}).Where("virtual_key_id = ? AND created_at >= ?", rule.VirtualKeyID, start).Select("COALESCE(SUM(cost), 0)").Scan(&value).Error; err != nil {
			return 0, false, err
		}
		return value, value >= rule.Threshold, nil
	default:
		return 0, false, fmt.Errorf("unknown alert rule type: %s", rule.RuleType)
	}
}

func (s *AlertService) fireEvent(rule *models.AlertRule, value float64) error {
	var existing models.AlertEvent
	err := s.db.Where("rule_id = ? AND status = ?", rule.ID, models.AlertStatusFired).First(&existing).Error
	if err == nil {
		return nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	event := &models.AlertEvent{TenantID: rule.TenantID, RuleID: rule.ID, VirtualKeyID: rule.VirtualKeyID, Status: models.AlertStatusFired, Message: alertMessage(rule, value), Value: value, Threshold: rule.Threshold, FiredAt: time.Now()}
	return s.db.Create(event).Error
}

func alertMessage(rule *models.AlertRule, value float64) string {
	switch rule.RuleType {
	case models.AlertRuleBudgetPercent:
		return fmt.Sprintf("Budget usage %.2f%% reached threshold %.2f%%", value, rule.Threshold)
	case models.AlertRuleCostWindow:
		return fmt.Sprintf("Cost %.6f reached threshold %.6f in the last %d hours", value, rule.Threshold, rule.WindowHours)
	default:
		return fmt.Sprintf("Alert threshold reached: %.6f >= %.6f", value, rule.Threshold)
	}
}
