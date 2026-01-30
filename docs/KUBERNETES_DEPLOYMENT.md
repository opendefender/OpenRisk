 Kubernetes Deployment Guide for OpenRisk

 Overview

This guide covers complete Kubernetes deployment of OpenRisk using Helm charts. The solution includes:

- Backend API: Go Fiber microservice with + replicas and auto-scaling
- Frontend: React/TypeScript SPA with Nginx serving
- Database: PostgreSQL StatefulSet with persistent volumes
- Cache: Redis with persistence and metrics
- Ingress: Nginx Ingress Controller with TLS/SSL
- Monitoring: Prometheus + Grafana integration
- High Availability: Pod Disruption Budgets, Anti-affinity rules, Health checks
- Security: Network Policies, RBAC, Pod Security Context, Secrets management

 Prerequisites

 Required Tools

bash
 Kubernetes CLI
kubectl version --client

 Helm (v.+)
helm version

 Docker (for building images)
docker version

 Optional: kubectx for context switching
brew install kubectx   macOS


 Kubernetes Cluster

- Kubernetes .+ (supports both managed and self-hosted)
- + GB RAM, + CPU cores minimum
- Storage provisioner for persistent volumes
- Ingress Controller (e.g., Nginx Ingress Controller)

 Recommended Cluster Setup

bash
 Using Kind (local development)
kind create cluster --name openrisk --config - <<EOF
kind: Cluster
apiVersion: kind.x-ks.io/valpha
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 
    hostPort: 
    protocol: TCP
  - containerPort: 
    hostPort: 
    protocol: TCP
- role: worker
- role: worker
EOF

 Verify cluster
kubectl cluster-info
kubectl get nodes


 Installation Steps

 . Add Helm Repository (Optional)

bash
 If deploying from a Helm repository
helm repo add opendefender https://charts.opendefender.io
helm repo update


 . Create Namespace

bash
 Create dedicated namespace
kubectl create namespace openrisk

 Label namespace for network policies
kubectl label namespace openrisk name=openrisk


 . Prepare Secrets

bash
 Option A: Create secrets from command line
kubectl create secret generic openrisk-secrets \
  --from-literal=database-url='postgresql://openrisk:password@postgres:/openrisk' \
  --from-literal=redis-url='redis://:password@redis:/' \
  --from-literal=jwt-secret='your-secret-key-here' \
  --from-literal=oauth-client-id='your-client-id' \
  --from-literal=oauth-client-secret='your-client-secret' \
  -n openrisk

 Option B: Create sealed secrets (production recommended)
kubectl apply -f sealed-secrets.yaml


 . Create Custom Values File

bash
 Create values-prod.yaml
cat > values-prod.yaml <<EOF
global:
  namespace: openrisk
  environment: production
  domain: openrisk.example.com

backend:
  replicaCount: 
  image:
    repository: ghcr.io/alex-dembele/openrisk-backend
    tag: v..
  resources:
    requests:
      cpu: m
      memory: Mi
    limits:
      cpu: m
      memory: Gi

frontend:
  replicaCount: 
  image:
    repository: ghcr.io/alex-dembele/openrisk-frontend
    tag: v..
  resources:
    requests:
      cpu: m
      memory: Mi
    limits:
      cpu: m
      memory: Mi

postgresql:
  auth:
    password: "$(openssl rand -base )"
    username: openrisk
    database: openrisk
  primary:
    persistence:
      size: Gi

redis:
  auth:
    password: "$(openssl rand -base )"

certManager:
  enabled: true
  issuer:
    email: admin@openrisk.example.com
EOF


 . Install Helm Chart

bash
 Dry-run to verify manifests
helm install openrisk ./helm/openrisk \
  -n openrisk \
  -f values-prod.yaml \
  --dry-run \
  --debug

 Actual installation
helm install openrisk ./helm/openrisk \
  -n openrisk \
  -f values-prod.yaml

 Monitor installation
kubectl get pods -n openrisk -w
kubectl get svc -n openrisk
kubectl get ingress -n openrisk


 . Install Nginx Ingress Controller (if not already installed)

bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update

helm install nginx-ingress ingress-nginx/ingress-nginx \
  --namespace ingress-nginx \
  --create-namespace \
  --set controller.service.type=LoadBalancer


 . Install Cert-Manager for SSL (Optional)

bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v../cert-manager.yaml

 Create ClusterIssuer
kubectl apply -f - <<EOF
apiVersion: cert-manager.io/v
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v.api.letsencrypt.org/directory
    email: admin@openrisk.example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http:
        ingress:
          class: nginx
EOF


 Verification

 Check Deployment Status

bash
 Check pods
kubectl get pods -n openrisk
kubectl describe pod <pod-name> -n openrisk
kubectl logs <pod-name> -n openrisk

 Check services
kubectl get svc -n openrisk
kubectl get endpoints -n openrisk

 Check ingress
kubectl get ingress -n openrisk
kubectl describe ingress openrisk-ingress -n openrisk

 Check PVCs
kubectl get pvc -n openrisk


 Test Connectivity

bash
 Port-forward to test backend
kubectl port-forward -n openrisk svc/backend : &
curl http://localhost:/health

 Port-forward to test frontend
kubectl port-forward -n openrisk svc/frontend : &
curl http://localhost:

 Test through ingress (requires DNS setup)
curl https://openrisk.example.com/health
curl https://openrisk.example.com/api/health


 Database Verification

bash
 Connect to PostgreSQL
kubectl exec -it -n openrisk postgres- -- psql -U openrisk -d openrisk

 Within psql:
\dt                     List tables
\d risks                Describe risks table
SELECT COUNT() FROM risks;   Count risks


 Redis Verification

bash
 Connect to Redis
kubectl exec -it -n openrisk redis-master- -- redis-cli

 Within redis-cli:
PING                    Verify connection
INFO                    Server info
KEYS                   List all keys


 Configuration

 Custom Backend Configuration

Edit values.yaml to customize:

yaml
backend:
  env:
    LOG_LEVEL: "debug"         Change log level
    PORT: ""               Change port
    ENVIRONMENT: "staging"     Different environment
  
  resources:
    requests:
      cpu: m                Minimum CPU
      memory: Mi            Minimum memory
    limits:
      cpu: m               Maximum CPU
      memory: Gi              Maximum memory
  
  autoscaling:
    minReplicas:              Minimum pods
    maxReplicas:             Maximum pods
    targetCPUUtilizationPercentage: 


 Custom Domain Setup

bash
 Update values.yaml
global:
  domain: your-domain.com

 Update DNS records
 A record pointing to ingress IP
 CNAME for SSL certificate


 Database Backup Strategy

yaml
backup:
  enabled: true
  schedule: "    "       Daily at  AM UTC
  retention:                Keep  days
  storageClass: standard
  size: Gi


 Updating & Upgrading

 Update Helm Release

bash
 Update values
helm values openrisk -n openrisk > values-current.yaml
 Edit values-current.yaml

 Upgrade release
helm upgrade openrisk ./helm/openrisk \
  -n openrisk \
  -f values-current.yaml

 Monitor upgrade
kubectl rollout status deployment/backend -n openrisk
kubectl rollout status deployment/frontend -n openrisk


 Rollback on Issues

bash
 View release history
helm history openrisk -n openrisk

 Rollback to previous version
helm rollback openrisk -n openrisk

 Rollback to specific revision
helm rollback openrisk  -n openrisk


 Monitoring & Observability

 Enable Prometheus Monitoring

bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install prometheus prometheus-community/kube-prometheus-stack \
  -n monitoring \
  --create-namespace


 Access Grafana Dashboard

bash
 Port-forward to Grafana
kubectl port-forward -n monitoring svc/prometheus-grafana : &

 Access at http://localhost:
 Default credentials: admin/prom-operator


 View Application Metrics

bash
 Port-forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus-kube-prometheus-prometheus : &

 Access at http://localhost:
 Query examples:
 - rate(http_requests_total[m])
 - container_memory_usage_bytes{pod=~"backend."}


 Troubleshooting

 Common Issues

 Pods Not Starting

bash
 Check pod status
kubectl describe pod <pod-name> -n openrisk

 Check resource availability
kubectl top nodes
kubectl top pods -n openrisk

 Check events
kubectl get events -n openrisk --sort-by='.lastTimestamp'


 Database Connection Issues

