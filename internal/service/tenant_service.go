package service

import (
	"errors"
	"fmt"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// TenantService handles tenant + tenant-user management. These operations are
// platform-level and intended to be called only by a super_admin.
type TenantService struct {
	db *gorm.DB
}

// NewTenantService creates a new TenantService.
func NewTenantService() *TenantService {
	return &TenantService{db: database.GetDB()}
}

// List returns all tenants.
func (s *TenantService) List() ([]models.Tenant, error) {
	var tenants []models.Tenant
	if err := s.db.Order("id ASC").Find(&tenants).Error; err != nil {
		return nil, err
	}
	return tenants, nil
}

// Create creates a new tenant.
func (s *TenantService) Create(req *models.TenantRequest) (*models.Tenant, error) {
	t := &models.Tenant{
		Name:   req.Name,
		Slug:   req.Slug,
		Status: "active",
	}
	if err := s.db.Create(t).Error; err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}
	return t, nil
}

// SetStatus enables/disables a tenant (status: active | disabled).
func (s *TenantService) SetStatus(id uint, status string) error {
	if status != "active" && status != "disabled" {
		return errors.New("invalid status")
	}
	res := s.db.Model(&models.Tenant{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("tenant not found")
	}
	return nil
}

// CreateUser creates a tenant_admin user bound to a tenant.
func (s *TenantService) CreateUser(req *models.UserRequest) (*models.User, error) {
	// Ensure the target tenant exists.
	var tenant models.Tenant
	if err := s.db.First(&tenant, req.TenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("tenant not found")
		}
		return nil, err
	}

	// Enforce username uniqueness within the tenant with a friendly error.
	var count int64
	if err := s.db.Model(&models.User{}).
		Where("tenant_id = ? AND username = ?", req.TenantID, req.Username).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("username %q already exists in this tenant", req.Username)
	}

	role := req.Role
	if role == "" {
		role = models.RoleTenantAdmin
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	tid := req.TenantID
	u := &models.User{
		TenantID:     &tid,
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         role,
		Status:       "active",
	}
	if err := s.db.Create(u).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return u, nil
}

// ListUsers returns users, optionally filtered by tenant (0 = all).
func (s *TenantService) ListUsers(tenantID uint) ([]models.User, error) {
	var users []models.User
	q := s.db.Order("id ASC")
	if tenantID != 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
