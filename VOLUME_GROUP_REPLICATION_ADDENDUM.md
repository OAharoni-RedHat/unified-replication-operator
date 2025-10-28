# Volume Group Replication - Addendum to Migration Plan

## Overview

This document adds Volume Group Replication support to the migration plan. Volume Group Replication is part of the kubernetes-csi-addons specification and allows replicating multiple PVCs together as a single unit for application consistency.

**Integration Point:** Add as **Phase 2B** (between Phase 2 and Phase 3) or as **Optional Enhancement** after Phase 8.

---

## What is Volume Group Replication?

### Single Volume Replication (Already Implemented in Phase 2)

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: database-data-replication
spec:
  volumeReplicationClass: ceph-replication
  pvcName: database-data-pvc  # Single PVC
  replicationState: primary
```

**Use Case:** Replicate a single volume

### Volume Group Replication (To Be Implemented)

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplication
metadata:
  name: database-complete-replication
  namespace: production
spec:
  volumeGroupReplicationClass: ceph-group-replication
  # Select multiple PVCs using labels
  selector:
    matchLabels:
      app: postgresql
      component: database
  replicationState: primary
```

**Use Case:** Replicate multiple related volumes together (e.g., database data + logs + config)

**Benefits:**
- **Crash consistency:** All volumes snapshotted at the same point in time
- **Atomic operations:** Promote/demote all volumes together
- **Application-level consistency:** No partial states during failover

---

## kubernetes-csi-addons Spec for Volume Groups

### VolumeGroupReplicationSpec

```go
type VolumeGroupReplicationSpec struct {
    // VolumeGroupReplicationClass is the name of the class
    VolumeGroupReplicationClass string `json:"volumeGroupReplicationClass"`
    
    // Selector to select PVCs that are part of this group
    Selector *metav1.LabelSelector `json:"selector"`
    
    // ReplicationState represents the desired group replication state
    // Valid values: primary, secondary, resync
    ReplicationState string `json:"replicationState"`
    
    // AutoResync indicates if the group should be automatically resynced
    AutoResync *bool `json:"autoResync,omitempty"`
    
    // Source is optional for specifying source volume group
    Source *corev1.TypedLocalObjectReference `json:"source,omitempty"`
}
```

### VolumeGroupReplicationStatus

```go
type VolumeGroupReplicationStatus struct {
    // Conditions represent the group's current state
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // State represents the current group state
    State string `json:"state,omitempty"`
    
    // Message provides detailed information
    Message string `json:"message,omitempty"`
    
    // LastSyncTime for the group
    LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
    
    // LastSyncDuration for the group
    LastSyncDuration *metav1.Duration `json:"lastSyncDuration,omitempty"`
    
    // ObservedGeneration reflects generation observed
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
    
    // PersistentVolumeClaimsRefList has info about PVCs in this group
    PersistentVolumeClaimsRefList []corev1.LocalObjectReference `json:"persistentVolumeClaimsRefList,omitempty"`
}
```

### VolumeGroupReplicationClassSpec

```go
type VolumeGroupReplicationClassSpec struct {
    // Provisioner identifies the backend
    Provisioner string `json:"provisioner"`
    
    // Parameters for group replication configuration
    Parameters map[string]string `json:"parameters,omitempty"`
}
```

---

## Implementation: Phase 2B (Volume Group Support)

### Prompt 2B.1: Create VolumeGroupReplication Types

Create `api/v1alpha2/volumegroupreplication_types.go` matching kubernetes-csi-addons specification.

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
   - persistentVolumeClaimsRefList: []corev1.LocalObjectReference - list of PVCs in group

3. **Kubebuilder markers:**
   - API group: replication.unified.io
   - Version: v1alpha2
   - Resource shortNames: vgr, volgrouprep
   - Status subresource
   - Validation: Required fields, state enum
   - PrintColumns: State, Class, PVCs, Age

