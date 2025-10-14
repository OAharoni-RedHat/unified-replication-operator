#!/bin/bash
# OpenShift-specific installation script for Unified Replication Operator

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default values
NAMESPACE="${NAMESPACE:-unified-replication-system}"
RELEASE_NAME="${RELEASE_NAME:-unified-replication-operator}"
HELM_TIMEOUT="${HELM_TIMEOUT:-10m}"
USE_CUSTOM_SCC="${USE_CUSTOM_SCC:-true}"

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

# Check if running on OpenShift
check_openshift() {
    echo_info "Checking if cluster is OpenShift..."
    
    if kubectl api-resources | grep -q "securitycontextconstraints"; then
        echo_info "OpenShift detected ✓"
        return 0
    else
        echo_warn "This doesn't appear to be an OpenShift cluster"
        echo_warn "OpenShift-specific resources (SCC) will not be created"
        return 1
    fi
}

# Check for existing SCC conflicts
check_scc_conflicts() {
    if [ "$USE_CUSTOM_SCC" != "true" ]; then
        return 0
    fi
    
    echo_info "Checking for SCC conflicts..."
    
    # Check if SCC exists without Helm ownership
    if kubectl get scc unified-replication-operator-scc &> /dev/null; then
        local managed_by=$(kubectl get scc unified-replication-operator-scc -o jsonpath='{.metadata.labels.app\.kubernetes\.io/managed-by}' 2>/dev/null || echo "")
        
        if [ "$managed_by" != "Helm" ]; then
            echo_warn "Found existing SCC not managed by Helm, deleting it..."
            kubectl delete scc unified-replication-operator-scc
            echo_info "Existing SCC deleted, Helm will create it properly"
        fi
    fi
}

# Verify SCC was created by Helm
verify_scc() {
    if [ "$USE_CUSTOM_SCC" != "true" ]; then
        return 0
    fi
    
    echo_info "Verifying SCC was created by Helm..."
    
    # Wait for SCC to be created
    local max_wait=30
    local wait_time=0
    while ! kubectl get scc unified-replication-operator-scc &> /dev/null; do
        if [ $wait_time -ge $max_wait ]; then
            echo_error "Timeout waiting for SCC to be created"
            echo_error "SCC should have been created by Helm"
            exit 1
        fi
        echo "Waiting for SCC to be created by Helm..."
        sleep 2
        wait_time=$((wait_time + 2))
    done
    
    echo_info "SCC verified ✓"
    
    # Check if binding is needed
    local users=$(kubectl get scc unified-replication-operator-scc -o jsonpath='{.users}' 2>/dev/null)
    if echo "$users" | grep -q "system:serviceaccount:${NAMESPACE}:${RELEASE_NAME}"; then
        echo_info "SCC already bound to ServiceAccount by Helm ✓"
    else
        echo_warn "SCC not automatically bound, attempting manual binding..."
        if command -v oc &> /dev/null; then
            oc adm policy add-scc-to-user unified-replication-operator-scc \
                -z "$RELEASE_NAME" \
                -n "$NAMESPACE" || echo_warn "Failed to bind SCC manually"
        else
            echo_warn "OpenShift CLI (oc) not found"
            echo_info "Please run manually:"
            echo "  oc adm policy add-scc-to-user unified-replication-operator-scc -z $RELEASE_NAME -n $NAMESPACE"
        fi
    fi
}

# Install operator
install_operator() {
    echo_info "Installing operator with OpenShift-compatible settings..."
    
    CHART_PATH="$PROJECT_ROOT/helm/unified-replication-operator"
    
    if [ ! -d "$CHART_PATH" ]; then
        echo_error "Helm chart not found at $CHART_PATH"
        exit 1
    fi
    
    # Check if we should use custom SCC or compatible security settings
    if [ "$USE_CUSTOM_SCC" = "true" ]; then
        echo_info "Using custom SCC mode (allows UID 65532)"
        EXTRA_ARGS="--set openshift.enabled=true"
    else
        echo_info "Using OpenShift-compatible security mode (OpenShift assigns UID)"
        EXTRA_ARGS="--set openshift.enabled=false --set openshift.compatibleSecurity=true"
    fi
    
    # Install or upgrade
    if helm list -n "$NAMESPACE" | grep -q "$RELEASE_NAME"; then
        echo_info "Upgrading existing release..."
        helm upgrade \
            "$RELEASE_NAME" \
            "$CHART_PATH" \
            --namespace "$NAMESPACE" \
            --timeout "$HELM_TIMEOUT" \
            --wait \
            $EXTRA_ARGS \
            "$@"
    else
        echo_info "Installing new release..."
        helm install \
            "$RELEASE_NAME" \
            "$CHART_PATH" \
            --namespace "$NAMESPACE" \
            --create-namespace \
            --timeout "$HELM_TIMEOUT" \
            --wait \
            $EXTRA_ARGS \
            "$@"
    fi
    
    echo_info "Operator installed ✓"
}

