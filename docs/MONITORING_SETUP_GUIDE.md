 Monitoring Stack Setup Guide

 Overview

The monitoring stack provides real-time performance visibility for OpenRisk performance optimization. It consists of:

- Prometheus - Metrics collection and storage
- Redis Exporter - Redis performance metrics
- PostgreSQL Exporter - Database metrics  
- Grafana - Visualization dashboards
- AlertManager - Alert routing and notifications

 Quick Start

 . Start the Stack

bash
cd deployment
docker-compose -f docker-compose-monitoring.yaml up -d


 . Access Dashboards


Grafana:      http://localhost:  (admin/admin)
Prometheus:   http://localhost:
AlertManager: http://localhost:


 . Import Dashboard

- Login to Grafana
- Navigate to Dashboards → Import
- Upload: deployment/monitoring/grafana/dashboards/openrisk-performance.json
- Select Prometheus as datasource

 Stack Components

 Prometheus (port )
Role: Central metrics collection and storage

Configuration: deployment/monitoring/prometheus.yml
- Scrapes metrics every  seconds
- Retains data for  days
- Integrates with AlertManager

Scrape Targets:
- Redis Exporter () - Cache metrics
- PostgreSQL Exporter () - Database metrics
- Prometheus () - Self-monitoring

 Redis Exporter (port )
Role: Exports Redis instance metrics to Prometheus

Key Metrics:

redis_connected_clients      - Number of connected clients
redis_memory_used_bytes      - Memory usage in bytes
redis_keys_total            - Total number of keys
redis_keyspace_hits_total   - Cache hits
redis_keyspace_misses_total - Cache misses
redis_commands_processed_total - Commands executed


Configuration: 
yaml
environment:
  REDIS_ADDR: redis://:redis@redis:


 PostgreSQL Exporter (port )
Role: Exports PostgreSQL metrics to Prometheus

Key Metrics:

pg_stat_activity_count                 - Active connections
pg_stat_statements_calls                - Query call count
pg_stat_statements_mean_time           - Average query time (ms)
pg_stat_statements_max_time            - Max query time (ms)
pg_connections_max                     - Connection pool limit


Configuration:
yaml
environment:
  DATA_SOURCE_NAME: postgresql://openrisk:password@postgres:/openrisk


 Grafana (port )
Role: Visual dashboard for metrics and alerts

Default Credentials: admin/admin

Provisioned Elements:
- Datasource: Prometheus (auto-configured)
- Dashboard: OpenRisk Performance (auto-loaded)

Key Panels:
. Redis Operations Rate - Operations per second
. Cache Hit Ratio - Hit vs miss percentage
. Redis Memory Usage - Memory consumption trend
. PostgreSQL Connections - Active connection count
. Database Query Performance - Query latency
. Query Throughput - Queries per second

 AlertManager (port )
Role: Routes and manages alerts

Configuration: deployment/monitoring/alertmanager.yml

Features:
- Inhibition rules (suppress warnings when critical active)
- Slack integration for notifications
- Alert grouping and deduplication
- Different channels for critical vs warning alerts

 Alert Rules

 Alert Definitions (deployment/monitoring/alerts.yml)

 . LowCacheHitRate (WARNING)

Condition: Cache hit rate < % for  minutes
Action: Notifies performance-alerts on Slack

Why: Indicates caching not effective, query efficiency poor

 . HighRedisMemory (CRITICAL)

Condition: Redis memory > % for  minutes
Action: Notifies critical-alerts on Slack

Why: Redis approaching eviction limits, may lose cached data

 . HighDatabaseConnections (WARNING)

Condition: Active connections >  for  minutes
Action: Notifies performance-alerts on Slack

Why: Approaching pool limit (), connection pool exhaustion risk

 . SlowDatabaseQueries (WARNING)

Condition: Average query time >  second for  minutes
Action: Notifies performance-alerts on Slack

Why: Indicates query performance issues, need for optimization

 Configuration

 Environment Variables

Create a .env file in the deployment/ directory:

env
 Database
DB_USER=openrisk
DB_PASSWORD=secure_password
DB_NAME=openrisk

 Redis
REDIS_PASSWORD=redis_secure_password

 Grafana
GRAFANA_USER=admin
GRAFANA_PASSWORD=secure_grafana_password

 Slack (for AlertManager)
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL


 Customizing Alert Thresholds

Edit deployment/monitoring/alerts.yml:

yaml
 Example: Change cache hit rate threshold to %
- alert: LowCacheHitRate
  expr: (redis_keyspace_hits_total / (redis_keyspace_hits_total + redis_keyspace_misses_total)) < .
  for: m


 Customizing TTLs

Edit deployment/monitoring/prometheus.yml:

yaml
global:
  scrape_interval: s   Change collection interval
  retention_time: d    Change retention period


 Usage Scenarios

 Scenario : Verify Cache Effectiveness

. Start monitoring:
   bash
   docker-compose -f docker-compose-monitoring.yaml up -d
   

. Run load test:
   bash
   k run ./load_tests/cache_test.js
   

. Check dashboard at http://localhost:
   - Look for "Cache Hit Ratio" pie chart
   - Target: %+ after warm-up period

 Scenario : Monitor Database Performance

