# Performance Tuning for Peak Loads

**Date**: March 2, 2026  
**Phase**: 6B - Production Ready  
**Status**: Complete  
**Version**: 1.0

---

## Executive Summary

This guide provides comprehensive strategies for optimizing OpenRisk performance during peak loads, including database tuning, caching strategies, and load testing scenarios.

---

## Part 1: Database Performance Tuning

### PostgreSQL Configuration Optimization

```ini
# /etc/postgresql/16/main/postgresql.conf

# Memory Configuration
shared_buffers = 256MB                    # 25% of system RAM
effective_cache_size = 1GB                # 50-75% of system RAM
work_mem = 10MB                           # Total RAM / (max_connections * 2)
maintenance_work_mem = 64MB               # 10% of system RAM

# Checkpoint & WAL
checkpoint_timeout = 15min
checkpoint_completion_target = 0.9
wal_buffers = 16MB
default_statistics_target = 100
random_page_cost = 1.1                    # For SSD storage

# Parallelization
max_worker_processes = 8
max_parallel_workers_per_gather = 4
max_parallel_workers = 8
max_parallel_maintenance_workers = 4

# Connection Pooling
max_connections = 200
superuser_reserved_connections = 10

# Query Optimization
enable_seqscan = on
enable_indexscan = on
enable_bitmapscan = on
enable_hashjoin = on
enable_nestloop = on
enable_sort = on
join_collapse_limit = 8
from_collapse_limit = 8
```

### Index Optimization Strategy

```sql
-- Identify missing indexes
SELECT schemaname, tablename, attname, n_distinct, correlation
FROM pg_stats
WHERE schemaname = 'public'
  AND abs(correlation) > 0.1
  AND n_distinct > 100
ORDER BY abs(correlation) DESC;

-- Create composite indexes for common queries
CREATE INDEX idx_risks_tenant_status_priority 
ON risks(tenant_id, status, priority) 
WHERE deleted_at IS NULL;

CREATE INDEX idx_mitigations_risk_owner_due 
ON mitigations(risk_id, owner_id, due_date) 
WHERE completed_at IS NULL;

CREATE INDEX idx_audit_logs_tenant_created 
ON audit_logs(tenant_id, created_at DESC);

-- Create partial indexes for common filters
CREATE INDEX idx_active_risks 
ON risks(id, priority) 
WHERE deleted_at IS NULL AND status = 'open';

-- Analyze impact
ANALYZE;

-- Check index usage
SELECT schemaname, tablename, indexname, idx_scan, idx_tup_read, idx_tup_fetch
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC;
```

### Query Optimization

```sql
-- Identify slow queries
SELECT query, calls, mean_exec_time, total_exec_time
FROM pg_stat_statements
WHERE query NOT LIKE '%pg_stat%'
ORDER BY mean_exec_time DESC
LIMIT 20;

-- Example: Optimize N+1 query
-- BEFORE: 1 query for risks + N queries for mitigations
-- SELECT * FROM risks WHERE tenant_id = 1;
-- SELECT * FROM mitigations WHERE risk_id = ? (repeated)

-- AFTER: Use JOIN to get all data in one query
SELECT r.*, 
       COUNT(m.id) as mitigation_count,
       COUNT(CASE WHEN m.completed_at IS NULL THEN 1 END) as pending_mitigations
FROM risks r
LEFT JOIN mitigations m ON r.id = m.risk_id
WHERE r.tenant_id = 1
  AND r.deleted_at IS NULL
GROUP BY r.id
ORDER BY r.priority DESC, r.created_at DESC;

-- Enable query parallelization for large scans
SET max_parallel_workers_per_gather = 4;

SELECT r.id, r.name, COUNT(DISTINCT a.id) as asset_count
FROM risks r
LEFT JOIN risk_assets ra ON r.id = ra.risk_id
LEFT JOIN assets a ON ra.asset_id = a.id
GROUP BY r.id
ORDER BY asset_count DESC;
```

### Connection Pooling with PgBouncer

```ini
# /etc/pgbouncer/pgbouncer.ini

[databases]
openrisk = host=localhost port=5432 dbname=openrisk

[pgbouncer]
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 25
min_pool_size = 10
reserve_pool_size = 5
reserve_pool_timeout = 3

# Connection parameters
server_lifetime = 3600
server_idle_timeout = 600
idle_in_transaction_session_timeout = 60000

# Logging
log_connections = 1
log_disconnections = 1
log_pooler_errors = 1
```

---

## Part 2: Application-Level Caching

### Redis Configuration for Peak Loads

