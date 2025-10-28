# API Reference - Unified Replication Operator

## Overview

The Unified Replication Operator provides kubernetes-csi-addons compatible APIs for storage replication across multiple backends. This document covers the v1alpha2 API which is the primary and recommended API.

**API Group:** `replication.unified.io/v1alpha2`

**Compatible With:** kubernetes-csi-addons `replication.storage.openshift.io/v1alpha1`

**Supported Backends:**
- Ceph (via kubernetes-csi-addons native VolumeReplication)
- NetApp Trident (with state translation)
- Dell PowerStore (with action translation)

---

## Core Resources

### VolumeReplication

Enables replication of a single PersistentVolumeClaim.

**API Version:** `replication.unified.io/v1alpha2`  
**Kind:** `VolumeReplication`  
**Short Names:** `vr`, `volrep`  
**Scope:** Namespaced

#### VolumeReplicationSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `volumeReplicationClass` | string | ✅ Yes | Name of the VolumeReplicationClass |
| `pvcName` | string | ✅ Yes | Name of the PVC to replicate |
| `replicationState` | enum | ✅ Yes | Desired state: `primary`, `secondary`, `resync` |
| `dataSource` | TypedLocalObjectReference | ❌ No | Optional data source for cloning |
| `autoResync` | bool | ❌ No | Auto-resync after connection recovery (default: false) |

#### VolumeReplicationStatus

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | []Condition | Standard Kubernetes conditions |
| `state` | string | Current replication state |
| `message` | string | Human-readable status message |
| `lastSyncTime` | Time | Last successful sync timestamp |
| `lastSyncDuration` | Duration | Duration of last sync |
| `observedGeneration` | int64 | Observed spec generation |

#### Example

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: database-replication
  namespace: production
spec:
  volumeReplicationClass: ceph-rbd-replication
  pvcName: database-pvc
  replicationState: primary
  autoResync: true
status:
  conditions:
  - type: Ready
    status: "True"
    reason: ReconcileComplete
    message: "Replication configured successfully"
  state: primary
  observedGeneration: 1
```

### VolumeReplicationClass

Defines how to replicate volumes (backend configuration).

**API Version:** `replication.unified.io/v1alpha2`  
**Kind:** `VolumeReplicationClass`  
**Short Names:** `vrc`, `volrepclass`  
**Scope:** Cluster

#### VolumeReplicationClassSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `provisioner` | string | ✅ Yes | CSI provisioner name (determines backend) |
| `parameters` | map[string]string | ❌ No | Backend-specific configuration |

#### Example - Ceph

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: ceph-rbd-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    mirroringMode: "snapshot"
    schedulingInterval: "5m"
    replication.storage.openshift.io/replication-secret-name: "rbd-secret"
    replication.storage.openshift.io/replication-secret-namespace: "rook-ceph"
```

#### Example - Trident

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: trident-san-replication
spec:
  provisioner: csi.trident.netapp.io
  parameters:
    replicationPolicy: "Async"
    replicationSchedule: "15m"
    remoteCluster: "dr-cluster"
    remoteSVM: "svm-dr"
    remoteVolume: "remote-volume-handle"
```

#### Example - Dell PowerStore

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: powerstore-replication
spec:
  provisioner: csi-powerstore.dellemc.com
  parameters:
    protectionPolicy: "15min-async"
    remoteSystem: "PS-DR-001"
    rpo: "15m"
```

---

## Volume Group Resources

### VolumeGroupReplication

Enables replication of multiple PVCs together as a crash-consistent group.

**API Version:** `replication.unified.io/v1alpha2`  
**Kind:** `VolumeGroupReplication`  
**Short Names:** `vgr`, `volgrouprep`  
**Scope:** Namespaced

#### VolumeGroupReplicationSpec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `volumeGroupReplicationClass` | string | ✅ Yes | Name of the VolumeGroupReplicationClass |
| `selector` | LabelSelector | ✅ Yes | Selects PVCs to replicate as a group |
| `replicationState` | enum | ✅ Yes | Desired state for all volumes: `primary`, `secondary`, `resync` |
| `autoResync` | bool | ❌ No | Auto-resync for the group (default: false) |
| `source` | TypedLocalObjectReference | ❌ No | Optional source group reference |

#### VolumeGroupReplicationStatus

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | []Condition | Group-level conditions |
| `state` | string | Current group state |
| `message` | string | Human-readable message |
| `lastSyncTime` | Time | Last successful group sync |
| `lastSyncDuration` | Duration | Duration of last group sync |
| `observedGeneration` | int64 | Observed spec generation |
| `persistentVolumeClaimsRefList` | []LocalObjectReference | List of PVCs in the group |

