package proxy

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// TestNewProxyHandler 测试创建 ProxyHandler
func TestNewProxyHandler(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewProxyHandler(cfg, nil)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.Config)
	assert.NotNil(t, handler.ProviderFactory)
	assert.NotNil(t, handler.ModelSelector)
}

// TestNewProxyHandlerWithConfig 测试使用自定义配置创建 ProxyHandler
func TestNewProxyHandlerWithConfig(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
	}

	handler := NewProxyHandler(cfg, nil)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.Config)
}

// TestProviderChain 测试 fallback 链组装：主 provider 在前，fallbacks 按序
// 跟随；未知名称与自引用被跳过。
func TestProviderChain(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.Providers = map[string]types.Options{
		"openai":    {Provider: "openai", APIKey: "sk-a"},
		"anthropic": {Provider: "anthropic", APIKey: "sk-b"},
	}
	handler := NewProxyHandler(cfg, nil)

	primary := &types.Options{Provider: "openai", APIKey: "sk-a", Fallbacks: []string{"anthropic", "missing", "openai"}}
	chain := handler.providerChain(primary)
	assert.Len(t, chain, 2)
	assert.Equal(t, "openai", chain[0].Provider)
	assert.Equal(t, "anthropic", chain[1].Provider)
}

// TestRetryWait tests exponential backoff bounds.
func TestRetryWait(t *testing.T) {
	base := retryWait(0, nil)
	if base <= 0 {
		t.Fatal("expected positive wait")
	}
	capped := retryWait(10, nil)
	if capped > 4*time.Second {
		t.Fatalf("expected cap at 4s, got %v", capped)
	}
}

// TestStopReasonFromFinish tests finish_reason mapping.
func TestStopReasonFromFinish(t *testing.T) {
	assert.Equal(t, "end_turn", stopReasonFromFinish("stop"))
	assert.Equal(t, "max_tokens", stopReasonFromFinish("length"))
	assert.Equal(t, "tool_use", stopReasonFromFinish("tool_calls"))
	assert.Equal(t, "", stopReasonFromFinish(""))
}

// TestCanonicalToMessages tests OpenAI→Anthropic response conversion.
func TestCanonicalToMessages(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1700000000,"model":"gpt-test",` +
		`"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],` +
		`"usage":{"prompt_tokens":3,"completion_tokens":5,"total_tokens":8,"cache_read":2}}`)
	out, err := canonicalToMessages(body, "claude-3")
	assert.NoError(t, err)

	var resp struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Role       string `json:"role"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Content    []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			CacheRead    int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	assert.NoError(t, json.Unmarshal(out, &resp))
	assert.Equal(t, "message", resp.Type)
	assert.Equal(t, "assistant", resp.Role)
	assert.Equal(t, "gpt-test", resp.Model)
	assert.Equal(t, "end_turn", resp.StopReason)
	assert.Len(t, resp.Content, 1)
	assert.Equal(t, "text", resp.Content[0].Type)
	assert.Equal(t, "hello", resp.Content[0].Text)
	assert.Equal(t, 3, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
	assert.Equal(t, 2, resp.Usage.CacheRead)
}

// TestConvertMessagesToCanonical tests Anthropic→canonical request conversion.
func TestConvertMessagesToCanonical(t *testing.T) {
	req := &aMessagesRequest{
		Model:     "claude-3",
		MaxTokens: 100,
		System:    json.RawMessage(`"be terse"`),
		Messages: []aMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
			{Role: "assistant", Content: json.RawMessage(`[{"type":"text","text":"hello"}]`)},
		},
	}
	out := convertMessagesToCanonical(req)
	assert.Equal(t, "claude-3", out.Model)
	assert.Equal(t, 100, out.MaxTokens)
	assert.Len(t, out.Messages, 3)
	assert.Equal(t, "system", out.Messages[0].Role)
	assert.Equal(t, "be terse", out.Messages[0].Content)
	assert.Equal(t, "hi", out.Messages[1].Content)
	assert.Equal(t, "hello", out.Messages[2].Content)
}

// TestExtractUsageFromEvent tests usage extraction from OpenAI and Anthropic
// stream events.
func TestExtractUsageFromEvent(t *testing.T) {
	openai := []byte(`{"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`)
	u := extractUsageFromEvent(openai)
	assert.NotNil(t, u)
	assert.Equal(t, 10, u.PromptTokens)

	anthropic := []byte(`{"type":"message_start","message":{"usage":{"input_tokens":7,"output_tokens":0}}}`)
	u = extractUsageFromEvent(anthropic)
	assert.NotNil(t, u)
	assert.Equal(t, 7, u.PromptTokens)

	assert.Nil(t, extractUsageFromEvent([]byte(`{"choices":[{"delta":{"content":"x"}}]}`)))
}

// TestEstimateTokens tests the CJK/ASCII heuristic.
func TestEstimateTokens(t *testing.T) {
	assert.Equal(t, 0, estimateTokens(""))
	assert.Equal(t, 1, estimateTokens("abcd")) // 4 ascii / 4
}
