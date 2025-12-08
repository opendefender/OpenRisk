# Production Deployment Runbook

## Quick Reference

| Component | Service | Port | Health Check |
|-----------|---------|------|--------------|
| Frontend | Nginx/React | 80/443 | https://openrisk.yourdomain.com |
| Backend API | Go Fiber | 8080 | https://openrisk.yourdomain.com/api/v1/health |
| Database | PostgreSQL | 5432 | Internal only |
| Cache | Redis | 6379 | Internal only |

## Pre-Deployment Checklist

### Code & Testing
- [ ] All tests passing (unit + integration)
- [ ] Code review completed
- [ ] Security scan passed
- [ ] Dependencies updated and audited
- [ ] Changelog updated
- [ ] Version bumped

### Infrastructure
- [ ] Production environment provisioned
- [ ] Database backups configured
- [ ] SSL/TLS certificates provisioned
- [ ] DNS entries created
- [ ] Firewall rules configured
- [ ] Monitoring/alerting configured
- [ ] Load balancer configured (if needed)
- [ ] CDN configured (if needed)

### Documentation
- [ ] Deployment runbook reviewed
- [ ] Incident response procedures documented
- [ ] Rollback procedures documented
- [ ] Team trained on procedures
- [ ] On-call rotation established

## Deployment Steps

### 1. Pre-Deployment Verification

```bash
#!/bin/bash
# deployment-verify.sh

set -e

echo "🔍 Pre-deployment verification..."

# Verify Docker images exist
docker images | grep openrisk || { echo "❌ Docker images not found"; exit 1; }

# Verify environment config
[ -f .env.production ] || { echo "❌ .env.production not found"; exit 1; }

# Verify database backups
[ -d ./backups ] || { echo "❌ Backup directory not found"; exit 1; }

# Verify certificates
[ -f ./certs/fullchain.pem ] || { echo "❌ SSL certificate not found"; exit 1; }

echo "✅ All pre-deployment checks passed"
```

### 2. Blue-Green Deployment

```bash
#!/bin/bash
# blue-green-deploy.sh
# Allows zero-downtime deployments

PRODUCTION_DIR="/opt/openrisk-prod"
BLUE_DIR="$PRODUCTION_DIR/blue"
GREEN_DIR="$PRODUCTION_DIR/green"
CURRENT_LINK="$PRODUCTION_DIR/current"

echo "🚀 Starting blue-green deployment..."

# Determine which is active (blue or green)
if [ -L "$CURRENT_LINK" ]; then
    ACTIVE=$(readlink "$CURRENT_LINK")
    [ "$ACTIVE" = "$BLUE_DIR" ] && NEW_DIR="$GREEN_DIR" || NEW_DIR="$BLUE_DIR"
else
    NEW_DIR="$BLUE_DIR"
fi

echo "Active: $ACTIVE"
echo "Deploying to: $NEW_DIR"

# Prepare new environment
mkdir -p "$NEW_DIR"
cp -r . "$NEW_DIR"

# Build and start new containers
cd "$NEW_DIR"
docker-compose -f docker-compose.prod.yml up -d

# Wait for services to be healthy
echo "⏳ Waiting for services to be healthy..."
for i in {1..60}; do
    if curl -sf https://openrisk.yourdomain.com/api/v1/health > /dev/null; then
        echo "✅ Services are healthy"
        break
    fi
    if [ $i -eq 60 ]; then
        echo "❌ Services failed to start"
        docker-compose -f docker-compose.prod.yml down
        exit 1
    fi
    sleep 1
done

# Run smoke tests
echo "🧪 Running smoke tests..."
./tests/smoke-tests.sh || {
    echo "❌ Smoke tests failed"
    docker-compose -f docker-compose.prod.yml down
    exit 1
}

# Switch traffic to new environment
echo "🔄 Switching traffic..."
ln -sfn "$NEW_DIR" "$CURRENT_LINK"

# Stop old containers
if [ "$ACTIVE" != "$NEW_DIR" ]; then
    cd "$ACTIVE"
    docker-compose -f docker-compose.prod.yml down
fi

echo "✅ Deployment completed successfully"
```

### 3. Database Migration

