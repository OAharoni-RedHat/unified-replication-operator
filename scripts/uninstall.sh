#!/bin/bash
# Uninstallation script for Unified Replication Operator

set -e

# Default values
NAMESPACE="${NAMESPACE:-unified-replication-system}"
RELEASE_NAME="${RELEASE_NAME:-unified-replication-operator}"
DELETE_NAMESPACE="${DELETE_NAMESPACE:-false}"
DELETE_CRDS="${DELETE_CRDS:-false}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Cleanup resources
cleanup_resources() {
    echo_info "Cleaning up UnifiedVolumeReplication resources..."
    
    # Delete all UVR resources
    if kubectl get unifiedvolumereplications -A &> /dev/null; then
        kubectl delete unifiedvolumereplications --all -A --wait=true --timeout=300s || \
            echo_warn "Some resources may still be deleting"
    fi
    
    echo_info "Resources cleaned up ✓"
}

# Uninstall operator
uninstall_operator() {
    echo_info "Uninstalling operator via Helm..."
    
    if helm list -n "$NAMESPACE" | grep -q "$RELEASE_NAME"; then
        helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" --wait
        echo_info "Operator uninstalled ✓"
    else
        echo_warn "Release $RELEASE_NAME not found in namespace $NAMESPACE"
    fi
}

# Delete webhook configuration
delete_webhook() {
    echo_info "Deleting webhook configuration..."
    
    WEBHOOK_NAME="${RELEASE_NAME}-validating-webhook"
    if kubectl get validatingwebhookconfiguration "$WEBHOOK_NAME" &> /dev/null; then
        kubectl delete validatingwebhookconfiguration "$WEBHOOK_NAME"
        echo_info "Webhook deleted ✓"
    fi
}

# Delete CRDs
delete_crds() {
    if [ "$DELETE_CRDS" = "true" ]; then
        echo_warn "Deleting CRDs (this will delete ALL UnifiedVolumeReplication resources)..."
        
        if kubectl get crd unifiedvolumereplications.replication.unified.io &> /dev/null; then
            kubectl delete crd unifiedvolumereplications.replication.unified.io
            echo_info "CRDs deleted ✓"
        fi
    else
        echo_info "Skipping CRD deletion (set DELETE_CRDS=true to delete)"
    fi
}

# Delete namespace
delete_namespace() {
    if [ "$DELETE_NAMESPACE" = "true" ]; then
        echo_warn "Deleting namespace: $NAMESPACE"
        
        if kubectl get namespace "$NAMESPACE" &> /dev/null; then
            kubectl delete namespace "$NAMESPACE" --wait=true --timeout=300s
            echo_info "Namespace deleted ✓"
        fi
    else
        echo_info "Skipping namespace deletion (set DELETE_NAMESPACE=true to delete)"
    fi
}

# Main uninstallation flow
main() {
    echo_info "=== Unified Replication Operator Uninstallation ==="
    echo ""
    echo_info "Namespace: $NAMESPACE"
    echo_info "Release: $RELEASE_NAME"
    echo_info "Delete CRDs: $DELETE_CRDS"
    echo_info "Delete Namespace: $DELETE_NAMESPACE"
    echo ""
    
    # Confirm
    read -p "Continue with uninstallation? (yes/no): " -r
    echo
    if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
        echo_info "Uninstallation cancelled"
        exit 0
    fi
    
    # Cleanup resources
    cleanup_resources
    
    # Delete webhook
    delete_webhook
    
    # Uninstall operator
    uninstall_operator
    
    # Delete CRDs (optional)
    delete_crds
    
    # Delete namespace (optional)
    delete_namespace
    
    echo ""
    echo_info "========================================="
    echo_info "Uninstallation Complete!"
    echo_info "========================================="
}

# Run main
main

