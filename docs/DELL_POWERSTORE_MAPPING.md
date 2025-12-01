# Dell PowerStore Mapping: CSI Addon Spec → Dell Spec

This document shows the complete mapping between the kubernetes-csi-addons compatible API (VolumeReplication) and Dell PowerStore's DellCSIReplicationGroup.

---

## Overview

**CSI Addon Resource:** `VolumeReplication` (replication.unified.io/v1alpha2)  
**Dell PowerStore Resource:** `DellCSIReplicationGroup` (replication.dell.com/v1)

---

## 1. Resource-Level Mapping

| CSI Addon Field | Dell PowerStore Field | Notes |
|----------------|----------------------|-------|
| `metadata.name` | `metadata.name` | Same name used |
| `metadata.namespace` | `metadata.namespace` | Same namespace |
| `spec.pvcName` | `spec.pvcSelector.matchLabels["replication.storage.dell.com/group"]` | PVC name → Label selector |
| `spec.volumeReplicationClass` | `spec.driverName` + `spec.protectionPolicy` + `spec.remoteSystem` | Class parameters extracted |

---

## 2. State Mapping

### CSI Addon States → Dell States/Actions

| CSI Addon State | Dell Action | Dell State | When Applied |
|----------------|-------------|------------|--------------|
| `primary` | `""` (no action) | Managed by Dell | Initial creation or steady state |
| `primary` | `"Failover"` | `"FailedOver"` | Only when transitioning from `secondary` → `primary` |
| `secondary` | `""` (no action) | Managed by Dell | Initial creation or steady state |
| `secondary` | `""` (no action) | `"Synchronized"` or `"Syncing"` | Dell manages via protection policy |
| `resync` | `"Reprotect"` | `"Syncing"` | When explicitly requesting resync |

### State Translation Logic

**Key Principle:** Dell PowerStore manages replication state primarily through:
1. **Protection Policy** - Defines replication behavior
2. **PVC Permissions** - Read-write (primary) vs read-only (secondary)
3. **Actions** - Only used for explicit operations (failover, reprotect)

**Action Determination Logic:**
```go
// Actions are ONLY set for explicit operations:
if desiredState == "resync" && currentState != "resync" {
    action = "Reprotect"
}
if existingDRG && currentState == "secondary" && desiredState == "primary" {
    action = "Failover"
}
// All other cases: no action (Dell manages via protection policy)
```

### Reverse Mapping: Dell Status → CSI Addon Status

| Dell Status | CSI Addon State | Notes |
|------------|-----------------|-------|
| `"Synchronized"` | `"secondary"` | Volume is synchronized (secondary) |
| `"Syncing"` | `"secondary"` | Volume is syncing (secondary) |
| `"FailedOver"` | `"primary"` | Volume has been failed over (primary) |
| Other/Unknown | `"secondary"` | Default fallback |

---

## 3. Parameter Mapping

### VolumeReplicationClass Parameters → DellCSIReplicationGroup Spec

| VolumeReplicationClass Parameter | DellCSIReplicationGroup Field | Required | Default | Notes |
|----------------------------------|-------------------------------|----------|---------|-------|
| `provisioner: "csi-powerstore.dellemc.com"` | `spec.driverName` | ✅ Yes | N/A | Hardcoded to `"csi-powerstore.dellemc.com"` |
| `parameters.protectionPolicy` | `spec.protectionPolicy` | ✅ Yes | N/A | Must match PowerStore array configuration |
| `parameters.remoteSystem` | `spec.remoteSystem` | ✅ Yes | N/A | PowerStore system ID |
| `parameters.rpo` | `spec.remoteRPO` | ❌ No | `"15m"` | Recovery Point Objective |
| `parameters.replicationMode` | `spec.consistencyType` | ❌ No | N/A | Translated: `synchronous` → `"Metro"`, `asynchronous` → `"Async"` |
| `parameters.remoteClusterId` | Not mapped | ❌ No | N/A | Informational only |
| `parameters.remoteNamespace` | Not mapped | ❌ No | N/A | Informational only |

### Mode Translation

| CSI Addon Mode | Dell PowerStore consistencyType | Notes |
|----------------|----------------------------------|-------|
| `"synchronous"` | `"Metro"` | Metro/active-active synchronous replication |
| `"asynchronous"` | `"Async"` | Asynchronous replication |
| Not specified | `""` (empty) | Uses default from protection policy |

---

## 4. Spec Field Mapping

### Complete DellCSIReplicationGroup Spec Structure

```yaml
apiVersion: replication.dell.com/v1
kind: DellCSIReplicationGroup
metadata:
  name: <VolumeReplication.name>
  namespace: <VolumeReplication.namespace>
  ownerReferences:
    - apiVersion: replication.unified.io/v1alpha2
      kind: VolumeReplication
      name: <VolumeReplication.name>
      controller: true
spec:
  driverName: "csi-powerstore.dellemc.com"  # Always this value
  protectionPolicy: <VolumeReplicationClass.parameters.protectionPolicy>
  remoteSystem: <VolumeReplicationClass.parameters.remoteSystem>
  remoteRPO: <VolumeReplicationClass.parameters.rpo>  # Default: "15m"
  consistencyType: <translated from replicationMode>  # Optional
  pvcSelector:
    matchLabels:
      replication.storage.dell.com/group: <VolumeReplication.name>
  action: <determined by state transition>  # Optional, only for explicit operations
```

