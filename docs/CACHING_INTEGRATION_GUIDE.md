# Phase 5: Caching Integration Guide

## Overview
This document outlines how to integrate the cache layer into your API handlers to improve performance. The caching infrastructure has been implemented in three layers:

1. **Cache Middleware** (`internal/cache/middleware.go`) - Generic middleware for any handler
2. **Cache Integration Helpers** (`internal/handlers/cache_integration.go`) - Handler-specific utilities
3. **Monitoring Stack** - Docker Compose with Prometheus + Grafana

## Quick Start: 3-Step Integration

### Step 1: Import Cache Package
```go
import (
    "github.com/opendefender/openrisk/internal/cache"
)
```

### Step 2: Initialize CacheableHandlers in main.go
In your `cmd/server/main.go`, after initializing services:

```go
// After database initialization and service setup
cacheInstance := cache.NewRedisCache(
    cfg.Redis.Host,
    cfg.Redis.Port,
    cfg.Redis.Password,
)
cacheableHandlers := handlers.NewCacheableHandlers(cacheInstance)
```

### Step 3: Wrap Your Route Handlers

#### Before (without caching):
```go
protected.Get("/risks", handlers.GetRisks)
```

#### After (with caching):
```go
protected.Get("/risks", cacheableHandlers.CacheRiskListGET(handlers.GetRisks))
```

## Endpoint-Specific Integration

### Risk Endpoints

#### List Risks (High-Impact - Frequently accessed)
```go
// Caches by: page, severity, status, search query
protected.Get("/risks", 
    cacheableHandlers.CacheRiskListGET(handlers.GetRisks))

// TTL: 5 minutes (configurable via CacheConfig.RiskCacheTTL)
// Cache Keys: risk:list:page=1:severity=high:status=open
```

#### Get Risk by ID
```go
// Caches individual risk details
protected.Get("/risks/:id",
    cacheableHandlers.CacheRiskGetByIDGET(handlers.GetRisk))

// TTL: 5 minutes
// Cache Keys: risk:id:{riskID}
```

#### Search Risks (High-Impact - Complex query)
```go
// Caches search results with MD5 hash of query
api.Get("/risks/search",
    cacheableHandlers.CacheRiskSearchGET(handlers.SearchRisks))

// TTL: 5 minutes
// Cache Keys: risk:search:{md5hash of query string}
```

#### Create/Update/Delete Risks (Invalidation)
```go
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
```

**Automatic Cache Invalidation:**
- Creating a risk → Invalidates: `risk:list:*`, `dashboard:stats:*`
- Updating a risk → Invalidates: `risk:id:{id}`, `risk:list:*`, `dashboard:*`
- Deleting a risk → Invalidates: `risk:id:{id}`, `risk:list:*`, `dashboard:*`, `report:*`

### Dashboard Endpoints

#### Dashboard Statistics (High-Impact)
```go
// Caches statistics by period (24h, 7d, 30d, etc.)
protected.Get("/stats",
    cacheableHandlers.CacheDashboardStatsGET(handlers.GetDashboardStats))

// TTL: 10 minutes
// Cache Keys: dashboard:stats:period=24h, dashboard:stats:period=7d
```

#### Risk Matrix (Static Data)
```go
// Caches risk matrix visualization data
api.Get("/stats/risk-matrix",
    cacheableHandlers.CacheDashboardMatrixGET(handlers.GetRiskMatrixData))

// TTL: 10 minutes
// Cache Keys: dashboard:matrix
```

#### Timeline Data
```go
// Caches risk trend timeline
api.Get("/stats/trends",
    middleware.Protected(),
    cacheableHandlers.CacheDashboardTimelineGET(handlers.GetGlobalRiskTrend))

// TTL: 10 minutes
// Cache Keys: dashboard:timeline:days=30
```

### Marketplace/Connector Endpoints

#### List Connectors
```go
// Caches with category and status filters
api.Get("/marketplace/connectors",
    cacheableHandlers.CacheConnectorListGET(handlers.ListConnectors))

// TTL: 15 minutes
// Cache Keys: marketplace:connectors:list:category=all:status=all
```

#### Get Connector by ID
```go
// Caches individual connector details
api.Get("/marketplace/connectors/:id",
    cacheableHandlers.CacheConnectorGetByIDGET(handlers.GetConnectorByID))

// TTL: 15 minutes
// Cache Keys: marketplace:connector:{connectorID}
```

#### Get Marketplace App
```go
// Caches app metadata (rarely changes)
api.Get("/marketplace/apps/:id",
    cacheableHandlers.CacheMarketplaceAppGetByIDGET(handlers.GetMarketplaceApp))

// TTL: 20 minutes (longest TTL for stable data)
// Cache Keys: marketplace:app:{appID}
```

## Cache Configuration

### Default TTLs (in `cache_integration.go`)
```go
CacheConfig{
    RiskCacheTTL:           5 * time.Minute,        // Risk data changes frequently
    DashboardCacheTTL:      10 * time.Minute,       // Stats are aggregate/slower to compute
    ConnectorCacheTTL:      15 * time.Minute,       // Connectors relatively stable
    MarketplaceAppTTL:      20 * time.Minute,       // App metadata very stable
}
```

