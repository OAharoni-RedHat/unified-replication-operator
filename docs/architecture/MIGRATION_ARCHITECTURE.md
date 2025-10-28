# Migration Architecture: v1alpha1 → v1alpha2

## Executive Summary

This document outlines the architectural approach for migrating the unified-replication-operator from its current `UnifiedVolumeReplication` v1alpha1 API to a kubernetes-csi-addons compatible `VolumeReplication` v1alpha2 API. The migration follows **Option B** (using our own API group `replication.unified.io` with kubernetes-csi-addons-compatible structure) while maintaining architectural flexibility for a future **Option A** transition (direct use of `replication.storage.openshift.io` API group).

**Key Goals:**
- ✅ Adopt kubernetes-csi-addons standard API for maximum ecosystem compatibility
- ✅ Maintain multi-backend translation capabilities (Ceph, Trident, Dell PowerStore)
- ✅ Preserve backward compatibility during 12-month deprecation period
- ✅ Enable seamless future migration to Option A if desired

---

## 1. API Version Strategy

### Current State (v1alpha1)

**API Group:** `replication.unified.io/v1alpha1`

**Resource:** `UnifiedVolumeReplication`

**Characteristics:**
- Complex spec with source/destination endpoints
- Rich volumeMapping configuration
- Schedule with RPO/RTO
- Vendor-specific extensions (Ceph, Trident, PowerStore)

**Example:**
```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: app-replication
spec:
  sourceEndpoint:
    cluster: "prod-cluster"
    region: "us-east-1"
    storageClass: "ceph-rbd"
  destinationEndpoint:
    cluster: "dr-cluster"
    region: "us-west-2"
    storageClass: "ceph-rbd"
  volumeMapping:
    source:
      pvcName: "app-data"
      namespace: "production"
    destination:
      volumeHandle: "vol-12345"
      namespace: "disaster-recovery"
  replicationState: source  # custom state names
  replicationMode: asynchronous
  schedule:
    rpo: "15m"
    rto: "5m"
    mode: continuous
  extensions:
    ceph:
      mirroringMode: "snapshot"
```

### Target State (v1alpha2)

**API Group:** `replication.unified.io/v1alpha2`

**Resources:** `VolumeReplication` + `VolumeReplicationClass`

**Characteristics:**
- Simple, kubernetes-csi-addons compatible spec
- PVC-centric (single PVC reference)
- Standard state names (primary, secondary, resync)
- Backend configuration separated into VolumeReplicationClass
- Binary-compatible with kubernetes-csi-addons spec

**Example:**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: ceph-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    mirroringMode: "snapshot"
    schedulingInterval: "15m"
    replication.storage.openshift.io/replication-secret-name: "rbd-secret"
    replication.storage.openshift.io/replication-secret-namespace: "rook-ceph"
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: app-replication
  namespace: production
spec:
  volumeReplicationClass: ceph-replication
  pvcName: app-data
  replicationState: primary  # kubernetes-csi-addons standard
  autoResync: true
```

### Coexistence Period

During the migration period (12 months), both APIs will be supported:

| Timeline | v1alpha1 | v1alpha2 | Notes |
|----------|----------|----------|-------|
| v2.0.0 Release | ⚠️ Deprecated | ✅ Stable | Migration begins |
| v2.x.x (0-12 months) | ⚠️ Supported | ✅ Recommended | Security fixes only for v1alpha1 |
| v3.0.0 (12 months) | ❌ Removed | ✅ Only version | Migration complete |

---

## 2. Backward Compatibility Plan

### Dual Controller Approach

```
┌─────────────────────────────────────────┐
│         Main Controller Manager         │
└─────────────────────────────────────────┘
              │         │
              │         │
    ┌─────────┘         └──────────┐
    │                                │
    ▼                                ▼
┌────────────────┐          ┌──────────────────┐
│ UVR Controller │          │  VR Controller   │
│  (v1alpha1)    │          │   (v1alpha2)     │
│  DEPRECATED    │          │   NEW/PRIMARY    │
└────────────────┘          └──────────────────┘
    │                                │
    │                                │
    ▼                                ▼
