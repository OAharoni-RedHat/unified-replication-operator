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

## Installing Trident

If you don't have Trident installed, follow these steps:

### Option 1: Install Trident via OperatorHub (OpenShift)

**For OpenShift clusters:**

```bash
# Install Trident Operator from OperatorHub
oc create -f - <<EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: trident-operator
  namespace: openshift-operators
spec:
  channel: stable
  name: trident-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF

# Wait for operator to be ready
oc wait --for=condition=Installed subscription/trident-operator -n openshift-operators --timeout=5m

# Create Trident instance
oc create -f - <<EOF
apiVersion: trident.netapp.io/v1
kind: TridentOrchestrator
metadata:
  name: trident
spec:
  debug: false
EOF

# Verify Trident is running
oc get pods -n trident
```

### Verify Trident Installation

After installation, verify everything is working:

```bash
# Check Trident pods are running
kubectl get pods

# Expected output:
# NAME                                READY   STATUS    RESTARTS   AGE
# trident-operator-xxxxxxxxx-xxxxx     1/1     Running   0          2m
# trident-csi-xxxxxxxxx-xxxxx         2/2     Running   0          2m

# Check Trident CRDs are installed
kubectl get crd | grep trident

# Expected CRDs:
# - tridentmirrorrelationships.trident.netapp.io
# - tridentactionmirrorupdates.trident.netapp.io
# - tridentbackendconfigs.trident.netapp.io
# - tridentstoragesets.trident.netapp.io
# - etc.

# Check Trident version
kubectl exec -n trident deployment/trident-operator -- tridentctl version
```

### Install TridentMirrorRelationship CRD (If Missing)

If Trident is installed but the `TridentMirrorRelationship` CRD is missing:

```bash
# Check if CRD exists
kubectl get crd tridentmirrorrelationships.trident.netapp.io

# If not found, you have several options to install it:
```

**Option 1: Restart Trident Operator (Simplest)**

The CRD should be installed automatically by the Trident operator. Try restarting it:

```bash
kubectl rollout restart deployment/trident-operator -n trident
kubectl wait --for=condition=ready pod -n trident -l app=trident-operator --timeout=5m

# Wait a moment for CRDs to be installed, then check
sleep 10
kubectl get crd tridentmirrorrelationships.trident.netapp.io
```

**Option 2: Download from Trident Release**

```bash
# Download a specific Trident release (replace v23.10.0 with your version)
TRIDENT_VERSION="v23.10.0"
curl -sL https://github.com/NetApp/trident/archive/refs/tags/${TRIDENT_VERSION}.tar.gz | tar -xz
find trident-${TRIDENT_VERSION#v} -name "*tridentmirror*" -type f | head -1 | xargs kubectl apply -f
rm -rf trident-${TRIDENT_VERSION#v}

# Or browse releases manually:
# Visit: https://github.com/NetApp/trident/releases
# Download the release tarball, extract, and find CRD in deploy/crds/
```

**Option 3: Extract from Trident Operator Pod**

```bash
# Try to find CRD files in the operator pod
POD_NAME=$(kubectl get pods -n trident -l app=trident-operator -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n trident $POD_NAME -- find / -name "*mirror*" -type f 2>/dev/null | head -1 | \
  xargs -I {} kubectl exec -n trident $POD_NAME -- cat {} | kubectl apply -f -
```

**Option 4: Check Trident Documentation**

The CRD definition may be available in Trident's official documentation or installation manifests. Check:
- Trident installation guide
- Trident Helm chart CRDs
- Your Trident operator deployment manifests

```bash
# Verify CRD is installed
kubectl get crd tridentmirrorrelationships.trident.netapp.io

# If still not found, check Trident operator logs for CRD installation status
kubectl logs -n trident -l app=trident-operator | grep -i "crd\|mirror"
```

**Note:** The `TridentMirrorRelationship` CRD is typically installed automatically when Trident operator starts. If it's missing, it may indicate:
- Trident version doesn't support mirror relationships (requires Trident 23.04+)
- Trident operator hasn't fully initialized
- CRD installation failed

Try restarting the Trident operator if the CRD should be present:
```bash
kubectl rollout restart deployment/trident-operator -n trident
kubectl wait --for=condition=ready pod -n trident -l app=trident-operator --timeout=5m
```

### Troubleshooting Trident Installation

**Issue: Trident pods not starting**

```bash
# Check pod logs
kubectl logs -n trident -l app=trident-operator --tail=50

# Check events
kubectl get events -n trident --sort-by='.lastTimestamp'
```

**Issue: CRDs not found**

```bash
# Verify namespace exists
kubectl get namespace trident

# Check if operator is running
kubectl get pods -n trident

# Reinstall CRDs if needed
helm upgrade trident netapp-trident/trident-operator -n trident
```

