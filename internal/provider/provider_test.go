package provider

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// MockProvider 模拟提供商
type MockProvider struct {
	*BaseProvider
	ChatCompletionFunc func(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error)
}

func (m *MockProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	return m.ChatCompletionFunc(ctx, req, opts)
}

func (m *MockProvider) Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error) {
	return nil, nil
}

func (m *MockProvider) Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error) {
	return nil, nil
}

func (m *MockProvider) ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, nil
}

func (m *MockProvider) AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, nil
}

func (m *MockProvider) AudioTranscription(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error) {
	return nil, nil
}

func (m *MockProvider) AudioTranslation(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error) {
	return nil, nil
}

func (m *MockProvider) Models(ctx context.Context, opts *types.Options) (*http.Response, error) {
	return nil, nil
}

func (m *MockProvider) TransformRequest(endpoint string, req interface{}) (interface{}, error) {
	return req, nil
}

func (m *MockProvider) TransformResponse(endpoint string, resp *http.Response) (*http.Response, error) {
	return resp, nil
}

func TestProviderFactory_Register(t *testing.T) {
	factory := NewProviderFactory()

	// 注册 mock provider
	factory.Register("mock", func(opts *types.Options) (Provider, error) {
		return &MockProvider{
			BaseProvider: &BaseProvider{
				Name:    "mock",
				BaseURL: "https://api.mock.com",
			},
		}, nil
	})

	// 创建 provider
	opts := &types.Options{Provider: "mock"}
	prov, err := factory.Create("mock", opts)

	assert.NoError(t, err)
	assert.NotNil(t, prov)
	assert.Equal(t, "mock", prov.GetName())
	assert.Equal(t, "https://api.mock.com", prov.GetBaseURL())
}

func TestProviderFactory_CreateUnknownProvider(t *testing.T) {
	factory := NewProviderFactory()

	// 尝试创建未注册的 provider
	opts := &types.Options{Provider: "unknown"}
	prov, err := factory.Create("unknown", opts)

	assert.Error(t, err)
	assert.Nil(t, prov)
	assert.Contains(t, err.Error(), "unsupported provider")
}

func TestBaseProvider_GetEndpoints(t *testing.T) {
	base := &BaseProvider{
		Name:    "test",
		BaseURL: "https://api.test.com",
		Endpoints: map[string]string{
			"chatCompletions": "/chat/completions",
			"completions":     "/completions",
			"embeddings":      "/embeddings",
		},
	}

	endpoints := base.GetEndpoints()

	assert.Len(t, endpoints, 3)
	assert.Contains(t, endpoints, "chatCompletions")
	assert.Contains(t, endpoints, "completions")
	assert.Contains(t, endpoints, "embeddings")
}

func TestCreateProvider(t *testing.T) {
	// 注册全局 provider
	RegisterGlobalProvider("test", func(opts *types.Options) (Provider, error) {
		return &MockProvider{
			BaseProvider: &BaseProvider{
				Name:    "test",
				BaseURL: "https://api.test.com",
			},
		}, nil
	})

	// 创建 provider
	opts := &types.Options{Provider: "test"}
	prov, err := CreateProvider("test", opts)

	assert.NoError(t, err)
	assert.NotNil(t, prov)
	assert.Equal(t, "test", prov.GetName())
}

// 辅助函数
func init() {
	// 初始化测试
}
