 Cache Integration Guide - Phase  Priority 

 Overview

This guide provides detailed instructions for integrating Redis caching into OpenRisk handlers and endpoints. The caching layer is transparent, optional, and gracefully degrades if Redis is unavailable.

 Middleware Types

 . CacheMiddleware - Simple Response Caching
- Caches all GET request responses with TTL
- Uses path + query params for cache key
- Sets X-Cache: HIT or X-Cache: MISS header

 . QueryCacheMiddleware - Advanced with Invalidation
- Selective cache invalidation on mutations
- Pattern-based invalidation (risk:, report:, etc.)
- Automatic cache miss on POST/PUT/DELETE

 . RequestCacheContext - Handler-Level Operations
- Cache operations within handler business logic
- GetOrSet pattern for cache-or-compute
- Manual invalidation after mutations

 . CacheDecoration - Decorator Pattern
- Wrap individual handlers with caching logic
- Custom cache key generation function
- Transparent to business logic

 Integration Patterns

 Pattern : Global Cache Middleware
go
app.Use(cache.CacheMiddleware(redisCache,   time.Minute))
// Applies to all GET endpoints automatically


 Pattern : Selective Middleware with Invalidation
go
queryCacheMW := cache.NewQueryCacheMiddleware(redisCache,   time.Minute)
queryCacheMW.RegisterInvalidator("create_risk", "risk:", "report:")
app.Use(queryCacheMW.Handler())


 Pattern : Handler-Level Caching
go
cacheCtx := cache.NewRequestCacheContext(redisCache, c.Context())
err := cacheCtx.GetOrSet(cacheKey, &data, func() (interface{}, error) {
    return h.service.FetchData()
})
// Invalidate after mutations
cacheCtx.InvalidatePattern("risk:")


 Cache Keys Organization


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


 Environment Variables

bash
REDIS_HOST=localhost
REDIS_PORT=
REDIS_PASSWORD=
REDIS_DB=
REDIS_TTL_SECONDS=


 Expected Performance Improvements

| Metric | Before | After | Improvement |
|--------|--------|-------|------------|
| Response Time (P) | ms | ms | x faster |
| Database Load | % | % | % reduction |
| Throughput | K req/s | K+ req/s | x improvement |

 Troubleshooting

Check Redis Connection:
bash
redis-cli ping   Should return PONG


Check X-Cache Headers:
bash
curl -i http://localhost:/api/risks   Look for X-Cache: HIT


Monitor Cache Hit Rate: See GRAFANA_DASHBOARDS.md

 Next Steps

. Apply middleware to risk endpoints
. Monitor cache hit rates for  hours
. Adjust TTLs based on data freshness
. Add selective invalidation for mutations
. Scale Redis for production workload
