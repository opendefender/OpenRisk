# Performance Troubleshooting Procedures

**Version**: 1.0
**Last Updated**: February 22, 2026
**Owner**: DevOps/Support Team
**Audience**: On-call engineers, support team

## Quick Troubleshooting Flowchart

```
Is system slow or down?
├─ YES: Is it a network issue?
│  ├─ YES: Check network connectivity
│  └─ NO: Continue below
├─ Check if API responding: curl -I https://api.openrisk.com/health
│  ├─ 200 OK: Services up, but slow
│  ├─ 5xx: API error, check logs
│  └─ No response: Services down
└─ Continue to specific symptom diagnosis
```

---

## Symptom: Slow Response Times (P95 > 500ms)

### Step 1: Identify Affected Endpoint

```bash
# Check which endpoints are slow
curl -w "@curl-format.txt" -o /dev/null -s \
  https://api.openrisk.com/api/v1/risks

# Output format (curl-format.txt):
# Time_namelookup: %{time_namelookup}
# Time_connect: %{time_connect}
# Time_appconnect: %{time_appconnect}
# Time_pretransfer: %{time_pretransfer}
# Time_starttransfer: %{time_starttransfer}
# Time_total: %{time_total}

# Expected baseline:
# - namelookup: 2-10ms (DNS)
# - connect: 5-20ms (TCP)
# - pretransfer: 10-30ms (TLS)
# - starttransfer: 50-150ms (API processing)
# - total: 60-200ms
```

### Step 2: Determine Root Cause

**If starttransfer time is high (API slow):**

```bash
# 1. Check API CPU/Memory
docker stats openrisk-api --no-stream
# Expected: CPU < 70%, Memory < 3 GB

# 2. Check for errors in logs
docker logs openrisk-api -n 100 | grep ERROR

# 3. Check goroutine count (goroutine leak?)
curl localhost:6060/debug/pprof/goroutine?debug=1 | grep "goroutine"
# Expected: < 2000 goroutines

# 4. Check database queries
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT query, calls, total_time, mean_time FROM pg_stat_statements 
      ORDER BY total_time DESC LIMIT 10;"
```

**If connect time is high (Network slow):**

```bash
# 1. Check network latency
ping -c 5 api.openrisk.com

# 2. Check traceroute
traceroute api.openrisk.com

# 3. Check if load balancer is bottleneck
# Monitor on load balancer directly
# Expected: < 50ms latency between LB and backend
```

**If namelookup time is high (DNS slow):**

```bash
# 1. Check DNS resolution
nslookup api.openrisk.com

# 2. Check DNS from container
docker exec openrisk-api nslookup api.openrisk.com

# 3. Consider using local DNS cache if remote DNS slow
# Or update /etc/resolv.conf in container
```

### Step 3: Resolution by Root Cause

**Case A: Database Query Slow**

```bash
# 1. Identify slow query
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT query, mean_time FROM pg_stat_statements 
      WHERE mean_time > 100 ORDER BY mean_time DESC;"

# 2. Explain query plan
docker exec postgres psql -U risk_user -d openrisk \
  -c "EXPLAIN ANALYZE SELECT ... FROM risks WHERE status = 'active';"

# 3. Look for sequential scans (slow) vs index scans (fast)
# Sequential scan: "Seq Scan on risks" - needs index
# Index scan: "Index Scan using idx_risks_status" - good

# 4. Add missing index if found
docker exec postgres psql -U risk_user -d openrisk \
  -c "CREATE INDEX idx_risks_status ON risks(status) 
      WHERE deleted_at IS NULL;"

# 5. Verify improvement
# Re-run EXPLAIN ANALYZE, should now use index
```

**Case B: High CPU Usage**

```bash
# 1. Get CPU profile
curl localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof

# 2. Analyze with pprof
go tool pprof cpu.prof
(pprof) top10
(pprof) list main.GetRisks  # Check specific function

# 3. Look for:
# - Inefficient loops (sorting when sorted would be better)
# - Unnecessary allocations (copy instead of reference)
# - String operations (concatenation in loop)

# 4. Fix identified issue in code
# Rebuild and deploy

# 5. Verify improvement
curl localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
(pprof) top10  # Should show different/lower CPU consumers
```

