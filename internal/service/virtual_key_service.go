package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"gorm.io/gorm"
)

// VirtualKeyService handles virtual key operations
type VirtualKeyService struct {
	db *gorm.DB
}

const DefaultVirtualKeyRateLimit = 60

// NewVirtualKeyService creates a new VirtualKeyService
func NewVirtualKeyService() *VirtualKeyService {
	return &VirtualKeyService{
		db: database.GetDB(),
	}
}

// Create creates a new virtual key
// Returns the full key (shown only once) and the DB record
func (s *VirtualKeyService) Create(tenantID uint, req *models.VirtualKeyRequest, createdByUserID *uint, createdByUsername string) (string, *models.VirtualKey, error) {
	// Generate a random 32-byte key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate key: %w", err)
	}
	fullKey := "vsk-" + hex.EncodeToString(keyBytes) // vsk- prefix + 64-char hex

	// Generate salt
	saltBytes := make([]byte, 16)
	rand.Read(saltBytes)
	salt := hex.EncodeToString(saltBytes)

	// Hash the key for storage (hash the full key including prefix)
	keyHash := hashKey(fullKey, salt)

	rateLimit := DefaultVirtualKeyRateLimit
	if req.RateLimit != nil {
		rateLimit = *req.RateLimit
	}

	// Create DB record
	vk := &models.VirtualKey{
		TenantID:          tenantID,
		CreatedByUserID:   createdByUserID,
		CreatedByUsername: createdByUsername,
		Name:              req.Name,
		KeyHash:           keyHash,
		KeySalt:           salt,
		HashedKey:         fullKey[:12], // Store first 12 chars (vsk-xxxxxxxx) for identification
		BudgetTotal:       req.BudgetTotal,
		BudgetUsed:        0,
		RateLimit:         rateLimit,
		RateLimitWindow:   req.RateLimitWindow,
		Status:            "active",
	}

	if vk.RateLimitWindow == 0 {
		vk.RateLimitWindow = 60
	}

	if req.Providers != nil && len(req.Providers) > 0 {
		// Store as comma-separated string for simplicity
		// In production, use JSON
		vk.Providers = joinStrings(req.Providers, ",")
	}

	if req.Metadata != "" {
		vk.Metadata = req.Metadata
	}

	if err := s.db.Create(vk).Error; err != nil {
		return "", nil, fmt.Errorf("failed to create virtual key: %w", err)
	}

	return fullKey, vk, nil
}

// GetByID retrieves a virtual key by ID, scoped to tenantID (0 = any tenant).
func (s *VirtualKeyService) GetByID(tenantID, id uint) (*models.VirtualKey, error) {
	var vk models.VirtualKey
	if err := s.db.Scopes(database.TenantScope(tenantID)).First(&vk, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("virtual key not found")
		}
		return nil, err
	}
	return &vk, nil
}

// GetByKeyHash retrieves a virtual key by its hash
func (s *VirtualKeyService) GetByKeyHash(keyHash string) (*models.VirtualKey, error) {
	var vk models.VirtualKey
	if err := s.db.Where("key_hash = ?", keyHash).First(&vk).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid virtual key")
		}
		return nil, err
	}
	return &vk, nil
}

// ValidateKey validates a virtual key string and returns the associated record
func (s *VirtualKeyService) ValidateKey(key string) (*models.VirtualKey, error) {
	// Find all active keys and verify
	var keys []models.VirtualKey
	if err := s.db.Where("status = ?", "active").Find(&keys).Error; err != nil {
		return nil, err
	}

	for _, vk := range keys {
		if VerifyKey(key, vk.KeyHash, vk.KeySalt) {
			// Check budget
			if vk.BudgetTotal > 0 && vk.BudgetUsed >= vk.BudgetTotal {
				return nil, errors.New("budget exceeded")
			}
			return &vk, nil
		}
	}

	return nil, errors.New("invalid virtual key")
}

