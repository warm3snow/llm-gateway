package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/warm3snow/llm-gateway/internal/types"
)

// Provider LLM 提供商接口
type Provider interface {
	// GetName 获取提供商名称
	GetName() string

	// GetBaseURL 获取基础 URL
	GetBaseURL() string

	// GetEndpoints 获取支持的端点
	GetEndpoints() []string

	// ChatCompletion 聊天补全
	ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error)

	// Completion 文本补全
	Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error)

	// Embedding 嵌入
	Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error)

	// ImageGeneration 图像生成
	ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error)

	// AudioSpeech 文本转语音
	AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error)

	// AudioTranscription 语音转文本
	AudioTranscription(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error)

	// AudioTranslation 语音翻译
	AudioTranslation(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error)

	// Models 获取模型列表
	Models(ctx context.Context, opts *types.Options) (*http.Response, error)

	// TransformRequest 转换请求格式
	TransformRequest(endpoint string, req interface{}) (interface{}, error)

	// TransformResponse 转换响应格式
	TransformResponse(endpoint string, resp *http.Response) (*http.Response, error)
}

// BaseProvider 基础提供商实现
type BaseProvider struct {
	Name      string
	BaseURL   string
	APIKey    string
	Endpoints map[string]string
}

// GetName 获取名称
func (p *BaseProvider) GetName() string {
	return p.Name
}

// GetBaseURL 获取基础 URL
func (p *BaseProvider) GetBaseURL() string {
	return p.BaseURL
}

// GetEndpoints 获取端点
func (p *BaseProvider) GetEndpoints() []string {
	endpoints := make([]string, 0, len(p.Endpoints))
	for ep := range p.Endpoints {
		endpoints = append(endpoints, ep)
	}
	return endpoints
}

// RegisterProvider 注册提供商
func RegisterProvider(name string, factory func(opts *types.Options) (Provider, error)) {
	// 注册逻辑
}

// ProviderFactory 提供商工厂
type ProviderFactory struct {
	providers map[string]func(opts *types.Options) (Provider, error)
}

// NewProviderFactory 创建提供商工厂
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		providers: make(map[string]func(opts *types.Options) (Provider, error)),
	}
}

// Register 注册提供商
func (f *ProviderFactory) Register(name string, factory func(opts *types.Options) (Provider, error)) {
	f.providers[name] = factory
}

// Create 创建提供商实例
func (f *ProviderFactory) Create(name string, opts *types.Options) (Provider, error) {
	factory, ok := f.providers[name]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", name)
	}

	return factory(opts)
}

// 全局提供商工厂
var globalFactory = NewProviderFactory()

// RegisterGlobalProvider 注册全局提供商
func RegisterGlobalProvider(name string, factory func(opts *types.Options) (Provider, error)) {
	globalFactory.Register(name, factory)
}

// CreateProvider 创建提供商
func CreateProvider(name string, opts *types.Options) (Provider, error) {
	return globalFactory.Create(name, opts)
}

// ListProviders 列出所有已注册的提供商名称
func (f *ProviderFactory) ListProviders() []string {
	names := make([]string, 0, len(f.providers))
	for name := range f.providers {
		names = append(names, name)
	}
	return names
}

// GetGlobalFactory 获取全局提供商工厂
func GetGlobalFactory() *ProviderFactory {
	return globalFactory
}
