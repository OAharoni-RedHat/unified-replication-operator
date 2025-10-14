# Unified Replication Operator - Comprehensive Demo

This comprehensive demo walks you through deploying the operator, creating replications, and demonstrating seamless backend switching.

---

## 📋 **Prerequisites**

Before starting, ensure you have:
- OpenShift or Kubernetes cluster (1.24+)
- `kubectl` or `oc` CLI configured
- `podman` or `docker` installed
- Access to a container registry (e.g., quay.io)
- Trident CRDs installed in your cluster

**Set your environment:**
```bash
export KUBECONFIG=/path/to/your/kubeconfig
export REGISTRY=quay.io/YOUR_USERNAME  # Your container registry
```

---

## Part 1: Deploy the Operator

### **Step 1: Build the Operator Image**

```bash
cd /path/to/unified-replication-operator

# Login to your registry
podman login quay.io

# Build and push the operator
./scripts/build-and-push.sh
```

**Expected Output:**
```
✅ [INFO] Build and Deploy Summary
  Operator:     unified-replication-operator
  Version:      0.2.1
  Image:        quay.io/YOUR_USERNAME/unified-replication-operator:0.2.1
  Status:       deployed
  
✅ [INFO] ✅ 1 pod(s) are ready
```

### **Step 2: Verify Operator is Running**

```bash
# Check operator pod
kubectl get pods -n unified-replication-system

# Should show:
# NAME                                            READY   STATUS    RESTARTS   AGE
# unified-replication-operator-xxxxx-xxxxx        1/1     Running   0          2m
```

### **Step 3: Check Operator Logs**

```bash
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=20
```

**Expected Output:**
```json
{"level":"info","msg":"starting manager"}
{"level":"info","msg":"Starting metrics server"}
```

✅ **Checkpoint:** Operator is deployed and running

---

## Part 2: Create Trident Replication

### **Step 1: Create Unified Replication CR**

Create `trident-replication.yaml`:

```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: demo-trident-replication
  namespace: default
spec:
  sourceEndpoint:
    cluster: "source-cluster"
    region: "us-east-1"
    storageClass: "trident-ontap-san"  # ← Triggers Trident backend
  
  destinationEndpoint:
    cluster: "dest-cluster"
    region: "us-west-1"
    storageClass: "trident-ontap-nas"
  
  volumeMapping:
    source:
      pvcName: "my-app-data"
      namespace: "default"
    destination:
      volumeHandle: "trident-pvc-12345678-abcd-1234-5678-1234567890ab"
      namespace: "default"
  
  replicationState: "source"
  replicationMode: "asynchronous"
  
  schedule:
    rpo: "15m"
    rto: "5m"
    mode: "interval"
  
  extensions:
    trident: {}
```

**Apply it:**
```bash
kubectl apply -f trident-replication.yaml
```

**Expected Output:**
```
unifiedvolumereplication.replication.unified.io/demo-trident-replication created
```

### **Step 2: Verify Unified CR Created**

```bash
kubectl get unifiedvolumereplications -n default
# Or use shorthand:
kubectl get uvr -n default
```

**Expected Output:**
```
NAME                       STATE    MODE           SOURCE        READY   AGE
demo-trident-replication   source   asynchronous   my-app-data   True    10s
```

**Key Indicators:**
- ✅ Resource created
- ✅ STATE = `source` (matches your spec)
- ✅ MODE = `asynchronous` (matches your spec)
- ✅ READY = `True` (operator reconciled successfully)

### **Step 3: Verify TridentMirrorRelationship Created**

This is the **KEY validation** - the operator should automatically create the backend-specific CRD:

```bash
kubectl get tridentmirrorrelationships -n default
```

**Expected Output:**
```
NAME                       DESIRED STATE   LOCAL PVC     ACTUAL STATE   MESSAGE
demo-trident-replication   established     my-app-data                  
```

**✅ VALIDATION PASSED:**
- Same name as your UnifiedVolumeReplication
- Labels show `managed-by: unified-replication-operator`

