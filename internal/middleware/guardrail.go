package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	pathpkg "path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/types"
	"github.com/warm3snow/llm-gateway/pkg/guardrail"
)

const guardrailManagerKey = "guardrail_manager"

func GuardrailMiddleware(manager *guardrail.GuardrailManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		if manager != nil {
			c.Set(guardrailManagerKey, manager)
		}
		if manager == nil || !manager.Enabled() || manager.Empty() || !shouldValidateGuardrailRequest(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

		var body []byte
		if c.Request.Body != nil {
			var err error
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
					Message: "Failed to read request body",
					Type:    "invalid_request_error",
				})
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		}

		payload := guardrailPayload(body)
		result, err := manager.ValidateRequest(payload)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
				Message: "Guardrail validation failed",
				Type:    "guardrail_error",
				Code:    "guardrail_validation_failed",
			})
			return
		}
		if result != nil && !result.Passed {
			abortWithGuardrailBlocked(c, "Request blocked by guardrail")
			return
		}

		c.Next()
	}
}

func validateCachedResponseGuardrail(c *gin.Context, responseText string) bool {
	manager := guardrailManagerFromContext(c)
	if manager == nil || !manager.Enabled() || manager.Empty() || !shouldValidateGuardrailResponse(c.Request.Method, c.Request.URL.Path, http.StatusOK) {
		return false
	}
	result, err := manager.ValidateResponse(responseText)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Guardrail validation failed",
			Type:    "guardrail_error",
			Code:    "guardrail_validation_failed",
		})
		return true
	}
	if result != nil && !result.Passed {
		abortWithGuardrailBlocked(c, "Response blocked by guardrail")
		return true
	}
	return false
}

func guardrailManagerFromContext(c *gin.Context) *guardrail.GuardrailManager {
	value, ok := c.Get(guardrailManagerKey)
	if !ok {
		return nil
	}
	manager, _ := value.(*guardrail.GuardrailManager)
	return manager
}

func guardrailPayload(body []byte) any {
	if len(body) == 0 {
		return ""
	}
	var payload any
	if json.Unmarshal(body, &payload) == nil {
		return payload
	}
	return string(body)
}

func shouldValidateGuardrailRequest(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	switch guardrailEndpointPath(path) {
	case "/chat/completions", "/completions", "/embeddings", "/chat/completions/stream":
		return true
	default:
		return false
	}
}

func shouldValidateGuardrailResponse(method, path string, status int) bool {
	if method != http.MethodPost || status < 200 || status >= 300 {
		return false
	}
	switch guardrailEndpointPath(path) {
	case "/chat/completions", "/completions":
		return true
	default:
		return false
	}
}

func guardrailEndpointPath(path string) string {
	path = strings.TrimPrefix(path, "/v1")
	path = strings.TrimPrefix(path, "/proxy")
	path = "/" + strings.TrimLeft(path, "/")
	return pathpkg.Clean(path)
}

func abortWithGuardrailBlocked(c *gin.Context, message string) {
	c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
		Message: message,
		Type:    "guardrail_error",
		Code:    "guardrail_blocked",
	})
}
