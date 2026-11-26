package service

import (
	"errors"
	"fmt"
	"strings"

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
// to the caller's privileges. The member's monthly quota is validated against
// the tenant's monthly budget (sum of member quotas must fit).
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

	if err := ValidateMemberQuota(s.db, tenantID, 0, req.BudgetMonthly); err != nil {
		return nil, err
	}

	return createTenantMembershipUser(s.db, tenantID, req.Username, req.Email, req.Password, role, req.BudgetMonthly, req.RpmLimit)
}

// UpdateQuota updates a member's monthly quota / rpm limit with the same
// sum-of-quotas validation as Create.
func (s *UserService) UpdateQuota(actorRole string, actorTenantID uint, id uint, req *models.UserQuotaRequest) (*models.User, error) {
	var member models.TenantMember
	if err := s.db.Where("user_id = ?", id).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	switch actorRole {
	case models.RoleSuperAdmin:
	case models.RoleTenantAdmin:
		if member.TenantID != actorTenantID || member.Role != models.RoleTenantUser {
			return nil, errors.New("user not found")
		}
	default:
		return nil, errors.New("user management privileges required")
	}

	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	budget := user.BudgetMonthly
	if req.BudgetMonthly != nil {
		budget = *req.BudgetMonthly
	}
	if err := ValidateMemberQuota(s.db, member.TenantID, id, budget); err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.BudgetMonthly != nil {
		updates["budget_monthly"] = *req.BudgetMonthly
	}
	if req.RpmLimit != nil {
		updates["rpm_limit"] = *req.RpmLimit
	}
	if len(updates) == 0 {
		return &user, nil
	}
	if err := s.db.Model(&models.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// SetRole changes a tenant member's role. Only a super_admin may change roles,
// and a member cannot change their own role. Demoting the last active
// tenant_admin of a tenant is rejected so the tenant keeps an administrator.
func (s *UserService) SetRole(actorRole string, actorUserID uint, id uint, req *models.UserRoleRequest) (*models.User, error) {
	if actorRole != models.RoleSuperAdmin {
		return nil, errors.New("user management privileges required")
	}
	if id == actorUserID {
		return nil, errors.New("cannot change your own role")
	}
	if req.Role != models.RoleTenantAdmin && req.Role != models.RoleTenantUser {
		return nil, errors.New("invalid role")
	}

	var member models.TenantMember
	if err := s.membershipForUser(id, req.TenantID, &member); err != nil {
		return nil, err
	}
	if member.Role == req.Role {
		var user models.User
		if err := s.db.First(&user, id).Error; err != nil {
			return nil, err
		}
		return &user, nil
	}
	if member.Role == models.RoleTenantAdmin {
		var admins int64
		if err := s.db.Model(&models.TenantMember{}).
			Where("tenant_id = ? AND role = ? AND status = ?", member.TenantID, models.RoleTenantAdmin, "active").
			Count(&admins).Error; err != nil {
			return nil, err
		}
		if admins <= 1 {
			return nil, errors.New("cannot demote the last active tenant_admin")
		}
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.TenantMember{}).Where("id = ?", member.ID).Update("role", req.Role).Error; err != nil {
			return err
		}
		// Keep users.role in sync when the user's primary tenant is this tenant.
		return tx.Model(&models.User{}).
			Where("id = ? AND tenant_id = ?", id, member.TenantID).
			Update("role", req.Role).Error
	})
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// membershipForUser loads the membership to update. A super_admin may hint the
// tenant with tenantID; otherwise the user's single membership is used.
func (s *UserService) membershipForUser(userID, tenantID uint, out *models.TenantMember) error {
	q := s.db.Where("user_id = ?", userID)
	if tenantID != 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	var members []models.TenantMember
	if err := q.Order("tenant_id ASC").Find(&members).Error; err != nil {
		return err
	}
	switch len(members) {
	case 0:
		return errors.New("user not found")
	case 1:
		*out = members[0]
		return nil
	default:
		return errors.New("user belongs to multiple tenants; tenant_id is required")
	}
}

// ValidateMemberQuota ensures the sum of tenant member monthly quotas plus
// the candidate quota stays within the tenant's monthly budget. candidateID
// is the user being updated (excluded from the sum); 0 for creates.
func ValidateMemberQuota(db *gorm.DB, tenantID, candidateID uint, budgetMonthly float64) error {
	if budgetMonthly <= 0 {
		return nil // unlimited quota — nothing to validate
	}
	var tenant models.Tenant
	if err := db.First(&tenant, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("tenant not found")
		}
		return err
	}
	if tenant.BudgetMonthly <= 0 {
		return nil // tenant budget unlimited
	}
	var sum float64
	q := db.Model(&models.User{}).
		Joins("JOIN tenant_members ON tenant_members.user_id = users.id").
		Where("tenant_members.tenant_id = ?", tenantID)
	if candidateID > 0 {
		q = q.Where("users.id <> ?", candidateID)
	}
	if err := q.Select("COALESCE(SUM(users.budget_monthly), 0)").Scan(&sum).Error; err != nil {
		return err
	}
	if sum+budgetMonthly > tenant.BudgetMonthly {
		return fmt.Errorf("member quota exceeds tenant monthly budget: allocated %.2f + %.2f > %.2f",
			sum, budgetMonthly, tenant.BudgetMonthly)
	}
	return nil
}

