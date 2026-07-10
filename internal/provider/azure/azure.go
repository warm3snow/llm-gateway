package azure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/warm3snow/llm-gateway/internal/provider"
	"github.com/warm3snow/llm-gateway/internal/types"
)

const (
	DefaultTimeout = 120 * time.Second
)

// AzureProvider Azure OpenAI 提供商
type AzureProvider struct {
	*provider.BaseProvider
	APIKey       string
	ResourceName string
	DeploymentID string
	APIVersion   string
	BaseURL      string
	HTTPClient   *http.Client
	Timeout      time.Duration
}

// NewAzureProvider 创建 Azure OpenAI 提供商
func NewAzureProvider(opts *types.Options) (provider.Provider, error) {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = opts.VirtualKey
	}

	resource := opts.ResourceName
	if resource == "" {
		resource = "my-azure-openai"
	}
	deployment := opts.DeploymentID
	if deployment == "" {
		deployment = "gpt-4"
	}
	apiVersion := opts.APIVersion
	if apiVersion == "" {
		apiVersion = "2024-02-15-preview"
	}

	baseURL := opts.CustomHost
	if baseURL == "" {
		baseURL = fmt.Sprintf("https://%s.openai.azure.com/openai", resource)
	}

	timeout := DefaultTimeout
	if opts.RequestTimeout > 0 {
		timeout = time.Duration(opts.RequestTimeout) * time.Millisecond
	}

	return &AzureProvider{
		BaseProvider: &provider.BaseProvider{
			Name:    string(types.ProviderAzureOpenAI),
			BaseURL: baseURL,
			Endpoints: map[string]string{
				"chatCompletions": "/chat/completions",
				"completions":     "/completions",
				"embeddings":      "/embeddings",
				"images":          "/images/generations",
			},
		},
		APIKey:       apiKey,
		ResourceName: resource,
		DeploymentID: deployment,
		APIVersion:   apiVersion,
		BaseURL:      baseURL,
		HTTPClient:   &http.Client{Timeout: timeout},
		Timeout:      timeout,
	}, nil
}

func (p *AzureProvider) GetName() string    { return p.Name }
func (p *AzureProvider) GetBaseURL() string { return p.BaseURL }
func (p *AzureProvider) GetEndpoints() []string {
	eps := make([]string, 0, len(p.Endpoints))
	for ep := range p.Endpoints {
		eps = append(eps, ep)
	}
	return eps
}

// buildURL 构建 Azure 格式的 URL
func (p *AzureProvider) buildURL(deployment, endpoint string) string {
	return fmt.Sprintf("%s/deployments/%s/%s?api-version=%s",
		p.BaseURL, deployment, endpoint, p.APIVersion)
}

// ChatCompletion 聊天补全
func (p *AzureProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	deployment := p.DeploymentID
	if opts != nil && opts.DeploymentID != "" {
		deployment = opts.DeploymentID
	}

	url := p.buildURL(deployment, "chat/completions")
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.APIKey)

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	return resp, nil
}

// Completion 文本补全
func (p *AzureProvider) Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error) {
	deployment := p.DeploymentID
	url := p.buildURL(deployment, "completions")
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.APIKey)

	return p.HTTPClient.Do(httpReq)
}

// Embedding 嵌入
func (p *AzureProvider) Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error) {
	deployment := p.DeploymentID
	url := p.buildURL(deployment, "embeddings")
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.APIKey)

	return p.HTTPClient.Do(httpReq)
}

// ImageGeneration 图像生成
func (p *AzureProvider) ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	deployment := p.DeploymentID
	url := p.buildURL(deployment, "images/generations")
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", p.APIKey)

	return p.HTTPClient.Do(httpReq)
}

// AudioSpeech 文本转语音 — Azure OpenAI 不支持
func (p *AzureProvider) AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("azure openai does not support audio speech")
}

// AudioTranscription 语音转文本 — Azure OpenAI 不支持
func (p *AzureProvider) AudioTranscription(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("azure openai does not support audio transcription")
}

func (p *AzureProvider) AudioTranslation(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("azure openai does not support audio translation")
}

// Models 获取模型列表
func (p *AzureProvider) Models(ctx context.Context, opts *types.Options) (*http.Response, error) {
	url := fmt.Sprintf("%s/deployments?api-version=%s", p.BaseURL, p.APIVersion)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("api-key", p.APIKey)
	return p.HTTPClient.Do(httpReq)
}

// TransformRequest 转换请求格式
func (p *AzureProvider) TransformRequest(endpoint string, req interface{}) (interface{}, error) {
	// Azure OpenAI 使用与 OpenAI 兼容的格式，无需转换
	return req, nil
}

// TransformResponse 转换响应格式
func (p *AzureProvider) TransformResponse(endpoint string, resp *http.Response) (*http.Response, error) {
	// Azure OpenAI 响应格式与 OpenAI 兼容，无需转换
	return resp, nil
}

func init() {
	provider.RegisterGlobalProvider(string(types.ProviderAzureOpenAI), NewAzureProvider)
}
