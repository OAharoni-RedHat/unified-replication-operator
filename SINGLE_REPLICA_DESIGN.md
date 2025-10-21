# Single-Replica Design

This operator is designed for **single-replica deployment only**. Leader election has been removed for simplicity.

---

## ⚠️ **IMPORTANT: Single Replica Only**

```yaml
replicaCount: 1  # DO NOT CHANGE!
```

**This operator does NOT support multiple replicas.**

Running multiple replicas will cause:
- ❌ Duplicate backend CRD creation
- ❌ Conflicting updates
- ❌ Race conditions
- ❌ Resource thrashing

---

## ✅ **Why Single Replica?**

### **Design Decision:**

Leader election was removed to **simplify** the operator:

1. **Simpler Code**
   - No lease management
   - No coordination logic
   - No standby overhead

2. **Lower Resource Usage**
   - Single pod only
   - No coordination API calls
   - Minimal footprint

3. **Easier to Debug**
   - Single source of logs
   - No "which pod is leader?" questions
   - Clear reconciliation path

4. **Sufficient for Most Use Cases**
   - Development and testing
   - Small to medium deployments
   - Non-critical workloads

---

## 🔒 **How Single Replica is Enforced**

### **1. Deployment Strategy: Recreate**

```yaml
# values.yaml
strategy:
  type: Recreate
```

**What this does:**
- During upgrades: Old pod terminates **first**
- Then: New pod starts
- **Never** two pods running simultaneously
- Brief downtime (~10-30 seconds) during upgrades

**Alternative (RollingUpdate) NOT used:**
```yaml
strategy:
  type: RollingUpdate  # ← Would allow 2 pods temporarily
```

### **2. Replica Count: 1**

```yaml
# values.yaml
replicaCount: 1
```

**Hard-coded in Helm chart.**

### **3. Warning Comments**

```yaml
# WARNING: This operator is designed for single-replica deployment only
# Leader election is disabled - do NOT scale beyond 1 replica
```

---

## 📊 **Comparison: Single vs Multi-Replica**

| Aspect | Single Replica (Current) | Multi-Replica (With Leader Election) |
|--------|--------------------------|--------------------------------------|
| **High Availability** | ❌ No | ✅ Yes (~15s failover) |
| **Complexity** | ✅ Low | ❌ Higher |
| **Resource Usage** | ✅ Minimal | ❌ More (multiple pods) |
| **Upgrade Downtime** | ⚠️ Yes (~30s) | ✅ None (rolling) |
| **Conflicts** | ✅ None | ✅ None (via leader election) |
| **Debugging** | ✅ Easy | ❌ Harder (which pod?) |
| **Good For** | Dev, test, small prod | Large prod, critical workloads |

---

## ⚙️ **Deployment Details**

### **Normal Deployment:**

```bash
helm install unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace
```

**Result:**
- 1 pod created
- No leader election
- Simple operation

### **Upgrade Process:**

```bash
helm upgrade unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system
```

**What happens:**
```
Time 0:   Old pod receives SIGTERM
Time 2s:  Old pod gracefully shuts down
Time 3s:  Old pod terminated
Time 4s:  New pod starts
Time 15s: New pod ready
Time 16s: Operator reconciling again
```

**Downtime:** ~10-30 seconds (acceptable for most workloads)

---

## ⚠️ **What NOT to Do**

### **❌ DO NOT Scale Beyond 1 Replica**

```bash
# This will cause problems!
kubectl scale deployment unified-replication-operator -n unified-replication-system --replicas=3
```

**Problems:**
- All 3 pods reconcile simultaneously
- Duplicate backend CRD creation
- Conflicting updates
- Resource waste

### **❌ DO NOT Use RollingUpdate Strategy**

```yaml
# DO NOT change this!
strategy:
  type: RollingUpdate  # ← WRONG! Allows 2 pods during update
```

---

## ✅ **What You CAN Do**

### **1. Configure Resource Limits**

```yaml
# values.yaml
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

Safe to adjust based on your workload.

### **2. Add Node Affinity**

```yaml
# values.yaml
nodeSelector:
  node-role.kubernetes.io/worker: ""