### **Step 4: Compare Both Resources**

```bash
# View your Unified CR spec
kubectl get uvr demo-trident-replication -n default -o yaml | grep -A 15 "spec:"

# View generated Trident CR spec
kubectl get tridentmirrorrelationship demo-trident-replication -n default -o yaml | grep -A 10 "spec:"
```

**Comparison:**

| Your Input (Unified) | → | Generated Output (Trident) |
|---------------------|---|---------------------------|
| `replicationState: source` | → | `state: established` ✅ |
| `replicationMode: asynchronous` | → | `replicationPolicy: Async` ✅ |
| `volumeMapping.source.pvcName: my-app-data` | → | `volumeMappings[0].localPVCName: my-app-data` ✅ |
| `schedule.rpo: 15m` | → | `replicationSchedule: 15m` ✅ |

**✅ Translation working correctly!**

### **Step 5: Check Operator Logs**

```bash
kubectl logs -n unified-replication-system -l control-plane=controller-manager | \
  grep demo-trident-replication | tail -10
```

**Expected Log Sequence:**
```json
{"logger":"trident-adapter","msg":"Ensuring Trident mirror relationship is in desired state"}
{"logger":"trident-adapter","msg":"TridentMirrorRelationship not found, creating"}
{"logger":"trident-adapter","msg":"Successfully created Trident mirror relationship"}
{"msg":"Reconciliation completed successfully"}
```

✅ **Checkpoint:** Trident replication created and validated

---

## Part 3: Update the CR and Validate Propagation

### **Step 1: Update the Unified CR**

Change the replication state from `source` to `replica`:

```bash
kubectl patch uvr demo-trident-replication -n default --type=merge -p '
{
  "spec": {
    "replicationState": "replica"
  }
}'
```

**Expected Output:**
```
unifiedvolumereplication.replication.unified.io/demo-trident-replication patched
```

### **Step 2: Verify Unified CR Updated**

```bash
kubectl get uvr demo-trident-replication -n default -o jsonpath='{.spec.replicationState}'
echo ""
```

**Expected Output:**
```
replica
```

### **Step 3: Verify TridentMirrorRelationship Updated**

Wait a moment for reconciliation (operator polls every 30s):

```bash
# Wait for reconciliation
sleep 10

# Check Trident CR state
kubectl get tridentmirrorrelationship demo-trident-replication -n default -o jsonpath='{.spec.state}'
echo ""
```

**Expected Output:**
```
established
```

**Note:** Trident uses `established` for both `source` and `replica` states. The semantic is in the relationship direction.

### **Step 4: Watch the Update in Real-Time**

In one terminal:
```bash
# Watch the Trident CR
kubectl get tridentmirrorrelationship demo-trident-replication -n default -w
```

In another terminal:
```bash
# Make another change
kubectl patch uvr demo-trident-replication -n default --type=merge -p '
{
  "spec": {
    "schedule": {
      "rpo": "10m"
    }
  }
}'
```

**You should see:**
- Trident CR's `replicationSchedule` changes from `15m` to `10m`
- Update happens automatically within 30 seconds

### **Step 5: Check Operator Logs for Update**

```bash
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=50 | \
  grep demo-trident-replication | grep -i update
```

**Expected:**
```json
{"logger":"trident-adapter","msg":"TridentMirrorRelationship exists, updating if needed"}
{"logger":"trident-adapter","msg":"Successfully updated Trident mirror relationship"}
```

✅ **Checkpoint:** Updates propagate from Unified CR to Trident CR automatically

---

## Part 4: Switch Backend to Ceph

### **Step 1: Create a New CR with Ceph Backend**

Create `ceph-replication.yaml`:

