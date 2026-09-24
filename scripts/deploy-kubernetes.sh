#!/bin/bash
# Kubernetes deployment script for OpenRisk
# Automates the complete deployment process

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
HELM_RELEASE="openrisk"
HELM_CHART="./helm/openrisk"
NAMESPACE="openrisk"
CONTEXT="${1:-kind-openrisk}"

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found. Please install kubectl."
        exit 1
    fi
    log_success "kubectl found: $(kubectl version --client --short)"
    
    # Check helm
    if ! command -v helm &> /dev/null; then
        log_error "helm not found. Please install helm."
        exit 1
    fi
    log_success "helm found: $(helm version --short)"
    
    # Check cluster connection
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster. Please configure kubectl."
        exit 1
    fi
    log_success "Connected to cluster: $(kubectl cluster-info | grep 'Kubernetes master')"
}

# Validate Helm chart
validate_helm_chart() {
    log_info "Validating Helm chart..."
    
    if [[ ! -f "$HELM_CHART/Chart.yaml" ]]; then
        log_error "Chart.yaml not found at $HELM_CHART"
        exit 1
    fi
    
    helm lint "$HELM_CHART"
    log_success "Helm chart validation passed"
}

# Create namespace
create_namespace() {
    log_info "Creating namespace '$NAMESPACE'..."
    
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_warning "Namespace '$NAMESPACE' already exists"
    else
        kubectl create namespace "$NAMESPACE"
        kubectl label namespace "$NAMESPACE" name="$NAMESPACE"
        log_success "Namespace created"
    fi
}

# Create secrets
create_secrets() {
    log_info "Creating secrets..."
    
    # Check if secrets already exist
    if kubectl get secret openrisk-secrets -n "$NAMESPACE" &> /dev/null; then
        log_warning "Secret 'openrisk-secrets' already exists"
        return
    fi
    
    # Prompt for secret values
    read -sp "PostgreSQL password: " DB_PASSWORD
    echo
    read -sp "Redis password: " REDIS_PASSWORD
    echo
    read -sp "OAuth2 Client ID: " OAUTH2_CLIENT_ID
    echo
    read -sp "OAuth2 Client Secret: " OAUTH2_CLIENT_SECRET
    echo
    
    # Create secret
    kubectl create secret generic openrisk-secrets \
        --from-literal=database-url="postgresql://openrisk:${DB_PASSWORD}@postgres:5432/openrisk?sslmode=require" \
        --from-literal=redis-url="redis://:${REDIS_PASSWORD}@redis:6379/0" \
        --from-literal=oauth2-client-id="$OAUTH2_CLIENT_ID" \
        --from-literal=oauth2-client-secret="$OAUTH2_CLIENT_SECRET" \
        -n "$NAMESPACE"
    
    log_success "Secrets created"
}

# Install Nginx Ingress Controller
install_ingress_controller() {
    log_info "Checking Nginx Ingress Controller..."
    
    if kubectl get namespace ingress-nginx &> /dev/null; then
        log_warning "Ingress namespace already exists"
    else
        log_info "Installing Nginx Ingress Controller..."
        helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
        helm repo update
        helm install nginx-ingress ingress-nginx/ingress-nginx \
            --namespace ingress-nginx \
            --create-namespace \
            --set controller.service.type=LoadBalancer \
            --wait
        log_success "Nginx Ingress Controller installed"
    fi
}

# Install Cert-Manager
install_cert_manager() {
    log_info "Checking cert-manager..."
    
    if kubectl get namespace cert-manager &> /dev/null; then
        log_warning "cert-manager namespace already exists"
    else
        log_info "Installing cert-manager..."
        kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
        kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=cert-manager -n cert-manager --timeout=300s
        log_success "cert-manager installed"
    fi
}

