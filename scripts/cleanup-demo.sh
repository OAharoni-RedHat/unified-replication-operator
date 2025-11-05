#!/bin/bash
# Cleanup script for Unified Replication Operator Demo
# Removes all demo resources and optionally the operator itself
# Usage: ./scripts/cleanup-demo.sh [--operator]

set -e

NAMESPACE="${NAMESPACE:-unified-replication-system}"
RELEASE_NAME="${RELEASE_NAME:-unified-replication-operator}"
REMOVE_OPERATOR="${1:-}"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

echo_step() {
    echo -e "${BLUE}🔧${NC} $1"
}

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Unified Replication Operator - Demo Cleanup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [ "$REMOVE_OPERATOR" = "--operator" ]; then
    echo "This will remove:"
    echo "  • All VolumeReplication resources (v1alpha2)"
    echo "  • All VolumeGroupReplication resources"
    echo "  • All VolumeReplicationClass resources"
    echo "  • All VolumeGroupReplicationClass resources"
    echo "  • Backend-specific resources (TridentMirrorRelationship, etc.)"
    echo "  • Helm release: $RELEASE_NAME"
    echo "  • Namespace: $NAMESPACE"
    echo "  • CRDs, RBAC, webhooks"
else
    echo "This will remove:"
    echo "  • All VolumeReplication resources (v1alpha2)"
    echo "  • All VolumeGroupReplication resources"
    echo "  • All VolumeReplicationClass resources"
    echo "  • All VolumeGroupReplicationClass resources"
    echo "  • Backend-specific resources (TridentMirrorRelationship, etc.)"
    echo ""
    echo "Operator will remain installed."
fi

echo ""

# Prompt for confirmation
read -p "Are you sure you want to continue? (yes/no): " CONFIRM
if [ "$CONFIRM" != "yes" ]; then
    echo "Cleanup cancelled."
    exit 0
fi

echo ""

