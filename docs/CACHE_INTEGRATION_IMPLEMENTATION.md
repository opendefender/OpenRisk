# Integration Implementation Guide: Adding Cache to main.go

## Overview
This guide shows the exact code changes needed to integrate the caching layer into your existing main.go routes.

## Step 1: Import Required Packages

**File**: `backend/cmd/server/main.go`

**Add to imports section** (around line 20):
```go
import (
    // ... existing imports ...
    "github.com/opendefender/openrisk/internal/cache"
)
```

## Step 2: Initialize Cache in main()

**Location**: After database initialization, before route setup (around line 120)

**Add this code**:
```go
    // =========================================================================
    // CACHE INITIALIZATION
    // =========================================================================
    
    // Initialize Redis cache for performance optimization
    redisHost := os.Getenv("REDIS_HOST")
    if redisHost == "" {
        redisHost = "localhost"
    }
    redisPort := os.Getenv("REDIS_PORT")
    if redisPort == "" {
        redisPort = "6379"
    }
    redisPassword := os.Getenv("REDIS_PASSWORD")
    if redisPassword == "" {
        redisPassword = "redis123"  // Development default
    }
    
    // Create Redis cache instance
    cacheInstance, err := cache.NewRedisCache(
        redisHost,
        redisPort,
        redisPassword,
    )
    if err != nil {
        log.Printf("Warning: Redis cache initialization failed: %v. Continuing without caching.", err)
        cacheInstance = cache.NewMemoryCache()  // Fallback to in-memory cache
    } else {
        log.Println("Cache: Redis initialized successfully")
    }
    defer cacheInstance.Close()
    
    // Initialize caching handler utilities
    cacheableHandlers := handlers.NewCacheableHandlers(cacheInstance)
    log.Println("Cache: Handler utilities initialized")
```

## Step 3: Update Route Registration

**Location**: API routes section (starting around line 200)

### Dashboard & Read-Only Routes (Add Caching)

**BEFORE:**
```go
    // Dashboard & Analytics (Read-Only accessible à tous les connectés)
    protected.Get("/stats", handlers.GetDashboardStats)
    protected.Get("/risks",
        middleware.RequirePermissions(permissionService, domain.Permission{
            Resource: domain.PermissionResourceRisk,
            Action:   domain.PermissionRead,
        }),
        handlers.GetRisks)
    protected.Get("/risks/:id",
        middleware.RequirePermissions(permissionService, domain.Permission{
            Resource: domain.PermissionResourceRisk,
            Action:   domain.PermissionRead,
        }),
        handlers.GetRisk)
```

**AFTER:**
```go
    // Dashboard & Analytics (Read-Only accessible à tous les connectés)
    protected.Get("/stats",
        cacheableHandlers.CacheDashboardStatsGET(handlers.GetDashboardStats))
    protected.Get("/risks",
        middleware.RequirePermissions(permissionService, domain.Permission{
            Resource: domain.PermissionResourceRisk,
            Action:   domain.PermissionRead,
        }),
        cacheableHandlers.CacheRiskListGET(handlers.GetRisks))
    protected.Get("/risks/:id",
        middleware.RequirePermissions(permissionService, domain.Permission{
            Resource: domain.PermissionResourceRisk,
            Action:   domain.PermissionRead,
        }),
        cacheableHandlers.CacheRiskGetByIDGET(handlers.GetRisk))
```

### Other GET Endpoints with Caching

**Add caching to these routes** (search for these in main.go and update):

```go
    // Statistics endpoints
    api.Get("/stats/risk-matrix",
        cacheableHandlers.CacheDashboardMatrixGET(handlers.GetRiskMatrixData))
    api.Get("/stats/risk-distribution",
        handlers.GetRiskDistribution)  // No caching (volatile data)
    api.Get("/stats/mitigation-metrics",
        handlers.GetMitigationMetrics)  // No caching (volatile data)
    api.Get("/stats/top-vulnerabilities",
        handlers.GetTopVulnerabilities)  // No caching (volatile data)
    api.Get("/stats/trends",
        middleware.Protected(),
        cacheableHandlers.CacheDashboardTimelineGET(handlers.GetGlobalRiskTrend))
    
    // Asset endpoints
    api.Get("/assets",
        middleware.Protected(),
        handlers.GetAssets)  // Consider adding cache if frequently accessed
    
    // Gamification
    api.Get("/gamification/me",
        middleware.Protected(),
        handlers.GetMyGamificationProfile)  // No caching (user-specific)
    
    // Mitigations
    api.Get("/mitigations/recommended",
        handlers.GetRecommendedMitigations)  // Consider adding cache
```