```bash
#!/bin/bash
# migrate-production.sh

set -e

BACKUP_DIR="./backups"
DATE=$(date +%Y%m%d_%H%M%S)

echo "📊 Running production database migration..."

# Backup before migration
echo "Backing up database..."
docker-compose -f docker-compose.prod.yml exec db pg_dump \
  -U ${DB_USER} ${DB_NAME} > "$BACKUP_DIR/pre_migration_${DATE}.sql"

echo "✅ Backup created: $BACKUP_DIR/pre_migration_${DATE}.sql"

# Run migrations
echo "Running migrations..."
docker-compose -f docker-compose.prod.yml exec backend \
  openrisk migrate up

echo "✅ Migrations completed"

# Verify migration
echo "Verifying migration..."
docker-compose -f docker-compose.prod.yml exec db psql -U ${DB_USER} -d ${DB_NAME} \
  -c "SELECT COUNT(*) FROM schema_migrations;"

echo "✅ Migration verification passed"
```

## Monitoring & Observability

### Health Check Endpoint

```bash
# Check backend health
curl -v https://openrisk.yourdomain.com/api/v1/health

# Expected response:
# HTTP/2 200
# {
#   "status": "ok",
#   "timestamp": "2025-12-08T10:00:00Z",
#   "version": "1.0.0",
#   "checks": {
#     "database": "ok",
#     "redis": "ok"
#   }
# }
```

### Key Metrics to Monitor

```
Backend Metrics:
- Request latency (p50, p95, p99)
- Error rate (5xx, 4xx)
- Active connections
- Memory usage
- CPU usage
- Database query time
- Cache hit ratio

Frontend Metrics:
- Page load time
- Time to interactive
- JavaScript bundle size
- API response time
- Error logs

Infrastructure Metrics:
- Disk usage
- Network bandwidth
- Container restarts
- SSL certificate expiration
- Database disk usage
```

### Prometheus Monitoring Example

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'openrisk-backend'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/api/v1/metrics'

  - job_name: 'postgres'
    static_configs:
      - targets: ['localhost:5432']

  - job_name: 'redis'
    static_configs:
      - targets: ['localhost:6379']
```

## Incident Response

### Service Down

```bash
#!/bin/bash
# incident-recovery.sh

echo "🚨 Incident response initiated..."

# 1. Check service status
docker-compose -f docker-compose.prod.yml ps

# 2. Check logs for errors
docker-compose -f docker-compose.prod.yml logs backend | tail -100

# 3. Restart affected service
docker-compose -f docker-compose.prod.yml restart backend

# 4. Verify health
sleep 5
curl -v https://openrisk.yourdomain.com/api/v1/health

# 5. If still failing, trigger rollback
# See rollback section below
```

### Rollback Procedure

```bash
#!/bin/bash
# rollback.sh

set -e

CURRENT=$(readlink /opt/openrisk-prod/current)

echo "⏮️  Initiating rollback from $CURRENT..."

# Restore from backup
BACKUP_FILE="./backups/pre_migration_${PREVIOUS_DATE}.sql"
[ -f "$BACKUP_FILE" ] || { echo "❌ Backup not found"; exit 1; }

echo "Restoring database from backup..."
docker-compose -f docker-compose.prod.yml exec db psql -U ${DB_USER} < "$BACKUP_FILE"

# Stop current containers
cd "$CURRENT"
docker-compose -f docker-compose.prod.yml down

# Switch to previous version
ln -sfn "$PREVIOUS_DIR" /opt/openrisk-prod/current

# Start previous version
cd /opt/openrisk-prod/current
docker-compose -f docker-compose.prod.yml up -d

# Verify
sleep 5
curl -v https://openrisk.yourdomain.com/api/v1/health

echo "✅ Rollback completed"
```

## Regular Maintenance

### Weekly Tasks

```bash
# Check certificate expiration (> 30 days OK)
openssl x509 -in /opt/openrisk-prod/certs/fullchain.pem -noout -dates

# Review logs for errors
docker-compose -f docker-compose.prod.yml logs backend | grep ERROR | wc -l

# Check disk usage
df -h /opt/openrisk-prod