┌────────────────────────────────────────────┐
│         Adapter Registry (Shared)          │
└────────────────────────────────────────────┘
    │              │              │
    ▼              ▼              ▼
┌────────┐    ┌─────────┐   ┌──────────┐
│  Ceph  │    │ Trident │   │   Dell   │
│Adapter │    │ Adapter │   │  Adapter │
└────────┘    └─────────┘   └──────────┘
```

### Controller Responsibilities

**UnifiedVolumeReplicationController (v1alpha1):**
- Watches `unifiedvolumereplications.replication.unified.io/v1alpha1`
- Continues existing reconciliation logic
- Marked deprecated in code and logs
- No new features added
- Security fixes and critical bugs only

**VolumeReplicationController (v1alpha2):**
- Watches `volumereplications.replication.unified.io/v1alpha2`
- Watches `volumereplicationclasses.replication.unified.io/v1alpha2`
- New reconciliation logic based on kubernetes-csi-addons patterns
- All new features implemented here

### Migration Timeline

**Month 0 (v2.0.0 Release):**
- v1alpha2 introduced, v1alpha1 deprecated
- Both APIs fully functional
- Migration tool released
- Documentation updated

**Months 1-3:**
- Early adopters migrate
- Collect feedback, fix issues
- Refinements to v1alpha2 based on usage

**Months 4-9:**
- Majority of users migrate
- Active communication about deprecation
- Warning logs for v1alpha1 usage

**Months 10-12:**
- Final migration push
- Announce v3.0.0 timeline
- Intensive support for remaining users

**Month 12 (v3.0.0 Release):**
- v1alpha1 removed completely
- v1alpha2 only

### Resource Coexistence

Users can run both v1alpha1 and v1alpha2 resources simultaneously:

```bash
# v1alpha1 resources (deprecated)
kubectl get uvr --all-namespaces

# v1alpha2 resources (new)
kubectl get vr --all-namespaces

# Both can exist in same namespace
kubectl get uvr,vr -n production
```

**No conflicts** because:
- Different API groups
- Different resource kinds
- Different backend resource naming
- Separate controllers

---

## 3. Backend Detection Strategy

### Detection Flow

```
VolumeReplication Created
    │
    ▼
Fetch VolumeReplicationClass
    │
    ▼
Extract .spec.provisioner
    │
    ▼
┌──────────────────────┐
│ Backend Detection    │
│                      │
│  provisioner string  │
│  contains...         │
│                      │
│  ✓ "ceph"           │──→  BackendCeph
│  ✓ "rbd.csi.ceph"   │
│                      │
│  ✓ "trident"        │──→  BackendTrident
│  ✓ "csi.trident"    │
│                      │
│  ✓ "powerstore"     │──→  BackendDell
│  ✓ "dellemc"        │
│                      │
│  ✗ unknown          │──→  Error
└──────────────────────┘
    │
    ▼
Get Adapter from Registry
    │
    ▼
