package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries    int
	BackoffMin    time.Duration
	BackoffMax    time.Duration
	BackoffFactor float64
	StatusCodes   []int
	UseRetryAfter bool
	Jitter        bool
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		BackoffMin:    1 * time.Second,
		BackoffMax:    30 * time.Second,
		BackoffFactor: 2.0,
		StatusCodes:   []int{429, 500, 502, 503, 504},
		UseRetryAfter: true,
		Jitter:        true,
	}
}

// Retryer 重试器
type Retryer struct {
	Config *RetryConfig
}

// NewRetryer 创建重试器
func NewRetryer(config *RetryConfig) *Retryer {
	if config == nil {
		config = DefaultRetryConfig()
	}
	return &Retryer{Config: config}
}

// Do 执行带重试的请求
func (r *Retryer) Do(ctx context.Context, fn func() (*http.Response, error)) (*http.Response, error) {
	var resp *http.Response
	var err error

	attempt := 0
	backoff := r.Config.BackoffMin

	for attempt <= r.Config.MaxRetries {
		attempt++

		// 执行请求
		resp, err = fn()

		// 如果没有错误且状态码不在重试列表中，直接返回
		if err == nil && !shouldRetry(resp.StatusCode, r.Config.StatusCodes) {
			return resp, nil
		}

		// 如果达到最大重试次数，返回结果
		if attempt > r.Config.MaxRetries {
			break
		}

		// 计算等待时间
		waitTime := r.calculateWaitTime(resp, backoff, attempt)

		// 等待
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitTime):
			// 继续重试
		}

		// 指数退避
		backoff = time.Duration(float64(backoff) * r.Config.BackoffFactor)
		if backoff > r.Config.BackoffMax {
			backoff = r.Config.BackoffMax
		}
	}

	// 返回最后一次的结果
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// shouldRetry 判断是否需要重试
func shouldRetry(statusCode int, retryCodes []int) bool {
	for _, code := range retryCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// calculateWaitTime 计算等待时间
func (r *Retryer) calculateWaitTime(resp *http.Response, backoff time.Duration, attempt int) time.Duration {
	waitTime := backoff

	// 如果响应头中有 Retry-After，使用它
	if r.Config.UseRetryAfter && resp != nil {
		if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				waitTime = time.Duration(seconds) * time.Second
			}
		}

		if retryAfterMS := resp.Header.Get("Retry-After-Ms"); retryAfterMS != "" {
			if milliseconds, err := strconv.Atoi(retryAfterMS); err == nil {
				waitTime = time.Duration(milliseconds) * time.Millisecond
			}
		}
	}

	// 添加抖动
	if r.Config.Jitter {
		jitter := time.Duration(rand.Int63n(int64(waitTime / 2)))
		waitTime = waitTime/2 + jitter
	}

	return waitTime
}

// RetryWithOptions 使用 Options 配置进行重试
func RetryWithOptions(ctx context.Context, opts *types.Options, fn func() (*http.Response, error)) (*http.Response, error) {
	if opts == nil || opts.Retry == nil {
		return fn()
	}

	config := &RetryConfig{
		MaxRetries:    opts.Retry.Attempts,
		BackoffMin:    1 * time.Second,
		BackoffMax:    60 * time.Second,
		BackoffFactor: 2.0,
		StatusCodes:   opts.Retry.OnStatusCodes,
		UseRetryAfter: opts.Retry.UseRetryAfterHeader,
		Jitter:        true,
	}

	retryer := NewRetryer(config)
	return retryer.Do(ctx, fn)
}

// GenerateTraceID 生成追踪 ID
func GenerateTraceID() string {
	return uuid.New().String()
}

// CalculateBackoff 计算退避时间
func CalculateBackoff(attempt int, min, max time.Duration, factor float64) time.Duration {
	backoff := time.Duration(float64(min) * math.Pow(factor, float64(attempt-1)))

	// 添加 20% 的抖动
	jitter := time.Duration(rand.Float64() * 0.2 * float64(backoff))
	backoff = backoff + jitter

	if backoff > max {
		backoff = max
	}

	if backoff < min {
		backoff = min
	}

	return backoff
}

// RetryPolicy 重试策略
type RetryPolicy struct {
	ShouldRetry func(*http.Response, error) bool
	OnRetry     func(attempt int, resp *http.Response, err error)
}

// DefaultRetryPolicy 默认重试策略
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		ShouldRetry: func(resp *http.Response, err error) bool {
			if err != nil {
				return true
			}
			return resp.StatusCode == 429 || resp.StatusCode >= 500
		},
		OnRetry: func(attempt int, resp *http.Response, err error) {
			fmt.Printf("Retry attempt %d\n", attempt)
		},
	}
}

// DoWithPolicy 使用策略进行重试
func DoWithPolicy(ctx context.Context, policy *RetryPolicy, fn func() (*http.Response, error)) (*http.Response, error) {
	if policy == nil {
		policy = DefaultRetryPolicy()
	}

	resp, err := fn()
	if err == nil && !policy.ShouldRetry(resp, err) {
		return resp, nil
	}

	// 重试逻辑
	for attempt := 1; attempt <= 3; attempt++ {
		if policy.OnRetry != nil {
			policy.OnRetry(attempt, resp, err)
		}

		// 等待
		waitTime := CalculateBackoff(attempt, time.Second, 30*time.Second, 2.0)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(waitTime):
		}

		resp, err = fn()
		if err == nil && !policy.ShouldRetry(resp, err) {
			return resp, nil
		}
	}

	return resp, err
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
