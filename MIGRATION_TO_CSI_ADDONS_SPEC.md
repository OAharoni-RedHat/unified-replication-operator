# Migration to kubernetes-csi-addons Compatible Spec

## Overview

This document provides a step-by-step migration plan to transform the unified-replication-operator from its current complex multi-endpoint specification to a kubernetes-csi-addons compatible specification. The migration follows **Option B** (using our own API group `replication.unified.io` with kubernetes-csi-addons-compatible structure) while maintaining architectural flexibility to transition to **Option A** (using `replication.storage.openshift.io` API group) in the future.

## Migration Strategy

### Goals
1. ✅ Adopt kubernetes-csi-addons `VolumeReplication` spec structure as our input API
2. ✅ Maintain translation capabilities to Trident and Dell backends
3. ✅ Keep our API group (`replication.unified.io`) for control and compatibility
4. ✅ Enable easy future migration to Option A if desired
5. ✅ Minimize breaking changes where possible through versioning

### Architecture Overview

**Current State:**
```
UnifiedVolumeReplication (complex spec)
  ↓
Controller → Adapter Registry
  ↓
Backend CRs (Ceph VolumeReplication, TridentMirrorRelationship, DellCSIReplicationGroup)
```

**Target State:**
```
VolumeReplication (simple, kubernetes-csi-addons compatible spec)
  ↓
VolumeReplicationClass lookup (backend detection + parameters)
  ↓
Controller → Adapter Registry (enhanced translation)
  ↓
Backend CRs (Ceph VolumeReplication, TridentMirrorRelationship, DellCSIReplicationGroup)
```

## Phased Implementation

The migration is divided into **8 phases**, each with specific prompts that can be executed sequentially.

---

## Phase 1: API Research and Structure Planning

### Prompt 1.1: Research kubernetes-csi-addons VolumeReplication Spec

Research and document the exact structure of the kubernetes-csi-addons VolumeReplication CRD from the official repository (github.com/csi-addons/kubernetes-csi-addons).

Create a document at `docs/api-reference/CSI_ADDONS_SPEC_REFERENCE.md` that includes:

1. The complete VolumeReplicationSpec structure with all fields, types, and validation rules
2. The complete VolumeReplicationStatus structure
3. The VolumeReplicationClass spec structure
4. Valid values for replicationState field (primary, secondary, resync, etc.)
5. Any kubebuilder markers used for validation
6. Example YAMLs from the kubernetes-csi-addons repository
7. Links to the source code for reference

This document will serve as the single source of truth for ensuring our implementation matches kubernetes-csi-addons exactly.

### Prompt 1.2: Create Migration Architecture Document

Create a detailed architecture document at `docs/architecture/MIGRATION_ARCHITECTURE.md` that outlines:

1. **API Version Strategy:**
   - Keep v1alpha1 for backward compatibility (deprecated)
   - Create v1alpha2 with new kubernetes-csi-addons compatible structure
   - Document API group decision: `replication.unified.io` (Option B)
   - Plan for future Option A transition using conversion webhooks

2. **Backward Compatibility Plan:**
   - How v1alpha1 (UnifiedVolumeReplication) will coexist with v1alpha2 (VolumeReplication)
   - Timeline for v1alpha1 deprecation (suggested: 12 months)
   - Migration path for existing users

3. **Backend Detection Strategy:**
   - How VolumeReplicationClass.spec.provisioner will map to backends:
     * Contains "ceph" → Ceph backend
     * Contains "trident" → Trident backend  
     * Contains "powerstore" or "dellemc" → Dell backend
   - Fallback mechanism if provisioner is ambiguous

4. **Translation Strategy:**
   - Ceph: Passthrough (native kubernetes-csi-addons compatibility)
   - Trident: Translate primary/secondary → established/reestablishing
   - Dell: Translate primary/secondary → Failover/Sync actions
   - How VolumeReplicationClass.parameters map to backend-specific configs

5. **Option A Future-Proofing:**
   - Use identical struct definitions as kubernetes-csi-addons
   - Document where to add conversion webhooks in the future
   - RBAC considerations for dual API group support
   - Testing strategy for API compatibility

Include diagrams showing the flow from VolumeReplication → Backend CRs for each adapter.

---

## Phase 2: Create New API Types (v1alpha2)

### Prompt 2.1: Create VolumeReplication Types (v1alpha2)

Create a new API version v1alpha2 at `api/v1alpha2/volumereplication_types.go` that defines the VolumeReplication CRD matching kubernetes-csi-addons specification exactly.

Requirements:

1. **Copy structure from kubernetes-csi-addons** (reference: docs/api-reference/CSI_ADDONS_SPEC_REFERENCE.md)

2. **VolumeReplicationSpec must include:**
   - volumeReplicationClass: string (required)
   - pvcName: string (required)  
   - replicationState: string enum (primary, secondary, resync) (required)
   - dataSource: *corev1.TypedLocalObjectReference (optional)
   - autoResync: *bool (optional)

3. **VolumeReplicationStatus must include:**
   - conditions: []metav1.Condition
   - state: string
   - message: string  
   - lastSyncTime: *metav1.Time
   - lastSyncDuration: *metav1.Duration
   - observedGeneration: int64

4. **Add kubebuilder markers:**
   - API group: replication.unified.io
   - Version: v1alpha2
   - Resource shortNames: vr, volrep
   - Status subresource
   - Validation: Required fields, state enum validation
   - PrintColumns: Name, PVC, State, Class, Age

5. **Add comment at top of file:**
   ```go
   // COMPATIBILITY NOTICE:
   // This API version (v1alpha2) is designed to be binary-compatible with
   // kubernetes-csi-addons replication.storage.openshift.io/v1alpha1.
   // 
   // DO NOT add custom fields to VolumeReplicationSpec or VolumeReplicationStatus.
   // Use VolumeReplicationClass.parameters for backend-specific configuration.
   //
   // This compatibility ensures future migration to Option A (using 
   // replication.storage.openshift.io API group directly) will be straightforward.
   ```

6. **Include all required methods:**
   - DeepCopy methods (will be generated)
   - List type
   - Init() method for defaulting

After creating the file, run `make generate` to generate deepcopy code.

### Prompt 2.2: Create VolumeReplicationClass Types (v1alpha2)

Create VolumeReplicationClass CRD at `api/v1alpha2/volumereplicationclass_types.go` matching kubernetes-csi-addons specification.

Requirements:

1. **VolumeReplicationClassSpec must include:**
   - provisioner: string (required) - identifies the backend
   - parameters: map[string]string (optional) - backend-specific configuration

2. **Add kubebuilder markers:**
   - API group: replication.unified.io
   - Version: v1alpha2
   - Scope: Cluster (not namespaced)
   - Resource shortNames: vrc, volrepclass
   - Validation: provisioner required and non-empty
   - PrintColumns: Provisioner, Age

3. **Add documentation comments for common parameters:**
   ```go
   // Common parameters across backends:
   // - replication.storage.openshift.io/replication-secret-name: Secret for authentication
   // - replication.storage.openshift.io/replication-secret-namespace: Secret namespace
   //
   // Ceph-specific parameters:
   // - mirroringMode: "snapshot" or "journal"  
   // - schedulingInterval: "5m", "15m", etc.
   //
   // Trident-specific parameters:
   // - replicationPolicy: "Async" or "Sync"
   // - replicationSchedule: "15m", "1h", etc.
   // - remoteCluster: Name of remote cluster
   // - remoteSVM: SVM name for remote cluster
   //
   // Dell PowerStore-specific parameters:
   // - protectionPolicy: Policy name (e.g., "15min-async")
   // - remoteSystem: Remote system ID
   // - rpo: RPO value (e.g., "15m")
   ```

4. **Add compatibility note:**
   ```go
   // COMPATIBILITY NOTICE:
   // This structure matches kubernetes-csi-addons VolumeReplicationClass.
   // The provisioner field is used to detect which backend adapter to use.
   ```

5. No status subresource needed (VolumeReplicationClass is configuration only).

After creating, run `make generate` and `make manifests` to generate CRDs.

### Prompt 2.3: Create GroupVersion Info for v1alpha2

Create `api/v1alpha2/groupversion_info.go` to register the v1alpha2 API version.

Requirements:

1. **Package declaration and imports:**
   ```go
   // Package v1alpha2 contains API Schema definitions for the replication v1alpha2 API group
   // This version is kubernetes-csi-addons compatible
   // +kubebuilder:object:generate=true
   // +groupName=replication.unified.io
   package v1alpha2
   ```

2. **Register GroupVersion:**
   - GroupVersion = "replication.unified.io/v1alpha2"

3. **AddToScheme function** for scheme registration

4. **Add documentation:**
   ```go
   // This API version (v1alpha2) provides kubernetes-csi-addons compatible
   // VolumeReplication and VolumeReplicationClass resources.
   //
   // It replaces v1alpha1 (UnifiedVolumeReplication) with a simpler, 
   // standard-compliant API while maintaining multi-backend translation
   // capabilities through adapters.
   ```

Reference `api/v1alpha1/groupversion_info.go` for structure, but update for v1alpha2.

### Prompt 2.4: Update Main.go to Register v1alpha2 Scheme

Update `main.go` to register the new v1alpha2 API types in the scheme.

Requirements:

1. **Add import:**
   ```go
   replicationv1alpha2 "github.com/unified-replication/operator/api/v1alpha2"
   ```

2. **Register in init() or main():**
   ```go
   utilruntime.Must(replicationv1alpha2.AddToScheme(scheme))
   ```

3. **Keep v1alpha1 registration** for backward compatibility:
   ```go
   utilruntime.Must(replicationv1alpha1.AddToScheme(scheme))
   ```

