# v2.0.0-beta Trident Demo - Translation in Action

## Overview

This demo showcases the v2.0.0-beta kubernetes-csi-addons compatible API with **automatic translation** to NetApp Trident backend. You'll see how the operator translates standard kubernetes-csi-addons states to Trident-specific states.

**What You'll Learn:**
- How to use v1alpha2 `VolumeReplication` API
- How backend detection works (from provisioner)
- How translation works (primary → established, secondary → reestablishing)
- How to verify backend resources are created correctly

**Time:** ~10 minutes

---

## Prerequisites

- Kubernetes cluster with kubectl access
- Unified Replication Operator v2.0.0-beta installed
- NetApp Trident CSI driver installed (optional - can use mock)
- TridentMirrorRelationship CRD installed (optional for full demo)

**Don't have Trident?** The demo will still work - you'll see the operator create the TridentMirrorRelationship CR, which Trident would then act on.

---

## Demo Steps

### Step 1: Install Operator

```bash
# Via Helm
helm install unified-replication-operator \
  ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace

# Verify operator is running
kubectl get pods -n unified-replication-system
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=50
```

**Expected:** Operator pod running and ready

### Step 2: Create VolumeReplicationClass

```bash
# Apply the Trident replication class
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: trident-async-replication
spec:
  # This provisioner tells the operator to use the Trident adapter
  provisioner: csi.trident.netapp.io
  
  # Trident-specific parameters
  parameters:
    replicationPolicy: "Async"
    replicationSchedule: "15m"
    remoteCluster: "dr-cluster"
    remoteSVM: "svm-dr"
    remoteVolume: "remote-volume-handle"
EOF

# Verify class created
kubectl get volumereplicationclass
# or
kubectl get vrc
```

**Expected Output:**
```
NAME                        PROVISIONER                AGE
trident-async-replication   csi.trident.netapp.io      5s
```

**What Happened:**
- ✅ VolumeReplicationClass created (cluster-scoped)
- ✅ Operator now knows this is a Trident backend
- ✅ Parameters stored for use during replication

### Step 3: Create a PVC (or Use Existing)

```bash
# Create a test namespace
kubectl create namespace applications

# Create a PVC (or use existing)
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: application-data-pvc
  namespace: applications
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: trident-san  # Your Trident storage class
EOF

# Verify PVC
kubectl get pvc -n applications
```

### Step 4: Create VolumeReplication (Primary Site)

```bash
# Apply the VolumeReplication using kubernetes-csi-addons API
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: trident-app-replication
  namespace: applications
spec:
  volumeReplicationClass: trident-async-replication
  pvcName: application-data-pvc
  
  # Using standard kubernetes-csi-addons state: "primary"
  # The operator will TRANSLATE this to Trident state: "established"
  replicationState: primary
  
  autoResync: true
EOF

# Verify VolumeReplication created
kubectl get volumereplication -n applications
# or short form
kubectl get vr -n applications
```

**Expected Output:**
```
NAME                       STATE     PVC                    CLASS                       AGE
trident-app-replication    primary   application-data-pvc   trident-async-replication   5s
```

### Step 5: Verify Backend Translation

**This is where the magic happens!**

```bash
# Check that TridentMirrorRelationship was created
kubectl get tridentmirrorrelationship -n applications

# Get detailed view to see the TRANSLATED state
kubectl get tridentmirrorrelationship trident-app-replication -n applications -o yaml
```

**Expected in TridentMirrorRelationship:**
```yaml
apiVersion: trident.netapp.io/v1
kind: TridentMirrorRelationship
metadata:
  name: trident-app-replication
  namespace: applications
  ownerReferences:
  - apiVersion: replication.unified.io/v1alpha2
    kind: VolumeReplication
    name: trident-app-replication
    controller: true
spec:
  state: established              # ← TRANSLATED from "primary"!
  replicationPolicy: Async        # From VolumeReplicationClass
  replicationSchedule: "15m"      # From VolumeReplicationClass
  volumeMappings:
  - localPVCName: application-data-pvc
    remoteVolumeHandle: remote-volume-handle
```

**What to Verify:**
- ✅ `spec.state: established` (NOT "primary" - it was translated!)
- ✅ `spec.replicationPolicy: Async` (from class parameters)
- ✅ `spec.replicationSchedule: "15m"` (from class parameters)
- ✅ `ownerReferences` points to our VolumeReplication
- ✅ Backend CR has same name as VolumeReplication

