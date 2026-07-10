package middleware

import (
	"sync"
	"time"

	"github.com/warm3snow/llm-gateway/internal/models"
)

const defaultPricingCacheMaxEntries = 10000

type pricingCache struct {
	mu          sync.Mutex
	entries     map[string]pricingCacheEntry
	positiveTTL time.Duration
	negativeTTL time.Duration
	maxEntries  int
	now         func() time.Time
}

type pricingCacheEntry struct {
	pricing   *models.ModelPricing
	expiresAt time.Time
}

func newPricingCache(positiveTTL, negativeTTL time.Duration) *pricingCache {
	return &pricingCache{
		entries:     make(map[string]pricingCacheEntry),
		positiveTTL: positiveTTL,
		negativeTTL: negativeTTL,
		maxEntries:  defaultPricingCacheMaxEntries,
		now:         time.Now,
	}
}

func (c *pricingCache) get(provider, model string) (*models.ModelPricing, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := pricingCacheKey(provider, model)
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !c.now().Before(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return entry.pricing, true
}

func (c *pricingCache) put(provider, model string, pricing *models.ModelPricing) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ttl := c.positiveTTL
	if pricing == nil {
		ttl = c.negativeTTL
	}
	c.sweepLocked()
	if len(c.entries) >= c.maxEntries {
		deleteOnePricing(c.entries)
	}
	c.entries[pricingCacheKey(provider, model)] = pricingCacheEntry{pricing: pricing, expiresAt: c.now().Add(ttl)}
}

func (c *pricingCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]pricingCacheEntry)
}

func (c *pricingCache) sweepLocked() {
	now := c.now()
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
}

func deleteOnePricing(entries map[string]pricingCacheEntry) {
	for key := range entries {
		delete(entries, key)
		return
	}
}

func pricingCacheKey(provider, model string) string {
	return provider + "\x00" + model
}
