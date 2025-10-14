# Validation Guide - Confirming Your Operator Works

## ✅ Complete Validation Checklist

### **Automated Validation**
```bash
export KUBECONFIG=/path/to/kubeconfig
./scripts/validate-replication.sh trident-volume-replication
```

### **Manual Validation Steps**

## 1. ✅ Unified CR Created

```bash
kubectl get uvr -A
```

**Expected Output:**
```
NAME                         STATE    MODE           SOURCE        READY   AGE
trident-volume-replication   source   asynchronous   my-app-data   True    34s
```

**Key Indicators:**
- ✅ Resource exists
- ✅ STATE matches your spec (`source`)
- ✅ MODE matches your spec (`asynchronous`)
- ✅ READY = `True`

---

## 2. ✅ Backend-Specific CRD Created

This is the KEY validation - your UnifiedVolumeReplication should create a Trident-specific CRD:

```bash
kubectl get tridentmirrorrelationships -n default
```

**Expected Output:**
```
NAME                         DESIRED STATE   LOCAL PVC     ACTUAL STATE   MESSAGE
trident-volume-replication   established     my-app-data                  
```

**Key Indicators:**
- ✅ TridentMirrorRelationship resource exists (SAME NAME as your UVR)
- ✅ Managed by operator (labels show `app.kubernetes.io/managed-by=unified-replication-operator`)

---

## 3. ✅ Translation Worked Correctly

### **Unified → Trident Translation:**

| **Your Input (UnifiedVolumeReplication)** | **Translated To (TridentMirrorRelationship)** |
|-------------------------------------------|----------------------------------------------|
| `replicationState: source` | `state: established` ✅ |
| `replicationMode: asynchronous` | `replicationPolicy: Async` ✅ |
| `volumeMapping.source.pvcName: my-app-data` | `volumeMappings[0].localPVCName: my-app-data` ✅ |
| `volumeMapping.destination.volumeHandle: trident-pvc-...` | `volumeMappings[0].remoteVolumeHandle: trident-pvc-...` ✅ |
| `schedule.rpo: 15m` | `replicationSchedule: 15m` ✅ |

**Verify with:**
```bash
kubectl get uvr trident-volume-replication -n default -o yaml | grep -A 5 "spec:"
kubectl get tridentmirrorrelationship trident-volume-replication -n default -o yaml | grep -A 10 "spec:"
```

---

## 4. ✅ Operator Logs Show Success

```bash
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=50 | grep trident-volume-replication
```

**Expected Log Sequence:**
```
✅ "logger":"trident-adapter"              → Real adapter (not mock)
✅ "Ensuring Trident mirror relationship"  → Reconciling
✅ "TridentMirrorRelationship not found"   → Detected need to create
✅ "Successfully created Trident mirror"   → Created successfully!
✅ "Reconciliation completed successfully" → Done
```

---

## 5. ✅ Status Conditions

```bash
kubectl get uvr trident-volume-replication -n default -o jsonpath='{.status.conditions}' | jq '.'
```

**Expected:**
```json
[
  {
    "type": "Ready",
    "status": "True",
    "reason": "ReconciliationSucceeded",
    "message": "Replication is operating normally"
  }
]
```

---

## 6. ✅ Finalizer Present

```bash
kubectl get uvr trident-volume-replication -n default -o jsonpath='{.metadata.finalizers}'
```

**Expected:**
```
["replication.storage.io/finalizer"]
```

**Why this matters:**
- Ensures the operator cleans up the TridentMirrorRelationship when you delete the UVR
- Prevents orphaned resources

---

## 7. ✅ Backend Detection

```bash
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=100 | grep "Backend discovery completed"
```

**Expected:**
```json
{"available":1,"total":3}  ← 1 backend (Trident) available
```

---

## 8. ✅ Describe Both Resources

```bash
# Your Unified CR
kubectl describe uvr trident-volume-replication -n default

# Generated Trident CR
kubectl describe tridentmirrorrelationship trident-volume-replication -n default
```