# Verify installation
verify_installation() {
    echo_info "Verifying installation..."
    
    # Wait for deployment
    kubectl wait --for=condition=available \
        --timeout=300s \
        deployment/"$RELEASE_NAME" \
        -n "$NAMESPACE" || echo_warn "Deployment may not be ready yet"
    
    # Check pods
    echo_info "Checking pods..."
    kubectl get pods -n "$NAMESPACE" -l "control-plane=controller-manager"
    
    # Check SCC if using custom SCC
    if [ "$USE_CUSTOM_SCC" = "true" ]; then
        echo_info "Verifying SCC binding..."
        if command -v oc &> /dev/null; then
            oc describe scc unified-replication-operator-scc | grep "Users:" || echo_warn "Could not verify SCC binding"
        fi
    fi
    
    echo_info "Installation verified ✓"
}

# Print status
print_status() {
    echo ""
    echo_info "========================================="
    echo_info "OpenShift Installation Complete!"
    echo_info "========================================="
    echo ""
    echo_info "Namespace: $NAMESPACE"
    echo_info "Release: $RELEASE_NAME"
    if [ "$USE_CUSTOM_SCC" = "true" ]; then
        echo_info "SCC: unified-replication-operator-scc"
    else
        echo_info "Security: OpenShift-compatible (no custom SCC)"
    fi
    echo ""
    echo_info "View pods:"
    echo "  kubectl get pods -n $NAMESPACE"
    echo ""
    echo_info "View logs:"
    echo "  kubectl logs -n $NAMESPACE -l control-plane=controller-manager -f"
    echo ""
    echo_info "Check SCC (if using custom SCC):"
    echo "  oc describe scc unified-replication-operator-scc"
    echo ""
}

# Main installation flow
main() {
    echo_info "=== OpenShift Installation for Unified Replication Operator ==="
    echo ""
    
    # Check if OpenShift
    IS_OPENSHIFT=false
    if check_openshift; then
        IS_OPENSHIFT=true
    fi
    
    # Create namespace
    echo_info "Creating namespace: $NAMESPACE"
    kubectl create namespace "$NAMESPACE" 2>/dev/null || echo_warn "Namespace already exists"
    
    # Install CRDs
    echo_info "Installing CRDs..."
    kubectl apply -f "$PROJECT_ROOT/config/crd/bases" || echo_warn "CRDs may already exist"
    
    # If OpenShift and using custom SCC, check for conflicts
    if [ "$IS_OPENSHIFT" = true ] && [ "$USE_CUSTOM_SCC" = "true" ]; then
        check_scc_conflicts
    fi
    
    # Install operator (Helm will create SCC if openshift.enabled=true)
    install_operator "$@"
    
    # Verify SCC after installation
    if [ "$IS_OPENSHIFT" = true ] && [ "$USE_CUSTOM_SCC" = "true" ]; then
        verify_scc
    fi
    
    # Verify
    verify_installation
    
    # Print status
    print_status
}

# Show help
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    echo "OpenShift Installation Script for Unified Replication Operator"
    echo ""
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --help, -h              Show this help message"
    echo "  --no-scc                Don't create custom SCC, use OpenShift-compatible settings"
    echo "  --namespace NAMESPACE   Override namespace (default: unified-replication-system)"
    echo "  --release-name NAME     Override release name (default: unified-replication-operator)"
    echo ""
    echo "Environment Variables:"
    echo "  NAMESPACE               Kubernetes namespace"
    echo "  RELEASE_NAME            Helm release name"
    echo "  HELM_TIMEOUT            Helm timeout (default: 10m)"
    echo "  USE_CUSTOM_SCC          Create custom SCC (default: true)"
    echo ""
    echo "Examples:"
    echo "  $0                      # Standard installation with custom SCC"
    echo "  $0 --no-scc             # Use OpenShift-assigned UIDs"
    echo "  USE_CUSTOM_SCC=false $0 # Same as --no-scc"
    echo ""
    exit 0
fi

# Handle --no-scc flag
if [ "$1" = "--no-scc" ]; then
    USE_CUSTOM_SCC=false
    shift
fi

# Run main
main "$@"

