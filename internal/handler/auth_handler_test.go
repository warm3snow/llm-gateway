package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func setupAuthTestRouter(t *testing.T) (*gin.Engine, *config.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := filepath.Join(t.TempDir(), "auth.db")
	if err := database.Connect(&database.Config{Driver: "sqlite", DSN: dsn, LogLevel: "silent"}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := database.Migrate(&models.Tenant{}, &models.User{}, &models.TenantMember{}, &models.ProviderConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
		database.DB = nil
	})

	cfg := &config.Config{}
	cfg.Security.JWTSecret = "test-secret"
	router := gin.New()
	NewAuthHandler(cfg).RegisterRoutes(router)
	return router, cfg
}

func createAuthTestUser(t *testing.T, username, password, status string) models.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	user := models.User{Username: username, Email: username + "-" + password + "@test.llmgw", PasswordHash: string(hash), Role: models.RoleTenantUser, Status: status}
	if err := database.GetDB().Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func createAuthTestTenant(t *testing.T, id uint, name string) models.Tenant {
	t.Helper()
	tenant := models.Tenant{ID: id, Name: name, Slug: name, Status: "active"}
	if err := database.GetDB().Create(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	return tenant
}

func createAuthTestMember(t *testing.T, userID, tenantID uint, role, status string) {
	t.Helper()
	member := models.TenantMember{UserID: userID, TenantID: tenantID, Role: role, Status: status}
	if err := database.GetDB().Create(&member).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
}

func postJSON(t *testing.T, router *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func parseAuthTestClaims(t *testing.T, tokenString, secret string) jwt.MapClaims {
	t.Helper()
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("parse token: %v", err)
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		t.Fatal("expected map claims")
	}
	return claims
}

func TestLoginSingleTenantReturnsFinalToken(t *testing.T) {
	router, cfg := setupAuthTestRouter(t)
	createAuthTestTenant(t, 1, "tenant-one")
	user := createAuthTestUser(t, "user1", "secret", "active")
	createAuthTestMember(t, user.ID, 1, models.RoleTenantAdmin, "active")

	w := postJSON(t, router, "/api/v1/auth/login", gin.H{"username": "user1", "password": "secret"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Status  string                    `json:"status"`
		Token   string                    `json:"token"`
		Tenant  models.TenantMembership   `json:"tenant"`
		Tenants []models.TenantMembership `json:"tenants"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Status != "success" || res.Token == "" {
		t.Fatalf("expected final token success, got %+v", res)
	}
	if res.Tenant.TenantID != 1 || len(res.Tenants) != 0 {
		t.Fatalf("expected single tenant response, got %+v", res)
	}
	claims := parseAuthTestClaims(t, res.Token, cfg.Security.JWTSecret)
	if claims["tenant_id"].(float64) != 1 || claims["role"] != models.RoleTenantAdmin || claims["purpose"] != nil {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestLoginSameUsernameDifferentPasswordsSelectsMatchedUserTenant(t *testing.T) {
	router, cfg := setupAuthTestRouter(t)
	createAuthTestTenant(t, 1, "tenant-one")
	createAuthTestTenant(t, 2, "tenant-two")
	first := createAuthTestUser(t, "user1", "secret-one", "active")
	second := createAuthTestUser(t, "user1", "secret-two", "active")
	createAuthTestMember(t, first.ID, 1, models.RoleTenantAdmin, "active")
	createAuthTestMember(t, second.ID, 2, models.RoleTenantUser, "active")

	w := postJSON(t, router, "/api/v1/auth/login", gin.H{"username": "user1", "password": "secret-two"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res struct {
		Status  string                    `json:"status"`
		Token   string                    `json:"token"`
		Tenant  models.TenantMembership   `json:"tenant"`
		Tenants []models.TenantMembership `json:"tenants"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if res.Status != "success" || res.Tenant.TenantID != 2 || len(res.Tenants) != 0 {
		t.Fatalf("expected only matched user's tenant, got %+v", res)
	}
	claims := parseAuthTestClaims(t, res.Token, cfg.Security.JWTSecret)
	if claims["user_id"].(float64) != float64(second.ID) || claims["tenant_id"].(float64) != 2 {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestLoginMultipleTenantsRequiresSelection(t *testing.T) {
	router, cfg := setupAuthTestRouter(t)
	createAuthTestTenant(t, 1, "tenant-one")
	createAuthTestTenant(t, 2, "tenant-two")
	user := createAuthTestUser(t, "user1", "secret", "active")
	createAuthTestMember(t, user.ID, 1, models.RoleTenantAdmin, "active")
	createAuthTestMember(t, user.ID, 2, models.RoleTenantUser, "active")

	w := postJSON(t, router, "/api/v1/auth/login", gin.H{"username": "user1", "password": "secret"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var loginRes struct {
		Status     string                    `json:"status"`
		LoginToken string                    `json:"login_token"`
		Tenants    []models.TenantMembership `json:"tenants"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginRes); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if loginRes.Status != "tenant_selection_required" || loginRes.LoginToken == "" || len(loginRes.Tenants) != 2 {
		t.Fatalf("expected tenant selection response, got %+v", loginRes)
	}
	loginClaims := parseAuthTestClaims(t, loginRes.LoginToken, cfg.Security.JWTSecret)
	if loginClaims["purpose"] != "select_tenant" || loginClaims["tenant_id"] != nil {
		t.Fatalf("unexpected login token claims: %+v", loginClaims)
	}

	w = postJSON(t, router, "/api/v1/auth/select-tenant", gin.H{"login_token": loginRes.LoginToken, "tenant_id": 2})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var selectRes struct {
		Status string                  `json:"status"`
		Token  string                  `json:"token"`
		Tenant models.TenantMembership `json:"tenant"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &selectRes); err != nil {
		t.Fatalf("decode select: %v", err)
	}
	if selectRes.Status != "success" || selectRes.Token == "" || selectRes.Tenant.TenantID != 2 {
		t.Fatalf("expected final selected tenant token, got %+v", selectRes)
	}
	claims := parseAuthTestClaims(t, selectRes.Token, cfg.Security.JWTSecret)
	if claims["tenant_id"].(float64) != 2 || claims["role"] != models.RoleTenantUser || claims["purpose"] != nil {
		t.Fatalf("unexpected final claims: %+v", claims)
	}
}
