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

const selectTenantPurpose = "select_tenant"

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
		auth.POST("/select-tenant", h.SelectTenant)
	}
}

// LoginRequest is the request body for login
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// SelectTenantRequest is the request body for selecting a tenant after login.
type SelectTenantRequest struct {
	LoginToken string `json:"login_token" binding:"required"`
	TenantID   uint   `json:"tenant_id" binding:"required"`
}

// Login handles user login and returns either a final JWT token or a tenant-selection token.
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "Username and password are required",
			Type:    "invalid_request_error",
		})
		return
	}

	user, ok := h.authenticateUser(c, req.Username, req.Password)
	if !ok {
		return
	}

	if user.Role == models.RoleSuperAdmin {
		token, err := generateToken(user, nil, h.cfg.Security.JWTSecret)
		if err != nil {
			h.internalTokenError(c)
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token, "status": "success"})
		return
	}

	members, err := activeMemberships(user.ID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
			Message: "Internal error",
			Type:    "internal_error",
		})
		return
	}
	if len(members) == 0 {
		c.AbortWithStatusJSON(http.StatusForbidden, types.ErrorResponse{
			Message: "No active tenant membership",
			Type:    "authorization_error",
		})
		return
	}
	if len(members) == 1 {
		token, err := generateToken(user, &members[0], h.cfg.Security.JWTSecret)
		if err != nil {
			h.internalTokenError(c)
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token, "tenant": members[0], "status": "success"})
		return
	}

	loginToken, err := generateSelectTenantToken(user, h.cfg.Security.JWTSecret)
	if err != nil {
		h.internalTokenError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":      "tenant_selection_required",
		"login_token": loginToken,
		"tenants":     members,
	})
}

// SelectTenant exchanges a short-lived tenant-selection token for a tenant-scoped JWT.
func (h *AuthHandler) SelectTenant(c *gin.Context) {
	var req SelectTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, types.ErrorResponse{
			Message: "login_token and tenant_id are required",
			Type:    "invalid_request_error",
		})
		return
	}

	claims, err := parseSelectTenantToken(req.LoginToken, h.cfg.Security.JWTSecret)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid or expired login token",
			Type:    "authentication_error",
		})
		return
	}

	uid, ok := numericClaim(claims, "user_id")
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid login token",
			Type:    "authentication_error",
		})
		return
	}

	var user models.User
	if err := database.GetDB().Where("id = ? AND status = ?", uid, "active").First(&user).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid login token",
			Type:    "authentication_error",
		})
		return
	}

	membership, err := activeMembership(user.ID, req.TenantID)
	if err != nil {
		status := http.StatusInternalServerError
		message := "Internal error"
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusForbidden
			message = "Tenant is not available for this user"
		}
		c.AbortWithStatusJSON(status, types.ErrorResponse{Message: message, Type: "authorization_error"})
		return
	}

	token, err := generateToken(&user, membership, h.cfg.Security.JWTSecret)
	if err != nil {
		h.internalTokenError(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "tenant": membership, "status": "success"})
}

func (h *AuthHandler) authenticateUser(c *gin.Context, username, password string) (*models.User, bool) {
	var user models.User
	err := database.GetDB().Where("username = ?", username).Order("id ASC").First(&user).Error
	if err != nil || user.Status != "active" {
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
				Message: "Internal error",
				Type:    "internal_error",
			})
			return nil, false
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid username or password",
			Type:    "authentication_error",
		})
		return nil, false
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, types.ErrorResponse{
			Message: "Invalid username or password",
			Type:    "authentication_error",
		})
		return nil, false
	}
	return &user, true
}

func (h *AuthHandler) internalTokenError(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusInternalServerError, types.ErrorResponse{
		Message: "Failed to generate token",
		Type:    "internal_error",
	})
}

func activeMemberships(userID uint) ([]models.TenantMembership, error) {
	var memberships []models.TenantMembership
	err := database.GetDB().Table("tenant_members").
		Select("tenant_members.tenant_id, tenants.name, tenants.slug, tenant_members.role, tenant_members.status").
		Joins("JOIN tenants ON tenants.id = tenant_members.tenant_id").
		Where("tenant_members.user_id = ? AND tenant_members.status = ? AND tenants.status = ?", userID, "active", "active").
		Order("tenants.name ASC, tenants.id ASC").
		Scan(&memberships).Error
	return memberships, err
}

func activeMembership(userID, tenantID uint) (*models.TenantMembership, error) {
	var membership models.TenantMembership
	err := database.GetDB().Table("tenant_members").
		Select("tenant_members.tenant_id, tenants.name, tenants.slug, tenant_members.role, tenant_members.status").
		Joins("JOIN tenants ON tenants.id = tenant_members.tenant_id").
		Where("tenant_members.user_id = ? AND tenant_members.tenant_id = ? AND tenant_members.status = ? AND tenants.status = ?", userID, tenantID, "active", "active").
		First(&membership).Error
	return &membership, err
}

func generateToken(u *models.User, membership *models.TenantMembership, secret string) (string, error) {
	if secret == "" {
		secret = "llm-gateway-secret-change-in-production"
	}

	role := u.Role
	claims := jwt.MapClaims{
		"user_id":  u.ID,
		"username": u.Username,
		"role":     role,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	if membership != nil {
		role = membership.Role
		claims["role"] = role
		claims["tenant_id"] = membership.TenantID
	} else if u.TenantID != nil {
		claims["tenant_id"] = *u.TenantID
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func generateSelectTenantToken(u *models.User, secret string) (string, error) {
	if secret == "" {
		secret = "llm-gateway-secret-change-in-production"
	}
	claims := jwt.MapClaims{
		"purpose":  selectTenantPurpose,
		"user_id":  u.ID,
		"username": u.Username,
		"exp":      time.Now().Add(10 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func parseSelectTenantToken(tokenStr, secret string) (jwt.MapClaims, error) {
	if secret == "" {
		secret = "llm-gateway-secret-change-in-production"
	}
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, jwt.ErrTokenInvalidClaims
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["purpose"] != selectTenantPurpose {
		return nil, jwt.ErrTokenInvalidClaims
	}
	return claims, nil
}

func numericClaim(claims jwt.MapClaims, key string) (uint, bool) {
	value, ok := claims[key].(float64)
	if !ok {
		return 0, false
	}
	return uint(value), true
}
