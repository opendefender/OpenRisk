# Production Deployment Finalization Guide

**Date**: March 2, 2026  
**Phase**: 6B - Production Ready  
**Status**: Complete  
**Version**: 1.0

---

## Executive Summary

This document provides comprehensive guidance for finalizing production deployments of OpenRisk, including staging-to-production transitions, blue-green deployment strategies, and production monitoring setup.

---

## Part 1: Pre-Production Validation

### Checklist

```
☑ Database migrations verified in staging
☑ All 140+ tests passing
☑ Performance benchmarks validated
☑ Security audit completed
☑ Load testing results reviewed
☑ SSL/TLS certificates prepared
☑ Environment variables configured
☑ Backup strategy tested
☑ Monitoring & alerting configured
☑ Incident response plan documented
```

### Database Validation

```bash
# 1. Verify schema integrity
psql -h staging-db.example.com -U openrisk -d openrisk_staging \
  -c "SELECT COUNT(*) as table_count FROM information_schema.tables 
      WHERE table_schema='public';"

# Expected: 25+ tables

# 2. Check constraints and indexes
psql -h staging-db.example.com -U openrisk -d openrisk_staging \
  -c "SELECT COUNT(*) as index_count FROM pg_indexes 
      WHERE schemaname='public';"

# Expected: 70+ indexes

# 3. Validate data integrity
psql -h staging-db.example.com -U openrisk -d openrisk_staging \
  -c "SELECT table_name, row_count FROM (
    SELECT tablename as table_name, n_live_tup as row_count 
    FROM pg_stat_user_tables
  ) t WHERE row_count > 0 ORDER BY row_count DESC;"
```

### Performance Validation

```bash
# 1. Load testing (k6)
k6 run tests/performance/load_test.js \
  --vus 100 \
  --duration 5m \
  --ramp-up 1m

# Expected results:
# - P95 latency < 500ms
# - P99 latency < 1s
# - Error rate < 0.1%

# 2. Database query performance
psql -h staging-db.example.com -U openrisk -d openrisk_staging \
  -c "SELECT query, mean_exec_time, calls FROM pg_stat_statements 
      ORDER BY mean_exec_time DESC LIMIT 20;"

# 3. Cache hit rates
redis-cli -h staging-redis.example.com INFO stats
# Look for: keyspace_hits / (keyspace_hits + keyspace_misses)
# Expected: > 70%
```

---

## Part 2: Staging to Production Deployment

### Pre-Deployment Steps

#### 1. Production Database Backup

```bash
# Create backup
BACKUP_DIR="/backups/prod/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$BACKUP_DIR"

# PostgreSQL backup
pg_dump -h prod-db.example.com \
  -U openrisk \
  -d openrisk \
  -F c \
  -v > "$BACKUP_DIR/openrisk.dump"

# Verify backup
pg_restore -h localhost \
  -U openrisk \
  -d openrisk_backup_test \
  -l "$BACKUP_DIR/openrisk.dump"

echo "Backup completed: $BACKUP_DIR"
```

#### 2. Health Check

```bash
# Check backend health
curl -X GET https://api.example.com/health \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Check database connectivity
psql -h prod-db.example.com -U openrisk -d openrisk \
  -c "SELECT version();"

# Check Redis connectivity
redis-cli -h prod-redis.example.com ping
```

#### 3. Configuration Validation

```bash
# Verify all required environment variables
required_vars=(
  "DATABASE_URL"
  "REDIS_HOST"
  "JWT_SECRET"
  "TLS_CERT_PATH"
  "TLS_KEY_PATH"
)

for var in "${required_vars[@]}"; do
  if [ -z "${!var}" ]; then
    echo "ERROR: $var not set"
    exit 1
  fi
done

echo "All required variables configured"
```

---

### Blue-Green Deployment

#### Strategy

This approach runs two identical production environments (Blue and Green), allowing zero-downtime deployments.

```
┌─────────────────────────────────────────────────┐
│  Load Balancer (SSL/TLS Termination)           │
└────────────┬────────────────────────────────────┘
             │
      ┌──────┴──────┐
      │             │
   ┌──▼──┐      ┌──▼──┐
   │Blue │      │Green│
   │  v1 │      │  v2 │
   └─────┘      └─────┘
   (Active)    (Standby)
```

#### Implementation

