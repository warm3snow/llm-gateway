package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

func setupIdempotencyRouter(t *testing.T) (*gin.Engine, *int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.SetupSQLiteDB(t)
	require.NoError(t, database.Migrate(&models.IdempotencyRecord{}))

	calls := 0
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("virtual_key_id", uint(7))
		c.Next()
	})
	router.Use(IdempotencyMiddleware(service.NewIdempotencyService(db), time.Hour))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		calls++
		c.JSON(http.StatusOK, gin.H{"call": calls, "ok": true})
	})
	return router, &calls
}

func TestIdempotencyMiddlewareReplaysCompletedResponse(t *testing.T) {
	router, calls := setupIdempotencyRouter(t)

	body := `{"model":"gpt-test","messages":[{"role":"user","content":"hi"}]}`
	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	firstReq.Header.Set("Idempotency-Key", "idem-1")
	router.ServeHTTP(first, firstReq)

	require.Equal(t, http.StatusOK, first.Code)
	require.Equal(t, 1, *calls)
	require.Empty(t, first.Header().Get("Idempotency-Replayed"))

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	secondReq.Header.Set("Idempotency-Key", "idem-1")
	router.ServeHTTP(second, secondReq)

	require.Equal(t, http.StatusOK, second.Code)
	require.Equal(t, 1, *calls)
	require.Equal(t, "true", second.Header().Get("Idempotency-Replayed"))
	require.JSONEq(t, first.Body.String(), second.Body.String())
}

func TestIdempotencyMiddlewareRejectsSameKeyDifferentBody(t *testing.T) {
	router, calls := setupIdempotencyRouter(t)

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"a"}`))
	firstReq.Header.Set("Idempotency-Key", "idem-conflict")
	router.ServeHTTP(first, firstReq)
	require.Equal(t, http.StatusOK, first.Code)

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"b"}`))
	secondReq.Header.Set("Idempotency-Key", "idem-conflict")
	router.ServeHTTP(second, secondReq)

	require.Equal(t, http.StatusConflict, second.Code)
	require.Equal(t, 1, *calls)
	require.Contains(t, second.Body.String(), "idempotency_key_conflict")
}

func TestIdempotencyMiddlewareRejectsInProgressRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupSQLiteDB(t)
	require.NoError(t, database.Migrate(&models.IdempotencyRecord{}))
	require.NoError(t, db.Create(&models.IdempotencyRecord{
		VirtualKeyID:   uint(7),
		IdempotencyKey: "idem-busy",
		RequestHash:    service.HashIdempotencyRequest([]byte(`{"model":"a"}`)),
		Status:         models.IdempotencyStatusProcessing,
		ExpiresAt:      time.Now().Add(time.Hour),
	}).Error)

	calls := 0
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("virtual_key_id", uint(7))
		c.Next()
	})
	router.Use(IdempotencyMiddleware(service.NewIdempotencyService(db), time.Hour))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		calls++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"a"}`))
	req.Header.Set("Idempotency-Key", "idem-busy")
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	require.Equal(t, 0, calls)
	require.Contains(t, w.Body.String(), "idempotency_key_in_progress")
}
