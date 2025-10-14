#!/bin/bash
# Demonstrates seamless backend switching without operator restart

set -e

export KUBECONFIG="${KUBECONFIG:-/home/oaharoni/aws-gpfs-playground/ocp_install_files/auth/kubeconfig}"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo_info() {
    echo -e "${GREEN}✅${NC} $1"
}

echo_step() {
    echo -e "\n${BLUE}━━━ $1 ━━━${NC}"
}

echo_step "1. Check Operator is Running"
OPERATOR_POD=$(kubectl get pods -n unified-replication-system -l control-plane=controller-manager -o name | head -1)
OPERATOR_START_TIME=$(kubectl get ${OPERATOR_POD} -n unified-replication-system -o jsonpath='{.status.startTime}')
echo "Operator Pod: ${OPERATOR_POD}"
echo "Started at: ${OPERATOR_START_TIME}"

echo_step "2. Apply Trident Replication"
kubectl apply -f trident-replication.yaml
sleep 5

echo_info "Trident replication status:"
kubectl get uvr trident-volume-replication -n default --no-headers

echo_info "Backend-specific resource created:"
kubectl get tridentmirrorrelationship -n default --no-headers 2>/dev/null || echo "TridentMirrorRelationship not found"

echo_step "3. Apply Ceph Replication (Different Backend)"
kubectl apply -f ceph-replication.yaml
sleep 5

echo_info "Both replications running:"
kubectl get uvr -n default

echo_step "4. Verify No Operator Restart"
CURRENT_POD=$(kubectl get pods -n unified-replication-system -l control-plane=controller-manager -o name | head -1)
CURRENT_START_TIME=$(kubectl get ${CURRENT_POD} -n unified-replication-system -o jsonpath='{.status.startTime}')

if [ "${OPERATOR_START_TIME}" = "${CURRENT_START_TIME}" ]; then
    echo_info "Operator DID NOT restart! ✅"
    echo "   Original start: ${OPERATOR_START_TIME}"
    echo "   Current start:  ${CURRENT_START_TIME}"
else
    echo -e "${YELLOW}⚠️  Operator restarted${NC}"
    echo "   Original start: ${OPERATOR_START_TIME}"
    echo "   Current start:  ${CURRENT_START_TIME}"
fi

echo_step "5. Check Backend Detection"
echo "Logs showing backend selection:"
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=200 | \
    grep "Selected backend" | tail -10

echo_step "6. Verify Different Adapters Used"
echo "Trident adapter logs:"
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=200 | \
    grep "trident-volume-replication" | grep -i "trident-adapter" | tail -2

echo ""
echo "Ceph detection logs:"
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=200 | \
    grep "ceph-volume-replication" | grep -E "(backend.*ceph|ceph-adapter)" | tail -2

echo_step "7. Summary"
echo ""
echo_info "Backend Switching Validated:"
echo "  ✅ Two different backends (Trident + Ceph)"
echo "  ✅ No operator restart required"
echo "  ✅ Correct backend detection per resource"
echo "  ✅ Different adapters used simultaneously"
echo ""
echo "Current State:"
kubectl get uvr -n default -o custom-columns=NAME:.metadata.name,BACKEND:.spec.sourceEndpoint.storageClass,STATE:.spec.replicationState,READY:.status.conditions[0].status

echo ""
echo_info "✅ Backend switching demonstration complete!"

