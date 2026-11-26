//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/provider/anthropic"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

// TestStreamedChatCompletionRecordsUsage exercises stream:true through the
// unified /v1/chat/completions route and asserts the usage record carries the
// final streamed usage block.
func TestStreamedChatCompletionRecordsUsage(t *testing.T) {
	upstream := newMockChatUpstream(t, 3, 4)
	cfg, router := setupGateway(t, upstream.URL(), nil)
	seedModelPricing(t, "openai", "gpt-test", 10, 20, 0)
	adminToken := loginAdmin(t, router, cfg)
	virtualKey := createVirtualKey(t, router, adminToken, "/api/v1/virtual-keys", "stream-key", 100, []string{"openai"})

	req := testutil.NewJSONRequest(t, http.MethodPost, "/v1/chat/completions", map[string]interface{}{
		"model":  "gpt-test",
		"stream": true,
		"messages": []map[string]string{
			{"role": "user", "content": "ping"},
		},
	})
	req.Header.Set(cfg.Security.APIKeyHeader, virtualKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "data:")
	assert.Contains(t, w.Body.String(), "[DONE]")

	// Streaming is async through the pipe; wait for the usage record.
	var record *models.UsageRecord
	require.Eventually(t, func() bool {
		var rec models.UsageRecord
		if err := database.GetDB().Where("virtual_key_id IS NOT NULL AND endpoint = ?", "/v1/chat/completions").Order("id DESC").First(&rec).Error; err != nil {
			return false
		}
		record = &rec
		return rec.InputTokens > 0
	}, 5*time.Second, 100*time.Millisecond)
	assert.Equal(t, 3, record.InputTokens)
	assert.Equal(t, 4, record.OutputTokens)
}

// TestAnthropicMessagesEndpoint exercises POST /v1/messages: an Anthropic
// request in, OpenAI-shaped upstream call, Anthropic-shaped response out.
func TestAnthropicMessagesEndpoint(t *testing.T) {
	var upstreamBody struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string      `json:"role"`
			Content interface{} `json:"content"`
		} `json:"messages"`
		MaxTokens int `json:"max_tokens"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat/completions", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&upstreamBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-msg","object":"chat.completion","created":1700000000,"model":"gpt-test","choices":[{"index":0,"message":{"role":"assistant","content":"hi there"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`))
	}))
	t.Cleanup(upstream.Close)

	cfg, router := setupGateway(t, upstream.URL, nil)
	adminToken := loginAdmin(t, router, cfg)
	virtualKey := createVirtualKey(t, router, adminToken, "/api/v1/virtual-keys", "messages-key", 100, []string{"openai"})

	req := testutil.NewJSONRequest(t, http.MethodPost, "/v1/messages", map[string]interface{}{
		"model":      "claude-3-5-sonnet",
		"max_tokens": 256,
		"system":     "be terse",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "hello"},
		},
	})
	req.Header.Set("x-api-key", virtualKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// System prompt translated into a system message for the upstream.
	require.Len(t, upstreamBody.Messages, 2)
	assert.Equal(t, "system", upstreamBody.Messages[0].Role)
	assert.Equal(t, "be terse", upstreamBody.Messages[0].Content)
	assert.Equal(t, 256, upstreamBody.MaxTokens)

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
		} `json:"usage"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "message", resp.Type)
	assert.Equal(t, "assistant", resp.Role)
	assert.Equal(t, "end_turn", resp.StopReason)
	require.Len(t, resp.Content, 1)
	assert.Equal(t, "text", resp.Content[0].Type)
	assert.Equal(t, "hi there", resp.Content[0].Text)
	assert.Equal(t, 5, resp.Usage.InputTokens)
	assert.Equal(t, 2, resp.Usage.OutputTokens)
}

// TestAnthropicStreamTranslation is a provider-level test: an Anthropic SSE
// stream converts into OpenAI chunk SSE ending with usage and [DONE].
func TestAnthropicStreamTranslation(t *testing.T) {
	anthropicSSE := "event: message_start\n" +
		`data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":11,"cache_read_input_tokens":4}}}` + "\n\n" +
		"event: content_block_delta\n" +
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"hello"}}` + "\n\n" +
		"event: message_delta\n" +
		`data: {"type":"message_delta","usage":{"output_tokens":6}}` + "\n\n" +
		"event: message_stop\n" +
		`data: {"type":"message_stop"}` + "\n\n"

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(anthropicSSE)),
	}
	converted := anthropic.Stream(resp, "claude-3")
	defer converted.Body.Close()

	body, err := io.ReadAll(converted.Body)
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, `"content":"hello"`)
	assert.Contains(t, text, `"prompt_tokens":11`)
	assert.Contains(t, text, `"cache_read":4`)
	assert.Contains(t, text, `"completion_tokens":6`)
	assert.Contains(t, text, "[DONE]")
}

var _ = config.Config{}
