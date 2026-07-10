package service

import (
	"testing"
	"time"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
)

func TestUsageServiceFiltersByStatusFamily(t *testing.T) {
	setupTenantTestDB(t)
	svc := NewUsageService()
	now := time.Now()
	records := []models.UsageRecord{
		{TenantID: 1, Provider: "openai", Model: "gpt", StatusCode: 200, Cost: 0.01, CreatedAt: now},
		{TenantID: 1, Provider: "openai", Model: "gpt", StatusCode: 399, Cost: 0.02, CreatedAt: now},
		{TenantID: 1, Provider: "openai", Model: "gpt", StatusCode: 400, Cost: 0.03, CreatedAt: now},
		{TenantID: 1, Provider: "openai", Model: "gpt", StatusCode: 500, Cost: 0.04, CreatedAt: now},
	}
	if err := database.GetDB().Create(&records).Error; err != nil {
		t.Fatalf("seed usage: %v", err)
	}

	success, total, err := svc.GetRecordsWithStatus(AccessScope{Role: models.RoleTenantAdmin, TenantID: 1}, "", "", "success", 0, time.Time{}, time.Time{}, 100, 0)
	if err != nil {
		t.Fatalf("success filter: %v", err)
	}
	if total != 2 || len(success) != 2 {
		t.Fatalf("success total=%d len=%d, want 2", total, len(success))
	}

	errors, total, err := svc.GetRecordsWithStatus(AccessScope{Role: models.RoleTenantAdmin, TenantID: 1}, "", "", "error", 0, time.Time{}, time.Time{}, 100, 0)
	if err != nil {
		t.Fatalf("error filter: %v", err)
	}
	if total != 2 || len(errors) != 2 {
		t.Fatalf("error total=%d len=%d, want 2", total, len(errors))
	}

	exact, total, err := svc.GetRecordsWithStatus(AccessScope{Role: models.RoleTenantAdmin, TenantID: 1}, "", "", "", 500, time.Time{}, time.Time{}, 100, 0)
	if err != nil {
		t.Fatalf("exact filter: %v", err)
	}
	if total != 1 || len(exact) != 1 || exact[0].StatusCode != 500 {
		t.Fatalf("exact 500 records=%+v total=%d, want one 500", exact, total)
	}
}

func TestStatsServiceHourlyTimeSeriesFiltersByStatusFamily(t *testing.T) {
	setupTenantTestDB(t)
	svc := NewStatsService()
	now := time.Now()
	records := []models.UsageRecord{
		{TenantID: 1, Provider: "openai", Model: "gpt", StatusCode: 200, Cost: 0.01, CreatedAt: now},
		{TenantID: 1, Provider: "openai", Model: "gpt", StatusCode: 500, Cost: 0.04, CreatedAt: now},
		{TenantID: 1, Provider: "openai", Model: "gpt", StatusCode: 429, Cost: 0.03, CreatedAt: now},
	}
	if err := database.GetDB().Create(&records).Error; err != nil {
		t.Fatalf("seed usage: %v", err)
	}

	points, err := svc.GetHourlyTimeSeriesWithStatus(AccessScope{Role: models.RoleTenantAdmin, TenantID: 1}, now.Add(-time.Hour), now.Add(time.Hour), "error", 0)
	if err != nil {
		t.Fatalf("hourly error filter: %v", err)
	}
	if len(points) != 1 || points[0].Count != 2 || points[0].Cost != 0.07 {
		t.Fatalf("points=%+v, want one bucket with 2 errors and 0.07 cost", points)
	}
}
