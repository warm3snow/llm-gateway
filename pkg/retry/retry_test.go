package retry

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// MockHTTPClient 模拟 HTTP 客户端
type MockHTTPClient struct {
	Responses []*http.Response
	Errors    []error
	CallCount int
}

// Do 模拟 HTTP 请求
func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.CallCount >= len(m.Responses) {
		return nil, fmt.Errorf("no more mock responses")
	}

	resp := m.Responses[m.CallCount]
	err := m.Errors[m.CallCount]
	m.CallCount++

	return resp, err
}

// TestRetryer_Do_Success 测试成功情况
func TestRetryer_Do_Success(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    3,
		BackoffMin:    10 * time.Millisecond,
		BackoffMax:    100 * time.Millisecond,
		StatusCodes:   []int{429, 500, 502, 503, 504},
		UseRetryAfter: false,
		Jitter:        false,
	}

	retryer := NewRetryer(config)

	// 模拟成功响应
	callCount := 0
	fn := func() (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: 200,
			Body:       http.NoBody,
		}, nil
	}

	ctx := context.Background()
	resp, err := retryer.Do(ctx, fn)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 1, callCount) // 应该只调用一次
}

// TestRetryer_Do_RetryOn500 测试在 500 错误时重试
func TestRetryer_Do_RetryOn500(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    3,
		BackoffMin:    10 * time.Millisecond,
		BackoffMax:    100 * time.Millisecond,
		StatusCodes:   []int{500, 502, 503, 504},
		UseRetryAfter: false,
		Jitter:        false,
	}

	retryer := NewRetryer(config)

	// 模拟：第一次 500，第二次成功
	callCount := 0
	fn := func() (*http.Response, error) {
		callCount++
		if callCount == 1 {
			return &http.Response{
				StatusCode: 500,
				Body:       http.NoBody,
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       http.NoBody,
		}, nil
	}

	ctx := context.Background()
	resp, err := retryer.Do(ctx, fn)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, callCount) // 应该调用两次
}

// TestRetryer_Do_MaxRetries 测试达到最大重试次数
func TestRetryer_Do_MaxRetries(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    2,
		BackoffMin:    10 * time.Millisecond,
		BackoffMax:    100 * time.Millisecond,
		StatusCodes:   []int{500},
		UseRetryAfter: false,
		Jitter:        false,
	}

	retryer := NewRetryer(config)

	// 模拟：一直返回 500
	callCount := 0
	fn := func() (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: 500,
			Body:       http.NoBody,
		}, nil
	}

	ctx := context.Background()
	resp, err := retryer.Do(ctx, fn)

	assert.NoError(t, err) // 不返回错误，但响应是最后一个
	assert.NotNil(t, resp)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Equal(t, 3, callCount) // 初始 + 2 次重试 = 3 次
}

// TestRetryer_Do_NoRetryOn400 测试 400 错误不重试
func TestRetryer_Do_NoRetryOn400(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    3,
		BackoffMin:    10 * time.Millisecond,
		BackoffMax:    100 * time.Millisecond,
		StatusCodes:   []int{500, 502, 503, 504},
		UseRetryAfter: false,
		Jitter:        false,
	}

	retryer := NewRetryer(config)

	// 模拟 400 错误
	callCount := 0
	fn := func() (*http.Response, error) {
		callCount++
		return &http.Response{
			StatusCode: 400,
			Body:       http.NoBody,
		}, nil
	}

	ctx := context.Background()
	resp, err := retryer.Do(ctx, fn)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, 1, callCount) // 应该只调用一次，不重试
}

// TestRetryer_Do_ContextCancel 测试上下文取消
func TestRetryer_Do_ContextCancel(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    5,
		BackoffMin:    1 * time.Second,
		BackoffMax:    5 * time.Second,
		StatusCodes:   []int{500},
		UseRetryAfter: false,
		Jitter:        false,
	}

	retryer := NewRetryer(config)

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())

	fn := func() (*http.Response, error) {
		// 取消上下文
		cancel()
		return &http.Response{
			StatusCode: 500,
			Body:       http.NoBody,
		}, nil
	}

	resp, err := retryer.Do(ctx, fn)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, context.Canceled, err)
}

