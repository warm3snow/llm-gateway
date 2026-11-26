package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/metrics"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
	"gorm.io/gorm"
)

// VirtualKeyAuth validates virtual keys from the Authorization header.
// Virtual keys are sent as `Authorization: Bearer <virtual-key>`, matching the
// OpenAI API convention. If a valid key is found, it sets "virtual_key_id" and
// "virtual_key_name" in the context.
func VirtualKeyAuth(cfg *config.Config) gin.HandlerFunc {
	return VirtualKeyAuthWithBudgetTracker(cfg, nil)
}

func VirtualKeyAuthWithBudgetTracker(cfg *config.Config, budgetTracker *service.BudgetTracker) gin.HandlerFunc {
	authCache := newVirtualKeyAuthCache(5*time.Minute, 10*time.Second)

	return func(c *gin.Context) {
		// Skip auth for health and admin UI static files
		path := c.Request.URL.Path
		if path == "/health" || strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/static") {
			c.Next()
			return
		}

		start := time.Now()
		metricResult := "success"
		activeKeysLoaded := 0
		recorded := false
		recordMetric := func() {
			if recorded {
				return
			}
			recorded = true
			metrics.RecordVirtualKeyAuth(middlewareMetricEndpoint(c), metricResult, activeKeysLoaded, time.Since(start))
		}

		// Read the virtual key from the Authorization header. OpenAI-compatible
		// clients already send `Authorization: Bearer <key>`; the gateway
		// distinguishes virtual keys from admin JWTs by matching against the
		// virtual-key store.
		var vk string
		if auth := c.GetHeader("Authorization"); len(auth) > 7 && (auth[:7] == "Bearer " || auth[:7] == "bearer ") {
			vk = strings.TrimSpace(auth[7:])
		}

		if vk == "" {
			metricResult = "missing_key"
			// No virtual key provided - reject
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":  "Missing virtual key. Provide it via the Authorization: Bearer header.",
				"status": "error",
			})
			recordMetric()
			return
		}

		cacheDigest := virtualKeyCacheDigest(vk)
		if authCache.isNegative(cacheDigest) {
			metricResult = "invalid_key_cached"
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":  "Invalid virtual key",
				"status": "error",
			})
			recordMetric()
			return
		}

		var matchedKey *models.VirtualKey
		if cachedID, ok := authCache.getPositive(cacheDigest); ok {
			var cachedKey models.VirtualKey
			if err := database.GetDB().Where("id = ? AND status = ?", cachedID, "active").First(&cachedKey).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					authCache.evictPositive(cacheDigest)
					metricResult = "invalid_key_cached"
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
						"error":  "Invalid virtual key",
						"status": "error",
					})
				} else {
					metricResult = "lookup_error_cached"
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
						"error":  "Internal server error",
						"status": "error",
					})
				}
				recordMetric()
				return
			}
			matchedKey = &cachedKey
		} else {
			// Validate the key against the database. Cold lookups still scan because
			// stored key hashes are salted per row; hot keys use the local TTL cache.
			var keys []models.VirtualKey
			if err := database.GetDB().Where("status = ?", "active").Find(&keys).Error; err != nil {
				metricResult = "lookup_error"
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":  "Internal server error",
					"status": "error",
				})
				recordMetric()
				return
			}

			activeKeysLoaded = len(keys)
			for i := range keys {
				if verifyKey(vk, keys[i].KeyHash, keys[i].KeySalt) {
					matchedKey = &keys[i]
					break
				}
			}
			if matchedKey != nil {
				authCache.putPositive(cacheDigest, matchedKey.ID)
			}
		}

		if matchedKey == nil {
			authCache.putNegative(cacheDigest)
			metricResult = "invalid_key"
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":  "Invalid virtual key",
				"status": "error",
			})
			recordMetric()
			return
		}

		// Check budget, including cost accepted by the async budget tracker but not
		// flushed to the database yet.
		pendingBudget := 0.0
		if budgetTracker != nil {
			pendingBudget = budgetTracker.Pending(matchedKey.ID)
		}
		if matchedKey.BudgetTotal > 0 && matchedKey.BudgetUsed+pendingBudget >= matchedKey.BudgetTotal {
			authCache.evictPositive(cacheDigest)
			metricResult = "budget_exceeded"
			c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
				"error":  "Virtual key budget exceeded",
				"status": "error",
			})
			recordMetric()
			return
		}

		// Cascade: key → owning user → tenant. Monthly quotas are checked
		// against month-to-date spend (BudgetUsed) plus pending async cost.
		quotaSvc := newAuthQuotaService()
		owner, err := quotaSvc.ResolveUser(matchedKey)
		if err != nil {
			metricResult = "owner_error"
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":  err.Error(),
				"status": "error",
			})
			recordMetric()
			return
		}
		tenant, err := quotaSvc.ResolveTenant(matchedKey)
		if err != nil {
			metricResult = "tenant_error"
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":  err.Error(),
				"status": "error",
			})
			recordMetric()
			return
		}
		if tenant != nil && tenant.Status != "active" {
			metricResult = "tenant_disabled"
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":  fmt.Sprintf("Tenant %q is disabled", tenant.Name),
				"status": "error",
			})
			recordMetric()
			return
		}
		if userPending, ok := userPendingCost(matchedKey, owner, pendingBudget); ok {
			if err := quotaSvc.CheckUserQuota(owner, userPending); err != nil {
				metricResult = "user_quota_exceeded"
				c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
					"error":  "User monthly quota exceeded",
					"status": "error",
				})
				recordMetric()
				return
			}
		}
		if err := quotaSvc.CheckTenantQuota(tenant, tenantPendingCost(matchedKey, pendingBudget)); err != nil {
			if errors.Is(err, service.ErrTenantRPMExceeded) {
				metricResult = "tenant_rpm_limited"
				c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
					"error":  "Tenant rpm limit exceeded",
					"status": "error",
				})
			} else {
				metricResult = "tenant_quota_exceeded"
				c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
					"error":  "Tenant monthly budget exceeded",
					"status": "error",
				})
			}
			recordMetric()
			return
		}

		// Read the body once (chat routes) so both the model whitelist and
		// the deferred provider check can inspect it.
		requestedProvider := c.GetHeader("x-llm-provider")
		var requestBody []byte
		if isAutoModeChatRoute(c.Request.Method, c.Request.URL.Path) && c.Request.Body != nil {
			if c.Request.ContentLength >= 0 && c.Request.ContentLength <= 1024*1024 {
				requestBody, _ = io.ReadAll(c.Request.Body)
				c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			}
		}

		// Tenant model whitelist — deferred for auto-mode requests without an
		// explicit model, same as the provider check below.
		requestedModel := requestedModelFromBody(c, requestBody)
		if err := quotaSvc.CheckModelAllowed(tenant, requestedModel); err != nil {
			metricResult = "model_not_allowed"
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":  "Model is not allowed for this tenant",
				"status": "error",
			})
			recordMetric()
			return
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
				metricResult = "provider_forbidden"
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":  "Provider is not allowed for this virtual key",
					"status": "error",
				})
				recordMetric()
				return
			}
		}

		// Set context values for downstream handlers
		c.Set("virtual_key_id", matchedKey.ID)
		c.Set("virtual_key_name", matchedKey.Name)
		c.Set("virtual_key_user_id", matchedKey.UserID)
		c.Set("virtual_key_created_by_user_id", matchedKey.CreatedByUserID)
		c.Set("virtual_key_created_by_username", matchedKey.CreatedByUsername)
		c.Set("tenant_id", matchedKey.TenantID)

		recordMetric()
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
