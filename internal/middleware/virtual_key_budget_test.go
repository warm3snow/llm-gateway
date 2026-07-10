package middleware

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
)

func setupVirtualKeyBudgetTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "virtual_key_budget.db")
	if err := database.Connect(&database.Config{Driver: "sqlite", DSN: dsn, LogLevel: "silent"}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
		database.DB = nil
	})
	if err := database.Migrate(&models.Tenant{}, &models.VirtualKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func TestVirtualKeyAuthIncludesPendingBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	setupVirtualKeyBudgetTestDB(t)

	virtualKeyService := service.NewVirtualKeyService()
	plainKey, vk, err := virtualKeyService.Create(1, &models.VirtualKeyRequest{Name: "limited", BudgetTotal: 1}, nil, "")
	if err != nil {
		t.Fatalf("create virtual key: %v", err)
	}
	tracker := service.NewBudgetTracker(database.GetDB(), service.BudgetTrackerOptions{QueueSize: 10, BatchSize: 10})
	if err := tracker.Enqueue(vk.ID, 1); err != nil {
		t.Fatalf("enqueue pending budget: %v", err)
	}

	router := gin.New()
	router.Use(VirtualKeyAuthWithBudgetTracker(&config.Config{Security: config.SecurityConfig{APIKeyHeader: "x-llm-gateway-api-key"}}, tracker))
	router.GET("/v1/test", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	req.Header.Set("x-llm-gateway-api-key", plainKey)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusForbidden, w.Body.String())
	}
}