Execute Adapter.ReconcileVolumeReplication()
```

### Detection Rules

**Ceph Backend:**
```go
func detectBackend(provisioner string) BackendType {
    prov := strings.ToLower(provisioner)
    
    if strings.Contains(prov, "ceph") ||
       strings.Contains(prov, "rbd.csi.ceph.com") {
        return BackendCeph
    }
    //...
}
```

**Examples:**
- `rbd.csi.ceph.com` → BackendCeph ✅
- `cephfs.csi.ceph.com` → BackendCeph ✅
- `ceph-rbd` → BackendCeph ✅

**Trident Backend:**
```go
if strings.Contains(prov, "trident") ||
   strings.Contains(prov, "csi.trident.netapp.io") ||
   strings.Contains(prov, "netapp") {
    return BackendTrident
}
```

**Examples:**
- `csi.trident.netapp.io` → BackendTrident ✅
- `trident-san` → BackendTrident ✅
- `netapp.io/trident` → BackendTrident ✅

**Dell PowerStore Backend:**
```go
if strings.Contains(prov, "powerstore") ||
   strings.Contains(prov, "dellemc") ||
   strings.Contains(prov, "csi-powerstore.dellemc.com") {
    return BackendDell
}
```

**Examples:**
- `csi-powerstore.dellemc.com` → BackendDell ✅
- `powerstore` → BackendDell ✅
- `dellemc.com/powerstore` → BackendDell ✅

### Fallback Mechanism

If provisioner is ambiguous, fall back to PVC's StorageClass:

```go
func (r *VolumeReplicationReconciler) detectBackendFromPVC(
    ctx context.Context,
    pvcName, namespace string,
) (BackendType, error) {
    // Get PVC
    pvc := &corev1.PersistentVolumeClaim{}
    if err := r.Get(ctx, types.NamespacedName{
        Name: pvcName, Namespace: namespace,
    }, pvc); err != nil {
        return "", err
    }
    
    // Get StorageClass
    sc := &storagev1.StorageClass{}
    if err := r.Get(ctx, types.NamespacedName{
        Name: *pvc.Spec.StorageClassName,
    }, sc); err != nil {
        return "", err
    }
    
    // Detect from SC provisioner
    return detectBackend(sc.Provisioner)
}
```

### Error Handling

```go
if backend == BackendUnknown {
    r.setCondition(vr, metav1.Condition{
        Type:    "Ready",
        Status:  metav1.ConditionFalse,
        Reason:  "UnknownBackend",
        Message: fmt.Sprintf("Cannot determine backend from provisioner: %s", provisioner),
    })
    return ctrl.Result{}, fmt.Errorf("unknown backend")
}
```

---

## 4. Translation Strategy

### Architecture Overview

```
VolumeReplication (v1alpha2)
    │
    │  kubernetes-csi-addons standard API
    │  - volumeReplicationClass
    │  - pvcName
    │  - replicationState (primary/secondary/resync)
    │  - autoResync
    │
    ▼
┌─────────────────────────────────────────┐
│      Adapter Layer (Translation)        │
└─────────────────────────────────────────┘
    │           │             │
    │           │             │
┌───▼───────┐ ┌─▼──────────┐ ┌──▼─────────────┐
│   Ceph    │ │  Trident   │ │  Dell          │
│ (Native)  │ │ (Translate)│ │ (Translate)    │
└───────────┘ └────────────┘ └────────────────┘
    │           │             │
    ▼           ▼             ▼
┌───────────────────────────────────────────┐
│         Backend CRs                        │
│  - VolumeReplication (Ceph)               │
│  - TridentMirrorRelationship (Trident)    │
│  - DellCSIReplicationGroup (Dell)         │
└───────────────────────────────────────────┘
```

### Ceph: Passthrough (Native)

Ceph uses kubernetes-csi-addons natively, so minimal translation:

```go
func (a *CephAdapter) ReconcileVolumeReplication(
    ctx context.Context,
    vr *v1alpha2.VolumeReplication,
    vrc *v1alpha2.VolumeReplicationClass,
) (ctrl.Result, error) {
    // Create backend Ceph VolumeReplication with identical spec
    cephVR := &CephVolumeReplication{
        ObjectMeta: metav1.ObjectMeta{
            Name:      vr.Name,
            Namespace: vr.Namespace,
            OwnerReferences: []metav1.OwnerReference{
                *metav1.NewControllerRef(vr, v1alpha2.GroupVersion.WithKind("VolumeReplication")),
            },
        },
        Spec: CephVolumeReplicationSpec{
            VolumeReplicationClass: vr.Spec.VolumeReplicationClass,
            PvcName:                vr.Spec.PvcName,
            ReplicationState:       vr.Spec.ReplicationState, // No translation!
            AutoResync:             vr.Spec.AutoResync,
        },
    }
    
    // Apply (create or update)
    return ctrl.Result{}, a.client.Patch(ctx, cephVR, client.Apply, fieldOwner)
}
```

**No State Translation Needed:**
- `primary` → `primary` (1:1)
- `secondary` → `secondary` (1:1)
- `resync` → `resync` (1:1)

### Trident: State Translation

Trident uses different state names requiring translation:

```go
func (a *TridentAdapter) ReconcileVolumeReplication(
    ctx context.Context,
    vr *v1alpha2.VolumeReplication,
    vrc *v1alpha2.VolumeReplicationClass,
) (ctrl.Result, error) {
    // Translate state
    tridentState := a.translateState(vr.Spec.ReplicationState)
    
    // Extract parameters
    replicationPolicy := vrc.Spec.Parameters["replicationPolicy"] // "Async" or "Sync"
    schedule := vrc.Spec.Parameters["replicationSchedule"]
    
    // Create TridentMirrorRelationship
    tmr := &TridentMirrorRelationship{
        ObjectMeta: metav1.ObjectMeta{
            Name:      vr.Name,
            Namespace: vr.Namespace,
            OwnerReferences: []metav1.OwnerReference{
                *metav1.NewControllerRef(vr, v1alpha2.GroupVersion.WithKind("VolumeReplication")),
            },
        },
        Spec: TridentMirrorRelationshipSpec{
            State:                tridentState,
            ReplicationPolicy:    replicationPolicy,
            ReplicationSchedule:  schedule,
            VolumeMappings: []VolumeMapping{
                {
                    LocalPVCName:       vr.Spec.PvcName,
                    RemoteVolumeHandle: vrc.Spec.Parameters["remoteVolume"],
                },
            },
        },
    }
    
    return ctrl.Result{}, a.client.Patch(ctx, tmr, client.Apply, fieldOwner)
}

