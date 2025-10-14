# Webhook & Validation Guide

This guide explains the validation mechanisms in the Unified Replication Operator.

---

## 🎯 **Validation Layers**

The operator has **three layers of validation**:

### **1. OpenAPI Schema Validation** (Always Active)

Built into Kubernetes via CRD OpenAPI schema:

```yaml
# In CRD definition
spec:
  replicationState:
    type: string
    enum: ["source", "replica", "promoting", "demoting", "syncing", "failed"]
```

**What it validates:**
- ✅ Enum values (replicationState, replicationMode)
- ✅ Required fields
- ✅ Data types
- ✅ String patterns/formats

**When it runs:**
- At `kubectl apply` time
- Before resource is created
- Cannot be disabled

**Example:**
```bash
kubectl apply -f test-invalid-replication.yaml
# Error: spec.replicationState: Unsupported value: "invalid-state"
```

---

### **2. Admission Webhook Validation** (Optional)

Custom validation logic via ValidatingWebhookConfiguration:

**What it validates:**
- ✅ Business logic (e.g., source != destination)
- ✅ Cross-field validation
- ✅ Complex rules
- ✅ External lookups

**When it runs:**
- After OpenAPI validation
- Before resource is persisted
- During `kubectl apply --dry-run=server`

**Current Status:**
- ⚠️ **DISABLED by default** (for simpler deployment)
- Can be enabled with: `ENABLE_WEBHOOKS=true ./scripts/build-and-push.sh`

---

### **3. Controller Validation** (Always Active)

Validation during reconciliation loop:

**What it validates:**
- ✅ Runtime conditions (PVC exists, backend available)
- ✅ State machine transitions
- ✅ Backend-specific requirements
- ✅ Resource dependencies

**When it runs:**
- During reconciliation
- Every 30 seconds
- After any spec change

**Result:**
- Sets `.status.conditions[].status = False`
- Provides detailed error messages
- Retries automatically

---

## 🧪 **Testing Validation**

### **Test 1: Schema Validation (Always Active)**

```bash
cd demo

# Try invalid enum value
kubectl apply -f test-invalid-replication.yaml
```

**Expected:**
```
Error: spec.replicationState: Unsupported value: "invalid-state": 
  supported values: "source", "replica", "promoting", "demoting", "syncing", "failed"
```

✅ **Rejected immediately by Kubernetes**

---

### **Test 2: Controller Validation**

```bash
# Create resource with valid schema but impossible requirements
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: test-nonexistent-pvc
  namespace: default
spec:
  sourceEndpoint:
    storageClass: trident-ontap-san
    cluster: test
    region: us-east-1
  destinationEndpoint:
    storageClass: trident-ontap-nas
    cluster: test2
    region: us-west-1
  volumeMapping:
    source:
      pvcName: "nonexistent-pvc"  # ← Doesn't exist
      namespace: default
    destination:
      volumeHandle: "vol-123"
      namespace: default
  replicationState: source
  replicationMode: asynchronous
  schedule:
    rpo: "15m"
    rto: "5m"
    mode: interval
EOF

# Check status after reconciliation
sleep 10
kubectl get uvr test-nonexistent-pvc -n default \
  -o jsonpath='{.status.conditions[0]}'
```

**Expected:**
```json
{
  "status": "False",
  "reason": "ReconciliationFailed",
  "message": "PVC nonexistent-pvc not found"
}
```

✅ **Validated by controller during reconciliation**

---

### **Test 3: Webhook Validation (If Enabled)**

**Enable webhooks first:**
```bash
ENABLE_WEBHOOKS=true ./scripts/build-and-push.sh
```

**Then test:**
```bash
# Create resource with same source/destination cluster
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: test-same-cluster
  namespace: default
spec:
  sourceEndpoint:
    cluster: "same"      # ← Same cluster
    region: "us-east-1"
    storageClass: trident-ontap-san
  destinationEndpoint:
    cluster: "same"      # ← Invalid: same as source
    region: "us-east-1"
    storageClass: trident-ontap-nas
  # ... rest of spec
EOF
```

