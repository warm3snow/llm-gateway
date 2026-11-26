package gemini

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
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
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"
	DefaultTimeout = 120 * time.Second
)

// GeminiProvider Gemini 提供商
type GeminiProvider struct {
	*provider.BaseProvider
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewGeminiProvider 创建 Gemini 提供商
func NewGeminiProvider(opts *types.Options) (provider.Provider, error) {
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

	return &GeminiProvider{
		BaseProvider: &provider.BaseProvider{
			Name:    string(types.ProviderGemini),
			BaseURL: baseURL,
			Endpoints: map[string]string{
				"chatCompletions": "/chat/completions",
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

func (p *GeminiProvider) GetName() string    { return p.Name }
func (p *GeminiProvider) GetBaseURL() string { return p.BaseURL }
func (p *GeminiProvider) GetEndpoints() []string {
	eps := make([]string, 0, len(p.Endpoints))
	for ep := range p.Endpoints {
		eps = append(eps, ep)
	}
	return eps
}

// ChatCompletion 聊天补全 — 转换 OpenAI 格式到 Gemini 格式
func (p *GeminiProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	geminiReq, err := convertChatToGemini(req)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request: %w", err)
	}

	model := strings.TrimPrefix(req.Model, "gemini-")
	if model == "" {
		model = "gemini-pro"
	}

	// 流式请求走 streamGenerateContent?alt=sse；非流式走 generateContent。
	action := ":generateContent"
	if req.Stream {
		action = ":streamGenerateContent?alt=sse"
	}

	url := fmt.Sprintf("%s/models/gemini-%s%s&key=%s", p.BaseURL, model, action, p.APIKey)
	if strings.Contains(p.BaseURL, "googleapis.com") {
		sep := "&"
		if action[0] == ':' {
			sep = "?"
		}
		url = fmt.Sprintf("%s/models/%s%s%skey=%s", p.BaseURL, req.Model, action, sep, p.APIKey)
	}

	bodyBytes, _ := json.Marshal(geminiReq)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 如果是错误响应，直接返回
	if resp.StatusCode != http.StatusOK {
		return resp, nil
	}

	// 流式响应把 Gemini SSE 翻译成 OpenAI chat chunk SSE。
	if req.Stream && strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return streamGeminiToOpenAI(resp, req.Model), nil
	}

	// 转换响应格式
	return convertGeminiResponse(resp)
}

// streamGeminiToOpenAI wraps a Gemini streamGenerateContent SSE stream and
// emits OpenAI chat.completion.chunk events, ending with a usage chunk.
func streamGeminiToOpenAI(resp *http.Response, model string) *http.Response {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		err := translateGeminiStream(resp.Body, pw, model)
		if err != nil && err != io.EOF && err != context.Canceled {
			_ = pw.CloseWithError(err)
		}
	}()
	out := &http.Response{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       pr,
	}
	out.Header.Set("Content-Type", "text/event-stream")
	return out
}

// translateGeminiStream reads Gemini generateContent SSE payloads and writes
// OpenAI-format SSE data lines to w.
func translateGeminiStream(r io.Reader, w io.Writer, model string) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	writeChunk := func(payload map[string]interface{}) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "data: %s\n\n", data)
		return err
	}

	sawContent := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		var event struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
			UsageMetadata *struct {
				PromptTokenCount        int `json:"promptTokenCount"`
				CandidatesTokenCount    int `json:"candidatesTokenCount"`
				TotalTokenCount         int `json:"totalTokenCount"`
				CachedContentTokenCount int `json:"cachedContentTokenCount"`
				ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
			} `json:"usageMetadata"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}

		for _, cand := range event.Candidates {
			for _, part := range cand.Content.Parts {
				if part.Text == "" {
					continue
				}
				sawContent = true
				if err := writeChunk(map[string]interface{}{
					"id": "chatcmpl-gemini", "object": "chat.completion.chunk", "model": model,
					"choices": []map[string]interface{}{
						{"index": 0, "delta": map[string]interface{}{"content": part.Text}},
					},
				}); err != nil {
					return err
				}
			}
			if cand.FinishReason != "" {
				finish := "stop"
				if cand.FinishReason == "MAX_TOKENS" {
					finish = "length"
				}
				if err := writeChunk(map[string]interface{}{
					"id": "chatcmpl-gemini", "object": "chat.completion.chunk", "model": model,
					"choices": []map[string]interface{}{
						{"index": 0, "delta": map[string]interface{}{}, "finish_reason": finish},
					},
				}); err != nil {
					return err
				}
			}
		}

		if m := event.UsageMetadata; m != nil {
			prompt := m.PromptTokenCount
			cacheRead := m.CachedContentTokenCount
			if cacheRead > 0 && cacheRead <= prompt {
				prompt -= cacheRead
			} else {
				cacheRead = 0
			}
			if err := writeChunk(map[string]interface{}{
				"id": "chatcmpl-gemini", "object": "chat.completion.chunk", "model": model,
				"choices": []map[string]interface{}{},
				"usage": map[string]interface{}{
					"prompt_tokens":     prompt,
					"completion_tokens": m.CandidatesTokenCount,
					"total_tokens":      m.TotalTokenCount,
					"cache_read":        cacheRead,
					"reasoning":         m.ThoughtsTokenCount,
				},
			}); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !sawContent {
		_, err := fmt.Fprint(w, "data: [DONE]\n\n")
		return err
	}
	_, err := fmt.Fprint(w, "data: [DONE]\n\n")
	return err
}

// Completion 文本补全 — Gemini 不支持，返回错误
func (p *GeminiProvider) Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("gemini does not support completion endpoint")
}

// Embedding 嵌入
func (p *GeminiProvider) Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error) {
	// Gemini embedding API
	url := fmt.Sprintf("%s/models/%s:embedContent?key=%s", p.BaseURL, req.Model, p.APIKey)

	geminiReq := map[string]interface{}{
		"content": map[string]interface{}{
			"parts": []map[string]string{{"text": fmt.Sprint(req.Input)}},
		},
	}

	bodyBytes, _ := json.Marshal(geminiReq)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	return p.HTTPClient.Do(httpReq)
}

// ImageGeneration 图像生成 — Gemini 不支持
func (p *GeminiProvider) ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("gemini does not support image generation")
}

// AudioSpeech 文本转语音 — Gemini 不支持
func (p *GeminiProvider) AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("gemini does not support audio speech")
}

// AudioTranscription 语音转文本 — Gemini 不支持
func (p *GeminiProvider) AudioTranscription(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("gemini does not support audio transcription")
}

func (p *GeminiProvider) AudioTranslation(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("gemini does not support audio translation")
}

// Models 获取模型列表
func (p *GeminiProvider) Models(ctx context.Context, opts *types.Options) (*http.Response, error) {
	url := fmt.Sprintf("%s/models?key=%s", p.BaseURL, p.APIKey)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return p.HTTPClient.Do(httpReq)
}

// TransformRequest 转换请求格式
func (p *GeminiProvider) TransformRequest(endpoint string, req interface{}) (interface{}, error) {
	switch endpoint {
	case "chat/completions":
		if r, ok := req.(*types.ChatCompletionRequest); ok {
			return convertChatToGemini(r)
		}
	}
	return req, nil
}

// TransformResponse 转换响应格式
func (p *GeminiProvider) TransformResponse(endpoint string, resp *http.Response) (*http.Response, error) {
	if endpoint == "chat/completions" {
		return convertGeminiResponse(resp)
	}
	return resp, nil
}

// convertChatToGemini 将 OpenAI 格式转换为 Gemini 格式
func convertChatToGemini(req *types.ChatCompletionRequest) (map[string]interface{}, error) {
	contents := make([]map[string]interface{}, 0, len(req.Messages))

	for _, msg := range req.Messages {
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		var text string
		switch c := msg.Content.(type) {
		case string:
			text = c
		case []interface{}:
			// 处理多模态内容
			for _, item := range c {
				if m, ok := item.(map[string]interface{}); ok {
					if t, ok := m["text"].(string); ok {
						text = t
						break
					}
				}
			}
		}

		contents = append(contents, map[string]interface{}{
			"role":  role,
			"parts": []map[string]string{{"text": text}},
		})
	}

	result := map[string]interface{}{
		"contents": contents,
	}

	if req.Temperature > 0 {
		result["generationConfig"] = map[string]interface{}{
			"temperature":     req.Temperature,
			"maxOutputTokens": req.MaxTokens,
			"topP":            req.TopP,
		}
	}

	return result, nil
}

// convertGeminiResponse 将 Gemini 响应转换为 OpenAI 格式
func convertGeminiResponse(resp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	var geminiResp map[string]interface{}
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		// 如果不是 JSON，返回原始响应
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	// 检查是否有错误
	if _, ok := geminiResp["error"].(map[string]interface{}); ok {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}

	// 转换为 OpenAI 格式
	candidates, _ := geminiResp["candidates"].([]interface{})
	text := ""
	if len(candidates) > 0 {
		if c, ok := candidates[0].(map[string]interface{}); ok {
			if content, ok := c["content"].(map[string]interface{}); ok {
				if parts, ok := content["parts"].([]interface{}); ok && len(parts) > 0 {
					if p, ok := parts[0].(map[string]interface{}); ok {
						text, _ = p["text"].(string)
					}
				}
			}
		}
	}

	openaiResp := map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%s", randomString(12)),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   getModelFromGeminiResp(geminiResp),
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"message":       map[string]interface{}{"role": "assistant", "content": text},
				"finish_reason": finishReasonFromGemini(geminiResp),
			},
		},
		"usage": geminiUsageToOpenAI(geminiResp),
	}

	newBody, _ := json.Marshal(openaiResp)
	resp.Body = io.NopCloser(bytes.NewReader(newBody))
	resp.StatusCode = http.StatusOK
	resp.Header.Set("Content-Type", "application/json")
	resp.ContentLength = int64(len(newBody))

	return resp, nil
}

// geminiUsageToOpenAI maps usageMetadata to the OpenAI usage shape. Gemini
// counts cached prompt tokens inside promptTokenCount, so they are split out.
func geminiUsageToOpenAI(geminiResp map[string]interface{}) map[string]interface{} {
	empty := map[string]interface{}{"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
	m, ok := geminiResp["usageMetadata"].(map[string]interface{})
	if !ok {
		return empty
	}
	prompt := jsonInt(m["promptTokenCount"])
	out := jsonInt(m["candidatesTokenCount"])
	total := jsonInt(m["totalTokenCount"])
	cacheRead := jsonInt(m["cachedContentTokenCount"])
	if cacheRead > 0 && cacheRead <= prompt {
		prompt -= cacheRead
	} else {
		cacheRead = 0
	}
	usage := map[string]interface{}{
		"prompt_tokens":     prompt,
		"completion_tokens": out,
		"total_tokens":      total,
		"cache_read":        cacheRead,
	}
	if reasoning := jsonInt(m["thoughtsTokenCount"]); reasoning > 0 {
		usage["reasoning"] = reasoning
	}
	return usage
}

// finishReasonFromGemini maps a Gemini finishReason to OpenAI's.
func finishReasonFromGemini(geminiResp map[string]interface{}) string {
	candidates, _ := geminiResp["candidates"].([]interface{})
	if len(candidates) == 0 {
		return "stop"
	}
	c, ok := candidates[0].(map[string]interface{})
	if !ok {
		return "stop"
	}
	switch reason, _ := c["finishReason"].(string); reason {
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "", "STOP":
		return "stop"
	default:
		return "stop"
	}
}

func getModelFromGeminiResp(resp map[string]interface{}) string {
	if model, ok := resp["modelVersion"].(string); ok {
		return model
	}
	return "gemini-pro"
}

// jsonInt converts a decoded JSON number (float64) to int; nil/other → 0.
func jsonInt(v interface{}) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

// randomString returns a random hex id.
func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}

var letters = "abcdefghijklmnopqrstuvwxyz0123456789"

func init() {
	provider.RegisterGlobalProvider(string(types.ProviderGemini), NewGeminiProvider)
}