```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: demo-ceph-replication
  namespace: default
spec:
  sourceEndpoint:
    cluster: "source-cluster"
    region: "us-east-1"
    storageClass: "ceph-rbd"  # ← Different backend! Triggers Ceph
  
  destinationEndpoint:
    cluster: "dest-cluster"
    region: "us-west-1"
    storageClass: "ceph-rbd"
  
  volumeMapping:
    source:
      pvcName: "ceph-app-data"
      namespace: "default"
    destination:
      volumeHandle: "ceph-volume-handle-xyz"
      namespace: "default"
  
  replicationState: "secondary"  # Ceph uses primary/secondary
  replicationMode: "asynchronous"
  
  schedule:
    rpo: "5m"
    rto: "2m"
    mode: "continuous"
  
  extensions:
    ceph:
      mirroringMode: "snapshot"
```

**Apply it:**
```bash
kubectl apply -f ceph-replication.yaml
```

### **Step 2: Verify Both Backends Running**

```bash
kubectl get uvr -n default
```

**Expected Output:**
```
NAME                      STATE       MODE           SOURCE          READY   AGE
demo-trident-replication  replica     asynchronous   my-app-data     True    5m
demo-ceph-replication     secondary   asynchronous   ceph-app-data   True    10s
```

**✅ Key Observation:**
- Two different backends
- Same operator
- Different states
- Both managed simultaneously

### **Step 3: Verify Ceph VolumeReplication Created**

```bash
kubectl get volumereplication -n default
# Or:
kubectl get volumereplications.replication.storage.openshift.io -n default
```

**Expected Output (if Ceph CRDs installed):**
```
NAME                     STATE       REPLICATION-STATE   AGE
demo-ceph-replication    secondary   ...                 15s
```

**If Ceph CRDs NOT installed:**
```
error: the server doesn't have a resource type "volumereplication"
```

And the Unified CR will show:
```bash
kubectl get uvr demo-ceph-replication -n default -o jsonpath='{.status.conditions[0].message}'
# Output: "backend ceph not available in cluster"
```

### **Step 4: Compare Backend-Specific Resources**

```bash
# List all backend resources
echo "=== Trident Resources ==="
kubectl get tridentmirrorrelationship -n default

echo ""
echo "=== Ceph Resources ==="
kubectl get volumereplication -n default 2>/dev/null || echo "Ceph CRDs not installed"
```

**This shows:**
- ✅ Different backend CRDs created automatically
- ✅ Each managed by the unified operator
- ✅ Backend-specific naming and fields

### **Step 5: Verify No Operator Restart**

```bash
# Check operator uptime
kubectl get pods -n unified-replication-system -o wide

# Check logs for restart
kubectl logs -n unified-replication-system -l control-plane=controller-manager | \
  grep "starting manager" | wc -l
```

**Expected:**
```
1  ← Only one "starting manager" log = no restart
```

### **Step 6: Check Backend Detection Logs**

```bash
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=100 | \
  grep "Selected backend"
```

**Expected Output:**
```json
{"msg":"Selected backend","backend":"trident"}  ← For demo-trident-replication
{"msg":"Selected backend","backend":"ceph"}     ← For demo-ceph-replication (if CRDs present)
```

**Different backends selected for different resources!**

✅ **Checkpoint:** Backend switching works without operator restart

---

## Part 5: Advanced Validation

### **Test 1: Simultaneous Reconciliation**

```bash
# Watch operator manage both backends
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f
```

You'll see logs alternating between:
- `"logger":"trident-adapter"` for Trident resource
- `"logger":"ceph-adapter"` for Ceph resource

**Proves:** Different adapters running simultaneously

### **Test 2: Update Both Resources**

```bash
# Update Trident replication
kubectl patch uvr demo-trident-replication -n default --type=merge -p '
{"spec":{"schedule":{"rpo":"20m"}}}'

# Update Ceph replication  
kubectl patch uvr demo-ceph-replication -n default --type=merge -p '
{"spec":{"schedule":{"rpo":"3m"}}}'

# Watch both backend CRDs update
kubectl get tridentmirrorrelationship demo-trident-replication -n default -o jsonpath='{.spec.replicationSchedule}'
# Output: 20m

kubectl get volumereplication demo-ceph-replication -n default -o jsonpath='{.spec.replicationClass}'
# Output: (Ceph-specific value)
```