```bash
#!/bin/bash
# deploy_blue_green.sh

NAMESPACE="openrisk"
BLUE_VERSION="v1.0.0"
GREEN_VERSION="v1.0.1"

# 1. Deploy new version to Green environment
echo "Deploying $GREEN_VERSION to Green environment..."
helm upgrade openrisk-green ./helm/openrisk \
  -f helm/values-prod.yaml \
  -f helm/values-green.yaml \
  --set image.tag=$GREEN_VERSION \
  --namespace $NAMESPACE \
  --wait

# 2. Wait for Green to be healthy
echo "Waiting for Green environment to be healthy..."
kubectl rollout status deployment/openrisk-green \
  -n $NAMESPACE \
  --timeout=5m

# 3. Run smoke tests on Green
echo "Running smoke tests..."
./tests/smoke_tests.sh "https://green.internal.example.com"

if [ $? -ne 0 ]; then
  echo "Smoke tests failed! Aborting deployment."
  kubectl rollout undo deployment/openrisk-green -n $NAMESPACE
  exit 1
fi

# 4. Switch traffic from Blue to Green
echo "Switching traffic to Green environment..."
kubectl patch service openrisk-api \
  -n $NAMESPACE \
  -p '{"spec":{"selector":{"deployment":"openrisk-green"}}}'

# 5. Monitor Green for 5 minutes
echo "Monitoring Green environment..."
./scripts/monitor_deployment.sh "openrisk-green" "5m"

# 6. Mark Green as Blue (old Blue can be kept as fallback)
echo "Marking Green as new primary..."
kubectl patch deployment openrisk-blue \
  -n $NAMESPACE \
  -p '{"spec":{"replicas":0}}'

echo "Deployment complete!"
```

#### Rollback Procedure

```bash
#!/bin/bash
# rollback.sh

NAMESPACE="openrisk"

echo "Rolling back to Blue environment..."

# 1. Switch traffic back to Blue
kubectl patch service openrisk-api \
  -n $NAMESPACE \
  -p '{"spec":{"selector":{"deployment":"openrisk-blue"}}}'

# 2. Scale up Blue
kubectl patch deployment openrisk-blue \
  -n $NAMESPACE \
  -p '{"spec":{"replicas":3}}'

# 3. Wait for rollback to complete
kubectl rollout status deployment/openrisk-blue \
  -n $NAMESPACE \
  --timeout=5m

echo "Rollback complete! Blue environment is now active."
```

---

### Canary Deployment

#### Strategy

Gradually shift traffic to new version while monitoring metrics.

```bash
#!/bin/bash
# deploy_canary.sh

NAMESPACE="openrisk"
NEW_VERSION="v1.0.1"

# Initial: 5% traffic to new version
kubectl patch virtualservice openrisk \
  -n $NAMESPACE \
  --type merge -p '{"spec":{"hosts":[{"name":"api.example.com","http":[{"match":[{"uri":{"prefix":"/"}}],"route":[{"destination":{"host":"openrisk","port":{"number":8080},"subset":"v1"},"weight":95},{"destination":{"host":"openrisk","port":{"number":8080},"subset":"v1-new"},"weight":5}]}]}]}}'

# Monitor error rate for 5 minutes
sleep 300

ERROR_RATE=$(prometheus_query 'rate(http_requests_total{status=~"5.."}[5m])')

if [ $(echo "$ERROR_RATE > 0.01" | bc) -eq 1 ]; then
  echo "High error rate detected! Rolling back canary..."
  kubectl patch virtualservice openrisk \
    -n $NAMESPACE \
    --type merge -p '{"spec":{"hosts":[{"name":"api.example.com","http":[{"match":[{"uri":{"prefix":"/"}}],"route":[{"destination":{"host":"openrisk","port":{"number":8080},"subset":"v1"},"weight":100}]}]}]}}'
  exit 1
fi

# Gradually increase traffic
for percentage in 25 50 75 100; do
  echo "Shifting to $percentage% traffic to new version..."
  kubectl patch virtualservice openrisk \
    -n $NAMESPACE \
    --type merge -p "{\"spec\":{\"hosts\":[{\"name\":\"api.example.com\",\"http\":[{\"match\":[{\"uri\":{\"prefix\":\"/\"}}],\"route\":[{\"destination\":{\"host\":\"openrisk\",\"port\":{\"number\":8080},\"subset\":\"v1\"},\"weight\":$((100-percentage))},{\"destination\":{\"host\":\"openrisk\",\"port\":{\"number\":8080},\"subset\":\"v1-new\"},\"weight\":$percentage}]}]}]}}"
  
  sleep 300
done

echo "Canary deployment complete!"
```

---

## Part 3: Production Environment Setup

### Kubernetes Manifest for Production

```yaml
# k8s/openrisk-prod.yaml

apiVersion: v1
kind: ConfigMap
metadata:
  name: openrisk-config
  namespace: openrisk
data:
  LOG_LEVEL: "info"
  METRICS_ENABLED: "true"
  CACHE_ENABLED: "true"
  TLS_ENABLED: "true"

---

apiVersion: v1
kind: Secret
metadata:
  name: openrisk-secrets
  namespace: openrisk
type: Opaque
stringData:
  DATABASE_URL: "postgres://user:password@prod-db:5432/openrisk"
  REDIS_PASSWORD: "your-redis-password"
  JWT_SECRET: "your-jwt-secret"

---

apiVersion: apps/v1
kind: Deployment
metadata:
  name: openrisk-api
  namespace: openrisk
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: openrisk-api
  template:
    metadata:
      labels:
        app: openrisk-api
        version: v1
    spec:
      containers:
      - name: api
        image: openrisk/api:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090  # Metrics
        envFrom:
        - configMapRef:
            name: openrisk-config
        - secretRef:
            name: openrisk-secrets
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "1Gi"
            cpu: "1000m"
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

---

apiVersion: v1
kind: Service
metadata:
  name: openrisk-api
  namespace: openrisk
spec:
  type: LoadBalancer
  ports:
  - name: http
    port: 80
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
  selector:
    app: openrisk-api

---

apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: openrisk-api-hpa
  namespace: openrisk
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: openrisk-api
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

---

## Part 4: Monitoring & Observability

### Prometheus Configuration

```yaml
# prometheus/prod-config.yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'openrisk'
    static_configs:
      - targets: ['localhost:9090']
    metrics_path: '/metrics'

  - job_name: 'postgres'
    static_configs:
      - targets: ['prod-db:9187']

  - job_name: 'redis'
    static_configs:
      - targets: ['prod-redis:9121']