4. **Add compatibility notice:**
   ```go
   // COMPATIBILITY NOTICE:
   // This matches kubernetes-csi-addons VolumeGroupReplication.
   // Use this for multi-volume application consistency.
   ```

### Prompt 2B.2: Create VolumeGroupReplicationClass Types

Create `api/v1alpha2/volumegroupreplicationclass_types.go`.

Requirements:

1. **VolumeGroupReplicationClassSpec:**
   - provisioner: string (required)
   - parameters: map[string]string (optional)

2. **Kubebuilder markers:**
   - Scope: Cluster
   - ShortNames: vgrc, volgrouprepclass
   - PrintColumns: Provisioner, Age

3. **Parameter documentation:**
   ```go
   // Common parameters for volume group replication:
   // - consistencyGroup: "enabled" - ensure crash consistency
   // - groupSnapshots: "true" - use group snapshots
   //
   // Ceph-specific:
   // - groupConsistency: "application" or "crash"
   //
   // Trident-specific:
   // - consistencyGroupPolicy: Policy name
   //
   // Dell PowerStore-specific:
   // - consistencyType: "Metro" or "Async"
   ```

### Prompt 2B.3: Update GroupVersion to Include Volume Groups

Update `api/v1alpha2/groupversion_info.go` init function:

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

### Prompt 2B.4: Create Sample YAMLs for Volume Groups

Create sample YAMLs demonstrating volume group replication:

**`config/samples/volumegroupreplicationclass_ceph.yaml`:**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplicationClass
metadata:
  name: ceph-rbd-group-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    mirroringMode: "snapshot"
    schedulingInterval: "5m"
    groupConsistency: "crash"  # Crash-consistent group snapshots
    replication.storage.openshift.io/replication-secret-name: "rbd-secret"
    replication.storage.openshift.io/replication-secret-namespace: "rook-ceph"
```

**`config/samples/volumegroupreplication_database.yaml`:**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplication
metadata:
  name: postgresql-database-group
  namespace: production
spec:
  volumeGroupReplicationClass: ceph-rbd-group-replication
  
  # Select all PVCs for the PostgreSQL database
  selector:
    matchLabels:
      app: postgresql
      instance: prod-db-01
  
  replicationState: primary
  autoResync: true

---
# This will replicate all PVCs with matching labels as a group:
# - postgresql-data-pvc (app=postgresql, instance=prod-db-01)
# - postgresql-logs-pvc (app=postgresql, instance=prod-db-01)
# - postgresql-config-pvc (app=postgresql, instance=prod-db-01)
```

---

## When to Implement Volume Group Replication

### Option 1: Essential Feature (Recommended)

**Add as Phase 2B** - Implement immediately after Phase 2

**Rationale:**
- Critical for multi-volume applications (databases, stateful apps)
- Part of kubernetes-csi-addons standard
- Required for full compatibility claim
- Relatively small additional work (similar to single volume types)

**Timeline Impact:** +1-2 weeks to overall project

### Option 2: Phase 2 Enhancement (Later)

**Add after Phase 8** - Implement as enhancement in v2.1.0

**Rationale:**
- Get v2.0.0 out faster with single volume support
- Gather feedback before adding group support
- Many use cases work with single volume replication

**Timeline Impact:** No impact on v2.0.0 release

### Option 3: Deferred Feature

**Implement only if users request it**

