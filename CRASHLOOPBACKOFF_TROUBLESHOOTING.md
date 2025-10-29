# CrashLoopBackOff Troubleshooting Guide

## Quick Diagnosis Commands

Run these commands from your cluster to diagnose the issue:

```bash
# 1. Check pod status
kubectl get pods -n unified-replication-system

# 2. Get pod logs (replace POD_NAME with actual pod name)
kubectl logs -n unified-replication-system POD_NAME

# 3. If pod is restarting, get previous logs
kubectl logs -n unified-replication-system POD_NAME --previous

# 4. Describe the pod for events
kubectl describe pod -n unified-replication-system POD_NAME

# 5. Check deployment
kubectl get deployment -n unified-replication-system
kubectl describe deployment unified-replication-operator -n unified-replication-system
```

---

## Common Causes and Fixes

### Issue 1: Scheme Registration Error

**Symptom in logs:**
```
panic: no kind is registered for the type...
panic: runtime error: invalid memory address
```

**Cause:** v1alpha2 types not properly registered in scheme

**Fix:** Verify main.go has both registrations:

```go
// In main.go init() function
func init() {
    utilruntime.Must(clientgoscheme.AddToScheme(scheme))
    utilruntime.Must(apiextensionsv1.AddToScheme(scheme))
    
    // Both API versions must be registered
    utilruntime.Must(replicationv1alpha1.AddToScheme(scheme))
    utilruntime.Must(replicationv1alpha2.AddToScheme(scheme))
}
```

**Verify:**
```bash
# Check that main.go has v1alpha2 import and registration
grep -n "replicationv1alpha2" main.go
```

Should show:
- Line with import
- Line with AddToScheme

---

### Issue 2: Controller Setup Failure

**Symptom in logs:**
```
unable to create controller
unable to start manager
```

**Cause:** Controller setup failed (type mismatch, missing registry, etc.)

**Check main.go controller setup:**

```go
// VolumeReplication controller
if err = (&controllers.VolumeReplicationReconciler{
    Client:          mgr.GetClient(),
    Scheme:          mgr.GetScheme(),
    AdapterRegistry: adapterRegistry,  // Must be adapters.Registry interface, not pointer
}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "VolumeReplication")
    os.Exit(1)
}
```

**Common mistake:** Using `*adapters.Registry` instead of `adapters.Registry`

**Fix in controllers:**
```go
type VolumeReplicationReconciler struct {
    client.Client
    Scheme          *runtime.Scheme
    AdapterRegistry adapters.Registry  // Interface, not pointer!
}
```

---

### Issue 3: Adapter Registration Failure

**Symptom in logs:**
```
panic: runtime error
adapter not found
backend not supported
```

**Cause:** v1alpha2 adapters not registered

**Fix:** Ensure adapters are registered in main.go:

```go
// In main() function, after creating adapterRegistry:

// Register v1alpha1 factories (for backward compatibility)
adapterRegistry.RegisterFactory(adapters.NewCephAdapterFactory())
adapterRegistry.RegisterFactory(adapters.NewTridentAdapterFactory())
adapterRegistry.RegisterFactory(adapters.NewPowerStoreAdapterFactory())

// Register v1alpha2 adapters (NEW - required!)
adapters.RegisterV1Alpha2Adapters(adapterRegistry, mgr.GetClient())
```

**Verify file exists:**
```bash
ls -la pkg/adapters/v1alpha2_init.go
```

---

### Issue 4: Missing CRDs

**Symptom in logs:**
```
failed to get API group resources
no matches for kind
```

**Cause:** CRDs not installed before starting operator

**Fix:**
```bash
# Install CRDs first
kubectl apply -f config/crd/bases/

# Wait for CRDs to be established
kubectl wait --for condition=established --timeout=60s \
  crd/volumereplications.replication.unified.io \
  crd/volumereplicationclasses.replication.unified.io

# Then deploy operator
kubectl apply -k config/default
# or
helm install unified-replication-operator ./helm/unified-replication-operator
```

---

### Issue 5: RBAC Permissions

**Symptom in logs:**
```
forbidden: User "system:serviceaccount:..." cannot...
is forbidden
```

**Cause:** Service account lacks required permissions

**Fix:**
```bash
# Regenerate RBAC
make manifests

# Apply RBAC
kubectl apply -f config/rbac/

# Or reinstall via Helm (includes RBAC)
helm upgrade unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system
```

---

### Issue 6: Import Cycle or Missing Imports

**Symptom in logs:**
```
package ... is not in std
cannot find package
```

**Cause:** Missing dependencies or import issues

**Fix:**
```bash
# Ensure all dependencies are present
go mod tidy
go mod vendor

# Rebuild
make build

# Rebuild image
docker build -t unified-replication-operator:latest .
```

---

### Issue 7: Port Conflict

**Symptom in logs:**
```
bind: address already in use
failed to listen on :8080
```

**Cause:** Port 8080 or 9443 already in use

**Fix in values.yaml:**
```yaml
service:
  port: 8081  # Change from 8080

webhook:
  port: 9444  # Change from 9443 if conflict
```

---

## Debugging Steps

### Step 1: Get Logs

```bash
# Get current logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager

# Get previous crash logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager --previous

# Follow logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f
```

### Step 2: Check Events

