package glm

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
	DefaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"
	DefaultTimeout = 120 * time.Second
)

type GLMProvider struct {
	*provider.BaseProvider
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

func NewGLMProvider(opts *types.Options) (provider.Provider, error) {
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
	return &GLMProvider{
		BaseProvider: &provider.BaseProvider{
			Name:    string(types.ProviderGLM),
			BaseURL: baseURL,
			Endpoints: map[string]string{
				"chatCompletions": "/chat/completions",
				"completions":     "/completions",
				"embeddings":      "/embeddings",
				"models":          "/models",
			},
		},
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: timeout},
		Timeout:    timeout,
	}, nil
}

func (p *GLMProvider) GetName() string    { return p.Name }
func (p *GLMProvider) GetBaseURL() string { return p.BaseURL }
func (p *GLMProvider) GetEndpoints() []string {
	eps := make([]string, 0, len(p.Endpoints))
	for ep := range p.Endpoints {
		eps = append(eps, ep)
	}
	return eps
}

func (p *GLMProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/chat/completions", req)
}

func (p *GLMProvider) Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/completions", req)
}

func (p *GLMProvider) Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/embeddings", req)
}

func (p *GLMProvider) ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("glm does not support image generation")
}

func (p *GLMProvider) AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("glm does not support audio speech")
}

func (p *GLMProvider) AudioTranscription(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("glm does not support audio transcription")
}

func (p *GLMProvider) AudioTranslation(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("glm does not support audio translation")
}

func (p *GLMProvider) Models(ctx context.Context, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "GET", "/models", nil)
}

func (p *GLMProvider) TransformRequest(endpoint string, req interface{}) (interface{}, error) {
	return req, nil
}

func (p *GLMProvider) TransformResponse(endpoint string, resp *http.Response) (*http.Response, error) {
	return resp, nil
}

func (p *GLMProvider) sendRequest(ctx context.Context, method, endpoint string, req interface{}) (*http.Response, error) {
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

func init() {
	provider.RegisterGlobalProvider(string(types.ProviderGLM), NewGLMProvider)
}
