package service

import (
	"errors"
	"fmt"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
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

// CreateUser creates or reuses a global user and attaches it to a tenant.
func (s *TenantService) CreateUser(req *models.UserRequest) (*models.User, error) {
	role := req.Role
	if role == "" {
		role = models.RoleTenantAdmin
	}
	if role != models.RoleTenantAdmin && role != models.RoleTenantUser {
		return nil, errors.New("invalid role")
	}
	return createTenantMembershipUser(s.db, req.TenantID, req.Username, req.Password, role)
}

// ListUsers returns users, optionally filtered by tenant (0 = all).
func (s *TenantService) ListUsers(tenantID uint) ([]models.User, error) {
	return listTenantUsers(s.db, tenantID)
}
