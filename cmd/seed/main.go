package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/warm3snow/llm-gateway/internal/config"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	seedmanifest "github.com/warm3snow/llm-gateway/internal/seed"
	"github.com/warm3snow/llm-gateway/internal/service"
	"github.com/warm3snow/llm-gateway/internal/types"
	"github.com/warm3snow/llm-gateway/pkg/encryption"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	defaultAdminPassword = "admin1"
	defaultUserPassword  = "user1"
)

type seedOptions struct {
	Profile              string
	ConfigPath           string
	ManifestPath         string
	BaseURL              string
	VirtualKeysPerTenant int
}

func main() {
	opts := seedOptions{}
	flag.StringVar(&opts.Profile, "profile", "demo", "seed profile: dev or demo")
	flag.StringVar(&opts.ConfigPath, "config", "", "path to config.yaml")
	flag.StringVar(&opts.ManifestPath, "manifest", "", "manifest output path")
	flag.StringVar(&opts.BaseURL, "base-url", "", "gateway base URL written to the manifest")
	flag.IntVar(&opts.VirtualKeysPerTenant, "virtual-keys-per-tenant", 1, "number of virtual keys to seed per tenant")
	flag.Parse()

	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(opts seedOptions) error {
	profile := strings.TrimSpace(opts.Profile)
	if profile == "" {
		profile = "demo"
	}
	if opts.ManifestPath == "" {
		opts.ManifestPath = seedmanifest.DefaultManifestPath(profile)
	}
	if opts.VirtualKeysPerTenant <= 0 {
		return fmt.Errorf("virtual-keys-per-tenant must be > 0")
	}

	cfgPath := opts.ConfigPath
	if cfgPath == "" {
		cfgPath = config.GetConfigPath()
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := initEncryption(cfg); err != nil {
		return err
	}
	if err := database.Connect(&database.Config{
		Driver:          cfg.Database.Driver,
		DSN:             cfg.Database.DSN,
		LogLevel:        cfg.Database.LogLevel,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	}); err != nil {
		return err
	}
	defer database.Close()
	if err := migrate(); err != nil {
		return err
	}
	if err := database.Bootstrap(cfg.Security.AdminUser, cfg.Security.AdminPass); err != nil {
		return fmt.Errorf("bootstrap database: %w", err)
	}

	manifest := &seedmanifest.Manifest{
		Profile: profile,
		BaseURL: baseURL(opts.BaseURL, cfg),
	}

	providerNames, err := seedProviders(cfg, manifest)
	if err != nil {
		return err
	}
	if len(providerNames) == 0 {
		return fmt.Errorf("no providers available; configure gateway.providers before seeding")
	}
	if err := seedPricing(providerNames); err != nil {
		return err
	}
	if err := seedTenantsUsersAndKeys(profile, providerNames, manifest, opts.VirtualKeysPerTenant); err != nil {
		return err
	}
	if err := seedmanifest.SaveManifest(opts.ManifestPath, manifest); err != nil {
		return err
	}

	log.Printf("seeded profile=%s tenants=%d users=%d providers=%d virtual_keys=%d manifest=%s",
		profile, len(manifest.Tenants), len(manifest.Users), len(manifest.Providers), len(manifest.VirtualKeys), opts.ManifestPath)
	return nil
}

func migrate() error {
	return database.Migrate(
		&models.Tenant{},
		&models.User{},
		&models.TenantMember{},
		&models.VirtualKey{},
		&models.UsageRecord{},
		&models.ProviderConfig{},
		&models.CacheEntry{},
		&models.ModelPricing{},
	)
}

func initEncryption(cfg *config.Config) error {
	encKey := cfg.Security.EncryptionKey
	if encKey == "" {
		sum := sha256.Sum256([]byte(cfg.Security.JWTSecret))
		encKey = hex.EncodeToString(sum[:])
	}
	if err := encryption.InitEncryptionKey(encKey); err != nil {
		return fmt.Errorf("init encryption key: %w", err)
	}
	return nil
}

func seedProviders(cfg *config.Config, manifest *seedmanifest.Manifest) ([]string, error) {
	providers := cfg.Gateway.Providers
	if len(providers) == 0 {
		providers = map[string]types.Options{
			"ollama": {
				Provider:       "ollama",
				APIKey:         "ollama",
				CustomHost:     "http://localhost:11434/v1",
				RequestTimeout: 30000,
			},
		}
	}

	svc := service.NewProviderConfigService()
	names := make([]string, 0, len(providers))
	for name, opts := range providers {
		if strings.TrimSpace(opts.Provider) == "" {
			opts.Provider = name
		}
		if err := svc.Upsert(name, opts); err != nil {
			return nil, fmt.Errorf("upsert provider %s: %w", name, err)
		}
		names = append(names, name)
		manifest.Providers = append(manifest.Providers, seedmanifest.ManifestProvider{
			Name:         name,
			ProviderType: opts.Provider,
			Models:       []string{"default"},
		})
	}
	return names, nil
}

func seedPricing(providerNames []string) error {
	db := database.GetDB()
	for _, provider := range providerNames {
		rows := []*models.ModelPricing{
			{Provider: provider, Model: "default", InputPrice: 0.00001, OutputPrice: 0.00003, Currency: "USD", Source: "seed"},
		}
		if provider == "ollama" {
			rows = append(rows,
				&models.ModelPricing{Provider: provider, Model: "qwen2.5:7b", InputPrice: 0.00001, OutputPrice: 0.00003, Currency: "USD", Source: "seed"},
				&models.ModelPricing{Provider: provider, Model: "llama3.2", InputPrice: 0.00001, OutputPrice: 0.00003, Currency: "USD", Source: "seed"},
			)
		}
		for _, row := range rows {
			if err := models.UpsertModelPricing(db, row); err != nil {
				return fmt.Errorf("upsert pricing %s/%s: %w", row.Provider, row.Model, err)
			}
		}
	}
	return nil
}

func seedTenantsUsersAndKeys(profile string, providerNames []string, manifest *seedmanifest.Manifest, virtualKeysPerTenant int) error {
	db := database.GetDB()
	tenantCount := 2
	if profile == "dev" {
		tenantCount = 1
	}

	for i := 1; i <= tenantCount; i++ {
		slug := fmt.Sprintf("seed-%s-tenant-%d", profile, i)
		tenant := models.Tenant{Slug: slug}
		if err := db.Where("slug = ?", slug).
			Assign(models.Tenant{Name: fmt.Sprintf("Seed %s Tenant %d", profile, i), Status: "active"}).
			FirstOrCreate(&tenant).Error; err != nil {
			return fmt.Errorf("upsert tenant %s: %w", slug, err)
		}
		manifest.Tenants = append(manifest.Tenants, seedmanifest.ManifestTenant{ID: tenant.ID, Name: tenant.Name, Slug: tenant.Slug})

		adminUsername, userUsername := seedUsernames(i)
		if err := migrateLegacySeedUsernames(db, tenant, adminUsername, userUsername); err != nil {
			return err
		}
		admin, err := upsertUser(db, tenant, adminUsername, defaultAdminPassword, models.RoleTenantAdmin)
		if err != nil {
			return err
		}
		user, err := upsertUser(db, tenant, userUsername, defaultUserPassword, models.RoleTenantUser)
		if err != nil {
			return err
		}
		manifest.Users = append(manifest.Users,
			seedmanifest.ManifestUser{ID: admin.ID, TenantID: tenant.ID, Username: admin.Username, Email: admin.Email, Password: defaultAdminPassword, Role: admin.Role},
			seedmanifest.ManifestUser{ID: user.ID, TenantID: tenant.ID, Username: user.Username, Email: user.Email, Password: defaultUserPassword, Role: user.Role},
		)

		for keyIndex := 1; keyIndex <= virtualKeysPerTenant; keyIndex++ {
			key, vk, err := upsertVirtualKey(db, profile, tenant, admin, providerNames, keyIndex)
			if err != nil {
				return err
			}
			manifest.VirtualKeys = append(manifest.VirtualKeys, seedmanifest.ManifestKey{
				ID:        vk.ID,
				TenantID:  tenant.ID,
				Tenant:    tenant.Slug,
				Name:      vk.Name,
				Key:       key,
				Providers: providerNames,
			})
		}
	}
	return nil
}

func seedUsernames(index int) (string, string) {
	if index <= 1 {
		return "admin1", "user1"
	}
	return fmt.Sprintf("admin%d", index), fmt.Sprintf("user%d", index)
}

func migrateLegacySeedUsernames(db *gorm.DB, tenant models.Tenant, adminUsername, userUsername string) error {
	if adminUsername != "admin1" {
		if err := renameSeedUser(db, tenant, "admin1", adminUsername, models.RoleTenantAdmin); err != nil {
			return err
		}
	}
	if userUsername != "user1" {
		if err := renameSeedUser(db, tenant, "user1", userUsername, models.RoleTenantUser); err != nil {
			return err
		}
	}
	return nil
}

func renameSeedUser(db *gorm.DB, tenant models.Tenant, oldUsername, newUsername, role string) error {
	var target models.User
	if err := db.Where("tenant_id = ? AND username = ?", tenant.ID, newUsername).First(&target).Error; err == nil {
		return db.Model(&models.User{}).
			Where("tenant_id = ? AND username = ? AND role = ? AND email = ?", tenant.ID, oldUsername, role, models.DefaultUserEmail(oldUsername, tenant.Slug)).
			Update("status", "disabled").Error
	} else if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("check seed user %s/%s: %w", tenant.Slug, newUsername, err)
	}

	updates := map[string]interface{}{
		"username": newUsername,
		"email":    models.DefaultUserEmail(newUsername, tenant.Slug),
	}
	if err := db.Model(&models.User{}).
		Where("tenant_id = ? AND username = ? AND role = ? AND email = ?", tenant.ID, oldUsername, role, models.DefaultUserEmail(oldUsername, tenant.Slug)).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("rename seed user %s/%s: %w", tenant.Slug, oldUsername, err)
	}
	return nil
}