4. **Add comment explaining dual version support:**
   ```go
   // Register both API versions:
   // - v1alpha1: Legacy UnifiedVolumeReplication (deprecated, will be removed in future)
   // - v1alpha2: kubernetes-csi-addons compatible VolumeReplication
   ```

After updating, verify with `go build`.

### Prompt 2.5: Generate CRD Manifests

Generate the CRD manifests for the new v1alpha2 types.

Steps:

1. Run `make manifests` to generate CRDs
2. Verify new files created:
   - `config/crd/bases/replication.unified.io_volumereplications.yaml` (v1alpha2)
   - `config/crd/bases/replication.unified.io_volumereplicationclasses.yaml`

3. Review the generated CRDs and verify:
   - API version is correct (replication.unified.io/v1alpha2)
   - Required fields have validation
   - Enum values for replicationState are correct
   - PrintColumns are defined
   - Status subresource is enabled for VolumeReplication

4. Create sample VolumeReplicationClass YAMLs at `config/samples/`:
   - `volumereplicationclass_ceph.yaml` (Ceph backend)
   - `volumereplicationclass_trident.yaml` (Trident backend)
   - `volumereplicationclass_powerstore.yaml` (Dell backend)

5. Create sample VolumeReplication YAMLs at `config/samples/`:
   - `volumereplication_ceph_primary.yaml`
   - `volumereplication_trident_secondary.yaml`
   - `volumereplication_powerstore_primary.yaml`

Include detailed comments in sample YAMLs explaining each field.

---

## Phase 2B: Volume Group Replication Support (OPTIONAL BUT RECOMMENDED)

**Note:** Volume Group Replication is part of kubernetes-csi-addons for replicating multiple PVCs together as a single unit. This is critical for multi-volume applications like databases. See `VOLUME_GROUP_REPLICATION_ADDENDUM.md` for full details.

**Decision Point:** Implement Phase 2B now for complete kubernetes-csi-addons compatibility, or defer to post-v2.0.0 release.

### Prompt 2B.1: Create VolumeGroupReplication Types (v1alpha2)

Create `api/v1alpha2/volumegroupreplication_types.go` for volume group replication support.

Requirements:

1. **VolumeGroupReplicationSpec must include:**
   - volumeGroupReplicationClass: string (required)
   - selector: *metav1.LabelSelector (required) - selects PVCs by labels
   - replicationState: enum (primary, secondary, resync) (required)
   - autoResync: *bool (optional)
   - source: *corev1.TypedLocalObjectReference (optional)

2. **VolumeGroupReplicationStatus must include:**
   - conditions: []metav1.Condition
   - state: string
   - message: string
   - lastSyncTime: *metav1.Time
   - lastSyncDuration: *metav1.Duration
   - observedGeneration: int64
   - persistentVolumeClaimsRefList: []corev1.LocalObjectReference - list of PVCs

3. **Kubebuilder markers:**
   - Resource shortNames: vgr, volgrouprep
   - Status subresource, storage version
   - PrintColumns: State, Class, PVCs (count), Age

4. **Add compatibility notice** matching kubernetes-csi-addons VolumeGroupReplication

After creating, run `make generate`.

### Prompt 2B.2: Create VolumeGroupReplicationClass Types (v1alpha2)

Create `api/v1alpha2/volumegroupreplicationclass_types.go`.

Requirements:

1. **VolumeGroupReplicationClassSpec:**
   - provisioner: string (required)
   - parameters: map[string]string (optional)

2. **Document group-specific parameters:**
   - consistencyGroup: "enabled"
   - groupSnapshots: "true"
   - Backend-specific group parameters (Ceph, Trident, Dell)

3. **Kubebuilder markers:**
   - Scope: Cluster
   - ShortNames: vgrc, volgrouprepclass

After creating, run `make generate` and `make manifests`.

### Prompt 2B.3: Update GroupVersion Init for Volume Groups

Update `api/v1alpha2/groupversion_info.go` to register volume group types:

```go
func init() {
    SchemeBuilder.Register(
        &VolumeReplication{}, &VolumeReplicationList{},
        &VolumeGroupReplication{}, &VolumeGroupReplicationList{},
        &VolumeReplicationClass{}, &VolumeReplicationClassList{},
        &VolumeGroupReplicationClass{}, &VolumeGroupReplicationClassList{},
    )
}
```

### Prompt 2B.4: Create Volume Group Sample YAMLs

Create sample YAMLs at `config/samples/`:

1. **`volumegroupreplicationclass_ceph_group.yaml`** - Ceph group class with crash consistency
2. **`volumegroupreplication_postgresql.yaml`** - Multi-volume database replication example
3. **`volumegroupreplicationclass_powerstore_group.yaml`** - Dell Metro/Async group replication

Include detailed comments explaining:
- How label selectors work
- Why groups are needed for databases
- Consistency guarantees
- Example PVC labels

**Note:** If deferring Phase 2B, skip to Phase 3. Volume groups can be added later as v2.1.0 feature.

---

## Phase 3: Create New Controller for VolumeReplication

### Prompt 3.1: Create VolumeReplication Controller Scaffold

Create a new controller at `controllers/volumereplication_controller.go` for reconciling the v1alpha2 VolumeReplication resources.

Requirements:

1. **Controller structure:**
   ```go
   type VolumeReplicationReconciler struct {
       client.Client
       Scheme          *runtime.Scheme
       AdapterRegistry *adapters.Registry
   }
   ```

2. **Basic reconciliation loop:**
   - Fetch VolumeReplication resource
   - Handle deletion with finalizer
   - Fetch VolumeReplicationClass  
   - Detect backend from VolumeReplicationClass.Spec.Provisioner
   - Get appropriate adapter
   - Call adapter's reconciliation method
   - Update VolumeReplication status

3. **RBAC markers:**
   ```go
   //+kubebuilder:rbac:groups=replication.unified.io,resources=volumereplications,verbs=get;list;watch;create;update;patch;delete
   //+kubebuilder:rbac:groups=replication.unified.io,resources=volumereplications/status,verbs=get;update;patch
   //+kubebuilder:rbac:groups=replication.unified.io,resources=volumereplications/finalizers,verbs=update
   //+kubebuilder:rbac:groups=replication.unified.io,resources=volumereplicationclasses,verbs=get;list;watch
   ```

4. **SetupWithManager:**
   - Watch VolumeReplication resources
   - Watch VolumeReplicationClass resources (trigger reconcile of dependent VRs)
   - Watch PVCs (to detect changes)

5. **Helper methods:**
   - `detectBackend(provisioner string) adapters.BackendType`
   - `fetchVolumeReplicationClass(ctx, name) (*VolumeReplicationClass, error)`
   - `updateStatus(ctx, vr, state, message) error`

6. **Keep for now but mark deprecated:** `controllers/unifiedvolumereplication_controller.go`
   - Add comment: "// DEPRECATED: This controller is for v1alpha1 API. Use volumereplication_controller.go for v1alpha2."
   - Keep functional for backward compatibility

Do not implement full business logic yet - focus on the scaffold and basic structure.

### Prompt 3.2: Implement Backend Detection Logic

Implement the backend detection logic in the VolumeReplication controller.

Add to `controllers/volumereplication_controller.go`:

1. **Backend detection method:**
   ```go
   func (r *VolumeReplicationReconciler) detectBackend(provisioner string) (adapters.BackendType, error) {
       // Detect based on provisioner string
       // Return adapters.BackendCeph, adapters.BackendTrident, adapters.BackendDell, or error
   }
   ```

2. **Detection rules:**
   - If provisioner contains "ceph" or "rbd.csi.ceph.com" → BackendCeph
   - If provisioner contains "trident" or "csi.trident.netapp.io" → BackendTrident
   - If provisioner contains "powerstore" or "dellemc" or "csi-powerstore.dellemc.com" → BackendDell
   - Otherwise → return error "unknown backend"

3. **Add logging:**
   - Log detected backend: `log.Info("Detected backend", "provisioner", provisioner, "backend", backend)`

4. **Fallback mechanism:**
   - If provisioner detection fails, try to detect from PVC's StorageClass provisioner
   - Add method: `detectBackendFromPVC(ctx context.Context, pvcName, namespace string) (BackendType, error)`

5. **Unit tests:**
   Create `controllers/volumereplication_controller_backend_detection_test.go`:
   - Test detection from various provisioner strings
   - Test fallback to PVC StorageClass
   - Test error cases for unknown backends

### Prompt 3.3: Implement VolumeReplicationClass Lookup and Validation

Implement VolumeReplicationClass lookup and validation in the controller.

Add to `controllers/volumereplication_controller.go`:

1. **VolumeReplicationClass fetch method:**
   ```go
   func (r *VolumeReplicationReconciler) fetchVolumeReplicationClass(
       ctx context.Context,
       className string,
   ) (*replicationv1alpha2.VolumeReplicationClass, error) {
       // Fetch from cluster scope
       // Return error if not found
       // Validate required fields
   }
   ```

2. **Validation logic:**
   - VolumeReplicationClass must exist
   - Provisioner field must not be empty
   - Parameters must be valid map (can be empty)

3. **Error handling:**
   - If VolumeReplicationClass not found, set VolumeReplication status condition:
     * Type: "Ready"
     * Status: "False"
     * Reason: "VolumeReplicationClassNotFound"
     * Message: "VolumeReplicationClass 'xxx' not found"
   - Don't requeue immediately, wait for VolumeReplicationClass creation

4. **Watch VolumeReplicationClass changes:**
   - In SetupWithManager, add handler to requeue VolumeReplications when their class changes
   - Use EnqueueRequestsFromMapFunc to map VolumeReplicationClass → VolumeReplications

5. **Unit tests:**
   Add tests to `controllers/volumereplication_controller_test.go`:
   - Test successful VolumeReplicationClass lookup
   - Test VolumeReplicationClass not found error handling
   - Test invalid VolumeReplicationClass handling