**Rationale:**
- YAGNI (You Aren't Gonna Need It)
- Adds complexity
- Not all backends support it equally well

**Risk:** Incomplete kubernetes-csi-addons compatibility

---

## Recommendation: Implement as Phase 2B

I recommend **adding Volume Group Replication as Phase 2B** for these reasons:

### ✅ Pros:
1. **Complete kubernetes-csi-addons compatibility** - Can claim full spec support
2. **Critical for databases** - PostgreSQL, MySQL, MongoDB often use multiple volumes
3. **Dell PowerStore already uses groups** - DellCSIReplicationGroup is inherently group-based
4. **Small incremental work** - Types are similar to VolumeReplication, just with selector
5. **Future-proof** - Matches where the industry is going

### ⚠️ Cons:
1. **More complex controller logic** - Must watch label changes on PVCs
2. **Group state management** - Need to track multiple volumes as one unit
3. **Testing complexity** - More scenarios to test

### Implementation Effort

**Phase 2B Effort Breakdown:**
- API Types: 1-2 days (similar to Phase 2)
- Controller Logic: 3-4 days (selector handling, group reconciliation)
- Adapters: 2-3 days (group translation per backend)
- Testing: 3-4 days
- Documentation: 1-2 days

**Total: ~2-3 weeks**

---

## Backend Support for Volume Groups

### Ceph (RBD)
- ✅ **Supported:** Yes, via group snapshots
- **Implementation:** Use VolumeGroupSnapshot for consistency
- **Parameters:** `groupConsistency: "crash"`

### Trident
- ⚠️ **Partial Support:** Consistency groups available
- **Implementation:** TridentMirrorRelationship with volumeMappings array
- **Parameters:** `consistencyGroupPolicy: "<policy-name>"`

### Dell PowerStore
- ✅ **Native Support:** DellCSIReplicationGroup is designed for groups
- **Implementation:** PVCSelector already uses label selector
- **Parameters:** `consistencyType: "Metro"` or `"Async"`

---

## Updated Migration Plan Structure

```
Phase 1: API Research ✅ (Completed)
Phase 2: Single Volume API Types ✅ (Completed)
Phase 2B: Volume Group API Types (NEW - RECOMMENDED)
  ├─ VolumeGroupReplication types
  ├─ VolumeGroupReplicationClass types
  ├─ Sample YAMLs for groups
  └─ CRD generation
Phase 3: Controllers (Extended for groups)
  ├─ VolumeReplication controller
  └─ VolumeGroupReplication controller (NEW)
Phase 4: Adapters (Extended for groups)
  ├─ Single volume adapters
  └─ Volume group adapters (NEW)
...continues
```

---

## Example: Database with Volume Groups

### Without Volume Group (Current Phase 2)

```yaml
# Need 3 separate VolumeReplication resources
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: db-data-replication
spec:
  volumeReplicationClass: ceph-replication
  pvcName: postgresql-data-pvc
  replicationState: primary
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: db-logs-replication
spec:
  volumeReplicationClass: ceph-replication
  pvcName: postgresql-logs-pvc
  replicationState: primary
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: db-config-replication
spec:
  volumeReplicationClass: ceph-replication
  pvcName: postgresql-config-pvc
  replicationState: primary
```

**Issues:**
- ❌ No consistency guarantee across volumes
- ❌ Must manage 3 separate resources
- ❌ Failover requires 3 separate promote operations
- ❌ Potential for split-brain if one fails

### With Volume Group (Phase 2B)

```yaml
# Single VolumeGroupReplication resource
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplication
metadata:
  name: postgresql-complete-replication
  namespace: production
spec:
  volumeGroupReplicationClass: ceph-group-replication
  
  # Selector matches all PVCs for the database
  selector:
    matchLabels:
      app: postgresql
      instance: prod-db-01
  
  replicationState: primary
  autoResync: true
```

**Benefits:**
- ✅ Crash-consistent across all volumes
- ✅ Single resource to manage
- ✅ Atomic failover (one operation)
- ✅ Guaranteed group consistency

---

## Detailed Implementation Plan for Phase 2B

### Prompt 2B.1: Create VolumeGroupReplication Types

Create `api/v1alpha2/volumegroupreplication_types.go`:

```go
package v1alpha2

import (
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// COMPATIBILITY NOTICE:
// This matches kubernetes-csi-addons VolumeGroupReplication.
// Use this for multi-volume application consistency.

type VolumeGroupReplicationSpec struct {
    // VolumeGroupReplicationClass is the name of the class
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    VolumeGroupReplicationClass string `json:"volumeGroupReplicationClass"`
    
    // Selector to select PVCs that are part of this replication group
    // +kubebuilder:validation:Required
    Selector *metav1.LabelSelector `json:"selector"`
    
    // ReplicationState represents the desired replication state for the group
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=primary;secondary;resync
    ReplicationState string `json:"replicationState"`
    
    // AutoResync indicates if volumes should be automatically resynced
    // +optional
    AutoResync *bool `json:"autoResync,omitempty"`
    
    // Source is optional for specifying source volume group
    // +optional
    Source *corev1.TypedLocalObjectReference `json:"source,omitempty"`
}

type VolumeGroupReplicationStatus struct {
    // Conditions represent the group's current state
    // +optional
    // +patchMergeKey=type
    // +patchStrategy=merge
    Conditions []metav1.Condition `json:"conditions,omitempty"`
    
    // State represents the current group state
    // +optional
    State string `json:"state,omitempty"`
    
    // Message provides detailed information
    // +optional
    Message string `json:"message,omitempty"`
    
    // LastSyncTime for the entire group
    // +optional
    LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
    
    // LastSyncDuration for group sync operation
    // +optional
    LastSyncDuration *metav1.Duration `json:"lastSyncDuration,omitempty"`
    
    // ObservedGeneration reflects generation observed
    // +optional
    ObservedGeneration int64 `json:"observedGeneration,omitempty"`
    
    // PersistentVolumeClaimsRefList lists all PVCs in this group
    // +optional
    PersistentVolumeClaimsRefList []corev1.LocalObjectReference `json:"persistentVolumeClaimsRefList,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:resource:scope=Namespaced,shortName=vgr;volgrouprep
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.spec.replicationState`
//+kubebuilder:printcolumn:name="Class",type=string,JSONPath=`.spec.volumeGroupReplicationClass`
//+kubebuilder:printcolumn:name="PVCs",type=integer,JSONPath=`.status.persistentVolumeClaimsRefList[*]`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type VolumeGroupReplication struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec   VolumeGroupReplicationSpec   `json:"spec,omitempty"`
    Status VolumeGroupReplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type VolumeGroupReplicationList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []VolumeGroupReplication `json:"items"`
}

func init() {
    SchemeBuilder.Register(&VolumeGroupReplication{}, &VolumeGroupReplicationList{})
}
```

### Prompt 2B.2: Create VolumeGroupReplicationClass Types

Create `api/v1alpha2/volumegroupreplicationclass_types.go`:

```go
package v1alpha2

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Parameters for volume group replication:
//
// Common parameters:
// - consistencyGroup: "enabled" - ensure crash consistency across volumes
// - groupSnapshots: "true" - use group snapshots for consistency
//
// Ceph-specific parameters:
// - groupConsistency: "application" or "crash"
// - groupMirroringMode: "snapshot" or "journal"
//
// Trident-specific parameters:
// - consistencyGroupPolicy: Policy name for CG
// - replicationPolicy: "Async" or "Sync" for the group
//
// Dell PowerStore-specific parameters:
// - consistencyType: "Metro" or "Async"
// - protectionPolicy: Group protection policy name

type VolumeGroupReplicationClassSpec struct {
    // Provisioner identifies the backend for group replication
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:MinLength=1
    Provisioner string `json:"provisioner"`
    
    // Parameters for group replication configuration
    // +optional
    Parameters map[string]string `json:"parameters,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster,shortName=vgrc;volgrouprepclass
//+kubebuilder:printcolumn:name="Provisioner",type=string,JSONPath=`.spec.provisioner`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

type VolumeGroupReplicationClass struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    
    Spec VolumeGroupReplicationClassSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

type VolumeGroupReplicationClassList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []VolumeGroupReplicationClass `json:"items"`
}