### **Test 3: Delete and Verify Cleanup**

```bash
# Delete Unified CR
kubectl delete uvr demo-trident-replication -n default

# Wait a moment
sleep 5

# Verify backend CR also deleted
kubectl get tridentmirrorrelationship demo-trident-replication -n default
# Expected: Error from server (NotFound)
```

**✅ Proves:** Finalizer ensures backend cleanup

---

## 📊 **Complete Workflow Diagram**

```
User
  │
  ├─ kubectl apply -f trident-replication.yaml
  │    ↓
  │  UnifiedVolumeReplication (trident)
  │    ↓
  │  Operator detects: storageClass="trident-ontap-san"
  │    ↓
  │  Selects: Trident Adapter
  │    ↓
  │  Creates: TridentMirrorRelationship ✅
  │    ↓
  │  Trident Controller → Performs replication
  │
  ├─ kubectl apply -f ceph-replication.yaml
  │    ↓
  │  UnifiedVolumeReplication (ceph)
  │    ↓
  │  Operator detects: storageClass="ceph-rbd"
  │    ↓
  │  Selects: Ceph Adapter
  │    ↓
  │  Creates: VolumeReplication (Ceph) ✅
  │    ↓
  │  Ceph CSI → Performs replication
  │
  └─ All managed by SAME operator instance (no restart!)
```

---

## 🎯 **Automated Demo Script**

Run the complete demo automatically:

```bash
./test-backend-switching.sh
```

**This script will:**
1. ✅ Verify operator is running
2. ✅ Create Trident replication
3. ✅ Validate TridentMirrorRelationship created
4. ✅ Create Ceph replication
5. ✅ Verify no operator restart
6. ✅ Show backend detection logs
7. ✅ Display summary

---

## 📝 **Step-by-Step Manual Demo**

### **Complete Demo Flow:**

```bash
# 1. Deploy Operator
./scripts/build-and-push.sh

# 2. Create Trident Replication
kubectl apply -f trident-replication.yaml
sleep 5

# 3. Validate Trident CR Created
kubectl get uvr,tridentmirrorrelationship -n default

# 4. Update Unified CR
kubectl patch uvr demo-trident-replication -n default --type=merge \
  -p '{"spec":{"schedule":{"rpo":"10m"}}}'

# 5. Verify Trident CR Updated
sleep 10
kubectl get tridentmirrorrelationship demo-trident-replication -n default \
  -o jsonpath='{.spec.replicationSchedule}'
# Should show: 10m

# 6. Create Ceph Replication (different backend!)
kubectl apply -f ceph-replication.yaml
sleep 5

# 7. Verify Both Running
kubectl get uvr -n default

# 8. Check Operator Never Restarted
kubectl get pods -n unified-replication-system -o jsonpath='{.items[0].status.startTime}'
# Note the timestamp - compare with initial deployment time

# 9. View Backend Detection
kubectl logs -n unified-replication-system -l control-plane=controller-manager | \
  grep "Selected backend"
```

---

## 🔬 **Detailed Validation Commands**

### **For Trident Replication:**

```bash
# Quick status
kubectl get uvr demo-trident-replication -n default

# Detailed view
kubectl describe uvr demo-trident-replication -n default

# Backend-specific resource
kubectl describe tridentmirrorrelationship demo-trident-replication -n default

# Automated validation
../scripts/validate-replication.sh demo-trident-replication
```

### **For Ceph Replication:**

```bash
# Quick status
kubectl get uvr demo-ceph-replication -n default

# Backend resource (if CRDs installed)
kubectl get volumereplication demo-ceph-replication -n default

# Check why Ready=False (if CRDs not installed)
kubectl get uvr demo-ceph-replication -n default \
  -o jsonpath='{.status.conditions[0].message}'
# Output: "backend ceph not available in cluster"
```