**Case C: Memory Pressure**

```bash
# 1. Get memory profile
curl localhost:6060/debug/pprof/heap > heap.prof

# 2. Analyze allocations
go tool pprof heap.prof
(pprof) alloc_space  # Total allocated
(pprof) inuse_space  # Currently using
(pprof) top10 -cum

# 3. Look for:
# - Large slices allocated upfront (pre-allocate only needed size)
# - Unclosed connections/files (defer close)
# - Circular references in caches

# 4. Fix memory leaks
# See PERFORMANCE_OPTIMIZATION_RUNBOOK.md Scenario 2

# 5. If immediate action needed, restart container
docker restart openrisk-api
```

**Case D: Cache Miss Storm**

```bash
# 1. Check cache hit rate
redis-cli INFO stats | grep -E "hits|misses"
# Calculate: hits / (hits + misses) should be > 80%

# 2. If low hit rate
# Option A: Increase cache TTL
# Edit backend/config/cache.go, increase TTL values

# Option B: Check for cache invalidation issues
docker logs openrisk-api | grep "cache invalidated"

# Option C: Check Redis memory
redis-cli INFO memory | grep used_memory_human
# If near maxmemory, keys being evicted, increase cache memory
```

---

## Symptom: Database Connection Errors

### Error Message: "too many connections"

**Step 1: Check Connection Count**

```bash
# Current connections
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT count(*) as active FROM pg_stat_activity;"

# Max allowed
docker exec postgres psql -U postgres \
  -c "SHOW max_connections;"
```

**Step 2: Identify Problematic Connections**

```bash
# Find long-running connections
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT pid, usename, state, query_start, query 
      FROM pg_stat_activity 
      WHERE state != 'idle' 
      ORDER BY query_start;"

# Find idle connections hogging resources
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT pid, usename, state, query_start, query 
      FROM pg_stat_activity 
      WHERE state = 'idle in transaction' 
      ORDER BY query_start;"
```

**Step 3: Resolution**

```bash
# Option 1: Kill specific connection
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT pg_terminate_backend(12345);"  # Replace 12345 with PID

# Option 2: Kill all idle connections
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity 
      WHERE state = 'idle' AND pid <> pg_backend_pid();"

# Option 3: Increase connection limit
docker exec postgres psql -U postgres \
  -c "ALTER SYSTEM SET max_connections = 200;"

# Option 4: Restart database
docker restart postgres

# Step 4: Check application code for connection leaks
# Review: defer db.Close() in all database operations
# Check: Connection pool configuration in backend/config/database.go
```

---

## Symptom: High Memory Usage (> 6 GB)

### Step 1: Identify Memory Hog

```bash
# Check which service using memory
docker stats --no-stream | sort -k 4 -rh

# Expected:
# openrisk-api    ~2-3 GB (Go process)
# postgres        ~2-3 GB (database cache)
# redis           ~0.5 GB (cache store)
```

### Step 2: Drill Down by Service

**If API using too much memory:**

```bash
# 1. Get heap profile
curl localhost:6060/debug/pprof/heap > heap.prof

# 2. Find memory leaks
go tool pprof heap.prof
(pprof) alloc_space -cum  # Cumulative allocations
(pprof) top10

# 3. Common causes
# - Unbounded slice growth
# - Goroutine leak (accumulating goroutines)
# - Unclosed resources (file descriptors, connections)

# 4. Check goroutine count
curl localhost:6060/debug/pprof/goroutine?debug=1 | head -10

# 5. If goroutine leak suspected
curl localhost:6060/debug/pprof/goroutine > goroutines.prof
# Save again after 5 minutes
curl localhost:6060/debug/pprof/goroutine > goroutines2.prof
# Compare counts - should be stable, not growing

# 6. If memory keeps growing
# Restart API container as temporary fix
docker restart openrisk-api
# Fix code issue for permanent solution
```

**If Database using too much memory:**