func init() {
    SchemeBuilder.Register(&VolumeGroupReplicationClass{}, &VolumeGroupReplicationClassList{})
}
```

### Prompt 2B.3: Create Sample YAMLs

**`config/samples/volumegroupreplicationclass_ceph_group.yaml`:**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplicationClass
metadata:
  name: ceph-rbd-group-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    mirroringMode: "snapshot"
    schedulingInterval: "5m"
    groupConsistency: "crash"
    replication.storage.openshift.io/replication-secret-name: "rbd-secret"
    replication.storage.openshift.io/replication-secret-namespace: "rook-ceph"
```

**`config/samples/volumegroupreplication_postgresql.yaml`:**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplication
metadata:
  name: postgresql-complete-replication
  namespace: production
spec:
  volumeGroupReplicationClass: ceph-rbd-group-replication
  
  # This selector will match all PostgreSQL PVCs
  selector:
    matchLabels:
      app: postgresql
      instance: prod-db-01
  
  replicationState: primary
  autoResync: true

---
# Ensure your PVCs have the required labels:
# apiVersion: v1
# kind: PersistentVolumeClaim
# metadata:
#   name: postgresql-data-pvc
#   namespace: production
#   labels:
#     app: postgresql
#     instance: prod-db-01
#     component: data
# spec:
#   storageClassName: ceph-rbd
#   accessModes: [ReadWriteOnce]
#   resources:
#     requests:
#       storage: 100Gi
```

---

## Controller Support (Phase 3 Extension)

### VolumeGroupReplication Controller

Add to Phase 3:

**Prompt 3.5: Create VolumeGroupReplication Controller**

Create `controllers/volumegroupreplication_controller.go`:

```go
type VolumeGroupReplicationReconciler struct {
    client.Client
    Scheme          *runtime.Scheme
    AdapterRegistry *adapters.Registry
}

