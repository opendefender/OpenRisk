# Cache Integration Guide - Phase 5 Priority #4

## Overview

This guide provides detailed instructions for integrating Redis caching into OpenRisk handlers and endpoints. The caching layer is transparent, optional, and gracefully degrades if Redis is unavailable.

## Middleware Types

### 1. CacheMiddleware - Simple Response Caching
- Caches all GET request responses with TTL
- Uses path + query params for cache key
- Sets `X-Cache: HIT` or `X-Cache: MISS` header

### 2. QueryCacheMiddleware - Advanced with Invalidation
- Selective cache invalidation on mutations
- Pattern-based invalidation (risk:*, report:*, etc.)
- Automatic cache miss on POST/PUT/DELETE

### 3. RequestCacheContext - Handler-Level Operations
- Cache operations within handler business logic
- GetOrSet pattern for cache-or-compute
- Manual invalidation after mutations

### 4. CacheDecoration - Decorator Pattern
- Wrap individual handlers with caching logic
- Custom cache key generation function
- Transparent to business logic

## Integration Patterns

### Pattern 1: Global Cache Middleware
```go
app.Use(cache.CacheMiddleware(redisCache, 5 * time.Minute))
// Applies to all GET endpoints automatically
```

### Pattern 2: Selective Middleware with Invalidation
```go
queryCacheMW := cache.NewQueryCacheMiddleware(redisCache, 10 * time.Minute)
queryCacheMW.RegisterInvalidator("create_risk", "risk:*", "report:*")
app.Use(queryCacheMW.Handler())
```

### Pattern 3: Handler-Level Caching
```go
cacheCtx := cache.NewRequestCacheContext(redisCache, c.Context())
err := cacheCtx.GetOrSet(cacheKey, &data, func() (interface{}, error) {
    return h.service.FetchData()
})
// Invalidate after mutations
cacheCtx.InvalidatePattern("risk:*")
```

## Cache Keys Organization

```
Risk Endpoints:
  - risk:id:{id}
  - risk:list:page:{p}
  - risk:search:{query}

Dashboard:
  - dashboard:stats:month
  - dashboard:matrix:all

Marketplace:
  - connector:list:category:{cat}
  - marketplace:app:{id}

Reports:
  - report:list:type:{type}
```

## Environment Variables

```bash
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_TTL_SECONDS=300
```

## Expected Performance Improvements

| Metric | Before | After | Improvement |
|--------|--------|-------|------------|
| Response Time (P95) | 500ms | 50ms | **10x faster** |
| Database Load | 100% | 30% | **70% reduction** |
| Throughput | 1K req/s | 5K+ req/s | **5x improvement** |

## Troubleshooting

**Check Redis Connection**:
```bash
redis-cli ping  # Should return PONG
```

**Check X-Cache Headers**:
```bash
curl -i http://localhost:3000/api/risks  # Look for X-Cache: HIT
```

**Monitor Cache Hit Rate**: See GRAFANA_DASHBOARDS.md

## Next Steps

1. Apply middleware to risk endpoints
2. Monitor cache hit rates for 24 hours
3. Adjust TTLs based on data freshness
4. Add selective invalidation for mutations
5. Scale Redis for production workload