### Prompt 3.4: Implement Core Reconciliation Logic

Implement the core reconciliation logic for VolumeReplication in the controller.

Update `controllers/volumereplication_controller.go`:

1. **Complete Reconcile method:**
   ```go
   func (r *VolumeReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
       log := log.FromContext(ctx)
       
       // 1. Fetch VolumeReplication
       vr := &replicationv1alpha2.VolumeReplication{}
       if err := r.Get(ctx, req.NamespacedName, vr); err != nil {
           return ctrl.Result{}, client.IgnoreNotFound(err)
       }
       
       // 2. Handle deletion
       if !vr.DeletionTimestamp.IsZero() {
           return r.handleDeletion(ctx, vr)
       }
       
       // 3. Add finalizer if missing
       if !controllerutil.ContainsFinalizer(vr, finalizerName) {
           controllerutil.AddFinalizer(vr, finalizerName)
           return ctrl.Result{}, r.Update(ctx, vr)
       }
       
       // 4. Fetch VolumeReplicationClass
       vrc, err := r.fetchVolumeReplicationClass(ctx, vr.Spec.VolumeReplicationClass)
       if err != nil {
           return r.handleError(ctx, vr, err)
       }
       
       // 5. Detect backend
       backend, err := r.detectBackend(vrc.Spec.Provisioner)
       if err != nil {
           return r.handleError(ctx, vr, err)
       }
       
       // 6. Get adapter
       adapter := r.AdapterRegistry.GetAdapter(backend)
       if adapter == nil {
           return r.handleError(ctx, vr, fmt.Errorf("no adapter for backend %s", backend))
       }
       
       // 7. Reconcile with adapter
       result, err := adapter.ReconcileVolumeReplication(ctx, vr, vrc)
       if err != nil {
           return r.handleError(ctx, vr, err)
       }
       
       // 8. Update status
       return result, r.updateStatus(ctx, vr, "Ready", "Replication configured successfully")
   }
   ```

2. **Helper methods to implement:**
   - `handleDeletion(ctx, vr) (Result, error)` - clean up backend resources
   - `handleError(ctx, vr, err) (Result, error)` - update status with error
   - `updateStatus(ctx, vr, condition, message) error` - update status conditions

3. **Status management:**
   - Use meta.SetStatusCondition to update conditions
   - Update state field based on backend status
   - Update lastSyncTime from backend
   - Update observedGeneration

4. **Error handling:**
   - Transient errors: requeue with backoff
   - Permanent errors: don't requeue, wait for spec change
   - Log all errors with context

Add comprehensive logging throughout reconciliation.

---

## Phase 4: Refactor and Enhance Adapters

### Prompt 4.1: Create Adapter Interface for v1alpha2

Create a new adapter interface that works with v1alpha2 VolumeReplication types.

Update `pkg/adapters/types.go`:

1. **Add new interface:**
   ```go
   // VolumeReplicationAdapter handles reconciliation for v1alpha2 VolumeReplication
   type VolumeReplicationAdapter interface {
       // ReconcileVolumeReplication reconciles a VolumeReplication resource
       ReconcileVolumeReplication(
           ctx context.Context,
           vr *replicationv1alpha2.VolumeReplication,
           vrc *replicationv1alpha2.VolumeReplicationClass,
       ) (ctrl.Result, error)
       
       // DeleteVolumeReplication cleans up backend resources
       DeleteVolumeReplication(
           ctx context.Context,
           vr *replicationv1alpha2.VolumeReplication,
       ) error
       
       // GetStatus fetches current replication status from backend
       GetStatus(
           ctx context.Context,
           vr *replicationv1alpha2.VolumeReplication,
       ) (*ReplicationStatus, error)
   }
   ```

2. **Keep old Adapter interface** for v1alpha1 backward compatibility:
   - Rename to `UnifiedVolumeReplicationAdapter`
   - Mark as deprecated

3. **Common ReplicationStatus struct:**
   ```go
   type ReplicationStatus struct {
       State            string
       Message          string
       LastSyncTime     *metav1.Time
       LastSyncDuration *metav1.Duration
       Conditions       []metav1.Condition
   }
   ```

4. **Update Registry to support both interfaces:**
   ```go
   type Registry struct {
       // For v1alpha2
       volumeReplicationAdapters map[BackendType]VolumeReplicationAdapter
       
       // For v1alpha1 (deprecated)
       unifiedAdapters map[BackendType]UnifiedVolumeReplicationAdapter
   }
   
   func (r *Registry) GetVolumeReplicationAdapter(backend BackendType) VolumeReplicationAdapter
   func (r *Registry) GetUnifiedAdapter(backend BackendType) UnifiedVolumeReplicationAdapter
   ```

This maintains backward compatibility while enabling new v1alpha2 functionality.

### Prompt 4.2: Implement Ceph Adapter for v1alpha2 (Passthrough)

Implement the Ceph adapter for v1alpha2 VolumeReplication. Since Ceph uses kubernetes-csi-addons natively, this is mostly passthrough.

Update `pkg/adapters/ceph.go`:

1. **Add method to CephAdapter:**
   ```go
   func (a *CephAdapter) ReconcileVolumeReplication(
       ctx context.Context,
       vr *replicationv1alpha2.VolumeReplication,
       vrc *replicationv1alpha2.VolumeReplicationClass,
   ) (ctrl.Result, error) {
       // For Ceph, VolumeReplication is native kubernetes-csi-addons
       // Just ensure backend VolumeReplication CR exists and matches
   }
   ```

2. **Implementation steps:**
   - Create or update Ceph VolumeReplication CR with same spec
   - Use API group: `replication.storage.openshift.io/v1alpha1`
   - Set owner reference to our VolumeReplication
   - Map fields directly (no translation needed):
     * volumeReplicationClass → volumeReplicationClass
     * pvcName → pvcName
     * replicationState → replicationState
     * autoResync → autoResync

3. **Extract Ceph-specific parameters from VolumeReplicationClass:**
   - `mirroringMode`: Pass to Ceph VolumeReplicationClass
   - `schedulingInterval`: Pass to Ceph VolumeReplicationClass
   - `replication.storage.openshift.io/*`: Pass through to Ceph

4. **Status synchronization:**
   - Read Ceph VolumeReplication status
   - Copy to our VolumeReplication status
   - Map conditions, state, lastSyncTime, lastSyncDuration

5. **Deletion:**
   ```go
   func (a *CephAdapter) DeleteVolumeReplication(ctx, vr) error {
       // Delete backend Ceph VolumeReplication CR
       // Owner reference handles this automatically, but clean up explicitly for safety
   }
   ```

6. **Add comments:**
   ```go
   // Ceph adapter is mostly passthrough since Ceph uses kubernetes-csi-addons natively.
   // We create a backend VolumeReplication CR in replication.storage.openshift.io/v1alpha1
   // and mirror its status back to our CR.
   ```

This is the simplest adapter since it's nearly 1:1 mapping.

### Prompt 4.3: Implement Trident Adapter for v1alpha2 (Translation)

Implement the Trident adapter for v1alpha2 VolumeReplication with translation from kubernetes-csi-addons spec to TridentMirrorRelationship.

Update `pkg/adapters/trident.go`:

1. **Add method to TridentAdapter:**
   ```go
   func (a *TridentAdapter) ReconcileVolumeReplication(
       ctx context.Context,
       vr *replicationv1alpha2.VolumeReplication,
       vrc *replicationv1alpha2.VolumeReplicationClass,
   ) (ctrl.Result, error) {
       // Translate VolumeReplication to TridentMirrorRelationship
   }
   ```

2. **State translation:**
   ```go
   func (a *TridentAdapter) translateStateToTrident(vrState string) string {
       switch vrState {
       case "primary":
           return "established"  // Trident primary state
       case "secondary":
           return "reestablishing"  // Trident secondary/replica state
       case "resync":
           return "reestablishing"  // Re-establish mirror
       default:
           return "established"  // Default to established
       }
   }
   ```

3. **Parameter extraction from VolumeReplicationClass:**
   - `replicationPolicy`: "Async" or "Sync" (default: "Async")
   - `replicationSchedule`: e.g., "15m", "1h"
   - `remoteCluster`: Remote cluster name
   - `remoteSVM`: Remote SVM name
   - `remoteVolume`: Remote volume handle (or derive from PVC)

4. **Create TridentMirrorRelationship:**
   ```go
   tmr := &TridentMirrorRelationship{
       ObjectMeta: metav1.ObjectMeta{
           Name:      vr.Name,
           Namespace: vr.Namespace,
           OwnerReferences: []metav1.OwnerReference{
               *metav1.NewControllerRef(vr, replicationv1alpha2.GroupVersion.WithKind("VolumeReplication")),
           },
       },
       Spec: TridentMirrorRelationshipSpec{
           State:                a.translateStateToTrident(vr.Spec.ReplicationState),
           ReplicationPolicy:    vrc.Spec.Parameters["replicationPolicy"],
           ReplicationSchedule:  vrc.Spec.Parameters["replicationSchedule"],
           VolumeMappings: []VolumeMappingType{
               {
                   LocalPVCName:       vr.Spec.PvcName,
                   RemoteVolumeHandle: vrc.Spec.Parameters["remoteVolume"],
               },
           },
       },
   }
   ```

5. **Status synchronization:**
   - Read TridentMirrorRelationship status
   - Translate Trident state back to kubernetes-csi-addons state:
     * "established" → "primary"
     * "reestablishing" → "secondary"
   - Extract last sync information if available
   - Map conditions

6. **Deletion:**
   - Delete TridentMirrorRelationship
   - Optionally delete TridentActionMirrorUpdate if exists

7. **Error handling:**
   - If TridentMirrorRelationship CRD not found, return clear error
   - If parameters missing, use sensible defaults and log warning

