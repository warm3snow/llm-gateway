package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// JWTAuth returns a Gin middleware that validates JWT tokens
func JWTAuth(cfg *config.Config) gin.HandlerFunc {
	secret := cfg.Security.JWTSecret
	if secret == "" {
		secret = "llm-gateway-secret-change-in-production"
	}

	return func(c *gin.Context) {
		// Skip auth for the public login endpoint only.
		if c.Request.URL.Path == "/api/v1/auth/login" {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
				Message: "Missing authorization header",
				Type:    "authentication_error",
			})
			return
		}

		// Parse Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
				Message: "Invalid authorization header format",
				Type:    "authentication_error",
			})
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
				Message: "Invalid or expired token",
				Type:    "authentication_error",
			})
			return
		}

		// Store claims in context
		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if purpose, ok := claims["purpose"].(string); ok && purpose == "select_tenant" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
					Message: "Invalid token scope",
					Type:    "authentication_error",
				})
				return
			}
			if username, ok := claims["username"].(string); ok {
				c.Set("username", username)
			}
			if role, ok := claims["role"].(string); ok {
				c.Set("role", role)
			}
			if ok := setUintClaim(c, claims, "tenant_id"); !ok {
				return
			}
			if ok := setUintClaim(c, claims, "user_id"); !ok {
				return
			}
		}

		c.Next()
	}
}

func setUintClaim(c *gin.Context, claims jwt.MapClaims, name string) bool {
	value, exists := claims[name]
	if !exists {
		return true
	}
	number, ok := value.(float64)
	maxUint := ^uint(0)
	if !ok || number < 0 || number > float64(maxUint) || number != float64(uint(number)) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid " + name + " claim",
			Type:    "authentication_error",
		})
		return false
	}
	c.Set(name, uint(number))
	return true
}

// IsSuperAdmin reports whether the authenticated user is a platform super_admin.
func IsSuperAdmin(c *gin.Context) bool {
	role, _ := c.Get("role")
	r, _ := role.(string)
	return r == models.RoleSuperAdmin
}

// GetUserTenantID returns the tenant_id embedded in the JWT (0 if absent, e.g.
// for a super_admin who has no tenant binding).
func GetUserTenantID(c *gin.Context) uint {
	if v, exists := c.Get("tenant_id"); exists {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}

// EffectiveTenantID resolves which tenant a request should be scoped to.
//   - tenant_admin: always their own tenant (query overrides are ignored).
//   - super_admin: 0 (all tenants) unless they pass ?tenant_id= to impersonate
//     a specific tenant.
func EffectiveTenantID(c *gin.Context) uint {
	if !IsSuperAdmin(c) {
		return GetUserTenantID(c)
	}
	if q := c.Query("tenant_id"); q != "" {
		if id, err := strconv.ParseUint(q, 10, 64); err == nil {
			return uint(id)
		}
	}
	return 0
}