func (a *TridentAdapter) translateState(vrState string) string {
    switch vrState {
    case "primary":
        return "established"  // Trident's primary state
    case "secondary":
        return "reestablishing"  // Trident's secondary state
    case "resync":
        return "reestablishing"  // Force re-establish
    default:
        return "established"
    }
}
```

**State Translation Table:**

| v1alpha2 (kubernetes-csi-addons) | Trident | Notes |
|----------------------------------|---------|-------|
| `primary` | `established` | Volume is primary, mirror established |
| `secondary` | `reestablishing` | Volume is secondary, receiving updates |
| `resync` | `reestablishing` | Force re-establishment of mirror |

**VolumeReplicationClass Parameters (Trident):**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: trident-replication
spec:
  provisioner: csi.trident.netapp.io
  parameters:
    replicationPolicy: "Async"  # or "Sync"
    replicationSchedule: "15m"
    remoteCluster: "dr-cluster"
    remoteSVM: "svm-dr"
    remoteVolume: "remote-vol-handle"
```

### Dell PowerStore: Action Translation

Dell uses "actions" instead of states:

```go
func (a *PowerStoreAdapter) ReconcileVolumeReplication(
    ctx context.Context,
    vr *v1alpha2.VolumeReplication,
    vrc *v1alpha2.VolumeReplicationClass,
) (ctrl.Result, error) {
    // Translate to Dell action
    dellAction := a.translateAction(vr.Spec.ReplicationState)
    
    // Extract parameters
    protectionPolicy := vrc.Spec.Parameters["protectionPolicy"]
    remoteSystem := vrc.Spec.Parameters["remoteSystem"]
    rpo := vrc.Spec.Parameters["rpo"]
    
    // Label PVC for Dell selector
    if err := a.labelPVC(ctx, vr.Spec.PvcName, vr.Namespace, vr.Name); err != nil {
        return ctrl.Result{}, err
    }
    
    // Create DellCSIReplicationGroup
    drg := &DellCSIReplicationGroup{
        ObjectMeta: metav1.ObjectMeta{
            Name:      vr.Name,
            Namespace: vr.Namespace,
            OwnerReferences: []metav1.OwnerReference{
                *metav1.NewControllerRef(vr, v1alpha2.GroupVersion.WithKind("VolumeReplication")),
            },
        },
        Spec: DellCSIReplicationGroupSpec{
            DriverName:       "csi-powerstore.dellemc.com",
            Action:           dellAction,
            ProtectionPolicy: protectionPolicy,
            RemoteSystem:     remoteSystem,
            RemoteRPO:        rpo,
            PVCSelector: &metav1.LabelSelector{
                MatchLabels: map[string]string{
                    "replication.storage.dell.com/group": vr.Name,
                },
            },
        },
    }
    
    return ctrl.Result{}, a.client.Patch(ctx, drg, client.Apply, fieldOwner)
}

func (a *PowerStoreAdapter) translateAction(vrState string) string {
    switch vrState {
    case "primary":
        return "Failover"  // Promote to primary (failover to this site)
    case "secondary":
        return "Sync"      // Sync as secondary from primary
    case "resync":
        return "Reprotect" // Re-establish protection after failover
    default:
        return "Sync"
    }
}
```

