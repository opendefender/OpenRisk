package cache

import (
"context"
"encoding/json"
"fmt"
"strings"
"time"

"github.com/redis/go-redis/v9"
)

// Cache provides Redis-backed caching functionality with TTL support
type Cache struct {
client *redis.Client
ttl    time.Duration
}

// CacheConfig holds Redis connection configuration
type CacheConfig struct {
Host     string
Port     int
Password string
DB       int
TTL      time.Duration // Default TTL for cache entries
}

// KeyPrefix constants for cache key organization
const (
RiskPrefix     = "risk:"
RiskListPrefix = "risk:list:"
UserPrefix = "user:"
ConnectorPrefix    = "connector:"
ConnectorListPrefix = "connector:list:"
StatsPrefix = "stats:"
)

// New creates a new Redis cache instance
func New(config CacheConfig) (*Cache, error) {
if config.Host == "" {
return nil, ErrConnectionFailed
}

client := redis.NewClient(&redis.Options{
Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
Password: config.Password,
DB:       config.DB,
})

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := client.Ping(ctx).Err(); err != nil {
return nil, fmt.Errorf("redis connection failed: %w", err)
}

return &Cache{client: client, ttl: config.TTL}, nil
}

// Set stores a value in cache with default TTL
func (c *Cache) Set(ctx context.Context, key string, value interface{}) error {
return c.SetWithTTL(ctx, key, value, c.ttl)
}

// SetWithTTL stores a value in cache with custom TTL
func (c *Cache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
data, err := json.Marshal(value)
if err != nil {
return fmt.Errorf("marshal failed: %w", err)
}
return c.client.Set(ctx, key, data, ttl).Err()
}

// Get retrieves a value from cache
func (c *Cache) Get(ctx context.Context, key string, dest interface{}) error {
val, err := c.client.Get(ctx, key).Result()
if err != nil {
if err == redis.Nil {
return ErrCacheMiss
}
return err
}
return json.Unmarshal([]byte(val), dest)
}

// GetString retrieves a string value from cache
func (c *Cache) GetString(ctx context.Context, key string) (string, error) {
val, err := c.client.Get(ctx, key).Result()
if err != nil {
if err == redis.Nil {
return "", ErrCacheMiss
}
return "", err
}
return val, nil
}

// Delete removes keys from cache
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
if len(keys) == 0 {
return nil
}
return c.client.Del(ctx, keys...).Err()
}

// DeletePattern deletes all keys matching a pattern
func (c *Cache) DeletePattern(ctx context.Context, pattern string) error {
iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
for iter.Next(ctx) {
key := iter.Val()
if err := c.client.Del(ctx, key).Err(); err != nil {
return err
}
}
return iter.Err()
}

// Close closes the Redis connection
func (c *Cache) Close() error {
return c.client.Close()
}

// HealthCheck verifies Redis connectivity
func (c *Cache) HealthCheck(ctx context.Context) error {
return c.client.Ping(ctx).Err()
}
