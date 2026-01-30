 Phase : Caching Integration Guide

 Overview
This document outlines how to integrate the cache layer into your API handlers to improve performance. The caching infrastructure has been implemented in three layers:

. Cache Middleware (internal/cache/middleware.go) - Generic middleware for any handler
. Cache Integration Helpers (internal/handlers/cache_integration.go) - Handler-specific utilities
. Monitoring Stack - Docker Compose with Prometheus + Grafana

 Quick Start: -Step Integration

 Step : Import Cache Package
go
import (
    "github.com/opendefender/openrisk/internal/cache"
)


 Step : Initialize CacheableHandlers in main.go
In your cmd/server/main.go, after initializing services:

go
// After database initialization and service setup
cacheInstance := cache.NewRedisCache(
    cfg.Redis.Host,
    cfg.Redis.Port,
    cfg.Redis.Password,
)
cacheableHandlers := handlers.NewCacheableHandlers(cacheInstance)


 Step : Wrap Your Route Handlers

 Before (without caching):
go
protected.Get("/risks", handlers.GetRisks)


 After (with caching):
go
protected.Get("/risks", cacheableHandlers.CacheRiskListGET(handlers.GetRisks))


 Endpoint-Specific Integration

 Risk Endpoints

 List Risks (High-Impact - Frequently accessed)
go
// Caches by: page, severity, status, search query
protected.Get("/risks", 
    cacheableHandlers.CacheRiskListGET(handlers.GetRisks))

// TTL:  minutes (configurable via CacheConfig.RiskCacheTTL)
// Cache Keys: risk:list:page=:severity=high:status=open


 Get Risk by ID
go
// Caches individual risk details
protected.Get("/risks/:id",
    cacheableHandlers.CacheRiskGetByIDGET(handlers.GetRisk))

// TTL:  minutes
// Cache Keys: risk:id:{riskID}


 Search Risks (High-Impact - Complex query)
go
// Caches search results with MD hash of query
api.Get("/risks/search",
    cacheableHandlers.CacheRiskSearchGET(handlers.SearchRisks))

// TTL:  minutes
// Cache Keys: risk:search:{mdhash of query string}


 Create/Update/Delete Risks (Invalidation)
go
// These mutations invalidate related caches automatically
protected.Post("/risks", 
    riskCreate, 
    cacheableHandlers.CacheInvalidationMiddleware(), // Optional pre-check
    handlers.CreateRisk)

protected.Patch("/risks/:id", 
    riskUpdate,
    cacheableHandlers.CacheInvalidationMiddleware(),
    handlers.UpdateRisk)

protected.Delete("/risks/:id",
    riskDelete,
    cacheableHandlers.CacheInvalidationMiddleware(),
    handlers.DeleteRisk)


Automatic Cache Invalidation:
- Creating a risk → Invalidates: risk:list:, dashboard:stats:
- Updating a risk → Invalidates: risk:id:{id}, risk:list:, dashboard:
- Deleting a risk → Invalidates: risk:id:{id}, risk:list:, dashboard:, report:

 Dashboard Endpoints

 Dashboard Statistics (High-Impact)
go
// Caches statistics by period (h, d, d, etc.)
protected.Get("/stats",
    cacheableHandlers.CacheDashboardStatsGET(handlers.GetDashboardStats))

// TTL:  minutes
// Cache Keys: dashboard:stats:period=h, dashboard:stats:period=d


 Risk Matrix (Static Data)
go
// Caches risk matrix visualization data
api.Get("/stats/risk-matrix",
    cacheableHandlers.CacheDashboardMatrixGET(handlers.GetRiskMatrixData))

// TTL:  minutes
// Cache Keys: dashboard:matrix


 Timeline Data
go
// Caches risk trend timeline
api.Get("/stats/trends",
    middleware.Protected(),
    cacheableHandlers.CacheDashboardTimelineGET(handlers.GetGlobalRiskTrend))

// TTL:  minutes
// Cache Keys: dashboard:timeline:days=


 Marketplace/Connector Endpoints

 List Connectors
go
// Caches with category and status filters
api.Get("/marketplace/connectors",
    cacheableHandlers.CacheConnectorListGET(handlers.ListConnectors))

// TTL:  minutes
// Cache Keys: marketplace:connectors:list:category=all:status=all


 Get Connector by ID
go
// Caches individual connector details
api.Get("/marketplace/connectors/:id",
    cacheableHandlers.CacheConnectorGetByIDGET(handlers.GetConnectorByID))

// TTL:  minutes
// Cache Keys: marketplace:connector:{connectorID}


 Get Marketplace App
go
// Caches app metadata (rarely changes)
api.Get("/marketplace/apps/:id",
    cacheableHandlers.CacheMarketplaceAppGetByIDGET(handlers.GetMarketplaceApp))

// TTL:  minutes (longest TTL for stable data)
// Cache Keys: marketplace:app:{appID}


 Cache Configuration

 Default TTLs (in cache_integration.go)
go
CacheConfig{
    RiskCacheTTL:             time.Minute,        // Risk data changes frequently
    DashboardCacheTTL:        time.Minute,       // Stats are aggregate/slower to compute
    ConnectorCacheTTL:        time.Minute,       // Connectors relatively stable
    MarketplaceAppTTL:        time.Minute,       // App metadata very stable
}


 Customize TTLs
