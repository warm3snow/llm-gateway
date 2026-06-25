package openai

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

// TestNewOpenAIProvider 测试创建 OpenAI 提供商
func TestNewOpenAIProvider(t *testing.T) {
	tests := []struct {
		name        string
		opts        *types.Options
		expectError bool
		expectName  string
	}{
		{
			name:        "Create with API key",
			opts:        &types.Options{Provider: "openai", APIKey: "sk-test123"},
			expectError: false,
			expectName:  "openai",
		},
		{
			name:        "Create with virtual key",
			opts:        &types.Options{Provider: "openai", VirtualKey: "vk-test456"},
			expectError: false,
			expectName:  "openai",
		},
		{
			name:        "Create with custom host",
			opts:        &types.Options{Provider: "openai", APIKey: "sk-test", CustomHost: "https://custom.openai.com/v1"},
			expectError: false,
			expectName:  "openai",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, err := NewOpenAIProvider(tt.opts)

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

// TestOpenAIProvider_GetBaseURL 测试获取基础 URL
func TestOpenAIProvider_GetBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		opts        *types.Options
		expectedURL string
	}{
		{
			name:        "Default URL",
			opts:        &types.Options{Provider: "openai", APIKey: "sk-test"},
			expectedURL: "https://api.openai.com/v1",
		},
		{
			name:        "Custom host",
			opts:        &types.Options{Provider: "openai", APIKey: "sk-test", CustomHost: "https://custom.api.com/v1"},
			expectedURL: "https://custom.api.com/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, err := NewOpenAIProvider(tt.opts)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedURL, prov.GetBaseURL())
		})
	}
}

// TestOpenAIProvider_GetEndpoints 测试获取端点
func TestOpenAIProvider_GetEndpoints(t *testing.T) {
	opts := &types.Options{Provider: "openai", APIKey: "sk-test"}
	prov, err := NewOpenAIProvider(opts)
	assert.NoError(t, err)

	endpoints := prov.GetEndpoints()
	assert.NotNil(t, endpoints)
	assert.NotEmpty(t, endpoints)

	// 检查必要的端点
	expectedEndpoints := []string{
		"chatCompletions",
		"completions",
		"embeddings",
		"images",
		"audio",
		"transcriptions",
		"translations",
		"models",
		"files",
		"batches",
	}

	for _, ep := range expectedEndpoints {
		assert.Contains(t, endpoints, ep)
	}
}

// TestOpenAIProvider_ChatCompletion 测试聊天补全
func TestOpenAIProvider_ChatCompletion(t *testing.T) {
	// 创建模拟 HTTP 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer sk-test123", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-123","object":"chat.completion","created":1234567890,"model":"gpt-3.5-turbo","choices":[{"index":0,"message":{"role":"assistant","content":"Hello!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`))
	}))
	defer server.Close()

	opts := &types.Options{Provider: "openai", APIKey: "sk-test123"}
	prov, err := NewOpenAIProvider(opts)
	assert.NoError(t, err)

	// 修改基础 URL 指向测试服务器
	openaiProv := prov.(*OpenAIProvider)
	openaiProv.BaseURL = server.URL + "/v1"

	ctx := context.Background()
	req := &types.ChatCompletionRequest{
		Model: "gpt-3.5-turbo",
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
	assert.Contains(t, string(body), "chatcmpl-123")
}

// TestOpenAIProvider_Completion 测试文本补全
func TestOpenAIProvider_Completion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/completions", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"cmpl-123","object":"text_completion","created":1234567890,"model":"text-davinci-003","choices":[{"text":"Hello world!","index":0,"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`))
	}))
	defer server.Close()

	opts := &types.Options{Provider: "openai", APIKey: "sk-test123"}
	prov, err := NewOpenAIProvider(opts)
	assert.NoError(t, err)

	openaiProv := prov.(*OpenAIProvider)
	openaiProv.BaseURL = server.URL + "/v1"

	ctx := context.Background()
	req := &types.CompletionRequest{
		Model:  "text-davinci-003",
		Prompt: "Hello",
	}

	resp, err := prov.Completion(ctx, req, opts)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestOpenAIProvider_Embedding 测试嵌入
func TestOpenAIProvider_Embedding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/embeddings", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[{"object":"embedding","embedding":[0.1,0.2,0.3],"index":0}],"model":"text-embedding-ada-002","usage":{"prompt_tokens":5,"total_tokens":5}}`))
	}))
	defer server.Close()

	opts := &types.Options{Provider: "openai", APIKey: "sk-test123"}
	prov, err := NewOpenAIProvider(opts)
	assert.NoError(t, err)

	openaiProv := prov.(*OpenAIProvider)
	openaiProv.BaseURL = server.URL + "/v1"

	ctx := context.Background()
	req := &types.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: "Hello world",
	}

	resp, err := prov.Embedding(ctx, req, opts)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestOpenAIProvider_Models 测试获取模型列表