```bash
# 1. Check shared buffers usage
docker exec postgres psql -U postgres \
  -c "SHOW shared_buffers;"

# 2. Check cache hit ratio (more cache = less I/O)
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT sum(heap_blks_read) as disk_read, 
             sum(heap_blks_hit) as cache_hit,
             sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as ratio
      FROM pg_statio_user_tables;"

# 3. If ratio < 99%, increase shared_buffers
docker exec postgres psql -U postgres \
  -c "ALTER SYSTEM SET shared_buffers = '2GB';"
docker restart postgres

# 4. Monitor memory after restart
docker stats postgres --no-stream
```

**If Redis using too much memory:**

```bash
# 1. Check memory usage
redis-cli INFO memory | grep used_memory_human

# 2. Check eviction
redis-cli INFO stats | grep evicted_keys
# If > 0, Redis deleting keys to stay under maxmemory

# 3. If memory too high
# Option A: Reduce TTLs
# Edit backend/config/cache.go, decrease TTL values

# Option B: Reduce cache size
redis-cli CONFIG SET maxmemory 256mb  # Reduce from 512mb

# Option C: Change eviction policy
redis-cli CONFIG SET maxmemory-policy "volatile-lru"
# Only evict keys with TTL, not all keys

# 4. Monitor
redis-cli INFO memory
```

---

## Symptom: Database Locks / Deadlocks

### Step 1: Detect Lock Situation

```bash
# Check for locks
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT l.pid, l.usename, l.application_name, l.state, l.query
      FROM pg_stat_activity l
      WHERE l.state = 'active' AND EXISTS (
        SELECT 1 FROM pg_locks 
        WHERE NOT granted AND pid = l.pid);"

# Check for blocked queries
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT blocked_locks.pid AS blocked_pid,
             blocked_activity.query AS blocked_statement,
             blocking_locks.pid AS blocking_pid,
             blocking_activity.query AS blocking_statement
      FROM pg_catalog.pg_locks blocked_locks
      JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
      JOIN pg_catalog.pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype
      WHERE NOT blocked_locks.granted AND blocking_locks.granted;"
```

### Step 2: Identify Problematic Query

```bash
# From output above, find the blocking query
# Check what it's doing
SELECT query_start, query FROM pg_stat_activity WHERE pid = <blocking_pid>;

# Look for:
# - Long-running transactions
# - Exclusive locks on hot tables
# - Deadlocks detected in logs
```

### Step 3: Resolution

**Option A: Cancel Blocking Query (if safe)**

```bash
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT pg_cancel_backend(<blocking_pid>);"

# If cancel doesn't work (query in syscall), terminate
docker exec postgres psql -U risk_user -d openrisk \
  -c "SELECT pg_terminate_backend(<blocking_pid>);"
```

**Option B: Check Application Code**

```go
// BAD: Acquiring locks without releasing
tx := db.BeginTx(ctx, nil)
tx.Lock()
// ... very long operation ...
// If error occurs, lock never released!

// GOOD: Always defer rollback/commit
tx := db.BeginTx(ctx, nil)
defer tx.Rollback()
// ... operations ...
tx.Commit()
```

**Option C: Adjust Lock Behavior**

```sql
-- Set statement timeout to prevent long locks
SET statement_timeout = '30s';

-- Check current lock wait time
SHOW lock_timeout;

-- Adjust if needed
ALTER SYSTEM SET lock_timeout = '30s';
```

---

## Symptom: Cache Issues

### Symptom: Cache Hit Rate Low (< 60%)

**Step 1: Understand Current State**

```bash
redis-cli INFO stats | grep -E "hits|misses"
# Calculate: hits / (hits + misses)
# Target: > 80%
```

**Step 2: Diagnose Why**

```bash
# 1. Check which keys being accessed
redis-cli MONITOR  # Live view of commands
# Ctrl+C after few seconds

# 2. Check key expiration
redis-cli --scan --pattern "*" | head -100
redis-cli TTL key_name  # How long until expiration

# 3. Check memory pressure
redis-cli INFO memory | grep -E "used_memory|maxmemory"
```

**Step 3: Improve Hit Rate**