// TestRetryer_Do_RetryAfterHeader 测试 Retry-After 头
func TestRetryer_Do_RetryAfterHeader(t *testing.T) {
	config := &RetryConfig{
		MaxRetries:    2,
		BackoffMin:    10 * time.Millisecond,
		BackoffMax:    100 * time.Millisecond,
		StatusCodes:   []int{429},
		UseRetryAfter: true,
		Jitter:        false,
	}

	retryer := NewRetryer(config)

	// 模拟：第一次 429 带 Retry-After，第二次成功
	callCount := 0
	fn := func() (*http.Response, error) {
		callCount++
		if callCount == 1 {
			resp := &http.Response{
				StatusCode: 429,
				Body:       http.NoBody,
				Header:     make(http.Header),
			}
			resp.Header.Set("Retry-After", "1") // 1 秒
			return resp, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       http.NoBody,
		}, nil
	}

	ctx := context.Background()
	resp, err := retryer.Do(ctx, fn)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, callCount)
}

// TestCalculateBackoff 测试退避时间计算
func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name      string
		attempt   int
		min       time.Duration
		max       time.Duration
		factor    float64
		expectMin time.Duration
		expectMax time.Duration
	}{
		{
			name:      "First attempt",
			attempt:   1,
			min:       100 * time.Millisecond,
			max:       30 * time.Second,
			factor:    2.0,
			expectMin: 100 * time.Millisecond,
			expectMax: 200 * time.Millisecond, // min * factor + jitter
		},
		{
			name:      "Second attempt",
			attempt:   2,
			min:       100 * time.Millisecond,
			max:       30 * time.Second,
			factor:    2.0,
			expectMin: 200 * time.Millisecond,
			expectMax: 400 * time.Millisecond,
		},
		{
			name:      "Exceeds max",
			attempt:   10,
			min:       100 * time.Millisecond,
			max:       5 * time.Second,
			factor:    2.0,
			expectMin: 5 * time.Second,
			expectMax: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoff := CalculateBackoff(tt.attempt, tt.min, tt.max, tt.factor)
			
			// 由于 jitter，我们只能检查范围
			assert.True(t, backoff >= tt.min, "backoff should be >= min")
			assert.True(t, backoff <= tt.max, "backoff should be <= max")
		})
	}
}

// TestDefaultRetryConfig 测试默认配置
func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 1*time.Second, config.BackoffMin)
	assert.Equal(t, 30*time.Second, config.BackoffMax)
	assert.Equal(t, 2.0, config.BackoffFactor)
	assert.Len(t, config.StatusCodes, 5)
	assert.Contains(t, config.StatusCodes, 429)
	assert.Contains(t, config.StatusCodes, 500)
	assert.Contains(t, config.StatusCodes, 502)
	assert.Contains(t, config.StatusCodes, 503)
	assert.Contains(t, config.StatusCodes, 504)
	assert.True(t, config.UseRetryAfter)
	assert.True(t, config.Jitter)
}

// TestGenerateTraceID 测试生成追踪 ID
func TestGenerateTraceID(t *testing.T) {
	id1 := GenerateTraceID()
	id2 := GenerateTraceID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)

	// UUID 格式检查（简单检查）
	assert.Len(t, id1, 36) // UUID 长度
	assert.Contains(t, id1, "-")
}

// TestRetryWithOptions 测试使用 Options 重试
func TestRetryWithOptions(t *testing.T) {
	// 测试不带重试配置
	t.Run("No retry options", func(t *testing.T) {
		callCount := 0
		fn := func() (*http.Response, error) {
			callCount++
			return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
		}

		ctx := context.Background()
		resp, err := RetryWithOptions(ctx, nil, fn)

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, 1, callCount)
	})

	// 测试带重试配置
	t.Run("With retry options", func(t *testing.T) {
		opts := &types.Options{
			Retry: &types.RetrySettings{
				Attempts:            2,
				OnStatusCodes:       []int{500},
				UseRetryAfterHeader: false,
			},
		}

		callCount := 0
		fn := func() (*http.Response, error) {
			callCount++
			if callCount == 1 {
				return &http.Response{StatusCode: 500, Body: http.NoBody}, nil
			}
			return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
		}

		ctx := context.Background()
		resp, err := RetryWithOptions(ctx, opts, fn)

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, 2, callCount)
	})
}

// BenchmarkRetryer 性能测试
func BenchmarkRetryer(b *testing.B) {
	config := DefaultRetryConfig()
	retryer := NewRetryer(config)

	fn := func() (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		retryer.Do(ctx, fn)
	}
}
