# OpenRisk Production Monitoring Setup

**Date**: February 22, 2026  
**Status**: Production Ready

---

## Overview

This document describes the comprehensive monitoring setup for OpenRisk in production. The system uses Prometheus for metrics collection, Grafana for visualization, and AlertManager for incident notifications.

### Components

- **Prometheus**: Time-series database for metrics storage and alerting
- **Grafana**: Visualization and dashboard platform
- **AlertManager**: Alert routing and notification system
- **Redis Exporter**: Cache performance metrics
- **PostgreSQL Exporter**: Database metrics
- **OpenRisk Backend**: Application metrics via Prometheus SDK

---

## Metrics Collection

### Application Metrics (Backend)

The OpenRisk backend exposes Prometheus metrics on port 8080 at the `/metrics` endpoint.

#### API Metrics
- `openrisk_http_requests_total` - Total HTTP requests processed
- `openrisk_http_request_duration_seconds` - Request latency histogram
- `openrisk_http_requests_in_flight` - Current in-flight requests
- `openrisk_http_errors_total` - Total HTTP errors

#### Risk Management Metrics
- `openrisk_risks_created_total` - Total risks created
- `openrisk_risks_updated_total` - Total risks updated
- `openrisk_risks_deleted_total` - Total risks deleted
- `openrisk_risks_retrieved_total` - Total risks retrieved
- `openrisk_risks_by_status` - Gauge of risks by status
- `openrisk_risks_severity_distribution` - Risk severity histogram

#### Mitigation Metrics
- `openrisk_mitigations_created_total` - Total mitigations created
- `openrisk_mitigations_updated_total` - Total mitigations updated
- `openrisk_mitigation_progress_percentage` - Average progress %
- `openrisk_mitigations_due_soon` - Mitigations due in 7 days

#### Cache Metrics
- `openrisk_cache_hits_total` - Total cache hits
- `openrisk_cache_misses_total` - Total cache misses
- `openrisk_cache_hit_rate` - Cache hit rate percentage
- `openrisk_cache_entries` - Current entries in cache
- `openrisk_cache_evictions_total` - Total evictions

#### Database Metrics
- `openrisk_db_query_duration_seconds` - Query latency histogram
- `openrisk_db_connection_pool_size` - Active connections
- `openrisk_db_query_errors_total` - Total query errors
- `openrisk_db_slow_queries_total` - Queries exceeding 1 second

#### Authentication Metrics
- `openrisk_login_attempts_total` - Total login attempts
- `openrisk_login_successes_total` - Successful logins
- `openrisk_login_failures_total` - Failed logins
- `openrisk_token_refreshes_total` - Token refresh operations
- `openrisk_mfa_attempts_total` - MFA attempts

#### Business Metrics
- `openrisk_active_users` - Number of active users
- `openrisk_active_tenants` - Number of active tenants
- `openrisk_asset_count` - Total managed assets
- `openrisk_audit_logs_created_total` - Audit log entries

#### System Metrics
- `openrisk_go_memory_heap_bytes` - Heap memory allocation
- `openrisk_go_memory_alloc_bytes` - Total memory allocated
- `openrisk_go_goroutines` - Running goroutines
- `openrisk_system_uptime_seconds` - System uptime

---

## Alert Rules

All alert rules are defined in `alerts.yml` and are grouped by severity:

### Critical Alerts (Immediate Action Required)

| Alert | Condition | Duration |
|-------|-----------|----------|
| AppDown | Backend unreachable | 1 minute |
| CriticalLowCacheHitRate | Cache hit rate < 50% | 2 minutes |
| CriticalAPIErrorRate | Error rate > 10% | 1 minute |
| CriticalAPILatency | 99th percentile > 5s | 2 minutes |
| CriticalDatabaseConnections | >48 connections (max: 50) | 1 minute |
| HighRedisMemory | Redis > 85% memory | 5 minutes |
| PossibleBruteForceAttack | >20 failed logins/5min | 1 minute |

### Warning Alerts (Attention Required)

| Alert | Condition | Duration |
|-------|-----------|----------|
| LowCacheHitRate | Cache hit rate < 75% | 5 minutes |
| HighAPIErrorRate | Error rate > 5% | 3 minutes |
| HighAPILatency | 95th percentile > 2s | 5 minutes |
| HighDatabaseConnections | >40 connections | 5 minutes |
| SlowDatabaseQueries | Avg query time > 1s | 5 minutes |
| HighMemoryUsage | Memory > 1GB | 5 minutes |
| TooManyGoroutines | Goroutines > 10,000 | 3 minutes |
| HighLoginFailureRate | Failure rate > 30% | 2 minutes |