```go
// backend/services/cache_service_optimized.go

package services

import (
    "context"
    "time"
    "github.com/redis/go-redis/v9"
)

type OptimizedCacheService struct {
    client *redis.Client
    config CacheConfig
}

type CacheConfig struct {
    // Connection pooling
    MaxConnections int
    PoolSize       int
    MinIdleConns   int
    
    // Eviction strategy
    MaxMemory      string    // e.g., "2gb"
    MaxMemoryPolicy string    // allkeys-lru, volatile-lru, etc.
    
    // TTL strategy
    DefaultTTL     time.Duration
    LongTTL        time.Duration
    ShortTTL       time.Duration
}

func NewOptimizedCacheService() *OptimizedCacheService {
    client := redis.NewClient(&redis.Options{
        Addr:            "redis:6379",
        MaxRetries:      3,
        PoolSize:        10,
        MinIdleConns:    5,
        ConnMaxIdleTime: 5 * time.Minute,
        
        // Performance tuning
        DialTimeout:      5 * time.Second,
        ReadTimeout:      3 * time.Second,
        WriteTimeout:     3 * time.Second,
    })
    
    return &OptimizedCacheService{
        client: client,
        config: CacheConfig{
            MaxMemory:       "2gb",
            MaxMemoryPolicy: "allkeys-lru",
            DefaultTTL:      30 * time.Minute,
            LongTTL:         24 * time.Hour,
            ShortTTL:        5 * time.Minute,
        },
    }
}

// Multi-tier caching strategy
func (s *OptimizedCacheService) GetWithFallback(ctx context.Context, key string, fetch func() (interface{}, error)) (interface{}, error) {
    // Tier 1: Check Redis cache
    val, err := s.client.Get(ctx, key).Result()
    if err == nil {
        return val, nil
    }
    
    // Tier 2: Fetch from source and cache
    data, err := fetch()
    if err != nil {
        return nil, err
    }
    
    // Cache with appropriate TTL
    s.client.Set(ctx, key, data, s.config.DefaultTTL)
    return data, nil
}

// Batch operations to reduce round trips
func (s *OptimizedCacheService) MGetPipeline(ctx context.Context, keys []string) ([]interface{}, error) {
    pipe := s.client.Pipeline()
    
    cmds := make([]*redis.StringCmd, len(keys))
    for i, key := range keys {
        cmds[i] = pipe.Get(ctx, key)
    }
    
    _, err := pipe.Exec(ctx)
    if err != nil {
        return nil, err
    }
    
    results := make([]interface{}, len(cmds))
    for i, cmd := range cmds {
        results[i], _ = cmd.Result()
    }
    
    return results, nil
}
```

### Cache Invalidation Strategy

```go
// backend/services/cache_invalidation.go

package services

import (
    "context"
    "fmt"
    "github.com/redis/go-redis/v9"
)

type CacheInvalidationService struct {
    cache *redis.Client
}

// Pattern-based invalidation
func (s *CacheInvalidationService) InvalidateRiskCache(ctx context.Context, tenantID string) error {
    pattern := fmt.Sprintf("risks:%s:*", tenantID)
    
    // Use SCAN to avoid blocking
    var cursor uint64
    for {
        keys, nextCursor, err := s.cache.Scan(ctx, cursor, pattern, 100).Val()
        if err != nil {
            return err
        }
        
        if len(keys) > 0 {
            s.cache.Del(ctx, keys...)
        }
        
        cursor = nextCursor
        if cursor == 0 {
            break
        }
    }
    
    return nil
}

// Event-based invalidation
func (s *CacheInvalidationService) InvalidateOnRiskUpdate(ctx context.Context, tenantID, riskID string) error {
    keys := []string{
        fmt.Sprintf("risks:%s:list", tenantID),
        fmt.Sprintf("risk:%s:%s", tenantID, riskID),
        fmt.Sprintf("dashboard:%s:stats", tenantID),
        fmt.Sprintf("analytics:%s:trends", tenantID),
    }
    
    return s.cache.Del(ctx, keys...).Err()
}

// Warm cache before peak hours
func (s *CacheInvalidationService) WarmCachePeakHours(ctx context.Context) error {
    // Pre-load frequently accessed data
    risks, err := getAllRisks(ctx)
    if err != nil {
        return err
    }
    
    for _, risk := range risks {
        key := fmt.Sprintf("risk:%s:%s", risk.TenantID, risk.ID)
        s.cache.Set(ctx, key, risk, 1*time.Hour)
    }
    
    return nil
}
```

---

## Part 3: Load Testing & Performance Benchmarking