func (r *VolumeGroupReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. Fetch VolumeGroupReplication
    vgr := &v1alpha2.VolumeGroupReplication{}
    if err := r.Get(ctx, req.NamespacedName, vgr); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // 2. Fetch VolumeGroupReplicationClass
    vgrc, err := r.fetchClass(ctx, vgr.Spec.VolumeGroupReplicationClass)
    if err != nil {
        return r.handleError(ctx, vgr, err)
    }
    
    // 3. Find PVCs matching selector
    pvcList := &corev1.PersistentVolumeClaimList{}
    if err := r.List(ctx, pvcList, client.InNamespace(vgr.Namespace), client.MatchingLabels(vgr.Spec.Selector.MatchLabels)); err != nil {
        return r.handleError(ctx, vgr, err)
    }
    
    if len(pvcList.Items) == 0 {
        return r.handleError(ctx, vgr, fmt.Errorf("no PVCs match selector"))
    }
    
    // 4. Detect backend
    backend, err := r.detectBackend(vgrc.Spec.Provisioner)
    if err != nil {
        return r.handleError(ctx, vgr, err)
    }
    
    // 5. Get adapter and reconcile group
    adapter := r.AdapterRegistry.GetVolumeGroupReplicationAdapter(backend)
    result, err := adapter.ReconcileVolumeGroupReplication(ctx, vgr, vgrc, pvcList.Items)
    if err != nil {
        return r.handleError(ctx, vgr, err)
    }
    
    // 6. Update status with PVC list
    vgr.Status.PersistentVolumeClaimsRefList = make([]corev1.LocalObjectReference, len(pvcList.Items))
    for i, pvc := range pvcList.Items {
        vgr.Status.PersistentVolumeClaimsRefList[i] = corev1.LocalObjectReference{Name: pvc.Name}
    }
    
    return result, r.Status().Update(ctx, vgr)
}