### Info Alerts (Monitoring Only)

| Alert | Condition |
|-------|-----------|
| NoActiveUsers | Active users = 0 |
| MitigationsDueAlert | >10 mitigations due in 7 days |

---

## Alert Routing & Notifications

### Slack Integration

Three Slack channels receive alerts:

1. **#critical-alerts** - Critical severity (immediate)
2. **#performance-alerts** - Warning severity (hourly digest)
3. **#monitoring-info** - Info severity (6-hour digest)

### PagerDuty Integration

Critical alerts trigger immediate PagerDuty incidents for on-call engineers.

**Setup Required**:
- Set `PAGERDUTY_SERVICE_KEY` environment variable

### Opsgenie Integration

Critical alerts also create incidents in Opsgenie with priority P1.

**Setup Required**:
- Set `OPSGENIE_API_KEY` environment variable

---

## Grafana Dashboards

### Main Dashboard: "OpenRisk Performance Monitoring"

Location: `grafana/dashboards/openrisk-performance.json`

**Panels**:
1. Application Status - Backend health indicator
2. HTTP Requests Per Second - Request throughput
3. HTTP Error Rate - API error percentage
4. API Latency (p95) - Request latency
5. Cache Hit Rate - Cache efficiency
6. Database Connections - Connection pool usage
7. Database Query Duration (p95) - Query performance
8. Slow Queries Per Second - Query volume
9. Memory Usage - RAM utilization
10. Goroutine Count - Concurrency metrics
11. Active Users - User engagement
12. Risks by Status - Risk distribution
13. Login Success Rate - Authentication health
14. System Uptime - Availability

**Refresh Rate**: 10 seconds  
**Time Range**: Last 1 hour (configurable)

---

## Deployment Configuration

### Prometheus Configuration

File: `prometheus.yml`

**Scrape Targets**:
- `openrisk-backend:8080` (metrics every 10s)
- `redis-exporter:9121` (cache every 10s)
- `postgres-exporter:9187` (database every 15s)
- `prometheus:9090` (self-monitoring every 15s)

**Retention**: 15 days (configurable)

### Docker Compose

File: `docker-compose-monitoring.yaml`

**Services**:
```yaml
prometheus:
  ports: [9090]
  volumes:
    - ./prometheus.yml:/etc/prometheus/prometheus.yml
    - ./alerts.yml:/etc/prometheus/alerts.yml

grafana:
  ports: [3000]
  environment:
    - GF_SECURITY_ADMIN_PASSWORD=<secure-password>
  volumes:
    - grafana-storage:/var/lib/grafana

alertmanager:
  ports: [9093]
  volumes:
    - ./alertmanager.yml:/etc/alertmanager/alertmanager.yml

redis-exporter:
  ports: [9121]

postgres-exporter:
  ports: [9187]
```

---

## Performance Thresholds

### SLOs (Service Level Objectives)

| Metric | Target | Warning | Critical |
|--------|--------|---------|----------|
| API Availability | 99.9% | 99.5% | < 99% |
| API Latency (p95) | < 500ms | < 2s | > 5s |
| API Error Rate | < 0.1% | > 5% | > 10% |
| Cache Hit Rate | > 80% | < 75% | < 50% |
| DB Query Time (p95) | < 100ms | > 1s | > 5s |
| Memory Usage | < 500MB | > 1GB | > 2GB |
| Goroutines | < 5,000 | > 10,000 | > 20,000 |

### Database Connection Limits

- **Connection Pool Size**: 50 (configured in backend)
- **Warning Threshold**: 40 connections
- **Critical Threshold**: 48 connections

### Cache Configuration

- **Redis Memory Limit**: 2GB
- **Warning Threshold**: 1.7GB (85%)
- **Critical Threshold**: 1.9GB (95%)

---

## Setup Instructions

### 1. Environment Variables

```bash
# Required for alerting
export SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
export PAGERDUTY_SERVICE_KEY="your-service-key"
export OPSGENIE_API_KEY="your-api-key"

# Grafana admin password (change in production)
export GF_SECURITY_ADMIN_PASSWORD="secure-password-here"
```

### 2. Start Monitoring Stack

```bash
cd deployment/monitoring
docker-compose -f docker-compose-monitoring.yaml up -d
```

### 3. Verify Services