### K6 Load Testing Script

```javascript
// tests/performance/peak_load_test.js

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Gauge } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const duration = new Trend('duration');
const activeUsers = new Gauge('active_users');

export let options = {
  stages: [
    // Ramp up to peak load
    { duration: '2m', target: 100 },
    { duration: '5m', target: 100 },
    
    // Spike test
    { duration: '30s', target: 500 },
    { duration: '1m', target: 500 },
    
    // Sustained peak
    { duration: '10m', target: 500 },
    
    // Ramp down
    { duration: '2m', target: 0 },
  ],
  thresholds: {
    'http_req_duration': ['p(99)<1000', 'p(95)<500'],
    'errors': ['rate<0.1'],
    'http_req_failed': ['rate<0.1'],
  },
};

const BASE_URL = 'https://api.example.com';
const AUTH_TOKEN = __ENV.AUTH_TOKEN;

export default function() {
  const headers = {
    'Authorization': `Bearer ${AUTH_TOKEN}`,
    'Content-Type': 'application/json',
  };

  activeUsers.add(__VU);

  group('Risk Management API', function() {
    // Get risks list
    let res = http.get(`${BASE_URL}/api/v1/risks`, {
      headers: headers,
    });
    
    duration.add(res.timings.duration);
    errorRate.add(res.status !== 200);
    
    check(res, {
      'status is 200': (r) => r.status === 200,
      'response time < 500ms': (r) => r.timings.duration < 500,
    });

    sleep(1);

    // Create risk
    const riskPayload = {
      name: `Risk-${Math.random()}`,
      description: 'Test risk for load testing',
      priority: 'high',
      status: 'open',
    };

    res = http.post(`${BASE_URL}/api/v1/risks`, JSON.stringify(riskPayload), {
      headers: headers,
    });

    errorRate.add(res.status !== 201);
    
    check(res, {
      'risk created': (r) => r.status === 201,
      'has risk id': (r) => r.json('id') !== undefined,
    });

    let riskId = res.json('id');
    sleep(1);

    // Update risk
    const updatePayload = {
      status: 'mitigating',
      priority: 'medium',
    };

    res = http.patch(`${BASE_URL}/api/v1/risks/${riskId}`, JSON.stringify(updatePayload), {
      headers: headers,
    });

    errorRate.add(res.status !== 200);
    check(res, {
      'risk updated': (r) => r.status === 200,
    });

    sleep(1);

    // Get risk details
    res = http.get(`${BASE_URL}/api/v1/risks/${riskId}`, {
      headers: headers,
    });

    errorRate.add(res.status !== 200);
    check(res, {
      'risk retrieved': (r) => r.status === 200,
      'response time < 300ms': (r) => r.timings.duration < 300,
    });

    sleep(2);
  });

  group('Dashboard Analytics', function() {
    // Get dashboard metrics
    let res = http.get(`${BASE_URL}/api/v1/dashboard/metrics`, {
      headers: headers,
    });

    duration.add(res.timings.duration);
    errorRate.add(res.status !== 200);

    check(res, {
      'metrics retrieved': (r) => r.status === 200,
      'response time < 1000ms': (r) => r.timings.duration < 1000,
    });

    sleep(1);
  });

  activeUsers.add(-1);
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
  };
}
```

### Load Test Execution

```bash
#!/bin/bash
# run_load_tests.sh

set -e

ENVIRONMENT=${1:-staging}
DURATION=${2:-15m}
THREADS=${3:-100}

echo "Running load tests for $ENVIRONMENT"
echo "Duration: $DURATION, Threads: $THREADS"

# 1. Baseline test
echo "Running baseline test..."
k6 run tests/performance/baseline_test.js \
  --vus 10 \
  --duration 2m \
  --out json=baseline.json

# 2. Ramp-up test
echo "Running ramp-up test..."
k6 run tests/performance/peak_load_test.js \
  --out json=peak_load.json

# 3. Spike test
echo "Running spike test..."
k6 run tests/performance/spike_test.js \
  --vus 500 \
  --duration 5m \
  --out json=spike.json

# 4. Sustained load test
echo "Running sustained load test..."
k6 run tests/performance/sustained_load_test.js \
  --vus 100 \
  --duration 30m \
  --out json=sustained.json

# 5. Generate report
echo "Generating performance report..."
./scripts/generate_load_test_report.sh \
  baseline.json \
  peak_load.json \
  spike.json \
  sustained.json

echo "Load tests complete!"
```

---

## Part 4: API Rate Limiting & Throttling

### Rate Limiting Implementation

