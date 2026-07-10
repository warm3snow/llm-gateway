package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
)

const idempotencyKeyHeader = "Idempotency-Key"

// IdempotencyMiddleware replays completed responses for repeated POST requests
// with the same virtual-key scoped Idempotency-Key. Replayed requests abort
// before UsageRecordMiddleware, so they do not create duplicate usage or budget
// charges.
func IdempotencyMiddleware(idempotencyService *service.IdempotencyService, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimSpace(c.GetHeader(idempotencyKeyHeader))
		if key == "" || idempotencyService == nil || !isIdempotencySupportedRequest(c) {
			c.Next()
			return
		}

		virtualKeyID, ok := idempotencyVirtualKeyID(c)
		if !ok {
			c.Next()
			return
		}

		requestBody, err := c.GetRawData()
		if err != nil {
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(requestBody))
		requestHash := service.HashIdempotencyRequest(requestBody)

		begin, err := idempotencyService.Begin(c.Request.Context(), virtualKeyID, key, requestHash, ttl)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrIdempotencyConflict):
				c.AbortWithStatusJSON(http.StatusConflict, types.ErrorResponse{Message: "Idempotency-Key was reused with a different request body", Type: "idempotency_key_conflict"})
			case errors.Is(err, service.ErrIdempotencyInProgress):
				c.AbortWithStatusJSON(http.StatusConflict, types.ErrorResponse{Message: "Idempotency-Key request is still processing", Type: "idempotency_key_in_progress"})
			default:
				c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{Message: "Failed to process Idempotency-Key", Type: "idempotency_error"})
			}
			return
		}
		if begin == nil || begin.Record == nil {
			c.Next()
			return
		}
		if begin.Replay {
			contentType := begin.Record.ResponseContentType
			if contentType == "" {
				contentType = "application/json"
			}
			status := begin.Record.ResponseStatus
			if status == 0 {
				status = http.StatusOK
			}
			c.Header("Idempotency-Replayed", "true")
			c.Data(status, contentType, begin.Record.ResponseBody)
			c.Abort()
			return
		}

		writer := &responseBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}, status: http.StatusOK}
		c.Writer = writer
		c.Next()

		contentType := c.Writer.Header().Get("Content-Type")
		_ = idempotencyService.Complete(c.Request.Context(), begin.Record.ID, writer.status, contentType, writer.body.Bytes())
	}
}

func isIdempotencySupportedRequest(c *gin.Context) bool {
	if c.Request.Method != http.MethodPost {
		return false
	}
	path := c.Request.URL.Path
	return !strings.Contains(path, "/stream")
}

func idempotencyVirtualKeyID(c *gin.Context) (uint, bool) {
	id, exists := c.Get("virtual_key_id")
	if !exists {
		return 0, false
	}
	switch v := id.(type) {
	case uint:
		return v, v != 0
	case int:
		return uint(v), v > 0
	case uint64:
		return uint(v), v > 0
	default:
		return 0, false
	}
}
