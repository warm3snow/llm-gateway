package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/service"
)

// RedactMiddleware masks sensitive values in /v1/* request bodies before the
// proxy forwards them upstream, and puts the placeholders back in the
// response. It is a no-op when gateway.redact.enabled is false or the route
// is not a model-invocation endpoint.
//
// The response is restored through a wrapping writer (per SSE event for
// streams, wholesale for buffered bodies), so the restore never depends on
// how the handler wrote its output. The count of masked values is recorded
// on the usage record via RedactionsKey.
func RedactMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		opts, enabled := service.RedactOptions()
		if !enabled || !shouldRedact(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

		engine := service.RedactEngine()

		// Mask the request body before handlers parse it.
		if c.Request.Body != nil && c.Request.ContentLength != 0 {
			body, err := io.ReadAll(c.Request.Body)
			if err == nil && len(body) > 0 {
				masked, redactions, blocked := engine.MaskRequest(string(body), opts)
				if blocked {
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"error":  "Request blocked by masking policy",
						"status": "error",
					})
					return
				}
				c.Set(RedactionsKey, redactions)
				c.Request.Body = io.NopCloser(bytes.NewBufferString(masked))
				c.Request.ContentLength = int64(len(masked))
				c.Header("x-redactions", strconv.Itoa(redactions))
			}
		}

		// Restore placeholders in whatever the handlers write.
		c.Writer = &redactResponseWriter{ResponseWriter: c.Writer, engine: engine}

		c.Next()
	}
}

// shouldRedact limits masking to model-invocation endpoints.
func shouldRedact(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	switch path {
	case "/v1/chat/completions", "/v1/completions", "/v1/messages":
		return true
	}
	return false
}

// redactResponseWriter restores placeholders in everything written through
// it. Values arrive as {{KIND_tag}}; the restore pass swaps them back.
type redactResponseWriter struct {
	gin.ResponseWriter
	engine interface {
		Restore(string) (string, int)
	}
}

func (w *redactResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *redactResponseWriter) Write(b []byte) (int, error) {
	if !service.RedactEngine().HasPlaceholder(string(b)) {
		return w.ResponseWriter.Write(b)
	}
	restored, _ := w.engine.Restore(string(b))
	return w.ResponseWriter.Write([]byte(restored))
}
