#!/bin/bash
# Upgrade script for Unified Replication Operator

set -e

# Default values
NAMESPACE="${NAMESPACE:-unified-replication-system}"
RELEASE_NAME="${RELEASE_NAME:-unified-replication-operator}"
HELM_TIMEOUT="${HELM_TIMEOUT:-10m}"
SKIP_PREFLIGHT="${SKIP_PREFLIGHT:-false}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Pre-upgrade checks
preupgrade_checks() {
    echo_info "Running pre-upgrade checks..."
    
    # Check if release exists
    if ! helm list -n "$NAMESPACE" | grep -q "$RELEASE_NAME"; then
        echo_warn "Release $RELEASE_NAME not found. Use install.sh instead."
        exit 1
    fi
    
    # Get current version
    CURRENT_VERSION=$(helm list -n "$NAMESPACE" -o json | jq -r ".[] | select(.name==\"$RELEASE_NAME\") | .chart" | cut -d'-' -f2-)
    echo_info "Current version: $CURRENT_VERSION"
    
    # Backup current values
    echo_info "Backing up current values..."
    helm get values "$RELEASE_NAME" -n "$NAMESPACE" > "/tmp/${RELEASE_NAME}-values-backup.yaml"
    echo_info "Values backed up to /tmp/${RELEASE_NAME}-values-backup.yaml"
    
    echo_info "Pre-upgrade checks passed ✓"
}

# Perform upgrade
perform_upgrade() {
    echo_info "Performing upgrade..."
    
    CHART_PATH="$PROJECT_ROOT/helm/unified-replication-operator"
    
    helm upgrade \
        "$RELEASE_NAME" \
        "$CHART_PATH" \
        --namespace "$NAMESPACE" \
        --timeout "$HELM_TIMEOUT" \
        --wait \
        --atomic \
        "$@"
    
    echo_info "Upgrade completed ✓"
}

# Verify upgrade
verify_upgrade() {
    echo_info "Verifying upgrade..."
    
    # Check deployment status
    kubectl rollout status deployment/"$RELEASE_NAME" -n "$NAMESPACE" --timeout=300s
    
    # Get new version
    NEW_VERSION=$(helm list -n "$NAMESPACE" -o json | jq -r ".[] | select(.name==\"$RELEASE_NAME\") | .chart" | cut -d'-' -f2-)
    echo_info "New version: $NEW_VERSION"
    
    # Check pod status
    PODS=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/name=unified-replication-operator" --field-selector=status.phase=Running -o name | wc -l)
    echo_info "Running pods: $PODS"
    
    echo_info "Upgrade verified ✓"
}

# Main upgrade flow
main() {
    echo_info "=== Unified Replication Operator Upgrade ==="
    echo ""
    
    # Pre-upgrade checks
    if [ "$SKIP_PREFLIGHT" != "true" ]; then
        preupgrade_checks
    fi
    
    # Perform upgrade
    perform_upgrade "$@"
    
    # Verify
    verify_upgrade
    
    echo ""
    echo_info "========================================="
    echo_info "Upgrade Complete!"
    echo_info "========================================="
    echo ""
    echo_info "Rollback if needed:"
    echo "  helm rollback $RELEASE_NAME -n $NAMESPACE"
    echo ""
}

# Run main
main "$@"

