package guardrail

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/types"
)

func TestPIIDetectorValidateRequest(t *testing.T) {
	detector := NewPIIDetector()

	tests := []struct {
		name       string
		request    any
		expectPass bool
	}{
		{name: "no pii", request: map[string]any{"message": "Hello, how are you?"}, expectPass: true},
		{name: "email", request: map[string]any{"message": "My email is test@example.com"}, expectPass: false},
		{name: "phone", request: map[string]any{"message": "Call me at 13800138000"}, expectPass: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.ValidateRequest(tt.request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectPass, result.Passed)
			if !tt.expectPass {
				assert.Contains(t, result.Actions, "deny")
				assert.NotEmpty(t, result.Reason)
			}
		})
	}
}

func TestPIIDetectorValidateResponse(t *testing.T) {
	detector := NewPIIDetector()

	result, err := detector.ValidateResponse(map[string]any{"content": "Hello there!"})
	require.NoError(t, err)
	assert.True(t, result.Passed)

	result, err = detector.ValidateResponse(map[string]any{"content": "My email is john@test.com"})
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "email")
}

func TestKeywordFilterValidateRequest(t *testing.T) {
	filter := NewKeywordFilter([]string{"violence", "hate", "illegal"}, "contains", false)

	tests := []struct {
		name       string
		request    any
		expectPass bool
	}{
		{name: "no blocked keywords", request: map[string]any{"messages": []any{map[string]any{"content": "Tell me about AI"}}}, expectPass: true},
		{name: "blocked keyword", request: map[string]any{"messages": []any{map[string]any{"content": "How to make violence?"}}}, expectPass: false},
		{name: "case insensitive", request: map[string]any{"messages": []any{map[string]any{"content": "I HATE this"}}}, expectPass: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := filter.ValidateRequest(tt.request)
			require.NoError(t, err)
			assert.Equal(t, tt.expectPass, result.Passed)
		})
	}
}

func TestKeywordFilterValidateResponse(t *testing.T) {
	filter := NewKeywordFilter([]string{"blocked"}, "contains", false)

	result, err := filter.ValidateResponse(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "safe response"}}}})
	require.NoError(t, err)
	assert.True(t, result.Passed)

	result, err = filter.ValidateResponse(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "blocked response"}}}})
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Actions, "deny")
}

func TestKeywordFilterMatchModes(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		filter := NewKeywordFilter([]string{"bad word"}, "exact", false)

		result, err := filter.ValidateRequest("bad word")
		require.NoError(t, err)
		assert.False(t, result.Passed)

		result, err = filter.ValidateRequest("this is a bad word")
		require.NoError(t, err)
		assert.True(t, result.Passed)
	})

	t.Run("regex match", func(t *testing.T) {
		filter := NewKeywordFilter([]string{`\d{11}`}, "regex", false)

		result, err := filter.ValidateRequest("My phone is 13800138000")
		require.NoError(t, err)
		assert.False(t, result.Passed)

		result, err = filter.ValidateRequest("Hello world")
		require.NoError(t, err)
		assert.True(t, result.Passed)
	})
}

func TestLengthLimiterValidateRequestAndResponse(t *testing.T) {
	limiter := NewLengthLimiter(20, 30)

	result, err := limiter.ValidateRequest("short")
	require.NoError(t, err)
	assert.True(t, result.Passed)

	result, err = limiter.ValidateRequest("this request is too long")
	require.NoError(t, err)
	assert.False(t, result.Passed)

	result, err = limiter.ValidateResponse("short response")
	require.NoError(t, err)
	assert.True(t, result.Passed)

	result, err = limiter.ValidateResponse("this response is definitely too long")
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestGuardrailManagerMultipleGuardrails(t *testing.T) {
	manager := NewGuardrailManager(true)
	manager.RegisterGuardrail(NewPIIDetector())
	manager.RegisterGuardrail(NewKeywordFilter([]string{"bad"}, "contains", false))

	result, err := manager.ValidateRequest(map[string]any{"messages": []any{map[string]any{"content": "Hello world"}}})
	require.NoError(t, err)
	assert.True(t, result.Passed)

	result, err = manager.ValidateRequest(map[string]any{"messages": []any{map[string]any{"content": "My email is test@example.com"}}})
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "email")

	result, err = manager.ValidateRequest(map[string]any{"messages": []any{map[string]any{"content": "This is bad content"}}})
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Contains(t, result.Reason, "bad")
}

func TestNewManagerFromConfigKeyword(t *testing.T) {
	manager, err := NewManagerFromConfig(true, []types.GuardrailConfig{
		{
			Type: "keyword",
			Parameters: map[string]any{
				"keywords":      []any{"blocked"},
				"matchMode":     "contains",
				"caseSensitive": false,
			},
		},
	})
	require.NoError(t, err)
	assert.True(t, manager.Enabled())
	assert.False(t, manager.Empty())

	result, err := manager.ValidateRequest(map[string]any{"messages": []any{map[string]any{"content": "blocked prompt"}}})
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestNewManagerFromConfigDisabled(t *testing.T) {
	manager, err := NewManagerFromConfig(false, []types.GuardrailConfig{
		{Type: "keyword", Parameters: map[string]any{"keywords": []any{"blocked"}}},
	})
	require.NoError(t, err)
	assert.False(t, manager.Enabled())

	result, err := manager.ValidateRequest("blocked prompt")
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestNewManagerFromConfigUnknownType(t *testing.T) {
	_, err := NewManagerFromConfig(true, []types.GuardrailConfig{{Type: "unknown"}})
	assert.Error(t, err)
}
