package deterministic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/warm3snow/llm-gateway/internal/provider"
	"github.com/warm3snow/llm-gateway/internal/types"
)

const (
	ProviderName = "deterministic"
	baseURL      = "in-memory://deterministic"
	createdAt    = int64(1700000000)
)

type DeterministicProvider struct {
	*provider.BaseProvider
}

var _ provider.Provider = (*DeterministicProvider)(nil)

func NewDeterministicProvider(opts *types.Options) (provider.Provider, error) {
	return &DeterministicProvider{
		BaseProvider: &provider.BaseProvider{
			Name:    ProviderName,
			BaseURL: baseURL,
			Endpoints: map[string]string{
				"chatCompletions": "/chat/completions",
				"completions":     "/completions",
				"embeddings":      "/embeddings",
				"models":          "/models",
			},
		},
	}, nil
}

func (p *DeterministicProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req != nil && req.Stream {
		return jsonResponse(http.StatusOK, "text/event-stream", streamChatBody(modelName(req.Model)))
	}

	body := map[string]interface{}{
		"id":      "chatcmpl-deterministic",
		"object":  "chat.completion",
		"created": createdAt,
		"model":   modelName(reqModel(req)),
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": "deterministic response",
				},
				"finish_reason": "stop",
			},
		},
		"usage": types.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}
	return marshalJSONResponse(http.StatusOK, body)
}

func (p *DeterministicProvider) Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req != nil && req.Stream {
		return jsonResponse(http.StatusOK, "text/event-stream", streamCompletionBody(modelName(req.Model)))
	}

	model := "deterministic-completion"
	if req != nil && req.Model != "" {
		model = req.Model
	}
	body := map[string]interface{}{
		"id":      "cmpl-deterministic",
		"object":  "text_completion",
		"created": createdAt,
		"model":   model,
		"choices": []map[string]interface{}{
			{
				"text":          "deterministic response",
				"index":         0,
				"finish_reason": "stop",
			},
		},
		"usage": types.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
	}
	return marshalJSONResponse(http.StatusOK, body)
}

func (p *DeterministicProvider) Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	model := "deterministic-embedding"
	if req != nil && req.Model != "" {
		model = req.Model
	}
	body := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{
				"object":    "embedding",
				"embedding": []float64{0.01, 0.02, 0.03},
				"index":     0,
			},
		},
		"model": model,
		"usage": map[string]int{"prompt_tokens": 1, "total_tokens": 1},
	}
	return marshalJSONResponse(http.StatusOK, body)
}

func (p *DeterministicProvider) ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return unsupported(ctx, "image generation")
}

func (p *DeterministicProvider) AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return unsupported(ctx, "audio speech")
}

func (p *DeterministicProvider) AudioTranscription(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return unsupported(ctx, "audio transcription")
}

func (p *DeterministicProvider) Models(ctx context.Context, opts *types.Options) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	body := map[string]interface{}{
		"object": "list",
		"data": []map[string]interface{}{
			{"id": "deterministic-chat", "object": "model", "created": createdAt, "owned_by": ProviderName},
			{"id": "deterministic-completion", "object": "model", "created": createdAt, "owned_by": ProviderName},
			{"id": "deterministic-embedding", "object": "model", "created": createdAt, "owned_by": ProviderName},
		},
	}
	return marshalJSONResponse(http.StatusOK, body)
}

func (p *DeterministicProvider) TransformRequest(endpoint string, req interface{}) (interface{}, error) {
	return req, nil
}

func (p *DeterministicProvider) TransformResponse(endpoint string, resp *http.Response) (*http.Response, error) {
	return resp, nil
}

func unsupported(ctx context.Context, operation string) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return marshalJSONResponse(http.StatusNotImplemented, map[string]interface{}{
		"error": types.ErrorResponse{
			Message: fmt.Sprintf("deterministic provider does not support %s", operation),
			Type:    "unsupported_operation",
		},
	})
}

func reqModel(req *types.ChatCompletionRequest) string {
	if req == nil {
		return ""
	}
	return req.Model
}

func modelName(model string) string {
	if model == "" {
		return "deterministic-chat"
	}
	return model
}

func marshalJSONResponse(status int, body interface{}) (*http.Response, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return jsonResponse(status, "application/json", string(buf))
}

func jsonResponse(status int, contentType string, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:        http.Header{"Content-Type": []string{contentType}},
		Body:          io.NopCloser(bytes.NewBufferString(body)),
		ContentLength: int64(len(body)),
	}, nil
}

func streamChatBody(model string) string {
	chunks := []interface{}{
		map[string]interface{}{
			"id":      "chatcmpl-deterministic",
			"object":  "chat.completion.chunk",
			"created": createdAt,
			"model":   model,
			"choices": []map[string]interface{}{
				{"index": 0, "delta": map[string]string{"role": "assistant", "content": "deterministic response"}, "finish_reason": nil},
			},
		},
		map[string]interface{}{
			"id":      "chatcmpl-deterministic",
			"object":  "chat.completion.chunk",
			"created": createdAt,
			"model":   model,
			"choices": []map[string]interface{}{},
			"usage":   types.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		},
	}
	return sseBody(chunks)
}

func streamCompletionBody(model string) string {
	chunks := []interface{}{
		map[string]interface{}{
			"id":      "cmpl-deterministic",
			"object":  "text_completion.chunk",
			"created": createdAt,
			"model":   model,
			"choices": []map[string]interface{}{
				{"text": "deterministic response", "index": 0, "finish_reason": "stop"},
			},
			"usage": types.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2},
		},
	}
	return sseBody(chunks)
}

func sseBody(chunks []interface{}) string {
	var b strings.Builder
	for _, chunk := range chunks {
		payload, _ := json.Marshal(chunk)
		b.WriteString("data: ")
		b.Write(payload)
		b.WriteString("\n\n")
	}
	b.WriteString("data: [DONE]\n\n")
	return b.String()
}

func init() {
	provider.RegisterGlobalProvider(ProviderName, NewDeterministicProvider)
}
