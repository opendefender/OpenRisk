package cache

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"sync"
	"time"
)

// CacheEntry represents a cached value with metadata
type CacheEntry struct {
	Key         string
	Value       interface{}
	CreatedAt   time.Time
	ExpiresAt   time.Time
	TTL         time.Duration
	AccessCount int64
	LastAccess  time.Time
	Size        int64
}

// IsExpired checks if the cache entry has expired
func (ce *CacheEntry) IsExpired() bool {
	return time.Now().After(ce.ExpiresAt)
}

// CachePolicy defines the cache eviction policy
type CachePolicy string

const (
	LRU  CachePolicy = "LRU"  // Least Recently Used
	LFU  CachePolicy = "LFU"  // Least Frequently Used
	FIFO CachePolicy = "FIFO" // First In First Out
	TTL  CachePolicy = "TTL"  // Time To Live
)

// AdvancedCache is a high-performance caching system with multiple strategies
type AdvancedCache struct {
	mu              sync.RWMutex
	entries         map[string]*CacheEntry
	maxSize         int64
	currentSize     int64
	policy          CachePolicy
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	stats           *CacheStats
	compression     bool
}

// CacheStats tracks cache statistics
type CacheStats struct {
	Hits            int64
	Misses          int64
	Evictions       int64
	ExpirationCount int64
	CurrentEntries  int64
	CurrentSize     int64
	AvgAccessTime   time.Duration
	HitRate         float64
}

// NewAdvancedCache creates a new advanced cache instance
func NewAdvancedCache(maxSize int64, policy CachePolicy, defaultTTL time.Duration) *AdvancedCache {
	cache := &AdvancedCache{
		entries:         make(map[string]*CacheEntry),
		maxSize:         maxSize,
		currentSize:     0,
		policy:          policy,
		defaultTTL:      defaultTTL,
		cleanupInterval: 1 * time.Minute,
		stats:           &CacheStats{},
		compression:     true,
	}

	// Start cleanup goroutine
	go cache.cleanupExpired()

	return cache
}

// Set stores a value in the cache
func (ac *AdvancedCache) Set(ctx context.Context, key string, value interface{}, ttl *time.Duration) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	// Use default TTL if not specified
	duration := ac.defaultTTL
	if ttl != nil {
		duration = *ttl
	}

	entry := &CacheEntry{
		Key:         key,
		Value:       value,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(duration),
		TTL:         duration,
		AccessCount: 0,
		LastAccess:  time.Now(),
		Size:        ac.estimateSize(value),
	}

	// Remove existing entry if present
	if existing, exists := ac.entries[key]; exists {
		ac.currentSize -= existing.Size
	}

	// Check if we need to evict entries
	if ac.currentSize+entry.Size > ac.maxSize {
		ac.evict(entry.Size)
	}

	ac.entries[key] = entry
	ac.currentSize += entry.Size
	ac.stats.CurrentSize = ac.currentSize
	ac.stats.CurrentEntries = int64(len(ac.entries))

	return nil
}

// Get retrieves a value from the cache
func (ac *AdvancedCache) Get(ctx context.Context, key string) (interface{}, bool) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	entry, exists := ac.entries[key]
	if !exists {
		ac.stats.Misses++
		ac.updateHitRate()
		return nil, false
	}

	if entry.IsExpired() {
		ac.currentSize -= entry.Size
		delete(ac.entries, key)
		ac.stats.Misses++
		ac.stats.ExpirationCount++
		ac.stats.CurrentEntries = int64(len(ac.entries))
		ac.stats.CurrentSize = ac.currentSize
		ac.updateHitRate()
		return nil, false
	}

	// Update access metadata
	entry.AccessCount++
	entry.LastAccess = time.Now()
	ac.stats.Hits++
	ac.updateHitRate()

	return entry.Value, true
}

// Delete removes a key from the cache
func (ac *AdvancedCache) Delete(ctx context.Context, key string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	if entry, exists := ac.entries[key]; exists {
		ac.currentSize -= entry.Size
		delete(ac.entries, key)
		ac.stats.CurrentEntries = int64(len(ac.entries))
		ac.stats.CurrentSize = ac.currentSize
	}
}

// Invalidate removes all keys matching a pattern
func (ac *AdvancedCache) Invalidate(ctx context.Context, pattern string) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	for key, entry := range ac.entries {
		if ac.matchPattern(key, pattern) {
			ac.currentSize -= entry.Size
			delete(ac.entries, key)
		}
	}
	ac.stats.CurrentEntries = int64(len(ac.entries))
	ac.stats.CurrentSize = ac.currentSize
}

// evict removes entries based on the configured policy
func (ac *AdvancedCache) evict(neededSpace int64) {
	switch ac.policy {
	case LRU:
		ac.evictLRU(neededSpace)
	case LFU:
		ac.evictLFU(neededSpace)
	case FIFO:
		ac.evictFIFO(neededSpace)
	case TTL:
		ac.evictTTL(neededSpace)
	}
}

