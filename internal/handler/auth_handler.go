package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/internal/types"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	cfg *config.Config
}

// NewAuthHandler creates a new AuthHandler
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

// RegisterRoutes registers auth routes
func (h *AuthHandler) RegisterRoutes(router *gin.Engine) {
	auth := router.Group("/api/v1/auth")
	{
		auth.POST("/login", h.Login)
	}
}

// LoginRequest is the request body for login
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login handles user login and returns a JWT token
// POST /api/v1/auth/login
// @Summary User login
// @Description Authenticate user and return JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} map[string]interface{} "Returns JWT token"
// @Failure 400 {object} types.ErrorResponse "Invalid request"
// @Failure 401 {object} types.ErrorResponse "Invalid credentials"
// @Failure 500 {object} types.ErrorResponse "Internal error"
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Username and password are required",
			Type:    "invalid_request_error",
		})
		return
	}

	// Look up the user in the database and verify the bcrypt password hash.
	// The config admin is seeded as a super_admin during bootstrap.
	var user models.User
	err := database.GetDB().Where("username = ?", req.Username).First(&user).Error
	if err != nil || user.Status != "active" {
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
				Message: "Internal error",
				Type:    "internal_error",
			})
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid username or password",
			Type:    "authentication_error",
		})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid username or password",
			Type:    "authentication_error",
		})
		return
	}

	// Generate JWT token
	token, err := generateToken(&user, h.cfg.Security.JWTSecret)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Failed to generate token",
			Type:    "internal_error",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":  token,
		"status": "success",
	})
}

// generateToken generates a JWT token for the given user, embedding the role
// and tenant scope so downstream middleware can enforce data isolation.
func generateToken(u *models.User, secret string) (string, error) {
	if secret == "" {
		secret = "llm-gateway-secret-change-in-production"
	}

	claims := jwt.MapClaims{
		"username": u.Username,
		"role":     u.Role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	// super_admin has no tenant binding (TenantID == nil).
	if u.TenantID != nil {
		claims["tenant_id"] = *u.TenantID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