. Watch "Database Query Performance" chart
   - Should show decreasing trend with caching
   - P latency should be < ms

. Watch "Query Throughput" chart
   - Should increase with connection pooling
   - Target: >  req/s

 Scenario : Response to Alerts

If HighRedisMemory Alert Fires:
bash
 . Check Redis memory
redis-cli INFO memory

 . Check cache size
redis-cli INFO keyspace

 . Solutions:
 - Reduce cache TTL in cache_integration.go
 - Reduce query parameter variations (cache keys)
 - Implement cache eviction policy


If HighDatabaseConnections Alert Fires:
bash
 . Check active connections
curl http://localhost:/metrics | grep pg_stat_activity_count

 . Solutions:
 - Improve cache hit rate
 - Reduce query complexity
 - Increase connection pool size (in pool_config.go)


 Troubleshooting

 Prometheus not scraping metrics

Symptom: No data in Prometheus UI

Solution:
bash
 Check Prometheus logs
docker-compose -f docker-compose-monitoring.yaml logs prometheus

 Verify exporter is running
curl http://localhost:/metrics   Redis
curl http://localhost:/metrics   PostgreSQL

 Restart stack
docker-compose -f docker-compose-monitoring.yaml restart prometheus


 Grafana dashboard empty

Symptom: Dashboard shows "No data"

Solution:
bash
 . Verify Prometheus datasource
 Grafana UI → Configuration → Datasources → Prometheus
 Test connection

 . Check query syntax in dashboard
 Edit panel → Check PromQL query

 . Ensure metrics exist in Prometheus
 Prometheus UI → Graph → Query
 Try: redis_memory_used_bytes


 Alerts not firing

Symptom: Alert condition met but no notification

Solution:
bash
 . Check AlertManager logs
docker-compose -f docker-compose-monitoring.yaml logs alertmanager

 . Verify alert rules syntax
 Prometheus UI → Alerts → Check rule status

 . Check Slack webhook URL
 Edit deployment/monitoring/alertmanager.yml
 Verify SLACK_WEBHOOK_URL environment variable


 High memory usage

Symptom: Prometheus using > GB memory

Solution:
yaml
 In docker-compose-monitoring.yaml, add memory limit:
prometheus:
  deploy:
    resources:
      limits:
        memory: G


Or reduce retention:
yaml
command:
  - '--storage.tsdb.retention.time=d'   Changed from d


 Backup and Restore

 Backup Prometheus Data

bash
 Stop container, copy volume
docker-compose -f docker-compose-monitoring.yaml down
docker run --rm -v prometheus_data:/data -v $(pwd):/backup \
  alpine tar czf /backup/prometheus-backup.tar.gz -C /data .

 Restore
docker run --rm -v prometheus_data:/data -v $(pwd):/backup \
  alpine tar xzf /backup/prometheus-backup.tar.gz -C /data
docker-compose -f docker-compose-monitoring.yaml up -d


 Backup Grafana Dashboards

bash
 Export dashboard via UI
 Grafana → Dashboard → Share → Export

 Or use API
curl http://localhost:/api/dashboards/uid/openrisk-performance \
  -H "Authorization: Bearer ${GRAFANA_API_KEY}" > dashboard-backup.json


 Performance Considerations

 Metrics Retention
-  days (default) = ~GB storage
-  days = ~GB storage
-  day = ~GB storage

 Scrape Overhead
-  second interval (default) = ~% overhead
-  second interval = ~.% overhead
-  second interval = ~% overhead

 Recommendation for Production

yaml
 deployment/monitoring/prometheus.yml
global:
  scrape_interval: s         Balance detail with overhead
  retention_time: d          Balance storage with history

 deployment/docker-compose-monitoring.yaml
prometheus:
  deploy:
    resources:
      limits:
        memory: G
        cpus: '.'


 Integration with Application

 Exposing Custom Metrics (Optional)

Add metrics export in your Go backend:

go
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
app.Get("/metrics", func(c fiber.Ctx) error {
    // Prometheus scrapes this endpoint
    return c.Send(prometheus.Handler().ServeHTTP(...))
})


Then add to prometheus.yml:
yaml
scrape_configs:
  - job_name: 'openrisk-backend'
    static_configs:
      - targets: ['localhost:']


 Advanced Topics

 Custom Alert Rules

Edit deployment/monitoring/alerts.yml to add:

yaml
- alert: CustomAlert
  expr: custom_metric > 
  for: m
  labels:
    severity: warning
  annotations:
    summary: "Custom condition triggered"
    description: "Custom metric is {{ $value }}"


 Advanced Dashboards

. Create dashboard in Grafana UI
. Add panels with PromQL queries:
   
    Cache hit rate percentage
   (redis_keyspace_hits_total / (redis_keyspace_hits_total + redis_keyspace_misses_total))  
   
    Query latency by percentile
   histogram_quantile(., request_duration_seconds_bucket)
   
    Memory growth rate
   rate(redis_memory_used_bytes[m])
   
. Export and save to deployment/monitoring/grafana/dashboards/

 References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [AlertManager Documentation](https://prometheus.io/docs/alerting/latest/overview/)
- [Redis Exporter](https://github.com/oliver/redis_exporter)
- [PostgreSQL Exporter](https://github.com/prometheus-community/postgres_exporter)
