package api

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/handler"
	"github.com/warm3snow/llm-gateway/internal/middleware"
)

func newAPIRouter(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS(cfg.Security.AllowedOrigins))

	handler.NewAuthHandler(cfg).RegisterRoutes(router)
	jwtMiddleware := middleware.JWTAuth(cfg)
	handler.NewHandler(cfg).RegisterRoutesWithAuth(router, jwtMiddleware)
	handler.NewVirtualKeyHandler().RegisterRoutesWithAuth(router, jwtMiddleware)
	return router
}
