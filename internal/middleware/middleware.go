package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 日志中间件：输出结构化访问日志，并为每个请求确保存在 trace_id，
// 供下游中间件/处理器复用（写入 gin.Context 的 "trace_id" 和响应头
// X-Request-ID）。
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Establish a trace ID as early as possible so every log line and the
		// persisted UsageRecord can be correlated.
		traceID := c.GetHeader("X-Request-ID")
		if traceID == "" {
			traceID = generateRequestID()
		}
		c.Set("trace_id", traceID)
		c.Writer.Header().Set("X-Request-ID", traceID)

		c.Next()

		if query != "" {
			path = path + "?" + query
		}
		status := c.Writer.Status()

		attrs := []slog.Attr{
			slog.String("trace_id", traceID),
			slog.Int("status", status),
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("client_ip", c.ClientIP()),
			slog.Duration("latency", time.Since(start)),
		}
		if errMsg := c.Errors.ByType(gin.ErrorTypePrivate).String(); errMsg != "" {
			attrs = append(attrs, slog.String("error", errMsg))
		}

		lvl := slog.LevelInfo
		if status >= 500 {
			lvl = slog.LevelError
		} else if status >= 400 {
			lvl = slog.LevelWarn
		}
		slog.LogAttrs(c.Request.Context(), lvl, "request", attrs...)
	}
}

// Recovery 恢复中间件
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				traceID, _ := c.Get("trace_id")
				slog.Error("panic recovered",
					slog.Any("panic", err),
					slog.Any("trace_id", traceID),
					slog.String("path", c.Request.URL.Path),
				)

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":  "Internal server error",
					"status": "error",
				})
			}
		}()

		c.Next()
	}
}

// CORS 跨域中间件
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// 检查是否允许该来源
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-llm-*, x-portkey-*")
			c.Writer.Header().Set("Access-Control-Expose-Headers", "x-llm-*, x-portkey-*")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Auth 认证中间件
func Auth(apiKeyHeader string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过认证的路径
		skipPaths := []string{
			"/health",
			"/public/",
		}

		path := c.Request.URL.Path
		for _, skipPath := range skipPaths {
			if len(path) >= len(skipPath) && path[:len(skipPath)] == skipPath {
				c.Next()
				return
			}
		}

		// 检查 API Key
		apiKey := c.GetHeader(apiKeyHeader)
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":  "Missing API key",
				"status": "error",
			})
			return
		}

		// TODO: 验证 API Key 是否有效
		// 这里应该查询数据库或缓存来验证

		c.Next()
	}
}

// RateLimit 限流中间件
func RateLimit(maxRequests int) gin.HandlerFunc {
	// 简单的内存限流（生产环境应该使用 Redis）
	requests := make(map[string][]time.Time)

	return func(c *gin.Context) {
		if maxRequests <= 0 {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		now := time.Now()

		// 清理旧的请求记录（保留最近 1 分钟）
		if times, exists := requests[clientIP]; exists {
			var validTimes []time.Time
			for _, t := range times {
				if now.Sub(t) < time.Minute {
					validTimes = append(validTimes, t)
				}
			}
			requests[clientIP] = validTimes
		}

		// 检查是否超过限制
		if times, exists := requests[clientIP]; exists && len(times) >= maxRequests {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":  "Rate limit exceeded",
				"status": "error",
			})
			return
		}

		// 记录请求
		requests[clientIP] = append(requests[clientIP], now)

		c.Next()
	}
}

// RequestID 请求 ID 中间件
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		c.Set("RequestID", requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)

		c.Next()
	}
}

// 生成请求 ID
func generateRequestID() string {
	return time.Now().Format("20060102-150405") + "-" + randomString(8)
}

// 生成随机字符串
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

// Timeout 超时中间件
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置上下文超时
		ctx := c.Request.Context()
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