// DB exposes the underlying handle (used by handlers for scoped checks).
func (s *UserService) DB() *gorm.DB {
	return s.db
}

// UserCostItem is one row of a user cost breakdown.
type UserCostItem struct {
	Model        string  `json:"model,omitempty"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	Cost         float64 `json:"cost"`
}

// UserCost is the response payload for per-user cost reports.
type UserCost struct {
	UserID    uint           `json:"user_id"`
	TotalReq  int64          `json:"total_requests"`
	TotalCost float64        `json:"total_cost"`
	Items     []UserCostItem `json:"items"`
}

// UserCost aggregates a user's month-to-date spend grouped by model.
func (s *UserService) UserCost(userID uint) (*UserCost, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	monthStart := startOfCurrentMonth()
	var rows []struct {
		Model        string
		Requests     int64
		InputTokens  int64
		OutputTokens int64
		Cost         float64
	}
	if err := s.db.Table("usage_records").
		Select("model, COUNT(*) AS requests, COALESCE(SUM(input_tokens),0) AS input_tokens, COALESCE(SUM(output_tokens),0) AS output_tokens, COALESCE(SUM(cost),0) AS cost").
		Where("user_id = ? AND created_at >= ?", userID, monthStart).
		Group("model").
		Order("cost DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := &UserCost{UserID: userID, Items: []UserCostItem{}}
	for _, r := range rows {
		result.Items = append(result.Items, UserCostItem{
			Model:        r.Model,
			Requests:     r.Requests,
			InputTokens:  r.InputTokens,
			OutputTokens: r.OutputTokens,
			Cost:         r.Cost,
		})
		result.TotalReq += r.Requests
		result.TotalCost += r.Cost
	}
	return result, nil
}

// ChangeOwnPassword updates the authenticated user's password after verifying the current password.
func (s *UserService) ChangeOwnPassword(userID uint, currentPassword, newPassword string) error {
	if len(newPassword) < 6 {
		return errors.New("new password must be at least 6 characters")
	}
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return errors.New("current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Model(&models.User{}).Where("id = ?", userID).Update("password_hash", string(hash)).Error
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
		Select("users.id, tenant_members.tenant_id, users.username, users.email, users.password_hash, tenant_members.role, tenant_members.status, users.budget_monthly, users.budget_used, users.rpm_limit, users.created_at, users.updated_at").
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

func createTenantMembershipUser(db *gorm.DB, tenantID uint, username, email, password, role string, budgetMonthly float64, rpmLimit int) (*models.User, error) {
	var tenant models.Tenant
	if err := db.First(&tenant, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("tenant not found")
		}
		return nil, err
	}

	// Email is the login identity and must be globally unique. Usernames stay
	// unique per tenant and default to the email's local part.
	email = models.NormalizeEmail(email)
	if email == "" {
		// The HTTP layer requires an email; internal callers (seed, bootstrap)
		// may omit it and get the tenant-scoped default instead.
		username := strings.TrimSpace(username)
		if username == "" {
			return nil, errors.New("email or username is required")
		}
		email = models.DefaultUserEmail(username, tenant.Slug)
	}
	if !models.ValidEmail(email) {
		return nil, fmt.Errorf("invalid email address %q", email)
	}
	var existingEmailCount int64
	if err := db.Model(&models.User{}).
		Where("LOWER(email) = ?", email).
		Count(&existingEmailCount).Error; err != nil {
		return nil, err
	}
	if existingEmailCount > 0 {
		return nil, fmt.Errorf("email %q is already taken", email)
	}

	username = strings.TrimSpace(username)
	if username == "" {
		username = models.EmailLocalPart(email)
	}
	var count int64
	if err := db.Table("tenant_members").
		Joins("JOIN users ON users.id = tenant_members.user_id").
		Where("tenant_members.tenant_id = ? AND users.username = ?", tenantID, username).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, fmt.Errorf("username %q already exists in this tenant", username)
	}

	hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if hashErr != nil {
		return nil, hashErr
	}
	user := models.User{
		TenantID:      &tenantID,
		Username:      username,
		Email:         email,
		PasswordHash:  string(hash),
		Role:          role,
		Status:        "active",
		BudgetMonthly: budgetMonthly,
		RpmLimit:      rpmLimit,
	}
	if err := db.Create(&user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	member := models.TenantMember{TenantID: tenantID, UserID: user.ID, Role: role, Status: "active"}
	if err := db.Create(&member).Error; err != nil {
		return nil, fmt.Errorf("failed to create user membership: %w", err)
	}

	return &user, nil
}
