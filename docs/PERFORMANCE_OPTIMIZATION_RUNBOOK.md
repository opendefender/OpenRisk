# Performance Optimization Runbook

**Version**: 1.0
**Last Updated**: February 22, 2026
**Owner**: DevOps/Performance Engineering Team
**Audience**: System Administrators, Performance Engineers, SREs

## Table of Contents

1. [Quick Reference](#quick-reference)
2. [Database Optimization](#database-optimization)
3. [Cache Management](#cache-management)
4. [API Performance](#api-performance)
5. [Frontend Optimization](#frontend-optimization)
6. [Load Balancing](#load-balancing)
7. [Common Scenarios](#common-scenarios)
8. [Escalation Procedures](#escalation-procedures)

---

## Quick Reference

### Performance Emergency Response

**If system is slow (P95 latency > 500ms):**

1. Check CPU usage: `top -b -n 1 | head -20`
2. Check memory: `free -h`
3. Check database connections: `psql -c "SELECT count(*) FROM pg_stat_activity;"`
4. Check Redis: `redis-cli INFO stats`
5. Review application logs: `docker logs <container> | tail -100`

**If database is slow:**

1. Check slow query log: `docker exec postgres psql -U risk_user -d openrisk -c "SELECT * FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;"`
2. Check lock contention: `docker exec postgres psql -U risk_user -d openrisk -c "SELECT * FROM pg_locks WHERE NOT granted;"`
3. Check index fragmentation: See [Index Maintenance](#index-maintenance)
4. Restart database if needed: `docker restart postgres`

**If cache is failing (hit rate < 60%):**

1. Check Redis memory: `redis-cli INFO memory`
2. Check eviction policy: `redis-cli CONFIG GET maxmemory-policy`
3. Check connected clients: `redis-cli INFO clients`
4. Flush and restart if needed: `redis-cli FLUSHALL && docker restart redis`

---

## Database Optimization

### Connection Pool Tuning

**File**: `backend/config/database.go`

**Current Configuration:**
```go
MaxOpenConns: 50
MaxIdleConns: 5
ConnMaxLifetime: 3600 * time.Second
ConnMaxIdleTime: 900 * time.Second
```

**When to Adjust:**

| Issue | Symptom | Action |
|-------|---------|--------|
| Too few connections | "too many connections" error | Increase MaxOpenConns to 100 |
| Connection leak | Connections never freed | Decrease ConnMaxIdleTime |
| Connection timeout | Requests timeout | Increase MaxOpenConns |
| High memory | Too many idle connections | Decrease MaxIdleConns |

**Procedure:**
```bash
# 1. Edit configuration
vi backend/config/database.go

# 2. Rebuild and test
docker build -t openrisk:test -f backend/Dockerfile .

# 3. Monitor in staging
docker-compose -f docker-compose.test.yaml up

# 4. Check connection usage
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT datname, count(*) FROM pg_stat_activity GROUP BY datname;"

# 5. Deploy to production if stable
docker push openrisk:test
# Update deployment manifests
```

### Query Optimization

**Identify Slow Queries:**

```sql
-- Enable slow query logging (> 1s)
SET log_min_duration_statement = 1000;

-- Check slow query log
SELECT query, calls, total_time, mean_time
FROM pg_stat_statements
ORDER BY total_time DESC
LIMIT 20;

-- Explain plan for problematic query
EXPLAIN ANALYZE SELECT ... FROM risks WHERE ...;
```

**Common Optimization Patterns:**

**1. Add Missing Index**
```sql
-- Identify missing indexes
SELECT * FROM pg_stat_user_tables 
WHERE seq_scan > 1000 AND idx_scan < 100;

-- Add index for frequently scanned tables
CREATE INDEX idx_risks_status ON risks(status) 
WHERE deleted_at IS NULL;

-- Verify improvement
ANALYZE;
SELECT seq_scan, idx_scan FROM pg_stat_user_tables 
WHERE relname = 'risks';
```

**2. Fix N+1 Queries**
```go
// BAD: N+1 query
risks := []Risk{}
db.Find(&risks)
for _, risk := range risks {
  db.Preload("Mitigations").Find(&risk)  // 1 + N queries!
}

// GOOD: Use Preload
db.Preload("Mitigations").Find(&risks)

// GOOD: Use Joins if needed
db.Joins("LEFT JOIN mitigations ON mitigations.risk_id = risks.id").
  Distinct("risks.*").
  Find(&risks)
```

**3. Optimize SELECT Clauses**
```go
// BAD: Select all columns
db.Where("status = ?", "active").Find(&risks)

// GOOD: Select only needed columns
db.Select("id", "title", "status", "severity").
  Where("status = ?", "active").
  Find(&risks)
```

**4. Use Pagination**
```go
// BAD: Select all rows
db.Find(&risks)

// GOOD: Limit results
pageSize := 100
page := 1
offset := (page - 1) * pageSize
db.Offset(offset).Limit(pageSize).Find(&risks)
```

### Index Maintenance

**Check Index Health:**

```sql
-- Find unused indexes
SELECT schemaname, tablename, indexname, idx_scan
FROM pg_stat_user_indexes
WHERE idx_scan = 0
ORDER BY idx_size DESC;

-- Drop unused indexes
DROP INDEX CONCURRENTLY idx_unused_index;

-- Check fragmentation
SELECT schemaname, tablename, indexname, round(100 * (pg_relation_size(indexrelid) - 
  pg_relation_size(relfilenode)) / pg_relation_size(indexrelid), 2) AS bloat_ratio
FROM pg_stat_user_indexes
WHERE pg_relation_size(indexrelid) > 1000000
ORDER BY bloat_ratio DESC;

-- Reindex fragmented indexes
REINDEX INDEX CONCURRENTLY idx_fragmented_index;
```

**Scheduled Index Maintenance:**

```bash
#!/bin/bash
# Weekly index maintenance

docker exec postgres psql -U risk_user -d openrisk << EOF

-- Analyze tables (update statistics)
ANALYZE;

-- Reindex bloated indexes
REINDEX INDEX CONCURRENTLY idx_risks_status;
REINDEX INDEX CONCURRENTLY idx_mitigations_risk_id;

-- Vacuum tables
VACUUM ANALYZE;

EOF

# Alert if issues found
if [ $? -ne 0 ]; then
  send_alert "Database maintenance failed"
fi
```

**Cron Job:**
```bash
# Add to /etc/cron.d/openrisk-maintenance
0 2 * * 0 /opt/openrisk/scripts/weekly-db-maintenance.sh
```

---

## Cache Management

### Redis Optimization

**Monitor Cache Health:**

```bash
# Check Redis stats
redis-cli INFO stats

# Output interpretation:
# total_commands_processed: 2,847,391 (commands since startup)
# total_net_input_bytes: 89,234,123 (data received)
# hits: 2,341,299 (cache hits)
# misses: 506,092 (cache misses)
# hit_ratio = hits / (hits + misses) = 82%
```

**Common Issues & Solutions:**

| Issue | Diagnosis | Solution |
|-------|-----------|----------|
| Low hit rate (< 60%) | `redis-cli INFO stats` | Increase TTLs or memory |
| High eviction | `redis-cli INFO stats \| grep evicted` | Increase maxmemory |
| Memory pressure | `redis-cli INFO memory` | Reduce TTLs or data size |
| Slow commands | `redis-cli --latency` | Check for blocking operations |

**Cache Invalidation:**

```go
// Pattern 1: Time-based (default)
cache.Set("risks:list", data, 30 * time.Second)

// Pattern 2: Event-based (when data changes)
func (h *RiskHandler) UpdateRisk(c *fiber.Ctx) error {
  // ... update logic
  
  // Invalidate cache
  cache.Del("risks:list")
  cache.Del("risks:dashboards")
  
  return c.JSON(updatedRisk)
}

// Pattern 3: Tag-based (multiple keys)
cache.SetWithTags("risk:1:details", data, []string{"risk", "risk:1"})
// Invalidate all risk tags
cache.InvalidateByTag("risk")
```

**Clear Cache Procedure:**

```bash
# Emergency cache clear (if corrupted)
redis-cli FLUSHALL

# Selective cache clear
redis-cli DEL risks:list:*
redis-cli DEL dashboard:*

# Verify cache cleared
redis-cli DBSIZE  # Should show 0 or reduced count
```

### Cache Tuning Parameters

**File**: `backend/config/cache.go`

```go
// Query cache TTL
QueryCacheTTL: 30 * time.Second  // 30s for frequently changing data
SessionCacheTTL: 5 * time.Minute // 5m for user sessions
PermissionCacheTTL: 15 * time.Minute // 15m for roles/permissions

// Redis memory
MaxMemory: 512 * 1024 * 1024  // 512 MB
EvictionPolicy: "allkeys-lru"  // Evict least recently used
```

**Adjustment Guide:**

```
Higher TTL values:
  - Pros: Better cache hit rate, less DB load
  - Cons: Stale data, higher memory usage
  - Use for: Static data, infrequent changes

Lower TTL values:
  - Pros: Fresh data, lower memory
  - Cons: Higher DB load, more cache misses
  - Use for: Dynamic data, frequent changes
```

---

## API Performance

### Response Time Optimization

**1. Database Query Optimization** (Already covered above)

**2. Middleware Optimization**

```go
// Problematic: Running for every request
func authMiddleware(c *fiber.Ctx) error {
  token := c.Get("Authorization")
  // Fetch user from DB every time!
  user := db.First(&User{}, "id = ?", token)
}

// Better: Cache user lookup
func authMiddlewareOptimized(c *fiber.Ctx) error {
  token := c.Get("Authorization")
  user := cache.Get("user:" + token)
  if user == nil {
    user = db.First(&User{}, "id = ?", token)
    cache.Set("user:" + token, user, 5 * time.Minute)
  }
}
```

**3. Response Compression**

```go
// Enable gzip compression in main.go
app.Use(compress.New(compress.Config{
  Level: compress.LevelBestSpeed, // Balance speed/ratio
}))

// Verify compression
curl -i -H "Accept-Encoding: gzip" \
  https://api.openrisk.com/api/v1/risks | grep -i content-encoding
```

**4. Lazy Loading**

```go
// Don't load all data upfront
type RiskListResponse struct {
  ID      int    `json:"id"`
  Title   string `json:"title"`
  Status  string `json:"status"`
  // Don't include: Mitigations, Assets, Comments, History
}

// Load details only when requested
type RiskDetailResponse struct {
  ID           int          `json:"id"`
  Title        string       `json:"title"`
  Status       string       `json:"status"`
  Mitigations  []Mitigation `json:"mitigations"`
  Assets       []Asset      `json:"assets"`
}
```

### Rate Limiting

**Current Configuration:**

```go
// Per-user rate limit: 1000 requests/minute
limitConfig := limiter.Config{
  Max:        1000,
  Expiration: 1 * time.Minute,
}

// Exceeding limit returns 429 Too Many Requests
```

**Adjust if needed:**

```go
// For high-load scenarios
Max: 2000,        // Increase limit
Expiration: 2 * time.Minute,  // Extend window

// For API abuse prevention
Max: 100,         // Strict limit
Expiration: 1 * time.Minute,
```

---

## Frontend Optimization

### Bundle Size Reduction

**Current Metrics:**
- Main JS: 189 KB (target: < 250 KB) ✅
- CSS: 38 KB (target: < 50 KB) ✅
- Total gzipped: 108 KB ✅

**If bundle grows too large:**

```bash
# Analyze bundle composition
npm run analyze
# Shows which dependencies take most space

# Remove unused dependencies
npm list
npm prune

# Use dynamic imports for code splitting
const Dashboard = lazy(() => import('./pages/Dashboard'));

# Optimize images
npx imagemin frontend/public/images --out-dir=optimized
```

### Caching Headers

**Set cache headers for static assets:**

```nginx
# In nginx.conf or CDN config
location ~* \.(js|css|png|gif|ico|jpg|jpeg|svg)$ {
  expires 1y;
  add_header Cache-Control "public, immutable";
}

# For HTML
location ~* \.html$ {
  expires 5m;
  add_header Cache-Control "public, max-age=300";
}
```

---

## Load Balancing

### Multi-Instance Setup

**Health Check Configuration:**

```yaml
# In kubernetes deployment
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
```

**Session Stickiness:**

```bash
# If using load balancer with sticky sessions
# Ensure Redis stores sessions (not in-memory)
# So requests can go to any instance

# Verify session handling
curl -c cookies.txt https://api.openrisk.com/login
curl -b cookies.txt https://api.openrisk.com/api/v1/risks
# Should work even if routed to different instance
```

---

## Common Scenarios

### Scenario 1: High CPU Usage (> 70%)

**Diagnosis:**
```bash
# 1. Check which process uses CPU
top -b -n 1 | head -15

# 2. Check goroutine count (Go process)
curl localhost:6060/debug/pprof/goroutine?debug=1

# 3. Check if it's a specific handler
curl localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
(pprof) top10
(pprof) list HandlerName
```

**Resolution:**
- [ ] Check for goroutine leaks (should be stable)
- [ ] Identify slow handler (see CPU profile)
- [ ] Add caching or optimize that handler
- [ ] Monitor for improvement

### Scenario 2: Memory Leak

**Diagnosis:**
```bash
# 1. Check memory growth over time
free -h -s 10  # Update every 10 seconds

# 2. Check container memory usage
docker stats openrisk-api

# 3. Get memory profile
curl localhost:6060/debug/pprof/heap > heap.prof
go tool pprof heap.prof
(pprof) top10
(pprof) alloc_space  # See where memory allocated
```

**Resolution:**
- [ ] Look for unclosed connections or channels
- [ ] Check for circular references in caches
- [ ] Review recent code changes
- [ ] Add memory limits to container

```yaml
# kubernetes resources
resources:
  limits:
    memory: "2Gi"
  requests:
    memory: "1Gi"
```

### Scenario 3: Database Connection Exhaustion

**Diagnosis:**
```bash
# 1. Check active connections
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT count(*) as active, state FROM pg_stat_activity GROUP BY state;"

# 2. Check connection age
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT pid, usename, query_start, state FROM pg_stat_activity ORDER BY query_start;"
```

**Resolution:**
```bash
# 1. Identify long-running queries
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT pid, usename, query, query_start FROM pg_stat_activity 
      WHERE query NOT LIKE 'autovacuum%' AND state != 'idle' 
      ORDER BY query_start LIMIT 10;"

# 2. Kill problematic query if needed
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity 
      WHERE pid = 12345;"

# 3. Restart database if many stuck connections
docker restart postgres

# 4. Increase connection limit if needed
# Edit postgres.conf: max_connections = 200
```

### Scenario 4: Cache Hit Rate Low (< 60%)

**Diagnosis:**
```bash
# 1. Check Redis stats
redis-cli INFO stats | grep -E "hits|misses"

# 2. Check memory usage
redis-cli INFO memory

# 3. Check eviction rate
redis-cli INFO stats | grep evicted
```

**Resolution:**
- [ ] If memory pressure: Increase Redis memory or reduce TTL
- [ ] If low hit rate: Review cache keys, increase TTLs
- [ ] If high eviction: Adjust eviction policy
  ```bash
  redis-cli CONFIG SET maxmemory-policy "allkeys-lru"
  ```

### Scenario 5: API Timeout (> 5s response)

**Diagnosis:**
```bash
# 1. Check API logs
docker logs openrisk-api | grep "ERROR\|timeout"

# 2. Check request trace
curl -w "DNS:%{time_namelookup}s, Connect:%{time_connect}s, Total:%{time_total}s\n" \
  https://api.openrisk.com/api/v1/risks

# 3. Check specific endpoint
curl -v https://api.openrisk.com/api/v1/risks?debug=true
```

**Resolution:**
- [ ] If database slow: Run query optimization (above)
- [ ] If API slow: Check CPU/memory
- [ ] If network slow: Check network latency
- [ ] Add caching to endpoint if possible

---

## Escalation Procedures

### On-Call Escalation

**Level 1 (SRE/Engineer)** - Initial Response
- [ ] Acknowledge alert within 5 minutes
- [ ] Assess severity (Critical/Major/Minor)
- [ ] Follow appropriate runbook section
- [ ] Update status page if customer-impacting

**Level 2 (Senior Engineer)** - Escalation Criteria
- [ ] Issue unresolved after 15 minutes
- [ ] Multiple services affected
- [ ] Customer-facing impact
- [ ] Security incident

**Level 3 (Team Lead)** - Senior Escalation
- [ ] Issue unresolved after 45 minutes
- [ ] Data loss or corruption risk
- [ ] Security breach confirmed
- [ ] Multiple customers affected

### Incident Communication

**Update frequency:**
- During incident: Every 5 minutes
- Post-mitigation: Every 15 minutes
- After resolution: Post-mortem within 24 hours

**Channels:**
- Slack: #incidents
- Status page: update.openrisk.com
- Email: For customer communication
- Pagerduty: For escalations

### Post-Incident Review

**Within 24 hours:**
1. Document what happened
2. Identify root cause
3. List preventive measures
4. Assign action items
5. Schedule follow-up

**Template:**
```markdown
# Incident: [Service] [Issue]

## Timeline
- 14:23 UTC: Alert triggered
- 14:28 UTC: SRE acknowledged
- 14:35 UTC: Root cause identified
- 14:42 UTC: Mitigation deployed
- 14:47 UTC: Service recovered

## Impact
- Duration: 24 minutes
- Affected: Risk creation API
- Requests failed: 1,247 (0.3%)

## Root Cause
Database connection pool exhaustion due to long-running query

## Preventive Measures
- [ ] Add query timeout (5 seconds)
- [ ] Monitor slow query log
- [ ] Implement better connection pooling
- [ ] Add connection pool alerting
```

---

## Appendix: Commands Reference

### Database
```bash
# Connect to database
docker exec -it postgres psql -U risk_user -d openrisk

# Check slow queries
SELECT query, calls, total_time FROM pg_stat_statements ORDER BY total_time DESC;

# Clear stats
SELECT pg_stat_statements_reset();

# Check locks
SELECT * FROM pg_locks WHERE NOT granted;
```

### Cache (Redis)
```bash
# Connect to Redis
docker exec -it redis redis-cli

# Check stats
INFO stats
INFO memory

# Clear cache
FLUSHALL

# Monitor in real-time
MONITOR
```

### Application
```bash
# View logs
docker logs -f openrisk-api

# Memory profile
curl localhost:6060/debug/pprof/heap > heap.prof

# CPU profile
curl localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof

# Goroutines
curl localhost:6060/debug/pprof/goroutine?debug=1
```

---

**Version History**
| Date | Version | Changes |
|------|---------|---------|
| 2026-02-22 | 1.0 | Initial runbook |
| (Pending) | 1.1 | Add more scenarios |

**Next Review**: March 22, 2026
**Owner**: DevOps Team