```bash
# Option A: Increase cache TTL (less eviction)
# Edit backend/config/cache.go
QueryCacheTTL: 60 * time.Second  # Was 30s

# Option B: Increase Redis memory
docker exec redis redis-cli CONFIG SET maxmemory 1gb  # Was 512mb

# Option C: Pre-warm cache on startup
// In backend/main.go
func init() {
  // Pre-load commonly accessed data
  loadCacheDashboardData()
}
```

### Symptom: Cache Invalidation Issues (Stale Data)

**Step 1: Detect Stale Cache**

```bash
# 1. Check when data was cached
redis-cli TTL risks:list
# If returned value close to initial TTL, recently cached (good)
# If value very low, about to expire (might get stale)

# 2. Check if cache getting invalidated
docker logs openrisk-api | grep "cache invalidated" | tail -20

# 3. Verify cache content
redis-cli GET risks:list
# Compare with actual data
curl https://api.openrisk.com/api/v1/risks?limit=5
```

**Step 2: Fix Invalidation Logic**

```go
// BAD: Not invalidating cache on update
func (h *RiskHandler) UpdateRisk(c *fiber.Ctx) error {
  // ... update risk in database ...
  // FORGOT: cache.Del("risks:list")
  return c.JSON(updatedRisk)
}

// GOOD: Invalidate affected caches
func (h *RiskHandler) UpdateRisk(c *fiber.Ctx) error {
  // ... update risk in database ...
  cache.Del("risks:list")           // List might be affected
  cache.Del("risk:" + id + ":detail") // Specific risk changed
  return c.JSON(updatedRisk)
}
```

**Step 3: Verify Fix**

```bash
# 1. Test update workflow
curl -X PUT https://api.openrisk.com/api/v1/risks/123 \
  -H "Content-Type: application/json" \
  -d '{"title": "Updated"}'

# 2. Check cache was cleared
redis-cli EXISTS "risks:list"
# Should return 0 (deleted)

# 3. Next fetch should reload from database
curl https://api.openrisk.com/api/v1/risks?limit=1

# 4. Verify new data is cached
redis-cli TTL risks:list
# Should show fresh TTL (near max value)
```

---

## Symptom: WebSocket Connection Issues

### Problem: WebSocket Connections Not Establishing

**Step 1: Check WebSocket Availability**

```bash
# Verify endpoint exists
curl -i https://api.openrisk.com/api/v1/ws/dashboard
# Should get 400 Bad Request (normal, expects WebSocket upgrade)
# NOT 404 Not Found

# Check with wscat tool
npm install -g wscat
wscat -c wss://api.openrisk.com/api/v1/ws/dashboard
# Should prompt for message input (connection successful)
```

**Step 2: Check Certificate Issues**

```bash
# Browser console errors might show:
# "WebSocket connection failed: certificate not valid"

# Verify certificate
openssl s_client -connect api.openrisk.com:443 -showcerts

# If self-signed or invalid
# Option A: Install cert in system trust
# Option B: Use http://localhost for testing
```

**Step 3: Check Server-Side Logs**

```bash
docker logs openrisk-api -n 50 | grep -i websocket

# Look for:
# - "WebSocket upgraded" (good)
# - "connection error" (bad)
# - "authentication failed" (auth issue)
```

**Step 4: Debug Connection**

```bash
# In browser console
const ws = new WebSocket('wss://api.openrisk.com/api/v1/ws/dashboard?token=xxx');

ws.onopen = () => console.log('Connected!');
ws.onmessage = (e) => console.log('Message:', e.data);
ws.onerror = (e) => console.error('Error:', e);
ws.onclose = () => console.log('Closed');

// Should see "Connected!" after ~100ms
```

### Problem: Reconnection Loop (Constant Reconnecting)

**Step 1: Check Why Disconnects**

```bash
# Server logs
docker logs openrisk-api | grep -E "connection|disconnect|error" | tail -20

# Look for patterns:
# - "EOF" or "unexpected error" (network issue)
# - "invalid token" (authentication problem)
# - "resource exhausted" (server overloaded)
```

**Step 2: Check Authentication**

