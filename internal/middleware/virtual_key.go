package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
)

// VirtualKeyAuth validates virtual keys from request headers.
// The header name is read from cfg.Security.APIKeyHeader (default: x-llm-gateway-api-key).
// If a valid key is found, it sets "virtual_key_id" and "virtual_key_name" in the context.
func VirtualKeyAuth(cfg *config.Config) gin.HandlerFunc {
	headerName := "x-llm-gateway-api-key"
	if cfg != nil && cfg.Security.APIKeyHeader != "" {
		headerName = cfg.Security.APIKeyHeader
	}

	return func(c *gin.Context) {
		// Skip auth for health and admin UI static files
		path := c.Request.URL.Path
		if path == "/health" || strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/static") {
			c.Next()
			return
		}

		// Read virtual key from the configured header
		vk := c.GetHeader(headerName)

		if vk == "" {
			// No virtual key provided - reject
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":  "Missing virtual key. Provide it via the " + headerName + " header.",
				"status": "error",
			})
			return
		}

		// Validate the key against the database
		var keys []models.VirtualKey
		if err := database.GetDB().Where("status = ?", "active").Find(&keys).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":  "Internal server error",
				"status": "error",
			})
			return
		}

		var matchedKey *models.VirtualKey
		for i := range keys {
			if verifyKey(vk, keys[i].KeyHash, keys[i].KeySalt) {
				matchedKey = &keys[i]
				break
			}
		}

		if matchedKey == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":  "Invalid virtual key",
				"status": "error",
			})
			return
		}

		// Check budget
		if matchedKey.BudgetTotal > 0 && matchedKey.BudgetUsed >= matchedKey.BudgetTotal {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":  "Budget exceeded",
				"status": "error",
			})
			return
		}

		// Empty Providers means all providers are allowed. If configured, enforce
		// against an explicit x-llm-provider immediately. For auto-mode requests
		// without x-llm-provider, defer enforcement to the model selector so it can
		// choose from the virtual key's full provider allowlist instead of the
		// gateway default provider.
		requestedProvider := c.GetHeader("x-llm-provider")
		var requestBody []byte
		if requestedProvider == "" && isAutoModeChatRoute(c.Request.Method, c.Request.URL.Path) && c.Request.Body != nil {
			if c.Request.ContentLength >= 0 && c.Request.ContentLength <= 1024*1024 {
				requestBody, _ = io.ReadAll(c.Request.Body)
				c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			}
		}
		allowedProviders := providerListFromString(matchedKey.Providers)
		if len(allowedProviders) > 0 {
			c.Set("virtual_key_allowed_providers", allowedProviders)
		}
		if !shouldDeferProviderCheckForAuto(c.Request.Method, c.Request.URL.Path, requestBody, requestedProvider) {
			providerName := requestedProvider
			if providerName == "" && cfg != nil {
				providerName = cfg.Gateway.DefaultProvider
			}
			if !virtualKeyAllowsProvider(matchedKey.Providers, providerName) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":  "Provider is not allowed for this virtual key",
					"status": "error",
				})
				return
			}
		}

		// Set context values for downstream handlers
		c.Set("virtual_key_id", matchedKey.ID)
		c.Set("virtual_key_name", matchedKey.Name)
		c.Set("tenant_id", matchedKey.TenantID)

		c.Next()
	}
}

// verifyKey verifies a plaintext key against a stored hash.
func verifyKey(key, keyHash, salt string) bool {
	h := sha256.Sum256([]byte(key + salt))
	return hex.EncodeToString(h[:]) == keyHash
}

func virtualKeyAllowsProvider(allowedProviders, providerName string) bool {
	if strings.TrimSpace(allowedProviders) == "" {
		return true
	}
	for _, allowed := range providerListFromString(allowedProviders) {
		if allowed == providerName {
			return true
		}
	}
	return false
}

func providerListFromString(allowedProviders string) []string {
	if strings.TrimSpace(allowedProviders) == "" {
		return nil
	}
	parts := strings.Split(allowedProviders, ",")
	providers := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			providers = append(providers, part)
		}
	}
	return providers
}

func isAutoModeChatRoute(method, path string) bool {
	return method == http.MethodPost && (path == "/v1/chat/completions" || path == "/v1/chat/completions/stream")
}

func shouldDeferProviderCheckForAuto(method, path string, body []byte, requestedProvider string) bool {
	if requestedProvider != "" || !isAutoModeChatRoute(method, path) {
		return false
	}
	var payload struct {
		Model *string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	if payload.Model == nil {
		return true
	}
	model := strings.TrimSpace(strings.ToLower(*payload.Model))
	return model == "" || model == "auto"
}

// GetVirtualKeyID extracts the virtual key ID from the gin context.
func GetVirtualKeyID(c *gin.Context) uint {
	if id, exists := c.Get("virtual_key_id"); exists {
		if v, ok := id.(uint); ok {
			return v
		}
	}
	return 0
}
