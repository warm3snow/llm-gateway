package models

import "time"

type AlertEvent struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TenantID     uint       `gorm:"index;not null;default:1" json:"tenant_id"`
	RuleID       uint       `gorm:"index;not null" json:"rule_id"`
	VirtualKeyID uint       `gorm:"index;not null" json:"virtual_key_id"`
	Status       string     `gorm:"size:20;not null;index" json:"status"`
	Message      string     `gorm:"type:text" json:"message"`
	Value        float64    `json:"value"`
	Threshold    float64    `json:"threshold"`
	FiredAt      time.Time  `gorm:"index" json:"fired_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (AlertEvent) TableName() string { return "alert_events" }
