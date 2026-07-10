package anthropic

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// TestNewAnthropicProvider 测试创建 Anthropic 提供商
func TestNewAnthropicProvider(t *testing.T) {
	tests := []struct {
		name        string
		opts        *types.Options
		expectError bool
		expectName  string
	}{
		{
			name:        "Create with API key",
			opts:        &types.Options{Provider: "anthropic", APIKey: "sk-ant-test123"},
			expectError: false,
			expectName:  "anthropic",
		},
		{
			name:        "Create with virtual key",
			opts:        &types.Options{Provider: "anthropic", VirtualKey: "vk-test456"},
			expectError: false,
			expectName:  "anthropic",
		},
		{
			name:        "Create with custom host",
			opts:        &types.Options{Provider: "anthropic", APIKey: "sk-ant-test", CustomHost: "https://custom.anthropic.com/v1"},
			expectError: false,
			expectName:  "anthropic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, err := NewAnthropicProvider(tt.opts)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, prov)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, prov)
				assert.Equal(t, tt.expectName, prov.GetName())
			}
		})
	}
}

// TestAnthropicProvider_GetBaseURL 测试获取基础 URL
func TestAnthropicProvider_GetBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		opts        *types.Options
		expectedURL string
	}{
		{
			name:        "Default URL",
			opts:        &types.Options{Provider: "anthropic", APIKey: "sk-ant-test"},
			expectedURL: "https://api.anthropic.com/v1",
		},
		{
			name:        "Custom host",
			opts:        &types.Options{Provider: "anthropic", APIKey: "sk-ant-test", CustomHost: "https://custom.api.com/v1"},
			expectedURL: "https://custom.api.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, err := NewAnthropicProvider(tt.opts)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedURL, prov.GetBaseURL())
		})
	}
}

// TestAnthropicProvider_GetEndpoints 测试获取端点
func TestAnthropicProvider_GetEndpoints(t *testing.T) {
	opts := &types.Options{Provider: "anthropic", APIKey: "sk-ant-test"}
	prov, err := NewAnthropicProvider(opts)
	assert.NoError(t, err)

	endpoints := prov.GetEndpoints()
	assert.NotNil(t, endpoints)
	assert.NotEmpty(t, endpoints)

	// 检查必要的端点
	expectedEndpoints := []string{
		"chatCompletions",
		"completions",
		"models",
	}

	for _, ep := range expectedEndpoints {
		assert.Contains(t, endpoints, ep)
	}
}

// TestAnthropicProvider_ChatCompletion 测试聊天补全
func TestAnthropicProvider_ChatCompletion(t *testing.T) {
	// 创建模拟 HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "sk-ant-test123", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_123","type":"message","role":"assistant","content":[{"type":"text","text":"Hello! How can I help you?"}],"model":"claude-3-opus-20240229","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":15}}`))
	}))
	defer server.Close()

	opts := &types.Options{Provider: "anthropic", APIKey: "sk-ant-test123"}
	prov, err := NewAnthropicProvider(opts)
	assert.NoError(t, err)

	// 修改基础 URL 指向测试服务器
	anthropicProv := prov.(*AnthropicProvider)
	anthropicProv.BaseURL = server.URL + "/v1"

	ctx := context.Background()
	req := &types.ChatCompletionRequest{
		Model: "claude-3-opus-20240229",
		Messages: []types.Message{
			{Role: "user", Content: "Hello!"},
		},
	}

	resp, err := prov.ChatCompletion(ctx, req, opts)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// 验证响应体
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "msg_123")
}

// TestAnthropicProvider_Completion 测试文本补全
func TestAnthropicProvider_Completion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Anthropic 不支持传统的 completions 端点，应该使用 messages
		assert.Equal(t, "/v1/messages", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_456","content":[{"type":"text","text":"Once upon a time..."}],"stop_reason":"end_turn"}`))
	}))
	defer server.Close()

	opts := &types.Options{Provider: "anthropic", APIKey: "sk-ant-test123"}
	prov, err := NewAnthropicProvider(opts)
	assert.NoError(t, err)

	anthropicProv := prov.(*AnthropicProvider)
	anthropicProv.BaseURL = server.URL + "/v1"

	ctx := context.Background()
	req := &types.CompletionRequest{
		Model:  "claude-3-opus-20240229",
		Prompt: "Once upon a time",
	}

	resp, err := prov.Completion(ctx, req, opts)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestAnthropicProvider_Embedding 测试嵌入（不支持）
func TestAnthropicProvider_Embedding(t *testing.T) {
	opts := &types.Options{Provider: "anthropic", APIKey: "sk-ant-test"}
	prov, err := NewAnthropicProvider(opts)
	assert.NoError(t, err)

	ctx := context.Background()
	req := &types.EmbeddingRequest{
		Model: "claude-3-opus-20240229",
		Input: "test",
	}

	resp, err := prov.Embedding(ctx, req, opts)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "does not support embedding")
}

// TestAnthropicProvider_ImageGeneration 测试图像生成（不支持）
func TestAnthropicProvider_ImageGeneration(t *testing.T) {
	opts := &types.Options{Provider: "anthropic", APIKey: "sk-ant-test"}
	prov, err := NewAnthropicProvider(opts)
	assert.NoError(t, err)

	ctx := context.Background()
	req := map[string]interface{}{
		"model":  "claude-3-opus-20240229",
		"prompt": "Generate an image",
	}

	resp, err := prov.ImageGeneration(ctx, req, opts)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "does not support image generation")
}

