package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/types"
)

// JWTAuth returns a Gin middleware that validates JWT tokens
func JWTAuth(cfg *config.Config) gin.HandlerFunc {
	secret := cfg.Security.JWTSecret
	if secret == "" {
		secret = "llm-gateway-secret-change-in-production"
	}

	return func(c *gin.Context) {
		// Skip auth for login endpoint
		if strings.HasSuffix(c.Request.URL.Path, "/auth/login") {
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
			if username, ok := claims["username"].(string); ok {
				c.Set("username", username)
			}
		}

		c.Next()
	}
}
