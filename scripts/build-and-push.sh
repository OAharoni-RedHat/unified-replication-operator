#!/bin/bash
set -e -o pipefail

# Configuration
OPERATOR="unified-replication-operator"
VERSION="${VERSION:-0.1.0}"
REGISTRY="${REGISTRY:-quay.io/rh-ee-oaharoni}"  # Override with your registry
IMAGE_NAME="${IMAGE_NAME:-unified-replication-operator}"
NAMESPACE="${NAMESPACE:-unified-replication-system}"
CONTAINER_TOOL="${CONTAINER_TOOL:-podman}"  # Can be 'docker' or 'podman'
SKIP_TESTS="${SKIP_TESTS:-true}"
SKIP_DEPLOY="${SKIP_DEPLOY:-false}"
PUSH_IMAGE="${PUSH_IMAGE:-true}"

# Computed values
FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${VERSION}"
LATEST_IMAGE="${REGISTRY}/${IMAGE_NAME}:latest"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}✅ [INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}⚠️  [WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}❌ [ERROR]${NC} $1"
}

echo_step() {
    echo -e "${BLUE}🔧 [STEP]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    echo_step "Checking prerequisites..."
    
    # Check container tool
    if ! command -v ${CONTAINER_TOOL} &> /dev/null; then
        echo_error "${CONTAINER_TOOL} not found. Please install ${CONTAINER_TOOL} or set CONTAINER_TOOL=docker"
        exit 1
    fi
    echo_info "${CONTAINER_TOOL} found: $(${CONTAINER_TOOL} --version | head -1)"
    
    # Check Go
    if ! command -v go &> /dev/null; then
        echo_error "Go not found. Please install Go 1.24+"
        exit 1
    fi
    echo_info "Go found: $(go version)"
    
    # Check kubectl/oc if not skipping deploy
    if [[ "${SKIP_DEPLOY}" != "true" ]]; then
        if command -v oc &> /dev/null; then
            echo_info "OpenShift CLI found: $(oc version --client | head -1)"
        elif command -v kubectl &> /dev/null; then
            echo_info "kubectl found: $(kubectl version --client --short 2>/dev/null || kubectl version --client)"
        else
            echo_warn "kubectl/oc not found. Deployment will be skipped."
            SKIP_DEPLOY="true"
        fi
    fi
    
    # Check Helm if not skipping deploy
    if [[ "${SKIP_DEPLOY}" != "true" ]] && ! command -v helm &> /dev/null; then
        echo_warn "Helm not found. Deployment will be skipped."
        SKIP_DEPLOY="true"
    fi
}

# Check cluster reachability
check_cluster() {
    if [[ "${SKIP_DEPLOY}" == "true" ]]; then
        return 0
    fi
    
    echo_step "Checking cluster reachability..."
    
    if command -v oc &> /dev/null; then
        if ! oc cluster-info &> /dev/null; then
            echo_warn "Cannot reach cluster. Deployment will be skipped."
            SKIP_DEPLOY="true"
        else
            echo_info "Cluster is reachable"
        fi
    elif command -v kubectl &> /dev/null; then
        if ! kubectl cluster-info &> /dev/null; then
            echo_warn "Cannot reach cluster. Deployment will be skipped."
            SKIP_DEPLOY="true"
        else
            echo_info "Cluster is reachable"
        fi
    fi
}

# Check git status
check_git_status() {
    echo_step "Checking git status..."
    
    if [[ -n $(git status --porcelain 2>/dev/null) ]]; then
        echo_warn "Uncommitted changes detected. Proceeding anyway..."
        git status --short
    else
        echo_info "Working directory is clean"
    fi
    
    # Get current commit
    if git rev-parse --git-dir > /dev/null 2>&1; then
        COMMIT=$(git rev-parse --short HEAD)
        BRANCH=$(git rev-parse --abbrev-ref HEAD)
        echo_info "Current commit: ${COMMIT} (${BRANCH})"
    fi
}