# Step 1: Delete all VolumeReplication resources
echo_step "Step 1: Deleting all VolumeReplication resources..."
if kubectl get vr --all-namespaces &>/dev/null 2>&1; then
    VR_COUNT=$(kubectl get vr --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$VR_COUNT" -gt 0 ]; then
        kubectl delete vr --all --all-namespaces --timeout=60s || echo_warn "Some VolumeReplication resources may still be deleting"
        echo_info "VolumeReplication resources deleted"
    else
        echo_info "No VolumeReplication resources found"
    fi
else
    echo_info "VolumeReplication CRD not found or no resources exist"
fi

# Step 2: Delete all VolumeGroupReplication resources
echo_step "Step 2: Deleting all VolumeGroupReplication resources..."
if kubectl get vgr --all-namespaces &>/dev/null 2>&1; then
    VGR_COUNT=$(kubectl get vgr --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$VGR_COUNT" -gt 0 ]; then
        kubectl delete vgr --all --all-namespaces --timeout=60s || echo_warn "Some VolumeGroupReplication resources may still be deleting"
        echo_info "VolumeGroupReplication resources deleted"
    else
        echo_info "No VolumeGroupReplication resources found"
    fi
else
    echo_info "VolumeGroupReplication CRD not found or no resources exist"
fi

# Step 3: Delete all VolumeReplicationClass resources
echo_step "Step 3: Deleting all VolumeReplicationClass resources..."
if kubectl get vrc &>/dev/null 2>&1; then
    VRC_COUNT=$(kubectl get vrc --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$VRC_COUNT" -gt 0 ]; then
        kubectl delete vrc --all --timeout=60s || echo_warn "Some VolumeReplicationClass resources may still be deleting"
        echo_info "VolumeReplicationClass resources deleted"
    else
        echo_info "No VolumeReplicationClass resources found"
    fi
else
    echo_info "VolumeReplicationClass CRD not found or no resources exist"
fi

# Step 4: Delete all VolumeGroupReplicationClass resources
echo_step "Step 4: Deleting all VolumeGroupReplicationClass resources..."
if kubectl get vgrc &>/dev/null 2>&1; then
    VGRC_COUNT=$(kubectl get vgrc --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$VGRC_COUNT" -gt 0 ]; then
        kubectl delete vgrc --all --timeout=60s || echo_warn "Some VolumeGroupReplicationClass resources may still be deleting"
        echo_info "VolumeGroupReplicationClass resources deleted"
    else
        echo_info "No VolumeGroupReplicationClass resources found"
    fi
else
    echo_info "VolumeGroupReplicationClass CRD not found or no resources exist"
fi

# Step 5: Delete backend-specific resources
echo_step "Step 5: Cleaning up backend-specific resources..."

# TridentMirrorRelationship
if kubectl get crd tridentmirrorrelationships.trident.netapp.io &>/dev/null 2>&1; then
    if kubectl get tridentmirrorrelationship --all-namespaces &>/dev/null 2>&1; then
        TMR_COUNT=$(kubectl get tridentmirrorrelationship --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")
        if [ "$TMR_COUNT" -gt 0 ]; then
            kubectl delete tridentmirrorrelationship --all --all-namespaces --timeout=60s || echo_warn "Some TridentMirrorRelationship resources may still be deleting"
            echo_info "TridentMirrorRelationship resources deleted"
        else
            echo_info "No TridentMirrorRelationship resources found"
        fi
    else
        echo_info "No TridentMirrorRelationship resources found"
    fi
fi

# Ceph VolumeReplication (from kubernetes-csi-addons)
if kubectl get crd volumereplications.replication.storage.openshift.io &>/dev/null 2>&1; then
    if kubectl get volumereplication.replication.storage.openshift.io --all-namespaces &>/dev/null 2>&1; then
        CEPH_VR_COUNT=$(kubectl get volumereplication.replication.storage.openshift.io --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")
        if [ "$CEPH_VR_COUNT" -gt 0 ]; then
            kubectl delete volumereplication.replication.storage.openshift.io --all --all-namespaces --timeout=60s || echo_warn "Some Ceph VolumeReplication resources may still be deleting"
            echo_info "Ceph VolumeReplication resources deleted"
        else
            echo_info "No Ceph VolumeReplication resources found"
        fi
    else
        echo_info "No Ceph VolumeReplication resources found"
    fi
fi

# Wait for finalizers to complete
echo_info "Waiting for finalizers to complete cleanup..."
sleep 5

# Step 6: Remove operator (if requested)
if [ "$REMOVE_OPERATOR" = "--operator" ]; then
    echo_step "Step 6: Uninstalling operator..."
    
    # Uninstall Helm release
    if helm list -n "$NAMESPACE" 2>/dev/null | grep -q "$RELEASE_NAME"; then
        helm uninstall "$RELEASE_NAME" -n "$NAMESPACE" --wait || echo_warn "Helm uninstall had issues"
        echo_info "Helm release uninstalled"
    else
        echo_info "Helm release not found (already uninstalled)"
    fi
    
    # Delete webhook configurations
    echo_step "Step 7: Deleting webhook configurations..."
    kubectl delete validatingwebhookconfiguration "${RELEASE_NAME}-validating-webhook" 2>/dev/null || echo_info "Webhook config not found"
    kubectl delete mutatingwebhookconfiguration "${RELEASE_NAME}-mutating-webhook" 2>/dev/null || echo_info "Mutating webhook not found"
    
    # Delete CRDs
    echo_step "Step 8: Deleting Custom Resource Definitions..."
    
    # Function to safely delete CRD with timeout and finalizer handling
    delete_crd_safe() {
        local crd_name=$1
        local description=$2
        
        # Check if CRD exists first
        if ! kubectl get crd "$crd_name" &>/dev/null 2>&1; then
            echo_info "$description CRD not found (already deleted or never installed)"
            return 0
        fi
        
        # Check if CRD has finalizers that might block deletion
        local has_finalizers=$(kubectl get crd "$crd_name" -o jsonpath='{.metadata.finalizers[*]}' 2>/dev/null || echo "")
        if [ -n "$has_finalizers" ]; then
            echo_warn "$description CRD has finalizers, removing them first..."
            kubectl patch crd "$crd_name" -p '{"metadata":{"finalizers":[]}}' --type=merge 2>/dev/null || true
            sleep 2
        fi
        
        # Try to delete with timeout (use timeout command if available, otherwise rely on kubectl timeout)
        if command -v timeout >/dev/null 2>&1; then
            # Use timeout command for extra safety
            if timeout 30s kubectl delete crd "$crd_name" --timeout=25s 2>/dev/null; then
                echo_info "$description CRD deleted"
            else
                echo_warn "$description CRD deletion timed out or failed (may still be deleting)"
            fi
        else
            # Fallback: rely on kubectl's timeout flag only
            if kubectl delete crd "$crd_name" --timeout=30s 2>/dev/null; then
                echo_info "$description CRD deleted"
            else
                echo_warn "$description CRD deletion timed out or failed (may still be deleting)"
            fi
        fi
    }
    
    delete_crd_safe "volumereplications.replication.unified.io" "VolumeReplication"
    delete_crd_safe "volumereplicationclasses.replication.unified.io" "VolumeReplicationClass"
    delete_crd_safe "volumegroupreplications.replication.unified.io" "VolumeGroupReplication"
    delete_crd_safe "volumegroupreplicationclasses.replication.unified.io" "VolumeGroupReplicationClass"
    
    # Delete OpenShift SCC if exists
    echo_step "Step 9: Deleting OpenShift SCC (if exists)..."
    if kubectl api-resources | grep -q securitycontextconstraints; then
        kubectl delete scc unified-replication-operator-scc 2>/dev/null || echo_info "SCC not found"
    else
        echo_info "Not an OpenShift cluster, skipping SCC"
    fi
    
    # Delete namespace
    echo_step "Step 10: Deleting namespace..."
    if kubectl get namespace "$NAMESPACE" &>/dev/null; then
        kubectl delete namespace "$NAMESPACE" --timeout=120s || echo_warn "Namespace deletion timed out"
        echo_info "Namespace deleted"
    else
        echo_info "Namespace not found"
    fi
    
    # Clean up cluster-level RBAC
    echo_step "Step 11: Cleaning up cluster-level RBAC..."
    kubectl delete clusterrole "${RELEASE_NAME}-manager" 2>/dev/null || echo_info "ClusterRole not found"
    kubectl delete clusterrolebinding "${RELEASE_NAME}-manager" 2>/dev/null || echo_info "ClusterRoleBinding not found"
fi

# Verification
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Verification"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

ALL_CLEAN=true

# Check VolumeReplication resources
if kubectl get vr --all-namespaces &>/dev/null 2>&1; then
    VR_REMAINING=$(kubectl get vr --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$VR_REMAINING" -gt 0 ]; then
        echo_warn "VolumeReplication resources still exist: $VR_REMAINING"
        ALL_CLEAN=false
    else
        echo_info "VolumeReplication: Clean"
    fi
else
    echo_info "VolumeReplication: Clean"
fi

# Check VolumeReplicationClass resources
if kubectl get vrc &>/dev/null 2>&1; then
    VRC_REMAINING=$(kubectl get vrc --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$VRC_REMAINING" -gt 0 ]; then
        echo_warn "VolumeReplicationClass resources still exist: $VRC_REMAINING"
        ALL_CLEAN=false
    else
        echo_info "VolumeReplicationClass: Clean"
    fi
else
    echo_info "VolumeReplicationClass: Clean"
fi

# Check backend resources
if kubectl get tridentmirrorrelationship --all-namespaces &>/dev/null 2>&1; then
    TMR_REMAINING=$(kubectl get tridentmirrorrelationship --all-namespaces --no-headers 2>/dev/null | wc -l || echo "0")
    if [ "$TMR_REMAINING" -gt 0 ]; then
        echo_warn "TridentMirrorRelationship resources still exist: $TMR_REMAINING"
        ALL_CLEAN=false
    else
        echo_info "TridentMirrorRelationship: Clean"
    fi
else
    echo_info "TridentMirrorRelationship: Clean"
fi

if [ "$REMOVE_OPERATOR" = "--operator" ]; then
    # Check namespace
    if kubectl get namespace "$NAMESPACE" &>/dev/null; then
        echo_warn "Namespace still exists (may be terminating)"
        ALL_CLEAN=false
    else
        echo_info "Namespace: Deleted"
    fi
    
    # Check CRDs
    if kubectl get crd | grep -q "replication.unified.io"; then
        echo_warn "CRDs still exist"
        ALL_CLEAN=false
    else
        echo_info "CRDs: Deleted"
    fi
    
    # Check Helm
    if helm list -n "$NAMESPACE" 2>/dev/null | grep -q "$RELEASE_NAME"; then
        echo_warn "Helm release still exists"
        ALL_CLEAN=false
    else
        echo_info "Helm release: Uninstalled"
    fi
fi

echo ""
if [ "$ALL_CLEAN" = true ]; then
    echo_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    if [ "$REMOVE_OPERATOR" = "--operator" ]; then
        echo_info "✅ COMPLETE CLEANUP - All resources and operator removed!"
    else
        echo_info "✅ DEMO CLEANUP COMPLETE - All demo resources removed!"
    fi
    echo_info "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
else
    echo_warn "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo_warn "⚠️  Some resources may still be terminating"
    echo_warn "Wait a moment and re-run this script to verify"
    echo_warn "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi

echo ""
if [ "$REMOVE_OPERATOR" = "--operator" ]; then
    echo "Your cluster is clean! The operator and all resources have been removed."
    echo ""
    echo "To reinstall:"
    echo "  REGISTRY=your-registry VERSION=2.0.0-beta ./scripts/build-and-push.sh"
else
    echo "Demo resources cleaned up! The operator remains installed."
    echo ""
    echo "To restart the demo:"
    echo "  cd demo && ./run-demo.sh"
fi
echo ""

