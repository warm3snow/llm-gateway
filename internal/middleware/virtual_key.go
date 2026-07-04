package middleware

import (
	"crypto/sha256"
	"encoding/hex"
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

		// Set context values for downstream handlers
		c.Set("virtual_key_id", matchedKey.ID)
		c.Set("virtual_key_name", matchedKey.Name)

		c.Next()
	}
}

// verifyKey verifies a plaintext key against a stored hash.
func verifyKey(key, keyHash, salt string) bool {
	h := sha256.Sum256([]byte(key + salt))
	return hex.EncodeToString(h[:]) == keyHash
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
