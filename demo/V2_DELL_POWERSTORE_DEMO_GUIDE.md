# v2.0.0-beta Dell PowerStore Demo - Unified Operator Integration

## Overview

This demo showcases the Unified Replication Operator working with Dell PowerStore backend. You'll see how the operator integrates with Dell CSI operator, creates unified CRs, and manages Dell-specific resources.

**What You'll Learn:**
- How to install Dell CSI operator (PowerStore)
- How to install Unified Replication Operator
- How Dell operator auto-creates `DellCSIReplicationGroup` from PVC
- How Unified operator creates `VolumeReplication` to manage Dell CRs
- How changes to `VolumeReplication` are automatically applied to Dell CRs

**Time:** ~20 minutes

---

## Prerequisites

- Kubernetes cluster with kubectl access
- Cluster admin permissions
- Access to Dell PowerStore storage system (or ability to install Dell CSI operator)
- Helm 3.x installed

**Note:** This demo assumes a fresh cluster. If you have existing installations, you may need to adjust the steps.

---

## Part 1: Install Dell CSI Operator (PowerStore)

The Dell CSI operator provides the PowerStore CSI driver and replication capabilities.

### Option 1: Install via OperatorHub (OpenShift)

**For OpenShift clusters:**

```bash
# Create namespace for Dell CSI operator
oc create namespace dell-csi-operator

# Install Dell CSI Operator from OperatorHub
oc create -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: dell-csi-operator
  namespace: dell-csi-operator
spec:
  channel: stable
  name: dell-csi-operator
  source: certified-operators
  sourceNamespace: openshift-marketplace
  installPlanApproval: Automatic
EOF

# Wait for operator to be ready
oc wait --for=condition=Installed subscription/dell-csi-operator -n dell-csi-operator --timeout=10m

# Verify operator is running
oc get pods -n dell-csi-operator
```

### Option 2: Install via Helm (Kubernetes/OpenShift)

```bash
# Add Dell Helm repository
helm repo add dell https://dell.github.io/helm-charts
helm repo update

# Install Dell CSI Operator
helm install dell-csi-operator dell/dell-csi-operator \
  --namespace dell-csi-operator \
  --create-namespace \
  --wait

# Verify installation
kubectl get pods -n dell-csi-operator
```

### Option 3: Install via Dell Documentation

