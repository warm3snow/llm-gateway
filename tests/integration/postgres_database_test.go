//go:build integration

package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

func TestPostgresDatabaseMigrateBootstrapAndTenantScope(t *testing.T) {
	dsn := testutil.StartPostgresContainer(t)
	require.NoError(t, database.Connect(&database.Config{Driver: "postgres", DSN: dsn, LogLevel: "silent"}))
	t.Cleanup(func() {
		_ = database.Close()
		database.DB = nil
	})

	require.NoError(t, database.Migrate(
		&models.Tenant{},
		&models.User{},
		&models.TenantMember{},
		&models.VirtualKey{},
		&models.UsageRecord{},
		&models.ProviderConfig{},
		&models.CacheEntry{},
		&models.ModelPricing{},
	))
	require.NoError(t, database.Bootstrap("root", "rootpass"))

	var tenant models.Tenant
	require.NoError(t, database.GetDB().First(&tenant, database.DefaultTenantID).Error)
	assert.Equal(t, "default", tenant.Slug)

	var admin models.User
	require.NoError(t, database.GetDB().Where("username = ?", "root").First(&admin).Error)
	assert.Equal(t, models.RoleSuperAdmin, admin.Role)
	assert.Nil(t, admin.TenantID)

	require.NoError(t, database.GetDB().Create(&models.Tenant{ID: 2, Name: "Tenant Two", Slug: "tenant-two", Status: "active"}).Error)
	require.NoError(t, database.GetDB().Create(&models.VirtualKey{TenantID: 1, Name: "tenant-one-key", KeyHash: "hash-1", KeySalt: "salt-1", HashedKey: "vsk-one", Status: "active"}).Error)
	require.NoError(t, database.GetDB().Create(&models.VirtualKey{TenantID: 2, Name: "tenant-two-key", KeyHash: "hash-2", KeySalt: "salt-2", HashedKey: "vsk-two", Status: "active"}).Error)

	var scoped []models.VirtualKey
	require.NoError(t, database.GetDB().Scopes(database.TenantScope(2)).Find(&scoped).Error)
	require.Len(t, scoped, 1)
	assert.Equal(t, "tenant-two-key", scoped[0].Name)
}