# Verify backups
ls -lah ./backups/ | tail -10
```

### Monthly Tasks

```bash
# Database optimization
docker-compose -f docker-compose.prod.yml exec db \
  psql -U ${DB_USER} -d ${DB_NAME} -c "VACUUM ANALYZE;"

# Review and rotate logs
logrotate -v /etc/logrotate.d/openrisk

# Security patches
sudo apt-get update && sudo apt-get upgrade -y
docker pull postgres:15-alpine
docker pull redis:7-alpine
```

### Quarterly Tasks

```bash
# Full system audit
./scripts/security-audit.sh

# Load testing
./scripts/load-test.sh

# Disaster recovery drill
./scripts/dr-test.sh

# Capacity planning review
./scripts/capacity-planning.sh
```

## Performance Tuning

### Database Optimization

```sql
-- Check slow queries
SELECT query, calls, total_time, mean_time 
FROM pg_stat_statements 
ORDER BY mean_time DESC 
LIMIT 10;

-- Add indexes as needed
CREATE INDEX idx_risks_status ON risks(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_mitigations_risk_id ON mitigations(risk_id);

-- Update statistics
ANALYZE;
```

### Redis Optimization

```bash
# Monitor memory usage
docker-compose -f docker-compose.prod.yml exec redis redis-cli INFO memory

# Configure eviction policy
docker-compose -f docker-compose.prod.yml exec redis \
  redis-cli CONFIG SET maxmemory-policy allkeys-lru

# Persist data
docker-compose -f docker-compose.prod.yml exec redis \
  redis-cli BGSAVE
```

### Container Resource Limits

```yaml
# docker-compose.prod.yml
services:
  backend:
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
```

## Disaster Recovery

### Recovery Time Objectives (RTO)

| Scenario | RTO | Recovery Method |
|----------|-----|-----------------|
| Single service restart | 5 min | Auto-restart |
| Entire service failure | 15 min | Blue-green failover |
| Database corruption | 1 hour | Restore from backup |
| Data center failure | 4 hours | DR site failover |

### Backup & Restore

```bash
# Daily automated backup
0 2 * * * /opt/openrisk-prod/backup.sh

# Monthly full system backup
0 3 1 * * /opt/openrisk-prod/backup-full.sh

# Restore from backup
psql -U ${DB_USER} -d ${DB_NAME} < backups/db_backup.sql

# Verify restore
psql -U ${DB_USER} -d ${DB_NAME} -c "SELECT COUNT(*) FROM schema_migrations;"
```

## Compliance & Audit

### Log Retention

```bash
# Keep logs for 90 days
logrotate -v /etc/logrotate.d/openrisk

# Archive old logs
tar -czf logs/archive/openrisk_logs_2025-01.tar.gz logs/2025-01-*.log

# Compress to save space
gzip logs/*.log
```

### Data Protection

- [ ] Encrypted database backups (AES-256)
- [ ] Encrypted data in transit (TLS 1.2+)
- [ ] Encrypted data at rest (dm-crypt or similar)
- [ ] Access logs and audit trails
- [ ] Regular security audits
- [ ] Penetration testing (annual)
- [ ] GDPR compliance verified

## Cost Optimization

### Resource Monitoring

```bash
# Analyze Docker resource usage
docker stats

# Identify unused resources
docker ps -a --filter "status=exited"
docker volume ls --filter "dangling=true"

# Clean up
docker system prune --volumes
```

### Scaling Strategy

```yaml
# Horizontal scaling (load balancing)
- Add multiple backend instances
- Use load balancer (HAProxy, Nginx)
- Configure health checks

# Vertical scaling (more resources)
- Increase container memory limits
- Increase CPU cores
- Upgrade database hardware
```

## Contact & Escalation

**On-Call Schedule**: [Link to rotation]
**Incident Channel**: #incidents on Slack
**Escalation**: Escalate to CTO after 15 min if not resolved

| Issue Type | Contact | Severity |
|-----------|---------|----------|
| Service down | On-call engineer | Critical |
| Database issues | DBA on-call | High |
| Performance degradation | Performance team | Medium |
| Security incident | Security team | Critical |

---

**Last Updated**: 2025-12-08  
**Next Review**: 2026-01-08
