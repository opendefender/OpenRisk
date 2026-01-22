# Monitoring Stack Setup Guide

## Overview

The monitoring stack provides real-time performance visibility for OpenRisk performance optimization. It consists of:

- **Prometheus** - Metrics collection and storage
- **Redis Exporter** - Redis performance metrics
- **PostgreSQL Exporter** - Database metrics  
- **Grafana** - Visualization dashboards
- **AlertManager** - Alert routing and notifications

## Quick Start

### 1. Start the Stack

```bash
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d
```

### 2. Access Dashboards

```
Grafana:      http://localhost:3001  (admin/admin)
Prometheus:   http://localhost:9090
AlertManager: http://localhost:9093
```

### 3. Import Dashboard

- Login to Grafana
- Navigate to Dashboards → Import
- Upload: `deployment/monitoring/grafana/dashboards/openrisk-performance.json`
- Select Prometheus as datasource

## Stack Components

### Prometheus (port 9090)
**Role**: Central metrics collection and storage

**Configuration**: `deployment/monitoring/prometheus.yml`
- Scrapes metrics every 15 seconds
- Retains data for 30 days
- Integrates with AlertManager

**Scrape Targets**:
- Redis Exporter (9121) - Cache metrics
- PostgreSQL Exporter (9187) - Database metrics
- Prometheus (9090) - Self-monitoring

### Redis Exporter (port 9121)
**Role**: Exports Redis instance metrics to Prometheus

**Key Metrics**:
```
redis_connected_clients      - Number of connected clients
redis_memory_used_bytes      - Memory usage in bytes
redis_keys_total            - Total number of keys
redis_keyspace_hits_total   - Cache hits
redis_keyspace_misses_total - Cache misses
redis_commands_processed_total - Commands executed
```

**Configuration**: 
```yaml
environment:
  REDIS_ADDR: redis://:redis123@redis:6379
```

### PostgreSQL Exporter (port 9187)
**Role**: Exports PostgreSQL metrics to Prometheus

**Key Metrics**:
```
pg_stat_activity_count                 - Active connections
pg_stat_statements_calls                - Query call count
pg_stat_statements_mean_time           - Average query time (ms)
pg_stat_statements_max_time            - Max query time (ms)
pg_connections_max                     - Connection pool limit
```

**Configuration**:
```yaml
environment:
  DATA_SOURCE_NAME: postgresql://openrisk:password@postgres:5432/openrisk
```

### Grafana (port 3001)
**Role**: Visual dashboard for metrics and alerts

**Default Credentials**: admin/admin

**Provisioned Elements**:
- Datasource: Prometheus (auto-configured)
- Dashboard: OpenRisk Performance (auto-loaded)

**Key Panels**:
1. **Redis Operations Rate** - Operations per second
2. **Cache Hit Ratio** - Hit vs miss percentage
3. **Redis Memory Usage** - Memory consumption trend
4. **PostgreSQL Connections** - Active connection count
5. **Database Query Performance** - Query latency
6. **Query Throughput** - Queries per second

### AlertManager (port 9093)
**Role**: Routes and manages alerts

**Configuration**: `deployment/monitoring/alertmanager.yml`

**Features**:
- Inhibition rules (suppress warnings when critical active)
- Slack integration for notifications
- Alert grouping and deduplication
- Different channels for critical vs warning alerts

## Alert Rules

### Alert Definitions (`deployment/monitoring/alerts.yml`)

#### 1. LowCacheHitRate (WARNING)
```
Condition: Cache hit rate < 75% for 5 minutes
Action: Notifies #performance-alerts on Slack
```
**Why**: Indicates caching not effective, query efficiency poor

#### 2. HighRedisMemory (CRITICAL)
```
Condition: Redis memory > 85% for 5 minutes
Action: Notifies #critical-alerts on Slack
```
**Why**: Redis approaching eviction limits, may lose cached data

