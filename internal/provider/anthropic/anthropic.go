package anthropic

import (
	"bufio"
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
	DefaultBaseURL = "https://api.anthropic.com/v1"
	DefaultTimeout = 120 * time.Second
)

// AnthropicProvider Anthropic 提供商
type AnthropicProvider struct {
	*provider.BaseProvider
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// NewAnthropicProvider 创建 Anthropic 提供商
func NewAnthropicProvider(opts *types.Options) (provider.Provider, error) {
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

	return &AnthropicProvider{
		BaseProvider: &provider.BaseProvider{
			Name:    string(types.ProviderAnthropic),
			BaseURL: baseURL,
			Endpoints: map[string]string{
				"chatCompletions": "/messages",
				"completions":     "/complete",
				"models":          "/models",
			},
		},
		APIKey:     apiKey,
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: timeout},
		Timeout:    timeout,
	}, nil
}

// GetName 获取名称
func (p *AnthropicProvider) GetName() string {
	return p.Name
}

// GetBaseURL 获取基础 URL
func (p *AnthropicProvider) GetBaseURL() string {
	return p.BaseURL
}

// ChatCompletion 聊天补全（转换为 Anthropic 格式）
func (p *AnthropicProvider) ChatCompletion(ctx context.Context, req *types.ChatCompletionRequest, opts *types.Options) (*http.Response, error) {
	url := p.BaseURL + "/messages"

	// 将 OpenAI 格式转换为 Anthropic 格式
	anthropicReq := convertToAnthropicRequest(req)

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置 Anthropic 请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// 发送请求
	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// 流式请求把 Anthropic messages SSE 翻译成 OpenAI chat chunk SSE；
	// 非流式按整体响应转换。
	if req.Stream && resp.StatusCode == http.StatusOK && isSSE(resp) {
		return streamAnthropicToOpenAI(resp, req.Model), nil
	}

	// 转换响应格式（从 Anthropic 到 OpenAI）
	convertedResp, err := convertResponseToOpenAI(resp)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	return convertedResp, nil
}

// isSSE reports whether the response carries an SSE stream.
func isSSE(resp *http.Response) bool {
	return strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream")
}

// Stream converts an Anthropic messages SSE response into an OpenAI
// chat-chunk SSE response. Exported for reuse and tests.
func Stream(resp *http.Response, model string) *http.Response {
	return streamAnthropicToOpenAI(resp, model)
}