// TestAnthropicProvider_AudioSpeech 测试文本转语音（不支持）
func TestAnthropicProvider_AudioSpeech(t *testing.T) {
	opts := &types.Options{Provider: "anthropic", APIKey: "sk-ant-test"}
	prov, err := NewAnthropicProvider(opts)
	assert.NoError(t, err)

	ctx := context.Background()
	req := map[string]interface{}{
		"model": "claude-3-opus-20240229",
		"input": "Hello",
	}

	resp, err := prov.AudioSpeech(ctx, req, opts)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "does not support audio speech")
}

// TestAnthropicProvider_AudioTranscription 测试语音转文本（不支持）
func TestAnthropicProvider_AudioTranscription(t *testing.T) {
	opts := &types.Options{Provider: "anthropic", APIKey: "sk-ant-test"}
	prov, err := NewAnthropicProvider(opts)
	assert.NoError(t, err)

	ctx := context.Background()
	req := &types.AudioRequest{Fields: map[string][]string{"model": {"claude-3-opus-20240229"}}}

	resp, err := prov.AudioTranscription(ctx, req, opts)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "does not support audio transcription")
}

// TestAnthropicProvider_Models 测试获取模型列表
func TestAnthropicProvider_Models(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "sk-ant-test123", r.Header.Get("x-api-key"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[{"id":"claude-3-opus-20240229","type":"model"},{"id":"claude-3-sonnet-20240229","type":"model"}]}`))
	}))
	defer server.Close()

	opts := &types.Options{Provider: "anthropic", APIKey: "sk-ant-test123"}
	prov, err := NewAnthropicProvider(opts)
	assert.NoError(t, err)

	anthropicProv := prov.(*AnthropicProvider)
	anthropicProv.BaseURL = server.URL + "/v1"

	ctx := context.Background()
	resp, err := prov.Models(ctx, opts)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestConvertToAnthropicRequest 测试请求格式转换
func TestConvertToAnthropicRequest(t *testing.T) {
	tests := []struct {
		name     string
		req      *types.ChatCompletionRequest
		expected map[string]interface{}
	}{
		{
			name: "Simple message",
			req: &types.ChatCompletionRequest{
				Model: "claude-3-opus-20240229",
				Messages: []types.Message{
					{Role: "user", Content: "Hello!"},
				},
				MaxTokens:   100,
				Temperature: 0.7,
			},
		},
		{
			name: "With system message",
			req: &types.ChatCompletionRequest{
				Model: "claude-3-opus-20240229",
				Messages: []types.Message{
					{Role: "system", Content: "You are a helpful assistant."},
					{Role: "user", Content: "Hello!"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anthropicReq := convertToAnthropicRequest(tt.req)

			assert.NotNil(t, anthropicReq)
			assert.Equal(t, tt.req.Model, anthropicReq["model"])
			assert.NotNil(t, anthropicReq["messages"])

			if tt.req.MaxTokens > 0 {
				assert.Equal(t, tt.req.MaxTokens, anthropicReq["max_tokens"])
			}

			if tt.req.Temperature > 0 {
				assert.Equal(t, tt.req.Temperature, anthropicReq["temperature"])
			}
		})
	}
}

// TestConvertResponseToOpenAI 测试响应格式转换
func TestConvertResponseToOpenAI(t *testing.T) {
	// 创建模拟 Anthropic 响应
	anthropicBody := `{"id":"msg_123","type":"message","role":"assistant","content":[{"type":"text","text":"Hello! How can I help you?"}],"model":"claude-3-opus-20240229","stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":15}}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(anthropicBody)),
	}
	resp.Header.Set("Content-Type", "application/json")

	openaiResp, err := convertResponseToOpenAI(resp)
	assert.NoError(t, err)
	assert.NotNil(t, openaiResp)

	// 验证 OpenAI 格式
	assert.Equal(t, http.StatusOK, openaiResp.StatusCode)

	body, err := io.ReadAll(openaiResp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(body), "chat.completion")
	assert.Contains(t, string(body), "Hello! How can I help you?")
}

// TestAnthropicProvider_Timeout 测试超时设置
func TestAnthropicProvider_Timeout(t *testing.T) {
	opts := &types.Options{
		Provider:       "anthropic",
		APIKey:         "sk-ant-test",
		RequestTimeout: 5000, // 5 秒
	}

	prov, err := NewAnthropicProvider(opts)
	assert.NoError(t, err)

	anthropicProv := prov.(*AnthropicProvider)
	assert.Equal(t, 5000*time.Millisecond, anthropicProv.Timeout)
	assert.Equal(t, 5000*time.Millisecond, anthropicProv.HTTPClient.Timeout)
}

// BenchmarkAnthropicProvider_ChatCompletion 性能测试
func BenchmarkAnthropicProvider_ChatCompletion(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"msg_123","content":[{"type":"text","text":"test"}]}`))
	}))
	defer server.Close()

	opts := &types.Options{Provider: "anthropic", APIKey: "sk-ant-test"}
	prov, _ := NewAnthropicProvider(opts)
	anthropicProv := prov.(*AnthropicProvider)
	anthropicProv.BaseURL = server.URL + "/v1"

	ctx := context.Background()
	req := &types.ChatCompletionRequest{
		Model:    "claude-3-opus-20240229",
		Messages: []types.Message{{Role: "user", Content: "test"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prov.ChatCompletion(ctx, req, opts)
	}
}
