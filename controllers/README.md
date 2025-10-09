# UnifiedVolumeReplication Controller

This directory contains the main controller implementation for the Unified Replication Operator. The controller orchestrates all replication operations by coordinating between the Kubernetes API, backend adapters, and translation/discovery engines.

## Architecture

The controller follows the Kubernetes operator pattern and implements the reconciliation loop for `UnifiedVolumeReplication` resources.

### Main Components

1. **UnifiedVolumeReplicationReconciler** - Main controller struct
2. **Reconcile Loop** - Handles resource lifecycle
3. **Finalizer Management** - Ensures proper cleanup
4. **Status Management** - Tracks replication state
5. **Adapter Integration** - Delegates backend operations

## Controller Structure

```go
type UnifiedVolumeReplicationReconciler struct {
    client.Client
    Log             logr.Logger
    Scheme          *runtime.Scheme
    Recorder        record.EventRecorder
    AdapterRegistry adapters.Registry
    
    
    // Configuration
    MaxConcurrentReconciles int
    ReconcileTimeout        time.Duration
}
```

## Reconciliation Flow

### 1. Resource Fetch
- Fetch `UnifiedVolumeReplication` resource
- Handle not found (resource deleted)
- Initialize status if needed

### 2. Deletion Handling
- Check for deletion timestamp
- Execute cleanup via adapter
- Remove finalizer
- Allow resource deletion

### 3. Finalizer Management
- Add finalizer on first reconcile
- Ensures cleanup happens before deletion
- Requeue after adding finalizer

### 4. Operation Determination
The controller determines the required operation based on resource state:

- **create**: New resource (no conditions or no Ready condition)
- **update**: Generation changed or Ready=false
- **sync**: Up-to-date resource, just sync status

### 5. Operation Execution
- **Create**: Call `adapter.CreateReplication()`
- **Update**: Call `adapter.UpdateReplication()`
- **Sync**: Update status from adapter

### 6. Status Update
- Fetch current status from adapter
- Update conditions
- Set observed generation
- Persist status to API

### 7. Requeue Strategy
- Success: Requeue after 30s
- Error: Requeue after 10s
- Fast operations: 5s

## Status Conditions

The controller maintains the following status conditions:

### Ready Condition
Indicates overall status of the replication relationship.

**States:**
- `True` - Replication is operating normally
- `False` - Replication has errors or is not ready
- `Unknown` - Status cannot be determined

**Reasons:**
- `ReconciliationSucceeded` - Normal operation
- `ValidationFailed` - Spec validation error
- `AdapterError` - Backend adapter error
- `InitializationFailed` - Adapter initialization failed
- `OperationFailed` - Backend operation failed

### Synced Condition
Indicates successful status synchronization from backend.

**Reasons:**
- `StatusUpdated` - Status successfully retrieved from backend

## Lifecycle Management

### Resource Creation
1. Resource created in Kubernetes
2. First reconcile adds finalizer
3. Second reconcile creates backend replication
4. Status updated with backend state
5. Ready condition set to True

### Resource Update
1. User modifies spec (generation increments)
2. Controller detects generation mismatch
3. Calls adapter UpdateReplication
4. Status synchronized from backend
5. Observed generation updated

### Resource Deletion
1. User deletes resource (deletion timestamp set)
2. Controller calls adapter DeleteReplication
3. Backend resources cleaned up
4. Finalizer removed
5. Resource deleted from Kubernetes

## Configuration

### Environment Variables
- `MAX_CONCURRENT_RECONCILES` - Maximum concurrent reconciliations (default: 1)
- `RECONCILE_TIMEOUT` - Timeout for reconciliation (default: 5m)

### Reconciler Options
```go
reconciler := &UnifiedVolumeReplicationReconciler{
    Client:                  mgr.GetClient(),
    Log:                     ctrl.Log.WithName("controllers").WithName("UnifiedVolumeReplication"),
    Scheme:                  mgr.GetScheme(),
    Recorder:                mgr.GetEventRecorderFor("unifiedvolumereplication-controller"),
    AdapterRegistry:         registry,
    MaxConcurrentReconciles: 3,
    ReconcileTimeout:        5 * time.Minute,
}
```


## RBAC Permissions

The controller requires the following permissions:

