package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/metrics"
	"github.com/warm3snow/llm-gateway/internal/middleware"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// HandleMessages serves the Anthropic Messages API at POST /v1/messages.
// A client speaking Anthropic's protocol gets Anthropic-shaped responses; the
// request is translated to the canonical ChatCompletionRequest and routed
// through the same provider chain as /v1/chat/completions, then the upstream
// reply is translated back into Anthropic messages (streaming included).

type aTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type aContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	// tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// tool_result
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
}

type aMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type aMessagesRequest struct {
	Model         string          `json:"model"`
	Messages      []aMessage      `json:"messages"`
	System        json.RawMessage `json:"system,omitempty"`
	MaxTokens     int             `json:"max_tokens"`
	Metadata      struct{}        `json:"-"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	Tools         []interface{}   `json:"tools,omitempty"`
}

// aMessagesResponse is the Anthropic Messages reply shape.
type aMessagesResponse struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Model        string          `json:"model"`
	Content      []aContentBlock `json:"content"`
	StopReason   *string         `json:"stop_reason"`
	StopSequence *string         `json:"stop_sequence"`
	Usage        aUsage          `json:"usage"`
}

type aUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// contentBlocks flattens an Anthropic content field (string or block array)
// into blocks.
func contentBlocks(raw json.RawMessage) []aContentBlock {
	if len(raw) == 0 {
		return nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []aContentBlock{{Type: "text", Text: text}}
	}
	var blocks []aContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		return blocks
	}
	return nil
}

// blocksToText extracts plain text from content blocks.
func blocksToText(blocks []aContentBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		if b.Type == "text" {
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}

// convertMessagesToCanonical maps an Anthropic request to the gateway's
// canonical chat request.
func convertMessagesToCanonical(req *aMessagesRequest) *types.ChatCompletionRequest {
	out := &types.ChatCompletionRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
	}
	if len(req.StopSequences) > 0 {
		out.Stop = req.StopSequences
	}
	if req.Temperature != nil {
		out.Temperature = *req.Temperature
	}
	if req.TopP != nil {
		out.TopP = *req.TopP
	}
	if len(req.Tools) > 0 {
		out.Tools = req.Tools
	}

	// system can be a string or content blocks
	if len(req.System) > 0 {
		var sys string
		if err := json.Unmarshal(req.System, &sys); err == nil && sys != "" {
			out.Messages = append(out.Messages, types.Message{Role: "system", Content: sys})
		} else if blocks := contentBlocks(req.System); len(blocks) > 0 {
			if text := blocksToText(blocks); text != "" {
				out.Messages = append(out.Messages, types.Message{Role: "system", Content: text})
			}
		}
	}

	for _, m := range req.Messages {
		blocks := contentBlocks(m.Content)
		if len(blocks) == 0 {
			out.Messages = append(out.Messages, types.Message{Role: m.Role, Content: ""})
			continue
		}
		// Text-only messages pass through as a string; messages with
		// tool_use/tool_result are preserved as structured content.
		textOnly := true
		for _, b := range blocks {
			if b.Type != "text" {
				textOnly = false
				break
			}
		}
		if textOnly {
			out.Messages = append(out.Messages, types.Message{Role: m.Role, Content: blocksToText(blocks)})
			continue
		}
		converted := make([]map[string]interface{}, 0, len(blocks))
		for _, b := range blocks {
			switch b.Type {
			case "text":
				converted = append(converted, map[string]interface{}{"type": "text", "text": b.Text})
			case "tool_use":
				converted = append(converted, map[string]interface{}{
					"type": "function", "id": b.ID, "name": b.Name,
					"arguments": json.RawMessage(b.Input),
				})
			case "tool_result":
				converted = append(converted, map[string]interface{}{
					"type": "tool_result", "tool_call_id": b.ToolUseID,
					"content": blocksToText(contentBlocks(b.Content)),
				})
			default:
				converted = append(converted, map[string]interface{}{"type": b.Type})
			}
		}
		out.Messages = append(out.Messages, types.Message{Role: m.Role, Content: converted})
	}
	return out
}

// canonicalToMessages converts an OpenAI-shaped chat completion reply into an
// Anthropic messages reply.
func canonicalToMessages(body []byte, requestedModel string) ([]byte, error) {
	var chat types.ChatCompletionResponse
	if err := json.Unmarshal(body, &chat); err != nil {
		return nil, err
	}

	resp := aMessagesResponse{
		ID:    chat.ID,
		Type:  "message",
		Role:  "assistant",
		Model: orString(chat.Model, requestedModel),
	}
	if len(chat.Choices) > 0 {
		choice := chat.Choices[0]
		if text, ok := choice.Message.Content.(string); ok && text != "" {
			resp.Content = append(resp.Content, aContentBlock{Type: "text", Text: text})
		}
		switch choice.FinishReason {
		case "stop", "":
			stop := "end_turn"
			resp.StopReason = &stop
		case "length":
			stop := "max_tokens"
			resp.StopReason = &stop
		case "tool_calls":
			stop := "tool_use"
			resp.StopReason = &stop
		default:
			stop := choice.FinishReason
			resp.StopReason = &stop
		}
	} else {
		stop := "end_turn"
		resp.StopReason = &stop
	}
	if len(resp.Content) == 0 {
		resp.Content = []aContentBlock{{Type: "text", Text: ""}}
	}
	resp.Usage = aUsage{
		InputTokens:              chat.Usage.PromptTokens,
		OutputTokens:             chat.Usage.CompletionTokens,
		CacheCreationInputTokens: chat.Usage.CacheWrite,
		CacheReadInputTokens:     chat.Usage.CacheRead,
	}

	return json.Marshal(resp)
}

// stopReasonFromFinish maps an OpenAI finish_reason chunk to Anthropic's.
func stopReasonFromFinish(finish string) string {
	switch finish {
	case "length":
		return "max_tokens"
	case "tool_calls":
		return "tool_use"
	case "":
		return ""
	default:
		return "end_turn"
	}
}

// HandleMessages 处理 Anthropic Messages 请求（流式与非流式统一入口）。
func (h *ProxyHandler) HandleMessages(c *gin.Context) {
	c.Set("request_start", time.Now())

	var req aMessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("Invalid request: %v", err))
		return
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 4096
	}

	chatReq := convertMessagesToCanonical(&req)

	opts := h.getOptionsFromContext(c)
	finishSelection := func() {}
	if isAutoModel(chatReq.Model) {
		if !h.autoModeEnabled() {
			writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", "auto mode is disabled")
			return
		}
		selection, done, err := h.selectAutoModel(c, chatReq)
		if err != nil {
			writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		opts = &selection.Options
		chatReq.Model = selection.Model
		finishSelection = done
	}
	defer finishSelection()

	if chatReq.Stream {
		if chatReq.StreamOptions == nil {
			chatReq.StreamOptions = &types.StreamOptions{}
		}
		chatReq.StreamOptions.IncludeUsage = true
	}

	chain := h.providerChain(opts)
	ctx := c.Request.Context()
	var resp *http.Response
	var lastErr error
	tries := 0

	for i, chainOpts := range chain {
		prov, err := h.ProviderFactory.Create(chainOpts.Provider, chainOpts)
		if err != nil {
			lastErr = err
			tries++
			continue
		}
		attempts := 1
		if chainOpts.Retry != nil && chainOpts.Retry.Attempts > 0 {
			attempts = chainOpts.Retry.Attempts
		}
		for attempt := 0; attempt < attempts; attempt++ {
			tries++
			var hopErr error
			var hopResp *http.Response
			hopResp, hopErr = prov.ChatCompletion(ctx, chatReq, chainOpts)
			if hopErr == nil && hopResp.StatusCode < 500 && hopResp.StatusCode != http.StatusTooManyRequests && hopResp.StatusCode != http.StatusUnauthorized && hopResp.StatusCode != http.StatusForbidden {
				resp, lastErr = hopResp, nil
				break
			}
			if hopResp != nil {
				hopResp.Body.Close()
			}
			lastErr = hopErr
			if lastErr == nil {
				lastErr = fmt.Errorf("provider returned status %d", hopRespStatus(hopResp))
			}
			if ctx.Err() != nil {
				break
			}
			time.Sleep(retryWait(attempt, chainOpts.Retry))
		}
		if lastErr == nil && resp != nil {
			break
		}
		if i+1 >= len(chain) {
			break
		}
	}

	c.Set(middleware.TriesKey, max(tries, 1))

	if lastErr != nil || resp == nil {
		status := http.StatusBadGateway
		message := "Request failed"
		if lastErr != nil {
			message = fmt.Sprintf("Request failed: %v", lastErr)
		}
		writeAnthropicError(c, status, "api_error", message)
		return
	}
	defer resp.Body.Close()

	if req.Stream {
		h.streamMessagesResponse(c, resp, chatReq)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeAnthropicError(c, http.StatusBadGateway, "api_error", fmt.Sprintf("Failed to read upstream response: %v", err))
		return
	}
	if resp.StatusCode != http.StatusOK {
		// Pass upstream errors through as Anthropic-shaped errors.
		writeAnthropicError(c, resp.StatusCode, "api_error", truncateMessage(string(body), 2000))
		return
	}

	out, err := canonicalToMessages(body, req.Model)
	if err != nil {
		writeAnthropicError(c, http.StatusBadGateway, "api_error", fmt.Sprintf("Failed to convert response: %v", err))
		return
	}
	c.Data(http.StatusOK, "application/json", out)
}

// streamMessagesResponse translates an upstream OpenAI-style SSE stream into
// Anthropic messages SSE events.
func (h *ProxyHandler) streamMessagesResponse(c *gin.Context, resp *http.Response, req *types.ChatCompletionRequest) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		writeAnthropicError(c, http.StatusInternalServerError, "api_error", "Streaming not supported")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(resp.StatusCode)

	messageID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	start := startOf(c)
	var ttftRecorded bool
	var usage types.Usage
	var sawUsage bool
	var contentBuf bytes.Buffer
	blockOpen := false

	send := func(event string, payload interface{}) {
		data, err := json.Marshal(payload)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	// message_start carries the input token count so far.
	send("message_start", gin.H{
		"type": "message_start",
		"message": gin.H{
			"id": messageID, "type": "message", "role": "assistant",
			"model":       req.Model,
			"content":     []interface{}{},
			"stop_reason": nil, "stop_sequence": nil,
			"usage": aUsage{InputTokens: 0},
		},
	})

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(scanSSEEvents)

	for scanner.Scan() {
		event := scanner.Bytes()
		for _, line := range bytes.Split(event, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			payload := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if bytes.Equal(payload, []byte("[DONE]")) {
				continue
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage *types.Usage `json:"usage"`
			}
			if err := json.Unmarshal(payload, &chunk); err != nil {
				continue
			}
			if u := chunk.Usage; u != nil {
				usage = *u
				sawUsage = true
			}
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					if !blockOpen {
						send("content_block_start", gin.H{
							"type":          "content_block_start",
							"index":         0,
							"content_block": gin.H{"type": "text", "text": ""},
						})
						blockOpen = true
					}
					if !ttftRecorded {
						metrics.TimeToFirstToken.WithLabelValues(
							metrics.LabelOrUnknown(""),
							metrics.LabelOrUnknown(req.Model),
						).Observe(time.Since(start).Seconds())
						ttftRecorded = true
					}
					contentBuf.WriteString(choice.Delta.Content)
					send("content_block_delta", gin.H{
						"type":  "content_block_delta",
						"index": 0,
						"delta": gin.H{"type": "text_delta", "text": choice.Delta.Content},
					})
				}
				if choice.FinishReason != "" {
					if blockOpen {
						send("content_block_stop", gin.H{"type": "content_block_stop", "index": 0})
						blockOpen = false
					}
					stop := stopReasonFromFinish(choice.FinishReason)
					send("message_delta", gin.H{
						"type":  "message_delta",
						"delta": gin.H{"stop_reason": stop, "stop_sequence": nil},
						"usage": gin.H{"output_tokens": usage.CompletionTokens},
					})
				}
			}
		}
	}

	if blockOpen {
		send("content_block_stop", gin.H{"type": "content_block_stop", "index": 0})
	}

	if !sawUsage {
		promptTokens := estimateTokens(promptTextFromRequest(req))
		completionTokens := estimateTokens(contentBuf.String())
		usage = types.Usage{PromptTokens: promptTokens, CompletionTokens: completionTokens, TotalTokens: promptTokens + completionTokens}
	}
	send("message_stop", gin.H{"type": "message_stop"})
}

func writeAnthropicError(c *gin.Context, status int, errType, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"type":  "error",
		"error": gin.H{"type": errType, "message": message},
	})
}

func truncateMessage(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// hopRespStatus extracts a status from a possibly-nil response.
func hopRespStatus(resp *http.Response) int {
	if resp != nil {
		return resp.StatusCode
	}
	return http.StatusBadGateway
}

func orString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// HandleAnthropicCountTokens serves POST /v1/messages/count_tokens with a
// local estimate — providers don't expose a portable count endpoint.
func (h *ProxyHandler) HandleAnthropicCountTokens(c *gin.Context) {
	var req aMessagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeAnthropicError(c, http.StatusBadRequest, "invalid_request_error", fmt.Sprintf("Invalid request: %v", err))
		return
	}
	total := 0
	if len(req.System) > 0 {
		total += estimateTokens(blocksToText(contentBlocks(req.System)))
	}
	for _, m := range req.Messages {
		total += estimateTokens(blocksToText(contentBlocks(m.Content)))
	}
	c.JSON(http.StatusOK, gin.H{"input_tokens": total})
}
