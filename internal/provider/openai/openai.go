package openai

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
	DefaultBaseURL = "https://api.openai.com/v1"
	DefaultTimeout = 120 * time.Second
)

// OpenAIProvider OpenAI 提供商
type OpenAIProvider struct {
	*provider.BaseProvider
	APIKey     string
	OrgID      string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewOpenAIProvider 创建 OpenAI 提供商
func NewOpenAIProvider(opts *types.Options) (provider.Provider, error) {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = opts.VirtualKey
	}

	baseURL := DefaultBaseURL
	if opts.CustomHost != "" {
		baseURL = opts.CustomHost
	}

	timeout := DefaultTimeout
	if opts.RequestTimeout > 0 {
		timeout = time.Duration(opts.RequestTimeout) * time.Millisecond
	}

	return &OpenAIProvider{
		BaseProvider: &provider.BaseProvider{
			Name:    string(types.ProviderOpenAI),
			BaseURL: baseURL,
			Endpoints: map[string]string{
				"chatCompletions": "/chat/completions",
				"completions":     "/completions",
				"embeddings":      "/embeddings",
				"images":          "/images/generations",
				"audio":           "/audio/speech",
				"transcriptions":  "/audio/transcriptions",
				"translations":    "/audio/translations",
				"models":          "/models",
				"files":           "/files",
				"batches":         "/batches",
			},
		},
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: timeout},
		Timeout:    timeout,
	}, nil
}

// GetName 获取名称
func (p *OpenAIProvider) GetName() string {
	return p.Name
}

// GetBaseURL 获取基础 URL
func (p *OpenAIProvider) GetBaseURL() string {
	return p.BaseURL
}

// sendRequest is a helper that marshals req, sets headers, and sends the request.
// Pass nil for req on GET requests (body is omitted).
func (p *OpenAIProvider) sendRequest(ctx context.Context, method, endpoint string, req interface{}) (*http.Response, error) {
	var bodyBytes []byte
	if req != nil {
		var err error
		bodyBytes, err = json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, p.BaseURL+endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if req != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	return resp, nil
}

// ChatCompletion 聊天补全
func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/chat/completions", req)
}

// Completion 文本补全
func (p *OpenAIProvider) Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/completions", req)
}

// Embedding 嵌入
func (p *OpenAIProvider) Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/embeddings", req)
}

// ImageGeneration 图像生成
func (p *OpenAIProvider) ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/images/generations", req)
}

// AudioSpeech 文本转语音
func (p *OpenAIProvider) AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/audio/speech", req)
}

// AudioTranscription 语音转文本
func (p *OpenAIProvider) AudioTranscription(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	// TODO: construct multipart form-data request for audio file upload
	return nil, fmt.Errorf("AudioTranscription not yet implemented")
}

// Models 获取模型列表
func (p *OpenAIProvider) Models(ctx context.Context, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "GET", "/models", nil)
}

// TransformRequest 转换请求
func (p *OpenAIProvider) TransformRequest(endpoint string, req interface{}) (interface{}, error) {
	// OpenAI 格式已经是标准格式，通常不需要转换
	return req, nil
}

// TransformResponse 转换响应
func (p *OpenAIProvider) TransformResponse(endpoint string, resp *http.Response) (*http.Response, error) {
	// OpenAI 响应通常不需要转换
	return resp, nil
}

// 注册 OpenAI provider
func init() {
	provider.RegisterGlobalProvider(string(types.ProviderOpenAI), NewOpenAIProvider)
}
