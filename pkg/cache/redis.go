package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisCache struct {
	client *redis.Client
}

func newRedisCache(cfg *Config) (Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &redisCache{client: client}, nil
}

func (c *redisCache) Get(_ context.Context, key string) (*CacheEntry, error) {
	val, err := c.client.Get(context.Background(), key).Result()
	if err != nil {
		return nil, err
	}

	var entry CacheEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

func (c *redisCache) Set(_ context.Context, key string, value *CacheEntry, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(context.Background(), key, string(data), ttl).Err()
}

func (c *redisCache) Delete(_ context.Context, key string) error {
	return c.client.Del(context.Background(), key).Err()
}

func (c *redisCache) Clear(_ context.Context) error {
	return c.client.FlushDB(context.Background()).Err()
}
