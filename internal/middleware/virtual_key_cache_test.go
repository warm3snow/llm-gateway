package middleware

import (
	"testing"
	"time"
)

func TestVirtualKeyAuthCachePositiveHitExpires(t *testing.T) {
	now := time.Unix(100, 0)
	cache := newVirtualKeyAuthCache(time.Minute, 10*time.Second)
	cache.now = func() time.Time { return now }
	digest := virtualKeyCacheDigest("vsk-test")

	cache.putPositive(digest, 42)
	id, ok := cache.getPositive(digest)
	if !ok || id != 42 {
		t.Fatalf("positive cache hit = (%d,%t), want (42,true)", id, ok)
	}

	now = now.Add(time.Minute + time.Nanosecond)
	if _, ok := cache.getPositive(digest); ok {
		t.Fatalf("positive cache entry should expire")
	}
}

func TestVirtualKeyAuthCacheNegativeHitExpires(t *testing.T) {
	now := time.Unix(100, 0)
	cache := newVirtualKeyAuthCache(time.Minute, 10*time.Second)
	cache.now = func() time.Time { return now }
	digest := virtualKeyCacheDigest("vsk-invalid")

	cache.putNegative(digest)
	if !cache.isNegative(digest) {
		t.Fatalf("negative cache should hit")
	}

	now = now.Add(10*time.Second + time.Nanosecond)
	if cache.isNegative(digest) {
		t.Fatalf("negative cache entry should expire")
	}
}

func TestVirtualKeyAuthCacheEnforcesCapacity(t *testing.T) {
	cache := newVirtualKeyAuthCache(time.Minute, 10*time.Second)
	cache.maxPositive = 2
	cache.maxNegative = 2

	cache.putPositive("a", 1)
	cache.putPositive("b", 2)
	cache.putPositive("c", 3)
	if len(cache.positive) != 2 {
		t.Fatalf("positive cache size = %d, want 2", len(cache.positive))
	}

	cache.putNegative("x")
	cache.putNegative("y")
	cache.putNegative("z")
	if len(cache.negative) != 2 {
		t.Fatalf("negative cache size = %d, want 2", len(cache.negative))
	}
}

func TestVirtualKeyAuthCacheEvictsPositive(t *testing.T) {
	cache := newVirtualKeyAuthCache(time.Minute, 10*time.Second)
	digest := virtualKeyCacheDigest("vsk-test")
	cache.putPositive(digest, 42)
	cache.evictPositive(digest)
	if _, ok := cache.getPositive(digest); ok {
		t.Fatalf("positive cache entry should be evicted")
	}
}
