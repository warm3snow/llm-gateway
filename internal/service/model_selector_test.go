package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/types"
)

func setupModelSelectorTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "model_selector.db")
	if err := database.Connect(&database.Config{Driver: "sqlite", DSN: dsn, LogLevel: "silent"}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := database.Migrate(&models.ModelPricing{}, &models.UsageRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
}

func insertPricing(t *testing.T, provider, model string, inputPrice, outputPrice float64) {
	t.Helper()
	if err := models.UpsertModelPricing(database.GetDB(), &models.ModelPricing{
		Provider:    provider,
		Model:       model,
		InputPrice:  inputPrice,
		OutputPrice: outputPrice,
		Currency:    "USD",
		Source:      "test",
	}); err != nil {
		t.Fatalf("insert pricing %s/%s: %v", provider, model, err)
	}
}

func testSelectorConfig() *config.Config {
	return &config.Config{
		Gateway: config.GatewayConfig{
			DefaultProvider: "openai-a",
			Providers: map[string]types.Options{
				"openai-a": {Provider: "openai", APIKey: "test", Weight: 1, Metadata: map[string]string{"auto_max_concurrency": "2"}},
				"openai-b": {Provider: "openai", APIKey: "test", Weight: 1, Metadata: map[string]string{"auto_models": "expensive-model"}},
				"gemini-a": {Provider: "gemini", APIKey: "test", Weight: 1},
			},
		},
	}
}

func TestModelSelectorChoosesLowestCostCandidate(t *testing.T) {
	setupModelSelectorTestDB(t)
	insertPricing(t, "openai", "cheap-model", 0.01, 0.02)
	insertPricing(t, "openai", "expensive-model", 0.10, 0.20)

	selector := NewModelSelector(testSelectorConfig(), NewModelConcurrencyTracker())
	selection, err := selector.Select(context.Background(), &types.ChatCompletionRequest{
		Model:     "auto",
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
		MaxTokens: 100,
	}, SelectionHint{})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if selection.Model != "cheap-model" || selection.ProviderName != "openai-a" {
		t.Fatalf("expected openai-a/cheap-model, got %s/%s", selection.ProviderName, selection.Model)
	}
}

func TestModelSelectorRespectsProviderHint(t *testing.T) {
	setupModelSelectorTestDB(t)
	insertPricing(t, "openai", "cheap-openai", 0.01, 0.02)
	insertPricing(t, "gemini", "gemini-model", 0.20, 0.30)

	selector := NewModelSelector(testSelectorConfig(), NewModelConcurrencyTracker())
	selection, err := selector.Select(context.Background(), &types.ChatCompletionRequest{
		Model:    "auto",
		Messages: []types.Message{{Role: "user", Content: "hello"}},
	}, SelectionHint{ProviderName: "gemini-a"})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if selection.ProviderName != "gemini-a" || selection.Model != "gemini-model" {
		t.Fatalf("expected gemini-a/gemini-model, got %s/%s", selection.ProviderName, selection.Model)
	}
}

func TestModelSelectorRespectsAllowedProviders(t *testing.T) {
	setupModelSelectorTestDB(t)
	insertPricing(t, "openai", "cheap-openai", 0.01, 0.02)
	insertPricing(t, "gemini", "gemini-model", 0.20, 0.30)

	selector := NewModelSelector(testSelectorConfig(), NewModelConcurrencyTracker())
	selection, err := selector.Select(context.Background(), &types.ChatCompletionRequest{
		Model:    "auto",
		Messages: []types.Message{{Role: "user", Content: "hello"}},
	}, SelectionHint{AllowedProviders: []string{"gemini-a"}})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if selection.ProviderName != "gemini-a" {
		t.Fatalf("expected gemini-a, got %s", selection.ProviderName)
	}
}

func TestModelSelectorAvoidsSaturatedCandidate(t *testing.T) {
	setupModelSelectorTestDB(t)
	insertPricing(t, "openai", "cheap-model", 0.01, 0.02)
	insertPricing(t, "openai", "expensive-model", 0.011, 0.021)

	tracker := NewModelConcurrencyTracker()
	done1 := tracker.Begin("openai-a", "cheap-model")
	defer done1()
	done2 := tracker.Begin("openai-a", "cheap-model")
	defer done2()

	selector := NewModelSelector(testSelectorConfig(), tracker)
	selection, err := selector.Select(context.Background(), &types.ChatCompletionRequest{
		Model:     "auto",
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
		MaxTokens: 100,
	}, SelectionHint{})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if selection.Model != "expensive-model" {
		t.Fatalf("expected saturated cheap-model to be avoided, got %s", selection.Model)
	}
}

func TestModelSelectorPenalizesRecentProviderTypeErrors(t *testing.T) {
	setupModelSelectorTestDB(t)
	insertPricing(t, "openai", "cheap-model", 0.01, 0.02)
	insertPricing(t, "openai", "expensive-model", 0.011, 0.021)
	if err := database.GetDB().Create(&models.UsageRecord{
		Provider:   "openai",
		Model:      "cheap-model",
		Endpoint:   "/v1/chat/completions",
		StatusCode: 500,
	}).Error; err != nil {
		t.Fatalf("insert usage record: %v", err)
	}

	cfg := testSelectorConfig()
	cfg.Gateway.AutoMode.CostWeight = 0.10
	cfg.Gateway.AutoMode.ErrorWeight = 0.80
	selector := NewModelSelector(cfg, NewModelConcurrencyTracker())
	selection, err := selector.Select(context.Background(), &types.ChatCompletionRequest{
		Model:     "auto",
		Messages:  []types.Message{{Role: "user", Content: "hello"}},
		MaxTokens: 100,
	}, SelectionHint{})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if selection.Model != "expensive-model" {
		t.Fatalf("expected recent errors to penalize cheap-model, got %s", selection.Model)
	}
}

func TestModelSelectorReturnsErrorWhenNoCandidates(t *testing.T) {
	setupModelSelectorTestDB(t)

	selector := NewModelSelector(testSelectorConfig(), NewModelConcurrencyTracker())
	_, err := selector.Select(context.Background(), &types.ChatCompletionRequest{
		Model:    "auto",
		Messages: []types.Message{{Role: "user", Content: "hello"}},
	}, SelectionHint{})
	if err == nil {
		t.Fatal("expected no-candidates error")
	}
}
