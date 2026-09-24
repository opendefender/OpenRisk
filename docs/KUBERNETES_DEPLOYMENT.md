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

### 2. Create Namespace

```bash
# Create dedicated namespace
kubectl create namespace openrisk

# Label namespace for network policies
kubectl label namespace openrisk name=openrisk
```

### 3. Prepare Secrets

```bash
# Option A: Create secrets from command line
kubectl create secret generic openrisk-secrets \
  --from-literal=database-url='postgresql://openrisk:password@postgres:5432/openrisk' \
  --from-literal=redis-url='redis://:password@redis:6379/0' \
  --from-literal=oauth2-client-id='your-client-id' \
  --from-literal=oauth2-client-secret='your-client-secret' \
  -n openrisk

# Option B: Create sealed secrets (production recommended)
kubectl apply -f sealed-secrets.yaml
```

### 4. Create Custom Values File

```bash
# Create values-prod.yaml
cat > values-prod.yaml <<EOF
global:
  namespace: openrisk
  environment: production
  domain: openrisk.example.com

backend:
  replicaCount: 3
  image:
    repository: ghcr.io/alex-dembele/openrisk-backend
    tag: v1.0.0
  resources:
    requests:
      cpu: 200m
      memory: 512Mi
    limits:
      cpu: 1000m
      memory: 1Gi

frontend:
  replicaCount: 3
  image:
    repository: ghcr.io/alex-dembele/openrisk-frontend
    tag: v1.0.0
  resources:
    requests:
      cpu: 100m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi

postgresql:
  auth:
    password: "$(openssl rand -base64 32)"
    username: openrisk
    database: openrisk
  primary:
    persistence:
      size: 50Gi

redis:
  auth:
    password: "$(openssl rand -base64 32)"

certManager:
  enabled: true
  issuer:
    email: admin@openrisk.example.com
EOF
```

### 5. Install Helm Chart

```bash
# Dry-run to verify manifests
helm install openrisk ./helm/openrisk \
  -n openrisk \
  -f values-prod.yaml \
  --dry-run \
  --debug

# Actual installation
helm install openrisk ./helm/openrisk \
  -n openrisk \
  -f values-prod.yaml

# Monitor installation
kubectl get pods -n openrisk -w
kubectl get svc -n openrisk
kubectl get ingress -n openrisk
```

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