**Action Translation Table:**

| v1alpha2 (kubernetes-csi-addons) | Dell Action | Dell Behavior |
|----------------------------------|-------------|---------------|
| `primary` | `Failover` | Promote this volume to primary (write-enabled) |
| `secondary` | `Sync` | Sync from remote primary (read-only) |
| `resync` | `Reprotect` | Re-establish replication relationship |

**VolumeReplicationClass Parameters (Dell):**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: powerstore-replication
spec:
  provisioner: csi-powerstore.dellemc.com
  parameters:
    protectionPolicy: "15min-async"  # Dell protection policy
    remoteSystem: "PS-DR-001"         # Remote PowerStore system ID
    rpo: "15m"                        # Recovery Point Objective
    remoteClusterId: "dr-k8s-cluster"
```

### Status Synchronization

Each adapter syncs status back to VolumeReplication:

```go
// Read backend status
backendStatus := a.getBackendStatus(ctx, vr)

// Update our VolumeReplication status
vr.Status.State = translateBackendState(backendStatus.State)
vr.Status.Message = backendStatus.Message
vr.Status.LastSyncTime = backendStatus.LastSyncTime
vr.Status.LastSyncDuration = backendStatus.LastSyncDuration

// Update conditions
meta.SetStatusCondition(&vr.Status.Conditions, metav1.Condition{
    Type:    "Ready",
    Status:  metav1.ConditionTrue,
    Reason:  "ReconcileComplete",
    Message: "Replication configured and syncing",
})

// Save status
return r.Status().Update(ctx, vr)
```

---

## 5. Option A Future-Proofing

### Why Option A Might Be Chosen Later

**Potential reasons to transition:**
1. **Native Ecosystem Integration:** Direct compatibility with Ceph and other kubernetes-csi-addons users
2. **Reduced Maintenance:** Leverage upstream kubernetes-csi-addons CRDs and controllers
3. **Community Standard:** Follow community direction if kubernetes-csi-addons becomes de facto standard
4. **Operator Coexistence:** Run alongside native kubernetes-csi-addons operator

### Architectural Decisions for Easy Option A Transition

**1. Identical Struct Definitions**

Our v1alpha2 types MUST be binary-identical to kubernetes-csi-addons:

```go
// ✅ CORRECT: Identical to kubernetes-csi-addons
type VolumeReplicationSpec struct {
    VolumeReplicationClass string                              `json:"volumeReplicationClass"`
    PvcName                string                              `json:"pvcName"`
    ReplicationState       string                              `json:"replicationState"`
    DataSource             *corev1.TypedLocalObjectReference   `json:"dataSource,omitempty"`
    AutoResync             *bool                               `json:"autoResync,omitempty"`
}

// ❌ WRONG: Added custom field - breaks Option A compatibility
type VolumeReplicationSpec struct {
    VolumeReplicationClass string                              `json:"volumeReplicationClass"`
    PvcName                string                              `json:"pvcName"`
    ReplicationState       string                              `json:"replicationState"`
    DataSource             *corev1.TypedLocalObjectReference   `json:"dataSource,omitempty"`
    AutoResync             *bool                               `json:"autoResync,omitempty"`
    CustomField            string                              `json:"customField,omitempty"` // ❌ NO!
}
```

**2. Conversion Webhook Framework**

Create framework (dormant) for future conversion:

```go
// api/conversion/webhook.go
package conversion

// ConvertTo converts from replication.unified.io to replication.storage.openshift.io
func (src *v1alpha2.VolumeReplication) ConvertTo(dstRaw conversion.Hub) error {
    // TODO: Implement when Option A is chosen
    // Since structs are identical, this is straightforward:
    // dst := dstRaw.(*csiaddonsv1alpha1.VolumeReplication)
    // dst.Spec = src.Spec  // Direct assignment works due to identical types
    // dst.Status = src.Status
    return nil
}

