# kubernetes-csi-addons VolumeReplication Spec Reference

## Overview

This document serves as the authoritative reference for the kubernetes-csi-addons `VolumeReplication` and `VolumeReplicationClass` CRD specifications. Our v1alpha2 API implementation must match these specifications exactly to ensure compatibility and enable future migration to Option A (using `replication.storage.openshift.io` API group directly).

**Source Repository:** [github.com/csi-addons/kubernetes-csi-addons](https://github.com/csi-addons/kubernetes-csi-addons)

**API Group:** `replication.storage.openshift.io`

**API Version:** `v1alpha1`

---

## VolumeReplication CRD

### API Definition

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeReplication
```

### VolumeReplicationSpec

The `VolumeReplicationSpec` defines the desired state of VolumeReplication.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `volumeReplicationClass` | string | ✅ Yes | Name of the VolumeReplicationClass to use for this replication |
| `pvcName` | string | ✅ Yes | Name of the PersistentVolumeClaim to be replicated |
| `replicationState` | string | ✅ Yes | Desired replication state. Valid values: `primary`, `secondary`, `resync` |
| `dataSource` | *corev1.TypedLocalObjectReference | ❌ No | Optional data source for cloning scenarios |
| `autoResync` | *bool | ❌ No | Whether to automatically resync after connection loss. Defaults to `false` |

#### Field Details

**volumeReplicationClass:**
- References a `VolumeReplicationClass` resource
- The class defines how replication should be configured (provisioner, parameters)
- Must exist in the cluster before creating VolumeReplication
- Cluster-scoped resource

**pvcName:**
- Must be a valid PersistentVolumeClaim name
- PVC must exist in the same namespace as the VolumeReplication
- PVC must be bound to a volume that supports replication

**replicationState:**
- **`primary`**: Volume is the primary source (can handle writes, replicates to secondary)
- **`secondary`**: Volume is a replica (read-only, receives replicated data from primary)
- **`resync`**: Force resynchronization of the volume (typically after failure recovery)

**dataSource:**
- Used for advanced cloning scenarios
- Type: `corev1.TypedLocalObjectReference`
- Optional field

**autoResync:**
- Controls automatic resynchronization behavior
- If `true`, automatically resync after connection recovery
- If `false` or nil, manual intervention required
- Defaults to `false` if not specified

### VolumeReplicationStatus

The `VolumeReplicationStatus` represents the observed state of VolumeReplication.

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | []metav1.Condition | Standard Kubernetes conditions representing the replication state |
| `state` | string | Current replication state observed by the controller |
| `message` | string | Human-readable message about the current state |
| `lastSyncTime` | *metav1.Time | Timestamp of the last successful synchronization |
| `lastSyncDuration` | *metav1.Duration | Duration of the last synchronization operation |
| `observedGeneration` | int64 | Generation of the spec that was observed by the controller |

#### Status Conditions

Standard condition types used:

| Type | Status | Reason | Description |
|------|--------|--------|-------------|
| `Ready` | `True` | `ReconcileComplete` | Replication is configured and working |
| `Ready` | `False` | `VolumeReplicationClassNotFound` | Referenced class doesn't exist |
| `Ready` | `False` | `PVCNotFound` | Referenced PVC doesn't exist |
| `Ready` | `False` | `ReplicationError` | Error in replication configuration or operation |
| `Syncing` | `True` | `Synchronizing` | Data synchronization in progress |
| `Syncing` | `False` | `Synchronized` | Data is synchronized |
| `Degraded` | `True` | `ConnectionLost` | Connection to remote site lost |
| `Degraded` | `False` | `Healthy` | Replication is healthy |

### Complete Example

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeReplication
metadata:
  name: database-replication
  namespace: production
spec:
  volumeReplicationClass: rbd-replication-class
  pvcName: database-pvc
  replicationState: primary
  autoResync: true
status:
  conditions:
  - type: Ready
    status: "True"
    reason: ReconcileComplete
    message: "Replication configured successfully"
    lastTransitionTime: "2024-10-28T10:00:00Z"
  - type: Syncing
    status: "False"
    reason: Synchronized
    message: "Data synchronized"
    lastTransitionTime: "2024-10-28T10:01:00Z"
  state: primary
  message: "Volume is primary and replicating"
  lastSyncTime: "2024-10-28T10:05:00Z"
  lastSyncDuration: 5s
  observedGeneration: 1
```

### Kubebuilder Markers

```go
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,shortName=vr;volrep
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.spec.replicationState`
//+kubebuilder:printcolumn:name="PVC",type=string,JSONPath=`.spec.pvcName`
//+kubebuilder:printcolumn:name="Class",type=string,JSONPath=`.spec.volumeReplicationClass`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

### Validation Rules

1. **Required Fields:**
   - `volumeReplicationClass` must not be empty
   - `pvcName` must not be empty
   - `replicationState` must not be empty

2. **Enum Validation:**
   - `replicationState` must be one of: `primary`, `secondary`, `resync`

3. **Name Validation:**
   - `pvcName` must be a valid Kubernetes resource name (DNS subdomain)
   - `volumeReplicationClass` must reference an existing VolumeReplicationClass

---

## VolumeReplicationClass CRD

### API Definition

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeReplicationClass
```

### VolumeReplicationClassSpec

The `VolumeReplicationClassSpec` defines replication parameters and backend configuration.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provisioner` | string | ✅ Yes | Name of the CSI driver provisioner that handles replication |
| `parameters` | map[string]string | ❌ No | Backend-specific parameters for replication configuration |

#### Field Details

**provisioner:**
- Identifies which CSI driver should handle the replication
- Must match the provisioner name of the StorageClass used by the PVC
- Common values:
  - `rbd.csi.ceph.com` - Ceph RBD
  - `csi.trident.netapp.io` - NetApp Trident
  - `csi-powerstore.dellemc.com` - Dell PowerStore

**parameters:**
- Key-value pairs passed to the CSI driver
- Backend-specific configuration
- Common parameters (varies by driver):

**Ceph-specific:**
```yaml
parameters:
  mirroringMode: "snapshot"  # or "journal"
  schedulingInterval: "1m"
  replication.storage.openshift.io/replication-secret-name: "rbd-secret"
  replication.storage.openshift.io/replication-secret-namespace: "default"
```

**General authentication parameters:**
```yaml
parameters:
  replication.storage.openshift.io/replication-secret-name: "replication-secret"
  replication.storage.openshift.io/replication-secret-namespace: "storage-system"
```

### Complete Example

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeReplicationClass
metadata:
  name: rbd-replication-class
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    mirroringMode: "snapshot"
    schedulingInterval: "5m"
    replication.storage.openshift.io/replication-secret-name: "rbd-replication-secret"
    replication.storage.openshift.io/replication-secret-namespace: "rook-ceph"
```

### Kubebuilder Markers

```go
//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Provisioner",type=string,JSONPath=`.spec.provisioner`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

### Validation Rules

1. **Required Fields:**
   - `provisioner` must not be empty

2. **Scope:**
   - VolumeReplicationClass is cluster-scoped (no namespace)

---

## State Machine

### Valid State Transitions

```
Initial → primary ✅
Initial → secondary ✅

primary → secondary ✅ (demotion)
primary → resync ✅ (force resync)

secondary → primary ✅ (promotion/failover)
secondary → resync ✅ (force resync)

resync → primary ✅ (after resync completes)
resync → secondary ✅ (after resync completes)
```

### State Behavior

**Primary State:**
- Volume accepts read/write operations
- Actively replicates changes to secondary site(s)
- Source of truth for data

**Secondary State:**
- Volume is in replica mode (typically read-only)
- Receives replicated data from primary
- Can be promoted to primary for failover

**Resync State:**
- Forces full resynchronization of data
- Used after split-brain scenarios or extended downtime
- May be resource-intensive operation
- Transitions to final state (primary or secondary) after completion

---

## Backend-Specific Implementations

### Ceph (RBD Mirroring)

**Provisioner:** `rbd.csi.ceph.com`

**Parameters:**
- `mirroringMode`: `snapshot` or `journal`
- `schedulingInterval`: Snapshot scheduling interval (e.g., "1m", "5m", "1h")

**State Mapping:**
- `primary` → Ceph: promote volume to primary
- `secondary` → Ceph: demote volume to secondary
- `resync` → Ceph: resync-image command

**Example:**
```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeReplicationClass
metadata:
  name: ceph-snapshot-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    mirroringMode: "snapshot"
    schedulingInterval: "5m"
    replication.storage.openshift.io/replication-secret-name: "rbd-secret"
    replication.storage.openshift.io/replication-secret-namespace: "rook-ceph"
```

---

## API Compatibility Notes

### For Option B (Our Current Plan)

We will use API group `replication.unified.io` with **identical** field structures:

```go
// Our API (replication.unified.io/v1alpha2)
type VolumeReplicationSpec struct {
    VolumeReplicationClass string                              `json:"volumeReplicationClass"`
    PvcName                string                              `json:"pvcName"`
    ReplicationState       string                              `json:"replicationState"`
    DataSource             *corev1.TypedLocalObjectReference   `json:"dataSource,omitempty"`
    AutoResync             *bool                               `json:"autoResync,omitempty"`
}
```

This MUST match kubernetes-csi-addons exactly:
```go
// kubernetes-csi-addons (replication.storage.openshift.io/v1alpha1)
type VolumeReplicationSpec struct {
    VolumeReplicationClass string                              `json:"volumeReplicationClass"`
    PvcName                string                              `json:"pvcName"`
    ReplicationState       string                              `json:"replicationState"`
    DataSource             *corev1.TypedLocalObjectReference   `json:"dataSource,omitempty"`
    AutoResync             *bool                               `json:"autoResync,omitempty"`
}
```

**Critical:** Field names, JSON tags, types, and order MUST be identical for future Option A migration.

### For Option A (Future Migration)

If we transition to Option A, we would:
1. Import kubernetes-csi-addons types directly
2. Use conversion webhooks to translate between API groups
3. Support both `replication.unified.io` and `replication.storage.openshift.io` simultaneously during migration
4. Eventually deprecate `replication.unified.io`

---

## VolumeGroupReplication CRD

### Overview

**VolumeGroupReplication** enables replication of multiple PVCs together as a single unit. This is critical for:
- Multi-volume applications (databases with separate data, logs, config volumes)
- Crash-consistent group snapshots
- Atomic group operations (promote all, demote all)

### API Definition

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
```

### VolumeGroupReplicationSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `volumeGroupReplicationClass` | string | ✅ Yes | Name of the VolumeGroupReplicationClass |
| `selector` | *metav1.LabelSelector | ✅ Yes | Label selector to identify PVCs in the group |
| `replicationState` | string | ✅ Yes | Desired state for all volumes in the group |
| `autoResync` | *bool | ❌ No | Auto-resync setting for the group |
| `source` | *corev1.TypedLocalObjectReference | ❌ No | Optional source group reference |

#### Field Details

**volumeGroupReplicationClass:**
- References a `VolumeGroupReplicationClass` resource
- Defines group-level replication parameters
- Cluster-scoped resource

**selector:**
- Standard Kubernetes label selector
- Selects all PVCs with matching labels in the same namespace
- Example:
  ```yaml
  selector:
    matchLabels:
      app: postgresql
      instance: prod-db-01
  ```

**replicationState:**
- Applied to ALL volumes in the group simultaneously
- Values: `primary`, `secondary`, `resync`
- Ensures consistent state across all volumes

### VolumeGroupReplicationStatus

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | []metav1.Condition | Group-level conditions |
| `state` | string | Current group state |
| `message` | string | Human-readable message |
| `lastSyncTime` | *metav1.Time | Last successful group sync |
| `lastSyncDuration` | *metav1.Duration | Duration of last group sync |
| `observedGeneration` | int64 | Observed spec generation |
| `persistentVolumeClaimsRefList` | []corev1.LocalObjectReference | List of PVCs in group |

### Complete Example

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
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
---
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplication
metadata:
  name: postgresql-database-group
  namespace: production
spec:
  volumeGroupReplicationClass: ceph-rbd-group-replication
  
  # Selector matches all PostgreSQL PVCs
  selector:
    matchLabels:
      app: postgresql
      instance: prod-db-01
  
  replicationState: primary
  autoResync: true

status:
  conditions:
  - type: Ready
    status: "True"
    reason: ReconcileComplete
    message: "Group replication configured for 3 volumes"
  state: primary
  message: "All volumes in group are primary and replicating"
  lastSyncTime: "2024-10-28T10:05:00Z"
  lastSyncDuration: 12s
  observedGeneration: 1
  persistentVolumeClaimsRefList:
  - name: postgresql-data-pvc
  - name: postgresql-logs-pvc
  - name: postgresql-config-pvc
```

### Kubebuilder Markers

```go
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:resource:scope=Namespaced,shortName=vgr;volgrouprep
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.spec.replicationState`
//+kubebuilder:printcolumn:name="Class",type=string,JSONPath=`.spec.volumeGroupReplicationClass`
//+kubebuilder:printcolumn:name="PVCs",type=integer,JSONPath=`.status.persistentVolumeClaimsRefList[*]`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
```

---

## VolumeGroupReplicationClass CRD

### API Definition

```yaml
apiVersion: replication.storage.openshift.io/v1alpha1
kind: VolumeGroupReplicationClass
```

### VolumeGroupReplicationClassSpec

Identical structure to VolumeReplicationClass but for groups:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provisioner` | string | ✅ Yes | CSI driver provisioner for group replication |
| `parameters` | map[string]string | ❌ No | Group-specific parameters |

#### Group-Specific Parameters

**Ceph:**
```yaml
parameters:
  mirroringMode: "snapshot"
  groupConsistency: "crash"  # or "application"
  groupSnapshots: "true"
```

**Trident:**
```yaml
parameters:
  replicationPolicy: "Async"
  consistencyGroupPolicy: "cg-policy-name"
  groupReplicationSchedule: "15m"
```

**Dell PowerStore:**
```yaml
parameters:
  consistencyType: "Metro"  # or "Async"
  protectionPolicy: "group-15min-async"
  remoteSystem: "PS-DR-001"
```

---

## Single Volume vs. Volume Group Decision Matrix

| Scenario | Use VolumeReplication | Use VolumeGroupReplication |
|----------|----------------------|---------------------------|
| Single-volume app (e.g., static website) | ✅ Yes | ❌ No |
| Database with separate data/logs volumes | ❌ No | ✅ Yes (consistency critical) |
| Multiple independent app volumes | ✅ Yes (one per volume) | ❌ No |
| Multi-tier app with shared state | ❌ No | ✅ Yes (atomic failover) |
| Stateful set with pod-specific volumes | ✅ Yes (one per pod) | ⚠️ Depends (group if consistency needed) |

---

## References

- **Repository:** https://github.com/csi-addons/kubernetes-csi-addons
- **CRD Definitions:** https://github.com/csi-addons/kubernetes-csi-addons/tree/main/config/crd
- **API Types:** https://github.com/csi-addons/kubernetes-csi-addons/tree/main/api/replication.storage/v1alpha1
- **Examples:** https://github.com/csi-addons/kubernetes-csi-addons/tree/main/config/samples

---

## Version History

| Date | Version | Changes |
|------|---------|---------|
| 2024-10-28 | 1.0 | Initial documentation based on kubernetes-csi-addons v0.9.0 |

---

## Compliance Checklist

When implementing v1alpha2, verify:

**Single Volume Replication:**
- [ ] VolumeReplication field names match exactly (including case)
- [ ] VolumeReplication JSON tags match exactly
- [ ] VolumeReplication field types match exactly
- [ ] Validation rules match (enum values, required fields)
- [ ] VolumeReplication status structure matches
- [ ] Condition types match standard Kubernetes conventions
- [ ] No custom fields added to VolumeReplicationSpec or Status
- [ ] VolumeReplicationClass parameters follow naming conventions
- [ ] State enum values are: `primary`, `secondary`, `resync` (no others)

**Volume Group Replication (if implementing Phase 2B):**
- [ ] VolumeGroupReplication field names match exactly
- [ ] VolumeGroupReplication JSON tags match exactly
- [ ] VolumeGroupReplication field types match exactly
- [ ] Selector field uses standard metav1.LabelSelector
- [ ] VolumeGroupReplication status includes persistentVolumeClaimsRefList
- [ ] No custom fields added to VolumeGroupReplicationSpec or Status
- [ ] VolumeGroupReplicationClass parameters documented
- [ ] Group state transitions work atomically across all PVCs

