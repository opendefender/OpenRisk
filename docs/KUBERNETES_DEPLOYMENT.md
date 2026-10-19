# Kubernetes Deployment Guide for OpenRisk

## Overview

This guide covers complete Kubernetes deployment of OpenRisk using Helm charts. The solution includes:

- **Backend API**: Go Fiber microservice with 3+ replicas and auto-scaling
- **Frontend**: React/TypeScript SPA with Nginx serving
- **Database**: PostgreSQL StatefulSet with persistent volumes
- **Cache**: Redis with persistence and metrics
- **Ingress**: Nginx Ingress Controller with TLS/SSL
- **Monitoring**: Prometheus + Grafana integration
- **High Availability**: Pod Disruption Budgets, Anti-affinity rules, Health checks
- **Security**: Network Policies, RBAC, Pod Security Context, Secrets management

## Prerequisites

### Required Tools

```bash
# Kubernetes CLI
kubectl version --client

# Helm (v3.0+)
helm version

# Docker (for building images)
docker version

# Optional: kubectx for context switching
brew install kubectx  # macOS
```

### Kubernetes Cluster

- Kubernetes 1.24+ (supports both managed and self-hosted)
- 8+ GB RAM, 4+ CPU cores minimum
- Storage provisioner for persistent volumes
- Ingress Controller (e.g., Nginx Ingress Controller)

### Recommended Cluster Setup

```bash
# Using Kind (local development)
kind create cluster --name openrisk --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
- role: worker
- role: worker
EOF

# Verify cluster
kubectl cluster-info
kubectl get nodes
```

## Installation Steps

### 1. Add Helm Repository (Optional)

```bash
# If deploying from a Helm repository
helm repo add opendefender https://charts.opendefender.io
helm repo update
```

### 2. Namespace

The chart creates the `global.namespace` namespace (default `openrisk`) with
the `name=openrisk` label its network policy selects. Do not create it
beforehand: Helm refuses to install over a namespace it does not own.

### 3. Prepare Secrets

The chart does not deploy PostgreSQL or Redis: point it at yours. It creates
two Secrets itself, `openrisk-secrets` and `openrisk-rsa`, from the values you
pass, so do not create them by hand (the install would fail on a conflict).

```bash
# RS256 signing pair, shared by every backend replica (#1022). Keep these files:
# a new pair signs everyone out.
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out private.pem
openssl rsa -in private.pem -pubout -out public.pem

# The database URL. Percent-encode the password: any character works then,
# '@ : / ? # %' and spaces included (#1001).
DB_PASSWORD='your password'
ENCODED=$(python3 -c 'import sys,urllib.parse;print(urllib.parse.quote(sys.argv[1],safe=""))' "$DB_PASSWORD")
DATABASE_URL="postgres://openrisk:${ENCODED}@postgres.example.internal:5432/openrisk?sslmode=require"
```

To manage the RSA pair outside Helm, create a Secret with `private.pem` and
`public.pem` and set `secrets.rsa.existingSecret` to its name.

### 4. Create Custom Values File

```yaml
# values-prod.yaml
global:
  namespace: openrisk
  domain: openrisk.example.com

backend:
  replicaCount: 3
  image:
    repository: ghcr.io/alex-dembele/openrisk-backend
    tag: v1.0.0
  # Uploaded evidence and exports. The backend stores files on disk, so every
  # replica must mount the same volume: a ReadWriteMany class (NFS, EFS,
  # CephFS...). With a ReadWriteOnce class, set replicaCount: 1 and
  # autoscaling.enabled: false; the chart refuses anything else.
  uploads:
    storageClass: your-rwx-class
    accessMode: ReadWriteMany
    size: 20Gi

frontend:
  replicaCount: 2
  image:
    repository: ghcr.io/alex-dembele/openrisk-frontend
    tag: v1.0.0

secrets:
  inline:
    redisUrl: redis://redis.example.internal:6379/0
```

### 5. Install Helm Chart

```bash
helm install openrisk ./helm/openrisk \
  -f values-prod.yaml \
  --set-string secrets.inline.databaseUrl="$DATABASE_URL" \
  --set-file secrets.inline.rsaPrivateKey=private.pem \
  --set-file secrets.inline.rsaPublicKey=public.pem

# The backend runs the SQL migrations at boot; its log says so:
kubectl -n openrisk logs deploy/backend | grep 'migrations: applied successfully'
kubectl -n openrisk rollout status deploy/backend
kubectl -n openrisk rollout status deploy/frontend
```

The `helm-kind` workflow installs the chart this way on every pull request
that touches it, and checks that both deployments become Ready and stay up
(`helm/kind/` holds its values).