### Step 6: Check VolumeReplication Status

```bash
# Check status of our VolumeReplication
kubectl describe vr trident-app-replication -n applications
```

**Expected Status:**
```yaml
Status:
  Conditions:
    Type:    Ready
    Status:  True
    Reason:  ReconcileComplete
    Message: Replication configured successfully
  State:     primary
  Observed Generation: 1
```

**What to Verify:**
- ✅ Ready condition is True
- ✅ State shows "primary" (our kubernetes-csi-addons input)
- ✅ No errors in conditions

### Step 7: Check Operator Logs (Translation Verification)

```bash
# View operator logs to see translation in action
kubectl logs -n unified-replication-system \
  -l control-plane=controller-manager \
  --tail=100 | grep -i "trident\|translation\|established"
```

**Expected Log Entries:**
```
INFO  trident-adapter  Reconciling VolumeReplication with Trident backend (with state translation)
INFO  trident-adapter  Translated state  vrState=primary  tridentState=established
INFO  trident-adapter  Successfully created/updated TridentMirrorRelationship with state translation
```

**What to Verify:**
- ✅ Logs show "Translated state"
- ✅ Shows `vrState=primary` → `tridentState=established`
- ✅ Shows successful creation

---

## Translation Verification

### The Translation Flow

```
User Input (kubernetes-csi-addons standard):
┌─────────────────────────────────────┐
│ VolumeReplication                   │
│ spec:                               │
│   replicationState: primary         │ ← Standard API
└─────────────────────────────────────┘
              ↓
      Operator Detects Backend
      (from provisioner: csi.trident.netapp.io)
              ↓
      Trident Adapter Translates
      primary → established
              ↓
Backend Output (Trident-specific):
┌─────────────────────────────────────┐
│ TridentMirrorRelationship           │
│ spec:                               │
│   state: established                │ ← Translated!
└─────────────────────────────────────┘
```

### Verify Translation

```bash
# 1. Check input (VolumeReplication)
kubectl get vr trident-app-replication -n applications -o jsonpath='{.spec.replicationState}'
# Output: primary

# 2. Check output (TridentMirrorRelationship)
kubectl get tridentmirrorrelationship trident-app-replication -n applications -o jsonpath='{.spec.state}'
# Output: established

# 3. Confirm translation happened
echo "Input: $(kubectl get vr trident-app-replication -n applications -o jsonpath='{.spec.replicationState}')"
echo "Output: $(kubectl get tridentmirrorrelationship trident-app-replication -n applications -o jsonpath='{.spec.state}')"
```

**Expected:**
```
Input: primary
Output: established
```

**✅ Translation working!**

---

## State Transition Demo

### Promote Secondary to Primary

On the secondary site, promote the replica to primary (failover scenario):

```bash
# Update state from secondary to primary
kubectl patch vr trident-app-replication -n applications \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'

# Watch the translation happen
kubectl get tridentmirrorrelationship trident-app-replication -n applications -o yaml | grep "state:"
```

**Before patch:**
```yaml
spec:
  state: reestablishing    # Was secondary
```

**After patch:**
```yaml
spec:
  state: established       # Now primary (translated!)
```

**Translation:** `primary` → `established` ✅

### Demote Primary to Secondary

```bash
# Demote back to secondary
kubectl patch vr trident-app-replication -n applications \
  --type merge \
  -p '{"spec":{"replicationState":"secondary"}}'

# Verify translation
kubectl get tridentmirrorrelationship trident-app-replication -n applications -o jsonpath='{.spec.state}'
# Output: reestablishing
```

**Translation:** `secondary` → `reestablishing` ✅

### Force Resync

```bash
# Force resynchronization
kubectl patch vr trident-app-replication -n applications \
  --type merge \
  -p '{"spec":{"replicationState":"resync"}}'

# Check translation
kubectl get tridentmirrorrelationship trident-app-replication -n applications -o jsonpath='{.spec.state}'
# Output: reestablishing
```

**Translation:** `resync` → `reestablishing` ✅

---

## Verification Commands

### Check All Resources

```bash
# List all v1alpha2 resources
kubectl get vr,vrc -A

# Check VolumeReplicationClass (cluster-scoped)
kubectl get vrc
kubectl describe vrc trident-async-replication

# Check VolumeReplication
kubectl get vr -n applications
kubectl describe vr trident-app-replication -n applications

# Check backend TridentMirrorRelationship
kubectl get tridentmirrorrelationship -n applications
kubectl describe tridentmirrorrelationship trident-app-replication -n applications
```

