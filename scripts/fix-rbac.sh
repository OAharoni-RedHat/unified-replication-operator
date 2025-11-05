#!/bin/bash
# Quick fix script to update RBAC permissions for v1alpha2
# This replaces unifiedvolumereplications rules with v1alpha2 volumereplications rules

set -e

export KUBECONFIG="${KUBECONFIG:-/home/oaharoni/aws-gpfs-playground/ocp_install_files/auth/kubeconfig}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}🔧 Fixing RBAC permissions for v1alpha2...${NC}"

CLUSTERROLE_NAME="unified-replication-operator-manager"

if ! kubectl get clusterrole "$CLUSTERROLE_NAME" &>/dev/null; then
    echo -e "${YELLOW}⚠️  ClusterRole not found. Deploy operator first.${NC}"
    exit 1
fi

# Get current ClusterRole and save to temp file
TEMP_FILE=$(mktemp)
kubectl get clusterrole "$CLUSTERROLE_NAME" -o yaml > "$TEMP_FILE"

# Check if it already has volumereplications
if grep -q "volumereplications" "$TEMP_FILE" && ! grep -q "unifiedvolumereplications" "$TEMP_FILE"; then
    echo -e "${GREEN}✅ RBAC already has v1alpha2 permissions!${NC}"
    rm -f "$TEMP_FILE"
    exit 0
fi

echo "Removing old unifiedvolumereplications rules and adding v1alpha2 rules..."

# Use kubectl patch to replace the first 3 rules (unifiedvolumereplications)
# We'll remove them and add the new ones
kubectl patch clusterrole "$CLUSTERROLE_NAME" --type='json' -p='[
  {
    "op": "remove",
    "path": "/rules/0"
  },
  {
    "op": "remove",
    "path": "/rules/0"
  },
  {
    "op": "remove",
    "path": "/rules/0"
  },
  {
    "op": "add",
    "path": "/rules/0",
    "value": {
      "apiGroups": ["replication.unified.io"],
      "resources": ["volumereplications", "volumereplicationclasses"],
      "verbs": ["get", "list", "watch", "create", "update", "patch", "delete"]
    }
  },
  {
    "op": "add",
    "path": "/rules/1",
    "value": {
      "apiGroups": ["replication.unified.io"],
      "resources": ["volumereplications/status"],
      "verbs": ["get", "update", "patch"]
    }
  },
  {
    "op": "add",
    "path": "/rules/2",
    "value": {
      "apiGroups": ["replication.unified.io"],
      "resources": ["volumereplications/finalizers"],
      "verbs": ["update"]
    }
  },
  {
    "op": "add",
    "path": "/rules/3",
    "value": {
      "apiGroups": ["replication.unified.io"],
      "resources": ["volumegroupreplications", "volumegroupreplicationclasses"],
      "verbs": ["get", "list", "watch", "create", "update", "patch", "delete"]
    }
  },
  {
    "op": "add",
    "path": "/rules/4",
    "value": {
      "apiGroups": ["replication.unified.io"],
      "resources": ["volumegroupreplications/status"],
      "verbs": ["get", "update", "patch"]
    }
  },
  {
    "op": "add",
    "path": "/rules/5",
    "value": {
      "apiGroups": ["replication.unified.io"],
      "resources": ["volumegroupreplications/finalizers"],
      "verbs": ["update"]
    }
  }
]'

rm -f "$TEMP_FILE"

echo -e "${GREEN}✅ RBAC updated! Restarting operator pod...${NC}"

# Restart operator pod to pick up new permissions
kubectl delete pod -n unified-replication-system -l control-plane=controller-manager 2>/dev/null || true

echo "Waiting for pod to restart..."
if kubectl wait --for=condition=ready pod -n unified-replication-system -l control-plane=controller-manager --timeout=2m 2>/dev/null; then
    echo -e "${GREEN}✅ Pod restarted successfully!${NC}"
else
    echo -e "${YELLOW}⚠️  Pod restart timeout - check manually${NC}"
fi

echo ""
echo -e "${GREEN}✅ RBAC fix complete!${NC}"
echo ""
echo "Verify the fix:"
echo "  kubectl get clusterrole unified-replication-operator-manager -o yaml | grep -A 2 volumereplication"
echo "  kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=20"
