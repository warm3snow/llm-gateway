package testutil

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/warm3snow/llm-gateway/internal/config"
)

type TokenClaims struct {
	UserID   uint
	Username string
	Role     string
	TenantID uint
}

func SignJWT(t *testing.T, cfg *config.Config, claims TokenClaims) string {
	t.Helper()
	mapClaims := jwt.MapClaims{
		"user_id":  claims.UserID,
		"username": claims.Username,
		"role":     claims.Role,
		"exp":      time.Now().Add(time.Hour).Unix(),
	}
	if claims.TenantID != 0 {
		mapClaims["tenant_id"] = claims.TenantID
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, mapClaims)
	signed, err := token.SignedString([]byte(cfg.Security.JWTSecret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return signed
}

func Authorize(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}