#### Example - PostgreSQL Database

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplication
metadata:
  name: postgresql-database-group
  namespace: production
spec:
  volumeGroupReplicationClass: ceph-rbd-group-replication
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
    message: "Group replication configured for 3 volumes"
  state: primary
  persistentVolumeClaimsRefList:
  - name: postgresql-data-pvc
  - name: postgresql-logs-pvc
  - name: postgresql-config-pvc
```

### VolumeGroupReplicationClass

Defines how to replicate volume groups.

**API Version:** `replication.unified.io/v1alpha2`  
**Kind:** `VolumeGroupReplicationClass`  
**Short Names:** `vgrc`, `volgrouprepclass`  
**Scope:** Cluster

#### VolumeGroupReplicationClassSpec

Same structure as VolumeReplicationClass but for groups.

#### Example - Ceph Group

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplicationClass
metadata:
  name: ceph-rbd-group-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    groupMirroringMode: "snapshot"
    schedulingInterval: "5m"
    groupConsistency: "crash"
    replication.storage.openshift.io/replication-secret-name: "rbd-secret"
    replication.storage.openshift.io/replication-secret-namespace: "rook-ceph"
```

---

## Replication States

### Valid States

| State | Meaning | Use When |
|-------|---------|----------|
| `primary` | Volume is the active source | This site is active, replicating to remote |
| `secondary` | Volume is a replica | This site receives replicated data from primary |
| `resync` | Force resynchronization | After split-brain, extended downtime, or manual resync |

### State Transitions

```
Initial → primary ✅
Initial → secondary ✅

primary ↔ secondary ✅ (promote/demote)
primary → resync ✅ (force resync)
secondary → resync ✅ (force resync)
resync → primary ✅ (after resync completes)
resync → secondary ✅ (after resync completes)
```

---

## Backend-Specific Parameters

### Ceph (rbd.csi.ceph.com)

**Common Parameters:**
- `mirroringMode`: `"snapshot"` or `"journal"` - RBD mirroring mode
- `schedulingInterval`: e.g., `"1m"`, `"5m"`, `"15m"` - Snapshot schedule
- `replication.storage.openshift.io/replication-secret-name`: Authentication secret
- `replication.storage.openshift.io/replication-secret-namespace`: Secret namespace

**Group Parameters:**
- `groupMirroringMode`: Mirroring mode for group
- `groupConsistency`: `"crash"` or `"application"` - Consistency level

**Translation:** None (Ceph uses kubernetes-csi-addons natively)

### Trident (csi.trident.netapp.io)

**Common Parameters:**
- `replicationPolicy`: `"Async"` or `"Sync"` - Replication mode
- `replicationSchedule`: e.g., `"15m"`, `"1h"` - Schedule for async
- `remoteCluster`: Remote cluster name
- `remoteSVM`: Remote Storage Virtual Machine name
- `remoteVolume`: Remote volume handle

**Group Parameters:**
- `consistencyGroupPolicy`: Consistency group policy name
- `groupReplicationSchedule`: Schedule for group

**Translation:**
| kubernetes-csi-addons | Trident State |
|-----------------------|---------------|
| `primary` | `established` |
| `secondary` | `reestablishing` |
| `resync` | `reestablishing` |

### Dell PowerStore (csi-powerstore.dellemc.com)

**Common Parameters (Required):**
- `protectionPolicy`: Protection policy name (e.g., `"15min-async"`)
- `remoteSystem`: Remote PowerStore system ID

**Common Parameters (Optional):**
- `rpo`: Recovery Point Objective (e.g., `"15m"`)
- `remoteClusterId`: Remote Kubernetes cluster ID

**Group Parameters:**
- `consistencyType`: `"Metro"` or `"Async"` - Consistency type
- `groupProtectionPolicy`: Group-level protection policy

**Translation:**
| kubernetes-csi-addons | Dell Action |
|-----------------------|-------------|
| `primary` | `Failover` |
| `secondary` | `Sync` |
| `resync` | `Reprotect` |

**Special Behavior:** Automatically labels PVCs with `replication.storage.dell.com/` labels for selector matching.

---

## Use Cases

### Single Volume Replication

**When to use:**
- Single-volume applications
- Independent volumes
- Simple replication scenarios

**Example:**
```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: my-replication-class
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    mirroringMode: "snapshot"
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: app-data-replication
  namespace: production
spec:
  volumeReplicationClass: my-replication-class
  pvcName: app-data-pvc
  replicationState: primary
  autoResync: true
EOF
```

### Volume Group Replication

**When to use:**
- Multi-volume databases (PostgreSQL, MySQL, MongoDB)
- Applications requiring crash consistency across volumes
- StatefulSets with related volumes
- Any application where partial state is unacceptable

