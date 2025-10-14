#!/bin/bash
# Validation script for UnifiedVolumeReplication resources

set -e

KUBECONFIG="${KUBECONFIG:-}"
RESOURCE_NAME="${1:-}"
NAMESPACE="${2:-default}"

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

echo_section() {
    echo -e "\n${BLUE}━━━ $1 ━━━${NC}"
}

# Show help
if [ "$RESOURCE_NAME" = "" ] || [ "$RESOURCE_NAME" = "--help" ] || [ "$RESOURCE_NAME" = "-h" ]; then
    cat <<EOF
UnifiedVolumeReplication Validation Script

Usage: $0 <resource-name> [namespace]

Arguments:
  resource-name   Name of the UnifiedVolumeReplication resource
  namespace       Namespace (default: default)

Examples:
  $0 trident-volume-replication
  $0 my-replication production

Environment:
  KUBECONFIG      Path to kubeconfig file

EOF
    exit 0
fi

echo_section "Validating UnifiedVolumeReplication: $RESOURCE_NAME"

# 1. Check if resource exists
echo_section "1. Resource Existence"
if kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" &>/dev/null; then
    echo_info "Resource exists"
else
    echo_error "Resource not found: $RESOURCE_NAME in namespace $NAMESPACE"
    exit 1
fi

# 2. Check Ready status
echo_section "2. Ready Status"
READY_STATUS=$(kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
READY_MESSAGE=$(kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.status.conditions[?(@.type=="Ready")].message}' 2>/dev/null)
READY_REASON=$(kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.status.conditions[?(@.type=="Ready")].reason}' 2>/dev/null)

if [ "$READY_STATUS" = "True" ]; then
    echo_info "Ready: True"
    echo "   Reason: $READY_REASON"
    echo "   Message: $READY_MESSAGE"
elif [ "$READY_STATUS" = "False" ]; then
    echo_error "Ready: False"
    echo "   Reason: $READY_REASON"
    echo "   Message: $READY_MESSAGE"
else
    echo_warn "Ready status: Unknown or not set"
fi

# 3. Check spec details
echo_section "3. Specification"
STATE=$(kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.replicationState}')
MODE=$(kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.replicationMode}')
SOURCE_SC=$(kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.sourceEndpoint.storageClass}')
DEST_SC=$(kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.destinationEndpoint.storageClass}')
SOURCE_PVC=$(kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.volumeMapping.source.pvcName}')

echo "   Replication State: $STATE"
echo "   Replication Mode:  $MODE"
echo "   Source Storage:    $SOURCE_SC"
echo "   Dest Storage:      $DEST_SC"
echo "   Source PVC:        $SOURCE_PVC"

# 4. Detect backend
echo_section "4. Backend Detection"
BACKEND="unknown"
if echo "$SOURCE_SC" | grep -qi "ceph\|rbd"; then
    BACKEND="ceph"
    BACKEND_CRD="volumereplication"
elif echo "$SOURCE_SC" | grep -qi "trident\|netapp"; then
    BACKEND="trident"
    BACKEND_CRD="tridentmirrorrelationship"
elif echo "$SOURCE_SC" | grep -qi "powerstore\|dell"; then
    BACKEND="powerstore"
    BACKEND_CRD="dellcsireplicationgroup"
fi

echo "   Detected Backend: $BACKEND"
echo "   Expected CRD: $BACKEND_CRD"

# Check if extensions hint is set
EXTENSION_HINT=$(kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.extensions}' 2>/dev/null)
if [ -n "$EXTENSION_HINT" ]; then
    echo "   Extension Hints: $EXTENSION_HINT"
fi

# 5. Check for backend-specific resources
echo_section "5. Backend-Specific Resources"

case $BACKEND in
    "ceph")
        if kubectl get volumereplication "$RESOURCE_NAME" -n "$NAMESPACE" &>/dev/null; then
            echo_info "VolumeReplication (Ceph) resource exists"
            kubectl get volumereplication "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.replicationState}: {.status.state}' 2>/dev/null || echo ""
        else
            echo_warn "VolumeReplication (Ceph) resource not found"
            echo "   This might be expected if using mock adapters"
        fi
        ;;
    "trident")
        if kubectl get tridentmirrorrelationship "$RESOURCE_NAME" -n "$NAMESPACE" &>/dev/null; then
            echo_info "TridentMirrorRelationship resource exists"
            kubectl get tridentmirrorrelationship "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.spec.state}' 2>/dev/null || echo ""
        else
            echo_warn "TridentMirrorRelationship resource not found"
            echo "   This might be expected if using mock adapters"
        fi
        ;;
    "powerstore")
        if kubectl get dellcsireplicationgroup "$RESOURCE_NAME" -n "$NAMESPACE" &>/dev/null; then
            echo_info "DellCSIReplicationGroup resource exists"
        else
            echo_warn "DellCSIReplicationGroup resource not found"
            echo "   This might be expected if using mock adapters"
        fi
        ;;
esac

# 6. Check finalizer
echo_section "6. Finalizer"
FINALIZER=$(kubectl get uvr "$RESOURCE_NAME" -n "$NAMESPACE" -o jsonpath='{.metadata.finalizers[0]}' 2>/dev/null)
if [ -n "$FINALIZER" ]; then
    echo_info "Finalizer present: $FINALIZER"
    echo "   (Ensures cleanup on deletion)"
else
    echo_warn "No finalizer found"
fi

# 7. Check events
echo_section "7. Recent Events"
kubectl get events -n "$NAMESPACE" --field-selector involvedObject.name="$RESOURCE_NAME" --sort-by='.lastTimestamp' | tail -10

# 8. Check operator logs
echo_section "8. Operator Reconciliation Logs"
echo "Recent reconciliation logs for this resource:"
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=50 | \
    grep "$RESOURCE_NAME" | tail -10

# 9. Summary
echo_section "Validation Summary"

ERRORS=0
WARNINGS=0

if [ "$READY_STATUS" != "True" ]; then
    ((ERRORS++))
    echo_error "Resource is not Ready"
else
    echo_info "Resource is Ready"
fi

if [ -z "$FINALIZER" ]; then
    ((WARNINGS++))
    echo_warn "Finalizer not set (cleanup may not work)"
fi

# Check if backend resource exists (skip for mock)
if [ "$BACKEND" != "unknown" ]; then
    echo ""
    echo "Backend: $BACKEND"
    echo "Looking for backend-specific resources..."
    
    # Note about mock adapters
    kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=100 | \
        grep -q "mock-${BACKEND}-adapter" && \
        echo_warn "Using MOCK adapter - backend CRD won't be created (simulation only)"
fi

echo ""
if [ $ERRORS -eq 0 ]; then
    echo_info "✅ Validation PASSED - Replication is working correctly!"
else
    echo_error "❌ Validation FAILED - $ERRORS error(s), $WARNINGS warning(s)"
    exit 1
fi

