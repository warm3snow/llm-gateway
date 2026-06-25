package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	// 转换响应格式（从 Anthropic 到 OpenAI）
	convertedResp, err := convertResponseToOpenAI(resp)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	return convertedResp, nil
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
func (p *AnthropicProvider) AudioTranscription(ctx context.Context, req map[string]interface{}, opts *types.Options) (*http.Response, error) {
	return nil, fmt.Errorf("anthropic does not support audio transcription")
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
		// 将 float64 转换为 int (JSON 数字默认是 float64)
		inputTokens := int(usage["input_tokens"].(float64))
		outputTokens := int(usage["output_tokens"].(float64))
		
		openaiResp["usage"] = map[string]interface{}{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
			"total_tokens":      inputTokens + outputTokens,
		}
	}

	return openaiResp
}

// 注册 Anthropic provider
func init() {
	provider.RegisterGlobalProvider(string(types.ProviderAnthropic), NewAnthropicProvider)
}
