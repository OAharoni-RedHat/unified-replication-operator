#!/bin/bash
# Installation script for Unified Replication Operator

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default values
NAMESPACE="${NAMESPACE:-unified-replication-system}"
RELEASE_NAME="${RELEASE_NAME:-unified-replication-operator}"
HELM_TIMEOUT="${HELM_TIMEOUT:-5m}"
SKIP_PREFLIGHT="${SKIP_PREFLIGHT:-false}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Pre-flight checks
preflight_checks() {
    echo_info "Running pre-flight checks..."
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        echo_error "kubectl not found. Please install kubectl."
        exit 1
    fi
    
    # Check cluster connectivity
    if ! kubectl cluster-info &> /dev/null; then
        echo_error "Cannot connect to Kubernetes cluster. Please check your kubeconfig."
        exit 1
    fi
    
    # Check Kubernetes version
    K8S_VERSION=$(kubectl version --short 2>/dev/null | grep "Server Version" | awk '{print $3}' || echo "unknown")
    echo_info "Kubernetes version: $K8S_VERSION"
    
    # Check Helm
    if ! command -v helm &> /dev/null; then
        echo_error "Helm not found. Please install Helm 3.x"
        exit 1
    fi
    
    HELM_VERSION=$(helm version --short 2>/dev/null || echo "unknown")
    echo_info "Helm version: $HELM_VERSION"
    
    echo_info "Pre-flight checks passed ✓"
}

# Create namespace
create_namespace() {
    echo_info "Creating namespace: $NAMESPACE"
    
    if kubectl get namespace "$NAMESPACE" &> /dev/null; then
        echo_warn "Namespace $NAMESPACE already exists"
    else
        kubectl create namespace "$NAMESPACE"
        
        # Apply pod security labels
        kubectl label namespace "$NAMESPACE" \
            pod-security.kubernetes.io/enforce=restricted \
            pod-security.kubernetes.io/audit=restricted \
            pod-security.kubernetes.io/warn=restricted \
            --overwrite
        
        echo_info "Namespace created ✓"
    fi
}

# Install CRDs
install_crds() {
    echo_info "Installing CRDs..."
    
    if [ -d "$PROJECT_ROOT/config/crd" ]; then
        kubectl apply -f "$PROJECT_ROOT/config/crd" || echo_warn "CRDs may already exist"
        echo_info "CRDs installed ✓"
    else
        echo_warn "CRD directory not found, skipping"
    fi
}

# Install operator via Helm
install_operator() {
    echo_info "Installing operator via Helm..."
    
    CHART_PATH="$PROJECT_ROOT/helm/unified-replication-operator"
    
    if [ ! -d "$CHART_PATH" ]; then
        echo_error "Helm chart not found at $CHART_PATH"
        exit 1
    fi
    
    helm upgrade --install \
        "$RELEASE_NAME" \
        "$CHART_PATH" \
        --namespace "$NAMESPACE" \
        --create-namespace \
        --timeout "$HELM_TIMEOUT" \
        --wait \
        "$@"
    
    echo_info "Operator installed ✓"
}

# Verify installation
verify_installation() {
    echo_info "Verifying installation..."
    
    # Wait for deployment to be ready
    echo_info "Waiting for deployment to be ready..."
    kubectl wait --for=condition=available \
        --timeout=300s \
        deployment/"$RELEASE_NAME" \
        -n "$NAMESPACE" || echo_warn "Deployment may not be ready yet"
    
    # Check pods
    PODS=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=unified-replication-operator" -o name | wc -l)
    echo_info "Running pods: $PODS"
    
    # Check webhook
    if kubectl get validatingwebhookconfiguration \
        "${RELEASE_NAME}-validating-webhook" &> /dev/null; then
        echo_info "Webhook configured ✓"
    fi
    
    echo_info "Installation verified ✓"
}

# Print status
print_status() {
    echo ""
    echo_info "========================================="
    echo_info "Installation Complete!"
    echo_info "========================================="
    echo ""
    echo_info "Namespace: $NAMESPACE"
    echo_info "Release: $RELEASE_NAME"
    echo ""
    echo_info "View pods:"
    echo "  kubectl get pods -n $NAMESPACE"
    echo ""
    echo_info "View logs:"
    echo "  kubectl logs -n $NAMESPACE -l control-plane=controller-manager -f"
    echo ""
    echo_info "Create a replication:"
    echo "  kubectl apply -f examples/sample-replication.yaml"
    echo ""
    echo_info "Check status:"
    echo "  kubectl get unifiedvolumereplications -A"
    echo ""
}

# Main installation flow
main() {
    echo_info "=== Unified Replication Operator Installation ==="
    echo ""
    
    # Pre-flight checks
    if [ "$SKIP_PREFLIGHT" != "true" ]; then
        preflight_checks
    fi
    
    # Create namespace
    create_namespace
    
    # Install CRDs
    install_crds
    
    # Install operator
    install_operator "$@"
    
    # Verify
    verify_installation
    
    # Print status
    print_status
}

# Run main
main "$@"