Follow the official Dell CSI PowerStore installation guide:
- [Dell CSI PowerStore Installation](https://dell.github.io/csm-docs/docs/installation/)

### Verify Dell CSI Operator Installation

```bash
# Check operator pods
kubectl get pods -n dell-csi-operator

# Expected output:
# NAME                                  READY   STATUS    RESTARTS   AGE
# dell-csi-operator-xxxxxxxxx-xxxxx     1/1     Running   0          2m

# Check Dell CSI CRDs are installed
kubectl get crd | grep dell

# Expected CRDs:
# - dellcsireplicationgroups.replication.dell.com
# - dellcsistoragearrays.dell.com
# - etc.

# Verify DellCSIReplicationGroup CRD exists
kubectl get crd dellcsireplicationgroups.replication.dell.com
```

**If CRD is missing:**
```bash
# The CRD should be installed automatically by Dell operator
# If missing, check operator logs
kubectl logs -n dell-csi-operator -l app=dell-csi-operator --tail=50

# Or restart the operator
kubectl rollout restart deployment/dell-csi-operator -n dell-csi-operator
kubectl wait --for=condition=ready pod -n dell-csi-operator -l app=dell-csi-operator --timeout=5m
```

---

## Part 2: Install Unified Replication Operator

### Step 1: Install CRDs

```bash
# Install Unified Replication Operator CRDs
kubectl apply -f config/crd/bases/replication.unified.io_volumereplicationclasses.yaml
kubectl apply -f config/crd/bases/replication.unified.io_volumereplications.yaml
kubectl apply -f config/crd/bases/replication.unified.io_volumegroupreplicationclasses.yaml
kubectl apply -f config/crd/bases/replication.unified.io_volumegroupreplications.yaml

# Or install all at once
kubectl apply -f config/crd/bases/

# Verify CRDs are installed
kubectl get crd | grep replication.unified.io
```

**Expected Output:**
```
NAME                                          CREATED AT
volumereplicationclasses.replication.unified.io        2024-10-28T10:00:00Z
volumereplications.replication.unified.io              2024-10-28T10:00:00Z
volumegroupreplicationclasses.replication.unified.io   2024-10-28T10:00:00Z
volumegroupreplications.replication.unified.io        2024-10-28T10:00:00Z
```

### Step 2: Build and Deploy Operator

**For OpenShift:**

```bash
export KUBECONFIG=/path/to/your/kubeconfig

# Expose OpenShift internal registry
oc patch configs.imageregistry.operator.openshift.io/cluster --type merge -p '{"spec":{"defaultRoute":true}}'

# Get registry URL and login
REGISTRY=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
TOKEN=$(oc whoami -t)
podman login -u $(oc whoami) -p $TOKEN $REGISTRY --tls-verify=false

# Build, push, and deploy
cd /path/to/unified-replication-operator
REGISTRY=$REGISTRY/unified-replication-system VERSION=2.0.0-beta ./scripts/build-and-push.sh
```

**For external registry:**

```bash
podman login quay.io
REGISTRY=quay.io/YOUR_USERNAME VERSION=2.0.0-beta ./scripts/build-and-push.sh
```

The script handles:
- ✅ Running tests
- ✅ Building operator binary
- ✅ Building container image
- ✅ Pushing to registry
- ✅ Installing CRDs
- ✅ Deploying operator via Helm

### Step 3: Verify Operator Installation

```bash
# Check operator pod is running
kubectl get pods -n unified-replication-system

# Expected output:
# NAME                                            READY   STATUS    RESTARTS   AGE
# unified-replication-operator-xxxxxxxxx-xxxxx   1/1     Running   0          1m

# Check operator logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=20

# Verify RBAC permissions include Dell resources
kubectl get clusterrole unified-replication-operator-manager -o yaml | grep dellcsireplicationgroups
```

---

## Part 3: Configure Dell PowerStore Storage

### Step 1: Create StorageClass with Replication Enabled

The StorageClass must have Dell replication parameters enabled:

```bash
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: powerstore-replication
provisioner: csi-powerstore.dellemc.com
parameters:
  # Enable replication
  replication.storage.dell.com/isReplicationEnabled: "true"
  # Remote PowerStore system ID (replace with your actual system ID)
  replication.storage.dell.com/remoteSystem: "PS-DR-001"
  # Protection policy (must exist on PowerStore array)
  replication.storage.dell.com/protectionPolicy: "15min-async"
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
EOF

# Verify StorageClass created
kubectl get storageclass powerstore-replication
```

**Note:** Replace `PS-DR-001` and `15min-async` with your actual PowerStore system ID and protection policy name.

### Step 2: Configure Dell CSI Driver (if needed)

If you haven't configured the Dell CSI driver yet, you'll need to:

1. **Create StorageArray CR** (if using Dell CSM):
```bash
kubectl apply -f - <<EOF
apiVersion: storage.dell.com/v1
kind: StorageArray
metadata:
  name: powerstore-primary
  namespace: dell-csi-operator
spec:
  # Configure your PowerStore array connection
  # See Dell documentation for details
EOF
```

2. **Verify driver is ready:**
```bash
kubectl get csidriver csi-powerstore.dellemc.com
kubectl get pods -n dell-csi-operator | grep powerstore
```

---

## Part 4: Create PVC and Observe Dell Auto-Creation

### Step 1: Create a PVC

```bash
# Create namespace for demo
kubectl create namespace dell-demo

# Create PVC with replication-enabled StorageClass
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-data-pvc
  namespace: dell-demo
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: powerstore-replication
EOF

# Wait for PVC to be bound (may take a moment)
kubectl wait --for=status=Bound pvc/app-data-pvc -n dell-demo --timeout=2m

# Verify PVC
kubectl get pvc -n dell-demo
```

### Step 2: Observe Dell Operator Auto-Creation

The Dell CSI operator should automatically detect the PVC with replication parameters and create a `DellCSIReplicationGroup`:

```bash
# Wait a few seconds for Dell operator to detect and create CR
sleep 10

# Check if DellCSIReplicationGroup was auto-created
kubectl get dellcsireplicationgroup -n dell-demo

# If created, view details
kubectl get dellcsireplicationgroup -n dell-demo -o yaml

# Check Dell operator logs to see auto-creation
kubectl logs -n dell-csi-operator -l app=dell-csi-operator --tail=50 | grep -i "replication\|pvc"
```

**Expected Behavior:**
- Dell operator watches for PVCs with `replication.storage.dell.com/isReplicationEnabled: "true"`
- Automatically creates `DellCSIReplicationGroup` CR
- CR name typically matches PVC name or follows Dell naming convention

**Note:** If Dell operator doesn't auto-create the CR, it may be because:
- Dell operator version doesn't support auto-creation
- PVC doesn't have required annotations/labels
- StorageClass parameters are incorrect

In this case, you can manually create the DellCSIReplicationGroup or proceed with Unified operator creating it.

---

## Part 5: Create VolumeReplication (Unified Operator)

Now we'll create a `VolumeReplication` CR that the Unified operator will use to manage the Dell CR.

### Step 1: Create VolumeReplicationClass

```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: powerstore-replication-class
spec:
  # Provisioner identifies this as Dell PowerStore backend
  provisioner: csi-powerstore.dellemc.com
  
  # Dell PowerStore-specific parameters
  parameters:
    # Protection policy name (must match StorageClass or PowerStore config)
    protectionPolicy: "15min-async"
    
    # Remote PowerStore system ID
    remoteSystem: "PS-DR-001"
    
    # Recovery Point Objective
    rpo: "15m"
    
    # Optional: Remote cluster identifier
    remoteClusterId: "dr-cluster"
EOF

# Verify class created
kubectl get volumereplicationclass powerstore-replication-class
```

### Step 2: Create VolumeReplication

```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: app-data-replication
  namespace: dell-demo
spec:
  # Reference to the VolumeReplicationClass
  volumeReplicationClass: powerstore-replication-class
  
  # Name of the PVC to replicate
  pvcName: app-data-pvc
  
  # Replication state: "primary" means this is the active site
  replicationState: primary
  
  # Automatically resync after failures
  autoResync: true
EOF

# Verify VolumeReplication created
kubectl get volumereplication -n dell-demo
```

### Step 3: Verify Unified Operator Created/Updated Dell CR

```bash
# Wait a moment for operator to reconcile
sleep 10

# Check VolumeReplication status
kubectl get vr app-data-replication -n dell-demo -o yaml

# Check DellCSIReplicationGroup
kubectl get dellcsireplicationgroup -n dell-demo -o yaml

# Verify Unified operator is managing the Dell CR
kubectl get dellcsireplicationgroup -n dell-demo -o jsonpath='{.items[0].metadata.ownerReferences}'
```

**Expected Results:**
- ✅ `VolumeReplication` shows `Ready: True`
- ✅ `DellCSIReplicationGroup` exists (either created by Unified operator or updated by it)
- ✅ Dell CR has owner reference pointing to `VolumeReplication`
- ✅ Dell CR spec matches Unified operator's translation

### Step 4: Verify Translation

```bash
# Check VolumeReplication state
kubectl get vr app-data-replication -n dell-demo -o jsonpath='{.spec.replicationState}'
echo ""

# Check Dell CR spec
kubectl get dellcsireplicationgroup app-data-replication -n dell-demo -o jsonpath='{.spec}' | jq .
```

**Expected Translation:**
- `replicationState: primary` → Dell CR may have `action: Failover` (if needed) or no action (Dell manages via protection policy)
- PVC is labeled with `replication.storage.dell.com/group: app-data-replication`
- Dell CR has `pvcSelector` matching the label

---

## Part 6: Demonstrate Change Propagation

This demonstrates that changes to `VolumeReplication` are automatically applied to the Dell CR.

### Step 1: Check Current State

```bash
echo "=== Current VolumeReplication State ==="
kubectl get vr app-data-replication -n dell-demo -o jsonpath='{.spec.replicationState}'
echo ""

echo "=== Current Dell CR Spec ==="
kubectl get dellcsireplicationgroup app-data-replication -n dell-demo -o jsonpath='{.spec}' | jq .
```

### Step 2: Update VolumeReplication State

```bash
# Change replication state from primary to secondary (simulating failover scenario)
kubectl patch vr app-data-replication -n dell-demo \
  --type merge \
  -p '{"spec":{"replicationState":"secondary"}}'

# Verify update
kubectl get vr app-data-replication -n dell-demo
```

### Step 3: Wait for Reconciliation

```bash
# Wait for operator to reconcile (may take 10-30 seconds)
echo "Waiting for operator to reconcile..."
sleep 15

# Check operator logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=50 | grep -i "app-data-replication\|dell\|powerstore"
```

### Step 4: Verify Dell CR Updated

```bash
echo "=== Updated Dell CR Spec ==="
kubectl get dellcsireplicationgroup app-data-replication -n dell-demo -o jsonpath='{.spec}' | jq .

# Check if action was set (for secondary state)
kubectl get dellcsireplicationgroup app-data-replication -n dell-demo -o jsonpath='{.spec.action}'
echo ""
```

**Expected Behavior:**
- ✅ Unified operator detects `VolumeReplication` change
- ✅ Operator updates `DellCSIReplicationGroup` spec
- ✅ Dell CR reflects the new replication state
- ✅ Changes propagate within 30 seconds

### Step 5: Change Back to Primary

```bash
# Change back to primary
kubectl patch vr app-data-replication -n dell-demo \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'

# Wait and verify
sleep 15
kubectl get dellcsireplicationgroup app-data-replication -n dell-demo -o jsonpath='{.spec}' | jq .
```

---

## Part 7: Verify Complete Integration

### Check All Resources

```bash
# List all Unified Replication resources
kubectl get vr,vrc -n dell-demo

# List Dell resources
kubectl get dellcsireplicationgroup -n dell-demo

# Check PVC labels (Unified operator adds labels)
kubectl get pvc app-data-pvc -n dell-demo -o yaml | grep -A 5 "labels:"
```

### Check Operator Logs

```bash
# View Unified operator logs for Dell operations
kubectl logs -n unified-replication-system \
  -l control-plane=controller-manager \
  --tail=100 | grep -i "dell\|powerstore\|app-data-replication"
```

**Expected Log Entries:**
```
INFO  powerstore-adapter  Reconciling VolumeReplication with Dell PowerStore backend
INFO  powerstore-adapter  Detected Dell PowerStore backend
INFO  powerstore-adapter  Successfully created/updated DellCSIReplicationGroup
```

### Verify Owner References

```bash
# Check that Dell CR is owned by VolumeReplication
kubectl get dellcsireplicationgroup app-data-replication -n dell-demo \
  -o jsonpath='{.metadata.ownerReferences[*].kind}' && echo ""

kubectl get dellcsireplicationgroup app-data-replication -n dell-demo \
  -o jsonpath='{.metadata.ownerReferences[*].name}' && echo ""
```

**Expected:**
- Owner kind: `VolumeReplication`
- Owner name: `app-data-replication`

This ensures that if you delete the `VolumeReplication`, the Dell CR will be automatically cleaned up.

---

## Cleanup

### Clean Up Demo Resources

```bash
# Delete VolumeReplication (this will also delete Dell CR via owner reference)
kubectl delete vr app-data-replication -n dell-demo

# Verify Dell CR is deleted
kubectl get dellcsireplicationgroup -n dell-demo

# Delete VolumeReplicationClass
kubectl delete vrc powerstore-replication-class

# Delete PVC (optional - may be in use)
kubectl delete pvc app-data-pvc -n dell-demo

# Delete StorageClass (optional)
kubectl delete storageclass powerstore-replication

# Delete namespace
kubectl delete namespace dell-demo
```

### Clean Up Operators (Optional)

**Unified Replication Operator:**
```bash
# From repo root
./scripts/cleanup-demo.sh --operator
```

**Dell CSI Operator:**
```bash
# Uninstall via Helm
helm uninstall dell-csi-operator -n dell-csi-operator

# Or via OperatorHub (OpenShift)
oc delete subscription dell-csi-operator -n dell-csi-operator
oc delete namespace dell-csi-operator
```

---

## Troubleshooting

### Issue: Dell Operator Not Auto-Creating CR

**Symptom:**
```bash
kubectl get dellcsireplicationgroup -n dell-demo
# No resources found
```

**Solutions:**

1. **Check Dell operator version:**
   ```bash
   kubectl get deployment dell-csi-operator -n dell-csi-operator -o jsonpath='{.spec.template.spec.containers[0].image}'
   ```
   Some versions may not support auto-creation.

2. **Check StorageClass parameters:**
   ```bash
   kubectl get storageclass powerstore-replication -o yaml | grep -A 10 "parameters:"
   ```
   Ensure `replication.storage.dell.com/isReplicationEnabled: "true"` is set.

3. **Check Dell operator logs:**
   ```bash
   kubectl logs -n dell-csi-operator -l app=dell-csi-operator --tail=100 | grep -i "replication\|pvc\|error"
   ```

4. **Manually create Dell CR or use Unified operator:**
   The Unified operator will create the Dell CR if it doesn't exist.

### Issue: Unified Operator Not Creating Dell CR

**Symptom:**
```bash
kubectl get vr app-data-replication -n dell-demo
# Ready: False
# Reason: ReconcileError
```

**Solutions:**

1. **Check DellCSIReplicationGroup CRD exists:**
   ```bash
   kubectl get crd dellcsireplicationgroups.replication.dell.com
   ```

2. **Check operator RBAC:**
   ```bash
   kubectl get clusterrole unified-replication-operator-manager -o yaml | grep dellcsireplicationgroups
   ```

3. **Check operator logs:**
   ```bash
   kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=100 | grep -i "error\|dell\|powerstore"
   ```

4. **Verify VolumeReplicationClass parameters:**
   ```bash
   kubectl get vrc powerstore-replication-class -o yaml
   ```
   Ensure `protectionPolicy` and `remoteSystem` are set.

### Issue: Changes Not Propagating to Dell CR

**Symptom:**
VolumeReplication updated but Dell CR unchanged.

**Solutions:**

1. **Check operator is running:**
   ```bash
   kubectl get pods -n unified-replication-system
   ```

2. **Check reconciliation logs:**
   ```bash
   kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=100 | grep "app-data-replication"
   ```

3. **Force reconciliation:**
   ```bash
   # Annotate VolumeReplication to trigger reconciliation
   kubectl annotate vr app-data-replication -n dell-demo \
     unified-replication.io/force-reconcile=$(date +%s) --overwrite
   ```

4. **Check for conflicts:**
   ```bash
   # Check if Dell CR has conflicting owner references
   kubectl get dellcsireplicationgroup app-data-replication -n dell-demo -o yaml | grep -A 5 "ownerReferences:"
   ```

### Issue: PVC Not Binding

**Symptom:**
```bash
kubectl get pvc app-data-pvc -n dell-demo
# STATUS: Pending
```

**Solutions:**

1. **Check StorageClass exists:**
   ```bash
   kubectl get storageclass powerstore-replication
   ```

2. **Check Dell CSI driver:**
   ```bash
   kubectl get csidriver csi-powerstore.dellemc.com
   kubectl get pods -n dell-csi-operator | grep powerstore
   ```

3. **Check events:**
   ```bash
   kubectl describe pvc app-data-pvc -n dell-demo
   ```

---

## Key Takeaways

### 1. **Dell Operator Auto-Creation**
- Dell CSI operator can automatically create `DellCSIReplicationGroup` when PVC has replication parameters
- This is native Dell behavior and works independently

### 2. **Unified Operator Management**
- Unified operator creates `VolumeReplication` CRs using standard kubernetes-csi-addons API
- Operator detects Dell backend from `provisioner` field
- Automatically creates/updates `DellCSIReplicationGroup` based on `VolumeReplication`

### 3. **Change Propagation**
- Changes to `VolumeReplication` are automatically reflected in `DellCSIReplicationGroup`
- Unified operator translates states: `primary` → Dell actions/states
- Owner references ensure cleanup

### 4. **Unified API Benefits**
- Same `VolumeReplication` API works with Dell, Ceph, and Trident
- No need to learn Dell-specific CR structure
- Centralized management of all replication resources

---

## Summary

**What You Demonstrated:**

1. ✅ **Installed Dell CSI Operator** - PowerStore driver and replication support
2. ✅ **Installed Unified Replication Operator** - Multi-backend replication management
3. ✅ **Created StorageClass** - With Dell replication parameters
4. ✅ **Created PVC** - Dell operator auto-detected and created Dell CR
5. ✅ **Created VolumeReplication** - Unified operator created/managed Dell CR
6. ✅ **Updated VolumeReplication** - Changes propagated to Dell CR automatically
7. ✅ **Verified Integration** - Both operators working together seamlessly

**The Unified Replication Operator successfully integrates with Dell PowerStore backend!** 🎉

---

## Next Steps

- Try other backends: [Trident Demo](V2_TRIDENT_DEMO_GUIDE.md)
- Read [API Reference](../docs/api-reference/API_REFERENCE.md)
- Explore [Architecture Documentation](../docs/architecture/MIGRATION_ARCHITECTURE.md)
- Check [Troubleshooting Guide](../docs/user-guide/TROUBLESHOOTING.md)