# Deploy with Helm
deploy_helm() {
    log_info "Deploying OpenRisk with Helm..."
    
    # Create values file if provided
    VALUES_FILE="${2:-values.yaml}"
    
    if [[ ! -f "$VALUES_FILE" ]]; then
        log_warning "Values file not found: $VALUES_FILE, using defaults"
        VALUES_FILE=""
    fi
    
    # Dry-run first
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Performing dry-run..."
        helm install "$HELM_RELEASE" "$HELM_CHART" \
            -n "$NAMESPACE" \
            $([ -z "$VALUES_FILE" ] && echo "" || echo "-f $VALUES_FILE") \
            --dry-run \
            --debug
        log_success "Dry-run completed successfully"
        return
    fi
    
    # Check if release exists
    if helm list -n "$NAMESPACE" | grep -q "$HELM_RELEASE"; then
        log_info "Upgrading existing release..."
        helm upgrade "$HELM_RELEASE" "$HELM_CHART" \
            -n "$NAMESPACE" \
            $([ -z "$VALUES_FILE" ] && echo "" || echo "-f $VALUES_FILE") \
            --wait \
            --timeout 10m
    else
        log_info "Installing new release..."
        helm install "$HELM_RELEASE" "$HELM_CHART" \
            -n "$NAMESPACE" \
            $([ -z "$VALUES_FILE" ] && echo "" || echo "-f $VALUES_FILE") \
            --wait \
            --timeout 10m
    fi
    
    log_success "Helm deployment completed"
}

# Wait for deployment
wait_for_deployment() {
    log_info "Waiting for deployments to be ready..."
    
    kubectl wait --for=condition=available --timeout=300s \
        deployment/backend -n "$NAMESPACE" || log_warning "Backend deployment timeout"
    
    kubectl wait --for=condition=available --timeout=300s \
        deployment/frontend -n "$NAMESPACE" || log_warning "Frontend deployment timeout"
    
    log_success "Deployments ready"
}

# Verify deployment
verify_deployment() {
    log_info "Verifying deployment..."
    
    echo ""
    log_info "Pod Status:"
    kubectl get pods -n "$NAMESPACE"
    
    echo ""
    log_info "Service Status:"
    kubectl get svc -n "$NAMESPACE"
    
    echo ""
    log_info "Ingress Status:"
    kubectl get ingress -n "$NAMESPACE"
    
    # Check pod health
    local unhealthy=$(kubectl get pods -n "$NAMESPACE" --field-selector=status.phase!=Running -o name | wc -l)
    if [[ $unhealthy -eq 0 ]]; then
        log_success "All pods are running"
    else
        log_warning "Some pods are not running"
    fi
}

# Show access information
show_access_info() {
    log_info "Deployment complete! Access information:"
    
    echo ""
    log_info "Using port-forwarding:"
    echo "  Backend: kubectl port-forward -n $NAMESPACE svc/backend 8080:8080"
    echo "  Frontend: kubectl port-forward -n $NAMESPACE svc/frontend 3000:80"
    
    echo ""
    log_info "Using Ingress (requires DNS setup):"
    local ingress_ip=$(kubectl get ingress openrisk-ingress -n "$NAMESPACE" -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "PENDING")
    echo "  Domain: openrisk.example.com"
    echo "  Ingress IP: $ingress_ip"
    
    echo ""
    log_info "Database access:"
    echo "  kubectl exec -it -n $NAMESPACE postgres-0 -- psql -U openrisk -d openrisk"
    
    echo ""
    log_info "View logs:"
    echo "  kubectl logs -n $NAMESPACE -f deployment/backend"
    echo "  kubectl logs -n $NAMESPACE -f deployment/frontend"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up..."
    
    if [[ "$CLEANUP" == "true" ]]; then
        log_warning "Uninstalling Helm release..."
        helm uninstall "$HELM_RELEASE" -n "$NAMESPACE" || true
        
        log_warning "Deleting namespace..."
        kubectl delete namespace "$NAMESPACE" || true
        
        log_success "Cleanup completed"
    fi
}

# Main execution
main() {
    log_info "OpenRisk Kubernetes Deployment Script"
    echo ""
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --dry-run)
                DRY_RUN="true"
                shift
                ;;
            --cleanup)
                CLEANUP="true"
                shift
                ;;
            --skip-ingress)
                SKIP_INGRESS="true"
                shift
                ;;
            --skip-cert-manager)
                SKIP_CERT_MANAGER="true"
                shift
                ;;
            --values)
                VALUES_FILE="$2"
                shift 2
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
    
    # Execute deployment
    check_prerequisites
    validate_helm_chart
    create_namespace
    create_secrets
    
    if [[ "$SKIP_INGRESS" != "true" ]]; then
        install_ingress_controller
    fi
    
    if [[ "$SKIP_CERT_MANAGER" != "true" ]]; then
        install_cert_manager
    fi
    
    deploy_helm
    wait_for_deployment
    verify_deployment
    show_access_info
    
    trap cleanup EXIT
}

# Execute main function
main "$@"