### POST/PATCH/DELETE Routes (Invalidation)

**No changes needed** - The cache invalidation is handled internally in the handlers. However, you can verify the handlers are calling invalidation methods:

```go
    // These routes automatically invalidate caches on mutations
    protected.Post("/risks", riskCreate, handlers.CreateRisk)
    protected.Patch("/risks/:id", riskUpdate, handlers.UpdateRisk)
    protected.Delete("/risks/:id", riskDelete, handlers.DeleteRisk)
```

**Ensure these handlers call**:
```go
// In risk_handler.go CreateRisk function (add after DB write):
cacheableHandlers.InvalidateRiskCaches(c.Context())

// In risk_handler.go UpdateRisk function (add after DB write):
cacheableHandlers.InvalidateSpecificRisk(c.Context(), riskID)

// In risk_handler.go DeleteRisk function (add after DB delete):
cacheableHandlers.InvalidateRiskCaches(c.Context())
```

## Step 4: Add Cache Configuration (Optional)

**Location**: After cacheableHandlers initialization (around line 130)

**Add this to customize TTLs** (if default values don't work):
```go
    // Optional: Customize cache TTLs per environment
    if os.Getenv("APP_ENV") == "production" {
        cacheableHandlers.Config.RiskCacheTTL = 10 * time.Minute      // More aggressive caching
        cacheableHandlers.Config.DashboardCacheTTL = 15 * time.Minute
    } else if os.Getenv("APP_ENV") == "staging" {
        cacheableHandlers.Config.RiskCacheTTL = 5 * time.Minute
        cacheableHandlers.Config.DashboardCacheTTL = 10 * time.Minute
    }
    log.Printf("Cache TTLs: Risk=%v, Dashboard=%v, Connector=%v, App=%v",
        cacheableHandlers.Config.RiskCacheTTL,
        cacheableHandlers.Config.DashboardCacheTTL,
        cacheableHandlers.Config.ConnectorCacheTTL,
        cacheableHandlers.Config.MarketplaceAppTTL)
```

## Step 5: Update Environment Variables

**File**: `.env` or `config/` environment setup

**Add these variables** (if not already present):
```env
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=redis123
CACHE_ENABLED=true
```

**For Docker/Production**:
```env
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_PASSWORD=secure_production_password
CACHE_ENABLED=true
```

## Complete Integration Example

Here's the full section of updated code for reference:

```go
func main() {
    // ... existing configuration and database setup ...

    // =========================================================================
    // CACHE INITIALIZATION
    // =========================================================================
    
    redisHost := os.Getenv("REDIS_HOST")
    if redisHost == "" {
        redisHost = "localhost"
    }
    redisPort := os.Getenv("REDIS_PORT")
    if redisPort == "" {
        redisPort = "6379"
    }
    redisPassword := os.Getenv("REDIS_PASSWORD")
    if redisPassword == "" {
        redisPassword = "redis123"
    }
    
    cacheInstance, err := cache.NewRedisCache(
        redisHost,
        redisPort,
        redisPassword,
    )
    if err != nil {
        log.Printf("Warning: Redis cache initialization failed: %v. Using in-memory cache.", err)
        cacheInstance = cache.NewMemoryCache()
    }
    defer cacheInstance.Close()
    
    cacheableHandlers := handlers.NewCacheableHandlers(cacheInstance)
    log.Println("Cache: Handler utilities initialized")

    // =========================================================================
    // API ROUTES
    // =========================================================================

    api := app.Group("/api/v1")

    // ... existing routes ...

    // Dashboard & Analytics with caching
    protected.Get("/stats",
        cacheableHandlers.CacheDashboardStatsGET(handlers.GetDashboardStats))
    
    protected.Get("/risks",
        middleware.RequirePermissions(permissionService, domain.Permission{
            Resource: domain.PermissionResourceRisk,
            Action:   domain.PermissionRead,
        }),
        cacheableHandlers.CacheRiskListGET(handlers.GetRisks))
    
    protected.Get("/risks/:id",
        middleware.RequirePermissions(permissionService, domain.Permission{
            Resource: domain.PermissionResourceRisk,
            Action:   domain.PermissionRead,
        }),
        cacheableHandlers.CacheRiskGetByIDGET(handlers.GetRisk))

    // ... rest of routes ...
}
```

## Testing the Integration

### 1. Verify Compilation
```bash
cd backend
go build ./cmd/server
```

### 2. Start Services
```bash
# Terminal 1: Start monitoring
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d

# Terminal 2: Start application
cd backend
go run ./cmd/server/main.go
```

### 3. Test Cache Functionality
```bash
# Make first request (cache miss)
curl http://localhost:3000/api/v1/risks

# Check Redis for cached data
redis-cli -a redis123
> KEYS risk:*
> TTL risk:list:page=0

# Make second request (cache hit)
curl http://localhost:3000/api/v1/risks

# Verify in Grafana
# Navigate to http://localhost:3001
# Check "Cache Hit Ratio" (should increase)
```

### 4. Run Load Test
```bash
cd load_tests
k6 run cache_test.js
```

## Verification Checklist

- [ ] Code compiles without errors
- [ ] Redis connection successful (check logs)
- [ ] Cache handler utils initialized (check logs)
- [ ] First request completes successfully
- [ ] Cache hit rate visible in Grafana
- [ ] Response time reduced (check query performance panel)
- [ ] Load test shows improved throughput
- [ ] Alerts functioning (can trigger manually)

## Common Issues During Integration

### Issue 1: "undefined: handlers.NewCacheableHandlers"
**Solution**: Verify cache_integration.go exists in `backend/internal/handlers/`

### Issue 2: Redis connection refused
**Solution**: Start Redis container first
```bash
docker-compose -f deployment/docker-compose-monitoring.yaml up redis -d
```

### Issue 3: Cache not working (hit rate = 0%)
**Solution**: 
1. Verify Redis is running: `redis-cli -a redis123 PING`
2. Check Redis keys: `redis-cli -a redis123 KEYS '*'`
3. Verify cache is enabled (not NoCache)
4. Check application logs for errors

### Issue 4: High memory usage
**Solution**: Reduce TTL values in cacheableHandlers.Config

## Next Steps After Integration

1. **Monitor Performance**
   - Watch Grafana dashboard
   - Target: Cache hit rate > 75%
   - Target: Response time < 100ms P95

2. **Optimize Cache Keys**
   - If hit rate low, review query parameters
   - Consider normalizing filters/pagination

3. **Tune TTLs**
   - Too short (3 min): Low hit rate
   - Too long (30 min): High memory usage
   - Sweet spot: 5-15 minutes for most endpoints

4. **Add to More Endpoints**
   - Apply to marketplace endpoints
   - Apply to other high-frequency GET routes

5. **Production Deployment**
   - Test on staging first
   - Monitor for 24 hours
   - Adjust thresholds based on actual data

## Rollback Procedure

If you need to disable caching:

```bash
# Option 1: Revert code changes
git checkout backend/cmd/server/main.go

# Option 2: Disable via environment variable
export CACHE_ENABLED=false
export REDIS_HOST=""  # Forces NoCache

# Option 3: Comment out cache wrappers in routes
# Replace:
#   cacheableHandlers.CacheRiskListGET(handlers.GetRisks)
# With:
#   handlers.GetRisks
```

## References

- Full integration guide: [CACHING_INTEGRATION_GUIDE.md](./CACHING_INTEGRATION_GUIDE.md)
- Cache API reference: [cache_integration.go](../backend/internal/handlers/cache_integration.go)
- Monitoring setup: [MONITORING_SETUP_GUIDE.md](./MONITORING_SETUP_GUIDE.md)
- Load testing: [README_LOAD_TESTING.md](../load_tests/README_LOAD_TESTING.md)