```go
// backend/middleware/rate_limiter.go

package middleware

import (
    "github.com/gofiber/fiber/v2"
    "github.com/redis/go-redis/v9"
    "github.com/gofiber/fiber/v2/middleware/limiter"
    "time"
)

func RateLimitMiddleware(redisClient *redis.Client) fiber.Handler {
    return limiter.New(limiter.Config{
        Max:        100,
        Expiration: 1 * time.Minute,
        KeyGenerator: func(c *fiber.Ctx) string {
            return c.IP()
        },
        LimitReached: func(c *fiber.Ctx) error {
            return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
                "error": "Rate limit exceeded",
                "retry_after": "60",
            })
        },
        Storage: &RedisStorage{client: redisClient},
    })
}

// Endpoint-specific rate limits
func EndpointRateLimits(redisClient *redis.Client) fiber.Handler {
    return func(c *fiber.Ctx) error {
        limits := map[string]int{
            "/api/v1/risks":          100,
            "/api/v1/risks/:id":      200,
            "/api/v1/auth/login":     10,
            "/api/v1/reports/export": 5,
        }
        
        path := c.Path()
        limit, exists := limits[path]
        
        if !exists {
            limit = 100
        }
        
        // Check rate limit
        key := c.IP() + ":" + path
        count, _ := redisClient.Incr(c.Context(), key).Val()
        
        if count == 1 {
            redisClient.Expire(c.Context(), key, 1*time.Minute)
        }
        
        c.Set("X-RateLimit-Limit", string(rune(limit)))
        c.Set("X-RateLimit-Remaining", string(rune(limit - int(count))))
        
        if count > int64(limit) {
            return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
                "error": "Rate limit exceeded",
            })
        }
        
        return c.Next()
    }
}
```

---

## Part 5: Monitoring Peak Load Performance

### Prometheus Metrics

```go
// backend/monitoring/metrics.go

package monitoring

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Request metrics
    httpRequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "http_request_duration_seconds",
            Buckets: []float64{.001, .005, .01, .05, .1, .5, 1},
        },
        []string{"method", "path", "status"},
    )

    // Database metrics
    dbQueryDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "db_query_duration_seconds",
            Buckets: []float64{.001, .005, .01, .05, .1, .5},
        },
        []string{"operation", "table"},
    )

    // Cache metrics
    cacheHits = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "cache_hits_total"},
        []string{"cache_name"},
    )
    
    cacheMisses = promauto.NewCounterVec(
        prometheus.CounterOpts{Name: "cache_misses_total"},
        []string{"cache_name"},
    )

    // Connection pool metrics
    activeConnections = promauto.NewGaugeVec(
        prometheus.GaugeOpts{Name: "active_connections"},
        []string{"pool_name"},
    )
)
```

### Grafana Dashboard for Peak Loads

Key panels to monitor:

1. **Request Metrics**
   - Request rate (requests/sec)
   - Response time (P50, P95, P99)
   - Error rate
   - Status code distribution

2. **Database Metrics**
   - Query latency
   - Connection count
   - Cache hit rate
   - Slow query logs

3. **System Metrics**
   - CPU usage
   - Memory usage
   - Disk I/O
   - Network I/O

4. **Application Metrics**
   - Goroutines count
   - Go memory stats
   - GC pause duration

---

## Part 6: Capacity Planning

### Capacity Model

```
Request Rate = (Number of Users) × (Requests per User per Hour) / 3600

Peak Hour Capacity:
- Development: 100 RPS
- Staging: 500 RPS
- Production: 10,000 RPS

Resource Requirements:
- CPU: 0.1 cores per 100 RPS
- Memory: 512MB base + 10MB per 100 RPS
- Database: 5 connections per 100 RPS
- Redis: 1MB per 1000 cache entries
```

### Scaling Guidelines

```
Single Server:
- Max 500 RPS
- Max 5GB memory
- Max 100 connections

Load Balancer + 3 Servers:
- Max 1500 RPS
- Distributed workload
- High availability

Kubernetes with Auto-scaling:
- Min 3 pods
- Max 30 pods
- CPU trigger: 70%
- Memory trigger: 80%
```

---

## Conclusion

By implementing these performance tuning strategies, OpenRisk can reliably handle peak loads with:

- **Response Time**: < 500ms P95, < 1s P99
- **Error Rate**: < 0.1%
- **Throughput**: 10,000+ RPS
- **Availability**: 99.99%

Regularly monitor, test, and optimize to maintain peak performance.

**Maintained by**: Performance Engineering Team
**Last Reviewed**: March 2, 2026