### 6. Install Nginx Ingress Controller (if not already installed)

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

helm install nginx-ingress ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --set controller.service.type=LoadBalancer
```

### 7. Install Cert-Manager for SSL (Optional)

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# Create ClusterIssuer
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: admin@openrisk.example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
EOF
```

## Verification

### Check Deployment Status

```bash
# Check pods
kubectl get pods -n openrisk
kubectl describe pod <pod-name> -n openrisk
kubectl logs <pod-name> -n openrisk

# Check services
kubectl get svc -n openrisk
kubectl get endpoints -n openrisk

# Check ingress
kubectl get ingress -n openrisk
kubectl describe ingress openrisk-ingress -n openrisk

# Check PVCs
kubectl get pvc -n openrisk
```

### Test Connectivity

```bash
# Port-forward to test backend
kubectl port-forward -n openrisk svc/backend 8080:8080 &
curl http://localhost:8080/health

# Port-forward to test frontend
kubectl port-forward -n openrisk svc/frontend 3000:80 &
curl http://localhost:3000

# Test through ingress (requires DNS setup)
curl https://openrisk.example.com/health
curl https://openrisk.example.com/api/health
```

### Database Verification

```bash
# Connect to PostgreSQL
kubectl exec -it -n openrisk postgres-0 -- psql -U openrisk -d openrisk

# Within psql:
\dt                    # List tables
\d risks               # Describe risks table
SELECT COUNT(*) FROM risks;  # Count risks
```

### Redis Verification

```bash
# Connect to Redis
kubectl exec -it -n openrisk redis-master-0 -- redis-cli

# Within redis-cli:
PING                   # Verify connection
INFO                   # Server info
KEYS *                 # List all keys
```

## Configuration

### Custom Backend Configuration

Edit `values.yaml` to customize:

```yaml
backend:
  env:
    LOG_LEVEL: "debug"        # Change log level
    PORT: "8080"              # Change port
    ENVIRONMENT: "staging"    # Different environment
  
  resources:
    requests:
      cpu: 250m               # Minimum CPU
      memory: 512Mi           # Minimum memory
    limits:
      cpu: 1000m              # Maximum CPU
      memory: 1Gi             # Maximum memory
  
  autoscaling:
    minReplicas: 3            # Minimum pods
    maxReplicas: 10           # Maximum pods
    targetCPUUtilizationPercentage: 70
```

### Custom Domain Setup

```bash
# Update values.yaml
global:
  domain: your-domain.com

# Update DNS records
# A record pointing to ingress IP
# CNAME for SSL certificate
```

### Database Backup Strategy

```yaml
backup:
  enabled: true
  schedule: "0 2 * * *"      # Daily at 2 AM UTC
  retention: 30              # Keep 30 days
  storageClass: standard
  size: 10Gi
```

## Updating & Upgrading

### Update Helm Release

```bash
# Update values
helm values openrisk -n openrisk > values-current.yaml
# Edit values-current.yaml

# Upgrade release
helm upgrade openrisk ./helm/openrisk \
  -n openrisk \
  -f values-current.yaml

# Monitor upgrade
kubectl rollout status deployment/backend -n openrisk
kubectl rollout status deployment/frontend -n openrisk
```

### Rollback on Issues