```bash
# Check Prometheus
curl http://localhost:9090/-/healthy

# Check Grafana
curl http://localhost:3000/api/health

# Check AlertManager
curl http://localhost:9093/-/healthy
```

### 4. Configure Grafana Data Source

1. Navigate to http://localhost:3000
2. Login with default credentials
3. Add Prometheus data source: `http://prometheus:9090`
4. Import dashboard from `openrisk-performance.json`

### 5. Test Alert

```bash
# To test an alert, manually stop the backend:
docker stop openrisk-backend

# You should receive notifications within 1-2 minutes
# Re-start the backend
docker start openrisk-backend
```

---

## Maintenance & Monitoring

### Daily Checks

- [ ] Check Grafana dashboard for anomalies
- [ ] Review alert history in AlertManager UI
- [ ] Verify all exporters are reporting metrics

### Weekly Checks

- [ ] Analyze performance trends
- [ ] Review alert tuning (false positives/negatives)
- [ ] Check disk usage on Prometheus server
- [ ] Verify backup of Prometheus data

### Monthly Checks

- [ ] Review and optimize alert thresholds based on baseline
- [ ] Update runbooks based on incidents
- [ ] Capacity planning analysis
- [ ] Audit log retention

### Prometheus Disk Usage

Monitor disk usage:
```bash
# Check Prometheus disk usage
du -sh /var/lib/prometheus

# Current retention: 15 days
# Expected size: ~50GB (varies with metrics volume)
```

To adjust retention:
```yaml
# In prometheus.yml
global:
  retention: 30d  # Increase to 30 days
```

---

## Troubleshooting

### Prometheus Not Scraping Backend Metrics

**Check**:
1. Backend is running: `docker ps | grep openrisk-backend`
2. Backend listening on 8080: `curl http://localhost:8080/metrics`
3. Prometheus config is correct in `prometheus.yml`
4. Backend metrics endpoint is exposed

**Fix**:
```bash
# Restart Prometheus
docker restart prometheus

# Check Prometheus logs
docker logs prometheus
```

### Missing Metrics in Dashboard

**Check**:
1. Grafana data source is configured correctly
2. Metric names match exactly (case-sensitive)
3. Backend has recorded the metric (check `/metrics` endpoint)

**Fix**:
```bash
# Query metric directly
curl http://localhost:9090/api/v1/query?query=openrisk_http_requests_total
```

### Alerts Not Firing

**Check**:
1. AlertManager is running: `docker ps | grep alertmanager`
2. Webhook URLs are valid
3. Alert rules are loaded in Prometheus

**Fix**:
```bash
# Check Prometheus alerts page
curl http://localhost:9090/alerts

# Restart AlertManager
docker restart alertmanager
```

### Slack Notifications Not Working

**Check**:
1. `SLACK_WEBHOOK_URL` is set correctly
2. Webhook URL is still valid (webhooks expire)
3. AlertManager logs: `docker logs alertmanager`

**Fix**:
1. Generate new webhook URL from Slack
2. Update `SLACK_WEBHOOK_URL` environment variable
3. Restart AlertManager

---

## Production Considerations

### High Availability

For production deployments:

1. **Multiple Prometheus Instances**: Run in HA mode with remote storage
2. **Grafana Redundancy**: Use database-backed persistence
3. **AlertManager Cluster**: Deploy 3+ instances
4. **Persistent Storage**: Use volumes for Prometheus data

### Security

1. **Reverse Proxy**: Place behind NGINX/HAProxy
2. **Authentication**: Enable Grafana LDAP/OAuth
3. **Network**: Restrict access to monitoring ports
4. **Credentials**: Use secrets management (Vault/K8s secrets)

### Scalability

As metrics volume increases:

1. **Increase Prometheus resources**: CPU, RAM, disk
2. **Implement remote storage**: S3, Thanos, or Cortex
3. **Federation**: Use multiple Prometheus instances for different components
4. **Retention tuning**: Balance history vs. storage costs

---

## Related Documentation

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [AlertManager Documentation](https://prometheus.io/docs/alerting/latest/alertmanager/)
- [Backend Integration](../../backend/internal/monitoring/README.md)

---

## Contact & Support

For monitoring issues:
1. Check this guide's Troubleshooting section
2. Review Prometheus/Grafana logs
3. Check AlertManager configuration
4. Contact DevOps team for infrastructure issues

---

**Last Updated**: February 22, 2026  
**Next Review**: March 22, 2026