Add detailed logging for translation steps.

### Prompt 4.4: Implement Dell PowerStore Adapter for v1alpha2 (Translation)

Implement the Dell PowerStore adapter for v1alpha2 VolumeReplication with translation to DellCSIReplicationGroup.

Update `pkg/adapters/powerstore.go`:

1. **Add method to PowerStoreAdapter:**
   ```go
   func (a *PowerStoreAdapter) ReconcileVolumeReplication(
       ctx context.Context,
       vr *replicationv1alpha2.VolumeReplication,
       vrc *replicationv1alpha2.VolumeReplicationClass,
   ) (ctrl.Result, error) {
       // Translate VolumeReplication to DellCSIReplicationGroup
   }
   ```

2. **State to Action translation:**
   ```go
   func (a *PowerStoreAdapter) translateStateToDellAction(vrState string) string {
       switch vrState {
       case "primary":
           return "Failover"  // Promote to primary (failover to this site)
       case "secondary":
           return "Sync"      // Operate as secondary (sync from primary)
       case "resync":
           return "Reprotect" // Re-establish replication after failover
       default:
           return "Sync"
       }
   }
   ```

3. **Parameter extraction from VolumeReplicationClass:**
   - `protectionPolicy`: Dell protection policy name (required)
   - `remoteSystem`: Remote system ID (required)
   - `rpo`: RPO value (e.g., "15m")
   - `remoteClusterId`: Remote cluster identifier

4. **PVC label management:**
   Dell uses label selectors to group PVCs. Ensure the PVC has appropriate labels:
   ```go
   // Add label to PVC for Dell replication group
   pvc := &corev1.PersistentVolumeClaim{}
   if err := a.client.Get(ctx, types.NamespacedName{
       Name: vr.Spec.PvcName, Namespace: vr.Namespace,
   }, pvc); err != nil {
       return ctrl.Result{}, err
   }
   
   if pvc.Labels == nil {
       pvc.Labels = make(map[string]string)
   }
   pvc.Labels["replication.storage.dell.com/replicated"] = "true"
   pvc.Labels["replication.storage.dell.com/group"] = vr.Name
   
   if err := a.client.Update(ctx, pvc); err != nil {
       return ctrl.Result{}, err
   }
   ```

5. **Create DellCSIReplicationGroup:**
   ```go
   drg := &DellCSIReplicationGroup{
       ObjectMeta: metav1.ObjectMeta{
           Name:      vr.Name,
           Namespace: vr.Namespace,
           OwnerReferences: []metav1.OwnerReference{
               *metav1.NewControllerRef(vr, replicationv1alpha2.GroupVersion.WithKind("VolumeReplication")),
           },
       },
       Spec: DellCSIReplicationGroupSpec{
           DriverName:       "csi-powerstore.dellemc.com",
           Action:           a.translateStateToDellAction(vr.Spec.ReplicationState),
           ProtectionPolicy: vrc.Spec.Parameters["protectionPolicy"],
           RemoteSystem:     vrc.Spec.Parameters["remoteSystem"],
           RemoteRPO:        vrc.Spec.Parameters["rpo"],
           PVCSelector: &metav1.LabelSelector{
               MatchLabels: map[string]string{
                   "replication.storage.dell.com/group": vr.Name,
               },
           },
       },
   }
   ```

6. **Status synchronization:**
   - Read DellCSIReplicationGroup status
   - Translate Dell state/action back:
     * "Synchronized" → "secondary"
     * "FailedOver" → "primary"
   - Extract last sync time from Dell status
   - Map conditions

7. **Deletion:**
   - Delete DellCSIReplicationGroup
   - Remove labels from PVC
   - Clean up any Dell-specific resources

Add extensive error handling for missing required parameters.

### Prompt 4.5: Update Adapter Registry for v1alpha2

Update the adapter registry to support both v1alpha1 (legacy) and v1alpha2 (new) adapters.

Update `pkg/adapters/registry.go`:

1. **Enhance Registry structure:**
   ```go
   type Registry struct {
       client              client.Client
       scheme              *runtime.Scheme
       
       // v1alpha2 adapters (new)
       vrAdapters          map[BackendType]VolumeReplicationAdapter
       
       // v1alpha1 adapters (legacy, deprecated)
       uvrAdapters         map[BackendType]UnifiedVolumeReplicationAdapter
   }
   ```

2. **Constructor updates:**
   ```go
   func NewRegistry(client client.Client, scheme *runtime.Scheme) *Registry {
       reg := &Registry{
           client:      client,
           scheme:      scheme,
           vrAdapters:  make(map[BackendType]VolumeReplicationAdapter),
           uvrAdapters: make(map[BackendType]UnifiedVolumeReplicationAdapter),
       }
       
       // Register v1alpha2 adapters
       reg.registerV1Alpha2Adapters()
       
       // Register v1alpha1 adapters (legacy)
       reg.registerV1Alpha1Adapters()
       
       return reg
   }
   ```

3. **Registration methods:**
   ```go
   func (r *Registry) registerV1Alpha2Adapters() {
       // Ceph adapter
       cephAdapter := NewCephAdapter(r.client, r.scheme)
       r.vrAdapters[BackendCeph] = cephAdapter
       
       // Trident adapter
       tridentAdapter := NewTridentAdapter(r.client, r.scheme)
       r.vrAdapters[BackendTrident] = tridentAdapter
       
       // PowerStore adapter
       psAdapter := NewPowerStoreAdapter(r.client, r.scheme)
       r.vrAdapters[BackendDell] = psAdapter
   }
   
   func (r *Registry) registerV1Alpha1Adapters() {
       // Keep old adapters for backward compatibility
       // ... existing code ...
   }
   ```

4. **Getter methods:**
   ```go
   func (r *Registry) GetVolumeReplicationAdapter(backend BackendType) VolumeReplicationAdapter {
       return r.vrAdapters[backend]
   }
   
   func (r *Registry) GetUnifiedAdapter(backend BackendType) UnifiedVolumeReplicationAdapter {
       return r.uvrAdapters[backend]
   }
   
   func (r *Registry) ListBackends() []BackendType {
       // Return all registered backends
   }
   ```

5. **Add backend availability check:**
   ```go
   func (r *Registry) IsBackendAvailable(ctx context.Context, backend BackendType) (bool, error) {
       // Check if required CRDs exist for the backend
       switch backend {
       case BackendCeph:
           return r.checkCRDExists(ctx, "volumereplications.replication.storage.openshift.io")
       case BackendTrident:
           return r.checkCRDExists(ctx, "tridentmirrorrelationships.trident.netapp.io")
       case BackendDell:
           return r.checkCRDExists(ctx, "dellcsireplicationgroups.replication.dell.com")
       }
   }
   ```

This enables clean separation between v1alpha1 and v1alpha2 workflows.

---

## Phase 5: Testing

### Prompt 5.1: Create Unit Tests for v1alpha2 Types

Create comprehensive unit tests for the new v1alpha2 types.

Create `api/v1alpha2/volumereplication_types_test.go`:

1. **Test VolumeReplication validation:**
   - Test required fields (volumeReplicationClass, pvcName, replicationState)
   - Test valid replicationState values (primary, secondary, resync)
   - Test invalid replicationState values (should fail)
   - Test optional fields (dataSource, autoResync)

2. **Test VolumeReplication defaulting:**
   - Test autoResync defaults to false if not specified
   - Test observedGeneration initialization

3. **Test deepcopy methods:**
   - Create VolumeReplication, deep copy it, verify independence
   - Modify copy, ensure original unchanged

Create `api/v1alpha2/volumereplicationclass_types_test.go`:

1. **Test VolumeReplicationClass validation:**
   - Test required provisioner field
   - Test empty provisioner (should fail)
   - Test parameters map (can be nil or empty)
   - Test parameters with various keys/values

2. **Test deepcopy:**
   - VolumeReplicationClass with nil parameters
   - VolumeReplicationClass with populated parameters

Run tests: `go test ./api/v1alpha2/... -v`

### Prompt 5.2: Create Unit Tests for VolumeReplication Controller

Create unit tests for the VolumeReplication controller.

Create `controllers/volumereplication_controller_test.go`:

1. **Test backend detection:**
   - Test detectBackend with various provisioner strings:
     * "rbd.csi.ceph.com" → BackendCeph
     * "csi.trident.netapp.io" → BackendTrident
     * "csi-powerstore.dellemc.com" → BackendDell
     * "unknown.provisioner.io" → error
   - Test fallback to PVC StorageClass provisioner

2. **Test VolumeReplicationClass lookup:**
   - Test successful lookup
   - Test VolumeReplicationClass not found error
   - Test invalid VolumeReplicationClass

3. **Test reconciliation flow:**
   - Test successful reconciliation with Ceph backend
   - Test successful reconciliation with Trident backend
   - Test successful reconciliation with Dell backend
   - Test VolumeReplicationClass not found handling
   - Test unknown backend handling

4. **Test status updates:**
   - Test setting Ready=True condition
   - Test setting Ready=False with error
   - Test state updates
   - Test observedGeneration updates

5. **Test deletion:**
   - Test finalizer added on creation
   - Test deletion with finalizer triggers backend cleanup
   - Test finalizer removed after cleanup

6. **Test watch configuration:**
   - Test VolumeReplication changes trigger reconcile
   - Test VolumeReplicationClass changes trigger dependent VR reconciles
   - Test PVC changes trigger reconcile

Use envtest for integration-like unit tests with fake client.

### Prompt 5.3: Create Unit Tests for Adapters

Create unit tests for each adapter's v1alpha2 implementation.

Create `pkg/adapters/ceph_v1alpha2_test.go`:

