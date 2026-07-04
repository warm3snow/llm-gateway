package cohere

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/warm3snow/llm-gateway/internal/provider"
	"github.com/warm3snow/llm-gateway/internal/types"
)

const (
	DefaultBaseURL = "https://api.cohere.ai/v1"
	DefaultTimeout = 120 * time.Second
)

type CohereProvider struct {
	*provider.BaseProvider
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

func NewCohereProvider(opts *types.Options) (provider.Provider, error) {
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
	return &CohereProvider{
		BaseProvider: &provider.BaseProvider{
			Name:    string(types.ProviderCohere),
			BaseURL: baseURL,
			Endpoints: map[string]string{
				"chat":      "/chat",
				"embeddings": "/embed",
				"models":     "/models",
			},
		},
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: timeout},
		Timeout:    timeout,
	}, nil
}

func (p *CohereProvider) GetName() string  { return p.Name }
func (p *CohereProvider) GetBaseURL() string { return p.BaseURL }
func (p *CohereProvider) GetEndpoints() []string {
	eps := make([]string, 0, len(p.Endpoints))
	for ep := range p.Endpoints {
		eps = append(eps, ep)
	}
	return eps
}

func (p *CohereProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	cohereReq := map[string]interface{}{
		"model":      req.Model,
		"messages":  convertMessagesToCohere(req.Messages),
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      req.Stream,
	}
	url := p.BaseURL + "/chat"
	bodyBytes, _ := json.Marshal(cohereReq)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}
	return convertCohereChatResponse(resp)
}

func (p *CohereProvider) Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("cohere does not support completion endpoint")
}

func (p *CohereProvider) Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error) {
	cohereReq := map[string]interface{}{
		"model":     req.Model,
		"texts":     req.Input,
		"input_type": "search_document",
	}
	url := p.BaseURL + "/embed"
	bodyBytes, _ := json.Marshal(cohereReq)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	return p.HTTPClient.Do(httpReq)
}

func (p *CohereProvider) ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("cohere does not support image generation via this endpoint")
}

func (p *CohereProvider) AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("cohere does not support audio speech")
}

func (p *CohereProvider) AudioTranscription(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("cohere does not support audio transcription")
}

func (p *CohereProvider) Models(ctx context.Context, opts *types.Options) (*http.Response, error) {
	url := p.BaseURL + "/models"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	return p.HTTPClient.Do(httpReq)
}

func (p *CohereProvider) TransformRequest(endpoint string, req interface{}) (interface{}, error) {
	return req, nil
}

func (p *CohereProvider) TransformResponse(endpoint string, resp *http.Response) (*http.Response, error) {
	if endpoint == "chat" {
		return convertCohereChatResponse(resp)
	}
	return resp, nil
}

func convertMessagesToCohere(messages []types.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		role := msg.Role
		if role == "assistant" {
			role = "CHATBOT"
		} else if role == "system" {
			role = "SYSTEM"
		} else {
			role = "USER"
		}
		var text string
		switch c := msg.Content.(type) {
		case string:
			text = c
		}
		result = append(result, map[string]interface{}{
			"role":  role,
			"content": text,
		})
	}
	return result
}

func convertCohereChatResponse(resp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var cohereResp map[string]interface{}
	if err := json.Unmarshal(body, &cohereResp); err != nil {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}
	text := ""
	if v, ok := cohereResp["text"].(string); ok {
		text = v
	}
	openaiResp := map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%s", randomString(12)),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   getString(cohereResp, "model"),
		"choices": []map[string]interface{}{
			{"index": 0, "message": map[string]interface{}{"role": "assistant", "content": text}, "finish_reason": "stop"},
		},
		"usage": map[string]interface{}{"prompt_tokens": 0, "completion_tokens": len(strings.Split(text, " ")), "total_tokens": 0},
	}
	newBody, _ := json.Marshal(openaiResp)
	resp.Body = io.NopCloser(bytes.NewReader(newBody))
	resp.ContentLength = int64(len(newBody))
	resp.Header.Set("Content-Type", "application/json")
	return resp, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func init() {
	provider.RegisterGlobalProvider(string(types.ProviderCohere), NewCohereProvider)
}
