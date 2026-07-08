package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/internal/models"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

func TestVirtualKeyServiceCreateAndValidateKey(t *testing.T) {
	testutil.SetupSQLiteDB(t)
	testutil.CreateTenant(t, 1, "tenant-one")
	creatorID := uint(42)
	svc := NewVirtualKeyService()

	fullKey, vk, err := svc.Create(1, &models.VirtualKeyRequest{
		Name:        "production",
		BudgetTotal: 100,
		Providers:   []string{"openai", "anthropic"},
	}, &creatorID, "admin")
	require.NoError(t, err)
	require.NotEmpty(t, fullKey)
	assert.Contains(t, fullKey, "vsk-")
	assert.Equal(t, uint(1), vk.TenantID)
	assert.Equal(t, "production", vk.Name)
	assert.Equal(t, float64(100), vk.BudgetTotal)
	assert.Equal(t, 60, vk.RateLimitWindow)
	assert.Equal(t, "openai,anthropic", vk.Providers)
	assert.Equal(t, &creatorID, vk.CreatedByUserID)

	validated, err := svc.ValidateKey(fullKey)
	require.NoError(t, err)
	assert.Equal(t, vk.ID, validated.ID)
}

func TestVirtualKeyServiceRejectsKeyWhenBudgetIsExceeded(t *testing.T) {
	testutil.SetupSQLiteDB(t)
	testutil.CreateTenant(t, 1, "tenant-one")
	svc := NewVirtualKeyService()

	fullKey, vk, err := svc.Create(1, &models.VirtualKeyRequest{Name: "limited", BudgetTotal: 1}, nil, "")
	require.NoError(t, err)
	require.NoError(t, svc.TrackUsage(vk.ID, 1))

	validated, err := svc.ValidateKey(fullKey)
	assert.Nil(t, validated)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "budget exceeded")
}

func TestVirtualKeyServiceScopesKeysByTenant(t *testing.T) {
	testutil.SetupSQLiteDB(t)
	testutil.CreateTenant(t, 1, "tenant-one")
	testutil.CreateTenant(t, 2, "tenant-two")
	svc := NewVirtualKeyService()

	_, tenantOneKey, err := svc.Create(1, &models.VirtualKeyRequest{Name: "tenant-one-key"}, nil, "")
	require.NoError(t, err)
	_, tenantTwoKey, err := svc.Create(2, &models.VirtualKeyRequest{Name: "tenant-two-key"}, nil, "")
	require.NoError(t, err)

	keys, err := svc.List(1)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, tenantOneKey.ID, keys[0].ID)

	got, err := svc.GetByID(1, tenantOneKey.ID)
	require.NoError(t, err)
	assert.Equal(t, tenantOneKey.ID, got.ID)

	got, err = svc.GetByID(1, tenantTwoKey.ID)
	assert.Nil(t, got)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "virtual key not found")
}

func TestVirtualKeyServiceListsKeysByCreator(t *testing.T) {
	testutil.SetupSQLiteDB(t)
	testutil.CreateTenant(t, 1, "tenant-one")
	svc := NewVirtualKeyService()
	creatorOne := uint(10)
	creatorTwo := uint(20)

	_, first, err := svc.Create(1, &models.VirtualKeyRequest{Name: "first"}, &creatorOne, "user-a")
	require.NoError(t, err)
	_, _, err = svc.Create(1, &models.VirtualKeyRequest{Name: "second"}, &creatorTwo, "user-b")
	require.NoError(t, err)

	keys, err := svc.ListByCreator(1, creatorOne)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Equal(t, first.ID, keys[0].ID)

	got, err := svc.GetByIDAndCreator(1, first.ID, creatorOne)
	require.NoError(t, err)
	assert.Equal(t, first.ID, got.ID)

	got, err = svc.GetByIDAndCreator(1, first.ID, creatorTwo)
	assert.Nil(t, got)
	assert.Error(t, err)
}