### Customize TTLs
```go
// In main.go
cacheConfig := handlers.CacheConfig{
    RiskCacheTTL:           3 * time.Minute,        // More aggressive for real-time data
    DashboardCacheTTL:      5 * time.Minute,        
    ConnectorCacheTTL:      30 * time.Minute,       
    MarketplaceAppTTL:      60 * time.Minute,       
}
cacheableHandlers.Config = cacheConfig
```

## Manual Cache Invalidation

When you need to invalidate caches programmatically:

```go
// Invalidate all risk-related caches
cacheableHandlers.InvalidateRiskCaches(ctx)

// Invalidate specific risk and cascade to reports/dashboards
cacheableHandlers.InvalidateSpecificRisk(ctx, riskID)

// Invalidate all dashboard caches
cacheableHandlers.InvalidateDashboardCaches(ctx)

// Invalidate all marketplace caches
cacheableHandlers.InvalidateMarketplaceCaches(ctx)
```

## Example: Full Handler Integration

### Before (No Caching)
```go
// File: cmd/server/main.go
protected.Get("/risks", 
    middleware.RequirePermissions(permissionService, domain.Permission{
        Resource: domain.PermissionResourceRisk,
        Action:   domain.PermissionRead,
    }),
    handlers.GetRisks)
```

### After (With Caching)
```go
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
```

## Monitoring Your Cache

### Start the Monitoring Stack
```bash
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d
```

### Access Dashboards
- **Grafana**: http://localhost:3001 (admin/admin)
- **Prometheus**: http://localhost:9090

### Key Metrics to Monitor

1. **Cache Hit Rate** (Target: > 75%)
   - Dashboard: "Cache Hit Ratio" pie chart
   - Alert: Fires if < 75% for 5 minutes

2. **Response Time** (Target: P95 < 100ms)
   - Dashboard: "Database Query Performance" line chart
   - Shows reduction in query time with caching

3. **Redis Memory** (Target: < 85%)
   - Dashboard: "Redis Memory Usage" line chart
   - Alert: Fires if > 85% for 5 minutes

4. **Database Connections** (Target: < 40)
   - Dashboard: "PostgreSQL Active Connections" stat
   - Alert: Fires if > 40 for 5 minutes

## Testing Your Integration

### 1. Start Services
```bash
# Terminal 1: Backend with cache
go run ./cmd/server/main.go

# Terminal 2: Start monitoring
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d

# Terminal 3: Run k6 load tests
k6 run ./load_tests/cache_test.js
```

### 2. Verify Cache Hits
```bash
# Watch real-time cache metrics
watch 'curl -s http://localhost:9090/api/v1/query?query=redis_hits_total | jq'
```

### 3. Expected Results
- **Without caching**: ~50ms per request, 0 cache hits
- **With caching**: ~5ms per request (warmed cache), 75%+ hit rate
- **Improvement**: 90% reduction in response time

## Troubleshooting

### Cache Misses Too High (< 50%)
**Cause**: Cache TTL too short or filters changing too much
**Solution**: 
- Increase TTL for that endpoint
- Check if query parameters are changing (causes different cache keys)

### High Memory Usage (> 90%)
**Cause**: Too much data cached or TTL too long
**Solution**:
- Reduce TTL (5min → 3min)
- Add cache size limits in Redis config
- Check for query parameter explosion (pagination differences)

### Cache Not Invalidating on Mutations
**Cause**: Forgot to wrap the POST/PUT/DELETE handler
**Solution**:
- Add `cacheableHandlers.CacheInvalidationMiddleware()` to mutation endpoints
- Verify `InvalidateRiskCaches()` is being called

### Redis Connection Failed
**Cause**: Redis container not running or wrong credentials
**Solution**:
```bash
# Restart monitoring stack
cd deployment
docker-compose -f docker-compose-monitoring.yaml restart redis

# Check credentials in .env
echo $REDIS_PASSWORD
```

## Performance Targets

### Phase 5 Objectives
- [ ] Cache hit rate: **> 75%**
- [ ] Response time P95: **< 100ms** (vs 200ms baseline)
- [ ] Throughput: **> 1000 req/s** (vs 500 req/s baseline)
- [ ] Redis memory: **< 500MB** for typical workload
- [ ] Database connections: **< 30** (vs 50 baseline)

### Load Test Command
```bash
# 10 virtual users, 5 minute test
k6 run \
  --vus 10 \
  --duration 5m \
  ./load_tests/cache_test.js
```

## Next Steps

1. **Integrate handlers** - Apply caching to risk/dashboard/marketplace endpoints
2. **Test performance** - Run k6 load tests and verify metrics in Grafana
3. **Optimize TTLs** - Adjust caching durations based on hit rate metrics
4. **Document results** - Update performance benchmarks in README
5. **Deploy to staging** - Test with realistic workload before production

## References

- [Cache Middleware Code](../backend/internal/cache/middleware.go)
- [Cache Integration Code](../backend/internal/handlers/cache_integration.go)
- [Load Testing Guide](./LOAD_TESTING_GUIDE.md)
- [Monitoring Dashboard](../deployment/docker-compose-monitoring.yaml)