alerting:
  alertmanagers:
    - static_configs:
        - targets: ['localhost:9093']
```

### Grafana Dashboards

Key dashboards to create:

1. **System Overview**
   - Request rate
   - Error rate
   - Latency (P50, P95, P99)
   - Active connections

2. **Database Performance**
   - Query latency
   - Connections (active/idle)
   - Cache hit rate
   - Replication lag

3. **Business Metrics**
   - Active users
   - Risk creation rate
   - API usage by endpoint
   - Customer tier distribution

### Alert Rules

```yaml
# prometheus/alert-rules.yaml

groups:
  - name: openrisk.rules
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        annotations:
          summary: "High error rate detected"

      - alert: HighLatency
        expr: histogram_quantile(0.99, http_request_duration_seconds) > 1
        for: 5m
        annotations:
          summary: "High latency detected"

      - alert: DatabaseConnectionPool
        expr: pg_stat_activity_count > 18
        for: 5m
        annotations:
          summary: "Database connection pool near limit"

      - alert: CacheHitRate
        expr: rate(cache_hits[5m]) / (rate(cache_hits[5m]) + rate(cache_misses[5m])) < 0.6
        for: 10m
        annotations:
          summary: "Cache hit rate below threshold"

      - alert: PodCrashLooping
        expr: increase(kube_pod_container_status_restarts_total[1h]) > 5
        annotations:
          summary: "Pod is crash looping"
```

---

## Part 5: Documentation & Runbooks

### Runbook Template

```markdown
# [Incident Name] Runbook

## Overview
Brief description of the issue this runbook addresses.

## Symptoms
- [Symptom 1]
- [Symptom 2]

## Root Causes
- [Cause 1]
- [Cause 2]

## Diagnosis Steps
1. Check Prometheus dashboard
2. Query database performance
3. Review application logs

## Resolution
1. Step 1
2. Step 2
3. Verify resolution

## Prevention
- Monitoring alert
- Code review process
- Testing coverage
```

### Example Runbooks

1. **High Error Rate**
2. **Database Connection Exhaustion**
3. **Cache Failure**
4. **Memory Leak**
5. **Disk Space Full**

---

## Part 6: Compliance & Security

### Security Checklist

```
☑ SSL/TLS certificates valid
☑ API authentication enforced
☑ CORS headers configured
☑ Rate limiting enabled
☑ Input validation implemented
☑ SQL injection prevention (parameterized queries)
☑ XSS protection (content security policy)
☑ CSRF tokens implemented
☑ Security headers configured
☑ Regular security scanning enabled
```

### Security Headers Configuration

```go
// backend/middleware/security_headers.go
package middleware

import "github.com/gofiber/fiber/v2"

func SecurityHeaders(c *fiber.Ctx) error {
    c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
    c.Set("X-Content-Type-Options", "nosniff")
    c.Set("X-Frame-Options", "DENY")
    c.Set("X-XSS-Protection", "1; mode=block")
    c.Set("Content-Security-Policy", "default-src 'self'")
    c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
    
    return c.Next()
}
```

---

## Part 7: Performance Tuning

### Database Optimization

```sql
-- Analyze query performance
EXPLAIN ANALYZE 
SELECT r.id, r.name, COUNT(m.id) as mitigation_count
FROM risks r
LEFT JOIN mitigations m ON r.id = m.risk_id
GROUP BY r.id, r.name;

-- Create composite indexes
CREATE INDEX idx_risks_tenant_status 
ON risks(tenant_id, status);

CREATE INDEX idx_mitigations_risk_due 
ON mitigations(risk_id, due_date);

-- Update statistics
ANALYZE;
```

### Cache Strategy

```go
// backend/services/cache_strategy.go
package services

type CacheStrategy struct {
    // Short-lived: 5 minutes
    UserProfiles: 5,
    
    // Medium-lived: 30 minutes
    RiskLists: 30,
    
    // Long-lived: 24 hours
    Configurations: 1440,
}

func (cs *CacheStrategy) GetTTL(key string) int {
    // Implement per-key TTL logic
}
```

---

## Conclusion

This comprehensive guide provides all necessary steps for production deployment, from pre-deployment validation through ongoing monitoring and incident response. Follow these procedures to ensure reliable, scalable, and secure production operations.

**Support**: For questions or issues, contact the DevOps team.