```yaml
# UnifiedVolumeReplication resources
- apiGroups: ["replication.storage.io"]
  resources: ["unifiedvolumereplications"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Status subresource
- apiGroups: ["replication.storage.io"]
  resources: ["unifiedvolumereplications/status"]
  verbs: ["get", "update", "patch"]

# Finalizers
- apiGroups: ["replication.storage.io"]
  resources: ["unifiedvolumereplications/finalizers"]
  verbs: ["update"]

# Events
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
```

## Error Handling

### Error Types
1. **Validation Errors** - Spec validation failures
2. **Adapter Errors** - Backend adapter errors
3. **Kubernetes API Errors** - API server communication errors
4. **Timeout Errors** - Operation timeout

### Error Recovery
- Automatic requeue with backoff
- Status conditions reflect error state
- Events emitted for visibility
- Logs include error details

### Retry Strategy
- Transient errors: Requeue after 10s
- Permanent errors: Update status, requeue after 30s
- Kubernetes API errors: Let controller-runtime handle

## Testing

### Unit Tests
```bash
# Run all unit tests
go test -v ./controllers/... -run "^TestReconciler_"

# Specific tests
go test -v ./controllers -run TestReconciler_BasicLifecycle
go test -v ./controllers -run TestReconciler_StatusUpdate
go test -v ./controllers -run TestReconciler_Deletion
go test -v ./controllers -run TestReconciler_ConditionManagement
go test -v ./controllers -run TestReconciler_OperationDetermination
```

### Integration Tests
```bash
# Run integration tests (requires envtest or -short flag)
go test -v ./controllers/... -run "^TestControllerIntegration"

# Skip tests requiring envtest
go test -v -short ./controllers/...
```

### Ginkgo Tests
```bash
# Run Ginkgo test suite (requires envtest setup)
ginkgo -v ./controllers

# Or via go test
go test -v ./controllers -ginkgo.v
```

## Development

### Adding New Operations

1. Add operation to `determineOperation()`:
```go
func (r *UnifiedVolumeReplicationReconciler) determineOperation(uvr *...) string {
    // ... existing logic
    if needsNewOperation(uvr) {
        return "new-operation"
    }
    return "sync"
}
```

2. Add handler in `reconcileReplication()`:
```go
case "new-operation":
    opErr = r.handleNewOperation(ctx, adapter, uvr, log)
```

3. Implement handler method:
```go
func (r *...) handleNewOperation(ctx, adapter, uvr, log) error {
    // Implementation
}
```

### Adding New Conditions

1. Update condition in appropriate handler:
```go
r.updateCondition(uvr, metav1.Condition{
    Type:               "NewCondition",
    Status:             metav1.ConditionTrue,
    Reason:             "SomeReason",
    Message:            "Descriptive message",
    ObservedGeneration: uvr.Generation,
})
```

2. Update status:
```go
if err := r.Status().Update(ctx, uvr); err != nil {
    return err
}
```

## Debugging

### Enable Verbose Logging
```go
// In main.go or test setup
ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
```

### View Reconciliation Events
```bash
kubectl get events -n <namespace> --sort-by='.lastTimestamp'
```

### Check Status
```bash
kubectl get uvr <name> -n <namespace> -o yaml
kubectl describe uvr <name> -n <namespace>
```

### View Logs
```bash
kubectl logs -n <operator-namespace> deployment/unified-replication-operator -f
```

## Common Issues

### Finalizer Not Removed
- **Symptom**: Resource stuck in terminating state
- **Cause**: Backend deletion failed
- **Solution**: Check adapter logs, manually remove finalizer if needed

### Status Not Updating
- **Symptom**: Status conditions outdated
- **Cause**: Adapter status retrieval failing
- **Solution**: Check adapter status, verify backend connectivity

### Rapid Reconciliations
- **Symptom**: High reconciliation frequency
- **Cause**: Operation failures or status changes
- **Solution**: Check error conditions, adjust requeue delays

## Next Steps

This controller foundation will be enhanced in subsequent prompts:

- **Prompt 4.2**: Integration with discovery and translation engines
- **Prompt 4.3**: Advanced features (retry logic, circuit breakers)
- **Prompt 5.1**: Security hardening and admission webhooks

## Files

- `unifiedvolumereplication_controller.go` - Main controller implementation
- `suite_test.go` - Ginkgo test suite setup
- `unifiedvolumereplication_controller_test.go` - Ginkgo BDD tests
- `controller_unit_test.go` - Traditional unit tests
- `controller_integration_test.go` - Integration tests
- `README.md` - This file