### Field Details

#### `spec.driverName`
- **Source:** Hardcoded
- **Value:** `"csi-powerstore.dellemc.com"`
- **Required:** Yes

#### `spec.protectionPolicy`
- **Source:** `VolumeReplicationClass.spec.parameters["protectionPolicy"]`
- **Required:** Yes
- **Example:** `"15min-async"`, `"1hour-async"`, `"metro-policy"`

#### `spec.remoteSystem`
- **Source:** `VolumeReplicationClass.spec.parameters["remoteSystem"]`
- **Required:** Yes
- **Example:** `"PS-DR-001"`, `"powerstore-remote"`

#### `spec.remoteRPO`
- **Source:** `VolumeReplicationClass.spec.parameters["rpo"]`
- **Required:** No
- **Default:** `"15m"`
- **Format:** Duration string (e.g., `"15m"`, `"1h"`, `"4h"`)

#### `spec.consistencyType`
- **Source:** `VolumeReplicationClass.spec.parameters["replicationMode"]` (translated)
- **Required:** No
- **Values:** `"Metro"` (synchronous) or `"Async"` (asynchronous)
- **Translation:** Uses `PowerStoreModeMap` from translation engine

#### `spec.pvcSelector`
- **Source:** Derived from `VolumeReplication.spec.pvcName`
- **Structure:**
  ```yaml
  pvcSelector:
    matchLabels:
      replication.storage.dell.com/group: <VolumeReplication.name>
  ```
- **PVC Labeling:** Operator adds label `replication.storage.dell.com/group: <VolumeReplication.name>` to PVC

#### `spec.action`
- **Source:** Determined by `determineRequiredAction()` logic
- **Required:** No (only set for explicit operations)
- **Values:** `"Failover"`, `"Reprotect"`, or `""` (empty)
- **Logic:**
  - `"Reprotect"`: When `replicationState: "resync"` and not already resyncing
  - `"Failover"`: When transitioning from `secondary` → `primary` (existing CR)
  - `""`: All other cases (Dell manages via protection policy)

---

## 5. PVC Labeling

The operator automatically labels PVCs for Dell replication:

| Label Key | Label Value | Purpose |
|-----------|-------------|---------|
| `replication.storage.dell.com/replicated` | `"true"` | Marks PVC as replicated |
| `replication.storage.dell.com/group` | `<VolumeReplication.name>` | Groups PVCs for replication group |

**Labeling Logic:**
- Labels added when `VolumeReplication` is created
- Labels removed when `VolumeReplication` is deleted
- Labels used by `spec.pvcSelector` to select PVCs

---

## 6. Status Mapping

### DellCSIReplicationGroup Status → VolumeReplication Status

| Dell Status Field | VolumeReplication Status Field | Translation |
|-------------------|-------------------------------|-------------|
| `status.state` | `status.state` | Translated via `translateStateFromDell()` |
| `status.message` | `status.message` | Direct copy |
| `status.conditions` | `status.conditions` | Not directly mapped (VolumeReplication uses standard Conditions) |

### Status Translation Function

```go
func translateStateFromDell(dellState string) string {
    switch dellState {
    case "Synchronized", "Syncing":
        return "secondary"
    case "FailedOver":
        return "primary"
    default:
        return "secondary"  // Default fallback
    }
}
```

---

## 7. Volume Group Mapping

For `VolumeGroupReplication`, the mapping is similar but uses `pvcSelector` to select multiple PVCs:

| CSI Addon Field | Dell PowerStore Field | Notes |
|----------------|----------------------|-------|
| `spec.selector.matchLabels` | `spec.pvcSelector.matchLabels` | All PVCs matching selector are labeled |
| `spec.replicationState` | `spec.action` (if needed) | Same action logic as single volume |
| `spec.replicationMode` | `spec.consistencyType` | Same mode translation |

**Key Difference:**
- Single Volume: One PVC → One DellCSIReplicationGroup
- Volume Group: Multiple PVCs → One DellCSIReplicationGroup with `pvcSelector`

---

## 8. Example Mappings

### Example 1: Primary VolumeReplication

**CSI Addon Spec:**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: app-data-replication
  namespace: production
spec:
  volumeReplicationClass: powerstore-replication
  pvcName: app-data-pvc
  replicationState: primary
  autoResync: true
```

**VolumeReplicationClass:**
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

**Generated DellCSIReplicationGroup:**
```yaml
apiVersion: replication.dell.com/v1
kind: DellCSIReplicationGroup
metadata:
  name: app-data-replication
  namespace: production
  ownerReferences:
    - apiVersion: replication.unified.io/v1alpha2
      kind: VolumeReplication
      name: app-data-replication
      controller: true