```bash
# Get deployment events
kubectl describe deployment unified-replication-operator -n unified-replication-system

# Get pod events
kubectl get events -n unified-replication-system --sort-by='.lastTimestamp'
```

### Step 3: Check Image

```bash
# Verify image exists and is pullable
kubectl describe pod -n unified-replication-system <POD_NAME> | grep -A 5 "Image:"

# Check image pull policy
kubectl get deployment unified-replication-operator -n unified-replication-system -o yaml | grep -A 3 "image:"
```

### Step 4: Verify Scheme Registration

```bash
# Check main.go has proper registration
cat main.go | grep -A 5 "func init()"
cat main.go | grep "AddToScheme"
```

Should see:
```go
utilruntime.Must(replicationv1alpha1.AddToScheme(scheme))
utilruntime.Must(replicationv1alpha2.AddToScheme(scheme))
```

### Step 5: Check Controllers

```bash
# Verify controller files exist
ls -la controllers/volumereplication_controller.go
ls -la controllers/volumegroupreplication_controller.go

# Check they're being set up in main.go
cat main.go | grep -A 10 "VolumeReplicationReconciler"
```

---

## Most Likely Causes

Based on the v1alpha2 migration, the most likely issues are:

### 1. Controller Type Mismatch (Most Likely)

**Problem:** Using `*adapters.Registry` instead of `adapters.Registry`

**Fix in main.go:**
```go
// Correct:
if err = (&controllers.VolumeReplicationReconciler{
    Client:          mgr.GetClient(),
    Scheme:          mgr.GetScheme(),
    AdapterRegistry: adapterRegistry,  // Interface, not *Interface
}).SetupWithManager(mgr); err != nil {
```

**And in controller files:**
```go
type VolumeReplicationReconciler struct {
    client.Client
    Scheme          *runtime.Scheme
    AdapterRegistry adapters.Registry  // Not *adapters.Registry
}
```

### 2. Missing v1alpha2 Adapter Registration (Likely)

**Problem:** `adapters.RegisterV1Alpha2Adapters()` not called

**Fix in main.go:**
```go
// After creating adapterRegistry:
adapters.RegisterV1Alpha2Adapters(adapterRegistry, mgr.GetClient())
```

### 3. Controller Not Set Up (Possible)

**Problem:** New controllers not added to main.go

**Fix:** Ensure both v1alpha2 controllers are set up:
```go
// VolumeReplication controller
if err = (&controllers.VolumeReplicationReconciler{...}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "VolumeReplication")
    os.Exit(1)
}

// VolumeGroupReplication controller  
if err = (&controllers.VolumeGroupReplicationReconciler{...}).SetupWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create controller", "controller", "VolumeGroupReplication")
    os.Exit(1)
}
```

---

## Quick Fix Checklist

Run these checks:

```bash
# 1. Verify build is clean
make build
# Should succeed

# 2. Check main.go for v1alpha2
grep "replicationv1alpha2" main.go
# Should show import and AddToScheme

# 3. Check adapter registration
grep "RegisterV1Alpha2Adapters" main.go
# Should show function call

# 4. Rebuild and redeploy
make docker-build IMG=your-registry/unified-replication-operator:2.0.0-beta
docker push your-registry/unified-replication-operator:2.0.0-beta

# 5. Update deployment
kubectl set image deployment/unified-replication-operator \
  manager=your-registry/unified-replication-operator:2.0.0-beta \
  -n unified-replication-system

# 6. Watch logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f
```

---

## Get Help

**If still crashing, provide:**

1. **Pod logs:**
```bash
kubectl logs -n unified-replication-system <POD_NAME> --previous > crash-logs.txt
```

2. **Pod description:**
```bash
kubectl describe pod -n unified-replication-system <POD_NAME> > pod-describe.txt
```

3. **Deployment info:**
```bash
kubectl get deployment unified-replication-operator -n unified-replication-system -o yaml > deployment.yaml
```

4. **Events:**
```bash
kubectl get events -n unified-replication-system --sort-by='.lastTimestamp' > events.txt
```

Then share the logs to diagnose the specific issue.

---

## Common Error Messages and Fixes

### "no kind is registered"
→ Missing AddToScheme for v1alpha2

### "cannot convert *Registry to Registry"
→ Wrong type in controller struct (should be interface, not pointer)

### "adapter not found for backend"
→ Missing RegisterV1Alpha2Adapters() call

### "failed to create controller"
→ Check controller struct fields match SetupWithManager parameters

### "CRD not found"
→ Apply CRDs before deploying operator

### "permission denied"
→ Apply RBAC manifests

---

## Prevention for Next Deployment

```bash
# 1. Apply CRDs first
kubectl apply -f config/crd/bases/

# 2. Verify CRDs
kubectl get crd | grep replication.unified.io

# 3. Build fresh image
make docker-build IMG=registry/image:2.0.0-beta

# 4. Deploy via Helm (handles everything)
helm install unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace \
  --set image.repository=registry/image \
  --set image.tag=2.0.0-beta

# 5. Watch startup
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f
```

---

## Next Steps

**Please run these commands on your cluster and share the output:**

```bash
# Get the exact error
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=100

# Or if you know the pod name:
kubectl logs -n unified-replication-system <POD_NAME> --previous
```

This will show the exact panic/error message, and I can provide a specific fix!