# Run tests
run_tests() {
    if [[ "${SKIP_TESTS}" == "true" ]]; then
        echo_warn "Skipping tests (SKIP_TESTS=true)"
        return 0
    fi
    
    echo_step "Running tests..."
    
    if ! make test-unit; then
        echo_error "Tests failed! Set SKIP_TESTS=true to skip tests."
        exit 1
    fi
    
    echo_info "Tests passed"
}

# Build the operator binary
build_binary() {
    echo_step "Building operator binary..."
    
    if ! make build; then
        echo_error "Binary build failed"
        exit 1
    fi
    
    echo_info "Binary built successfully"
}

# Build container image
build_image() {
    echo_step "Building container image: ${FULL_IMAGE}"
    
    ${CONTAINER_TOOL} build -t ${FULL_IMAGE} -t ${LATEST_IMAGE} .
    
    if [ $? -ne 0 ]; then
        echo_error "Image build failed"
        exit 1
    fi
    
    echo_info "Image built successfully"
    
    # Show image info
    echo_info "Image size: $(${CONTAINER_TOOL} images ${FULL_IMAGE} --format '{{.Size}}')"
}

# Login to registry
login_registry() {
    if [[ "${PUSH_IMAGE}" != "true" ]]; then
        return 0
    fi
    
    echo_step "Logging in to registry: ${REGISTRY}"
    
    # Extract registry host
    REGISTRY_HOST=$(echo ${REGISTRY} | cut -d'/' -f1)
    
    # Check if already logged in
    if ${CONTAINER_TOOL} login ${REGISTRY_HOST} --get-login &> /dev/null; then
        echo_info "Already logged in to ${REGISTRY_HOST}"
        return 0
    fi
    
    echo_warn "Not logged in to ${REGISTRY_HOST}"
    echo "Please login to the registry:"
    
    if ! ${CONTAINER_TOOL} login ${REGISTRY_HOST}; then
        echo_error "Registry login failed"
        exit 1
    fi
    
    echo_info "Successfully logged in to ${REGISTRY_HOST}"
}

# Push image to registry
push_image() {
    if [[ "${PUSH_IMAGE}" != "true" ]]; then
        echo_warn "Skipping image push (PUSH_IMAGE=false)"
        return 0
    fi
    
    echo_step "Pushing image to registry..."
    
    echo_info "Pushing: ${FULL_IMAGE}"
    if ! ${CONTAINER_TOOL} push ${FULL_IMAGE}; then
        echo_error "Failed to push ${FULL_IMAGE}"
        exit 1
    fi
    
    echo_info "Pushing: ${LATEST_IMAGE}"
    if ! ${CONTAINER_TOOL} push ${LATEST_IMAGE}; then
        echo_warn "Failed to push ${LATEST_IMAGE} (not critical)"
    fi
    
    echo_info "Images pushed successfully"
}

# Install CRDs
install_crds() {
    if [[ "${SKIP_DEPLOY}" == "true" ]]; then
        return 0
    fi
    
    echo_step "Installing CRDs..."
    
    local kubectl_cmd="kubectl"
    if command -v oc &> /dev/null; then
        kubectl_cmd="oc"
    fi
    
    # Apply CRDs
    ${kubectl_cmd} apply -f config/crd/bases/ 2>&1 | head -5 || echo_warn "CRDs may already exist"
    echo_info "CRDs installed"
}

# Create webhook certificates
create_webhook_cert() {
    if [[ "${SKIP_DEPLOY}" == "true" ]]; then
        return 0
    fi
    
    echo_step "Creating webhook certificates..."
    
    local kubectl_cmd="kubectl"
    if command -v oc &> /dev/null; then
        kubectl_cmd="oc"
    fi
    
    # Check if secret already exists
    if ${kubectl_cmd} get secret ${OPERATOR}-webhook-cert -n ${NAMESPACE} &>/dev/null; then
        echo_info "Webhook certificate already exists"
        return 0
    fi
    
    # Create namespace if it doesn't exist
    ${kubectl_cmd} create namespace ${NAMESPACE} 2>/dev/null || true
    
    # Generate certificate
    echo_info "Generating self-signed certificate..."
    TMPDIR=$(mktemp -d)
    trap "rm -rf $TMPDIR" EXIT
    
    cd "$TMPDIR"
    
    openssl req -x509 -newkey rsa:2048 -nodes -keyout tls.key -out tls.crt -days 365 \
        -subj "/CN=webhook" \
        -addext "subjectAltName=DNS:${OPERATOR}-webhook-service,DNS:${OPERATOR}-webhook-service.${NAMESPACE},DNS:${OPERATOR}-webhook-service.${NAMESPACE}.svc,DNS:${OPERATOR}-webhook-service.${NAMESPACE}.svc.cluster.local" \
        2>/dev/null
    
    # Create secret
    ${kubectl_cmd} create secret tls ${OPERATOR}-webhook-cert \
        --cert=tls.crt --key=tls.key -n ${NAMESPACE}
    
    echo_info "Webhook certificate created"
    
    cd - > /dev/null
}