1. **Test Ceph adapter ReconcileVolumeReplication:**
   - Test creating backend Ceph VolumeReplication CR
   - Test updating existing backend CR when spec changes
   - Test parameters mapping from VolumeReplicationClass
   - Test owner reference is set
   - Test status synchronization from backend CR

2. **Test Ceph adapter DeleteVolumeReplication:**
   - Test backend CR is deleted
   - Test graceful handling if backend CR already deleted

Create `pkg/adapters/trident_v1alpha2_test.go`:

1. **Test state translation:**
   - "primary" → "established"
   - "secondary" → "reestablishing"
   - "resync" → "reestablishing"

2. **Test Trident adapter ReconcileVolumeReplication:**
   - Test TridentMirrorRelationship creation with correct translations
   - Test parameter extraction from VolumeReplicationClass
   - Test volumeMapping creation
   - Test status translation back to VolumeReplication

3. **Test error cases:**
   - TridentMirrorRelationship CRD not available
   - Missing required parameters

Create `pkg/adapters/powerstore_v1alpha2_test.go`:

1. **Test action translation:**
   - "primary" → "Failover"
   - "secondary" → "Sync"
   - "resync" → "Reprotect"

2. **Test PowerStore adapter ReconcileVolumeReplication:**
   - Test DellCSIReplicationGroup creation
   - Test PVC labeling
   - Test parameter extraction (protectionPolicy, remoteSystem, rpo)
   - Test PVCSelector configuration

3. **Test status translation:**
   - Dell "Synchronized" → "secondary"
   - Dell "FailedOver" → "primary"

Use fake clients and mock objects to avoid requiring actual backend CRDs.

### Prompt 5.4: Create Integration Tests

Create integration tests that validate end-to-end workflows.

Create `test/integration/volumereplication_test.go`:

1. **Test full Ceph workflow:**
   - Create VolumeReplicationClass for Ceph
   - Create PVC
   - Create VolumeReplication referencing the class and PVC
   - Verify backend Ceph VolumeReplication CR created
   - Update VolumeReplication state (primary → secondary)
   - Verify backend CR updated
   - Delete VolumeReplication
   - Verify backend CR deleted

2. **Test full Trident workflow:**
   - Create VolumeReplicationClass for Trident
   - Create PVC
   - Create VolumeReplication
   - Verify TridentMirrorRelationship created with correct translations
   - Verify state translation (primary → established)
   - Update state and verify translation
   - Delete and verify cleanup

3. **Test full Dell workflow:**
   - Create VolumeReplicationClass for Dell
   - Create PVC
   - Create VolumeReplication
   - Verify DellCSIReplicationGroup created
   - Verify PVC has correct labels
   - Verify action translation (primary → Failover)
   - Delete and verify cleanup

4. **Test VolumeReplicationClass changes:**
   - Create VolumeReplication with classA
   - Modify VolumeReplication to use classB
   - Verify backend resources updated

5. **Test error scenarios:**
   - VolumeReplicationClass not found
   - PVC not found
   - Invalid backend provisioner
   - Backend CRD not installed

Use envtest with registered CRDs for real Kubernetes API interactions.

### Prompt 5.5: Update Existing Tests for Backward Compatibility

Update existing tests to ensure v1alpha1 (UnifiedVolumeReplication) still works alongside v1alpha2.

Update test files in `controllers/`, `pkg/adapters/`, and `api/v1alpha1/`:

1. **Ensure v1alpha1 tests still pass:**
   - Run existing tests: `go test ./api/v1alpha1/... -v`
   - Run existing controller tests for UnifiedVolumeReplication
   - Fix any breakage caused by refactoring

2. **Add dual-version tests:**
   Create `test/integration/backward_compatibility_test.go`:
   - Test v1alpha1 UnifiedVolumeReplication still reconciles correctly
   - Test v1alpha2 VolumeReplication works in same cluster
   - Test both can coexist
   - Test both can manage different resources simultaneously

3. **Mark v1alpha1 tests as deprecated:**
   Add comment at top of v1alpha1 test files:
   ```go
   // DEPRECATED: These tests are for v1alpha1 UnifiedVolumeReplication API
   // which is deprecated. They are maintained for backward compatibility only.
   // For new tests, use v1alpha2 VolumeReplication API.
   ```

4. **Verify all tests pass:**
   ```bash
   go test ./... -v
   make test
   ```

Ensure 100% backward compatibility - no v1alpha1 functionality should break.

---

## Phase 6: Migration Tooling and Documentation

### Prompt 6.1: Create Migration Tool

Create a CLI tool to migrate existing v1alpha1 UnifiedVolumeReplication resources to v1alpha2 VolumeReplication resources.

Create `cmd/migrate/main.go`:

1. **Tool structure:**
   - CLI flags: --dry-run, --namespace (or --all-namespaces), --delete-old
   - Connect to Kubernetes cluster (use kubeconfig)
   - List all v1alpha1 UnifiedVolumeReplication resources
   - For each resource, create equivalent v1alpha2 VolumeReplication + VolumeReplicationClass
   - Optionally delete old resources

2. **Translation logic:**
   ```go
   func translateToV1Alpha2(
       uvr *v1alpha1.UnifiedVolumeReplication,
   ) (*v1alpha2.VolumeReplication, *v1alpha2.VolumeReplicationClass, error) {
       // Extract backend from sourceEndpoint.storageClass
       backend := detectBackendFromStorageClass(uvr.Spec.SourceEndpoint.StorageClass)
       
       // Create VolumeReplicationClass
       vrc := &v1alpha2.VolumeReplicationClass{
           ObjectMeta: metav1.ObjectMeta{
               Name: fmt.Sprintf("%s-class", uvr.Name),
           },
           Spec: v1alpha2.VolumeReplicationClassSpec{
               Provisioner: getProvisioner(backend),
               Parameters:  extractParameters(uvr, backend),
           },
       }
       
       // Create VolumeReplication
       vr := &v1alpha2.VolumeReplication{
           ObjectMeta: metav1.ObjectMeta{
               Name:      uvr.Name,
               Namespace: uvr.Namespace,
               Labels:    uvr.Labels,
               Annotations: mergeMigrationAnnotations(uvr.Annotations),
           },
           Spec: v1alpha2.VolumeReplicationSpec{
               VolumeReplicationClass: vrc.Name,
               PvcName:                uvr.Spec.VolumeMapping.Source.PvcName,
               ReplicationState:       translateState(uvr.Spec.ReplicationState),
               AutoResync:             extractAutoResync(uvr),
           },
       }
       
       return vr, vrc, nil
   }
   ```

3. **State translation:**
   - v1alpha1 "source" → v1alpha2 "primary"
   - v1alpha1 "replica" → v1alpha2 "secondary"
   - v1alpha1 "promoting" → v1alpha2 "primary" (transition state)
   - v1alpha1 "demoting" → v1alpha2 "secondary" (transition state)
   - v1alpha1 "syncing" → v1alpha2 "resync"
   - v1alpha1 "failed" → v1alpha2 "secondary" with error annotation

4. **Parameter extraction:**
   - Ceph: Extract mirroringMode from Extensions.Ceph
   - Trident: Extract replicationPolicy from ReplicationMode, schedule from Schedule.Rpo
   - Dell: Extract from Extensions.Powerstore or infer defaults
   - Common: Extract RPO from Schedule.Rpo

5. **Dry-run mode:**
   - Print what would be created without actually creating
   - Show diff between old and new

6. **Output:**
   - Summary: X resources migrated, Y classes created, Z errors
   - Detailed log of each migration
   - YAML output of created resources (optional flag)

7. **Build and installation:**
   - Add to Makefile: `make migrate-tool`
   - Document in README

This tool enables seamless migration for users.

### Prompt 6.2: Create Migration Guide Documentation

Create a comprehensive migration guide for users transitioning from v1alpha1 to v1alpha2.

Create `docs/migration/V1ALPHA1_TO_V1ALPHA2_MIGRATION_GUIDE.md`:

1. **Executive Summary:**
   - Why we're migrating (kubernetes-csi-addons compatibility)
   - Timeline and deprecation schedule
   - Key differences between v1alpha1 and v1alpha2
   - Migration effort estimation (low/medium/high based on number of resources)

2. **API Comparison:**
   - Side-by-side comparison table of v1alpha1 vs v1alpha2 specs
   - Field mapping table
   - State name mapping table
   - What moved from Spec to VolumeReplicationClass.Parameters

3. **Migration Paths:**
   - **Automated Migration** (recommended):
     * Using the migration tool: `kubectl migrate-uvr --all-namespaces`
     * Step-by-step instructions
     * Example output
   - **Manual Migration**:
     * Create VolumeReplicationClass manually
     * Convert UnifiedVolumeReplication YAML to VolumeReplication YAML
     * Apply new resources
     * Verify backend resources unchanged
     * Delete old resources
   - **Gradual Migration**:
     * Run both APIs in parallel
     * Migrate namespace by namespace
     * Verification steps

4. **Backend-Specific Considerations:**
   - **Ceph migrations:**
     * VolumeReplicationClass for Ceph
     * Parameter migration from Extensions.Ceph
     * Verify Ceph VolumeReplication backend CR unchanged
   - **Trident migrations:**
     * State translation notes
     * TridentMirrorRelationship should be recreated
     * Verify no replication interruption
   - **Dell PowerStore migrations:**
     * Action translation notes
     * DellCSIReplicationGroup recreation
     * PVC label additions

5. **Example Migrations:**
   - Before (v1alpha1) and After (v1alpha2) YAMLs for:
     * Ceph replication
     * Trident replication
     * Dell replication

6. **Troubleshooting:**
   - Common issues and solutions
   - Rollback procedure if needed
   - Verification steps

7. **FAQ:**
   - Can I run both versions simultaneously? (Yes, during migration)
   - Will my data be affected? (No, backend replication unchanged)
   - When will v1alpha1 be removed? (Timeline)
   - How to verify migration success?