func TestOpenAIProvider_Models(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/models", r.URL.Path)
		assert.Equal(t, "Bearer sk-test123", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"object":"list","data":[{"id":"gpt-3.5-turbo","object":"model","created":1234567890,"owned_by":"openai"},{"id":"text-embedding-ada-002","object":"model","created":1234567890,"owned_by":"openai"}]}`))
	}))
	defer server.Close()

	opts := &types.Options{Provider: "openai", APIKey: "sk-test123"}
	prov, err := NewOpenAIProvider(opts)
	assert.NoError(t, err)

	openaiProv := prov.(*OpenAIProvider)
	openaiProv.BaseURL = server.URL + "/v1"

	ctx := context.Background()
	resp, err := prov.Models(ctx, opts)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestOpenAIProvider_TransformRequest 测试请求转换
func TestOpenAIProvider_TransformRequest(t *testing.T) {
	opts := &types.Options{Provider: "openai", APIKey: "sk-test"}
	prov, err := NewOpenAIProvider(opts)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		endpoint string
		req      interface{}
	}{
		{
			name:     "Chat completion request",
			endpoint: "chatCompletions",
			req: &types.ChatCompletionRequest{
				Model:    "gpt-3.5-turbo",
				Messages: []types.Message{{Role: "user", Content: "Hello"}},
			},
		},
		{
			name:     "Embedding request",
			endpoint: "embeddings",
			req: &types.EmbeddingRequest{
				Model: "text-embedding-ada-002",
				Input: "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := prov.TransformRequest(tt.endpoint, tt.req)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			// OpenAI 格式已经是标准格式，不需要转换
			assert.Equal(t, tt.req, result)
		})
	}
}

// TestOpenAIProvider_TransformResponse 测试响应转换
func TestOpenAIProvider_TransformResponse(t *testing.T) {
	opts := &types.Options{Provider: "openai", APIKey: "sk-test"}
	prov, err := NewOpenAIProvider(opts)
	assert.NoError(t, err)

	tests := []struct {
		name     string
		endpoint string
		resp     *http.Response
	}{
		{
			name:     "Chat completion response",
			endpoint: "chatCompletions",
			resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"id":"chatcmpl-123"}`)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := prov.TransformResponse(tt.endpoint, tt.resp)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			// OpenAI 响应通常不需要转换
			assert.Equal(t, tt.resp, result)
		})
	}
}

// TestOpenAIProvider_Timeout 测试超时设置
func TestOpenAIProvider_Timeout(t *testing.T) {
	opts := &types.Options{
		Provider:       "openai",
		APIKey:         "sk-test",
		RequestTimeout: 5000, // 5 秒
	}

	prov, err := NewOpenAIProvider(opts)
	assert.NoError(t, err)

	openaiProv := prov.(*OpenAIProvider)
	assert.Equal(t, 5000*time.Millisecond, openaiProv.Timeout)
	assert.Equal(t, 5000*time.Millisecond, openaiProv.HTTPClient.Timeout)
}

// TestOpenAIProvider_StreamRequest 测试流式请求
func TestOpenAIProvider_StreamRequest(t *testing.T) {
	// 创建一个简单的测试服务器来模拟 SSE 流
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		// OpenAI 使用请求体中的 stream:true，而不是查询参数
		// 所以我们只检查路径

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		// 写入 SSE 数据
		data := []string{
			"data: {\"id\":\"chatcmpl-123\",\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}",
			"data: {\"id\":\"chatcmpl-123\",\"choices\":[{\"delta\":{\"content\":\" world\"}}]}",
			"data: [DONE]",
		}

		for _, d := range data {
			w.Write([]byte(d + "\n\n"))
			w.(http.Flusher).Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer server.Close()

	opts := &types.Options{Provider: "openai", APIKey: "sk-test123"}
	prov, err := NewOpenAIProvider(opts)
	assert.NoError(t, err)

	openaiProv := prov.(*OpenAIProvider)
	openaiProv.BaseURL = server.URL + "/v1"

	ctx := context.Background()
	req := &types.ChatCompletionRequest{
		Model:    "gpt-3.5-turbo",
		Messages: []types.Message{{Role: "user", Content: "Hello"}},
		Stream:   true,
	}

	resp, err := prov.ChatCompletion(ctx, req, opts)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

// BenchmarkOpenAIProvider_ChatCompletion 性能测试
func BenchmarkOpenAIProvider_ChatCompletion(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"chatcmpl-123","choices":[{"message":{"content":"test"}}]}`))
	}))
	defer server.Close()

	opts := &types.Options{Provider: "openai", APIKey: "sk-test"}
	prov, _ := NewOpenAIProvider(opts)
	openaiProv := prov.(*OpenAIProvider)
	openaiProv.BaseURL = server.URL + "/v1"

	ctx := context.Background()
	req := &types.ChatCompletionRequest{
		Model:    "gpt-3.5-turbo",
		Messages: []types.Message{{Role: "user", Content: "test"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prov.ChatCompletion(ctx, req, opts)
	}
}
