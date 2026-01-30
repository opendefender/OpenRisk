 Integration Implementation Guide: Adding Cache to main.go

 Overview
This guide shows the exact code changes needed to integrate the caching layer into your existing main.go routes.

 Step : Import Required Packages

File: backend/cmd/server/main.go

Add to imports section (around line ):
go
import (
    // ... existing imports ...
    "github.com/opendefender/openrisk/internal/cache"
)


 Step : Initialize Cache in main()

Location: After database initialization, before route setup (around line )

Add this code:
go
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
        redisPort = ""
    }
    redisPassword := os.Getenv("REDIS_PASSWORD")
    if redisPassword == "" {
        redisPassword = "redis"  // Development default
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


 Step : Update Route Registration

Location: API routes section (starting around line )

 Dashboard & Read-Only Routes (Add Caching)

BEFORE:
go
    // Dashboard & Analytics (Read-Only accessible à tous les connects)
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


AFTER:
go
    // Dashboard & Analytics (Read-Only accessible à tous les connects)
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


 Other GET Endpoints with Caching

Add caching to these routes (search for these in main.go and update):

go
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


 POST/PATCH/DELETE Routes (Invalidation)

No changes needed - The cache invalidation is handled internally in the handlers. However, you can verify the handlers are calling invalidation methods:

go
    // These routes automatically invalidate caches on mutations
    protected.Post("/risks", riskCreate, handlers.CreateRisk)
    protected.Patch("/risks/:id", riskUpdate, handlers.UpdateRisk)
    protected.Delete("/risks/:id", riskDelete, handlers.DeleteRisk)


Ensure these handlers call:
go
// In risk_handler.go CreateRisk function (add after DB write):
cacheableHandlers.InvalidateRiskCaches(c.Context())

// In risk_handler.go UpdateRisk function (add after DB write):
cacheableHandlers.InvalidateSpecificRisk(c.Context(), riskID)

// In risk_handler.go DeleteRisk function (add after DB delete):
cacheableHandlers.InvalidateRiskCaches(c.Context())


 Step : Add Cache Configuration (Optional)

Location: After cacheableHandlers initialization (around line )

Add this to customize TTLs (if default values don't work):
go
    // Optional: Customize cache TTLs per environment
    if os.Getenv("APP_ENV") == "production" {
        cacheableHandlers.Config.RiskCacheTTL =   time.Minute      // More aggressive caching
        cacheableHandlers.Config.DashboardCacheTTL =   time.Minute
    } else if os.Getenv("APP_ENV") == "staging" {
        cacheableHandlers.Config.RiskCacheTTL =   time.Minute
        cacheableHandlers.Config.DashboardCacheTTL =   time.Minute
    }
    log.Printf("Cache TTLs: Risk=%v, Dashboard=%v, Connector=%v, App=%v",
        cacheableHandlers.Config.RiskCacheTTL,
        cacheableHandlers.Config.DashboardCacheTTL,
        cacheableHandlers.Config.ConnectorCacheTTL,
        cacheableHandlers.Config.MarketplaceAppTTL)


 Step : Update Environment Variables

File: .env or config/ environment setup

Add these variables (if not already present):
env
REDIS_HOST=localhost
REDIS_PORT=
REDIS_PASSWORD=redis
CACHE_ENABLED=true


For Docker/Production:
env
REDIS_HOST=redis
REDIS_PORT=
REDIS_PASSWORD=secure_production_password
CACHE_ENABLED=true


 Complete Integration Example

Here's the full section of updated code for reference:

go
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
        redisPort = ""
    }
    redisPassword := os.Getenv("REDIS_PASSWORD")
    if redisPassword == "" {
        redisPassword = "redis"
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

    api := app.Group("/api/v")

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


 Testing the Integration

 . Verify Compilation
bash
cd backend
go build ./cmd/server


 . Start Services
bash
 Terminal : Start monitoring
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d

 Terminal : Start application
cd backend
go run ./cmd/server/main.go


 . Test Cache Functionality
bash
 Make first request (cache miss)
curl http://localhost:/api/v/risks

 Check Redis for cached data
redis-cli -a redis
> KEYS risk:
> TTL risk:list:page=

 Make second request (cache hit)
curl http://localhost:/api/v/risks

 Verify in Grafana
 Navigate to http://localhost:
 Check "Cache Hit Ratio" (should increase)


 . Run Load Test
bash
cd load_tests
k run cache_test.js


 Verification Checklist

- [ ] Code compiles without errors
- [ ] Redis connection successful (check logs)
- [ ] Cache handler utils initialized (check logs)
- [ ] First request completes successfully
- [ ] Cache hit rate visible in Grafana
- [ ] Response time reduced (check query performance panel)
- [ ] Load test shows improved throughput
- [ ] Alerts functioning (can trigger manually)

 Common Issues During Integration

 Issue : "undefined: handlers.NewCacheableHandlers"
Solution: Verify cache_integration.go exists in backend/internal/handlers/

 Issue : Redis connection refused
Solution: Start Redis container first
bash
docker-compose -f deployment/docker-compose-monitoring.yaml up redis -d


 Issue : Cache not working (hit rate = %)
Solution: 
. Verify Redis is running: redis-cli -a redis PING
. Check Redis keys: redis-cli -a redis KEYS ''
. Verify cache is enabled (not NoCache)
. Check application logs for errors

 Issue : High memory usage
Solution: Reduce TTL values in cacheableHandlers.Config

 Next Steps After Integration

. Monitor Performance
   - Watch Grafana dashboard
   - Target: Cache hit rate > %
   - Target: Response time < ms P

. Optimize Cache Keys
   - If hit rate low, review query parameters
   - Consider normalizing filters/pagination

. Tune TTLs
   - Too short ( min): Low hit rate
   - Too long ( min): High memory usage
   - Sweet spot: - minutes for most endpoints

. Add to More Endpoints
   - Apply to marketplace endpoints
   - Apply to other high-frequency GET routes

. Production Deployment
   - Test on staging first
   - Monitor for  hours
   - Adjust thresholds based on actual data

 Rollback Procedure

If you need to disable caching:

bash
 Option : Revert code changes
git checkout backend/cmd/server/main.go

 Option : Disable via environment variable
export CACHE_ENABLED=false
export REDIS_HOST=""   Forces NoCache

 Option : Comment out cache wrappers in routes
 Replace:
   cacheableHandlers.CacheRiskListGET(handlers.GetRisks)
 With:
   handlers.GetRisks


 References

- Full integration guide: [CACHING_INTEGRATION_GUIDE.md](./CACHING_INTEGRATION_GUIDE.md)
- Cache API reference: [cache_integration.go](../backend/internal/handlers/cache_integration.go)
- Monitoring setup: [MONITORING_SETUP_GUIDE.md](./MONITORING_SETUP_GUIDE.md)
- Load testing: [README_LOAD_TESTING.md](../load_tests/README_LOAD_TESTING.md)
