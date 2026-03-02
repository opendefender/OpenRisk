# Staging Deployment Guide - Phase 6

**Last Updated**: March 2, 2026
**Target**: Validate WebSocket, Analytics, Incidents, Export, and Custom Metrics
**Environment**: Docker Compose with isolated staging stack

## Prerequisites

- Docker & Docker Compose v2.0+
- 4GB RAM available
- Git with feat/staging-deployment-config branch
- Environment variables configured

## Quick Start (5 minutes)

### 1. Configure Environment

```bash
cp .env.staging.example .env.staging
# Edit .env.staging with your credentials
```

### 2. Deploy Staging Stack

```bash
# Bring up all services (PostgreSQL, Redis, Backend, Frontend, Prometheus)
docker-compose -f docker-compose.staging.yaml up -d

# Check service status
docker-compose -f docker-compose.staging.yaml ps

# View logs
docker-compose -f docker-compose.staging.yaml logs -f backend-staging
```

### 3. Verify Services

```bash
# Backend health
curl http://localhost:8080/health

# Frontend (open browser)
http://localhost:3000

# Prometheus metrics
http://localhost:9090
```

## Phase 6 Feature Validation

### WebSocket Real-Time Updates

**Test**: Open analytics dashboard and verify live metric updates

```bash
# Terminal 1: Monitor WebSocket connections
docker-compose -f docker-compose.staging.yaml logs -f backend-staging | grep websocket

# Terminal 2: Load frontend
# Navigate to AnalyticsDashboard
# Verify metrics update in real-time without refresh
```

**Expected Behavior**:
- ✅ Metrics update every 10 seconds
- ✅ No page refresh required
- ✅ Connection persists on page navigation
- ✅ Auto-reconnect on connection loss

### Analytics Dashboard

**Test**: Create metrics and verify data export

```bash
# Create custom metric via API
curl -X POST http://localhost:8080/api/metrics/custom \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Critical Risks",
    "metric_type": "count",
    "formula": "count(*)",
    "data_source": "risks",
    "aggregation": "daily"
  }'

# Export metrics as CSV
curl http://localhost:8080/api/export/metrics/staging?format=csv \
  -o metrics.csv

# Export as JSON
curl http://localhost:8080/api/export/metrics/staging?format=json \
  -o metrics.json
```

### Incident Management

**Test**: Create incident and link to risk

```bash
# Create incident
curl -X POST http://localhost:8080/api/incidents \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Critical Vulnerability Detected",
    "description": "SQL injection in login form",
    "incident_type": "vulnerability",
    "severity": "critical",
    "source": "internal",
    "reported_by": "security-team"
  }'

# Link incident to risk
curl -X POST http://localhost:8080/api/incidents/1/link-risk/1 \
  -H "Authorization: Bearer YOUR_TOKEN"

# Get incident timeline
curl http://localhost:8080/api/incidents/1/timeline \
  -H "Authorization: Bearer YOUR_TOKEN"

# Get incident stats
curl http://localhost:8080/api/incidents/stats \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Compliance Reporting

**Test**: Export compliance data

```bash
# Get compliance report
curl http://localhost:8080/api/compliance/report/staging \
  -H "Authorization: Bearer YOUR_TOKEN"

# Export compliance as CSV
curl http://localhost:8080/api/export/compliance/staging?format=csv \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o compliance.csv

# Export as JSON
curl http://localhost:8080/api/export/compliance/staging?format=json \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -o compliance.json
```

## Performance Validation

### Load Testing

```bash
# Install k6
brew install k6  # macOS

# Run baseline test
k6 run load_tests/phase6_baseline.js

# Run stress test (WebSocket)
k6 run load_tests/phase6_websocket_stress.js
```

**Expected Results**:
- ✅ Dashboard load < 3 seconds
- ✅ Incident creation < 500ms
- ✅ WebSocket latency < 100ms
- ✅ Export endpoints < 2 seconds

### Database Performance

```bash
# Connect to staging DB
psql -h localhost -p 5433 -U openrisk_user -d openrisk_staging

# Check slow queries
SELECT query, calls, mean_time FROM pg_stat_statements
  ORDER BY mean_time DESC LIMIT 10;

# Monitor index usage
SELECT schemaname, tablename, indexname, idx_scan, idx_tup_read
  FROM pg_stat_user_indexes
  ORDER BY idx_scan DESC;
```

## Data Validation

### Database Migrations

```bash
# Check applied migrations
docker-compose -f docker-compose.staging.yaml exec backend-staging \
  go run ./cmd/migrate up