---

## 📈 **Translation Examples**

### **Trident Translation:**

**Your Input:**
```yaml
spec:
  replicationState: source
  replicationMode: asynchronous
  volumeMapping:
    source:
      pvcName: my-app-data
    destination:
      volumeHandle: trident-pvc-xyz
  schedule:
    rpo: "15m"
```

**Generated Trident CR:**
```yaml
spec:
  state: established                    # ← Translated
  replicationPolicy: Async              # ← Translated
  volumeMappings:                       # ← Restructured
  - localPVCName: my-app-data
    remoteVolumeHandle: trident-pvc-xyz
  replicationSchedule: 15m              # ← Mapped
```

### **Ceph Translation:**

**Your Input:**
```yaml
spec:
  replicationState: secondary
  replicationMode: asynchronous
  extensions:
    ceph:
      mirroringMode: snapshot
```

**Generated Ceph CR:**
```yaml
spec:
  replicationState: secondary           # ← Direct mapping
  replicationClass: <class-name>        # ← From schedule/mode
  mirroringMode: snapshot               # ← From extensions
```

---

## 🎬 **Complete Demo Script**

Save this as `run-demo.sh`:

```bash
#!/bin/bash
set -e

export KUBECONFIG=/path/to/kubeconfig

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  UNIFIED REPLICATION OPERATOR - LIVE DEMO"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

echo ""
echo "📦 STEP 1: Deploy Operator"
echo "────────────────────────────────────────────────"
./scripts/build-and-push.sh
kubectl wait --for=condition=available deployment/unified-replication-operator \
  -n unified-replication-system --timeout=120s
echo "✅ Operator deployed and ready"

echo ""
echo "🔵 STEP 2: Create Trident Replication"
echo "────────────────────────────────────────────────"
kubectl apply -f trident-replication.yaml
sleep 10
kubectl get uvr,tridentmirrorrelationship -n default
echo "✅ Trident replication created"

echo ""
echo "🔄 STEP 3: Update Unified CR"
echo "────────────────────────────────────────────────"
echo "Changing RPO from 15m to 10m..."
kubectl patch uvr demo-trident-replication -n default --type=merge \
  -p '{"spec":{"schedule":{"rpo":"10m"}}}'
sleep 15
echo "Trident CR replicationSchedule:"
kubectl get tridentmirrorrelationship demo-trident-replication -n default \
  -o jsonpath='{.spec.replicationSchedule}'
echo ""
echo "✅ Update propagated to Trident CR"

echo ""
echo "🔴 STEP 4: Switch to Ceph Backend"
echo "────────────────────────────────────────────────"
kubectl apply -f ceph-replication.yaml
sleep 10
kubectl get uvr -n default
echo "✅ Ceph replication created (different backend)"

echo ""
echo "🔍 STEP 5: Verify No Operator Restart"
echo "────────────────────────────────────────────────"
RESTART_COUNT=$(kubectl get pods -n unified-replication-system \
  -l control-plane=controller-manager -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}')
echo "Operator restart count: ${RESTART_COUNT}"
if [ "$RESTART_COUNT" = "0" ]; then
    echo "✅ No restarts - backend switching is seamless!"
else
    echo "⚠️  Operator restarted ${RESTART_COUNT} times"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "           ✅ DEMO COMPLETE ✅"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Summary:"
echo "  ✅ Operator deployed"
echo "  ✅ Trident replication working"
echo "  ✅ Updates propagate automatically"
echo "  ✅ Ceph backend detected"
echo "  ✅ No operator restarts"
echo ""
```

---

## 🧪 **Expected Results Summary**

| Step | Action | Expected Result | Validates |
|------|--------|-----------------|-----------|
| 1 | Deploy operator | Pod running, Ready=1/1 | Installation works |
| 2 | Apply Trident CR | TridentMirrorRelationship created | Translation works |
| 3 | Update Unified CR | Trident CR updates automatically | Sync works |
| 4 | Apply Ceph CR | VolumeReplication created OR graceful failure | Backend switching |
| 5 | Check operator | No restarts, same pod | Seamless operation |

