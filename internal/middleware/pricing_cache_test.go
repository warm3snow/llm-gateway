package middleware

import (
	"testing"
	"time"

	"github.com/warm3snow/llm-gateway/internal/models"
)

func TestPricingCachePositiveHitExpires(t *testing.T) {
	now := time.Unix(100, 0)
	cache := newPricingCache(time.Minute, 10*time.Second)
	cache.now = func() time.Time { return now }
	pricing := &models.ModelPricing{Provider: "openai", Model: "gpt-test", InputPrice: 1, OutputPrice: 2}

	cache.put("openai", "gpt-test", pricing)
	got, ok := cache.get("openai", "gpt-test")
	if !ok || got != pricing {
		t.Fatalf("pricing cache hit = (%v,%t), want pricing,true", got, ok)
	}

	now = now.Add(time.Minute + time.Nanosecond)
	if _, ok := cache.get("openai", "gpt-test"); ok {
		t.Fatalf("pricing cache entry should expire")
	}
}

func TestPricingCacheEnforcesCapacity(t *testing.T) {
	cache := newPricingCache(time.Minute, 10*time.Second)
	cache.maxEntries = 2
	cache.put("openai", "a", &models.ModelPricing{})
	cache.put("openai", "b", &models.ModelPricing{})
	cache.put("openai", "c", &models.ModelPricing{})
	if len(cache.entries) != 2 {
		t.Fatalf("pricing cache size = %d, want 2", len(cache.entries))
	}
}

func TestPricingCacheCachesMissBriefly(t *testing.T) {
	now := time.Unix(100, 0)
	cache := newPricingCache(time.Minute, 10*time.Second)
	cache.now = func() time.Time { return now }

	cache.put("openai", "missing", nil)
	got, ok := cache.get("openai", "missing")
	if !ok || got != nil {
		t.Fatalf("missing pricing cache hit = (%v,%t), want nil,true", got, ok)
	}

	now = now.Add(10*time.Second + time.Nanosecond)
	if _, ok := cache.get("openai", "missing"); ok {
		t.Fatalf("negative pricing cache entry should expire")
	}
}
