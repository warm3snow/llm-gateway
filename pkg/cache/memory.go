package cache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type memoryCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	maxEntries int
}

func newMemoryCache(maxEntries int) (Cache, error) {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &memoryCache{
		entries:    make(map[string]*CacheEntry),
		maxEntries: maxEntries,
	}, nil
}

func (c *memoryCache) Get(_ context.Context, key string) (*CacheEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, fmt.Errorf("cache miss")
	}

	if time.Now().After(entry.ExpiresAt) {
		// Lazily clean up expired entry
		go c.Delete(context.Background(), key)
		return nil, fmt.Errorf("cache miss (expired)")
	}

	return entry, nil
}

func (c *memoryCache) Set(_ context.Context, key string, value *CacheEntry, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Simple LRU: if at capacity, remove a random entry
	if len(c.entries) >= c.maxEntries {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	if value.ExpiresAt.IsZero() {
		value.ExpiresAt = time.Now().Add(ttl)
	}
	c.entries[key] = value
	return nil
}

func (c *memoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
	return nil
}

func (c *memoryCache) Clear(_ context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
	return nil
}