#### 3. HighDatabaseConnections (WARNING)
```
Condition: Active connections > 40 for 5 minutes
Action: Notifies #performance-alerts on Slack
```
**Why**: Approaching pool limit (50), connection pool exhaustion risk

#### 4. SlowDatabaseQueries (WARNING)
```
Condition: Average query time > 1 second for 5 minutes
Action: Notifies #performance-alerts on Slack
```
**Why**: Indicates query performance issues, need for optimization

## Configuration

### Environment Variables

Create a `.env` file in the `deployment/` directory:

```env
# Database
DB_USER=openrisk
DB_PASSWORD=secure_password
DB_NAME=openrisk

# Redis
REDIS_PASSWORD=redis_secure_password

# Grafana
GRAFANA_USER=admin
GRAFANA_PASSWORD=secure_grafana_password

# Slack (for AlertManager)
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
```

### Customizing Alert Thresholds

Edit `deployment/monitoring/alerts.yml`:

```yaml
# Example: Change cache hit rate threshold to 80%
- alert: LowCacheHitRate
  expr: (redis_keyspace_hits_total / (redis_keyspace_hits_total + redis_keyspace_misses_total)) < 0.80
  for: 5m
```

### Customizing TTLs

Edit `deployment/monitoring/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s  # Change collection interval
  retention_time: 30d   # Change retention period
```

## Usage Scenarios

### Scenario 1: Verify Cache Effectiveness

1. **Start monitoring**:
   ```bash
   docker-compose -f docker-compose-monitoring.yaml up -d
   ```

2. **Run load test**:
   ```bash
   k6 run ./load_tests/cache_test.js
   ```

3. **Check dashboard** at http://localhost:3001
   - Look for "Cache Hit Ratio" pie chart
   - Target: 75%+ after warm-up period

### Scenario 2: Monitor Database Performance

1. **Watch "Database Query Performance" chart**
   - Should show decreasing trend with caching
   - P95 latency should be < 100ms

2. **Watch "Query Throughput" chart**
   - Should increase with connection pooling
   - Target: > 1000 req/s

### Scenario 3: Response to Alerts

**If HighRedisMemory Alert Fires**:
```bash
# 1. Check Redis memory
redis-cli INFO memory

# 2. Check cache size
redis-cli INFO keyspace

# 3. Solutions:
# - Reduce cache TTL in cache_integration.go
# - Reduce query parameter variations (cache keys)
# - Implement cache eviction policy
```

**If HighDatabaseConnections Alert Fires**:
```bash
# 1. Check active connections
curl http://localhost:9187/metrics | grep pg_stat_activity_count

# 2. Solutions:
# - Improve cache hit rate
# - Reduce query complexity
# - Increase connection pool size (in pool_config.go)
```

## Troubleshooting

### Prometheus not scraping metrics

**Symptom**: No data in Prometheus UI

**Solution**:
```bash
# Check Prometheus logs
docker-compose -f docker-compose-monitoring.yaml logs prometheus

# Verify exporter is running
curl http://localhost:9121/metrics  # Redis
curl http://localhost:9187/metrics  # PostgreSQL

# Restart stack
docker-compose -f docker-compose-monitoring.yaml restart prometheus
```

### Grafana dashboard empty

**Symptom**: Dashboard shows "No data"

**Solution**:
```bash
# 1. Verify Prometheus datasource
# Grafana UI → Configuration → Datasources → Prometheus
# Test connection

# 2. Check query syntax in dashboard
# Edit panel → Check PromQL query

# 3. Ensure metrics exist in Prometheus
# Prometheus UI → Graph → Query
# Try: redis_memory_used_bytes
```

### Alerts not firing

**Symptom**: Alert condition met but no notification