// Watch PVCs and requeue VolumeGroupReplications if labels change
func (r *VolumeGroupReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha2.VolumeGroupReplication{}).
        Watches(
            &source.Kind{Type: &corev1.PersistentVolumeClaim{}},
            handler.EnqueueRequestsFromMapFunc(r.findGroupsForPVC),
        ).
        Complete(r)
}
```

---

## Adapter Interface Extension (Phase 4)

Add to `pkg/adapters/types.go`:

```go
// VolumeGroupReplicationAdapter handles group replication
type VolumeGroupReplicationAdapter interface {
    // ReconcileVolumeGroupReplication reconciles a group of volumes
    ReconcileVolumeGroupReplication(
        ctx context.Context,
        vgr *v1alpha2.VolumeGroupReplication,
        vgrc *v1alpha2.VolumeGroupReplicationClass,
        pvcs []corev1.PersistentVolumeClaim,
    ) (ctrl.Result, error)
    
    // DeleteVolumeGroupReplication cleans up group resources
    DeleteVolumeGroupReplication(
        ctx context.Context,
        vgr *v1alpha2.VolumeGroupReplication,
    ) error
}
```

### Dell Adapter (Naturally Group-Based)

Dell PowerStore already uses groups, so this is natural:

```go
func (a *PowerStoreAdapter) ReconcileVolumeGroupReplication(
    ctx context.Context,
    vgr *v1alpha2.VolumeGroupReplication,
    vgrc *v1alpha2.VolumeGroupReplicationClass,
    pvcs []corev1.PersistentVolumeClaim,
) (ctrl.Result, error) {
    // Label all PVCs in the group
    for _, pvc := range pvcs {
        if err := a.labelPVCForGroup(ctx, &pvc, vgr.Name); err != nil {
            return ctrl.Result{}, err
        }
    }
    
    // Create DellCSIReplicationGroup with PVCSelector
    drg := &DellCSIReplicationGroup{
        Spec: DellCSIReplicationGroupSpec{
            Action: a.translateAction(vgr.Spec.ReplicationState),
            PVCSelector: &metav1.LabelSelector{
                MatchLabels: map[string]string{
                    "replication.storage.dell.com/group": vgr.Name,
                },
            },
            ProtectionPolicy: vgrc.Spec.Parameters["protectionPolicy"],
            RemoteSystem:     vgrc.Spec.Parameters["remoteSystem"],
        },
    }
    
    return ctrl.Result{}, a.client.Patch(ctx, drg, client.Apply)
}
```

### Ceph Adapter (Use VolumeGroupSnapshot)

For Ceph, use Kubernetes VolumeGroupSnapshot for consistency:

```go
func (a *CephAdapter) ReconcileVolumeGroupReplication(
    ctx context.Context,
    vgr *v1alpha2.VolumeGroupReplication,
    vgrc *v1alpha2.VolumeGroupReplicationClass,
    pvcs []corev1.PersistentVolumeClaim,
) (ctrl.Result, error) {
    // Option 1: Create individual VolumeReplication for each PVC
    // but coordinate them for group consistency
    
    // Option 2: Use VolumeGroupSnapshot for crash consistency
    // then replicate the snapshot
    
    // For now, create coordinated VolumeReplications
    for _, pvc := range pvcs {
        vr := &VolumeReplication{
            ObjectMeta: metav1.ObjectMeta{
                Name:      fmt.Sprintf("%s-%s", vgr.Name, pvc.Name),
                Namespace: vgr.Namespace,
                Labels: map[string]string{
                    "volumeGroupReplication": vgr.Name,
                },
                OwnerReferences: []metav1.OwnerReference{
                    *metav1.NewControllerRef(vgr, v1alpha2.GroupVersion.WithKind("VolumeGroupReplication")),
                },
            },
            Spec: VolumeReplicationSpec{
                VolumeReplicationClass: vgr.Spec.VolumeGroupReplicationClass,
                PvcName:                pvc.Name,
                ReplicationState:       vgr.Spec.ReplicationState,
                AutoResync:             vgr.Spec.AutoResync,
            },
        }
        
        if err := a.client.Patch(ctx, vr, client.Apply); err != nil {
            return ctrl.Result{}, err
        }
    }
    
    return ctrl.Result{}, nil
}
```

### Trident Adapter (Consistency Groups)

Trident supports consistency groups natively:

```go
func (a *TridentAdapter) ReconcileVolumeGroupReplication(
    ctx context.Context,
    vgr *v1alpha2.VolumeGroupReplication,
    vgrc *v1alpha2.VolumeGroupReplicationClass,
    pvcs []corev1.PersistentVolumeClaim,
) (ctrl.Result, error) {
    // Trident TridentMirrorRelationship supports volumeMappings array
    volumeMappings := make([]VolumeMappingType, len(pvcs))
    for i, pvc := range pvcs {
        volumeMappings[i] = VolumeMappingType{
            LocalPVCName:       pvc.Name,
            RemoteVolumeHandle: fmt.Sprintf("remote-%s", pvc.Name),
        }
    }
    
    tmr := &TridentMirrorRelationship{
        ObjectMeta: metav1.ObjectMeta{
            Name:      vgr.Name,
            Namespace: vgr.Namespace,
        },
        Spec: TridentMirrorRelationshipSpec{
            State:             a.translateState(vgr.Spec.ReplicationState),
            ReplicationPolicy: vgrc.Spec.Parameters["replicationPolicy"],
            VolumeMappings:    volumeMappings,  // Multiple volumes!
        },
    }
    
    return ctrl.Result{}, a.client.Patch(ctx, tmr, client.Apply)
}
```

---

## Migration from v1alpha1 to Volume Groups

### Current v1alpha1 Limitation

The current UnifiedVolumeReplication only supports **single volume** replication:

```yaml
spec:
  volumeMapping:
    source:
      pvcName: "single-pvc"  # Only one PVC