`helm rollback` alone is not enough when the newer release applied SQL
migrations: the previous image refuses to boot on a database ahead of its own
migrations (`no migration found for version N`) and crash-loops. Record the
previous version with the previous image first (`openrisk migrate force <N>`).
See [docs/runbooks/migrations.md](runbooks/migrations.md#rolling-back-to-a-previous-release),
and [docs/runbooks/migration-0060.md](runbooks/migration-0060.md) for the upgrade
across migration 0060, which also needs longer probes on large databases.

```bash
# View release history
helm history openrisk -n openrisk

# Rollback to previous version
helm rollback openrisk -n openrisk

# Rollback to specific revision
helm rollback openrisk 2 -n openrisk
```

## Monitoring & Observability

### Enable Prometheus Monitoring

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install prometheus prometheus-community/kube-prometheus-stack \
  -n monitoring \
  --create-namespace
```

### Access Grafana Dashboard

```bash
# Port-forward to Grafana
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80 &

# Access at http://localhost:3000
# Default credentials: admin/prom-operator
```

### View Application Metrics

```bash
# Port-forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus 9090:9090 &

# Access at http://localhost:9090
# Query examples:
# - rate(http_requests_total[5m])
# - container_memory_usage_bytes{pod=~"backend.*"}
```

## Troubleshooting

### Common Issues

#### Pods Not Starting

```bash
# Check pod status
kubectl describe pod <pod-name> -n openrisk

# Check resource availability
kubectl top nodes
kubectl top pods -n openrisk

# Check events
kubectl get events -n openrisk --sort-by='.lastTimestamp'
```

#### Database Connection Issues

```bash
# Test database connectivity
kubectl exec -it -n openrisk <backend-pod> -- \
  psql "$DATABASE_URL" -c "SELECT version();"

# Check database credentials
kubectl get secret openrisk-secrets -n openrisk -o yaml
```

#### Ingress Not Working

```bash
# Check ingress status
kubectl describe ingress openrisk-ingress -n openrisk

# Check ingress controller logs
kubectl logs -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx

# Verify DNS resolution
nslookup openrisk.example.com
```

### Debug Container

```bash
# Launch debug pod
kubectl debug -it <pod-name> -n openrisk --image=busybox

# Or create temporary debug container
kubectl run -it --rm debug --image=busybox --restart=Never -- sh
```

## Security Best Practices

### 1. Network Policies

Already configured in `networkpolicy.yaml`. Restricts:
- Inbound traffic (only from Ingress Controller)
- Outbound traffic (only to necessary services)

### 2. Pod Security Context

Already configured:
- Non-root user (UID 1000)
- No privilege escalation
- Read-only filesystem (where applicable)

### 3. Secrets Management

**Development**: Inline secrets (values.yaml)
**Production**: Use one of:
- Sealed Secrets
- Hashicorp Vault
- AWS Secrets Manager
- Azure Key Vault
- Google Cloud Secrets

```bash
# Example: Using Sealed Secrets
# Install sealed-secrets controller
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.22.0/controller.yaml

# Create sealed secret
echo -n "my-secret-value" | kubectl create secret generic my-secret \
  --dry-run=client \
  --from-file=value=/dev/stdin \
  -o yaml | kubeseal -f - > sealed-secret.yaml

kubectl apply -f sealed-secret.yaml
```

### 4. RBAC

Service account created with minimal permissions. Extend as needed:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: openrisk-role
  namespace: openrisk
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
```

### 5. Network Encryption

- TLS/SSL enabled via cert-manager and Let's Encrypt
- All ingress traffic encrypted
- Database connections use SSL mode
- Internal service-to-service can use mTLS

## Performance Optimization

### Resource Optimization

```yaml
# Update resources in values.yaml based on monitoring
backend:
  resources:
    requests:
      cpu: 250m      # Adjust based on actual usage
      memory: 512Mi
    limits:
      cpu: 1000m
      memory: 1Gi

# Enable caching with Redis
redis:
  enabled: true
  master:
    persistence:
      size: 10Gi
```

### Horizontal Pod Autoscaling

Already configured with CPU and memory targets:

```bash
# View HPA status
kubectl get hpa -n openrisk
kubectl describe hpa backend-hpa -n openrisk

# Manual scaling
kubectl scale deployment backend --replicas=5 -n openrisk
```

### Database Performance

```bash
# Monitor database performance
kubectl exec -it -n openrisk postgres-0 -- psql -U openrisk -d openrisk

# Create indexes for common queries
CREATE INDEX idx_risks_status ON risks(status);
CREATE INDEX idx_risks_created_at ON risks(created_at);

# Analyze query performance
EXPLAIN ANALYZE SELECT * FROM risks WHERE status='active';
```

## Maintenance

### Regular Tasks

```bash
# Daily: Check pod status
kubectl get pods -n openrisk

# Weekly: Check resource usage
kubectl top nodes
kubectl top pods -n openrisk

# Monthly: Review and update
helm fetch opendefender/openrisk --untar
helm upgrade openrisk ./openrisk -n openrisk -f values.yaml

# Quarterly: Full backup
kubectl exec -i -n openrisk postgres-0 -- pg_dump -U openrisk openrisk > backup.sql
```

### Backup & Restore

```bash
# Backup PostgreSQL
kubectl exec -it -n openrisk postgres-0 -- pg_dump -U openrisk openrisk > backup.sql

# Restore PostgreSQL
kubectl exec -i -n openrisk postgres-0 -- psql -U openrisk openrisk < backup.sql

# Backup entire namespace
kubectl get all -n openrisk -o yaml > openrisk-backup.yaml

# Restore entire namespace
kubectl apply -f openrisk-backup.yaml
```

## Next Steps

1. **Set up CI/CD**: Deploy automatically from GitHub
2. **Add monitoring**: Prometheus + Grafana dashboards
3. **Enable auto-scaling**: Based on metrics
4. **Configure backups**: Automated database backups
5. **Implement GitOps**: Using Flux or ArgoCD
