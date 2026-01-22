package handlers

import (
"context"
"fmt"
"time"

"github.com/gofiber/fiber/v2"
"github.com/opendefender/openrisk/internal/cache"
)

// CacheConfig holds cache configuration for handlers
type CacheConfig struct {
RiskCacheTTL       time.Duration
DashboardCacheTTL  time.Duration
ConnectorCacheTTL  time.Duration
MarketplaceAppTTL  time.Duration
}

// DefaultCacheConfig returns default cache TTLs
func DefaultCacheConfig() CacheConfig {
return CacheConfig{
RiskCacheTTL:      5 * time.Minute,
DashboardCacheTTL: 10 * time.Minute,
ConnectorCacheTTL: 15 * time.Minute,
MarketplaceAppTTL: 20 * time.Minute,
}
}

// CacheableHandlers provides cache-enabled handler wrapper
type CacheableHandlers struct {
cache       *cache.Cache
cacheConfig CacheConfig
decoration  *cache.CacheDecoration
}

// NewCacheableHandlers creates a new caching handler wrapper
func NewCacheableHandlers(cacheInstance *cache.Cache) *CacheableHandlers {
return &CacheableHandlers{
cache:       cacheInstance,
cacheConfig: DefaultCacheConfig(),
decoration:  cache.NewCacheDecoration(cacheInstance),
}
}

// SetCacheConfig updates cache TTL configuration
func (ch *CacheableHandlers) SetCacheConfig(cfg CacheConfig) {
ch.cacheConfig = cfg
}

// ============================================================================
// RISK HANDLER CACHE HELPERS
// ============================================================================

// GetRiskCacheKey generates cache key for risk operations
func (ch *CacheableHandlers) GetRiskCacheKey(operation string, params ...string) string {
key := fmt.Sprintf("risk:%s", operation)
for _, param := range params {
key += fmt.Sprintf(":%s", param)
}
return key
}

// InvalidateRiskCaches invalidates all risk-related cache entries
func (ch *CacheableHandlers) InvalidateRiskCaches(ctx context.Context) error {
return ch.decoration.BatchInvalidate(ctx,
"risk:*",        // All risk entries
"report:*",      // Reports depend on risks
"dashboard:*",   // Dashboard depends on risks
)
}

// InvalidateSpecificRisk invalidates cache for a specific risk
func (ch *CacheableHandlers) InvalidateSpecificRisk(ctx context.Context, riskID string) error {
return ch.decoration.BatchInvalidate(ctx,
fmt.Sprintf("risk:id:%s", riskID),
"risk:list:*",
"risk:search:*",
"report:*",
"dashboard:*",
)
}

// CacheRiskListGET wraps a GET risk list handler with caching
func (ch *CacheableHandlers) CacheRiskListGET(handler fiber.Handler) fiber.Handler {
return ch.decoration.WrapWithCache(
handler,
func(c *fiber.Ctx) string {
page := c.Query("page", "1")
severity := c.Query("severity", "")
status := c.Query("status", "")
return fmt.Sprintf("risk:list:page:%s:sev:%s:status:%s", page, severity, status)
},
ch.cacheConfig.RiskCacheTTL,
)
}

// CacheRiskSearchGET wraps a GET risk search handler with caching
func (ch *CacheableHandlers) CacheRiskSearchGET(handler fiber.Handler) fiber.Handler {
return ch.decoration.WrapWithCache(
handler,
func(c *fiber.Ctx) string {
query := c.Query("q", "")
return fmt.Sprintf("risk:search:%s", hashQuery(query))
},
ch.cacheConfig.RiskCacheTTL,
)
}

// CacheRiskGetByIDGET wraps a GET risk by ID handler with caching
func (ch *CacheableHandlers) CacheRiskGetByIDGET(handler fiber.Handler) fiber.Handler {
return ch.decoration.WrapWithCache(
handler,
func(c *fiber.Ctx) string {
riskID := c.Params("id")
return fmt.Sprintf("risk:id:%s", riskID)
},
ch.cacheConfig.RiskCacheTTL,
)
}

// ============================================================================
// DASHBOARD HANDLER CACHE HELPERS
// ============================================================================

// GetDashboardCacheKey generates cache key for dashboard operations
func (ch *CacheableHandlers) GetDashboardCacheKey(operation string, params ...string) string {
key := fmt.Sprintf("dashboard:%s", operation)
for _, param := range params {
key += fmt.Sprintf(":%s", param)
}
return key
}

// InvalidateDashboardCaches invalidates all dashboard cache entries
func (ch *CacheableHandlers) InvalidateDashboardCaches(ctx context.Context) error {
return ch.cache.DeletePattern(ctx, "dashboard:*")
}

// CacheDashboardStatsGET wraps a GET dashboard stats handler with caching
func (ch *CacheableHandlers) CacheDashboardStatsGET(handler fiber.Handler) fiber.Handler {
return ch.decoration.WrapWithCache(
handler,
func(c *fiber.Ctx) string {
period := c.Query("period", "month")
return fmt.Sprintf("dashboard:stats:%s", period)
},
ch.cacheConfig.DashboardCacheTTL,
)
}