# Deploy/Update via Helm
deploy_operator() {
    if [[ "${SKIP_DEPLOY}" == "true" ]]; then
        echo_warn "Skipping deployment (SKIP_DEPLOY=true)"
        return 0
    fi
    
    echo_step "Deploying operator via Helm..."
    
    # Install CRDs first
    install_crds
    
    # Create webhook cert
    create_webhook_cert
    
    # Check if release exists
    if helm list -n ${NAMESPACE} 2>/dev/null | grep -q ${OPERATOR}; then
        echo_info "Upgrading existing release..."
        helm upgrade ${OPERATOR} ./helm/${OPERATOR} \
            --namespace ${NAMESPACE} \
            --reuse-values \
            --set image.repository=${REGISTRY}/${IMAGE_NAME} \
            --set image.tag=${VERSION} \
            --set image.pullPolicy=Always \
            --set webhook.enabled=false \
            --set security.networkPolicy.enabled=false \
            --no-hooks \
            --wait=false
    else
        echo_info "Installing new release..."
        helm install ${OPERATOR} ./helm/${OPERATOR} \
            --namespace ${NAMESPACE} \
            --create-namespace \
            --set image.repository=${REGISTRY}/${IMAGE_NAME} \
            --set image.tag=${VERSION} \
            --set image.pullPolicy=Always \
            --set openshift.compatibleSecurity=true \
            --set webhook.enabled=false \
            --set security.networkPolicy.enabled=false \
            --no-hooks \
            --wait=false
    fi
    
    if [ $? -eq 0 ]; then
        echo_info "Helm deployment completed"
    else
        echo_error "Helm deployment failed"
        return 1
    fi
}

# Wait for rollout
wait_for_rollout() {
    if [[ "${SKIP_DEPLOY}" == "true" ]]; then
        return 0
    fi
    
    echo_step "Waiting for operator rollout..."
    
    # Give it a moment to start
    sleep 5
    
    if command -v kubectl &> /dev/null; then
        kubectl rollout status deployment/${OPERATOR} -n ${NAMESPACE} --timeout=120s || \
            echo_warn "Rollout status check timed out or failed"
    elif command -v oc &> /dev/null; then
        oc rollout status deployment/${OPERATOR} -n ${NAMESPACE} --timeout=120s || \
            echo_warn "Rollout status check timed out or failed"
    fi
}

# Verify deployment
verify_deployment() {
    if [[ "${SKIP_DEPLOY}" == "true" ]]; then
        return 0
    fi
    
    echo_step "Verifying deployment..."
    
    local kubectl_cmd="kubectl"
    if command -v oc &> /dev/null; then
        kubectl_cmd="oc"
    fi
    
    # Check pods
    echo_info "Checking pods in namespace ${NAMESPACE}:"
    ${kubectl_cmd} get pods -n ${NAMESPACE} -l "app.kubernetes.io/name=${OPERATOR}"
    
    # Check pod status
    local ready_pods=$(${kubectl_cmd} get pods -n ${NAMESPACE} -l "app.kubernetes.io/name=${OPERATOR}" -o jsonpath='{.items[*].status.conditions[?(@.type=="Ready")].status}' | grep -o "True" | wc -l)
    
    if [ ${ready_pods} -gt 0 ]; then
        echo_info "✅ ${ready_pods} pod(s) are ready"
    else
        echo_warn "⚠️  No pods are ready yet. Check logs with:"
        echo "    ${kubectl_cmd} logs -n ${NAMESPACE} -l control-plane=controller-manager -f"
    fi
}

