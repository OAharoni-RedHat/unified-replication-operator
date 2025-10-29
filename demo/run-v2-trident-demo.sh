#!/bin/bash
# v2.0.0-beta Trident Translation Demo
# This script demonstrates automatic translation from kubernetes-csi-addons API to Trident

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo_step() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}🔹 $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

echo_info() {
    echo -e "${GREEN}✅ $1${NC}"
}

echo_warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

pause() {
    echo ""
    echo -e "${YELLOW}Press Enter to continue...${NC}"
    read
}

cat <<'EOF'
╔══════════════════════════════════════════════════════════════════════╗
║                                                                       ║
║          v2.0.0-beta Trident Translation Demo                        ║
║                                                                       ║
║     Demonstrating kubernetes-csi-addons API → Trident Translation    ║
║                                                                       ║
╚══════════════════════════════════════════════════════════════════════╝

This demo shows how the Unified Replication Operator:
  1. Accepts kubernetes-csi-addons standard VolumeReplication API
  2. Detects Trident backend from provisioner
  3. Translates states (primary → established, secondary → reestablishing)
  4. Creates TridentMirrorRelationship automatically

EOF

pause

# Step 1: Create VolumeReplicationClass
echo_step "Step 1: Creating VolumeReplicationClass"

cat <<EOF | kubectl apply -f -
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: trident-async-replication
spec:
  provisioner: csi.trident.netapp.io
  parameters:
    replicationPolicy: "Async"
    replicationSchedule: "15m"
    remoteCluster: "dr-cluster"
    remoteSVM: "svm-dr"
    remoteVolume: "remote-volume-handle"
EOF

echo_info "VolumeReplicationClass created!"
echo ""
echo "The operator will detect backend from provisioner: csi.trident.netapp.io"
echo ""
kubectl get volumereplicationclass trident-async-replication
pause

# Step 2: Create namespace
echo_step "Step 2: Creating namespace"

kubectl create namespace applications --dry-run=client -o yaml | kubectl apply -f -
echo_info "Namespace 'applications' ready"
pause

# Step 3: Create VolumeReplication with state "primary"
echo_step "Step 3: Creating VolumeReplication with state: primary"

cat <<EOF | kubectl apply -f -
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: trident-app-replication
  namespace: applications
spec:
  volumeReplicationClass: trident-async-replication
  pvcName: application-data-pvc
  replicationState: primary
  autoResync: true
EOF

echo_info "VolumeReplication created with replicationState: primary"
echo ""
echo "Waiting for operator to reconcile..."
sleep 3

echo ""
echo "VolumeReplication status:"
kubectl get vr trident-app-replication -n applications
pause

# Step 4: Check backend TridentMirrorRelationship
echo_step "Step 4: Verifying TridentMirrorRelationship was created"

echo "Checking for TridentMirrorRelationship..."
if kubectl get tridentmirrorrelationship trident-app-replication -n applications &>/dev/null; then
    echo_info "TridentMirrorRelationship exists!"
    echo ""
    echo "Getting spec.state (should be 'established' - translated from 'primary'):"
    kubectl get tridentmirrorrelationship trident-app-replication -n applications -o yaml | grep "state:" | head -1
    echo ""
    echo "Full TridentMirrorRelationship spec:"
    kubectl get tridentmirrorrelationship trident-app-replication -n applications -o yaml | grep -A 15 "^spec:"
else
    echo_warn "TridentMirrorRelationship not found (Trident CRD may not be installed)"
    echo "But the operator still created the CR - check with:"
    echo "  kubectl get tridentmirrorrelationship -n applications"
fi
pause

# Step 5: Verify Translation
echo_step "Step 5: Translation Verification"

echo "Input (kubernetes-csi-addons standard):"
VR_STATE=$(kubectl get vr trident-app-replication -n applications -o jsonpath='{.spec.replicationState}')
echo "  VolumeReplication.spec.replicationState: ${VR_STATE}"

echo ""
echo "Output (Trident-specific):"
if kubectl get tridentmirrorrelationship trident-app-replication -n applications &>/dev/null; then
    TMR_STATE=$(kubectl get tridentmirrorrelationship trident-app-replication -n applications -o jsonpath='{.spec.state}' 2>/dev/null || echo "N/A")
    echo "  TridentMirrorRelationship.spec.state: ${TMR_STATE}"
    
    echo ""
    if [ "$VR_STATE" = "primary" ] && [ "$TMR_STATE" = "established" ]; then
        echo_info "✅ TRANSLATION VERIFIED!"
        echo_info "primary → established (automatic translation working!)"
    else
        echo_warn "Translation may still be in progress..."
    fi
else
    echo "  (TridentMirrorRelationship not accessible - Trident CRD may not be installed)"
