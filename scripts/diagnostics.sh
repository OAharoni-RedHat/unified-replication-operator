#!/bin/bash
# Diagnostic tool for Unified Replication Operator

set -e

NAMESPACE="${NAMESPACE:-unified-replication-system}"
OUTPUT_DIR="diagnostics-$(date +%Y%m%d-%H%M%S)"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
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

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo_info "=== Unified Replication Operator Diagnostics ==="
echo_info "Collecting diagnostic information into: $OUTPUT_DIR"
echo ""

# Cluster information
echo_info "Collecting cluster information..."
kubectl version > "$OUTPUT_DIR/cluster-version.txt" 2>&1
kubectl get nodes -o wide > "$OUTPUT_DIR/nodes.txt" 2>&1
kubectl get namespaces > "$OUTPUT_DIR/namespaces.txt" 2>&1

# Operator information
echo_info "Collecting operator information..."
kubectl get deployment -n "$NAMESPACE" -o yaml > "$OUTPUT_DIR/deployment.yaml" 2>&1
kubectl get pods -n "$NAMESPACE" -o yaml > "$OUTPUT_DIR/pods.yaml" 2>&1
kubectl get svc -n "$NAMESPACE" -o yaml > "$OUTPUT_DIR/services.yaml" 2>&1

# Operator logs
echo_info "Collecting operator logs..."
kubectl logs -n "$NAMESPACE" -l control-plane=controller-manager --tail=1000 \
  > "$OUTPUT_DIR/operator-logs.txt" 2>&1

# If multiple pods, get all
POD_COUNT=$(kubectl get pods -n "$NAMESPACE" -l control-plane=controller-manager --no-headers | wc -l)
if [ "$POD_COUNT" -gt 1 ]; then
    for POD in $(kubectl get pods -n "$NAMESPACE" -l control-plane=controller-manager -o name); do
        POD_NAME=$(echo $POD | cut -d'/' -f2)
        kubectl logs -n "$NAMESPACE" "$POD" --tail=500 > "$OUTPUT_DIR/logs-$POD_NAME.txt" 2>&1
    done
fi

# CRDs
echo_info "Collecting CRD information..."
kubectl get crd unifiedvolumereplications.replication.unified.io -o yaml \
  > "$OUTPUT_DIR/uvr-crd.yaml" 2>&1

# All UnifiedVolumeReplication resources
echo_info "Collecting UnifiedVolumeReplication resources..."
kubectl get uvr -A -o yaml > "$OUTPUT_DIR/all-replications.yaml" 2>&1
kubectl get uvr -A > "$OUTPUT_DIR/replications-list.txt" 2>&1

# Events
echo_info "Collecting events..."
kubectl get events -A --sort-by='.lastTimestamp' | tail -200 \
  > "$OUTPUT_DIR/recent-events.txt" 2>&1
kubectl get events -n "$NAMESPACE" -o yaml > "$OUTPUT_DIR/operator-events.yaml" 2>&1

# Webhook configuration
echo_info "Collecting webhook information..."
kubectl get validatingwebhookconfiguration -o yaml \
  > "$OUTPUT_DIR/webhook-config.yaml" 2>&1

# RBAC
echo_info "Collecting RBAC information..."
kubectl get clusterrole,clusterrolebinding | grep unified-replication \
  > "$OUTPUT_DIR/rbac-list.txt" 2>&1
kubectl get clusterrole -l app.kubernetes.io/name=unified-replication-operator -o yaml \
  > "$OUTPUT_DIR/clusterroles.yaml" 2>&1

# Backend CRDs
echo_info "Collecting backend CRD information..."
kubectl get crd | grep -E "volumereplication|trident|dell" \
  > "$OUTPUT_DIR/backend-crds.txt" 2>&1

# Backend resources (if any)
kubectl get volumereplication -A -o yaml > "$OUTPUT_DIR/ceph-resources.yaml" 2>&1 || true
kubectl get tridentmirrorrelationship -A -o yaml > "$OUTPUT_DIR/trident-resources.yaml" 2>&1 || true
kubectl get dellcsireplicationgroup -A -o yaml > "$OUTPUT_DIR/powerstore-resources.yaml" 2>&1 || true

# Metrics
echo_info "Collecting metrics..."
if kubectl get svc -n "$NAMESPACE" | grep -q metrics; then
    kubectl port-forward -n "$NAMESPACE" svc/unified-replication-operator-metrics 8080:8080 &
    PF_PID=$!
    sleep 2
    curl -s http://localhost:8080/metrics > "$OUTPUT_DIR/metrics.txt" 2>&1 || echo "Metrics unavailable" > "$OUTPUT_DIR/metrics.txt"
    kill $PF_PID 2>/dev/null || true
fi

# Health status
echo_info "Collecting health status..."
if kubectl get deployment -n "$NAMESPACE" unified-replication-operator &> /dev/null; then
    kubectl port-forward -n "$NAMESPACE" deployment/unified-replication-operator 8081:8081 &
    PF_PID=$!
    sleep 2
    curl -s http://localhost:8081/healthz > "$OUTPUT_DIR/health.json" 2>&1 || echo "Health unavailable" > "$OUTPUT_DIR/health.json"
    curl -s http://localhost:8081/readyz > "$OUTPUT_DIR/readiness.json" 2>&1 || echo "Readiness unavailable" > "$OUTPUT_DIR/readiness.json"
    kill $PF_PID 2>/dev/null || true
fi

# Helm release
echo_info "Collecting Helm release information..."
if command -v helm &> /dev/null; then
    helm list -n "$NAMESPACE" > "$OUTPUT_DIR/helm-releases.txt" 2>&1
    helm get values unified-replication-operator -n "$NAMESPACE" \
      > "$OUTPUT_DIR/helm-values.yaml" 2>&1 || true
fi

# Package everything
echo_info "Creating archive..."
tar -czf "$OUTPUT_DIR.tar.gz" "$OUTPUT_DIR"
rm -rf "$OUTPUT_DIR"

echo ""
echo_info "========================================="
echo_info "Diagnostics Collection Complete!"
echo_info "========================================="
echo_info "Archive: $OUTPUT_DIR.tar.gz"
echo ""
echo_info "Please attach this file when reporting issues."
echo ""