// evictLRU evicts least recently used entries
func (ac *AdvancedCache) evictLRU(neededSpace int64) {
	freed := int64(0)
	var lruEntry *CacheEntry
	var lruKey string

	for key, entry := range ac.entries {
		if lruEntry == nil || entry.LastAccess.Before(lruEntry.LastAccess) {
			lruEntry = entry
			lruKey = key
		}
	}

	if lruEntry != nil {
		ac.currentSize -= lruEntry.Size
		delete(ac.entries, lruKey)
		freed += lruEntry.Size
		ac.stats.Evictions++

		if freed < neededSpace {
			ac.evictLRU(neededSpace - freed)
		}
	}
}

// evictLFU evicts least frequently used entries
func (ac *AdvancedCache) evictLFU(neededSpace int64) {
	var lfuEntry *CacheEntry
	var lfuKey string

	for key, entry := range ac.entries {
		if lfuEntry == nil || entry.AccessCount < lfuEntry.AccessCount {
			lfuEntry = entry
			lfuKey = key
		}
	}

	if lfuEntry != nil {
		ac.currentSize -= lfuEntry.Size
		delete(ac.entries, lfuKey)
		ac.stats.Evictions++

		if ac.currentSize+neededSpace > ac.maxSize {
			ac.evictLFU(neededSpace)
		}
	}
}

// evictFIFO evicts entries in first-in-first-out order
func (ac *AdvancedCache) evictFIFO(neededSpace int64) {
	var oldestEntry *CacheEntry
	var oldestKey string

	for key, entry := range ac.entries {
		if oldestEntry == nil || entry.CreatedAt.Before(oldestEntry.CreatedAt) {
			oldestEntry = entry
			oldestKey = key
		}
	}

	if oldestEntry != nil {
		ac.currentSize -= oldestEntry.Size
		delete(ac.entries, oldestKey)
		ac.stats.Evictions++

		if ac.currentSize+neededSpace > ac.maxSize {
			ac.evictFIFO(neededSpace)
		}
	}
}

// evictTTL evicts entries closest to expiration
func (ac *AdvancedCache) evictTTL(neededSpace int64) {
	var soonestEntry *CacheEntry
	var soonestKey string

	for key, entry := range ac.entries {
		if soonestEntry == nil || entry.ExpiresAt.Before(soonestEntry.ExpiresAt) {
			soonestEntry = entry
			soonestKey = key
		}
	}

	if soonestEntry != nil {
		ac.currentSize -= soonestEntry.Size
		delete(ac.entries, soonestKey)
		ac.stats.Evictions++

		if ac.currentSize+neededSpace > ac.maxSize {
			ac.evictTTL(neededSpace)
		}
	}
}

// cleanupExpired removes expired entries periodically
func (ac *AdvancedCache) cleanupExpired() {
	ticker := time.NewTicker(ac.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		ac.mu.Lock()
		now := time.Now()
		for key, entry := range ac.entries {
			if now.After(entry.ExpiresAt) {
				ac.currentSize -= entry.Size
				delete(ac.entries, key)
				ac.stats.ExpirationCount++
			}
		}
		ac.stats.CurrentEntries = int64(len(ac.entries))
		ac.stats.CurrentSize = ac.currentSize
		ac.mu.Unlock()
	}
}

// GetStats returns current cache statistics
func (ac *AdvancedCache) GetStats() *CacheStats {
	ac.mu.RLock()
	defer ac.mu.RUnlock()

	statsCopy := *ac.stats
	return &statsCopy
}

// updateHitRate calculates the current hit rate
func (ac *AdvancedCache) updateHitRate() {
	total := ac.stats.Hits + ac.stats.Misses
	if total > 0 {
		ac.stats.HitRate = float64(ac.stats.Hits) / float64(total)
	}
}

// estimateSize estimates the size of a value
func (ac *AdvancedCache) estimateSize(value interface{}) int64 {
	// In production, use a more sophisticated approach
	// This is a simplified estimate
	return 1024 // 1KB default estimate
}

// matchPattern checks if a key matches a pattern
func (ac *AdvancedCache) matchPattern(key, pattern string) bool {
	// Simple glob-style pattern matching
	if pattern == "*" {
		return true
	}
	if pattern == key {
		return true
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(key) >= len(prefix) && key[:len(prefix)] == prefix
	}
	return false
}

// GenerateCacheKey generates a cache key from components
func GenerateCacheKey(components ...string) string {
	key := ""
	for _, comp := range components {
		key += comp + ":"
	}

	// Create MD5 hash for shorter, consistent keys
	hash := md5.Sum([]byte(key))
	return hex.EncodeToString(hash[:])
}

// CacheWarmup preloads frequently accessed data into cache
type CacheWarmup struct {
	cache    *AdvancedCache
	preload  map[string]interface{}
	interval time.Duration
}

// NewCacheWarmup creates a new cache warmup utility
func NewCacheWarmup(cache *AdvancedCache, interval time.Duration) *CacheWarmup {
	return &CacheWarmup{
		cache:    cache,
		preload:  make(map[string]interface{}),
		interval: interval,
	}
}

// AddPreload adds a key-value pair to be preloaded
func (cw *CacheWarmup) AddPreload(key string, value interface{}) {
	cw.preload[key] = value
}

// Start begins the cache warmup process
func (cw *CacheWarmup) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initial warmup
	for key, value := range cw.preload {
		cw.cache.Set(ctx, key, value, nil)
	}

	// Periodic warmup
	ticker := time.NewTicker(cw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for key, value := range cw.preload {
				cw.cache.Set(ctx, key, value, nil)
			}
		}
	}
}
