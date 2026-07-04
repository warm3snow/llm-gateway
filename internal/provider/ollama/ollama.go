package ollama

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
	DefaultBaseURL = "http://localhost:11434/v1"
	DefaultTimeout = 120 * time.Second
)

// OllamaProvider implements the Provider interface for Ollama (OpenAI-compatible).
type OllamaProvider struct {
	*provider.BaseProvider
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(opts *types.Options) (provider.Provider, error) {
	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = "ollama" // Ollama doesn't require a real key, but some clients need a non-empty value
	}
	baseURL := DefaultBaseURL
	if opts.CustomHost != "" {
		baseURL = opts.CustomHost
	}
	timeout := DefaultTimeout
	if opts.RequestTimeout > 0 {
		timeout = time.Duration(opts.RequestTimeout) * time.Millisecond
	}
	return &OllamaProvider{
		BaseProvider: &provider.BaseProvider{
			Name:    string(types.ProviderOllama),
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

func (p *OllamaProvider) GetName() string    { return p.Name }
func (p *OllamaProvider) GetBaseURL() string { return p.BaseURL }
func (p *OllamaProvider) GetEndpoints() []string {
	eps := make([]string, 0, len(p.Endpoints))
	for ep := range p.Endpoints {
		eps = append(eps, ep)
	}
	return eps
}

func (p *OllamaProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/chat/completions", req)
}

func (p *OllamaProvider) Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/completions", req)
}

func (p *OllamaProvider) Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "POST", "/embeddings", req)
}

func (p *OllamaProvider) ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("ollama does not support image generation")
}

func (p *OllamaProvider) AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("ollama does not support audio speech")
}

func (p *OllamaProvider) AudioTranscription(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("ollama does not support audio transcription")
}

func (p *OllamaProvider) Models(ctx context.Context, opts *types.Options) (*http.Response, error) {
	return p.sendRequest(ctx, "GET", "/models", nil)
}

func (p *OllamaProvider) TransformRequest(endpoint string, req interface{}) (interface{}, error) {
	return req, nil
}

func (p *OllamaProvider) TransformResponse(endpoint string, resp *http.Response) (*http.Response, error) {
	return resp, nil
}

func (p *OllamaProvider) sendRequest(ctx context.Context, method, endpoint string, req interface{}) (*http.Response, error) {
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
	// Ollama doesn't require auth, but the OpenAI-compatible endpoint accepts any Bearer token
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	return resp, nil
}

func init() {
	provider.RegisterGlobalProvider(string(types.ProviderOllama), NewOllamaProvider)
}
