package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCacheSetGetDeleteAndClear(t *testing.T) {
	c, err := newMemoryCache(10)
	require.NoError(t, err)
	ctx := context.Background()
	entry := &CacheEntry{Key: "chat:1", ResponseText: "hello"}

	require.NoError(t, c.Set(ctx, entry.Key, entry, time.Minute))
	got, err := c.Get(ctx, entry.Key)
	require.NoError(t, err)
	assert.Equal(t, "hello", got.ResponseText)
	assert.False(t, got.ExpiresAt.IsZero())

	require.NoError(t, c.Delete(ctx, entry.Key))
	_, err = c.Get(ctx, entry.Key)
	assert.Error(t, err)

	require.NoError(t, c.Set(ctx, "chat:2", &CacheEntry{Key: "chat:2"}, time.Minute))
	require.NoError(t, c.Clear(ctx))
	_, err = c.Get(ctx, "chat:2")
	assert.Error(t, err)
}

func TestMemoryCacheReturnsMissForExpiredEntry(t *testing.T) {
	c, err := newMemoryCache(10)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "expired", &CacheEntry{Key: "expired", ExpiresAt: time.Unix(0, 0)}, time.Minute))
	_, err = c.Get(ctx, "expired")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestMemoryCacheEvictsWhenAtCapacity(t *testing.T) {
	c, err := newMemoryCache(2)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "one", &CacheEntry{Key: "one"}, time.Minute))
	require.NoError(t, c.Set(ctx, "two", &CacheEntry{Key: "two"}, time.Minute))
	require.NoError(t, c.Set(ctx, "three", &CacheEntry{Key: "three"}, time.Minute))

	mc := c.(*memoryCache)
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	assert.Len(t, mc.entries, 2)
	assert.Contains(t, mc.entries, "three")
}
