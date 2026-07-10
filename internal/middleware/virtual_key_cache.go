package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

const (
	defaultVirtualKeyPositiveCacheMax = 10000
	defaultVirtualKeyNegativeCacheMax = 10000
)

type virtualKeyAuthCache struct {
	mu          sync.Mutex
	positive    map[string]virtualKeyPositiveEntry
	negative    map[string]time.Time
	positiveTTL time.Duration
	negativeTTL time.Duration
	maxPositive int
	maxNegative int
	now         func() time.Time
}

type virtualKeyPositiveEntry struct {
	id        uint
	expiresAt time.Time
}

func newVirtualKeyAuthCache(positiveTTL, negativeTTL time.Duration) *virtualKeyAuthCache {
	return &virtualKeyAuthCache{
		positive:    make(map[string]virtualKeyPositiveEntry),
		negative:    make(map[string]time.Time),
		positiveTTL: positiveTTL,
		negativeTTL: negativeTTL,
		maxPositive: defaultVirtualKeyPositiveCacheMax,
		maxNegative: defaultVirtualKeyNegativeCacheMax,
		now:         time.Now,
	}
}

func virtualKeyCacheDigest(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (c *virtualKeyAuthCache) getPositive(digest string) (uint, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.positive[digest]
	if !ok {
		return 0, false
	}
	if !c.now().Before(entry.expiresAt) {
		delete(c.positive, digest)
		return 0, false
	}
	return entry.id, true
}

func (c *virtualKeyAuthCache) putPositive(digest string, id uint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.negative, digest)
	c.sweepPositiveLocked()
	if len(c.positive) >= c.maxPositive {
		deleteOnePositive(c.positive)
	}
	c.positive[digest] = virtualKeyPositiveEntry{id: id, expiresAt: c.now().Add(c.positiveTTL)}
}

func (c *virtualKeyAuthCache) evictPositive(digest string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.positive, digest)
}

func (c *virtualKeyAuthCache) isNegative(digest string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	expiresAt, ok := c.negative[digest]
	if !ok {
		return false
	}
	if !c.now().Before(expiresAt) {
		delete(c.negative, digest)
		return false
	}
	return true
}

func (c *virtualKeyAuthCache) putNegative(digest string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.positive, digest)
	c.sweepNegativeLocked()
	if len(c.negative) >= c.maxNegative {
		deleteOneNegative(c.negative)
	}
	c.negative[digest] = c.now().Add(c.negativeTTL)
}

func (c *virtualKeyAuthCache) sweepPositiveLocked() {
	now := c.now()
	for digest, entry := range c.positive {
		if !now.Before(entry.expiresAt) {
			delete(c.positive, digest)
		}
	}
}

func (c *virtualKeyAuthCache) sweepNegativeLocked() {
	now := c.now()
	for digest, expiresAt := range c.negative {
		if !now.Before(expiresAt) {
			delete(c.negative, digest)
		}
	}
}

func deleteOnePositive(entries map[string]virtualKeyPositiveEntry) {
	for digest := range entries {
		delete(entries, digest)
		return
	}
}

func deleteOneNegative(entries map[string]time.Time) {
	for digest := range entries {
		delete(entries, digest)
		return
	}
}
