package middleware

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestBuildModelInputPreview(t *testing.T) {
	t.Run("chat messages include role labels", func(t *testing.T) {
		preview := buildModelInputPreview([]byte(`{"messages":[{"role":"system","content":"be terse"},{"role":"user","content":"hello"}]}`), 4096)

		assert.Equal(t, "chat_messages", preview.Kind)
		assert.Contains(t, preview.Preview, "system: be terse")
		assert.Contains(t, preview.Preview, "user: hello")
		assert.False(t, preview.Truncated)
		assert.Greater(t, preview.InputBytes, 0)
		assert.Equal(t, len([]byte(preview.Preview)), preview.PreviewBytes)
	})

	t.Run("chat content parts preserve text and mark non-text", func(t *testing.T) {
		preview := buildModelInputPreview([]byte(`{"messages":[{"role":"user","content":[{"type":"text","text":"describe this"},{"type":"image_url","image_url":{"url":"data:image/png;base64,abc"}}]}]}`), 4096)

		assert.Equal(t, "chat_messages", preview.Kind)
		assert.Contains(t, preview.Preview, "user: describe this [image]")
		assert.NotContains(t, preview.Preview, "data:image/png")
	})

	t.Run("completion prompt is extracted", func(t *testing.T) {
		preview := buildModelInputPreview([]byte(`{"prompt":["first","second"]}`), 4096)

		assert.Equal(t, "completion_prompt", preview.Kind)
		assert.Contains(t, preview.Preview, "first")
		assert.Contains(t, preview.Preview, "second")
	})

	t.Run("embedding input is extracted", func(t *testing.T) {
		preview := buildModelInputPreview([]byte(`{"input":["alpha","beta"]}`), 4096)

		assert.Equal(t, "embedding_input", preview.Kind)
		assert.Contains(t, preview.Preview, "alpha")
		assert.Contains(t, preview.Preview, "beta")
	})

	t.Run("sensitive values are redacted", func(t *testing.T) {
		preview := buildModelInputPreview([]byte(`{"messages":[{"role":"user","content":"password: hunter2 token=abc123 api_key sk-secret"}]}`), 4096)

		assert.Contains(t, preview.Preview, "[redacted]")
		assert.NotContains(t, preview.Preview, "hunter2")
		assert.NotContains(t, preview.Preview, "abc123")
		assert.NotContains(t, preview.Preview, "sk-secret")
	})

	t.Run("long input truncates on valid text boundary", func(t *testing.T) {
		text := strings.Repeat("你好", 100)
		preview := buildModelInputPreview([]byte(`{"messages":[{"role":"user","content":"`+text+`"}]}`), 64)

		assert.Equal(t, "chat_messages", preview.Kind)
		assert.True(t, preview.Truncated)
		assert.LessOrEqual(t, preview.PreviewBytes, 64)
		assert.True(t, strings.HasPrefix(preview.Preview, "user: "))
		assert.True(t, utf8.ValidString(preview.Preview))
	})

	t.Run("invalid json does not store raw payload", func(t *testing.T) {
		preview := buildModelInputPreview([]byte(`{"messages":`), 4096)

		assert.Equal(t, "unknown", preview.Kind)
		assert.Empty(t, preview.Preview)
		assert.False(t, preview.Truncated)
	})
}
