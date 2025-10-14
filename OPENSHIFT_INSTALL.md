# OpenShift Installation Guide

This guide provides solutions for installing the Unified Replication Operator on OpenShift clusters.

## Problem

The default configuration uses UID/GID 65532, which conflicts with OpenShift's Security Context Constraints (SCCs). OpenShift requires UIDs in dynamically assigned ranges (e.g., 1000740000-1000749999).

## Solutions

### ✅ Solution 1: OpenShift Installation Script (Recommended)

Use the OpenShift-specific installation script:

```bash
# Standard installation with custom SCC
./scripts/install-openshift.sh

# Or use OpenShift-assigned UIDs (no custom SCC)
./scripts/install-openshift.sh --no-scc
```

**What this does:**
- Detects OpenShift automatically
- Creates custom SCC (if using default mode)
- Installs operator with OpenShift-compatible settings
- Binds SCC to ServiceAccount
- Verifies installation

### ✅ Solution 2: Manual Helm Installation with Values Override

#### Option A: Use Custom SCC (Preferred)

```bash
# 1. Create custom SCC
kubectl apply -f openshift-scc.yaml

# 2. Install operator
helm install unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace \
  --set openshift.enabled=true \
  --wait

# 3. Bind SCC to ServiceAccount
oc adm policy add-scc-to-user unified-replication-operator-scc \
  -z unified-replication-operator \
  -n unified-replication-system
```

#### Option B: Use OpenShift-Compatible Settings

```bash
# Install with OpenShift-assigned UIDs
helm install unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace \
  --set openshift.compatibleSecurity=true \
  --wait
```

Or use the pre-configured values file:

```bash
helm install unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace \
  -f values-openshift.yaml \
  --wait
```

### ✅ Solution 3: Fix Existing Installation

If you already have a failed installation:

```bash
# Uninstall failed release
helm uninstall unified-replication-operator -n unified-replication-system

# Reinstall with OpenShift support
./scripts/install-openshift.sh
```

## Verification

Check that pods are running:

```bash
# Check pods
kubectl get pods -n unified-replication-system

# Check pod details
kubectl describe pod -n unified-replication-system -l control-plane=controller-manager

# Check SCC binding (if using custom SCC)
oc describe scc unified-replication-operator-scc

# View logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f
```

## Understanding the Solutions

### Custom SCC Approach
- **Pros:** Operator runs with its preferred UID (65532), consistent across platforms
- **Cons:** Requires cluster-admin to create SCC
- **Use when:** You have cluster-admin access and want consistent behavior

### OpenShift-Compatible Approach
- **Pros:** No custom SCC needed, uses OpenShift's standard security model
- **Cons:** UID varies by namespace, harder to debug
- **Use when:** You don't have cluster-admin access or prefer standard OpenShift security

## Troubleshooting

### Pods Still Not Starting

1. **Check which SCC is being used:**
   ```bash
   kubectl get pod <pod-name> -n unified-replication-system -o yaml | grep "openshift.io/scc"
   ```

2. **Verify SCC allows the ServiceAccount:**
   ```bash
   oc describe scc unified-replication-operator-scc
   ```

3. **Check pod security context:**
   ```bash
   kubectl get pod <pod-name> -n unified-replication-system -o jsonpath='{.spec.securityContext}'
   ```

### SCC Not Binding

If you see "Forbidden: not usable by user or serviceaccount":

```bash
# Re-bind SCC
oc adm policy add-scc-to-user unified-replication-operator-scc \
  -z unified-replication-operator \
  -n unified-replication-system

# Restart deployment
kubectl rollout restart deployment unified-replication-operator \
  -n unified-replication-system
```

### Wrong UID Still Being Used

If the operator is still trying to use UID 65532 after upgrading:

```bash
# Verify Helm values
helm get values unified-replication-operator -n unified-replication-system

# Force upgrade
helm upgrade unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --set openshift.compatibleSecurity=true \
  --force \
  --wait
```

## Configuration Options

### Helm Values for OpenShift

```yaml
# Enable OpenShift features
openshift:
  enabled: true              # Creates custom SCC
  compatibleSecurity: false  # Keep false when using custom SCC

# OR use OpenShift-compatible mode
openshift:
  enabled: false             # Don't create SCC
  compatibleSecurity: true   # Remove hardcoded UIDs
```

### Custom SCC Configuration

The custom SCC (`openshift-scc.yaml`) provides:
- Non-root user requirement (secure)
- Drops all capabilities (secure)
- Allows any non-root UID (flexible)
- Read-only root filesystem (secure)
- Priority 10 (moderate restrictiveness)

## References

- [OpenShift Security Context Constraints](https://docs.openshift.com/container-platform/latest/authentication/managing-security-context-constraints.html)
- [Kubernetes Security Context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/)
- Main README: [README.md](README.md)
- Installation Script: [scripts/install-openshift.sh](scripts/install-openshift.sh)

## Quick Command Reference

```bash
# Install with custom SCC (recommended)
./scripts/install-openshift.sh

# Install without custom SCC
./scripts/install-openshift.sh --no-scc

# Check SCC
oc get scc unified-replication-operator-scc

# Check pod SCC assignment
oc describe pod <pod-name> -n unified-replication-system | grep scc

# Bind SCC manually
oc adm policy add-scc-to-user unified-replication-operator-scc \
  -z unified-replication-operator \
  -n unified-replication-system

# View SCC bindings
oc adm policy who-can use scc/unified-replication-operator-scc
```

