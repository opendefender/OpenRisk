package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheService handles Redis caching operations
type CacheService struct {
	client *redis.Client
	ttl    time.Duration
}

// NewCacheService creates a new cache service instance
func NewCacheService(redisClient *redis.Client, ttl time.Duration) *CacheService {
	if ttl == 0 {
		ttl = 15 * time.Minute // default TTL
	}
	return &CacheService{
		client: redisClient,
		ttl:    ttl,
	}
}

// Get retrieves a cached value
func (cs *CacheService) Get(ctx context.Context, key string, dest interface{}) error {
	if cs.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	val, err := cs.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("cache miss for key: %s", key)
		}
		return err
	}

	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return err
	}

	return nil
}

// Set stores a value in cache with TTL
func (cs *CacheService) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
	if cs.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	duration := cs.ttl
	if len(ttl) > 0 && ttl[0] > 0 {
		duration = ttl[0]
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return cs.client.Set(ctx, key, data, duration).Err()
}

// Delete removes a cache entry
func (cs *CacheService) Delete(ctx context.Context, keys ...string) error {
	if cs.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	if len(keys) == 0 {
		return nil
	}

	return cs.client.Del(ctx, keys...).Err()
}

// DeletePattern deletes all keys matching a pattern
func (cs *CacheService) DeletePattern(ctx context.Context, pattern string) error {
	if cs.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	iter := cs.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		if err := cs.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// SetWithCallback caches a value by first executing a callback function
func (cs *CacheService) SetWithCallback(ctx context.Context, key string, callback func() (interface{}, error), ttl ...time.Duration) (interface{}, error) {
	// Check cache first
	var cached interface{}
	if err := cs.Get(ctx, key, &cached); err == nil {
		return cached, nil
	}

	// Cache miss, execute callback
	value, err := callback()
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := cs.Set(ctx, key, value, ttl...); err != nil {
		// Log the error but don't fail the request
		fmt.Printf("cache set error for key %s: %v\n", key, err)
	}

	return value, nil
}

// Exists checks if a key exists in cache
func (cs *CacheService) Exists(ctx context.Context, key string) (bool, error) {
	if cs.client == nil {
		return false, fmt.Errorf("redis client not initialized")
	}

	result, err := cs.client.Exists(ctx, key).Result()
	return result > 0, err
}

// GetTTL returns the remaining TTL for a key
func (cs *CacheService) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	if cs.client == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}

	return cs.client.TTL(ctx, key).Result()
}

// Keys retrieves all keys matching a pattern (use with caution in production)
func (cs *CacheService) Keys(ctx context.Context, pattern string) ([]string, error) {
	if cs.client == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}

	return cs.client.Keys(ctx, pattern).Result()
}

// Flush clears all cache
func (cs *CacheService) Flush(ctx context.Context) error {
	if cs.client == nil {
		return fmt.Errorf("redis client not initialized")
	}

	return cs.client.FlushDB(ctx).Err()
}

// BuildKey constructs a cache key with namespace
func BuildKey(namespace, id string) string {
	return fmt.Sprintf("%s:%s", namespace, id)
}

// BuildListKey constructs a cache key for list queries with filters
func BuildListKey(namespace string, filters map[string]string) string {
	key := namespace
	for k, v := range filters {
		key += fmt.Sprintf(":%s=%s", k, v)
	}
	return key
}