// ConvertFrom converts from replication.storage.openshift.io to replication.unified.io
func (dst *v1alpha2.VolumeReplication) ConvertFrom(srcRaw conversion.Hub) error {
    // TODO: Implement when Option A is chosen
    return nil
}
```

**3. Compatibility Tests**

Continuous verification that our API matches kubernetes-csi-addons:

```go
// test/compatibility/csi_addons_compatibility_test.go
func TestVolumeReplicationSpecCompatibility(t *testing.T) {
    // Use reflection to verify struct fields match kubernetes-csi-addons
    ourType := reflect.TypeOf(v1alpha2.VolumeReplicationSpec{})
    // csiAddonsType := reflect.TypeOf(csiaddonsv1alpha1.VolumeReplicationSpec{})
    
    // Verify field count, names, types, tags all match
    // This test fails if we drift from kubernetes-csi-addons
}
```

**4. Controller Abstraction**

Controller logic should be backend-agnostic:

```go
// ✅ GOOD: Works with any API group
func (r *VolumeReplicationReconciler) reconcile(
    ctx context.Context,
    vr VolumeReplicationInterface, // Interface, not concrete type
) (ctrl.Result, error) {
    // Business logic doesn't care about API group
}

// VolumeReplicationInterface can be satisfied by:
// - replication.unified.io/v1alpha2 VolumeReplication
// - replication.storage.openshift.io/v1alpha1 VolumeReplication (future)
```

**5. Dual API Group Support Plan**

When transitioning to Option A:

```
Phase 1: Current (Option B)
  - Watch: replication.unified.io/v1alpha2
  - CRDs: Our VolumeReplication + VolumeReplicationClass
  
Phase 2: Dual Support (Transition)
  - Watch: replication.unified.io/v1alpha2 AND replication.storage.openshift.io/v1alpha1
  - CRDs: Both API groups installed
  - Conversion webhooks active
  
Phase 3: Option A (Final)
  - Watch: replication.storage.openshift.io/v1alpha1 only
  - CRDs: kubernetes-csi-addons only
  - Legacy replication.unified.io deprecated
```

### Option A Transition Complexity

**Low Complexity (due to preparations):**
- ✅ Struct fields identical → simple conversion
- ✅ Controller logic API-agnostic → minimal changes
- ✅ Adapter layer unchanged → no backend impact
- ✅ Webhook framework ready → just implement TODO sections

**Estimated Effort:**
- Development: 3-4 weeks
- Testing: 2-3 weeks
- User Migration: 6-12 months (same as v1alpha1 → v1alpha2)

---

## 6. Data Flow Diagrams

### VolumeReplication Creation Flow

```
User Creates VolumeReplication
          │
          │  apiVersion: replication.unified.io/v1alpha2
          │  kind: VolumeReplication
          │  spec:
          │    volumeReplicationClass: "my-class"
          │    pvcName: "my-data"
          │    replicationState: "primary"
          │
          ▼
┌─────────────────────────┐
│  API Server Validation  │
│  - Schema validation    │
│  - Webhooks (if any)    │
└─────────────────────────┘
          │
          ▼
┌──────────────────────────────────┐
│  VolumeReplicationReconciler     │
│  1. Fetch VolumeReplication      │
│  2. Add finalizer                │
│  3. Fetch VolumeReplicationClass │
└──────────────────────────────────┘
          │
          ▼
┌──────────────────────────┐
│  Backend Detection       │
│  provisioner: "rbd..."   │
│  → BackendCeph           │
└──────────────────────────┘
          │
          ▼
┌──────────────────────────┐
│  Get Adapter from        │
│  Registry                │
│  → CephAdapter           │
└──────────────────────────┘
          │
          ▼
┌────────────────────────────────────┐
│  CephAdapter.ReconcileVolumeRep    │
│  - Create backend VR CR            │
│  - Set owner reference             │
│  - Apply (Server-Side Apply)       │
└────────────────────────────────────┘
          │
          ▼
┌────────────────────────────────┐
│  Backend Ceph VolumeReplication│
│  (replication.storage.         │
│   openshift.io/v1alpha1)       │
└────────────────────────────────┘
          │
          ▼
┌──────────────────────────┐
│  Ceph CSI Driver         │
│  - RBD mirroring         │
│  - Snapshot scheduling   │
└──────────────────────────┘
          │
          ▼