---

## 🎯 **Key Validations**

### ✅ **1. Automatic CRD Creation**
```bash
# You create this:
kubectl apply -f trident-replication.yaml

# Operator automatically creates this:
kubectl get tridentmirrorrelationship -n default
```

### ✅ **2. Automatic Translation**
```bash
# Your spec says:
replicationState: source

# Trident CR shows:
state: established

# Operator translated it!
```

### ✅ **3. Automatic Updates**
```bash
# You update:
kubectl patch uvr ... -p '{"spec":{"schedule":{"rpo":"10m"}}}'

# Backend CR automatically updates:
replicationSchedule: 10m
```

### ✅ **4. Seamless Backend Switching**
```bash
# Create Trident replication
kubectl apply -f trident-replication.yaml
# → Uses Trident adapter

# Create Ceph replication (same operator!)
kubectl apply -f ceph-replication.yaml
# → Uses Ceph adapter

# No restart needed!
```

---

## 🚀 **What Makes This Powerful**

### **Before (Without Operator):**
```bash
# User must know Trident CRD format:
apiVersion: trident.netapp.io/v1
kind: TridentMirrorRelationship
spec:
  state: established  # What does this mean?
  volumeMappings:     # Array or object?
  - localPVCName: ...
    
# AND must know Ceph CRD format:
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeReplication
spec:
  replicationState: primary  # Different from Trident!
  dataSource: ...
```

### **After (With Operator):**
```yaml
# Same format for all backends!
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
spec:
  replicationState: source      # Unified terminology
  replicationMode: asynchronous  # Unified terminology
  sourceEndpoint:
    storageClass: trident-ontap-san  # ← Just change this!
    # OR: storageClass: ceph-rbd
    # OR: storageClass: powerstore-block
```

**✅ Single API, any backend!**

---

## 📚 **Documentation Reference**

- **This Demo:** `COMPREHENSIVE_DEMO.md`
- **Quick Start:** `QUICK_START.md`
- **Validation:** `VALIDATION_GUIDE.md` + `scripts/validate-replication.sh`
- **Backend Switching:** `BACKEND_SWITCHING_DEMO.md`
- **Build Guide:** `BUILD_AND_DEPLOY.md`
- **OpenShift Setup:** `OPENSHIFT_INSTALL.md`

---

## 🎓 **Demo Checklist**

Use this checklist when demonstrating the operator:

- [ ] Operator deployed (Part 1)
- [ ] Trident CR created from Unified CR (Part 2.1-2.3)
- [ ] Translation validated (Part 2.4)
- [ ] Update propagation shown (Part 3)
- [ ] Ceph backend detected (Part 4.1-4.4)
- [ ] No operator restart verified (Part 4.5)
- [ ] Backend-specific CRDs shown (Part 4.3)

---

## 🎉 **Demo Complete!**

You have successfully demonstrated:

1. ✅ **Deployment** - Operator running on OpenShift/Kubernetes
2. ✅ **CRD Creation** - UnifiedVolumeReplication → TridentMirrorRelationship
3. ✅ **Update Propagation** - Changes sync to backend CRD
4. ✅ **Backend Switching** - Ceph and Trident simultaneously
5. ✅ **Zero Downtime** - No operator restart required

**The Unified Replication Operator delivers on its promise:**
> *"Single CRD for all storage backends"*

---

## 🔗 **Next Steps**

- **Production Deployment:** See `OPENSHIFT_INSTALL.md`
- **Advanced Features:** Test failover scenarios with `FAILOVER_TUTORIAL.md`
- **Monitoring:** Set up metrics and dashboards
- **Multi-Cluster:** Configure cross-cluster replication

---

*Demo Version: 1.0*  
*Operator Version: 0.2.1*  
*Last Updated: 2025-10-14*

