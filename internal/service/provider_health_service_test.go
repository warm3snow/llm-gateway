package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/provider"
	"github.com/warm3snow/llm-gateway/internal/types"
)

type healthTestProvider struct{}

func (healthTestProvider) GetName() string        { return "health-test" }
func (healthTestProvider) GetBaseURL() string     { return "" }
func (healthTestProvider) GetEndpoints() []string { return nil }
func (healthTestProvider) ChatCompletion(context.Context, *types.ChatCompletionRequest, *types.Options) (*http.Response, error) {
	return nil, nil
}
func (healthTestProvider) Completion(context.Context, *types.CompletionRequest, *types.Options) (*http.Response, error) {
	return nil, nil
}
func (healthTestProvider) Embedding(context.Context, *types.EmbeddingRequest, *types.Options) (*http.Response, error) {
	return nil, nil
}
func (healthTestProvider) ImageGeneration(context.Context, map[string]interface{}, *types.Options) (*http.Response, error) {
	return nil, nil
}
func (healthTestProvider) AudioSpeech(context.Context, map[string]interface{}, *types.Options) (*http.Response, error) {
	return nil, nil
}
func (healthTestProvider) AudioTranscription(context.Context, *types.AudioRequest, *types.Options) (*http.Response, error) {
	return nil, nil
}
func (healthTestProvider) AudioTranslation(context.Context, *types.AudioRequest, *types.Options) (*http.Response, error) {
	return nil, nil
}
func (healthTestProvider) Models(context.Context, *types.Options) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, nil
}
func (healthTestProvider) TransformRequest(string, interface{}) (interface{}, error) { return nil, nil }
func (healthTestProvider) TransformResponse(string, *http.Response) (*http.Response, error) {
	return nil, nil
}

func TestProviderHealthServiceCheckAllRecordsHealthyProvider(t *testing.T) {
	setupTenantTestDB(t)
	if err := database.Migrate(&models.ProviderConfig{}, &models.ProviderHealth{}); err != nil {
		t.Fatalf("migrate provider health: %v", err)
	}
	factory := provider.NewProviderFactory()
	factory.Register("health-test", func(opts *types.Options) (provider.Provider, error) { return healthTestProvider{}, nil })

	providerSvc := NewProviderConfigService()
	if _, err := providerSvc.Create("test-provider", types.Options{Provider: "health-test", APIKey: "sk-test"}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	svc := NewProviderHealthService(factory)
	if err := svc.CheckAll(context.Background()); err != nil {
		t.Fatalf("check all: %v", err)
	}
	rows, err := svc.List()
	if err != nil {
		t.Fatalf("list health: %v", err)
	}
	if len(rows) != 1 || rows[0].ProviderName != "test-provider" || !rows[0].Healthy || rows[0].Status != "healthy" {
		t.Fatalf("unexpected health rows: %+v", rows)
	}
}
