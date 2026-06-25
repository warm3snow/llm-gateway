package guardrail

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// TestPIIDetector_ValidateRequest 测试 PII 检测器请求验证
func TestPIIDetector_ValidateRequest(t *testing.T) {
	detector := NewPIDetector()

	tests := []struct {
		name       string
		request    interface{}
		expectPass bool
		expectPII  []string
	}{
		{
			name:       "No PII",
			request:    map[string]interface{}{"message": "Hello, how are you?"},
			expectPass: true,
		},
		{
			name:       "Contains Email",
			request:    map[string]interface{}{"message": "My email is test@example.com"},
			expectPass: false,
			expectPII:  []string{"email"},
		},
		{
			name:       "Contains Phone",
			request:    map[string]interface{}{"message": "Call me at 13800138000"},
			expectPass: false,
			expectPII:  []string{"phone"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.ValidateRequest(tt.request)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectPass, result.Passed)

			if !tt.expectPass {
				assert.NotEmpty(t, result.Reason)
				assert.NotEmpty(t, result.Actions)
			}
		})
	}
}

// TestPIIDetector_ValidateResponse 测试 PII 检测器响应验证
func TestPIIDetector_ValidateResponse(t *testing.T) {
	detector := NewPIDetector()

	tests := []struct {
		name       string
		response   interface{}
		expectPass bool
	}{
		{
			name:       "No PII in response",
			response:   map[string]interface{}{"content": "Hello there!"},
			expectPass: true,
		},
		{
			name:       "PII in response",
			response:   map[string]interface{}{"content": "My email is john@test.com"},
			expectPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detector.ValidateResponse(tt.response)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectPass, result.Passed)
		})
	}
}

// TestKeywordFilter_ValidateRequest 测试关键词过滤器
func TestKeywordFilter_ValidateRequest(t *testing.T) {
	filter := NewKeywordFilter(
		[]string{"violence", "hate", "illegal"},
		"contains",
		false,
	)

	tests := []struct {
		name       string
		request    interface{}
		expectPass bool
	}{
		{
			name:       "No blocked keywords",
			request:    map[string]interface{}{"messages": []interface{}{map[string]interface{}{"content": "Tell me about AI"}}},
			expectPass: true,
		},
		{
			name:       "Contains blocked keyword",
			request:    map[string]interface{}{"messages": []interface{}{map[string]interface{}{"content": "How to make violence?"}}},
			expectPass: false,
		},
		{
			name:       "Case insensitive",
			request:    map[string]interface{}{"messages": []interface{}{map[string]interface{}{"content": "I HATE this"}}},
			expectPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := filter.ValidateRequest(tt.request)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectPass, result.Passed)
		})
	}
}

// TestKeywordFilter_MatchModes 测试不同的匹配模式
func TestKeywordFilter_MatchModes(t *testing.T) {
	// Test exact match
	t.Run("Exact match", func(t *testing.T) {
		filter := NewKeywordFilter([]string{"bad word"}, "exact", false)

		result, _ := filter.ValidateRequest("bad word")
		assert.False(t, result.Passed)

		result, _ = filter.ValidateRequest("this is a bad word")
		assert.True(t, result.Passed)
	})

	// Test regex match
	t.Run("Regex match", func(t *testing.T) {
		filter := NewKeywordFilter([]string{`\d{11}`}, "regex", false)

		result, _ := filter.ValidateRequest("My phone is 13800138000")
		assert.False(t, result.Passed)

		result, _ = filter.ValidateRequest("Hello world")
		assert.True(t, result.Passed)
	})
}

// TestLengthLimiter_ValidateRequest 测试长度限制器
func TestLengthLimiter_ValidateRequest(t *testing.T) {
	limiter := NewLengthLimiter(100, 500)

	tests := []struct {
		name       string
		request    interface{}
		expectPass bool
	}{
		{
			name:       "Within limit",
			request:    map[string]interface{}{"message": "Short message"},
			expectPass: true,
		},
		{
			name:       "Exceeds limit",
			request:    map[string]interface{}{"message": string(make([]byte, 200))},
			expectPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := limiter.ValidateRequest(tt.request)

			assert.NoError(t, err)
			assert.Equal(t, tt.expectPass, result.Passed)
		})
	}
}

// TestGuardrailManager_MultipleGuardrails 测试多个护栏的组合
func TestGuardrailManager_MultipleGuardrails(t *testing.T) {
	manager := NewGuardrailManager()

	// 添加 PII 检测器
	manager.RegisterGuardrail(NewPIDetector())

	// 添加关键词过滤器
	manager.RegisterGuardrail(NewKeywordFilter([]string{"bad"}, "contains", false))

	// 测试：通过所有护栏
	t.Run("Pass all guardrails", func(t *testing.T) {
		req := map[string]interface{}{"messages": []interface{}{map[string]interface{}{"content": "Hello world"}}}
		result, err := manager.ValidateRequest(req)

		assert.NoError(t, err)
		assert.True(t, result.Passed)
	})

	// 测试：被 PII 检测器阻止
	t.Run("Blocked by PII detector", func(t *testing.T) {
		req := map[string]interface{}{"messages": []interface{}{map[string]interface{}{"content": "My email is test@example.com"}}}
		result, err := manager.ValidateRequest(req)

		assert.NoError(t, err)
		assert.False(t, result.Passed)
		assert.Contains(t, result.Reason, "PII")
	})

	// 测试：被关键词过滤器阻止
	t.Run("Blocked by keyword filter", func(t *testing.T) {
		req := map[string]interface{}{"messages": []interface{}{map[string]interface{}{"content": "This is bad content"}}}
		result, err := manager.ValidateRequest(req)

		assert.NoError(t, err)
		assert.False(t, result.Passed)
		assert.Contains(t, result.Reason, "keywords")
	})
}

// TestGuardrailResult 测试护栏结果
func TestGuardrailResult(t *testing.T) {
	result := &types.GuardrailResult{
		Passed:  false,
		Message: "Blocked content",
		Reason:  "Contains PII",
		Actions: []string{"mask", "deny"},
	}

	assert.False(t, result.Passed)
	assert.Equal(t, "Blocked content", result.Message)
	assert.Equal(t, "Contains PII", result.Reason)
	assert.Len(t, result.Actions, 2)
	assert.Contains(t, result.Actions, "mask")
	assert.Contains(t, result.Actions, "deny")
}

// BenchmarkPIIDetector 性能测试
func BenchmarkPIIDetector(b *testing.B) {
	detector := NewPIDetector()
	request := map[string]interface{}{
		"message": "My email is test@example.com and phone is 13800138000",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.ValidateRequest(request)
	}
}

// BenchmarkKeywordFilter 性能测试
func BenchmarkKeywordFilter(b *testing.B) {
	filter := NewKeywordFilter([]string{"violence", "hate", "illegal"}, "contains", false)
	request := map[string]interface{}{
		"message": "This is a test message with bad content",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter.ValidateRequest(request)
	}
}