# Print summary
print_summary() {
    echo ""
    echo_info "========================================="
    echo_info "Build and Deploy Summary"
    echo_info "========================================="
    echo ""
    echo "  Operator:     ${OPERATOR}"
    echo "  Version:      ${VERSION}"
    echo "  Image:        ${FULL_IMAGE}"
    echo "  Namespace:    ${NAMESPACE}"
    echo "  Registry:     ${REGISTRY}"
    echo "  Tests:        $([ "${SKIP_TESTS}" == "true" ] && echo "Skipped" || echo "Passed")"
    echo "  Push:         $([ "${PUSH_IMAGE}" == "true" ] && echo "Yes" || echo "No")"
    echo "  Deploy:       $([ "${SKIP_DEPLOY}" == "true" ] && echo "Skipped" || echo "Completed")"
    echo ""
    
    if [[ "${SKIP_DEPLOY}" != "true" ]]; then
        local kubectl_cmd="kubectl"
        if command -v oc &> /dev/null; then
            kubectl_cmd="oc"
        fi
        
        echo_info "Next steps:"
        echo "  • View logs:         ${kubectl_cmd} logs -n ${NAMESPACE} -l control-plane=controller-manager -f"
        echo "  • Check pods:        ${kubectl_cmd} get pods -n ${NAMESPACE}"
        echo "  • Test replication:  ${kubectl_cmd} apply -f trident-replication.yaml"
        echo "  • Check status:      ${kubectl_cmd} get unifiedvolumereplications -A"
    else
        echo_info "To deploy manually:"
        echo "  helm install ${OPERATOR} ./helm/${OPERATOR} \\"
        echo "    --namespace ${NAMESPACE} --create-namespace \\"
        echo "    --set image.repository=${REGISTRY}/${IMAGE_NAME} \\"
        echo "    --set image.tag=${VERSION}"
    fi
    echo ""
}

# Main execution
main() {
    echo ""
    echo_info "========================================="
    echo_info "Unified Replication Operator Build"
    echo_info "========================================="
    echo ""
    
    check_prerequisites
    check_cluster
    check_git_status
    run_tests
    build_binary
    build_image
    login_registry
    push_image
    deploy_operator
    wait_for_rollout
    verify_deployment
    print_summary
    
    echo_info "✅ Build and deploy process completed!"
}

# Show help
show_help() {
    cat << EOF
Unified Replication Operator Build and Deploy Script

Usage: $0 [options]

Environment Variables:
  VERSION              Version tag for the image (default: 0.1.0)
  REGISTRY             Container registry (default: quay.io/YOUR_USERNAME)
  IMAGE_NAME           Image name (default: unified-replication-operator)
  NAMESPACE            Kubernetes namespace (default: unified-replication-system)
  CONTAINER_TOOL       Container tool to use (default: podman, can be docker)
  SKIP_TESTS           Skip running tests (default: false)
  SKIP_DEPLOY          Skip deployment to cluster (default: false)
  PUSH_IMAGE           Push image to registry (default: true)

Examples:
  # Build, push, and deploy with default settings
  REGISTRY=quay.io/myuser $0

  # Build and push only (no deploy)
  REGISTRY=quay.io/myuser SKIP_DEPLOY=true $0

  # Build only (no push, no deploy)
  PUSH_IMAGE=false SKIP_DEPLOY=true $0

  # Build with custom version
  VERSION=0.2.0 REGISTRY=quay.io/myuser $0

  # Skip tests and use docker instead of podman
  SKIP_TESTS=true CONTAINER_TOOL=docker REGISTRY=quay.io/myuser $0

  # Build for OpenShift internal registry
  REGISTRY=image-registry.openshift-image-registry.svc:5000/unified-replication-system $0

EOF
}

# Parse arguments
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    show_help
    exit 0
fi

# Run main
main "$@"