**Solution**:
```bash
# 1. Check AlertManager logs
docker-compose -f docker-compose-monitoring.yaml logs alertmanager

# 2. Verify alert rules syntax
# Prometheus UI → Alerts → Check rule status

# 3. Check Slack webhook URL
# Edit deployment/monitoring/alertmanager.yml
# Verify SLACK_WEBHOOK_URL environment variable
```

### High memory usage

**Symptom**: Prometheus using > 2GB memory

**Solution**:
```yaml
# In docker-compose-monitoring.yaml, add memory limit:
prometheus:
  deploy:
    resources:
      limits:
        memory: 2G
```

Or reduce retention:
```yaml
command:
  - '--storage.tsdb.retention.time=7d'  # Changed from 30d
```

## Backup and Restore

### Backup Prometheus Data

```bash
# Stop container, copy volume
docker-compose -f docker-compose-monitoring.yaml down
docker run --rm -v prometheus_data:/data -v $(pwd):/backup \
  alpine tar czf /backup/prometheus-backup.tar.gz -C /data .

# Restore
docker run --rm -v prometheus_data:/data -v $(pwd):/backup \
  alpine tar xzf /backup/prometheus-backup.tar.gz -C /data
docker-compose -f docker-compose-monitoring.yaml up -d
```

### Backup Grafana Dashboards

```bash
# Export dashboard via UI
# Grafana → Dashboard → Share → Export

# Or use API
curl http://localhost:3001/api/dashboards/uid/openrisk-performance \
  -H "Authorization: Bearer ${GRAFANA_API_KEY}" > dashboard-backup.json
```

## Performance Considerations

### Metrics Retention
- **30 days** (default) = ~50GB storage
- **7 days** = ~12GB storage
- **1 day** = ~2GB storage

### Scrape Overhead
- **15 second interval** (default) = ~5% overhead
- **30 second interval** = ~2.5% overhead
- **60 second interval** = ~1% overhead

### Recommendation for Production

```yaml
# deployment/monitoring/prometheus.yml
global:
  scrape_interval: 30s        # Balance detail with overhead
  retention_time: 14d         # Balance storage with history

# deployment/docker-compose-monitoring.yaml
prometheus:
  deploy:
    resources:
      limits:
        memory: 2G
        cpus: '1.0'
```

## Integration with Application

### Exposing Custom Metrics (Optional)

Add metrics export in your Go backend:

```go
import "github.com/prometheus/client_golang/prometheus"

// Register custom metric
var requestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "openrisk_request_duration_seconds",
        Help: "Request duration in seconds",
    },
    []string{"method", "endpoint"},
)

// Export endpoint
app.Get("/metrics", func(c *fiber.Ctx) error {
    // Prometheus scrapes this endpoint
    return c.Send(prometheus.Handler().ServeHTTP(...))
})
```

Then add to `prometheus.yml`:
```yaml
scrape_configs:
  - job_name: 'openrisk-backend'
    static_configs:
      - targets: ['localhost:2112']
```

## Advanced Topics

### Custom Alert Rules

Edit `deployment/monitoring/alerts.yml` to add:

```yaml
- alert: CustomAlert
  expr: custom_metric > 100
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Custom condition triggered"
    description: "Custom metric is {{ $value }}"
```

### Advanced Dashboards

1. Create dashboard in Grafana UI
2. Add panels with PromQL queries:
   ```
   # Cache hit rate percentage
   (redis_keyspace_hits_total / (redis_keyspace_hits_total + redis_keyspace_misses_total)) * 100
   
   # Query latency by percentile
   histogram_quantile(0.95, request_duration_seconds_bucket)
   
   # Memory growth rate
   rate(redis_memory_used_bytes[5m])
   ```
3. Export and save to `deployment/monitoring/grafana/dashboards/`

## References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [AlertManager Documentation](https://prometheus.io/docs/alerting/latest/overview/)
- [Redis Exporter](https://github.com/oliver006/redis_exporter)
- [PostgreSQL Exporter](https://github.com/prometheus-community/postgres_exporter)