// CacheDashboardMatrixGET wraps a GET dashboard matrix handler with caching
func (ch *CacheableHandlers) CacheDashboardMatrixGET(handler fiber.Handler) fiber.Handler {
return ch.decoration.WrapWithCache(
handler,
func(c *fiber.Ctx) string {
return "dashboard:matrix:all"
},
ch.cacheConfig.DashboardCacheTTL,
)
}

// CacheDashboardTimelineGET wraps a GET dashboard timeline handler with caching
func (ch *CacheableHandlers) CacheDashboardTimelineGET(handler fiber.Handler) fiber.Handler {
return ch.decoration.WrapWithCache(
handler,
func(c *fiber.Ctx) string {
days := c.Query("days", "30")
return fmt.Sprintf("dashboard:timeline:%s", days)
},
ch.cacheConfig.DashboardCacheTTL,
)
}

// ============================================================================
// MARKETPLACE HANDLER CACHE HELPERS
// ============================================================================

// GetMarketplaceCacheKey generates cache key for marketplace operations
func (ch *CacheableHandlers) GetMarketplaceCacheKey(operation string, params ...string) string {
key := fmt.Sprintf("connector:%s", operation)
for _, param := range params {
key += fmt.Sprintf(":%s", param)
}
return key
}

// InvalidateMarketplaceCaches invalidates all marketplace cache entries
func (ch *CacheableHandlers) InvalidateMarketplaceCaches(ctx context.Context) error {
return ch.decoration.BatchInvalidate(ctx,
"connector:*",
"marketplace:app:*",
)
}

// InvalidateSpecificConnector invalidates cache for a specific connector
func (ch *CacheableHandlers) InvalidateSpecificConnector(ctx context.Context, connectorID string) error {
return ch.decoration.BatchInvalidate(ctx,
fmt.Sprintf("connector:id:%s", connectorID),
"connector:list:*",
"connector:health",
)
}

// CacheConnectorListGET wraps a GET connector list handler with caching
func (ch *CacheableHandlers) CacheConnectorListGET(handler fiber.Handler) fiber.Handler {
return ch.decoration.WrapWithCache(
handler,
func(c *fiber.Ctx) string {
category := c.Query("category", "all")
status := c.Query("status", "all")
return fmt.Sprintf("connector:list:cat:%s:status:%s", category, status)
},
ch.cacheConfig.ConnectorCacheTTL,
)
}

// CacheConnectorGetByIDGET wraps a GET connector by ID handler with caching
func (ch *CacheableHandlers) CacheConnectorGetByIDGET(handler fiber.Handler) fiber.Handler {
return ch.decoration.WrapWithCache(
handler,
func(c *fiber.Ctx) string {
connectorID := c.Params("id")
return fmt.Sprintf("connector:id:%s", connectorID)
},
ch.cacheConfig.ConnectorCacheTTL,
)
}

// CacheMarketplaceAppGetByIDGET wraps a GET app by ID handler with caching
func (ch *CacheableHandlers) CacheMarketplaceAppGetByIDGET(handler fiber.Handler) fiber.Handler {
return ch.decoration.WrapWithCache(
handler,
func(c *fiber.Ctx) string {
appID := c.Params("id")
return fmt.Sprintf("marketplace:app:%s", appID)
},
ch.cacheConfig.MarketplaceAppTTL,
)
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// hashQuery creates a hash of a query string for cache key
func hashQuery(query string) string {
if query == "" {
return "empty"
}
// Simple hash for cache key (can use MD5 for collision resistance)
h := 0
for _, c := range query {
h = ((h << 5) - h) + int(c)
}
return fmt.Sprintf("%x", h)
}

// ============================================================================
// CONTEXT CACHE HELPERS (for inline handler caching)
// ============================================================================

// GetOrSetRiskData provides cache-or-compute pattern for risk data
func (ch *CacheableHandlers) GetOrSetRiskData(ctx context.Context, key string, dest interface{}, compute func() (interface{}, error)) error {
cacheCtx := cache.NewRequestCacheContext(ch.cache, ctx)
return cacheCtx.GetOrSet(key, dest, compute)
}

// GetOrSetDashboardData provides cache-or-compute pattern for dashboard data
func (ch *CacheableHandlers) GetOrSetDashboardData(ctx context.Context, key string, dest interface{}, compute func() (interface{}, error)) error {
cacheCtx := cache.NewRequestCacheContext(ch.cache, ctx)
return cacheCtx.GetOrSet(key, dest, compute)
}

// GetOrSetMarketplaceData provides cache-or-compute pattern for marketplace data
func (ch *CacheableHandlers) GetOrSetMarketplaceData(ctx context.Context, key string, dest interface{}, compute func() (interface{}, error)) error {
cacheCtx := cache.NewRequestCacheContext(ch.cache, ctx)
return cacheCtx.GetOrSet(key, dest, compute)
}