func upsertUser(db *gorm.DB, tenant models.Tenant, username, password, role string) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password for %s/%s: %w", tenant.Slug, username, err)
	}
	tenantID := tenant.ID
	user := models.User{}
	attrs := models.User{
		TenantID:     &tenantID,
		Username:     username,
		Email:        models.DefaultUserEmail(username, tenant.Slug),
		PasswordHash: string(hash),
		Role:         role,
		Status:       "active",
	}
	if err := db.Where("tenant_id = ? AND username = ?", tenant.ID, username).
		Assign(attrs).
		FirstOrCreate(&user).Error; err != nil {
		return nil, fmt.Errorf("upsert user %s/%s: %w", tenant.Slug, username, err)
	}
	member := models.TenantMember{TenantID: tenant.ID, UserID: user.ID}
	if err := db.Where("tenant_id = ? AND user_id = ?", tenant.ID, user.ID).
		Assign(models.TenantMember{Role: role, Status: "active"}).
		FirstOrCreate(&member).Error; err != nil {
		return nil, fmt.Errorf("upsert tenant member %s/%s: %w", tenant.Slug, username, err)
	}
	return &user, nil
}

func upsertVirtualKey(db *gorm.DB, profile string, tenant models.Tenant, user *models.User, providerNames []string, index int) (string, *models.VirtualKey, error) {
	name := fmt.Sprintf("seed-%s-key-%s", profile, tenant.Slug)
	if index > 1 {
		name = fmt.Sprintf("%s-%d", name, index)
	}
	key, err := randomVirtualKey()
	if err != nil {
		return "", nil, err
	}
	salt, err := randomHex(16)
	if err != nil {
		return "", nil, err
	}
	vk := models.VirtualKey{}
	attrs := models.VirtualKey{
		TenantID:          tenant.ID,
		CreatedByUserID:   &user.ID,
		CreatedByUsername: user.Username,
		Name:              name,
		KeyHash:           encryption.HashVirtualKey(key, salt),
		KeySalt:           salt,
		HashedKey:         key[:12],
		BudgetTotal:       10000,
		RateLimit:         1000,
		RateLimitWindow:   60,
		Providers:         strings.Join(providerNames, ","),
		Status:            "active",
		Metadata:          fmt.Sprintf(`{"seed_profile":"%s","tenant":"%s"}`, profile, tenant.Slug),
	}
	if err := db.Where("tenant_id = ? AND name = ?", tenant.ID, name).First(&vk).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return "", nil, fmt.Errorf("load virtual key %s/%s: %w", tenant.Slug, name, err)
		}
		vk = attrs
		if err := db.Create(&vk).Error; err != nil {
			return "", nil, fmt.Errorf("create virtual key %s/%s: %w", tenant.Slug, name, err)
		}
		return key, &vk, nil
	}
	attrs.ID = vk.ID
	attrs.CreatedAt = vk.CreatedAt
	attrs.BudgetUsed = vk.BudgetUsed
	if err := db.Save(&attrs).Error; err != nil {
		return "", nil, fmt.Errorf("update virtual key %s/%s: %w", tenant.Slug, name, err)
	}
	return key, &attrs, nil
}

func randomVirtualKey() (string, error) {
	value, err := randomHex(32)
	if err != nil {
		return "", fmt.Errorf("generate virtual key: %w", err)
	}
	return "vsk-" + value, nil
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func baseURL(override string, cfg *config.Config) string {
	if override != "" {
		return strings.TrimRight(override, "/")
	}
	host := cfg.Server.Host
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	if strings.Contains(host, ":") && net.ParseIP(host) != nil {
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:%d", host, cfg.Server.Port)
}