**Example:**
```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplicationClass
metadata:
  name: db-group-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    groupMirroringMode: "snapshot"
    groupConsistency: "crash"
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplication
metadata:
  name: postgresql-group
  namespace: production
spec:
  volumeGroupReplicationClass: db-group-replication
  selector:
    matchLabels:
      app: postgresql
      instance: prod-01
  replicationState: primary
  autoResync: true
EOF
```

---

## Common Operations

### Promote Secondary to Primary (Failover)

```bash
# Update replication state
kubectl patch vr my-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'

# Verify
kubectl get vr my-replication -n production
```

**What happens:**
- Ceph: Volume promoted to primary
- Trident: State changes to "established"
- Dell: Action changes to "Failover"

### Demote Primary to Secondary (Failback)

```bash
kubectl patch vr my-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"secondary"}}'
```

### Force Resynchronization

```bash
kubectl patch vr my-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"resync"}}'

# After resync completes, set desired final state
kubectl patch vr my-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

### Delete Replication

```bash
kubectl delete vr my-replication -n production

# Backend resources are automatically cleaned up via owner references
```

---

## Status Conditions

### Condition Types

| Type | Status | Reason | Meaning |
|------|--------|--------|---------|
| `Ready` | `True` | `ReconcileComplete` | Replication configured successfully |
| `Ready` | `False` | `VolumeReplicationClassNotFound` | Referenced class doesn't exist |
| `Ready` | `False` | `PVCNotFound` | Referenced PVC doesn't exist |
| `Ready` | `False` | `UnknownBackend` | Cannot detect backend from provisioner |
| `Ready` | `False` | `ReconcileError` | Error during reconciliation |

### Checking Status

```bash
# List all replications
kubectl get vr --all-namespaces

# Get detailed status
kubectl describe vr my-replication -n production

# Check conditions
kubectl get vr my-replication -n production -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'
```

---

## Backend Detection

The operator automatically detects the backend from the `VolumeReplicationClass` provisioner field.

### Detection Rules

| Provisioner Contains | Detected Backend | Backend CR Created |
|---------------------|------------------|-------------------|
| `ceph`, `rbd.csi.ceph.com` | Ceph | `VolumeReplication` (replication.storage.openshift.io) |
| `trident`, `netapp` | Trident | `TridentMirrorRelationship` (trident.netapp.io) |
| `powerstore`, `dellemc` | Dell PowerStore | `DellCSIReplicationGroup` (replication.dell.com) |

### Verification

```bash
# Check backend resource created

# For Ceph
kubectl get volumereplication.replication.storage.openshift.io -n production

# For Trident
kubectl get tridentmirrorrelationship -n production

# For Dell
kubectl get dellcsireplicationgroup -n production
```

---

## Complete Examples

### Example 1: Ceph RBD Replication (Passthrough)

```yaml
apiVersion: replication.unified.io/v1alpha2
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
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: web-app-replication
  namespace: web
spec:
  volumeReplicationClass: ceph-snapshot-replication
  pvcName: web-app-data
  replicationState: primary
  autoResync: true
```

**Result:** Operator creates Ceph `VolumeReplication` with identical spec (passthrough).

### Example 2: Trident Async Replication (With Translation)

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: trident-async-replication
spec:
  provisioner: csi.trident.netapp.io
  parameters:
    replicationPolicy: "Async"
    replicationSchedule: "15m"
    remoteCluster: "dr-cluster"
    remoteSVM: "svm-dr"
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: app-data-replication
  namespace: apps
spec:
  volumeReplicationClass: trident-async-replication
  pvcName: app-data-pvc
  replicationState: primary  # Translated to state="established"
  autoResync: true
```

**Result:** Operator creates `TridentMirrorRelationship` with `state: established`.

### Example 3: Dell PowerStore Metro Replication

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: powerstore-metro
spec:
  provisioner: csi-powerstore.dellemc.com
  parameters:
    protectionPolicy: "metro-sync"
    remoteSystem: "PS-DR-001"
    consistencyType: "Metro"
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: critical-data-replication
  namespace: critical
spec:
  volumeReplicationClass: powerstore-metro
  pvcName: critical-data-pvc
  replicationState: primary  # Translated to action="Failover"
  autoResync: true
```

**Result:** Operator creates `DellCSIReplicationGroup` with `action: Failover` and labels PVC.

### Example 4: PostgreSQL Multi-Volume Group

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplicationClass
metadata:
  name: postgresql-group-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    groupMirroringMode: "snapshot"
    groupConsistency: "crash"
    schedulingInterval: "5m"
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplication
metadata:
  name: postgresql-prod-group
  namespace: databases
spec:
  volumeGroupReplicationClass: postgresql-group-replication
  selector:
    matchLabels:
      app: postgresql
      instance: prod-01
  replicationState: primary
  autoResync: true
```