// streamAnthropicToOpenAI wraps an Anthropic messages SSE stream and emits
// OpenAI chat.completion.chunk events with the same text deltas, finishing
// with a usage-carrying final chunk (as stream_options.include_usage would).
func streamAnthropicToOpenAI(resp *http.Response, model string) *http.Response {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		err := translateAnthropicStream(resp.Body, pw, model)
		if err != nil && err != io.EOF && err != context.Canceled {
			// A broken client pipe is expected; surface others by closing
			// the pipe with the error so the reader sees it.
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

// translateAnthropicStream reads Anthropic messages SSE events and writes
// OpenAI-format SSE data lines to w.
func translateAnthropicStream(r io.Reader, w io.Writer, model string) error {
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

	// inputTokens is remembered from message_start for the final usage chunk.
	inputTokens, cacheRead, cacheWrite := 0, 0, 0
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
			Type  string `json:"type"`
			Delta *struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			Message *struct {
				ID    string `json:"id"`
				Usage *struct {
					InputTokens              int `json:"input_tokens"`
					CacheReadInputTokens     int `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage *struct {
				OutputTokens             int `json:"output_tokens"`
				CacheReadInputTokens     int `json:"cache_read_input_tokens"`
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		switch event.Type {
		case "message_start":
			if event.Message != nil && event.Message.Usage != nil {
				inputTokens = event.Message.Usage.InputTokens
				cacheRead = event.Message.Usage.CacheReadInputTokens
				cacheWrite = event.Message.Usage.CacheCreationInputTokens
			}
		case "content_block_delta":
			if event.Delta != nil && event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				sawContent = true
				if err := writeChunk(map[string]interface{}{
					"id": "chatcmpl-anthropic", "object": "chat.completion.chunk", "model": model,
					"choices": []map[string]interface{}{
						{"index": 0, "delta": map[string]interface{}{"content": event.Delta.Text}},
					},
				}); err != nil {
					return err
				}
			}
		case "message_delta":
			if event.Usage != nil {
				finish := "stop"
				if err := writeChunk(map[string]interface{}{
					"id": "chatcmpl-anthropic", "object": "chat.completion.chunk", "model": model,
					"choices": []map[string]interface{}{
						{"index": 0, "delta": map[string]interface{}{}, "finish_reason": finish},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     inputTokens,
						"completion_tokens": event.Usage.OutputTokens,
						"total_tokens":      inputTokens + event.Usage.OutputTokens,
						"cache_read":        cacheRead,
						"cache_write":       cacheWrite,
					},
				}); err != nil {
					return err
				}
			}
		case "message_stop":
			_, err := fmt.Fprint(w, "data: [DONE]\n\n")
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !sawContent {
		// No content events arrived; still terminate the stream cleanly.
		_, err := fmt.Fprint(w, "data: [DONE]\n\n")
		return err
	}
	return nil
}

// Completion 文本补全
func (p *AnthropicProvider) Completion(ctx context.Context, req *types.CompletionRequest, opts *types.Options) (*http.Response, error) {
	// Anthropic 不支持传统的 completions 端点，使用 messages 代替
	// 将 CompletionRequest 转换为 ChatCompletionRequest
	chatReq := &types.ChatCompletionRequest{
		Model:       req.Model,
		Messages:    []types.Message{{Role: "user", Content: req.Prompt}},
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}

	return p.ChatCompletion(ctx, chatReq, opts)
}

// Embedding 嵌入（Anthropic 不支持）
func (p *AnthropicProvider) Embedding(ctx context.Context, req *types.EmbeddingRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("anthropic does not support embedding endpoint")
}

// ImageGeneration 图像生成（Anthropic 不支持）
func (p *AnthropicProvider) ImageGeneration(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("anthropic does not support image generation")
}

// AudioSpeech 文本转语音（Anthropic 不支持）
func (p *AnthropicProvider) AudioSpeech(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("anthropic does not support audio speech")
}

// AudioTranscription 语音转文本（Anthropic 不支持）
func (p *AnthropicProvider) AudioTranscription(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("anthropic does not support audio transcription")
}

func (p *AnthropicProvider) AudioTranslation(ctx context.Context, req *types.AudioRequest, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("anthropic does not support audio translation")
}

// Models 获取模型列表
func (p *AnthropicProvider) Models(ctx context.Context, opts *types.Options) (*http.Response, error) {
	url := p.BaseURL + "/models"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	return resp, nil
}

// TransformRequest 转换请求
func (p *AnthropicProvider) TransformRequest(endpoint string, req interface{}) (interface{}, error) {
	// 实现请求格式转换
	return req, nil
}

// TransformResponse 转换响应
func (p *AnthropicProvider) TransformResponse(endpoint string, resp *http.Response) (*http.Response, error) {
	// 实现响应格式转换
	return resp, nil
}

// convertToAnthropicRequest 将 OpenAI 请求转换为 Anthropic 格式
func convertToAnthropicRequest(req *types.ChatCompletionRequest) map[string]interface{} {
	// 提取系统消息
	var system string
	var messages []map[string]interface{}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Anthropic 使用单独的 system 参数
			if content, ok := msg.Content.(string); ok {
				system = content
			}
		} else {
			message := map[string]interface{}{
				"role":    msg.Role,
				"content": msg.Content,
			}
			messages = append(messages, message)
		}
	}

	// 构建 Anthropic 请求
	anthropicReq := map[string]interface{}{
		"model":      req.Model,
		"messages":   messages,
		"max_tokens": req.MaxTokens,
	}

	if system != "" {
		anthropicReq["system"] = system
	}

	if req.Temperature > 0 {
		anthropicReq["temperature"] = req.Temperature
	}

	if req.TopP > 0 {
		anthropicReq["top_p"] = req.TopP
	}

	if req.Stream {
		anthropicReq["stream"] = true
	}

	return anthropicReq
}

// convertResponseToOpenAI 将 Anthropic 响应转换为 OpenAI 格式
func convertResponseToOpenAI(resp *http.Response) (*http.Response, error) {
	// 读取 Anthropic 响应
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 解析 Anthropic 响应
	var anthropicResp map[string]interface{}
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 转换为 OpenAI 格式
	openaiResp := convertAnthropicToOpenAIFormat(anthropicResp)

	// 序列化 OpenAI 响应
	openaiBody, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal converted response: %w", err)
	}

	// 创建新的响应
	newResp := &http.Response{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBuffer(openaiBody)),
	}

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			newResp.Header.Add(key, value)
		}
	}

	// 更新 content type
	newResp.Header.Set("Content-Type", "application/json")

	return newResp, nil
}

// convertAnthropicToOpenAIFormat 转换 Anthropic 响应格式到 OpenAI
func convertAnthropicToOpenAIFormat(anthropicResp map[string]interface{}) map[string]interface{} {
	// 构建 OpenAI 格式的响应
	openaiResp := map[string]interface{}{
		"id":      anthropicResp["id"],
		"object":  "chat.completion",
		"created": anthropicResp["created_at"],
		"model":   anthropicResp["model"],
	}

	// 转换内容
	if content, ok := anthropicResp["content"].([]interface{}); ok {
		var messageContent string
		for _, block := range content {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockMap["type"] == "text" {
					messageContent = blockMap["text"].(string)
					break
				}
			}
		}

		message := map[string]interface{}{
			"role":    "assistant",
			"content": messageContent,
		}

		openaiResp["choices"] = []map[string]interface{}{
			{
				"index":         0,
				"message":       message,
				"finish_reason": anthropicResp["stop_reason"],
			},
		}
	}

	// 使用信息
	if usage, ok := anthropicResp["usage"].(map[string]interface{}); ok {
		inputTokens := jsonInt(usage["input_tokens"])
		outputTokens := jsonInt(usage["output_tokens"])
		cacheRead := jsonInt(usage["cache_read_input_tokens"])
		cacheWrite := jsonInt(usage["cache_creation_input_tokens"])

		openaiResp["usage"] = map[string]interface{}{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
			"total_tokens":      inputTokens + outputTokens,
			"cache_read":        cacheRead,
			"cache_write":       cacheWrite,
		}
	}

	return openaiResp
}

// jsonInt converts a decoded JSON number (float64) to int; nil/other → 0.
func jsonInt(v interface{}) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

// 注册 Anthropic provider
func init() {
	provider.RegisterGlobalProvider(string(types.ProviderAnthropic), NewAnthropicProvider)
}