```

### Volume Group Enhancement

For users who need multi-volume consistency, add a **new optional field** to v1alpha1 → v1alpha2 migration:

**Option A: Detect from labels**
```go
func translateToV1Alpha2(uvr *v1alpha1.UnifiedVolumeReplication) (*v1alpha2.VolumeReplication, error) {
    // Check if this is part of a group (via labels)
    if uvr.Labels["volumeGroup"] != "" {
        // This should be migrated to VolumeGroupReplication instead
        return nil, fmt.Errorf("resource is part of volume group, use VolumeGroupReplication")
    }
    
    // Normal single volume migration
    return &v1alpha2.VolumeReplication{...}, nil
}
```

**Option B: Manual migration for groups**
- Document that multi-volume applications should use VolumeGroupReplication
- Provide migration guide for grouping existing VolumeReplications

---

## Recommended Approach

### Phase 2B: Implement Volume Groups Now

**Do Phase 2B immediately after Phase 2** because:

1. **Dell already uses groups:** DellCSIReplicationGroup inherently works with groups via PVCSelector
2. **Complete spec coverage:** Full kubernetes-csi-addons compatibility
3. **Critical for databases:** PostgreSQL, MySQL, MongoDB commonly use multiple volumes
4. **Trident supports it:** TridentMirrorRelationship has `volumeMappings` array for groups

### Minimal Impact

Adding volume groups is **relatively simple** because:
- Types are similar to single volume types (just add selector)
- Controllers follow same pattern (just iterate over matched PVCs)
- Adapters already support groups (Dell native, Trident via arrays, Ceph via coordination)

### Updated Timeline

**Without Volume Groups:**
- Phase 2: 1 week

**With Volume Groups (Phase 2B):**
- Phase 2: 1 week
- Phase 2B: 1-2 weeks (types + samples)
- Phase 3 extension: +2-3 days (group controller)
- Phase 4 extension: +3-4 days (group adapters)

**Total Additional Time: ~2-3 weeks**

---

## Summary

**Volume Group Replication should be implemented as Phase 2B** to provide:
- ✅ Full kubernetes-csi-addons compatibility
- ✅ Multi-volume application support
- ✅ Crash-consistent group snapshots
- ✅ Atomic group operations
- ✅ Better Dell PowerStore integration (naturally group-based)

Would you like me to:
1. **Implement Phase 2B now** (add volume group types), or
2. **Continue with Phase 3** (controllers) and add groups later, or
3. **Create Phase 2B prompts** in the migration document for future implementation?