**Required:** PVCs must have matching labels:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgresql-data-pvc
  namespace: databases
  labels:
    app: postgresql
    instance: prod-01
    component: data
spec:
  storageClassName: ceph-rbd
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
```

---

## Troubleshooting

### VolumeReplicationClass Not Found

**Symptom:**
```
Conditions:
  Ready: False
  Reason: VolumeReplicationClassNotFound
  Message: VolumeReplicationClass "my-class" not found
```

**Solution:**
```bash
# Check class exists
kubectl get vrc

# Create the class
kubectl apply -f volumereplicationclass.yaml
```

### Unknown Backend

**Symptom:**
```
Conditions:
  Ready: False
  Reason: UnknownBackend
  Message: unable to detect backend from provisioner: unknown.provisioner.io
```

**Solution:**
- Verify provisioner name in VolumeReplicationClass
- Supported: ceph, trident/netapp, powerstore/dellemc
- Check for typos in provisioner field

### No PVCs Match Selector (Volume Groups)

**Symptom:**
```
Conditions:
  Ready: False
  Reason: ReconcileError
  Message: no PVCs match selector in namespace production
```

**Solution:**
```bash
# Check PVC labels
kubectl get pvc -n production --show-labels

# Ensure PVCs have matching labels
kubectl label pvc my-pvc app=postgresql instance=prod-01 -n production
```

---

## kubectl Commands

### List Resources

```bash
# Single volume replications
kubectl get vr --all-namespaces
kubectl get volumereplication --all-namespaces  # long form

# Volume group replications
kubectl get vgr --all-namespaces
kubectl get volumegroupreplication --all-namespaces  # long form

# Classes (cluster-scoped)
kubectl get vrc
kubectl get vgrc

# Backend resources
kubectl get volumereplication.replication.storage.openshift.io -A  # Ceph
kubectl get tridentmirrorrelationship -A  # Trident
kubectl get dellcsireplicationgroup -A  # Dell
```

### Watch Resources

```bash
# Watch for changes
kubectl get vr -n production -w

# Watch with custom columns
kubectl get vr -n production -o custom-columns=\
NAME:.metadata.name,\
STATE:.spec.replicationState,\
PVC:.spec.pvcName,\
CLASS:.spec.volumeReplicationClass,\
READY:.status.conditions[?(@.type==\"Ready\")].status
```

### Debugging

```bash
# Get full resource YAML
kubectl get vr my-replication -n production -o yaml

# Check events
kubectl get events -n production --field-selector involvedObject.name=my-replication

# Check operator logs
kubectl logs -n unified-replication-system deployment/unified-replication-operator -f

# Check backend resource
kubectl describe volumereplication.replication.storage.openshift.io my-replication -n production
```

---

## Best Practices

### 1. Use VolumeReplicationClass for Shared Configuration

```yaml
# Define class once
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: production-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    mirroringMode: "snapshot"
    schedulingInterval: "5m"
---
# Use in multiple replications
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: app1-replication
spec:
  volumeReplicationClass: production-replication  # Shared!
  pvcName: app1-data
  replicationState: primary
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: app2-replication
spec:
  volumeReplicationClass: production-replication  # Shared!
  pvcName: app2-data
  replicationState: primary
```

### 2. Use Volume Groups for Multi-Volume Apps

For applications like databases with multiple volumes, use `VolumeGroupReplication` instead of multiple `VolumeReplication` resources to ensure crash consistency.

### 3. Set autoResync for Automatic Recovery

```yaml
spec:
  autoResync: true  # Automatically resync after connection recovery
```

### 4. Label PVCs for Volume Groups

```yaml
metadata:
  labels:
    app: my-app          # Application identifier
    instance: prod-01    # Instance identifier
    component: data      # Component identifier (optional)
```

### 5. Monitor Status Conditions

```bash
# Check ready status
kubectl get vr -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}'
```

---

## Reference

For complete kubernetes-csi-addons specification details, see:
- [CSI Addons Spec Reference](./CSI_ADDONS_SPEC_REFERENCE.md)
- [Migration Architecture](../architecture/MIGRATION_ARCHITECTURE.md)
- [Volume Group Replication Guide](../../VOLUME_GROUP_REPLICATION_ADDENDUM.md)

---

## Version Information

**Current API Version:** v1alpha2  
**Status:** Stable  
**kubernetes-csi-addons Compatibility:** 100%  
**Supported Backends:** Ceph, NetApp Trident, Dell PowerStore