spec:
  driverName: "csi-powerstore.dellemc.com"
  protectionPolicy: "15min-async"
  remoteSystem: "PS-DR-001"
  remoteRPO: "15m"
  pvcSelector:
    matchLabels:
      replication.storage.dell.com/group: app-data-replication
  # action: "" (not set - Dell manages via protection policy)
```

### Example 2: Secondary → Primary Transition (Failover)

**Before (Secondary):**
```yaml
spec:
  replicationState: secondary
```

**After (Primary - Failover):**
```yaml
spec:
  replicationState: primary
```

**DellCSIReplicationGroup Updated:**
```yaml
spec:
  # ... other fields unchanged ...
  action: "Failover"  # Set because transitioning from secondary → primary
```

### Example 3: Resync Operation

**CSI Addon Spec:**
```yaml
spec:
  replicationState: resync
```

**DellCSIReplicationGroup:**
```yaml
spec:
  # ... other fields unchanged ...
  action: "Reprotect"  # Set because resync requested
```

---

## 9. Key Design Decisions

### Why Actions Are Optional

1. **Dell Native Management:** Dell PowerStore manages replication state primarily through protection policies and PVC permissions, not through explicit actions.

2. **Actions for Explicit Operations Only:**
   - `Failover`: Only when explicitly promoting secondary to primary
   - `Reprotect`: Only when explicitly requesting resync
   - No action: For initial setup and steady state (Dell handles via protection policy)

3. **State vs Action:**
   - **State** (`primary`/`secondary`): Desired end state
   - **Action** (`Failover`/`Reprotect`): Explicit operation to trigger

### Why PVC Selector Instead of Direct PVC Reference

1. **Dell Native Support:** DellCSIReplicationGroup uses `pvcSelector` to select PVCs, which is the native Dell approach.

2. **Volume Group Support:** Same mechanism works for both single volumes and volume groups.

3. **Label-Based Selection:** More flexible and aligns with Dell's design.

---

## 10. Translation Code References

### Key Functions

| Function | Location | Purpose |
|----------|----------|---------|
| `ReconcileVolumeReplication()` | `pkg/adapters/powerstore_v1alpha2.go:60` | Main reconciliation logic |
| `determineRequiredAction()` | `pkg/adapters/powerstore_v1alpha2.go:225` | Determines if action is needed |
| `translateStateFromDell()` | `pkg/adapters/powerstore_v1alpha2.go:267` | Translates Dell status → CSI addon status |
| `determineConsistencyType()` | `pkg/adapters/powerstore_v1alpha2.go:347` | Translates replicationMode → consistencyType |
| `labelPVCForReplication()` | `pkg/adapters/powerstore_v1alpha2.go:279` | Adds Dell labels to PVC |

### Translation Maps

| Map | Location | Purpose |
|-----|----------|---------|
| `PowerStoreStateMap` | `pkg/translation/maps.go:47` | State translation map (not directly used - actions used instead) |
| `PowerStoreModeMap` | `pkg/translation/maps.go:76` | Mode translation map (`synchronous` → `Metro`, `asynchronous` → `Async`) |

---

## 11. Summary Table

| Aspect | CSI Addon | Dell PowerStore | Mapping Type |
|--------|-----------|----------------|--------------|
| **Resource** | `VolumeReplication` | `DellCSIReplicationGroup` | 1:1 |
| **State: primary** | `replicationState: "primary"` | No action (or `action: "Failover"` if transitioning) | Conditional |
| **State: secondary** | `replicationState: "secondary"` | No action | Direct |
| **State: resync** | `replicationState: "resync"` | `action: "Reprotect"` | Direct |
| **Mode: synchronous** | `replicationMode: "synchronous"` | `consistencyType: "Metro"` | Translated |
| **Mode: asynchronous** | `replicationMode: "asynchronous"` | `consistencyType: "Async"` | Translated |
| **PVC Reference** | `spec.pvcName` | `spec.pvcSelector.matchLabels` | Label-based |
| **Protection Policy** | `parameters.protectionPolicy` | `spec.protectionPolicy` | Direct |
| **Remote System** | `parameters.remoteSystem` | `spec.remoteSystem` | Direct |
| **RPO** | `parameters.rpo` | `spec.remoteRPO` | Direct |

---

## 12. Notes and Limitations

1. **Actions Are Transient:** Actions (`Failover`, `Reprotect`) are only set when needed and cleared by Dell operator after completion.

2. **State Management:** Dell manages replication state primarily through protection policies, not through explicit state fields in the CR.

3. **PVC Permissions:** Dell determines primary/secondary based on PVC read-write vs read-only permissions, not through CR state.

4. **Status Translation:** Status translation from Dell → CSI addon is approximate, as Dell states don't map 1:1 to CSI addon states.

5. **Mode Translation:** Mode translation only applies if `replicationMode` parameter is provided in VolumeReplicationClass.

---

*Last Updated: 2025-01-XX*  
*Based on: unified-replication-operator v2.0.0-beta*