**Issue: Storage backend not configured**

Trident installation only installs the operator. You still need to configure storage backends:

```bash
# Check available backends
kubectl get tridentbackendconfigs -n trident

# Create a backend configuration (example for ONTAP)
kubectl apply -f - <<EOF
apiVersion: trident.netapp.io/v1
kind: TridentBackendConfig
metadata:
  name: ontap-backend
  namespace: trident
spec:
  version: 1
  storageDriverName: ontap-san
  managementLIF: <your-ontap-mgmt-ip>
  dataLIF: <your-ontap-data-ip>
  svm: <your-svm-name>
  username: <your-username>
  password: <your-password>
  storagePrefix: trident
EOF
```

**For more details, see:** [Trident Documentation](https://netapp-trident.readthedocs.io/)

---

## Demo Steps

### Step 1: Install CRDs

**CRDs must be installed before creating any VolumeReplication resources.**

```bash
# Install CRDs from the config directory
kubectl apply -f config/crd/bases/replication.unified.io_volumereplicationclasses.yaml
kubectl apply -f config/crd/bases/replication.unified.io_volumereplications.yaml
kubectl apply -f config/crd/bases/replication.unified.io_volumegroupreplicationclasses.yaml
kubectl apply -f config/crd/bases/replication.unified.io_volumegroupreplications.yaml

# Or install all CRDs at once
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

**Note:** If using Helm, CRDs may be installed automatically. Check with:
```bash
kubectl get crd | grep replication.unified.io
```

If the CRDs are already installed, you can skip this step.

### Step 2: Build and Deploy Operator

**Use the build script to build, push, and deploy the operator in one step.**

```bash
export KUBECONFIG=/home/oaharoni/aws-gpfs-playground/ocp_install_files/auth/kubeconfig

# 1. Expose OpenShift internal registry (if not already done)
oc patch configs.imageregistry.operator.openshift.io/cluster --type merge -p '{"spec":{"defaultRoute":true}}'

# 2. Get registry URL and login
REGISTRY=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
TOKEN=$(oc whoami -t)
podman login -u $(oc whoami) -p $TOKEN $REGISTRY --tls-verify=false

# 3. Build, push, and deploy (all in one command!)
cd /home/oaharoni/github_workspaces/replication_extensions/unified-replication-operator
REGISTRY=$REGISTRY/unified-replication-system VERSION=2.0.0-beta ./scripts/build-and-push.sh
```

The script will:
- ✅ Run tests
- ✅ Build operator binary
- ✅ Build container image
- ✅ Push to registry
- ✅ Install CRDs
- ✅ Deploy operator via Helm with OpenShift-compatible settings


### Step 3: Create VolumeReplicationClass

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

### Step 4: Create a PVC (or Use Existing)

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

### Step 5: Create VolumeReplication (Primary Site)

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

### Step 6: Verify Backend Translation

**This is where the magic happens!**

```bash
# First, verify VolumeReplication is ready
kubectl get vr trident-app-replication -n applications
kubectl describe vr trident-app-replication -n applications

# Wait a moment for operator to reconcile (if needed)
sleep 5

# Check that TridentMirrorRelationship was created
kubectl get tridentmirrorrelationship -n applications

# If not found in applications namespace, check all namespaces
kubectl get tridentmirrorrelationship -A

# Also check default namespace (sometimes created there)
kubectl get tridentmirrorrelationship -n default

# If still not found, check if operator is running first
kubectl get pods -n unified-replication-system

# If operator is running, check logs for errors
# (Replace <pod-name> with actual pod name if label selector doesn't work)
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=100 | grep -i "trident\|error\|failed" || \
kubectl logs -n unified-replication-system $(kubectl get pods -n unified-replication-system -o name | head -1) --tail=100 | grep -i "trident\|error\|failed"

# Get detailed view to see the TRANSLATED state (once created)
kubectl get tridentmirrorrelationship trident-app-replication -n applications -o yaml
```

**Quick Diagnostic:**

If TridentMirrorRelationship is not found, run this diagnostic:

```bash
# Check VolumeReplication status and conditions
kubectl get vr trident-app-replication -n applications -o yaml | grep -A 20 "status:"

# Check for error conditions
kubectl get vr trident-app-replication -n applications -o jsonpath='{.status.conditions[*].reason}' && echo

# Check if operator is running
kubectl get pods -n unified-replication-system

# Check operator logs for this specific resource
# (Try label selector first, fallback to pod name if needed)
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=200 2>/dev/null | grep "trident-app-replication" || \
kubectl logs -n unified-replication-system $(kubectl get pods -n unified-replication-system -o jsonpath='{.items[0].metadata.name}' 2>/dev/null) --tail=200 2>/dev/null | grep "trident-app-replication" || \
echo "Operator not running or no logs found. Install operator first (see Step 2)."
```

**If TridentMirrorRelationship is not created, see troubleshooting section below.**

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

### Step 7: Check VolumeReplication Status

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

### Step 8: Check Operator Logs (Translation Verification)

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

### Quick Cleanup (Demo Resources Only)

Clean up all demo resources while keeping the operator installed:

```bash
# From the repo root
./scripts/cleanup-demo.sh
```

This removes:
- ✅ All VolumeReplication resources
- ✅ All VolumeGroupReplication resources
- ✅ All VolumeReplicationClass resources
- ✅ All VolumeGroupReplicationClass resources
- ✅ Backend-specific resources (TridentMirrorRelationship, etc.)

### Complete Cleanup (Including Operator)

Remove everything including the operator:

```bash
# From the repo root
./scripts/cleanup-demo.sh --operator
```

This removes everything above plus:
- ✅ Helm release
- ✅ Operator namespace
- ✅ CRDs
- ✅ RBAC resources
- ✅ Webhooks

### Manual Cleanup

If you prefer to clean up manually:

```bash
# Delete VolumeReplication
kubectl delete vr trident-app-replication -n applications

# Delete VolumeReplicationClass
kubectl delete vrc trident-async-replication

# Verify backend CR is also deleted (owner reference)
kubectl get tridentmirrorrelationship -n applications
# Should be empty - automatic cleanup!

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

### Issue: Helm Installation Fails - ClusterRole Ownership Error

**Symptom:**
```
Error: INSTALLATION FAILED: Unable to continue with install: ClusterRole "unified-replication-operator-manager" 
exists and cannot be imported into the current release: invalid ownership metadata
```

**Solution:**

This happens when the operator was previously installed in a different namespace. Clean up and reinstall:

```bash
# Clean up existing installation
helm uninstall unified-replication-operator -n default 2>/dev/null || true
helm uninstall unified-replication-operator -n unified-replication-system 2>/dev/null || true
kubectl delete clusterrole unified-replication-operator-manager 2>/dev/null || true
kubectl delete clusterrolebinding unified-replication-operator-manager 2>/dev/null || true

# Reinstall using build script (recommended)
REGISTRY=your-registry VERSION=2.0.0-beta ./scripts/build-and-push.sh
```

The build script handles cleanup and deployment automatically.

### Issue: ImagePullBackOff - Operator Image Not Found

**Symptom:**
```
NAME                                            READY   STATUS             RESTARTS   AGE
unified-replication-operator-xxx   0/1     ImagePullBackOff   0          7s
```

**Solution:**

The operator image needs to be built and pushed before deployment. Follow Step 2 above to build the image.

**Quick Fix:**

Use the build script - it handles everything automatically:

```bash
export KUBECONFIG=/home/oaharoni/aws-gpfs-playground/ocp_install_files/auth/kubeconfig

# For OpenShift
REGISTRY=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
TOKEN=$(oc whoami -t)
podman login -u $(oc whoami) -p $TOKEN $REGISTRY --tls-verify=false

REGISTRY=$REGISTRY/unified-replication-system VERSION=2.0.0-beta ./scripts/build-and-push.sh

# For external registry
REGISTRY=quay.io/YOUR_USERNAME VERSION=2.0.0-beta ./scripts/build-and-push.sh
```

The script will rebuild the image and redeploy automatically.

### Issue: CRDs Not Installed

**Symptom:**
```
error: resource mapping not found for name: "trident-async-replication" namespace: "" from "STDIN": 
no matches for kind "VolumeReplicationClass" in version "replication.unified.io/v1alpha2"
ensure CRDs are installed first
```

**Solution:**
```bash
# Install CRDs first (see Step 1 above)
kubectl apply -f config/crd/bases/

# Verify CRDs are installed
kubectl get crd | grep replication.unified.io

# You should see:
# - volumereplicationclasses.replication.unified.io
# - volumereplications.replication.unified.io
# - volumegroupreplicationclasses.replication.unified.io
# - volumegroupreplications.replication.unified.io
```

**Note:** Helm may or may not install CRDs automatically depending on chart configuration. Always verify CRDs are installed before creating resources.

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
```
kubectl get tridentmirrorrelationship -n applications
# No resources found

# Or VolumeReplication shows error:
# Ready: False
# Reason: ReconcileError
# Message: no matches for kind "TridentMirrorRelationship" in version "trident.netapp.io/v1"
```

**Root Cause:**
The `TridentMirrorRelationship` CRD is not installed. This CRD is required for Trident mirror relationships and may not be included in all Trident installations.

**Diagnostic Steps:**

```bash
# 1. Check if TridentMirrorRelationship CRD is installed
kubectl get crd tridentmirrorrelationships.trident.netapp.io

# If not found, you'll see:
# Error from server (NotFound): customresourcedefinitions.apiextensions.k8s.io "tridentmirrorrelationships.trident.netapp.io" not found
```

**Solution:**

```bash
# Option 1: Restart Trident operator (CRD should install automatically)
kubectl rollout restart deployment/trident-operator -n trident
kubectl wait --for=condition=ready pod -n trident -l app=trident-operator --timeout=5m
sleep 10  # Wait for CRDs to be installed
kubectl get crd tridentmirrorrelationships.trident.netapp.io

# Option 2: Download from Trident release (replace version as needed)
TRIDENT_VERSION="v23.10.0"
curl -sL https://github.com/NetApp/trident/archive/refs/tags/${TRIDENT_VERSION}.tar.gz | tar -xz
find trident-${TRIDENT_VERSION#v} -name "*tridentmirror*" -type f | head -1 | xargs kubectl apply -f
rm -rf trident-${TRIDENT_VERSION#v}

# Option 3: Extract from operator pod
POD_NAME=$(kubectl get pods -n trident -l app=trident-operator -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n trident $POD_NAME -- find / -name "*mirror*" -type f 2>/dev/null | head -1 | \
  xargs -I {} kubectl exec -n trident $POD_NAME -- cat {} | kubectl apply -f -

# Option 4: Upgrade Trident (mirror relationships require Trident 23.04+)
helm upgrade trident netapp-trident/trident-operator -n trident

# Verify CRD is installed
kubectl get crd tridentmirrorrelationships.trident.netapp.io
```

**After installing the CRD, the operator will automatically retry and create the TridentMirrorRelationship.**

**Additional Diagnostic Steps:**

```bash
# 2. Check VolumeReplication status
kubectl get vr trident-app-replication -n applications -o yaml
kubectl describe vr trident-app-replication -n applications

# Look for:
# - Ready condition status
# - Error messages in conditions
# - Events

# 3. Check if operator is running
kubectl get pods -n unified-replication-system

# If no pods found, operator may not be installed. Install it first:
# helm install unified-replication-operator ./helm/unified-replication-operator \
#   --namespace unified-replication-system --create-namespace

# Once operator is running, check logs for errors
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=200 | grep -i "trident\|error\|failed"

# Or if the label selector doesn't work, try:
kubectl get pods -n unified-replication-system
# Then use the pod name directly:
# kubectl logs -n unified-replication-system <pod-name> --tail=200 | grep -i "trident\|error\|failed"

# 4. Verify operator has RBAC permissions
kubectl get clusterrole unified-replication-operator-manager -o yaml | grep -A 10 tridentmirrorrelationships

# 5. Check if VolumeReplication is being reconciled
kubectl get events -n applications --sort-by='.lastTimestamp' | grep trident-app-replication
```

**Common Causes & Solutions:**

1. **TridentMirrorRelationship CRD not installed:**
   ```bash
   kubectl get crd tridentmirrorrelationships.trident.netapp.io
   # If not found, install Trident (see installation section above)
   ```

2. **Operator RBAC permissions missing:**
   ```bash
   # Check ClusterRole has tridentmirrorrelationships permissions
   kubectl get clusterrole unified-replication-operator-manager -o yaml | grep tridentmirrorrelationships
   
   # If missing, reinstall operator or check Helm values
   helm upgrade unified-replication-operator ./helm/unified-replication-operator -n unified-replication-system
   ```

3. **VolumeReplication not ready:**
   ```bash
   # Check VolumeReplication conditions
   kubectl get vr trident-app-replication -n applications -o jsonpath='{.status.conditions}' | jq '.'
   
   # If Ready=False, check the reason and message
   kubectl describe vr trident-app-replication -n applications | grep -A 5 "Conditions:"
   ```

4. **Operator not running or not reconciling:**
   ```bash
   # First, verify operator is installed and running
   kubectl get pods -n unified-replication-system
   
   # If no pods found, rebuild and redeploy using build script
   REGISTRY=your-registry VERSION=2.0.0-beta ./scripts/build-and-push.sh
   
   # Check operator logs show reconciliation attempts
   kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=100 | grep "trident-app-replication"
   
   # Should see logs like:
   # "Reconciling VolumeReplication"
   # "Translated state"
   # "Successfully created/updated TridentMirrorRelationship"
   ```

5. **Namespace mismatch:**
   ```bash
   # Verify VolumeReplication namespace matches where you're looking
   kubectl get vr trident-app-replication -n applications -o jsonpath='{.metadata.namespace}'
   
   # TridentMirrorRelationship is created in the same namespace as VolumeReplication
   ```

6. **PVC not found or not bound:**
   ```bash
   # Verify PVC exists and is bound
   kubectl get pvc application-data-pvc -n applications
   
   # If PVC doesn't exist or is Pending, TridentMirrorRelationship may not be created
   ```

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

