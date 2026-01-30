package cache

import (
"context"
"crypto/md"
"fmt"
"time"

"github.com/gofiber/fiber/v"
)

// CacheMiddleware creates middleware that caches GET request responses
func CacheMiddleware(cache Cache, duration time.Duration) fiber.Handler {
return func(c fiber.Ctx) error {
// Only cache GET requests
if c.Method() != fiber.MethodGet {
return c.Next()
}

// Generate cache key from request path and query params
queryString := string(c.Request().URI().QueryString())
cacheKey := generateCacheKey(c.Path(), queryString)

// Try to get from cache
cachedResponse, err := cache.GetString(c.Context(), cacheKey)
if err == nil {
c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
c.Set("X-Cache", "HIT")
return c.SendString(cachedResponse)
}

// Process request
if err := c.Next(); err != nil {
return err
}

// Cache successful GET responses
responseBody := string(c.Response().Body())
if c.Response().StatusCode() == fiber.StatusOK && responseBody != "" {
if err := cache.SetWithTTL(context.Background(), cacheKey, responseBody, duration); err != nil {
fmt.Printf("failed to cache response: %v\n", err)
}
}

c.Set("X-Cache", "MISS")
return nil
}
}

// QueryCacheMiddleware provides cache with invalidation support
type QueryCacheMiddleware struct {
cache        Cache
ttl          time.Duration
invalidators map[string][]string
}

// NewQueryCacheMiddleware creates query-specific cache middleware
func NewQueryCacheMiddleware(cache Cache, ttl time.Duration) QueryCacheMiddleware {
return &QueryCacheMiddleware{
cache:        cache,
ttl:          ttl,
invalidators: make(map[string][]string),
}
}

// RegisterInvalidator registers cache patterns to invalidate on specific operations
func (qcm QueryCacheMiddleware) RegisterInvalidator(operation string, patterns ...string) {
qcm.invalidators[operation] = patterns
}

// Handler returns the middleware handler
func (qcm QueryCacheMiddleware) Handler() fiber.Handler {
return func(c fiber.Ctx) error {
// Handle cache invalidation for mutations
if c.Method() != fiber.MethodGet {
patterns, ok := qcm.invalidators[c.Route().Name]
if ok {
ctx := context.Background()
for _, pattern := range patterns {
if err := qcm.cache.DeletePattern(ctx, pattern); err != nil {
fmt.Printf("failed to invalidate cache pattern %s: %v\n", pattern, err)
}
}
}
return c.Next()
}

// Cache GET requests
queryString := string(c.Request().URI().QueryString())
cacheKey := generateCacheKey(c.Path(), queryString)

if cached, err := qcm.cache.GetString(c.Context(), cacheKey); err == nil {
c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
c.Set("X-Cache", "HIT")
return c.SendString(cached)
}

return c.Next()
}
}

// InvalidatePattern manually invalidates cache patterns
func (qcm QueryCacheMiddleware) InvalidatePattern(ctx context.Context, pattern string) error {
return qcm.cache.DeletePattern(ctx, pattern)
}

// generateCacheKey creates a unique cache key from path and query params
func generateCacheKey(path string, query string) string {
if query == "" {
return "http:" + path
}
hash := md.Sum([]byte(path + "?" + query))
return fmt.Sprintf("http:%s:%x", path, hash)
}

// RequestCacheContext extends fiber context with cache helpers
type RequestCacheContext struct {
cache Cache
ctx   context.Context
}

// NewRequestCacheContext creates a request-scoped cache context
func NewRequestCacheContext(cache Cache, ctx context.Context) RequestCacheContext {
return &RequestCacheContext{cache: cache, ctx: ctx}
}

// GetOrSet gets a value from cache or sets it
func (rcc RequestCacheContext) GetOrSet(key string, dest interface{}, compute func() (interface{}, error)) error {
// Try to get from cache first
err := rcc.cache.Get(rcc.ctx, key, dest)
if err == nil {
return nil
}

// Compute value if not cached
value, err := compute()
if err != nil {
return err
}

// Set in cache
return rcc.cache.Set(rcc.ctx, key, value)
}

// Invalidate invalidates specific cache keys
func (rcc RequestCacheContext) Invalidate(keys ...string) error {
return rcc.cache.Delete(rcc.ctx, keys...)
}

// InvalidatePattern invalidates cache by pattern
func (rcc RequestCacheContext) InvalidatePattern(pattern string) error {
return rcc.cache.DeletePattern(rcc.ctx, pattern)
}

// CacheDecoration provides utility methods for caching in handlers
type CacheDecoration struct {
cache Cache
}

// NewCacheDecoration creates cache decoration utility
func NewCacheDecoration(cache Cache) CacheDecoration {
return &CacheDecoration{cache: cache}
}

// WrapWithCache wraps a handler with caching
func (cd CacheDecoration) WrapWithCache(
handler fiber.Handler,
cacheKeyFunc func(c fiber.Ctx) string,
ttl time.Duration,
) fiber.Handler {
return func(c fiber.Ctx) error {
if c.Method() != fiber.MethodGet {
return handler(c)
}

cacheKey := cacheKeyFunc(c)
if cached, err := cd.cache.GetString(c.Context(), cacheKey); err == nil {
c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
c.Set("X-Cache", "HIT")
return c.SendString(cached)
}

if err := handler(c); err != nil {
return err
}

responseBody := string(c.Response().Body())
if c.Response().StatusCode() == fiber.StatusOK && responseBody != "" {
if err := cd.cache.SetWithTTL(context.Background(), cacheKey, responseBody, ttl); err != nil {
fmt.Printf("cache write failed: %v\n", err)
}
}

c.Set("X-Cache", "MISS")
return nil
}
}

// BatchInvalidate invalidates multiple patterns at once
func (cd CacheDecoration) BatchInvalidate(ctx context.Context, patterns ...string) error {
for _, pattern := range patterns {
if err := cd.cache.DeletePattern(ctx, pattern); err != nil {
return fmt.Errorf("failed to invalidate pattern %s: %w", pattern, err)
}
}
return nil
}