### Compare Input vs Output

```bash
# Side-by-side comparison
echo "=== VolumeReplication (Input) ==="
kubectl get vr trident-app-replication -n applications -o yaml | grep -A 5 "spec:"

echo ""
echo "=== TridentMirrorRelationship (Output) ==="
kubectl get tridentmirrorrelationship trident-app-replication -n applications -o yaml | grep -A 10 "spec:"
```

**You'll see:**
- Input uses kubernetes-csi-addons standard (`primary`, `secondary`, `resync`)
- Output uses Trident-specific (`established`, `reestablishing`)
- Translation is automatic and bidirectional

---

## Cleanup

```bash
# Delete VolumeReplication
kubectl delete vr trident-app-replication -n applications

# Verify backend CR is also deleted (owner reference)
kubectl get tridentmirrorrelationship -n applications
# Should be empty - automatic cleanup!

# Delete class
kubectl delete vrc trident-async-replication

# Delete namespace (optional)
kubectl delete namespace applications
```

**What Happens:**
1. VolumeReplication deleted
2. Operator detects deletion (finalizer)
3. Operator deletes TridentMirrorRelationship
4. Finalizer removed
5. VolumeReplication deleted
6. **Clean cleanup!** ✅

---

## Key Takeaways

### 1. Standard API Works

You used kubernetes-csi-addons standard `VolumeReplication` API:
```yaml
spec:
  volumeReplicationClass: trident-async-replication
  pvcName: application-data-pvc
  replicationState: primary  # Standard!
```

**NOT** Trident-specific API!

### 2. Automatic Backend Detection

Operator detected Trident from:
```yaml
VolumeReplicationClass:
  spec:
    provisioner: csi.trident.netapp.io  # ← This triggers Trident adapter
```

### 3. Automatic Translation

| Your Input (standard) | Trident Output (translated) |
|-----------------------|-----------------------------|
| `primary` | `established` |
| `secondary` | `reestablishing` |
| `resync` | `reestablishing` |

**You never had to know Trident states!**

### 4. Owner References

Backend CR owned by VolumeReplication:
- Automatic cleanup when you delete
- Kubernetes garbage collection
- No orphaned resources

### 5. Same API, Different Backend

**Want to use Ceph instead?**
Just change the `volumeReplicationClass`:
```yaml
spec:
  volumeReplicationClass: ceph-replication  # ← That's it!
  pvcName: application-data-pvc
  replicationState: primary
```

**Want to use Dell PowerStore?**
```yaml
spec:
  volumeReplicationClass: powerstore-replication  # ← That's it!
  pvcName: application-data-pvc
  replicationState: primary
```

**Same VolumeReplication API for all backends!**

---

## Troubleshooting

### Issue: VolumeReplicationClass Not Found

**Symptom:**
```
Ready: False
Reason: VolumeReplicationClassNotFound
```

**Solution:**
```bash
# Check class exists
kubectl get vrc

# Create if missing
kubectl apply -f demo/v2-trident-demo.yaml
```

### Issue: Backend Not Detected

**Symptom:**
```
Ready: False
Reason: UnknownBackend
Message: unable to detect backend from provisioner: unknown
```

**Solution:**
- Verify provisioner in VolumeReplicationClass
- Must contain "trident" or "netapp" or be "csi.trident.netapp.io"
- Check for typos

### Issue: TridentMirrorRelationship Not Created

**Symptom:**
- VolumeReplication shows Ready: True
- But no TridentMirrorRelationship exists

**Check:**
```bash
# Check operator logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=100

# Look for errors in reconciliation
```

**Common Causes:**
- TridentMirrorRelationship CRD not installed
- Operator doesn't have RBAC permissions
- Error in adapter (check logs)

---

## Advanced: Volume Group Demo

Want to replicate multiple PVCs together for a multi-volume app?