// List returns all virtual keys for a tenant (tenantID 0 = all tenants).
func (s *VirtualKeyService) List(tenantID uint) ([]models.VirtualKey, error) {
	var keys []models.VirtualKey
	if err := s.db.Scopes(database.TenantScope(tenantID)).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// ListByCreator returns virtual keys created by a user within a tenant.
func (s *VirtualKeyService) ListByCreator(tenantID, createdByUserID uint) ([]models.VirtualKey, error) {
	var keys []models.VirtualKey
	if err := s.db.Scopes(database.TenantScope(tenantID)).Where("created_by_user_id = ?", createdByUserID).Order("created_at DESC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// GetByIDAndCreator retrieves a virtual key by ID and creator, scoped to tenantID.
func (s *VirtualKeyService) GetByIDAndCreator(tenantID, id, createdByUserID uint) (*models.VirtualKey, error) {
	var vk models.VirtualKey
	if err := s.db.Scopes(database.TenantScope(tenantID)).Where("created_by_user_id = ?", createdByUserID).First(&vk, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("virtual key not found")
		}
		return nil, err
	}
	return &vk, nil
}

// Update updates a virtual key, scoped to tenantID (0 = any tenant).
func (s *VirtualKeyService) Update(tenantID, id uint, req *models.VirtualKeyRequest) (*models.VirtualKey, error) {
	var vk models.VirtualKey
	if err := s.db.Scopes(database.TenantScope(tenantID)).First(&vk, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("virtual key not found")
		}
		return nil, err
	}

	if req.Name != "" {
		vk.Name = req.Name
	}
	if req.BudgetTotal >= 0 {
		vk.BudgetTotal = req.BudgetTotal
	}
	if req.RateLimit != nil {
		vk.RateLimit = *req.RateLimit
	}
	if req.RateLimitWindow > 0 {
		vk.RateLimitWindow = req.RateLimitWindow
	}
	if req.Metadata != "" {
		vk.Metadata = req.Metadata
	}
	if req.Providers != nil {
		vk.Providers = joinStrings(req.Providers, ",")
	}

	if err := s.db.Save(&vk).Error; err != nil {
		return nil, fmt.Errorf("failed to update virtual key: %w", err)
	}

	return &vk, nil
}

// Delete soft-deletes a virtual key, scoped to tenantID (0 = any tenant).
func (s *VirtualKeyService) Delete(tenantID, id uint) error {
	res := s.db.Scopes(database.TenantScope(tenantID)).Delete(&models.VirtualKey{}, id)
	if res.Error != nil {
		return fmt.Errorf("failed to delete virtual key: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return errors.New("virtual key not found")
	}
	return nil
}

// TrackUsage updates budget usage for a virtual key
func (s *VirtualKeyService) TrackUsage(id uint, cost float64) error {
	if cost <= 0 {
		return nil
	}
	if err := s.db.Model(&models.VirtualKey{}).Where("id = ?", id).
		UpdateColumn("budget_used", gorm.Expr("budget_used + ?", cost)).Error; err != nil {
		return fmt.Errorf("failed to track usage: %w", err)
	}
	if err := NewAlertService(s.db).CheckVirtualKey(id); err != nil {
		return fmt.Errorf("failed to check budget alerts: %w", err)
	}
	return nil
}

// ToResponse converts a VirtualKey to VirtualKeyResponse
func (s *VirtualKeyService) ToResponse(vk *models.VirtualKey, fullKey string) *models.VirtualKeyResponse {
	resp := &models.VirtualKeyResponse{
		ID:                vk.ID,
		TenantID:          vk.TenantID,
		CreatedByUserID:   vk.CreatedByUserID,
		CreatedByUsername: vk.CreatedByUsername,
		Name:              vk.Name,
		KeyHashPrefix:     vk.HashedKey + "...",
		BudgetTotal:       vk.BudgetTotal,
		BudgetUsed:        vk.BudgetUsed,
		BudgetRemaining:   vk.BudgetTotal - vk.BudgetUsed,
		BudgetResetAt:     vk.BudgetResetAt,
		RateLimit:         vk.RateLimit,
		RateLimitWindow:   vk.RateLimitWindow,
		Providers:         vk.Providers,
		Status:            vk.Status,
		CreatedAt:         vk.CreatedAt,
		UpdatedAt:         vk.UpdatedAt,
	}

	if fullKey != "" {
		resp.Key = fullKey
	}

	return resp
}

// hashKey creates a hash of the key with salt
func hashKey(key, salt string) string {
	// In production, use bcrypt or argon2
	// For now, use simple SHA-256
	h := sha256.Sum256([]byte(key + salt))
	return hex.EncodeToString(h[:])
}

// VerifyKey verifies a key against a hash
func VerifyKey(key, keyHash, salt string) bool {
	h := sha256.Sum256([]byte(key + salt))
	return hex.EncodeToString(h[:]) == keyHash
}

// joinStrings joins a string slice with a separator
func joinStrings(strs []string, sep string) string {
	return strings.Join(strs, sep)
}

// splitStrings splits a string by separator
func splitStrings(s, sep string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, sep)
}
