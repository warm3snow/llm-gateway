package models

import "time"

const (
	AlertRuleBudgetPercent = "budget_percent"
	AlertRuleCostWindow    = "cost_window"

	AlertStatusFired    = "fired"
	AlertStatusResolved = "resolved"
)

type AlertRule struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	TenantID     uint      `gorm:"index;not null;default:1" json:"tenant_id"`
	VirtualKeyID uint      `gorm:"index;not null" json:"virtual_key_id"`
	Name         string    `gorm:"size:100;not null" json:"name"`
	RuleType     string    `gorm:"size:50;not null;index" json:"rule_type"`
	Threshold    float64   `gorm:"not null" json:"threshold"`
	WindowHours  int       `gorm:"default:24" json:"window_hours"`
	Enabled      bool      `gorm:"default:true;index" json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (AlertRule) TableName() string { return "alert_rules" }

type AlertRuleRequest struct {
	VirtualKeyID uint    `json:"virtual_key_id" binding:"required"`
	Name         string  `json:"name"`
	RuleType     string  `json:"rule_type" binding:"required"`
	Threshold    float64 `json:"threshold" binding:"required"`
	WindowHours  int     `json:"window_hours"`
	Enabled      *bool   `json:"enabled"`
}
