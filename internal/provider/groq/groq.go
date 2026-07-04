package groq

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
	DefaultBaseURL = "https://api.groq.com/openai/v1"
	DefaultTimeout  = 60 * time.Second
)

type GroqProvider struct {
	*provider.BaseProvider
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

func NewGroqProvider(opts *types.Options) (provider.Provider, error) {
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
	return &GroqProvider{
		BaseProvider: &provider.BaseProvider{
			Name:    string(types.ProviderGroq),
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

func (p *GroqProvider) GetName() string    { return p.Name }
func (p *GroqProvider) GetBaseURL() string { return p.BaseURL }
func (p *GroqProvider) GetEndpoints() []string {
	eps := make([]string, 0, len(p.Endpoints))
	for ep := range p.Endpoints {
		eps = append(eps, ep)
	}
	return eps
}

func (p *GroqProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/chat/completions", req)
}

func (p *GroqProvider) Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/completions", req)
}

func (p *GroqProvider) Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/embeddings", req)
}

func (p *GroqProvider) ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("groq does not support image generation")
}

func (p *GroqProvider) AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("groq does not support audio speech")
}

func (p *GroqProvider) AudioTranscription(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("groq does not support audio transcription")
}

func (p *GroqProvider) Models(ctx context.Context, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "GET", "/models", nil)
}

func (p *GroqProvider) TransformRequest(endpoint string, req interface{}) (interface{}, error) {
	return req, nil
}

func (p *GroqProvider) TransformResponse(endpoint string, resp *http.Response) (*http.Response, error) {
	return resp, nil
}

func (p *GroqProvider) sendRequest(ctx context.Context, method, endpoint string, req interface{}) (*http.Response, error) {
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
	provider.RegisterGlobalProvider(string(types.ProviderGroq), NewGroqProvider)
}
