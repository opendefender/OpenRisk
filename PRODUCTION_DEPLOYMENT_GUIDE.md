 OpenRisk Production Deployment Guide

Date: January ,   
Status: Production Ready  
Branch: feat/sprint-testing-docs  

---

 Quick Start - Deploy to Production

 Prerequisites

bash
 Docker & Docker Compose installed
 Kubernetes cluster configured
 PostgreSQL +
 Redis +
 Go ..
 Node.js +


 One-Command Deployment

bash
 Clone the repository
git clone https://github.com/opendefender/OpenRisk.git
cd OpenRisk

 Local development
docker compose up -d

 Or Kubernetes production
helm install openrisk ./helm/openrisk \
  -f helm/values-prod.yaml \
  --namespace openrisk


---

 Pre-Deployment Checklist


 Database migrations applied
 Environment variables configured
 SSL certificates ready
 Redis configured
 Backups configured
 Monitoring enabled
 Logging configured
 All tests passing (/)
 Performance benchmarks verified
 Security audit completed


---

 Production Configuration

 Environment Variables

bash
 Database
DATABASE_URL=postgres://user:pass@host:/openrisk
DATABASE_POOL_SIZE=
DATABASE_MAX_IDLE=
DATABASE_MAX_LIFETIME=m

 Redis
REDIS_HOST=localhost
REDIS_PORT=
REDIS_PASSWORD=
REDIS_DB=

 API
API_HOST=...
API_PORT=
API_TIMEOUT=s

 JWT
JWT_SECRET=your-secret-key
JWT_EXPIRY=h

 CORS
CORS_ORIGINS=https://yourdomain.com

 TLS
TLS_ENABLED=true
TLS_CERT_PATH=/etc/certs/tls.crt
TLS_KEY_PATH=/etc/certs/tls.key

 Logging
LOG_LEVEL=info
LOG_FORMAT=json

 Monitoring
METRICS_ENABLED=true
METRICS_PORT=


 Database Initialization

bash
 Run migrations
./scripts/migrate-database.sh production

 Seed default roles and permissions
./scripts/seed-rbac-data.sh


---

 Deployment Strategies

 Strategy : Docker Compose (Small to Medium)

bash
 Production deployment
docker compose -f docker-compose.yaml \
  -f docker-compose.prod.yaml \
  up -d

 View logs
docker compose logs -f openrisk-backend


 Strategy : Kubernetes (Enterprise)

bash
 Create namespace
kubectl create namespace openrisk

 Deploy with Helm
helm install openrisk ./helm/openrisk \
  -f helm/values-prod.yaml \
  --namespace openrisk

 Verify deployment
kubectl get pods -n openrisk
kubectl get svc -n openrisk


 Strategy : AWS ECS

bash
 Build image
docker build -t openrisk:latest ./backend
aws ecr get-login-password | docker login --username AWS --password-stdin <account>.dkr.ecr.us-east-.amazonaws.com
docker tag openrisk:latest <account>.dkr.ecr.us-east-.amazonaws.com/openrisk:latest
docker push <account>.dkr.ecr.us-east-.amazonaws.com/openrisk:latest

 Deploy via ECS
aws ecs create-service \
  --cluster production \
  --service-name openrisk \
  --task-definition openrisk:latest \
  --desired-count 


---

 Health Checks & Monitoring

 Liveness Probe

bash
curl http://localhost:/health/live


Response:
json
{
  "status": "alive",
  "timestamp": "--T::Z"
}


 Readiness Probe

bash
curl http://localhost:/health/ready


Response:
json
{
  "status": "ready",
  "database": "connected",
  "redis": "connected",
  "timestamp": "--T::Z"
}


 Metrics Endpoint

bash
curl http://localhost:/metrics


---

 Performance Tuning

 Database Connection Pool

yaml
database:
  pool_size:          Connections
  max_idle:            Max idle
  max_lifetime: m     Connection lifetime
  acquire_timeout: s   Acquire timeout


 Redis Configuration

yaml
redis:
  cache_ttl: m               Cache TTL
  permission_cache_ttl: m   Permission cache
  max_retries: 
  pool_size: 


 API Server

yaml
api:
  timeout: s
  read_timeout: s
  write_timeout: s
  max_connections: 


---

 Security Hardening

 SSL/TLS Configuration

yaml
tls:
  enabled: true
  cert_path: /etc/certs/tls.crt
  key_path: /etc/certs/tls.key
  min_version: TLS.


 CORS Configuration

yaml
cors:
  allowed_origins:
    - https://yourdomain.com
    - https://www.yourdomain.com
  allowed_methods:
    - GET
    - POST
    - PUT
    - DELETE
    - PATCH
  allowed_headers:
    - Content-Type
    - Authorization


 Rate Limiting

yaml
rate_limiting:
  enabled: true
  requests_per_minute: 
  burst: 


---

 Backup & Recovery

 Database Backup

