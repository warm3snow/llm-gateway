package service

import (
	"testing"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
)

func TestAlertServiceFiresBudgetPercentRuleOnce(t *testing.T) {
	setupTenantTestDB(t)
	if err := database.Migrate(&models.AlertRule{}, &models.AlertEvent{}); err != nil {
		t.Fatalf("migrate alerts: %v", err)
	}
	db := database.GetDB()
	key := models.VirtualKey{TenantID: 1, Name: "budgeted", KeyHash: "hash", KeySalt: "salt", HashedKey: "vsk", BudgetTotal: 10, BudgetUsed: 0, Status: "active"}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create key: %v", err)
	}

	alerts := NewAlertService(db)
	_, err := alerts.CreateRule(1, &models.AlertRuleRequest{VirtualKeyID: key.ID, RuleType: models.AlertRuleBudgetPercent, Threshold: 50})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	vk := NewVirtualKeyService()
	if err := vk.TrackUsage(key.ID, 6); err != nil {
		t.Fatalf("track usage: %v", err)
	}
	if err := vk.TrackUsage(key.ID, 1); err != nil {
		t.Fatalf("track usage again: %v", err)
	}

	events, err := alerts.ListEvents(1, key.ID, true)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 || events[0].Status != models.AlertStatusFired || events[0].Value < 50 {
		t.Fatalf("unexpected alert events: %+v", events)
	}
}
