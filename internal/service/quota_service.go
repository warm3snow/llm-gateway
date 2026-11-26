package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"gorm.io/gorm"
)

// Cascade quota errors — mapped by the middleware to HTTP statuses.
var (
	ErrUserQuotaExceeded   = errors.New("user monthly quota exceeded")
	ErrTenantQuotaExceeded = errors.New("tenant monthly budget exceeded")
	ErrTenantRPMExceeded   = errors.New("tenant rpm limit exceeded")
	ErrModelNotAllowed     = errors.New("model is not allowed for this tenant")
)

// tenantRPM is a lightweight sliding-window limiter for tenant rpm_limit.
type rpmLimiter struct {
	mu      sync.Mutex
	windows map[uint][]time.Time
}

func (l *rpmLimiter) allow(tenantID uint, limit int) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windows == nil {
		l.windows = make(map[uint][]time.Time)
	}
	window := l.windows[tenantID]
	cutoff := now.Add(-time.Minute)
	kept := window[:0]
	for _, t := range window {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= limit {
		l.windows[tenantID] = kept
		return false
	}
	l.windows[tenantID] = append(kept, now)
	return true
}

var tenantRPM rpmLimiter

// QuotaService resolves key → user → tenant quota chains. The DB handle is
// resolved per call so a database re-initialization (tests, hot swap) is
// always picked up. Pending (not yet flushed) async budget costs are supplied
// by the caller from BudgetTracker.
type QuotaService struct{}

// NewQuotaService creates a QuotaService.
func NewQuotaService() *QuotaService {
	return &QuotaService{}
}

func (s *QuotaService) db() *gorm.DB {
	return database.GetDB()
}