8. **Support:**
   - Where to get help
   - How to report migration issues
   - Slack/Discord/GitHub links

Make this the definitive guide for users.

### Prompt 6.3: Update API Reference Documentation

Update the API reference documentation to cover v1alpha2.

Update `docs/api-reference/API_REFERENCE.md`:

1. **Add v1alpha2 section:**
   - Complete VolumeReplication spec documentation
   - Complete VolumeReplicationClass spec documentation
   - Complete status documentation
   - All fields with descriptions, types, required/optional, defaults

2. **Document valid values:**
   - replicationState: primary, secondary, resync
   - provisioner: examples for each backend
   - parameters: document common and backend-specific parameters

3. **Add extensive examples:**
   - VolumeReplicationClass for each backend
   - VolumeReplication for each state
   - Common scenarios (promote, demote, resync)

4. **Deprecation notice:**
   Add prominent notice at top:
   ```markdown
   ## ⚠️ API Version Notice
   
   - **v1alpha2** (recommended): kubernetes-csi-addons compatible API
   - **v1alpha1** (deprecated): Will be removed in version 3.0.0 (approx. 12 months)
   
   New users should use v1alpha2. Existing users should migrate using the 
   [Migration Guide](../migration/V1ALPHA1_TO_V1ALPHA2_MIGRATION_GUIDE.md).
   ```

5. **Add comparison table:**
   - v1alpha1 vs v1alpha2 field mapping
   - What changed and why
   - Migration notes

6. **Update code examples:**
   - Update README.md examples to use v1alpha2
   - Update QUICK_START.md to use v1alpha2
   - Keep v1alpha1 examples in deprecated section

Ensure documentation is comprehensive and clear.

### Prompt 6.4: Update Helm Chart for v1alpha2

Update the Helm chart to support both v1alpha1 and v1alpha2.

Update `helm/unified-replication-operator/`:

1. **Update Chart.yaml:**
   - Bump version to 2.0.0 (major version for breaking changes)
   - Update description to mention kubernetes-csi-addons compatibility
   - Add notes about v1alpha1 deprecation

2. **Update templates/crd.yaml:**
   - Include both v1alpha1 and v1alpha2 CRDs
   - Add comment explaining dual support

3. **Update values.yaml:**
   ```yaml
   # API version configuration
   api:
     # Enable v1alpha1 support (deprecated, will be removed in 3.0.0)
     v1alpha1Enabled: true
     # Enable v1alpha2 support (recommended)
     v1alpha2Enabled: true
   
   # Controller configuration
   controller:
     # Watch v1alpha1 resources
     watchV1Alpha1: true
     # Watch v1alpha2 resources
     watchV1Alpha2: true
   ```

4. **Update RBAC:**
   - Add permissions for VolumeReplication (v1alpha2)
   - Add permissions for VolumeReplicationClass (v1alpha2)
   - Keep permissions for UnifiedVolumeReplication (v1alpha1)

5. **Update NOTES.txt:**
   Add post-install message:
   ```
   ✅ Unified Replication Operator 2.0.0 installed successfully!
   
   📢 IMPORTANT: API Version Notice
   - v1alpha2 is now recommended (kubernetes-csi-addons compatible)
   - v1alpha1 is deprecated and will be removed in 3.0.0
   
   To migrate existing resources:
   $ kubectl migrate-uvr --all-namespaces --dry-run
   $ kubectl migrate-uvr --all-namespaces
   
   For details, see: https://docs.unified-replication.io/migration
   ```

6. **Update README.md in helm chart:**
   - Document new values
   - Document API version support
   - Add migration instructions

7. **Test Helm chart:**
   ```bash
   helm lint ./helm/unified-replication-operator
   helm template test ./helm/unified-replication-operator
   helm install test ./helm/unified-replication-operator --dry-run
   ```

Ensure smooth upgrade path for existing installations.

### Prompt 6.5: Create Deprecation Policy Document

Create a formal deprecation policy document.

Create `docs/DEPRECATION_POLICY.md`:

1. **Deprecation Timeline:**
   ```
   v2.0.0 (Now): 
   - v1alpha2 introduced (kubernetes-csi-addons compatible)
   - v1alpha1 marked deprecated but fully supported
   
   v2.x.x (Next 12 months):
   - Both v1alpha1 and v1alpha2 supported
   - Security fixes and critical bugs only for v1alpha1
   - New features only in v1alpha2
   
   v3.0.0 (12 months from now):
   - v1alpha1 removed
   - v1alpha2 only
   - No backward compatibility with v1alpha1
   ```

2. **Support Policy:**
   - What "deprecated" means
   - What "supported" means
   - What "removed" means
   - Bug fix policy for deprecated APIs
   - Security patch policy

3. **Migration Requirements:**
   - When users must migrate by
   - What happens if they don't migrate
   - Migration support offerings

4. **Communication Plan:**
   - How deprecations are announced
   - Where to find deprecation notices
   - Release notes format
   - Warning messages in operator logs

5. **Version Compatibility Matrix:**
   ```
   | Operator Version | v1alpha1 | v1alpha2 | Notes |
   |------------------|----------|----------|-------|
   | 1.x.x            | ✅ Stable | ❌ N/A    | Before migration |
   | 2.x.x            | ⚠️ Deprecated | ✅ Stable | Migration period |
   | 3.x.x            | ❌ Removed | ✅ Stable | After migration |
   ```

6. **Upgrade Path:**
   - 1.x.x → 2.x.x: Safe, no action required (but migration recommended)
   - 2.x.x → 3.x.x: Requires migration of all v1alpha1 resources first

This provides transparency and predictability for users.

---

## Phase 7: Future-Proofing for Option A

### Prompt 7.1: Create Conversion Webhook Framework

Create the framework for conversion webhooks to enable future Option A migration (replication.unified.io → replication.storage.openshift.io).

Create `api/conversion/webhook.go`:

1. **Webhook server setup:**
   ```go
   // Package conversion provides webhook-based conversion between API versions.
   // This enables future migration from replication.unified.io to 
   // replication.storage.openshift.io (Option A).
   package conversion
   
   import (
       "context"
       "net/http"
       
       "sigs.k8s.io/controller-runtime/pkg/conversion"
       "sigs.k8s.io/controller-runtime/pkg/webhook"
   )
   
   // VolumeReplicationWebhook handles conversion between API groups
   type VolumeReplicationWebhook struct{}
   
   // SetupWebhookWithManager registers the conversion webhook
   func SetupWebhookWithManager(mgr ctrl.Manager) error {
       // Register webhook endpoint
       // This is currently a no-op but provides the structure for future use
       return nil
   }
   ```

2. **Conversion interface implementation:**
   ```go
   // ConvertTo converts from replication.unified.io to replication.storage.openshift.io
   func (src *v1alpha2.VolumeReplication) ConvertTo(dstRaw conversion.Hub) error {
       // dst := dstRaw.(*csiaddonsv1alpha1.VolumeReplication)
       // 
       // Since structs are identical, this is straightforward field copying:
       // dst.Spec.VolumeReplicationClass = src.Spec.VolumeReplicationClass
       // dst.Spec.PvcName = src.Spec.PvcName
       // dst.Spec.ReplicationState = src.Spec.ReplicationState
       // dst.Spec.DataSource = src.Spec.DataSource
       // dst.Spec.AutoResync = src.Spec.AutoResync
       
       // TODO: Implement when Option A is chosen
       return nil
   }
   
   // ConvertFrom converts from replication.storage.openshift.io to replication.unified.io
   func (dst *v1alpha2.VolumeReplication) ConvertFrom(srcRaw conversion.Hub) error {
       // src := srcRaw.(*csiaddonsv1alpha1.VolumeReplication)
       // 
       // Reverse of ConvertTo
       
       // TODO: Implement when Option A is chosen
       return nil
   }
   ```

3. **Add markers to types:**
   Update `api/v1alpha2/volumereplication_types.go`:
   ```go
   // +kubebuilder:object:root=true
   // +kubebuilder:subresource:status
   // +kubebuilder:storageversion  // Mark as storage version
   // +kubebuilder:resource:scope=Namespaced,shortName=vr;volrep
   // TODO: Add conversion webhook annotation when implementing Option A:
   // +kubebuilder:webhook:path=/convert,mutating=false,failurePolicy=fail,groups=replication.unified.io;replication.storage.openshift.io,resources=volumereplications,verbs=create;update,versions=v1alpha2,name=volumereplication.replication.unified.io
   type VolumeReplication struct {
       // ...
   }
   ```

4. **Documentation:**
   Create `docs/architecture/OPTION_A_TRANSITION_PLAN.md`:
   - How to enable conversion webhooks
   - Steps to migrate to replication.storage.openshift.io
   - Certificate management for webhooks
   - Testing plan for conversion
   - Rollback procedure

This framework is dormant but ready to activate when needed.

### Prompt 7.2: Create API Compatibility Tests

Create tests that verify our v1alpha2 API remains compatible with kubernetes-csi-addons spec.

Create `test/compatibility/csi_addons_compatibility_test.go`:

1. **Struct comparison tests:**
   ```go
   func TestVolumeReplicationSpecCompatibility(t *testing.T) {
       // Compare our v1alpha2.VolumeReplicationSpec with 
       // kubernetes-csi-addons VolumeReplicationSpec
       // Ensure all fields match by name and type
   }
   
   func TestVolumeReplicationStatusCompatibility(t *testing.T) {
       // Compare status structs
   }
   
   func TestVolumeReplicationClassSpecCompatibility(t *testing.T) {
       // Compare class specs
   }
   ```

