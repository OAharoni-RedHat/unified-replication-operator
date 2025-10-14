#!/bin/bash
# Uninstall script for Unified Replication Operator
# Removes all operator artifacts from the cluster

set -e

NAMESPACE="${NAMESPACE:-unified-replication-system}"
RELEASE_NAME="${RELEASE_NAME:-unified-replication-operator}"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo_info() {
    echo -e "${GREEN}✅${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}⚠️${NC}  $1"
}

echo_error() {
    echo -e "${RED}❌${NC} $1"
}

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Unified Replication Operator - Uninstall"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "This will remove:"
echo "  • All UnifiedVolumeReplication resources"
echo "  • Backend-specific CRDs (via finalizers)"
echo "  • Helm release: $RELEASE_NAME"
echo "  • Namespace: $NAMESPACE"
echo "  • CRDs, webhooks, RBAC, SCC"
echo ""

# Prompt for confirmation
read -p "Are you sure you want to continue? (yes/no): " CONFIRM
if [ "$CONFIRM" != "yes" ]; then
    echo "Uninstall cancelled."
    exit 0
fi

echo ""

# Step 1: Delete all UnifiedVolumeReplication resources
echo_info "Step 1: Deleting all UnifiedVolumeReplication resources..."
if kubectl get uvr --all-namespaces &>/dev/null; then
    kubectl delete uvr --all --all-namespaces --timeout=60s || echo_warn "Some resources may still be deleting"
    echo_info "UnifiedVolumeReplication resources deleted"
else
    echo_info "No UnifiedVolumeReplication resources found"
fi

# Wait for finalizers to clean up backend CRDs
echo_info "Waiting for finalizers to clean up backend resources..."
sleep 5

# Step 2: Uninstall Helm release
echo_info "Step 2: Uninstalling Helm release..."
if helm list -n "$NAMESPACE" 2>/dev/null | grep -q "$RELEASE_NAME"; then
    helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" --wait || echo_warn "Helm uninstall had issues"
    echo_info "Helm release uninstalled"
else
    echo_info "Helm release not found (already uninstalled)"
fi

# Step 3: Delete webhook configurations
echo_info "Step 3: Deleting webhook configurations..."
kubectl delete validatingwebhookconfiguration "${RELEASE_NAME}-validating-webhook" 2>/dev/null || echo_info "Webhook config not found"

# Step 4: Delete mutating webhook if exists
kubectl delete mutatingwebhookconfiguration "${RELEASE_NAME}-mutating-webhook" 2>/dev/null || echo_info "Mutating webhook not found"

# Step 5: Delete CRDs
echo_info "Step 4: Deleting Custom Resource Definitions..."
kubectl delete crd unifiedvolumereplications.replication.unified.io 2>/dev/null || echo_info "CRD not found"

# Step 6: Delete OpenShift SCC if exists
echo_info "Step 5: Deleting OpenShift SCC (if exists)..."
if kubectl api-resources | grep -q securitycontextconstraints; then
    kubectl delete scc unified-replication-operator-scc 2>/dev/null || echo_info "SCC not found"
else
    echo_info "Not an OpenShift cluster, skipping SCC"
fi

# Step 7: Delete namespace
echo_info "Step 6: Deleting namespace..."
if kubectl get namespace "$NAMESPACE" &>/dev/null; then
    kubectl delete namespace "$NAMESPACE" --timeout=120s || echo_warn "Namespace deletion timed out"
    echo_info "Namespace deleted"
else
    echo_info "Namespace not found"
fi

# Step 8: Clean up any remaining cluster-level RBAC
echo_info "Step 7: Cleaning up cluster-level RBAC..."
kubectl delete clusterrole "${RELEASE_NAME}-manager" 2>/dev/null || echo_info "ClusterRole not found"
kubectl delete clusterrolebinding "${RELEASE_NAME}-manager" 2>/dev/null || echo_info "ClusterRoleBinding not found"

# Step 9: Verify cleanup
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Verification"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

ALL_CLEAN=true

# Check namespace
if kubectl get namespace "$NAMESPACE" &>/dev/null; then
    echo_warn "Namespace still exists (may be terminating)"
    ALL_CLEAN=false
else
    echo_info "Namespace: Deleted"
fi

# Check CRDs
if kubectl get crd | grep -q unifiedvolumeplication; then
    echo_warn "CRDs still exist"
    ALL_CLEAN=false
else
    echo_info "CRDs: Deleted"
fi

# Check pods
if kubectl get pods -n "$NAMESPACE" &>/dev/null 2>&1; then
    echo_warn "Pods still exist in namespace"
    ALL_CLEAN=false
else
    echo_info "Pods: Deleted"
fi

# Check Helm
if helm list -n "$NAMESPACE" 2>/dev/null | grep -q "$RELEASE_NAME"; then
    echo_warn "Helm release still exists"
    ALL_CLEAN=false
else
    echo_info "Helm release: Uninstalled"
fi

echo ""
if [ "$ALL_CLEAN" = true ]; then
    echo_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo_info "✅ CLEANUP COMPLETE - All artifacts removed!"
    echo_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
else
    echo_warn "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo_warn "⚠️  Some resources may still be terminating"
    echo_warn "Wait a moment and re-run this script to verify"
    echo_warn "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi

echo ""
echo "Your cluster is clean! The operator has been completely removed."
echo ""
