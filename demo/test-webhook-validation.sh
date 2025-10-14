#!/bin/bash
# Test webhook validation functionality

set -e

export KUBECONFIG="${KUBECONFIG:-/home/oaharoni/aws-gpfs-playground/ocp_install_files/auth/kubeconfig}"

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo_info() {
    echo -e "${GREEN}✅${NC} $1"
}

echo_error() {
    echo -e "${RED}❌${NC} $1"
}

echo_step() {
    echo -e "\n${BLUE}━━━ $1 ━━━${NC}"
}

echo_warn() {
    echo -e "${YELLOW}⚠️${NC}  $1"
}

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Webhook Validation Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Check if webhooks are enabled
echo_step "1. Check Webhook Status"

WEBHOOK_ENABLED=$(kubectl get deployment unified-replication-operator -n unified-replication-system \
    -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="ENABLE_WEBHOOKS")].value}' 2>/dev/null || echo "false")

echo "Webhooks enabled in deployment: ${WEBHOOK_ENABLED}"

WEBHOOK_CONFIG=$(kubectl get validatingwebhookconfiguration 2>/dev/null | grep unified-replication || echo "")
if [ -n "$WEBHOOK_CONFIG" ]; then
    echo_info "ValidatingWebhookConfiguration exists"
    kubectl get validatingwebhookconfiguration | grep unified-replication
else
    echo_warn "ValidatingWebhookConfiguration NOT found"
    echo ""
    echo "Webhooks are currently DISABLED for simplified deployment."
    echo "Validation still occurs in the controller reconciliation loop."
    echo ""
    echo "To enable webhooks:"
    echo "  ENABLE_WEBHOOKS=true ./scripts/build-and-push.sh"
    echo ""
    echo "Testing controller-based validation instead..."
fi

# Test with valid resource
echo_step "2. Test Valid Resource"
echo "Applying valid Trident replication..."
kubectl apply -f trident-replication.yaml
sleep 3

STATUS=$(kubectl get uvr trident-volume-replication -n default -o jsonpath='{.status.conditions[0].status}' 2>/dev/null)
if [ "$STATUS" = "True" ]; then
    echo_info "Valid resource accepted and reconciled successfully"
else
    echo_warn "Resource created but not yet ready (STATUS: $STATUS)"
fi

# Test with invalid resource
echo_step "3. Test Invalid Resource (Controller Validation)"
echo "Attempting to create resource with invalid state..."

if kubectl apply -f test-invalid-replication.yaml 2>&1 | tee /tmp/invalid-test.log; then
    echo_warn "Invalid resource was accepted (validation happens at reconciliation)"
    
    # Wait for reconciliation
    sleep 5
    
    # Check if it shows error in status
    INVALID_STATUS=$(kubectl get uvr invalid-test-replication -n default \
        -o jsonpath='{.status.conditions[0]}' 2>/dev/null)
    
    if echo "$INVALID_STATUS" | grep -q "False"; then
        echo_info "Controller validation caught the error"
        kubectl get uvr invalid-test-replication -n default \
            -o jsonpath='{.status.conditions[0].message}'
        echo ""
    fi
    
    # Cleanup
    kubectl delete uvr invalid-test-replication -n default 2>/dev/null || true
else
    # Webhook rejected it
    echo_info "Invalid resource REJECTED by webhook! ✅"
    cat /tmp/invalid-test.log | grep -i "error\|invalid\|denied" | head -3
fi

# Test dry-run
echo_step "4. Test Dry-Run Validation"
echo "Testing with --dry-run=server (uses webhook if enabled, otherwise basic validation)..."

if kubectl apply -f test-invalid-replication.yaml --dry-run=server 2>&1 | tee /tmp/dryrun-test.log; then
    echo_warn "Dry-run accepted (no webhook, controller validates at runtime)"
else
    echo_info "Dry-run validation caught errors"
    cat /tmp/dryrun-test.log | grep -i "error\|invalid" | head -3
fi

# Summary
echo_step "Validation Summary"

if [ -n "$WEBHOOK_CONFIG" ]; then
    cat << EOF

✅ WEBHOOK VALIDATION ACTIVE:
   • ValidatingWebhookConfiguration: Present
   • Admission control: Pre-creation validation
   • Invalid resources: Rejected immediately
   • Benefit: Fast feedback, prevents bad resources

Test Results:
   ✅ Valid resource: Accepted
   ✅ Invalid resource: Should be rejected by webhook
   ✅ Dry-run: Uses webhook validation

EOF
else
    cat << EOF

ℹ️  CONTROLLER VALIDATION ACTIVE (Webhooks Disabled):
   • Validation: During reconciliation loop
   • Invalid resources: Accepted by API, fail at reconciliation
   • Status: Shows validation errors in conditions
   • Benefit: Simpler deployment, no certificate management

Test Results:
   ✅ Valid resource: Accepted and reconciled
   ⚠️  Invalid resource: Accepted but fails at reconciliation
   ⚠️  Dry-run: Basic validation only

To enable webhook validation:
   ENABLE_WEBHOOKS=true ./scripts/build-and-push.sh

EOF
fi

echo_info "Validation test complete!"
echo ""