2. **JSON serialization compatibility:**
   ```go
   func TestJSONSerializationCompatibility(t *testing.T) {
       // Create our v1alpha2.VolumeReplication
       ourVR := &v1alpha2.VolumeReplication{...}
       
       // Serialize to JSON
       jsonData, err := json.Marshal(ourVR)
       require.NoError(t, err)
       
       // Deserialize into kubernetes-csi-addons type
       // var csiVR csiaddonsv1alpha1.VolumeReplication
       // err = json.Unmarshal(jsonData, &csiVR)
       // require.NoError(t, err)
       
       // Verify all fields match
       // assert.Equal(t, ourVR.Spec.PvcName, csiVR.Spec.PvcName)
       // ...
       
       // TODO: Uncomment when we have actual kubernetes-csi-addons dependency
   }
   ```

3. **YAML roundtrip tests:**
   ```go
   func TestYAMLRoundtrip(t *testing.T) {
       // Ensure YAML generated from our types can be parsed by kubernetes-csi-addons
       // and vice versa
   }
   ```

4. **Field addition detection:**
   ```go
   func TestNoExtraFieldsAdded(t *testing.T) {
       // Use reflection to ensure we haven't added fields to Spec or Status
       // that don't exist in kubernetes-csi-addons
       // This prevents "Option A compatibility drift"
   }
   ```

5. **Continuous monitoring:**
   - Add GitHub Action that periodically checks kubernetes-csi-addons repo for changes
   - Alert if kubernetes-csi-addons API changes
   - Document in `docs/architecture/API_COMPATIBILITY_MONITORING.md`

These tests act as guardrails to prevent divergence from kubernetes-csi-addons.

### Prompt 7.3: Document Option A Transition Procedure

Create a detailed procedure document for transitioning from Option B to Option A.

Create `docs/architecture/OPTION_A_TRANSITION_PROCEDURE.md`:

1. **Prerequisites:**
   - All users must be on v2.x.x (using v1alpha2)
   - No v1alpha1 resources remaining
   - Compatibility tests passing
   - Conversion webhooks tested

2. **Decision Criteria:**
   When should we transition to Option A?
   - User demand for native kubernetes-csi-addons compatibility
   - Need to coexist with kubernetes-csi-addons operator
   - Community preference
   - Maintenance burden of separate API group

3. **Transition Steps:**
   
   **Step 1: Add kubernetes-csi-addons as dependency**
   ```bash
   go get github.com/csi-addons/kubernetes-csi-addons@latest
   ```
   
   **Step 2: Enable conversion webhooks**
   - Implement conversion methods in `api/conversion/webhook.go`
   - Generate webhook manifests: `make webhook-manifests`
   - Deploy cert-manager if not present
   - Deploy webhook with TLS certificates
   
   **Step 3: Support both API groups simultaneously**
   - Controller watches both `replication.unified.io` and `replication.storage.openshift.io`
   - Dual reconciliation with shared backend logic
   - Status sync between both resource types
   
   **Step 4: Create migration tool v2**
   - Tool to migrate from `replication.unified.io` to `replication.storage.openshift.io`
   - Similar to v1alpha1 → v1alpha2 migration tool
   - Automatic conversion using conversion webhooks
   
   **Step 5: Announce deprecation of replication.unified.io**
   - 6-12 month timeline
   - Communication to users
   - Documentation updates
   
   **Step 6: Remove replication.unified.io support**
   - Stop watching replication.unified.io resources
   - Remove CRDs for replication.unified.io
   - Update documentation
   - Release as v4.0.0

4. **Rollback Plan:**
   - How to revert to Option B if issues arise
   - Data preservation during rollback
   - User communication

5. **Testing Strategy:**
   - Test conversion webhooks thoroughly
   - Test dual API group operation
   - Test migration tool
   - Canary deployment approach

6. **Timeline Estimation:**
   - Development: 4-6 weeks
   - Testing: 2-3 weeks
   - Beta period: 1-2 months
   - Migration period: 6-12 months
   - Total: ~12-18 months from decision to full transition

7. **Success Criteria:**
   - All resources migrated to replication.storage.openshift.io
   - No user-reported issues
   - Full kubernetes-csi-addons compatibility
   - Can coexist with kubernetes-csi-addons operator (if needed)

This document serves as the roadmap if Option A is chosen.

---

## Phase 8: Release and Deployment

### Prompt 8.1: Update Makefile for v1alpha2

Update the Makefile to support building, testing, and deploying v1alpha2.

Update `Makefile`:

1. **Add v1alpha2 generation targets:**
   ```makefile
   # Generate code for both API versions
   .PHONY: generate
   generate: controller-gen
   	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/v1alpha1/..." paths="./api/v1alpha2/..."
   
   # Generate manifests for both API versions  
   .PHONY: manifests
   manifests: controller-gen
   	$(CONTROLLER_GEN) rbac:roleName=unified-replication-operator-manager \
   		crd:crdVersions=v1 \
   		webhook \
   		paths="./api/v1alpha1/..." paths="./api/v1alpha2/..." \
   		output:crd:artifacts:config=config/crd/bases
   ```

2. **Add migration tool build target:**
   ```makefile
   # Build migration tool
   .PHONY: migrate-tool
   migrate-tool:
   	go build -o bin/migrate-uvr ./cmd/migrate
   
   .PHONY: install-migrate-tool
   install-migrate-tool: migrate-tool
   	cp bin/migrate-uvr /usr/local/bin/
   ```

3. **Add compatibility test target:**
   ```makefile
   # Run API compatibility tests
   .PHONY: test-compatibility
   test-compatibility:
   	go test -v ./test/compatibility/...
   ```

4. **Update test targets:**
   ```makefile
   # Run all tests including v1alpha2
   .PHONY: test
   test: manifests generate fmt vet envtest
   	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" \
   		go test ./api/... ./controllers/... ./pkg/... -coverprofile cover.out
   	
   # Run v1alpha2 specific tests
   .PHONY: test-v1alpha2
   test-v1alpha2: manifests generate fmt vet envtest
   	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" \
   		go test ./api/v1alpha2/... ./controllers/volumereplication_controller_test.go -v
   ```

5. **Add sample deployment target:**
   ```makefile
   # Deploy sample v1alpha2 resources
   .PHONY: deploy-samples-v1alpha2
   deploy-samples-v1alpha2:
   	kubectl apply -f config/samples/volumereplicationclass_ceph.yaml
   	kubectl apply -f config/samples/volumereplication_ceph_primary.yaml
   ```

6. **Update install target:**
   ```makefile
   # Install CRDs (both v1alpha1 and v1alpha2)
   .PHONY: install
   install: manifests kustomize
   	$(KUSTOMIZE) build config/crd | kubectl apply -f -
   ```

Test all Makefile targets after updates.

### Prompt 8.2: Create Release Notes for v2.0.0

Create comprehensive release notes for the v2.0.0 release.

Create `docs/releases/RELEASE_NOTES_v2.0.0.md`:

1. **Executive Summary:**
   ```markdown
   # Unified Replication Operator v2.0.0
   
   ## 🎉 Major Release: kubernetes-csi-addons Compatibility
   
   This release introduces v1alpha2 API, which is fully compatible with the
   kubernetes-csi-addons VolumeReplication specification while maintaining our
   unique value proposition: multi-backend translation to Trident and Dell PowerStore.
   
   **Key Highlights:**
   - ✅ kubernetes-csi-addons compatible API (v1alpha2)
   - ✅ Simplified user experience with VolumeReplicationClass
   - ✅ Maintained translation to Trident and Dell backends
   - ✅ Backward compatibility with v1alpha1 (deprecated)
   - ✅ Automated migration tool
   ```

2. **What's New:**
   - VolumeReplication v1alpha2 API
   - VolumeReplicationClass for backend configuration
   - Enhanced Ceph adapter (passthrough)
   - Enhanced Trident adapter (state translation)
   - Enhanced Dell adapter (action translation)
   - Migration tool: `migrate-uvr`
   - Comprehensive migration guide

3. **Breaking Changes:**
   - v1alpha1 UnifiedVolumeReplication API deprecated (still supported)
   - New resources use different API structure
   - Migration required for new features

4. **API Changes:**
   Detailed comparison table of v1alpha1 vs v1alpha2

5. **Migration Guide:**
   Link to full migration documentation

6. **Upgrade Instructions:**
   ```bash
   # Upgrade operator to v2.0.0
   helm upgrade unified-replication-operator \
     unified-replication/unified-replication-operator \
     --version 2.0.0
   
   # Migrate resources (dry-run first)
   migrate-uvr --all-namespaces --dry-run
   
   # Actual migration
   migrate-uvr --all-namespaces
   ```

7. **Deprecation Notices:**
   - v1alpha1 API deprecated, will be removed in v3.0.0 (12 months)
   - Users should migrate before v3.0.0

8. **Known Issues:**
   - List any known issues or limitations

9. **Contributors:**
   - Thank contributors
   - Link to GitHub contributors page

10. **What's Next:**
    - Roadmap for v2.x.x series
    - Possible Option A transition
    - Community feedback solicitation

### Prompt 8.3: Update CI/CD Pipeline

Update the CI/CD pipeline to test and build v1alpha2.

Update `.github/workflows/` (if using GitHub Actions) or equivalent:

1. **Update test workflow:**
   ```yaml
   name: Tests
   on: [push, pull_request]
   
   jobs:
     test:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v3
         - uses: actions/setup-go@v4
           with:
             go-version: '1.21'
         
         # Test both API versions
         - name: Test v1alpha1 (deprecated)
           run: make test-v1alpha1
         
         - name: Test v1alpha2
           run: make test-v1alpha2
         
         # Test compatibility
         - name: Test API compatibility
           run: make test-compatibility
         
         # Integration tests
         - name: Integration tests
           run: make test-integration
   ```