# Verify schema
docker-compose -f docker-compose.staging.yaml exec postgres-staging \
  psql -U openrisk_user -d openrisk_staging -c "\dt"
```

### Redis Cache

```bash
# Monitor cache operations
docker-compose -f docker-compose.staging.yaml exec redis-staging \
  redis-cli -a ${STAGING_REDIS_PASSWORD} INFO stats

# Flush cache (for testing)
docker-compose -f docker-compose.staging.yaml exec redis-staging \
  redis-cli -a ${STAGING_REDIS_PASSWORD} FLUSHALL
```

## Monitoring & Logging

### View Logs

```bash
# Backend logs
docker-compose -f docker-compose.staging.yaml logs -f backend-staging

# Frontend logs
docker-compose -f docker-compose.staging.yaml logs -f frontend-staging

# All services
docker-compose -f docker-compose.staging.yaml logs -f
```

### Prometheus Metrics

Access http://localhost:9090 to:
- ✅ Query request latency
- ✅ Monitor error rates
- ✅ Track database connections
- ✅ Verify cache hit rates

### Sample Queries

```promql
# Request rate (per second)
rate(http_requests_total[1m])

# Error rate
rate(http_requests_failed_total[1m])

# Database connection pool
pg_stat_activity_count

# Cache hit ratio
redis_keyspace_hits_total / (redis_keyspace_hits_total + redis_keyspace_misses_total)
```

## Testing Checklist

- [ ] **WebSocket Connectivity**
  - [ ] Real-time dashboard updates
  - [ ] Connection persists on navigation
  - [ ] Auto-reconnect on disconnect
  - [ ] Metrics update < 100ms latency

- [ ] **Analytics Features**
  - [ ] Custom metric creation
  - [ ] Metric calculation accuracy
  - [ ] Trend detection (up/down/stable)
  - [ ] Export CSV/JSON formats

- [ ] **Incident Management**
  - [ ] Create incident
  - [ ] Update status workflow
  - [ ] Link to risk
  - [ ] Add timeline entries
  - [ ] Create mitigation actions

- [ ] **Export Functionality**
  - [ ] Export metrics CSV
  - [ ] Export metrics JSON
  - [ ] Export compliance data
  - [ ] Export dashboard report
  - [ ] Export audit logs

- [ ] **Performance**
  - [ ] Dashboard load < 3s
  - [ ] Incident operations < 500ms
  - [ ] WebSocket latency < 100ms
  - [ ] Export endpoints < 2s

- [ ] **Compliance**
  - [ ] Audit trail complete
  - [ ] Multi-tenant isolation verified
  - [ ] Permission checks working
  - [ ] Data encryption enabled

## Troubleshooting

### Services Won't Start

```bash
# Check docker resources
docker system df

# Verify network
docker network ls

# Restart services
docker-compose -f docker-compose.staging.yaml down
docker-compose -f docker-compose.staging.yaml up -d
```

### Database Connection Issues

```bash
# Test connection
docker-compose -f docker-compose.staging.yaml exec backend-staging \
  curl -v postgresql://openrisk_user:password@postgres-staging:5432/openrisk_staging

# Check DNS
docker-compose -f docker-compose.staging.yaml exec backend-staging \
  nslookup postgres-staging
```

### WebSocket Not Connecting

```bash
# Check WebSocket listener
docker-compose -f docker-compose.staging.yaml exec backend-staging \
  netstat -tlnp | grep 8080

# Test WebSocket connection
docker-compose -f docker-compose.staging.yaml exec frontend-staging \
  curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  http://backend-staging:8080/ws
```

## Cleanup

```bash
# Stop services
docker-compose -f docker-compose.staging.yaml down

# Remove volumes (CAUTION: loses data)
docker-compose -f docker-compose.staging.yaml down -v

# Clean all images
docker-compose -f docker-compose.staging.yaml down --rmi all
```

## Next Steps

1. **Validate All Features** - Complete testing checklist above
2. **Performance Benchmark** - Run load tests and document results
3. **Security Audit** - Verify authentication, authorization, encryption
4. **Document Issues** - Create GitHub issues for any blockers
5. **Merge to Main** - Once validation complete, create PR
6. **Production Deployment** - Schedule for production rollout

---

**Status**: Ready for Phase 6 staging validation
**Features Deployed**: WebSocket, Analytics, Incidents, Export, Custom Metrics
**Target Date**: March 2, 2026
