#!/bin/bash
# Test script for Helm chart validation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
CHART_PATH="$PROJECT_ROOT/helm/unified-replication-operator"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo_pass() {
    echo -e "${GREEN}✓${NC} $1"
}

echo_fail() {
    echo -e "${RED}✗${NC} $1"
}

TESTS_PASSED=0
TESTS_FAILED=0

test_chart_structure() {
    echo "Testing chart structure..."
    
    if [ -f "$CHART_PATH/Chart.yaml" ]; then
        echo_pass "Chart.yaml exists"
        ((TESTS_PASSED++))
    else
        echo_fail "Chart.yaml missing"
        ((TESTS_FAILED++))
        return
    fi
    
    if [ -f "$CHART_PATH/values.yaml" ]; then
        echo_pass "values.yaml exists"
        ((TESTS_PASSED++))
    else
        echo_fail "values.yaml missing"
        ((TESTS_FAILED++))
    fi
    
    if [ -d "$CHART_PATH/templates" ]; then
        echo_pass "templates/ directory exists"
        ((TESTS_PASSED++))
    else
        echo_fail "templates/ directory missing"
        ((TESTS_FAILED++))
    fi
}

test_helm_lint() {
    echo "Running Helm lint..."
    
    if helm lint "$CHART_PATH" > /dev/null 2>&1; then
        echo_pass "Helm lint passed"
        ((TESTS_PASSED++))
    else
        echo_fail "Helm lint failed"
        helm lint "$CHART_PATH"
        ((TESTS_FAILED++))
    fi
}

test_template_rendering() {
    echo "Testing template rendering..."
    
    if helm template test "$CHART_PATH" > /dev/null 2>&1; then
        echo_pass "Templates render successfully"
        ((TESTS_PASSED++))
    else
        echo_fail "Template rendering failed"
        ((TESTS_FAILED++))
    fi
    
    # Test with custom values
    if helm template test "$CHART_PATH" \
        --set controller.maxConcurrentReconciles=5 \
        --set security.audit.enabled=false \
        > /dev/null 2>&1; then
        echo_pass "Custom values work"
        ((TESTS_PASSED++))
    else
        echo_fail "Custom values failed"
        ((TESTS_FAILED++))
    fi
}

test_required_resources() {
    echo "Testing required resources..."
    
    OUTPUT=$(helm template test "$CHART_PATH")
    
    # Check for Deployment
    if echo "$OUTPUT" | grep -q "kind: Deployment"; then
        echo_pass "Deployment template exists"
        ((TESTS_PASSED++))
    else
        echo_fail "Deployment template missing"
        ((TESTS_FAILED++))
    fi
    
    # Check for ServiceAccount
    if echo "$OUTPUT" | grep -q "kind: ServiceAccount"; then
        echo_pass "ServiceAccount template exists"
        ((TESTS_PASSED++))
    else
        echo_fail "ServiceAccount template missing"
        ((TESTS_FAILED++))
    fi
    
    # Check for ClusterRole
    if echo "$OUTPUT" | grep -q "kind: ClusterRole"; then
        echo_pass "ClusterRole template exists"
        ((TESTS_PASSED++))
    else
        echo_fail "ClusterRole template missing"
        ((TESTS_FAILED++))
    fi
    
    # Check for Service
    if echo "$OUTPUT" | grep -q "kind: Service"; then
        echo_pass "Service template exists"
        ((TESTS_PASSED++))
    else
        echo_fail "Service template missing"
        ((TESTS_FAILED++))
    fi
}

test_security_context() {
    echo "Testing security context..."
    
    OUTPUT=$(helm template test "$CHART_PATH")
    
    # Check for runAsNonRoot
    if echo "$OUTPUT" | grep -q "runAsNonRoot: true"; then
        echo_pass "runAsNonRoot enabled"
        ((TESTS_PASSED++))
    else
        echo_fail "runAsNonRoot not set"
        ((TESTS_FAILED++))
    fi
    
    # Check for readOnlyRootFilesystem
    if echo "$OUTPUT" | grep -q "readOnlyRootFilesystem: true"; then
        echo_pass "readOnlyRootFilesystem enabled"
        ((TESTS_PASSED++))
    else
        echo_fail "readOnlyRootFilesystem not set"
        ((TESTS_FAILED++))
    fi
}

test_kustomize_overlays() {
    echo "Testing Kustomize overlays..."
    
    for env in development staging production; do
        OVERLAY_PATH="$PROJECT_ROOT/config/overlays/$env"
        if [ -f "$OVERLAY_PATH/kustomization.yaml" ]; then
            if kubectl kustomize "$OVERLAY_PATH" > /dev/null 2>&1; then
                echo_pass "$env overlay valid"
                ((TESTS_PASSED++))
            else
                echo_fail "$env overlay invalid"
                ((TESTS_FAILED++))
            fi
        fi
    done
}

# Main test execution
main() {
    echo "=== Helm Chart Test Suite ==="
    echo ""
    
    test_chart_structure
    echo ""
    
    test_helm_lint
    echo ""
    
    test_template_rendering
    echo ""
    
    test_required_resources
    echo ""
    
    test_security_context
    echo ""
    
    test_kustomize_overlays
    echo ""
    
    # Summary
    TOTAL=$((TESTS_PASSED + TESTS_FAILED))
    echo "=== Test Summary ==="
    echo "Passed: $TESTS_PASSED/$TOTAL"
    echo "Failed: $TESTS_FAILED/$TOTAL"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed${NC}"
        exit 1
    fi
}

main