2. **Update build workflow:**
   ```yaml
   - name: Generate manifests
     run: make manifests
   
   - name: Verify CRDs include v1alpha2
     run: |
       test -f config/crd/bases/replication.unified.io_volumereplications.yaml
       test -f config/crd/bases/replication.unified.io_volumereplicationclasses.yaml
       grep -q "version: v1alpha2" config/crd/bases/replication.unified.io_volumereplications.yaml
   ```

3. **Update release workflow:**
   ```yaml
   - name: Build migration tool
     run: make migrate-tool
   
   - name: Upload migration tool artifact
     uses: actions/upload-artifact@v3
     with:
       name: migrate-uvr
       path: bin/migrate-uvr
   ```

4. **Add compatibility check workflow:**
   ```yaml
   name: API Compatibility Check
   on:
     schedule:
       - cron: '0 0 * * 0'  # Weekly
     workflow_dispatch:
   
   jobs:
     compatibility:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v3
         
         - name: Check kubernetes-csi-addons for changes
           run: |
             # Clone kubernetes-csi-addons repo
             # Compare VolumeReplication spec
             # Alert if differences found
         
         - name: Run compatibility tests
           run: make test-compatibility
   ```

Ensure all pipelines pass before release.

### Prompt 8.4: Create Upgrade Guide

Create a guide for operators upgrading from v1.x.x to v2.0.0.

Create `docs/upgrade/UPGRADE_v1_to_v2.md`:

1. **Pre-Upgrade Checklist:**
   - [ ] Review release notes
   - [ ] Back up existing UnifiedVolumeReplication resources
   - [ ] Test upgrade in non-production environment
   - [ ] Plan migration timeline
   - [ ] Communicate downtime (if any) to users

2. **Upgrade Steps:**
   
   **Step 1: Backup existing resources**
   ```bash
   kubectl get uvr --all-namespaces -o yaml > uvr-backup.yaml
   ```
   
   **Step 2: Upgrade operator**
   ```bash
   # Via Helm
   helm repo update
   helm upgrade unified-replication-operator \
     unified-replication/unified-replication-operator \
     --version 2.0.0 \
     --namespace unified-replication-system
   
   # Via Kustomize
   kubectl apply -k config/overlays/production
   ```
   
   **Step 3: Verify operator running**
   ```bash
   kubectl get pods -n unified-replication-system
   kubectl logs -n unified-replication-system deployment/unified-replication-operator
   ```
   
   **Step 4: Verify v1alpha1 resources still working**
   ```bash
   kubectl get uvr --all-namespaces
   kubectl describe uvr <name> -n <namespace>
   ```
   
   **Step 5: Install migration tool**
   ```bash
   kubectl apply -f https://releases.unified-replication.io/v2.0.0/migrate-uvr-job.yaml
   # Or download binary
   curl -LO https://releases.unified-replication.io/v2.0.0/migrate-uvr
   chmod +x migrate-uvr
   ```
   
   **Step 6: Run migration (dry-run first)**
   ```bash
   ./migrate-uvr --all-namespaces --dry-run
   # Review output carefully
   ```
   
   **Step 7: Execute migration**
   ```bash
   ./migrate-uvr --all-namespaces
   ```
   
   **Step 8: Verify migration**
   ```bash
   kubectl get vr --all-namespaces  # New v1alpha2 resources
   kubectl get vrc                   # VolumeReplicationClasses created
   
   # Check backend resources unchanged
   kubectl get volumereplication --all-namespaces  # Ceph
   kubectl get tridentmirrorrelationship --all-namespaces  # Trident
   kubectl get dellcsireplicationgroup --all-namespaces  # Dell
   ```
   
   **Step 9: Optional - Remove v1alpha1 resources**
   ```bash
   # Only after verifying v1alpha2 works correctly
   kubectl delete uvr --all --all-namespaces
   ```

3. **Rollback Procedure:**
   ```bash
   # Rollback operator
   helm rollback unified-replication-operator
   
   # Restore v1alpha1 resources
   kubectl apply -f uvr-backup.yaml
   
   # Delete v1alpha2 resources if created
   kubectl delete vr --all --all-namespaces
   kubectl delete vrc --all
   ```

4. **Troubleshooting:**
   - Migration tool fails: check logs, verify permissions
   - v1alpha2 resources not reconciling: check VolumeReplicationClass exists
   - Backend resources duplicated: check owner references
   - Status not updating: check controller logs

5. **Post-Upgrade Validation:**
   - [ ] All VolumeReplication resources in Ready state
   - [ ] Backend replication still functioning
   - [ ] No data loss or interruption
   - [ ] Status reflects current state
   - [ ] Can create new v1alpha2 resources successfully

6. **Timeline Recommendations:**
   - Small deployments (<10 resources): 1-2 hours
   - Medium deployments (10-100 resources): 4-8 hours
   - Large deployments (>100 resources): Plan maintenance window

7. **Support:**
   - GitHub Issues: link
   - Slack: link
   - Email: support email

### Prompt 8.5: Final Testing and Validation

Perform comprehensive final testing before release.

Create `test/validation/release_validation.md` as a checklist:

1. **API Validation:**
   - [ ] v1alpha2 CRDs generated correctly
   - [ ] v1alpha2 types have proper kubebuilder markers
   - [ ] API matches kubernetes-csi-addons spec exactly
   - [ ] All validation rules working
   - [ ] Status subresources enabled

2. **Controller Validation:**
   - [ ] VolumeReplication controller watches correct resources
   - [ ] Backend detection working for all backends
   - [ ] VolumeReplicationClass lookup working
   - [ ] Status updates working
   - [ ] Finalizers working correctly

3. **Adapter Validation:**
   - [ ] Ceph adapter creates backend VolumeReplication CR
   - [ ] Trident adapter translates states correctly
   - [ ] Dell adapter translates actions correctly
   - [ ] All adapters handle errors gracefully
   - [ ] Status synchronization working

4. **End-to-End Testing:**
   - [ ] Create Ceph VolumeReplication, verify replication works
   - [ ] Create Trident VolumeReplication, verify TridentMirrorRelationship created
   - [ ] Create Dell VolumeReplication, verify DellCSIReplicationGroup created
   - [ ] Promote secondary to primary, verify backend changes
   - [ ] Demote primary to secondary, verify backend changes
   - [ ] Delete VolumeReplication, verify backend cleanup

5. **Migration Testing:**
   - [ ] Migration tool builds successfully
   - [ ] Dry-run mode works correctly
   - [ ] Actual migration creates correct v1alpha2 resources
   - [ ] State translation correct in migration
   - [ ] Parameters extracted correctly
   - [ ] Old resources can be deleted safely

6. **Backward Compatibility:**
   - [ ] v1alpha1 resources still reconcile
   - [ ] v1alpha1 and v1alpha2 can coexist
   - [ ] No interference between API versions
   - [ ] Existing backend resources not affected

7. **Documentation Validation:**
   - [ ] All docs updated to reference v1alpha2
   - [ ] Migration guide complete and accurate
   - [ ] API reference complete
   - [ ] Examples working
   - [ ] Helm chart README updated

8. **Deployment Validation:**
   - [ ] Helm chart installs successfully
   - [ ] Kustomize overlays work
   - [ ] RBAC permissions correct
   - [ ] CRDs installed correctly
   - [ ] Controller starts successfully

9. **Performance Testing:**
   - [ ] No performance regression
   - [ ] Migration handles large number of resources
   - [ ] Controller scales appropriately

10. **Security Validation:**
    - [ ] No security regressions
    - [ ] RBAC minimally permissive
    - [ ] No secrets leaked in logs
    - [ ] Admission webhooks secure (if enabled)

11. **Compatibility Testing:**
    - [ ] Works on Kubernetes 1.24+
    - [ ] Works on OpenShift 4.10+
    - [ ] Works with Ceph-CSI latest version
    - [ ] Works with Trident latest version
    - [ ] Works with Dell CSI latest version

Execute this checklist thoroughly before tagging release.

---

## Summary

This migration plan provides a comprehensive, phased approach to transforming the unified-replication-operator from its current complex specification to a kubernetes-csi-addons compatible specification (Option B) while maintaining the architectural flexibility to transition to Option A in the future.

### Key Principles

1. **Compatibility First:** Match kubernetes-csi-addons spec exactly to enable future Option A transition
2. **Backward Compatibility:** Support v1alpha1 during migration period (12 months)
3. **Multi-Backend Support:** Maintain translation capabilities to Trident and Dell
4. **User-Centric:** Provide excellent migration tools and documentation
5. **Future-Proof:** Build infrastructure for Option A transition if needed

### Execution Timeline

- **Phase 1-2 (API Design):** 1-2 weeks
- **Phase 3-4 (Controller & Adapters):** 3-4 weeks  
- **Phase 5 (Testing):** 2-3 weeks
- **Phase 6 (Migration & Docs):** 2-3 weeks
- **Phase 7 (Future-Proofing):** 1 week
- **Phase 8 (Release):** 1 week

**Total Estimated Time:** 10-16 weeks (2.5-4 months)

### Success Metrics

- ✅ API matches kubernetes-csi-addons exactly
- ✅ All three backends (Ceph, Trident, Dell) working with v1alpha2
- ✅ Migration tool successfully migrates all test resources
- ✅ Documentation comprehensive and clear
- ✅ Zero data loss during migration
- ✅ Backward compatibility maintained
- ✅ Path to Option A clear and achievable

---

## Next Steps

1. **Review this plan with stakeholders**
2. **Prioritize phases based on resources**
3. **Begin with Phase 1: Research and Planning**
4. **Execute prompts sequentially**
5. **Test thoroughly at each phase**
6. **Communicate with users throughout**

Each prompt can be given to an AI coding assistant or developer as a standalone task. The prompts are designed to be specific, actionable, and to build upon previous work systematically.