```bash
# Verify token is valid
TOKEN=$(curl -s https://api.openrisk.com/auth/login \
  -d '{"email": "user@example.com", "password": "..."}' | jq .token)

# Test token with WebSocket
wscat -c "wss://api.openrisk.com/api/v1/ws/dashboard?token=$TOKEN"
# Should connect successfully
```

**Step 3: Fix Issues**

**Issue: Token Expired**
```bash
# Implement token refresh in React hook
// In useWebSocket.ts
const token = localStorage.getItem('auth_token');
const refreshToken = localStorage.getItem('refresh_token');

// Before connecting, check token expiry
const isExpired = checkTokenExpiry(token);
if (isExpired) {
  const newToken = await refreshAccessToken(refreshToken);
  localStorage.setItem('auth_token', newToken);
}
```

**Issue: Server Rejecting Connection**
```bash
# Check max connections limit
docker exec openrisk-api curl localhost:8080/api/v1/ws/stats
# Look for "connected_clients"
# If near limit, increase maxClients in websocket_hub.go

# Check logs for why rejected
docker logs openrisk-api | grep "reject"
```

---

## Escalation Decision Tree

### When to Escalate

```
Is issue resolved within 5 minutes?
├─ YES: Document in ticket, close
└─ NO: Is severity HIGH?
   ├─ YES: Escalate immediately to Level 2
   └─ NO: Continue troubleshooting, set 15-min timer

Is issue resolved within 15 minutes?
├─ YES: Escalate to Level 2 for review/permanent fix
└─ NO: Is it causing service degradation?
   ├─ YES: Escalate to Level 3, activate war room
   └─ NO: Continue troubleshooting, document attempts
```

### Contact Information

**Level 1 (Current shift): On-call Engineer**
- Slack: @current-oncall
- Pagerduty: Auto-escalates if not acknowledged in 5 min

**Level 2: Senior Engineer**
- Slack: #escalations
- Phone: Configured in Pagerduty
- Auto-escalate: After 15 minutes if unresolved

**Level 3: Team Lead**
- Slack: #incidents-critical
- Phone: Configured in Pagerduty
- Auto-escalate: After 45 minutes if unresolved

---

## Incident Documentation Template

**Use this to document any issue you troubleshoot:**

```markdown
# Incident: [Service] [Issue Date/Time]

## Initial Symptoms
- Service: [API/Database/Cache/etc]
- Symptom: [Slow/Down/Error]
- Detection: [Monitoring/Customer report/etc]
- Severity: [Critical/High/Medium/Low]

## Troubleshooting Steps
1. Checked [system] - Found [status]
2. Checked [system] - Found [status]
3. Checked [system] - Found [status]

## Root Cause
[What was actually wrong]

## Resolution
[What you did to fix it]

## Verification
[How you confirmed it's fixed]

## Timeline
- HH:MM - Symptom started
- HH:MM - Issue detected
- HH:MM - Troubleshooting started
- HH:MM - Root cause found
- HH:MM - Fixed
- HH:MM - Verified

## Follow-up Actions
- [ ] Monitor system for regression
- [ ] Update runbook with findings
- [ ] Schedule post-mortem if critical
- [ ] Track permanent fix in backlog
```

---

## Quick Reference: Common Commands

### Docker
```bash
docker stats --no-stream              # Resource usage
docker logs <container> -n 100        # Last 100 lines
docker exec <container> <command>     # Run command in container
docker restart <container>            # Restart service
```

### Database
```bash
docker exec postgres psql -U risk_user -d openrisk -c "SELECT ..."
docker exec postgres pg_dump -U risk_user openrisk > backup.sql
```

### Cache
```bash
redis-cli INFO stats
redis-cli MONITOR
redis-cli FLUSHALL
redis-cli CONFIG GET *
```

### Network
```bash
curl -w "@curl-format.txt" -o /dev/null https://api.openrisk.com/health
ping api.openrisk.com
traceroute api.openrisk.com
nslookup api.openrisk.com
```

---

**Document Version**: 1.0
**Last Updated**: February 22, 2026
**Next Review**: March 22, 2026
**Owner**: DevOps Team