**Expected (with webhook):**
```
Error from server (admission webhook denied): 
  source and destination cluster must be different
```

✅ **Rejected by webhook before creation**

---

## 🔧 **Running the Validation Test**

### **Automated Test Script**

```bash
cd demo
./test-webhook-validation.sh
```

**This script:**
1. ✅ Checks webhook configuration status
2. ✅ Tests valid resource acceptance
3. ✅ Tests invalid resource handling
4. ✅ Shows validation layer in use
5. ✅ Provides comparison and recommendations

---

## 📊 **Validation Comparison**

| Validation Type | Always Active | Validates | Timing | Good For |
|----------------|---------------|-----------|--------|----------|
| **OpenAPI Schema** | ✅ Yes | Enums, types, required fields | Pre-admission | All cases |
| **Admission Webhook** | ⚠️ Optional | Business logic, cross-field | Pre-admission | Production |
| **Controller** | ✅ Yes | Runtime conditions, dependencies | During reconciliation | All cases |

---

## 🎯 **Recommended Setup**

### **Development/Testing:**
```bash
# Default: Webhooks disabled
./scripts/build-and-push.sh
```

**Benefits:**
- ✅ Faster deployment
- ✅ No certificate management
- ✅ OpenAPI + Controller validation sufficient

### **Production:**
```bash
# Enable webhooks
ENABLE_WEBHOOKS=true ./scripts/build-and-push.sh
```

**Benefits:**
- ✅ Immediate feedback on invalid resources
- ✅ Prevents bad resources in cluster
- ✅ Better for multi-user environments
- ✅ Catches errors before reconciliation

---

## 🧪 **Example Validation Scenarios**

### **Scenario 1: Invalid Enum Value**

**Resource:**
```yaml
replicationState: "invalid-state"
```

**Validation:**
- ✅ OpenAPI Schema → **REJECTED**
- ⏭️ Webhook → Not reached
- ⏭️ Controller → Not reached

**Result:** Immediate rejection

---

### **Scenario 2: Valid Schema, Invalid Logic**

**Resource:**
```yaml
sourceEndpoint:
  cluster: "cluster-a"
destinationEndpoint:
  cluster: "cluster-a"  # Same as source!
```

**Validation:**
- ✅ OpenAPI Schema → PASSES (both are valid strings)
- ⚠️ Webhook → REJECTS (if enabled)
- ✅ Controller → REJECTS (sets Ready=False)

**Result:** 
- With webhooks: Rejected at apply time
- Without webhooks: Accepted, fails at reconciliation

---

### **Scenario 3: Runtime Condition**

**Resource:**
```yaml
volumeMapping:
  source:
    pvcName: "nonexistent-pvc"
```

**Validation:**
- ✅ OpenAPI Schema → PASSES (valid string)
- ✅ Webhook → PASSES (PVC existence checked at runtime)
- ⚠️ Controller → REJECTS (PVC not found)

**Result:** Accepted, fails during reconciliation

---

## 📋 **Quick Reference**

```bash
# Test current validation
cd demo && ./test-webhook-validation.sh

# Enable webhooks
ENABLE_WEBHOOKS=true ./scripts/build-and-push.sh

# Test with invalid resource
kubectl apply -f demo/test-invalid-replication.yaml

# Check validation in dry-run
kubectl apply -f my-replication.yaml --dry-run=server

# View validation errors in status
kubectl get uvr <name> -n default -o jsonpath='{.status.conditions[0].message}'
```

---

## 🎓 **Best Practices**

1. **Always rely on OpenAPI schema validation** (can't be disabled)
2. **Use webhooks in production** for immediate feedback
3. **Monitor controller validation** via status conditions
4. **Test with `--dry-run=server`** before applying
5. **Check `.status.conditions`** for runtime errors

---

## ✅ **Current Demo Configuration**

The demo uses **controller-based validation** (webhooks disabled) because:
- ✅ Simpler for demonstrations
- ✅ No certificate management required
- ✅ Validation still works (via controller)
- ✅ Easier to show validation errors in status

**For production, enable webhooks for better user experience.**

---

*Last Updated: 2025-10-14*  
*Operator Version: 0.2.3+*