**Compare:**
- Same name ✅
- Same namespace ✅
- Trident CR shows `Labels: app.kubernetes.io/managed-by=unified-replication-operator` ✅

---

## 9. ✅ Test Modifications

Update your UnifiedVolumeReplication and watch Trident CR update automatically:

```bash
# Edit your CR
kubectl edit uvr trident-volume-replication -n default

# Change replicationState from "source" to "replica"
# Save and exit

# Watch the Trident CR update
kubectl get tridentmirrorrelationship trident-volume-replication -n default -w
```

**Expected:**
- Trident CR's `state` changes from `established` to `established` (mirrors the change)
- Logs show: `"TridentMirrorRelationship exists, updating if needed"`

---

## 10. ✅ Test Deletion

```bash
# Delete your Unified CR
kubectl delete uvr trident-volume-replication -n default

# Watch what happens
kubectl get tridentmirrorrelationship trident-volume-replication -n default
```

**Expected:**
- TridentMirrorRelationship is deleted automatically ✅
- Logs show: `"Deleting Trident mirror relationship"`

---

## 🎯 **Success Criteria Summary**

Your operator is working correctly if ALL of these are true:

| Check | Status | Command |
|-------|--------|---------|
| UnifiedVolumeReplication exists | ✅ | `kubectl get uvr -n default` |
| Ready = True | ✅ | `kubectl get uvr ... -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'` |
| TridentMirrorRelationship created | ✅ | `kubectl get tridentmirrorrelationship -n default` |
| State translated correctly | ✅ | `source` → `established` |
| Mode translated correctly | ✅ | `asynchronous` → `Async` |
| volumeMappings populated | ✅ | Check TMR spec |
| Managed-by label present | ✅ | TMR shows `unified-replication-operator` |
| Finalizer added | ✅ | Check UVR metadata |
| Real adapter used | ✅ | Logs show `trident-adapter`, not `mock-trident-adapter` |
| Reconciliation succeeds | ✅ | Logs show "Successfully created" |

---

## 🐛 Common Issues

### TridentMirrorRelationship Not Found

**Symptom:**
```bash
kubectl get tridentmirrorrelationship -n default
# No resources found
```

**Causes:**
1. **Using mock adapter** - Check logs for `mock-trident-adapter`
   - Fix: Use real adapter (update main.go)
2. **Trident CRDs not installed** - CRD doesn't exist
   - Check: `kubectl get crd | grep trident`
3. **Creation failed** - Check operator logs for errors

---

### Ready = False

**Symptom:**
```bash
kubectl get uvr -n default
# READY = False
```

**Check:**
```bash
kubectl describe uvr trident-volume-replication -n default | grep -A 10 "Conditions:"
```

**Common reasons:**
- Backend not detected
- Validation failed
- Adapter error

---

### State Not Updating

**Symptom:** Change state but Trident CR doesn't update

**Check:**
```bash
# View reconciliation logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f
```

---

## 📋 **Quick Reference Commands**

```bash
# Set KUBECONFIG
export KUBECONFIG=/home/oaharoni/aws-gpfs-playground/ocp_install_files/auth/kubeconfig

# Full validation
./scripts/validate-replication.sh trident-volume-replication

# Quick status
kubectl get uvr,tridentmirrorrelationship -n default

# Watch logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f

# Check events
kubectl get events -n default --field-selector involvedObject.name=trident-volume-replication

# Describe both resources
kubectl describe uvr trident-volume-replication -n default
kubectl describe tridentmirrorrelationship trident-volume-replication -n default
```

---

## 🎉 **Your Current Status**

Based on validation:

✅ **UnifiedVolumeReplication** - Created, Ready=True  
✅ **Real Trident Adapter** - Being used (not mock)  
✅ **TridentMirrorRelationship** - Created successfully  
✅ **State Translation** - `source` → `established`  
✅ **Mode Translation** - `asynchronous` → `Async`  
✅ **Volume Mappings** - Populated correctly  
✅ **Labels** - Managed by operator  
✅ **Finalizer** - Added for cleanup  

**Your operator is fully operational!** 🚀

