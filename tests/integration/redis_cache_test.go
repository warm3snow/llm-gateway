//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/warm3snow/llm-gateway/pkg/cache"
	"github.com/warm3snow/llm-gateway/tests/testutil"
)

func TestRedisCacheSetGetDeleteClearAndTTL(t *testing.T) {
	addr := testutil.StartRedisContainer(t)
	c, err := cache.NewCache(&cache.Config{Type: "redis", RedisAddr: addr, DefaultTTL: time.Minute})
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, c.Set(ctx, "chat:1", &cache.CacheEntry{Key: "chat:1", ResponseText: "hello"}, time.Minute))
	got, err := c.Get(ctx, "chat:1")
	require.NoError(t, err)
	assert.Equal(t, "hello", got.ResponseText)

	require.NoError(t, c.Delete(ctx, "chat:1"))
	_, err = c.Get(ctx, "chat:1")
	assert.Error(t, err)

	require.NoError(t, c.Set(ctx, "chat:2", &cache.CacheEntry{Key: "chat:2"}, time.Minute))
	require.NoError(t, c.Clear(ctx))
	_, err = c.Get(ctx, "chat:2")
	assert.Error(t, err)

	require.NoError(t, c.Set(ctx, "expiring", &cache.CacheEntry{Key: "expiring"}, 200*time.Millisecond))
	require.Eventually(t, func() bool {
		_, err := c.Get(ctx, "expiring")
		return err != nil
	}, time.Second, 50*time.Millisecond)
}
