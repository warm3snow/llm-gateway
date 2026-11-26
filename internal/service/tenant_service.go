package service

import (
	"errors"
	"fmt"
	"time"

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
		Name:          req.Name,
		Slug:          req.Slug,
		Status:        "active",
		BudgetMonthly: req.BudgetMonthly,
		TpmLimit:      req.TpmLimit,
		RpmLimit:      req.RpmLimit,
		AllowedModels: models.MarshalAllowedModels(req.AllowedModels),
		AllowSelfKey:  req.AllowSelfKey,
	}
	if err := s.db.Create(t).Error; err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}
	return t, nil
}

// Update updates tenant name/quota fields.
func (s *TenantService) Update(id uint, req *models.TenantRequest) (*models.Tenant, error) {
	var t models.Tenant
	if err := s.db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("tenant not found")
		}
		return nil, err
	}
	t.Name = req.Name
	t.BudgetMonthly = req.BudgetMonthly
	t.TpmLimit = req.TpmLimit
	t.RpmLimit = req.RpmLimit
	t.AllowedModels = models.MarshalAllowedModels(req.AllowedModels)
	t.AllowSelfKey = req.AllowSelfKey
	if err := s.db.Save(&t).Error; err != nil {
		return nil, fmt.Errorf("failed to update tenant: %w", err)
	}
	return &t, nil
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
	return createTenantMembershipUser(s.db, req.TenantID, req.Username, req.Email, req.Password, role, req.BudgetMonthly, req.RpmLimit)
}

// ListUsers returns users, optionally filtered by tenant (0 = all).
func (s *TenantService) ListUsers(tenantID uint) ([]models.User, error) {
	return listTenantUsers(s.db, tenantID)
}

// TenantCostItem is one row of a tenant cost breakdown.
type TenantCostItem struct {
	UserID       uint    `json:"user_id,omitempty"`
	Username     string  `json:"username,omitempty"`
	Model        string  `json:"model,omitempty"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	Cost         float64 `json:"cost"`
}

// TenantCost is the response payload for tenant cost reports.
type TenantCost struct {
	TenantID  uint             `json:"tenant_id"`
	TotalReq  int64            `json:"total_requests"`
	TotalCost float64          `json:"total_cost"`
	Items     []TenantCostItem `json:"items"`
}

// Cost aggregates the tenant's month-to-date spend grouped by user or model.
func (s *TenantService) Cost(tenantID uint, by string) (*TenantCost, error) {
	var tenant models.Tenant
	if err := s.db.First(&tenant, tenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("tenant not found")
		}
		return nil, err
	}

	groupField := "COALESCE(usage_records.user_id, 0)"
	if by == "model" {
		groupField = "usage_records.model"
	}
	monthStart := startOfCurrentMonth()

	q := s.db.Table("usage_records").
		Select(groupField+" AS grp, COUNT(*) AS requests, COALESCE(SUM(input_tokens),0) AS input_tokens, COALESCE(SUM(output_tokens),0) AS output_tokens, COALESCE(SUM(cost),0) AS cost").
		Where("tenant_id = ? AND created_at >= ?", tenantID, monthStart).
		Group("grp").
		Order("cost DESC")
	var rows []struct {
		Grp          interface{}
		Requests     int64
		InputTokens  int64
		OutputTokens int64
		Cost         float64
	}
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := &TenantCost{TenantID: tenantID, Items: []TenantCostItem{}}
	if by == "user" {
		usernames := map[uint]string{}
		var users []models.User
		if err := s.db.Find(&users).Error; err == nil {
			for _, u := range users {
				usernames[u.ID] = u.Username
			}
		}
		for _, r := range rows {
			uid, _ := toUint(r.Grp)
			item := TenantCostItem{
				UserID:       uid,
				Requests:     r.Requests,
				InputTokens:  r.InputTokens,
				OutputTokens: r.OutputTokens,
				Cost:         r.Cost,
			}
			if name, ok := usernames[uid]; ok {
				item.Username = name
			}
			result.Items = append(result.Items, item)
		}
	} else {
		for _, r := range rows {
			model, _ := r.Grp.(string)
			result.Items = append(result.Items, TenantCostItem{
				Model:        model,
				Requests:     r.Requests,
				InputTokens:  r.InputTokens,
				OutputTokens: r.OutputTokens,
				Cost:         r.Cost,
			})
		}
	}
	for _, r := range rows {
		result.TotalReq += r.Requests
		result.TotalCost += r.Cost
	}
	return result, nil
}

func toUint(v interface{}) (uint, bool) {
	switch n := v.(type) {
	case uint:
		return n, true
	case int64:
		return uint(n), true
	case float64:
		return uint(n), true
	case []uint8:
		var out uint64
		for _, b := range n {
			out = out*10 + uint64(b-'0')
		}
		return uint(out), true
	}
	return 0, false
}

func startOfCurrentMonth() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}
