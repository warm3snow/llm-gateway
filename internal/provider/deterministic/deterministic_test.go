package deterministic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/warm3snow/llm-gateway/internal/provider"
	"github.com/warm3snow/llm-gateway/internal/types"
)

func TestDeterministicProviderChatCompletion(t *testing.T) {
	prov, err := NewDeterministicProvider(&types.Options{Provider: ProviderName})
	if err != nil {
		t.Fatalf("NewDeterministicProvider returned error: %v", err)
	}

	resp, err := prov.ChatCompletion(context.Background(), &types.ChatCompletionRequest{
		Model: "deterministic-chat",
		Messages: []types.Message{
			{Role: "user", Content: "hello"},
		},
	}, &types.Options{Provider: ProviderName})
	if err != nil {
		t.Fatalf("ChatCompletion returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}

	var body struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage types.Usage `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.ID != "chatcmpl-deterministic" {
		t.Fatalf("id = %q", body.ID)
	}
	if body.Object != "chat.completion" {
		t.Fatalf("object = %q", body.Object)
	}
	if body.Model != "deterministic-chat" {
		t.Fatalf("model = %q", body.Model)
	}
	if len(body.Choices) != 1 || body.Choices[0].Message.Role != "assistant" || body.Choices[0].Message.Content != "deterministic response" || body.Choices[0].FinishReason != "stop" {
		t.Fatalf("unexpected choices: %+v", body.Choices)
	}
	if body.Usage.PromptTokens != 1 || body.Usage.CompletionTokens != 1 || body.Usage.TotalTokens != 2 {
		t.Fatalf("usage = %+v", body.Usage)
	}
}

func TestDeterministicProviderStreamingChatCompletionIncludesUsage(t *testing.T) {
	prov, err := NewDeterministicProvider(&types.Options{Provider: ProviderName})
	if err != nil {
		t.Fatalf("NewDeterministicProvider returned error: %v", err)
	}

	resp, err := prov.ChatCompletion(context.Background(), &types.ChatCompletionRequest{
		Model:  "deterministic-chat",
		Stream: true,
		Messages: []types.Message{
			{Role: "user", Content: "hello"},
		},
	}, &types.Options{Provider: ProviderName})
	if err != nil {
		t.Fatalf("ChatCompletion stream returned error: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read stream body: %v", err)
	}
	body := string(bodyBytes)

	for _, want := range []string{
		"data: {",
		`"object":"chat.completion.chunk"`,
		`"content":"deterministic response"`,
		`"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}`,
		"data: [DONE]",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("stream body missing %q in %s", want, body)
		}
	}
}

func TestDeterministicProviderRegistersGlobally(t *testing.T) {
	prov, err := provider.CreateProvider(ProviderName, &types.Options{Provider: ProviderName})
	if err != nil {
		t.Fatalf("CreateProvider returned error: %v", err)
	}
	if prov.GetName() != ProviderName {
		t.Fatalf("provider name = %q", prov.GetName())
	}
}
