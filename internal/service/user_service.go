package service

import (
	"errors"
	"fmt"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserService handles admin-console user management with role and tenant scoping.
type UserService struct {
	db *gorm.DB
}

// NewUserService creates a new UserService.
func NewUserService() *UserService {
	return &UserService{db: database.GetDB()}
}

// List returns users visible to the caller. super_admin can see all tenants or
// a requested tenant; tenant_admin sees only its own tenant; tenant_user cannot
// manage users.
func (s *UserService) List(actorRole string, actorTenantID, requestedTenantID uint) ([]models.User, error) {
	var users []models.User
	q := s.db.Order("id ASC")
	switch actorRole {
	case models.RoleSuperAdmin:
		if requestedTenantID != 0 {
			q = q.Where("tenant_id = ?", requestedTenantID)
		}
	case models.RoleTenantAdmin:
		q = q.Where("tenant_id = ?", actorTenantID)
	default:
		return nil, errors.New("user management privileges required")
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// Create creates a tenant-bound user according to the caller's privileges.
func (s *UserService) Create(actorRole string, actorTenantID uint, req *models.UserRequest) (*models.User, error) {
	role := req.Role
	if role == "" {
		role = models.RoleTenantUser
	}

	tenantID := req.TenantID
	switch actorRole {
	case models.RoleSuperAdmin:
		if tenantID == 0 {
			return nil, errors.New("tenant_id is required")
		}
		if role != models.RoleTenantAdmin && role != models.RoleTenantUser {
			return nil, errors.New("invalid role")
		}
	case models.RoleTenantAdmin:
		tenantID = actorTenantID
		if role != models.RoleTenantUser {
			return nil, errors.New("tenant_admin can only create tenant_user")
		}
	default:
		return nil, errors.New("user management privileges required")
	}

	return s.createTenantUser(tenantID, req.Username, req.Password, role)
}

// SetStatus enables/disables users according to the caller's privileges.
func (s *UserService) SetStatus(actorRole string, actorTenantID uint, actorUsername string, id uint, status string) error {
	if status != "active" && status != "disabled" {
		return errors.New("invalid status")
	}

	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}
	if user.Username == actorUsername {
		return errors.New("cannot update your own status")
	}

	switch actorRole {
	case models.RoleSuperAdmin:
		if user.Role == models.RoleSuperAdmin {
			return errors.New("cannot update super_admin status")
		}
	case models.RoleTenantAdmin:
		if user.TenantID == nil || *user.TenantID != actorTenantID {
			return errors.New("user not found")
		}
		if user.Role != models.RoleTenantUser {
			return errors.New("tenant_admin can only update tenant_user status")
		}
	default:
		return errors.New("user management privileges required")
	}

	return s.db.Model(&models.User{}).Where("id = ?", id).Update("status", status).Error
}

func (s *UserService) createTenantUser(tenantID uint, username, password, role string) (*models.User, error) {
	var tenant models.Tenant
	if err := s.db.First(&tenant, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("tenant not found")
		}
		return nil, err
	}

	var count int64
	if err := s.db.Model(&models.User{}).
		Where("tenant_id = ? AND username = ?", tenantID, username).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("username %q already exists in this tenant", username)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &models.User{
		TenantID:     &tenantID,
		Username:     username,
		PasswordHash: string(hash),
		Role:         role,
		Status:       "active",
	}
	if err := s.db.Create(u).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return u, nil
}
