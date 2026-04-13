package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache provides Redis-backed caching functionality with TTL support
type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

// MemoryCache provides in-memory caching fallback
type MemoryCache struct {
	data map[string]interface{}
	ttl  time.Duration
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
	RiskPrefix          = "risk:"
	RiskListPrefix      = "risk:list:"
	UserPrefix          = "user:"
	ConnectorPrefix     = "connector:"
	ConnectorListPrefix = "connector:list:"
	StatsPrefix         = "stats:"
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

// InitializeCache initializes Redis cache from environment variables
func InitializeCache() (*Cache, error) {
	host := os.Getenv("REDIS_HOST")
	if host == "" {
		host = "localhost"
	}

	port := 6379
	if p := os.Getenv("REDIS_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	password := os.Getenv("REDIS_PASSWORD")

	db := 0
	if d := os.Getenv("REDIS_DB"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil {
			db = parsed
		}
	}

	ttl := 5 * time.Minute
	if t := os.Getenv("REDIS_TTL_SECONDS"); t != "" {
		if seconds, err := strconv.Atoi(t); err == nil {
			ttl = time.Duration(seconds) * time.Second
		}
	}

	cfg := CacheConfig{
		Host:     host,
		Port:     port,
		Password: password,
		DB:       db,
		TTL:      ttl,
	}

	return New(cfg)
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(host, port, password string) (*Cache, error) {
	portInt := 6379
	if p, err := strconv.Atoi(port); err == nil {
		portInt = p
	}
	cfg := CacheConfig{
		Host:     host,
		Port:     portInt,
		Password: password,
		DB:       0,
		TTL:      5 * time.Minute,
	}
	return New(cfg)
}

// NewMemoryCache creates a new in-memory cache instance
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		data: make(map[string]interface{}),
		ttl:  5 * time.Minute,
	}
}

// MemoryCache methods for interface compatibility
func (mc *MemoryCache) Set(ctx context.Context, key string, value interface{}) error {
	mc.data[key] = value
	return nil
}

func (mc *MemoryCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	mc.data[key] = value
	return nil
}

func (mc *MemoryCache) Get(ctx context.Context, key string, dest interface{}) error {
	val, ok := mc.data[key]
	if !ok {
		return ErrCacheMiss
	}
	if b, ok := val.([]byte); ok {
		return json.Unmarshal(b, dest)
	}
	return fmt.Errorf("type assertion failed")
}

func (mc *MemoryCache) GetString(ctx context.Context, key string) (string, error) {
	val, ok := mc.data[key]
	if !ok {
		return "", ErrCacheMiss
	}
	if s, ok := val.(string); ok {
		return s, nil
	}
	return "", fmt.Errorf("type assertion failed")
}

func (mc *MemoryCache) Delete(ctx context.Context, keys ...string) error {
	for _, key := range keys {
		delete(mc.data, key)
	}
	return nil
}

func (mc *MemoryCache) DeletePattern(ctx context.Context, pattern string) error {
	// Simple pattern matching for memory cache
	for key := range mc.data {
		if matchPattern(key, pattern) {
			delete(mc.data, key)
		}
	}
	return nil
}

func (mc *MemoryCache) Close() error {
	mc.data = make(map[string]interface{})
	return nil
}

func (mc *MemoryCache) HealthCheck(ctx context.Context) error {
	return nil
}

// matchPattern performs simple wildcard pattern matching
func matchPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == key {
		return true
	}
	// Handle prefix* pattern
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	return false
}