go
// In main.go
cacheConfig := handlers.CacheConfig{
    RiskCacheTTL:             time.Minute,        // More aggressive for real-time data
    DashboardCacheTTL:        time.Minute,        
    ConnectorCacheTTL:        time.Minute,       
    MarketplaceAppTTL:        time.Minute,       
}
cacheableHandlers.Config = cacheConfig


 Manual Cache Invalidation

When you need to invalidate caches programmatically:

go
// Invalidate all risk-related caches
cacheableHandlers.InvalidateRiskCaches(ctx)

// Invalidate specific risk and cascade to reports/dashboards
cacheableHandlers.InvalidateSpecificRisk(ctx, riskID)

// Invalidate all dashboard caches
cacheableHandlers.InvalidateDashboardCaches(ctx)

// Invalidate all marketplace caches
cacheableHandlers.InvalidateMarketplaceCaches(ctx)


 Example: Full Handler Integration

 Before (No Caching)
go
// File: cmd/server/main.go
protected.Get("/risks", 
    middleware.RequirePermissions(permissionService, domain.Permission{
        Resource: domain.PermissionResourceRisk,
        Action:   domain.PermissionRead,
    }),
    handlers.GetRisks)


 After (With Caching)
go
// File: cmd/server/main.go
import (
    "github.com/opendefender/openrisk/internal/cache"
)

func main() {
    // ... existing setup ...
    
    // Initialize cache (after database setup)
    cacheInstance := cache.NewRedisCache(
        cfg.Redis.Host,
        cfg.Redis.Port,
        cfg.Redis.Password,
    )
    cacheableHandlers := handlers.NewCacheableHandlers(cacheInstance)
    
    // ... route setup ...
    
    protected.Get("/risks", 
        middleware.RequirePermissions(permissionService, domain.Permission{
            Resource: domain.PermissionResourceRisk,
            Action:   domain.PermissionRead,
        }),
        cacheableHandlers.CacheRiskListGET(handlers.GetRisks))
}


 Monitoring Your Cache

 Start the Monitoring Stack
bash
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d


 Access Dashboards
- Grafana: http://localhost: (admin/admin)
- Prometheus: http://localhost:

 Key Metrics to Monitor

. Cache Hit Rate (Target: > %)
   - Dashboard: "Cache Hit Ratio" pie chart
   - Alert: Fires if < % for  minutes

. Response Time (Target: P < ms)
   - Dashboard: "Database Query Performance" line chart
   - Shows reduction in query time with caching

. Redis Memory (Target: < %)
   - Dashboard: "Redis Memory Usage" line chart
   - Alert: Fires if > % for  minutes

. Database Connections (Target: < )
   - Dashboard: "PostgreSQL Active Connections" stat
   - Alert: Fires if >  for  minutes

 Testing Your Integration

 . Start Services
bash
 Terminal : Backend with cache
go run ./cmd/server/main.go

 Terminal : Start monitoring
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d

 Terminal : Run k load tests
k run ./load_tests/cache_test.js


 . Verify Cache Hits
bash
 Watch real-time cache metrics
watch 'curl -s http://localhost:/api/v/query?query=redis_hits_total | jq'


 . Expected Results
- Without caching: ~ms per request,  cache hits
- With caching: ~ms per request (warmed cache), %+ hit rate
- Improvement: % reduction in response time

 Troubleshooting

 Cache Misses Too High (< %)
Cause: Cache TTL too short or filters changing too much
Solution: 
- Increase TTL for that endpoint
- Check if query parameters are changing (causes different cache keys)

 High Memory Usage (> %)
Cause: Too much data cached or TTL too long
Solution:
- Reduce TTL (min → min)
- Add cache size limits in Redis config
- Check for query parameter explosion (pagination differences)

 Cache Not Invalidating on Mutations
Cause: Forgot to wrap the POST/PUT/DELETE handler
Solution:
- Add cacheableHandlers.CacheInvalidationMiddleware() to mutation endpoints
- Verify InvalidateRiskCaches() is being called

 Redis Connection Failed
Cause: Redis container not running or wrong credentials
Solution:
bash
 Restart monitoring stack
cd deployment
docker-compose -f docker-compose-monitoring.yaml restart redis

 Check credentials in .env
echo $REDIS_PASSWORD


 Performance Targets

 Phase  Objectives
- [ ] Cache hit rate: > %
- [ ] Response time P: < ms (vs ms baseline)
- [ ] Throughput: >  req/s (vs  req/s baseline)
- [ ] Redis memory: < MB for typical workload
- [ ] Database connections: <  (vs  baseline)

 Load Test Command
bash
  virtual users,  minute test
k run \
  --vus  \
  --duration m \
  ./load_tests/cache_test.js


 Next Steps

. Integrate handlers - Apply caching to risk/dashboard/marketplace endpoints
. Test performance - Run k load tests and verify metrics in Grafana
. Optimize TTLs - Adjust caching durations based on hit rate metrics
. Document results - Update performance benchmarks in README
. Deploy to staging - Test with realistic workload before production

 References

- [Cache Middleware Code](../backend/internal/cache/middleware.go)
- [Cache Integration Code](../backend/internal/handlers/cache_integration.go)
- [Load Testing Guide](./LOAD_TESTING_GUIDE.md)
- [Monitoring Dashboard](../deployment/docker-compose-monitoring.yaml)
