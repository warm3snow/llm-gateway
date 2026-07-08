package middleware

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const modelInputPreviewMaxBytes = 4096

type modelInputPreview struct {
	Kind         string
	Preview      string
	Truncated    bool
	InputBytes   int
	PreviewBytes int
}

var sensitiveTextPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password\s*[:=]\s*)\S+`),
	regexp.MustCompile(`(?i)(token\s*[:=]\s*)\S+`),
	regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]?\s*)\S+`),
	regexp.MustCompile(`(?i)(secret\s*[:=]\s*)\S+`),
	regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)\S+`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9._-]+`),
}

func buildModelInputPreview(body []byte, maxBytes int) modelInputPreview {
	if maxBytes <= 0 {
		maxBytes = modelInputPreviewMaxBytes
	}

	var payload map[string]any
	if len(body) == 0 || json.Unmarshal(body, &payload) != nil {
		return modelInputPreview{Kind: "unknown"}
	}

	kind, extracted := extractKnownModelInput(payload)
	if extracted == "" {
		return modelInputPreview{Kind: kind}
	}

	extracted = redactSensitiveText(strings.TrimSpace(extracted))
	result := modelInputPreview{
		Kind:       kind,
		InputBytes: len([]byte(extracted)),
	}
	result.Preview, result.Truncated = truncateUTF8(extracted, maxBytes)
	result.PreviewBytes = len([]byte(result.Preview))
	return result
}

func extractKnownModelInput(payload map[string]any) (string, string) {
	if messages, ok := payload["messages"].([]any); ok {
		return "chat_messages", extractMessagesPreview(messages)
	}
	if prompt, ok := payload["prompt"]; ok {
		return "completion_prompt", valueToPreview(prompt)
	}
	if input, ok := payload["input"]; ok {
		return "embedding_input", valueToPreview(input)
	}
	if prompt, ok := payload["prompt"]; ok {
		return "image_prompt", valueToPreview(prompt)
	}
	return "unknown", ""
}

func extractMessagesPreview(messages []any) string {
	lines := make([]string, 0, len(messages))
	for _, item := range messages {
		message, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := message["role"].(string)
		if role == "" {
			role = "message"
		}
		content := valueToPreview(message["content"])
		if content == "" {
			continue
		}
		lines = append(lines, role+": "+content)
	}
	return strings.Join(lines, "\n")
}

func valueToPreview(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			part := valueToPreviewPart(item)
			if part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, " ")
	case float64, bool:
		return fmt.Sprint(v)
	default:
		return ""
	}
}

func valueToPreviewPart(value any) string {
	part, ok := value.(map[string]any)
	if !ok {
		return valueToPreview(value)
	}

	partType, _ := part["type"].(string)
	switch partType {
	case "text", "input_text":
		if text, ok := part["text"].(string); ok {
			return text
		}
	case "image_url", "input_image", "image":
		return "[image]"
	case "audio", "input_audio":
		return "[audio]"
	case "file", "input_file":
		return "[file]"
	}
	if text, ok := part["text"].(string); ok {
		return text
	}
	return "[non-text]"
}

func redactSensitiveText(text string) string {
	redacted := text
	for _, pattern := range sensitiveTextPatterns {
		redacted = pattern.ReplaceAllString(redacted, `${1}[redacted]`)
	}
	return redacted
}

func truncateUTF8(text string, maxBytes int) (string, bool) {
	if len([]byte(text)) <= maxBytes {
		return text, false
	}
	cut := text
	for len([]byte(cut)) > maxBytes {
		_, size := utf8.DecodeLastRuneInString(cut)
		if size == 0 {
			break
		}
		cut = cut[:len(cut)-size]
	}
	return cut, true
}