┌──────────────────────────┐
│  Status Sync Back        │
│  Backend → VolumeRep     │
└──────────────────────────┘
```

### Backend Translation Comparison

#### Ceph (Passthrough)
```
VR.spec.replicationState: "primary"
  │
  │ (no translation)
  ▼
CephVR.spec.replicationState: "primary"
```

#### Trident (Translation)
```
VR.spec.replicationState: "primary"
  │
  │ (translate)
  ▼
TMR.spec.state: "established"
```

#### Dell (Translation)
```
VR.spec.replicationState: "primary"
  │
  │ (translate)
  ▼
DRG.spec.action: "Failover"
```

---

## 7. Testing Strategy

### API Compatibility Tests
- Struct field comparison with kubernetes-csi-addons
- JSON serialization compatibility
- YAML roundtrip tests
- Continuous monitoring for upstream changes

### Unit Tests
- Backend detection logic
- State translation functions
- Adapter reconciliation logic
- Status synchronization

### Integration Tests
- End-to-end VolumeReplication lifecycle
- Backend resource creation/update/deletion
- Cross-backend workflows
- Error scenarios

### Backward Compatibility Tests
- v1alpha1 and v1alpha2 coexistence
- No interference between versions
- Migration tool validation
- Rollback scenarios

### Performance Tests
- Large-scale resource management
- Controller scalability
- Backend adapter efficiency

---

## 8. Rollout Plan

### Pre-Release (Weeks 1-12)
- [ ] Implement v1alpha2 types
- [ ] Create new controller
- [ ] Refactor adapters
- [ ] Comprehensive testing
- [ ] Documentation

### v2.0.0 Release (Week 13)
- [ ] Release with both APIs
- [ ] Mark v1alpha1 deprecated
- [ ] Migration tool available
- [ ] Full documentation published

### Post-Release (Months 1-12)
- [ ] Monitor adoption
- [ ] Support users during migration
- [ ] Collect feedback
- [ ] Iterate on v1alpha2

### v3.0.0 Prep (Month 11-12)
- [ ] Final migration push
- [ ] Verify all users migrated
- [ ] Prepare v3.0.0 (v1alpha1 removal)

### v3.0.0 Release (Month 13)
- [ ] Remove v1alpha1 code
- [ ] Clean up deprecated paths
- [ ] v1alpha2 only

---

## 9. Risk Mitigation

### Risk: Users Don't Migrate in Time
**Mitigation:**
- 12-month deprecation period (generous)
- Active communication (docs, warnings, emails)
- Migration tool makes it easy
- Support during transition

### Risk: v1alpha2 Has Issues Not Found in Testing
**Mitigation:**
- Extensive testing before release
- Both APIs supported during transition
- Easy rollback to v1alpha1
- Iterative improvements in v2.x.x

### Risk: kubernetes-csi-addons API Changes
**Mitigation:**
- Compatibility tests detect changes
- Monitor upstream repository
- Quick response to changes
- Our API group gives us control

### Risk: Backend Adapter Issues
**Mitigation:**
- Comprehensive adapter tests
- Gradual rollout per backend
- Adapter registry allows easy fixes
- Backend-specific release notes

---

## 10. Success Metrics

- ✅ **API Compatibility:** 100% match with kubernetes-csi-addons spec
- ✅ **Migration Rate:** >90% users migrated by month 9
- ✅ **Zero Data Loss:** No replication interruption during migration
- ✅ **Test Coverage:** >85% code coverage
- ✅ **Documentation Quality:** All features documented with examples
- ✅ **Performance:** No regression from v1alpha1
- ✅ **User Satisfaction:** Positive feedback on migration process

---

## Document Version

| Date | Version | Changes |
|------|---------|---------|
| 2024-10-28 | 1.0 | Initial architecture document |

---

## References

- [CSI Addons Spec Reference](../api-reference/CSI_ADDONS_SPEC_REFERENCE.md)
- [Migration Plan](../../MIGRATION_TO_CSI_ADDONS_SPEC.md)
- [Option A Transition Procedure](./OPTION_A_TRANSITION_PROCEDURE.md) (to be created)

