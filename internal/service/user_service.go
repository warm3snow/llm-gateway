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
	switch actorRole {
	case models.RoleSuperAdmin:
		return listTenantUsers(s.db, requestedTenantID)
	case models.RoleTenantAdmin:
		return listTenantUsers(s.db, actorTenantID)
	default:
		return nil, errors.New("user management privileges required")
	}
}

// Create creates or reuses a global user and attaches it to a tenant according
// to the caller's privileges.
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

	return createTenantMembershipUser(s.db, tenantID, req.Username, req.Password, role)
}

// SetStatus enables/disables tenant memberships according to the caller's privileges.
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
		res := s.db.Model(&models.TenantMember{}).Where("user_id = ?", id).Update("status", status)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("user not found")
		}
		return nil
	case models.RoleTenantAdmin:
		var member models.TenantMember
		if err := s.db.Where("user_id = ? AND tenant_id = ?", id, actorTenantID).First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("user not found")
			}
			return err
		}
		if member.Role != models.RoleTenantUser {
			return errors.New("tenant_admin can only update tenant_user status")
		}
		return s.db.Model(&models.TenantMember{}).Where("id = ?", member.ID).Update("status", status).Error
	default:
		return errors.New("user management privileges required")
	}
}

func listTenantUsers(db *gorm.DB, tenantID uint) ([]models.User, error) {
	var users []models.User
	q := db.Table("tenant_members").
		Select("users.id, tenant_members.tenant_id, users.username, users.password_hash, tenant_members.role, tenant_members.status, users.created_at, users.updated_at").
		Joins("JOIN users ON users.id = tenant_members.user_id").
		Where("users.status = ?", "active").
		Order("tenant_members.tenant_id ASC, users.id ASC")
	if tenantID != 0 {
		q = q.Where("tenant_members.tenant_id = ?", tenantID)
	}
	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func createTenantMembershipUser(db *gorm.DB, tenantID uint, username, password, role string) (*models.User, error) {
	var tenant models.Tenant
	if err := db.First(&tenant, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("tenant not found")
		}
		return nil, err
	}

	var user models.User
	err := db.Where("username = ?", username).Order("id ASC").First(&user).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return nil, hashErr
		}
		user = models.User{
			Username:     username,
			PasswordHash: string(hash),
			Role:         models.RoleTenantUser,
			Status:       "active",
		}
		if err := db.Create(&user).Error; err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	}

	var count int64
	if err := db.Model(&models.TenantMember{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, user.ID).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("username %q already exists in this tenant", username)
	}

	member := models.TenantMember{TenantID: tenantID, UserID: user.ID, Role: role, Status: "active"}
	if err := db.Create(&member).Error; err != nil {
		return nil, fmt.Errorf("failed to create user membership: %w", err)
	}

	user.TenantID = &tenantID
	user.Role = role
	user.Status = member.Status
	return &user, nil
}