// ResolveUser loads the owning user for a virtual key (nil if unassigned).
func (s *QuotaService) ResolveUser(vk *models.VirtualKey) (*models.User, error) {
	if vk.UserID == nil {
		return nil, nil
	}
	var user models.User
	if err := s.db().First(&user, *vk.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if user.Status != "active" {
		return nil, fmt.Errorf("owning user %q is disabled", user.Username)
	}
	return &user, nil
}

// ResolveTenant loads the tenant for a virtual key.
func (s *QuotaService) ResolveTenant(vk *models.VirtualKey) (*models.Tenant, error) {
	var tenant models.Tenant
	if err := s.db().First(&tenant, vk.TenantID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tenant, nil
}

// CheckUserQuota verifies the user's monthly budget with pending spend.
// Returns nil when the user has no quota set (unlimited).
func (s *QuotaService) CheckUserQuota(user *models.User, pending float64) error {
	if user == nil || user.BudgetMonthly <= 0 {
		return nil
	}
	ensureMonthlyResetUser(s.db(), user)
	if user.BudgetUsed+pending >= user.BudgetMonthly {
		return ErrUserQuotaExceeded
	}
	return nil
}

// CheckTenantQuota verifies the tenant's monthly budget with pending spend.
// Returns nil when the tenant has no budget set (unlimited). The tenant's
// rpm_limit is enforced via an in-process limiter.
func (s *QuotaService) CheckTenantQuota(tenant *models.Tenant, pending float64) error {
	if tenant == nil {
		return nil
	}
	if tenant.Status != "active" {
		return fmt.Errorf("tenant %q is disabled", tenant.Name)
	}
	if tenant.RpmLimit > 0 && !tenantRPM.allow(tenant.ID, tenant.RpmLimit) {
		return ErrTenantRPMExceeded
	}
	if tenant.BudgetMonthly <= 0 {
		return nil
	}
	ensureMonthlyResetTenant(s.db(), tenant)
	if tenant.BudgetUsed+pending >= tenant.BudgetMonthly {
		return ErrTenantQuotaExceeded
	}
	return nil
}

// CheckModelAllowed verifies the request model against the tenant whitelist.
// Empty whitelist = all models allowed.
func (s *QuotaService) CheckModelAllowed(tenant *models.Tenant, model string) error {
	if tenant == nil {
		return nil
	}
	allowed := models.UnmarshalAllowedModels(tenant.AllowedModels)
	if len(allowed) == 0 {
		return nil
	}
	if strings.TrimSpace(model) == "" {
		return nil // deferred/unknown models cannot be validated here
	}
	m := strings.ToLower(strings.TrimSpace(model))
	for _, a := range allowed {
		if strings.ToLower(strings.TrimSpace(a)) == m {
			return nil
		}
	}
	return ErrModelNotAllowed
}

// TrackSpend records cost against user and tenant month-to-date budgets.
// Called from the async budget tracker flush path.
func (s *QuotaService) TrackSpend(vk *models.VirtualKey, cost float64) error {
	if vk == nil || cost <= 0 {
		return nil
	}
	now := time.Now().UTC()
	if vk.UserID != nil {
		if err := s.db().Model(&models.User{}).Where("id = ?", *vk.UserID).
			UpdateColumn("budget_used", gorm.Expr("budget_used + ?", cost)).Error; err != nil {
			return err
		}
		if err := s.db().Model(&models.User{}).Where("id = ?", *vk.UserID).
			UpdateColumn("budget_reset_at", nextMonthStart(now)).Error; err != nil {
			return err
		}
	}
	if err := s.db().Model(&models.Tenant{}).Where("id = ?", vk.TenantID).
		UpdateColumn("budget_used", gorm.Expr("budget_used + ?", cost)).Error; err != nil {
		return err
	}
	return s.db().Model(&models.Tenant{}).Where("id = ?", vk.TenantID).
		UpdateColumn("budget_reset_at", nextMonthStart(now)).Error
}

// ResetMonthlyBudgets zeroes month-to-date spends whose reset time passed.
// Runs at startup and periodically; returns the number of rows reset.
func ResetMonthlyBudgets(db *gorm.DB) (int64, error) {
	if db == nil {
		return 0, nil
	}
	now := time.Now().UTC()
	var reset int64

	if err := db.Model(&models.Tenant{}).
		Where("budget_reset_at IS NOT NULL AND budget_reset_at <= ? AND budget_used > 0", now).
		Update("budget_used", 0).Error; err != nil {
		return 0, err
	}
	res := db.Model(&models.Tenant{}).
		Where("budget_reset_at IS NULL OR budget_reset_at <= ?", now).
		Update("budget_reset_at", nextMonthStart(now))
	if res.Error != nil {
		return 0, res.Error
	}
	reset += res.RowsAffected

	if err := db.Model(&models.User{}).
		Where("budget_reset_at IS NOT NULL AND budget_reset_at <= ? AND budget_used > 0", now).
		Update("budget_used", 0).Error; err != nil {
		return reset, err
	}
	res = db.Model(&models.User{}).
		Where("budget_reset_at IS NULL OR budget_reset_at <= ?", now).
		Update("budget_reset_at", nextMonthStart(now))
	if res.Error != nil {
		return reset, res.Error
	}
	reset += res.RowsAffected
	return reset, nil
}

func ensureMonthlyResetTenant(db *gorm.DB, tenant *models.Tenant) {
	if tenant.BudgetResetAt == nil || tenant.BudgetResetAt.Before(time.Now().UTC()) {
		_ = db.Model(&models.Tenant{}).Where("id = ?", tenant.ID).
			Update("budget_reset_at", nextMonthStart(time.Now().UTC())).Error
		tenant.BudgetResetAt = nil
		tenant.BudgetUsed = 0 // optimistically treat as reset for this check
	}
}

func ensureMonthlyResetUser(db *gorm.DB, user *models.User) {
	if user.BudgetResetAt == nil || user.BudgetResetAt.Before(time.Now().UTC()) {
		_ = db.Model(&models.User{}).Where("id = ?", user.ID).
			Update("budget_reset_at", nextMonthStart(time.Now().UTC())).Error
		user.BudgetResetAt = nil
		user.BudgetUsed = 0
	}
}

// nextMonthStart returns the first instant of the next calendar month (UTC).
func nextMonthStart(now time.Time) time.Time {
	year, month, _ := now.Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	next := firstOfMonth.AddDate(0, 1, 0)
	return next
}