```bash
# Create VolumeGroupReplicationClass
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplicationClass
metadata:
  name: trident-group-replication
spec:
  provisioner: csi.trident.netapp.io
  parameters:
    replicationPolicy: "Async"
    groupReplicationSchedule: "15m"
    consistencyGroupPolicy: "cg-async-policy"
EOF

# Create VolumeGroupReplication
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplication
metadata:
  name: app-volume-group
  namespace: applications
spec:
  volumeGroupReplicationClass: trident-group-replication
  
  # Select multiple PVCs via labels
  selector:
    matchLabels:
      app: myapp
      tier: backend
  
  replicationState: primary
  autoResync: true
EOF

# Check group status
kubectl describe vgr app-volume-group -n applications

# See which PVCs are in the group
kubectl get vgr app-volume-group -n applications -o jsonpath='{.status.persistentVolumeClaimsRefList[*].name}'
```

**Result:**
- ✅ Single TridentMirrorRelationship created
- ✅ `volumeMappings` array contains all PVCs
- ✅ All volumes replicated as a group
- ✅ Crash-consistent snapshots

---

## Historical Note: v1alpha1 vs v1alpha2

**Note:** v1alpha1 has been removed from the operator. The following is provided for historical reference only.

### v1alpha1 (Removed - Was Complex)

```yaml
# THIS API HAS BEEN REMOVED - DO NOT USE
# apiVersion: replication.unified.io/v1alpha1
# kind: UnifiedVolumeReplication
metadata:
  name: trident-replication
spec:
  sourceEndpoint:
    cluster: "primary"
    region: "us-east-1"
    storageClass: "trident-san"
  destinationEndpoint:
    cluster: "dr"
    region: "us-west-1"
    storageClass: "trident-san"
  volumeMapping:
    source:
      pvcName: "app-data"
      namespace: "applications"
    destination:
      volumeHandle: "remote-handle"
      namespace: "dr"
  replicationState: "source"     # Custom state name
  replicationMode: "asynchronous"
  schedule:
    rpo: "15m"
    mode: "continuous"
```

**Issues:**
- ❌ Complex (7 top-level fields)
- ❌ Custom state names (source/replica)
- ❌ Not kubernetes-csi-addons compatible

### v1alpha2 (New - Simple!)

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: trident-replication
spec:
  provisioner: csi.trident.netapp.io
  parameters:
    replicationPolicy: "Async"
    replicationSchedule: "15m"
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: trident-replication
  namespace: applications
spec:
  volumeReplicationClass: trident-replication
  pvcName: app-data
  replicationState: primary  # Standard state name
```

**Benefits:**
- ✅ Simple (3 required fields)
- ✅ Standard state names (primary/secondary/resync)
- ✅ kubernetes-csi-addons compatible
- ✅ Separation of concerns (class vs instance)

---

## Summary

**What You Demonstrated:**

1. ✅ **kubernetes-csi-addons API** - Used standard VolumeReplication
2. ✅ **Backend Detection** - Operator detected Trident from provisioner
3. ✅ **State Translation** - primary → established automatically
4. ✅ **Backend CR Creation** - TridentMirrorRelationship created
5. ✅ **Owner References** - Automatic cleanup
6. ✅ **Simple API** - Only 3 required fields

**Translation Verified:**
- primary → established ✅
- secondary → reestablishing ✅
- resync → reestablishing ✅

**The operator successfully translates kubernetes-csi-addons standard API to Trident-specific CRs!**

---

## Next Steps

### Try Other Backends

**Ceph (Passthrough - No Translation):**
```bash
kubectl apply -f config/samples/volumereplicationclass_ceph.yaml
kubectl apply -f config/samples/volumereplication_ceph_primary.yaml
```

**Dell PowerStore (Action Translation):**
```bash
kubectl apply -f config/samples/volumereplicationclass_powerstore.yaml
kubectl apply -f config/samples/volumereplication_powerstore_primary.yaml
```

### Try Volume Groups

```bash
kubectl apply -f config/samples/volumegroupreplicationclass_ceph_group.yaml
kubectl apply -f config/samples/volumegroupreplication_postgresql.yaml
```

### Read Documentation

- **API Reference:** `docs/api-reference/API_REFERENCE.md`
- **Quick Start:** `QUICK_START.md`
- **Architecture:** `docs/architecture/MIGRATION_ARCHITECTURE.md`

---

## Demo Complete! 🎉

You've successfully:
- ✅ Used kubernetes-csi-addons compatible API
- ✅ Seen automatic backend detection
- ✅ Verified state translation (primary → established)
- ✅ Confirmed backend CR creation
- ✅ Validated the v2.0.0-beta operator!

**The operator makes it easy to use standard APIs while supporting multiple storage backends!** 🚀