```

Pin to specific nodes if needed.

### **3. Use PodDisruptionBudget**

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: unified-replication-operator
spec:
  maxUnavailable: 0  # Prevent voluntary disruptions
  selector:
    matchLabels:
      app.kubernetes.io/name: unified-replication-operator
```

Prevents node drains during maintenance (pod must be rescheduled first).

---

## 🔍 **Monitoring Single Replica**

### **Check Pod Health:**

```bash
kubectl get pods -n unified-replication-system

# Expected: 1 pod, Running
# NAME                                            READY   STATUS
# unified-replication-operator-xxxxx-yyyyy        1/1     Running
```

### **What if Pod Crashes?**

```
Pod crashes → Kubernetes restarts it automatically
Restart time: ~10-30 seconds
During restart: No reconciliation (brief gap)
After restart: All replications reconciled
```

**Recovery is automatic via Kubernetes restart policy.**

### **Check Reconciliation:**

```bash
# View logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f

# Check replications being reconciled
kubectl get uvr -A -w
```

---

## 📋 **Troubleshooting**

### **Issue: Pod Not Starting**

```bash
# Check pod status
kubectl describe pod -n unified-replication-system -l control-plane=controller-manager

# Check logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager
```

### **Issue: Reconciliation Slow**

Single replica handles all replications:

```bash
# Check how many replications
kubectl get uvr -A | wc -l

# If too many (>100), consider:
# - Adjusting reconciliation concurrency
# - Splitting across namespaces
# - OR re-enabling leader election with multiple replicas
```

### **Issue: Upgrade Causing Issues**

Brief downtime is expected:

```bash
# Check upgrade status
kubectl rollout status deployment unified-replication-operator \
  -n unified-replication-system

# Check new pod started
kubectl get pods -n unified-replication-system
```

---

## 🎯 **When to Consider Leader Election**

Consider re-enabling leader election if:

- ✅ You have > 100 replications
- ✅ You need 24/7 availability
- ✅ You can't tolerate upgrade downtime
- ✅ You need automatic failover

**To re-enable:**
1. Uncomment leader election code in `main.go`
2. Add back leader election RBAC
3. Set `replicaCount: 3` in values
4. Change strategy to `RollingUpdate`

---

## 📊 **Production Readiness**

### **Single Replica is Production-Ready For:**

✅ **Development/Staging**
- Testing and validation
- CI/CD pipelines
- Non-critical environments

✅ **Small Production**
- < 50 replications
- Can tolerate brief downtime
- Lower complexity preferred

✅ **Cost-Sensitive**
- Minimal resource usage
- Single pod overhead
- Lower cloud costs

### **NOT Production-Ready For:**

❌ **Large-Scale Production**
- > 100 replications
- High reconciliation volume
- 24/7 uptime requirements

❌ **Critical Workloads**
- Zero downtime requirements
- Automatic failover needed
- Multiple availability zones

---

## 🚀 **Current Configuration**

```yaml
# Enforced Settings
replicaCount: 1          # Single pod only
strategy.type: Recreate  # No overlap during updates
LeaderElection: false    # No coordination needed

# Effects
- One active pod at all times
- No leader election overhead
- Brief downtime during upgrades
- Simpler operation
```

---

## 📖 **Summary**

### **Design Choice:**
**Single-replica deployment** with leader election removed

### **Rationale:**
- Simplicity over high availability
- Sufficient for most use cases
- Lower operational overhead

### **Constraints:**
- **DO NOT scale beyond 1 replica**
- **Expect downtime during upgrades** (~30s)
- **No automatic failover**

### **Benefits:**
- ✅ Simpler deployment
- ✅ Lower resource usage
- ✅ Easier to debug
- ✅ No coordination overhead

**This design choice prioritizes simplicity and is appropriate for development, testing, and small-to-medium production deployments.** 🎯

---

*Design Decision: Single Replica*  
*Leader Election: Disabled*  
*Last Updated: 2025-10-20*  
*Operator Version: 0.3.0*