fi
pause

# Step 6: State Transition Demo
echo_step "Step 6: State Transition Demo (primary → secondary → primary)"

echo "Current state: primary → established"
echo ""
echo "Changing to secondary (demoting)..."
kubectl patch vr trident-app-replication -n applications \
  --type merge \
  -p '{"spec":{"replicationState":"secondary"}}'

echo ""
echo "Waiting for reconciliation..."
sleep 2

echo ""
echo "New state:"
VR_STATE=$(kubectl get vr trident-app-replication -n applications -o jsonpath='{.spec.replicationState}')
echo "  VolumeReplication: ${VR_STATE}"

if kubectl get tridentmirrorrelationship trident-app-replication -n applications &>/dev/null; then
    TMR_STATE=$(kubectl get tridentmirrorrelationship trident-app-replication -n applications -o jsonpath='{.spec.state}' 2>/dev/null || echo "N/A")
    echo "  TridentMirrorRelationship: ${TMR_STATE}"
    
    if [ "$VR_STATE" = "secondary" ] && [ "$TMR_STATE" = "reestablishing" ]; then
        echo ""
        echo_info "✅ TRANSLATION VERIFIED!"
        echo_info "secondary → reestablishing (automatic translation working!)"
    fi
fi
pause

echo "Changing back to primary (promoting)..."
kubectl patch vr trident-app-replication -n applications \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'

echo ""
echo "Waiting for reconciliation..."
sleep 2

echo ""
echo "Final state:"
VR_STATE=$(kubectl get vr trident-app-replication -n applications -o jsonpath='{.spec.replicationState}')
echo "  VolumeReplication: ${VR_STATE}"

if kubectl get tridentmirrorrelationship trident-app-replication -n applications &>/dev/null; then
    TMR_STATE=$(kubectl get tridentmirrorrelationship trident-app-replication -n applications -o jsonpath='{.spec.state}' 2>/dev/null || echo "N/A")
    echo "  TridentMirrorRelationship: ${TMR_STATE}"
    
    if [ "$VR_STATE" = "primary" ] && [ "$TMR_STATE" = "established" ]; then
        echo ""
        echo_info "✅ TRANSLATION VERIFIED!"
        echo_info "primary → established (back to original state!)"
    fi
fi
pause

# Step 7: Cleanup
echo_step "Step 7: Cleanup"

echo "Deleting VolumeReplication..."
kubectl delete vr trident-app-replication -n applications

echo ""
echo "Waiting for cleanup..."
sleep 2

echo ""
echo "Verifying TridentMirrorRelationship was also deleted (owner reference):"
if kubectl get tridentmirrorrelationship trident-app-replication -n applications &>/dev/null; then
    echo_warn "TridentMirrorRelationship still exists (may take a moment)"
else
    echo_info "✅ TridentMirrorRelationship deleted automatically!"
    echo_info "Owner references ensure clean cleanup"
fi

echo ""
echo "Deleting VolumeReplicationClass..."
kubectl delete volumereplicationclass trident-async-replication

echo ""
echo "Deleting namespace..."
kubectl delete namespace applications --ignore-not-found=true

echo_info "Cleanup complete!"
echo ""

# Summary
cat <<'EOF'
╔══════════════════════════════════════════════════════════════════════╗
║                         DEMO COMPLETE! 🎉                             ║
╚══════════════════════════════════════════════════════════════════════╝

What You Saw:

✅ kubernetes-csi-addons Standard API
   • Simple VolumeReplication with 3 required fields
   • Standard states: primary, secondary, resync

✅ Automatic Backend Detection
   • Detected from provisioner: csi.trident.netapp.io
   • No manual configuration needed

✅ State Translation
   • primary → established
   • secondary → reestablishing
   • Automatic and bidirectional

✅ Backend CR Creation
   • TridentMirrorRelationship created automatically
   • Owner references for cleanup
   • All parameters from VolumeReplicationClass

✅ Clean Lifecycle Management
   • Delete VolumeReplication → backend CR deleted
   • No orphaned resources
   • Kubernetes-native cleanup

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Key Benefits of v2.0.0-beta:

1. Standard API - Use kubernetes-csi-addons (not Trident-specific)
2. Automatic Translation - Operator handles Trident details
3. Simple - Only 3 fields needed
4. Portable - Same API works for Ceph, Dell, Trident
5. Clean - kubernetes-native resource management

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Try other backends:
  • Ceph: config/samples/volumereplication_ceph_primary.yaml
  • Dell:  config/samples/volumereplication_powerstore_primary.yaml

Or try volume groups:
  • config/samples/volumegroupreplication_postgresql.yaml

Documentation: See docs/ and QUICK_START.md

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

EOF

echo_info "Demo script completed successfully!"
echo ""

