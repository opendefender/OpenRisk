 Phase  Priority : Quick Reference Card

 One-Minute Setup

bash
 . Start monitoring
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d

 . Access Grafana
open http://localhost:   admin/admin

 . Run load test
cd ../load_tests
k run cache_test.js


 Three Common Tasks

 Task : Add Caching to a GET Endpoint

Before:
go
protected.Get("/risks", handlers.GetRisks)


After:
go
protected.Get("/risks", cacheableHandlers.CacheRiskListGET(handlers.GetRisks))


 Task : Verify Cache is Working

bash
 Check cache hit rate in Grafana
 Navigate to: http://localhost: → OpenRisk Performance dashboard
 Look for: "Cache Hit Ratio" pie chart (target: %+)

 Or check Prometheus directly
curl 'http://localhost:/api/v/query?query=redis_keyspace_hits_total'


 Task : Invalidate Cache Manually

go
// In your handler or service
cacheableHandlers.InvalidateRiskCaches(ctx)
// OR
cacheableHandlers.InvalidateSpecificRisk(ctx, riskID)


 Cache Methods Reference

 Risk Endpoints
go
cacheableHandlers.CacheRiskListGET(handler)        // List with filters
cacheableHandlers.CacheRiskGetByIDGET(handler)     // Single risk
cacheableHandlers.CacheRiskSearchGET(handler)      // Search results
cacheableHandlers.InvalidateRiskCaches(ctx)        // Clear all risk caches
cacheableHandlers.InvalidateSpecificRisk(ctx, id)  // Clear one risk


 Dashboard Endpoints
go
cacheableHandlers.CacheDashboardStatsGET(handler)      // Stats by period
cacheableHandlers.CacheDashboardMatrixGET(handler)     // Risk matrix
cacheableHandlers.CacheDashboardTimelineGET(handler)   // Timeline data


 Marketplace Endpoints
go
cacheableHandlers.CacheConnectorListGET(handler)       // Connector list
cacheableHandlers.CacheConnectorGetByIDGET(handler)    // Single connector
cacheableHandlers.CacheMarketplaceAppGetByIDGET(handler) // App details


 TTL Configuration

Default values (in cache_integration.go):
- Risk data:  minutes
- Dashboard stats:  minutes
- Connectors:  minutes
- Marketplace apps:  minutes

Override in main.go:
go
cacheableHandlers.Config.RiskCacheTTL =   time.Minute


 Performance Monitoring

 Key Metrics
| Metric | Target | Where to Check |
|--------|--------|-----------------|
| Cache Hit Rate | > % | Grafana → Cache Hit Ratio |
| Response Time P | < ms | Grafana → Query Performance |
| DB Connections | <  | Grafana → PG Connections |
| Redis Memory | < MB | Grafana → Redis Memory Usage |

 What to Do If...

Cache hit rate low (< %)
- Increase TTL for that endpoint
- Check query parameters aren't changing too much

Response time still slow
- Verify cache is enabled (check Redis connection)
- Run k run cache_test.js to verify improvement
- Check database queries in query logs

Alert triggered (Redis/DB)
- Check Grafana dashboard for trend
- Review alert details in AlertManager (http://localhost:)
- Check Slack notifications

 Files You'll Modify

. backend/cmd/server/main.go (Route integration)
   - Add cacheableHandlers initialization
   - Wrap GET handlers with cache methods

. Optional: backend/internal/handlers/cache_integration.go (Customization)
   - Adjust TTL values
   - Add custom cache keys

. Optional: deployment/monitoring/alerts.yml (Alert tuning)
   - Adjust thresholds based on production data

 Command Reference

bash
 Start everything
docker-compose -f deployment/docker-compose-monitoring.yaml up -d

 View logs
docker-compose logs -f prometheus
docker-compose logs -f grafana
docker-compose logs -f alertmanager

 Test Redis connection
redis-cli -a redis PING

 Check Prometheus targets
curl http://localhost:/api/v/targets

 Query specific metric
curl 'http://localhost:/api/v/query?query=redis_memory_used_bytes'

 Run load test with specific users/duration
k run --vus  --duration m ./load_tests/cache_test.js

 Stop everything
docker-compose -f deployment/docker-compose-monitoring.yaml down


 Common Issues & Fixes

| Issue | Fix |
|-------|-----|
| Redis won't connect | Check Redis password in docker-compose env vars |
| Grafana empty | Verify Prometheus datasource connection (Config → Data Sources) |
| Alerts not firing | Check AlertManager logs: docker-compose logs alertmanager |
| High memory | Reduce TTL in cache_integration.go or set Redis memory limit |
| Cache misses high | Wrap more endpoints with cacheableHandlers methods |

 Performance Checklist

- [ ] Wrapped GET endpoints with cache methods
- [ ] Started monitoring stack
- [ ] Verified Grafana dashboard loads
- [ ] Ran baseline k test
- [ ] Checked cache hit rate > % in Grafana
- [ ] Verified response time < ms P
- [ ] Confirmed DB connections < 
- [ ] Set up Slack alerts (optional)
- [ ] Documented any custom configurations

 Next Steps

. Integration (- hours)
   - Apply cache wrapper to - key endpoints
   - Test with manual requests

. Testing (- hours)
   - Run k load test script
   - Compare metrics before/after

. Tuning ( mins)
   - Adjust TTLs based on hit rate
   - Fine-tune connection pool if needed

. Production (depends on approval)
   - Deploy to staging first
   - Monitor for  hours
   - Deploy to production

 Debug Checklist

bash
 Is Redis running?
docker ps | grep redis

 Is cache being used?
redis-cli -a redis INFO keyspace

 Are metrics flowing?
curl http://localhost:/api/v/targets | jq '.data.activeTargets'

 Check specific metric
curl 'http://localhost:/api/v/query?query=redis_commands_processed_total'

 Verify alert rules loaded
curl http://localhost:/api/v/rules | jq '.data.groups[].rules'

 Check Grafana datasource
curl -u admin:admin http://localhost:/api/datasources


 Useful Links

- Grafana: http://localhost:
- Prometheus: http://localhost:
- AlertManager: http://localhost:
- Redis CLI: redis-cli -a redis
- k Documentation: https://k.io/docs/

 Performance Baseline (After Integration)

Expected improvements with this optimization:


Metric                 Before    After      Improvement

Response Time (avg)    ms  →  ms      % ↓
Response Time (P)    ms  →  ms      % ↓
Throughput             /s  → /s     x ↑
DB Connections         -  →  -     % ↓
Cache Hit Rate         %     →  %+      New
CPU Usage              %    →  %       % ↓


---

Last Updated:   
Status: Production Ready  
Phase:  Priority 