bash
 Test database connectivity
kubectl exec -it -n openrisk <backend-pod> -- \
  psql "$DATABASE_URL" -c "SELECT version();"

 Check database credentials
kubectl get secret openrisk-secrets -n openrisk -o yaml


 Ingress Not Working

bash
 Check ingress status
kubectl describe ingress openrisk-ingress -n openrisk

 Check ingress controller logs
kubectl logs -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx

 Verify DNS resolution
nslookup openrisk.example.com


 Debug Container

bash
 Launch debug pod
kubectl debug -it <pod-name> -n openrisk --image=busybox

 Or create temporary debug container
kubectl run -it --rm debug --image=busybox --restart=Never -- sh


 Security Best Practices

 . Network Policies

Already configured in networkpolicy.yaml. Restricts:
- Inbound traffic (only from Ingress Controller)
- Outbound traffic (only to necessary services)

 . Pod Security Context

Already configured:
- Non-root user (UID )
- No privilege escalation
- Read-only filesystem (where applicable)

 . Secrets Management

Development: Inline secrets (values.yaml)
Production: Use one of:
- Sealed Secrets
- Hashicorp Vault
- AWS Secrets Manager
- Azure Key Vault
- Google Cloud Secrets

bash
 Example: Using Sealed Secrets
 Install sealed-secrets controller
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v../controller.yaml

 Create sealed secret
echo -n "my-secret-value" | kubectl create secret generic my-secret \
  --dry-run=client \
  --from-file=value=/dev/stdin \
  -o yaml | kubeseal -f - > sealed-secret.yaml

kubectl apply -f sealed-secret.yaml


 . RBAC

Service account created with minimal permissions. Extend as needed:

yaml
apiVersion: rbac.authorization.ks.io/v
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


 . Network Encryption

- TLS/SSL enabled via cert-manager and Let's Encrypt
- All ingress traffic encrypted
- Database connections use SSL mode
- Internal service-to-service can use mTLS

 Performance Optimization

 Resource Optimization

yaml
 Update resources in values.yaml based on monitoring
backend:
  resources:
    requests:
      cpu: m       Adjust based on actual usage
      memory: Mi
    limits:
      cpu: m
      memory: Gi

 Enable caching with Redis
redis:
  enabled: true
  master:
    persistence:
      size: Gi


 Horizontal Pod Autoscaling

Already configured with CPU and memory targets:

bash
 View HPA status
kubectl get hpa -n openrisk
kubectl describe hpa backend-hpa -n openrisk

 Manual scaling
kubectl scale deployment backend --replicas= -n openrisk


 Database Performance

bash
 Monitor database performance
kubectl exec -it -n openrisk postgres- -- psql -U openrisk -d openrisk

 Create indexes for common queries
CREATE INDEX idx_risks_status ON risks(status);
CREATE INDEX idx_risks_created_at ON risks(created_at);

 Analyze query performance
EXPLAIN ANALYZE SELECT  FROM risks WHERE status='active';


 Maintenance

 Regular Tasks

bash
 Daily: Check pod status
kubectl get pods -n openrisk

 Weekly: Check resource usage
kubectl top nodes
kubectl top pods -n openrisk

 Monthly: Review and update
helm fetch opendefender/openrisk --untar
helm upgrade openrisk ./openrisk -n openrisk -f values.yaml

 Quarterly: Full backup
kubectl exec -i -n openrisk postgres- -- pg_dump -U openrisk openrisk > backup.sql


 Backup & Restore

bash
 Backup PostgreSQL
kubectl exec -it -n openrisk postgres- -- pg_dump -U openrisk openrisk > backup.sql

 Restore PostgreSQL
kubectl exec -i -n openrisk postgres- -- psql -U openrisk openrisk < backup.sql

 Backup entire namespace
kubectl get all -n openrisk -o yaml > openrisk-backup.yaml

 Restore entire namespace
kubectl apply -f openrisk-backup.yaml


 Next Steps

. Set up CI/CD: Deploy automatically from GitHub
. Add monitoring: Prometheus + Grafana dashboards
. Enable auto-scaling: Based on metrics
. Configure backups: Automated database backups
. Implement GitOps: Using Flux or ArgoCD