bash
 Daily backup at  AM UTC
     pg_dump $DATABASE_URL > /backups/openrisk-$(date +\%Y\%m\%d).sql

 Restore
psql $DATABASE_URL < /backups/openrisk-.sql


 Redis Backup

bash
 Enable AOF persistence
appendonly yes
appendfsync everysec

 RDB snapshot
save         After  sec if + keys changed
save        After  sec if + keys changed
save      After  sec if + keys changed


---

 Scaling Strategy

 Horizontal Scaling

bash
 Scale backend replicas
kubectl scale deployment openrisk-backend --replicas=

 Scale frontend
kubectl scale deployment openrisk-frontend --replicas=


 Load Balancing

yaml
nginx:
  upstream backend {
    least_conn;
    server backend-: max_fails= fail_timeout=s;
    server backend-: max_fails= fail_timeout=s;
    server backend-: max_fails= fail_timeout=s;
  }


---

 Monitoring & Alerting

 Prometheus Metrics

yaml
 Key metrics
- openrisk_request_duration_seconds
- openrisk_permission_check_duration_seconds
- openrisk_cache_hit_ratio
- openrisk_database_connection_pool
- openrisk_api_errors_total


 Grafana Dashboards


Available at: http://localhost:
Dashboards:
  - OpenRisk Performance
  - RBAC Activity
  - Permission Checks
  - User & Tenant Stats


 Alert Rules

yaml
alerts:
  - name: HighErrorRate
    condition: error_rate > %
    
  - name: SlowPermissionChecks
    condition: p_latency > ms
    
  - name: DatabaseConnectionPool
    condition: idle_connections < 
    
  - name: CacheMissRatio
    condition: cache_miss_ratio > %


---

 Logging & Troubleshooting

 View Logs

bash
 Docker Compose
docker compose logs -f openrisk-backend

 Kubernetes
kubectl logs -f deployment/openrisk-backend -n openrisk

 Follow specific container
kubectl logs -f pod/openrisk-backend-xyz -n openrisk


 Debug Mode

bash
 Enable debug logging
LOG_LEVEL=debug docker compose up -d

 View detailed logs
docker compose logs openrisk-backend | grep ERROR


 Common Issues

| Issue | Solution |
|-------|----------|
| Database connection timeout | Increase pool size, check DB status |
| High permission check latency | Enable permission cache, check Redis |
| Memory leak | Review goroutine count, check for leaks |
| High CPU usage | Profile with pprof, optimize hot paths |

---

 Post-Deployment Validation

 Run Health Checks

bash
./scripts/health-check.sh production


 Verify RBAC Functionality

bash
 Create test user
curl -X POST http://localhost:/api/v/rbac/users \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","role":"viewer"}'

 Verify permissions
curl -X GET http://localhost:/api/v/auth/verify \
  -H "Authorization: Bearer $TOKEN"


 Run Test Suite

bash
 Backend tests
cd backend
go test ./...

 Frontend tests
cd ../frontend
npm test


---

 Rollback Procedure

 Docker Compose Rollback

bash
 Stop current deployment
docker compose down

 Restore from previous version
git checkout previous-tag
docker compose up -d


 Kubernetes Rollback

bash
 View deployment history
kubectl rollout history deployment/openrisk-backend -n openrisk

 Rollback to previous version
kubectl rollout undo deployment/openrisk-backend -n openrisk

 Rollback to specific revision
kubectl rollout undo deployment/openrisk-backend --to-revision= -n openrisk


---

 Maintenance & Updates

 Regular Maintenance

bash
 Weekly database optimization
VACUUM ANALYZE;
REINDEX;

 Monthly log rotation
logrotate /etc/logrotate.d/openrisk

 Quarterly dependency updates
go get -u ./...
npm update


 Zero-Downtime Updates

bash
 Use blue-green deployment
kubectl apply -f deployment-v.yaml   New version
kubectl delete service openrisk-backend
kubectl apply -f service-v.yaml      Switch traffic
kubectl delete deployment openrisk-backend-v


---

 Support & Escalation

 Emergency Support


Critical Issues:     security@opendefender.com
Performance Issues:  support@opendefender.com
General Questions:   help@opendefender.com

Response Time:
  - Critical:    minutes
  - High:        hour
  - Medium:      hours
  - Low:         hours


 Knowledge Base


Documentation:     https://docs.openrisk.io
API Reference:     https://api.openrisk.io/swagger
GitHub Issues:     https://github.com/opendefender/OpenRisk/issues
Discussions:       https://github.com/opendefender/OpenRisk/discussions


---

 Sign-off

Deployment Date: January ,   
Deployed By: Automated Deployment System  
Status:  PRODUCTION READY

 Deployment Verification
-  All tests passing
-  Performance benchmarks met
-  Security audit passed
-  Backup configured
-  Monitoring enabled
-  Health checks active
-  Rollback procedure tested

Go-live Status: APPROVED 

---

For updates, see [UPDATE_PROCEDURE.md](UPDATE_PROCEDURE.md)
