#!/bin/bash
# Manual webhook certificate creation script

set -e

NAMESPACE="${NAMESPACE:-unified-replication-system}"
SERVICE_NAME="unified-replication-operator-webhook-service"
SECRET_NAME="unified-replication-operator-webhook-cert"
WEBHOOK_NAME="unified-replication-operator-validating-webhook"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_info "Creating webhook certificates for unified-replication-operator..."

# Create temp directory
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

cd "$TMPDIR"

# Generate private key
echo_info "Generating private key..."
openssl genrsa -out tls.key 4096

# Create certificate signing request
echo_info "Creating certificate signing request..."
cat > csr.conf <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
prompt = no

[req_distinguished_name]
CN = ${SERVICE_NAME}.${NAMESPACE}.svc

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${SERVICE_NAME}
DNS.2 = ${SERVICE_NAME}.${NAMESPACE}
DNS.3 = ${SERVICE_NAME}.${NAMESPACE}.svc
DNS.4 = ${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local
EOF

# Generate certificate signing request
openssl req -new -key tls.key -out tls.csr -config csr.conf

# Generate self-signed certificate
echo_info "Generating self-signed certificate..."
openssl x509 -req -in tls.csr -signkey tls.key -out tls.crt -days 365 -extensions v3_req -extfile csr.conf

# Verify certificate
echo_info "Verifying certificate..."
openssl x509 -in tls.crt -text -noout | grep -A1 "Subject Alternative Name" || echo_warn "No SAN found"

# Check if secret already exists
if kubectl get secret "$SECRET_NAME" -n "$NAMESPACE" &>/dev/null; then
    echo_warn "Secret $SECRET_NAME already exists, deleting it..."
    kubectl delete secret "$SECRET_NAME" -n "$NAMESPACE"
fi

# Create Kubernetes secret
echo_info "Creating Kubernetes secret..."
kubectl create secret tls "$SECRET_NAME" \
    --cert=tls.crt \
    --key=tls.key \
    -n "$NAMESPACE"

echo_info "Secret created successfully ✓"

# Patch webhook configuration with CA bundle
echo_info "Patching webhook configuration with CA bundle..."
CA_BUNDLE=$(cat tls.crt | base64 | tr -d '\n')

if kubectl get validatingwebhookconfiguration "$WEBHOOK_NAME" &>/dev/null; then
    kubectl patch validatingwebhookconfiguration "$WEBHOOK_NAME" \
        --type='json' \
        -p="[{'op': 'add', 'path': '/webhooks/0/clientConfig/caBundle', 'value':'${CA_BUNDLE}'}]"
    echo_info "Webhook configuration patched ✓"
else
    echo_warn "ValidatingWebhookConfiguration $WEBHOOK_NAME not found, skipping patch"
    echo_info "The webhook will be patched automatically when it's created"
fi

# Restart operator deployment if it exists
echo_info "Checking for operator deployment..."
if kubectl get deployment unified-replication-operator -n "$NAMESPACE" &>/dev/null; then
    echo_info "Restarting operator deployment..."
    kubectl rollout restart deployment unified-replication-operator -n "$NAMESPACE"
    echo_info "Waiting for rollout to complete..."
    kubectl rollout status deployment unified-replication-operator -n "$NAMESPACE" --timeout=120s || echo_warn "Rollout may still be in progress"
else
    echo_warn "Deployment not found yet, will use certificate when created"
fi

echo ""
echo_info "========================================="
echo_info "Webhook Certificate Created Successfully!"
echo_info "========================================="
echo ""
echo_info "Certificate Details:"
echo "  Secret: $SECRET_NAME"
echo "  Namespace: $NAMESPACE"
echo "  Valid for: 365 days"
echo ""
echo_info "Verify the certificate:"
echo "  kubectl get secret $SECRET_NAME -n $NAMESPACE"
echo "  kubectl describe secret $SECRET_NAME -n $NAMESPACE"
echo ""
echo_info "Check pods:"
echo "  kubectl get pods -n $NAMESPACE"
echo ""

