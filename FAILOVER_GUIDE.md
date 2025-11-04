# Failover Guide - v2.0.0-beta

## Overview

This guide explains how to perform a failover (disaster recovery) using the Unified Replication Operator v2.0.0-beta, with detailed explanation of Dell PowerStore translation.

**Failover Definition:** Promoting a secondary (replica) site to primary (active) when the primary site fails or becomes unavailable.

---

## Quick Failover Steps

### Step 1: Verify Current State

```bash
# Check primary site
kubectl get vr my-replication -n production

# Should show:
# STATE: primary
```

### Step 2: Promote Secondary to Primary (Failover)

```bash
# On the secondary/DR site, change state to primary
kubectl patch vr my-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

### Step 3: Verify Failover

```bash
# Check new state
kubectl get vr my-replication -n production

# Should show:
# STATE: primary

# Check backend resource
kubectl describe vr my-replication -n production
```

**That's it!** The operator handles the backend-specific failover automatically.

---

## Detailed Failover Process

### Initial Setup (Before Disaster)

**Primary Site (Production):**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: critical-data-replication
  namespace: production
spec:
  volumeReplicationClass: powerstore-replication
  pvcName: critical-data-pvc
  replicationState: primary     # This site is active
  autoResync: true
```

**Secondary Site (DR):**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: critical-data-replication
  namespace: production
spec:
  volumeReplicationClass: powerstore-replication
  pvcName: critical-data-pvc-replica
  replicationState: secondary   # This site is standby
  autoResync: true
```

---

## Failover Scenarios

### Scenario 1: Unplanned Failover (Primary Site Down)

**Situation:** Primary site is unavailable (datacenter outage, network failure, etc.)

**Action:** Promote DR site

```bash
# On DR site/cluster
export KUBECONFIG=/path/to/dr-cluster-kubeconfig

# Promote to primary
kubectl patch vr critical-data-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

**What Happens:**

1. **Operator detects change** (state: secondary → primary)
2. **Backend detection** (from VolumeReplicationClass provisioner)
3. **Translation occurs** (based on backend)
4. **Backend resource updated**
5. **Volume becomes active** for application use

---

## Dell PowerStore Failover Translation

### How Dell PowerStore Failover Works

**Dell PowerStore Model:**
- Uses "replication groups" with protection policies
- Uses "actions" (not states) to control replication
- Volumes are in replication groups with protection relationships

### Translation: Primary → Failover

**User Input (kubernetes-csi-addons):**
```yaml
spec:
  replicationState: primary
```

**Operator Translation:**
1. **Detects:** Provisioner contains "powerstore" → Dell adapter
2. **Translates:** `primary` → `Failover` action
3. **Creates/Updates:** DellCSIReplicationGroup

**Backend Resource Created (DellCSIReplicationGroup):**
```yaml
apiVersion: replication.dell.com/v1
kind: DellCSIReplicationGroup
metadata:
  name: critical-data-replication
  namespace: production
  ownerReferences:
  - apiVersion: replication.unified.io/v1alpha2
    kind: VolumeReplication
    name: critical-data-replication
spec:
  driverName: csi-powerstore.dellemc.com
  action: Failover              # ← TRANSLATED from "primary"!
  protectionPolicy: "15min-async"
  remoteSystem: "PS-DR-001"
  remoteRPO: "15m"
  pvcSelector:
    matchLabels:
      replication.storage.dell.com/group: critical-data-replication
```

### What "Failover" Action Does in Dell PowerStore

**Dell PowerStore `Failover` Action:**
1. **Breaks** the replication relationship to the remote system
2. **Promotes** the local volumes to read-write (active)
3. **Allows** applications to access the volumes
4. **Makes** this site the new primary/active site

**Physical Storage Operations:**
- Volume at DR site becomes independent
- No longer receiving replication from (failed) primary
- Applications can now write to this volume
- Original primary relationship is severed

**Result:**
- ✅ DR site volumes are now active (read-write)
- ✅ Applications can fail over to DR site
- ✅ Data is accessible
- ✅ DR site is now the primary

---

## Complete Dell PowerStore Failover Example

### Initial State

**Primary Site (before disaster):**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
spec:
  volumeReplicationClass: powerstore-replication
  pvcName: database-pvc
  replicationState: primary
```

**Backend at Primary:**
```yaml
# DellCSIReplicationGroup
spec:
  action: Failover      # Site is primary
  protectionPolicy: "15min-async"
  remoteSystem: "PS-DR-001"
```

**Secondary Site (before disaster):**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
spec:
  volumeReplicationClass: powerstore-replication
  pvcName: database-pvc-replica
  replicationState: secondary
```

**Backend at Secondary:**
```yaml
# DellCSIReplicationGroup
spec:
  action: Sync          # TRANSLATED from "secondary"
  protectionPolicy: "15min-async"
  remoteSystem: "PS-PRIMARY-001"  # Points to primary
```

**What's Happening:**
- Primary site: `action: Failover` = active, replicating OUT
- Secondary site: `action: Sync` = syncing, receiving data IN
- Replication flowing: Primary → Secondary

---

### Disaster Occurs (Primary Site Fails)

**Primary Site:**
- 💥 Datacenter outage
- 💥 Network failure
- 💥 Storage system failure
- ❌ Unavailable

**Secondary Site:**
- ✅ Still running
- ✅ Has most recent replicated data (RPO: 15 minutes)
- ⏳ Waiting for failover command

---

### Failover Action

**On DR Site:**
```bash
# Access DR cluster
export KUBECONFIG=/path/to/dr-cluster-kubeconfig

# Promote VolumeReplication to primary
kubectl patch vr database-pvc-replica -n production \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

**Operator Actions:**

1. **Detects state change:** `secondary` → `primary`
2. **Reads VolumeReplicationClass:**
   - Provisioner: `csi-powerstore.dellemc.com`
   - Parameters: protectionPolicy, remoteSystem
3. **Selects Dell PowerStore adapter**
4. **Translates state:** `primary` → `Failover` action
5. **Updates DellCSIReplicationGroup:**

```yaml
spec:
  action: Failover      # ← Changed from "Sync" to "Failover"
  protectionPolicy: "15min-async"
  remoteSystem: "PS-PRIMARY-001"  # Still points to (failed) primary
```

**Dell PowerStore CSI Driver Actions:**

1. **Receives** DellCSIReplicationGroup with `action: Failover`
2. **Breaks** replication relationship to remote (failed) system
3. **Promotes** local volumes to read-write
4. **Makes** volumes independent and accessible
5. **Allows** applications to use the volumes

**Result:**
- ✅ DR site volumes are now primary (read-write)
- ✅ Applications can access data
- ✅ Failover complete!

---

### After Failover

**DR Site (now active):**
```yaml
# VolumeReplication
spec:
  replicationState: primary

# DellCSIReplicationGroup  
spec:
  action: Failover
  
# PVC
status:
  phase: Bound
  accessModes: [ReadWriteOnce]  # Can write!
```

**Application can now:**
- ✅ Read from PVC
- ✅ Write to PVC
- ✅ Continue operations at DR site

---

## Failback (Optional - After Primary Site Recovers)

### Step 1: Prepare Original Primary Site

**After primary site recovers:**

```bash
# On recovered primary site
export KUBECONFIG=/path/to/primary-cluster-kubeconfig

# Resync data from DR site (now the active primary)
kubectl patch vr database-pvc -n production \
  --type merge \
  -p '{"spec":{"replicationState":"resync"}}'
```

**Dell PowerStore Translation:**
```yaml
# DellCSIReplicationGroup at primary site
spec:
  action: Reprotect     # TRANSLATED from "resync"
  remoteSystem: "PS-DR-001"  # Points to DR (current primary)
```

**What Reprotect Does:**
- Re-establishes replication relationship
- Syncs data FROM DR site (current primary)
- Prepares for failback
- Volume remains read-only during sync

**Wait for resync to complete**, then:

### Step 2: Demote DR Site to Secondary

```bash
# On DR site (currently primary)
export KUBECONFIG=/path/to/dr-cluster-kubeconfig

kubectl patch vr database-pvc-replica -n production \
  --type merge \
  -p '{"spec":{"replicationState":"secondary"}}'
```

**Dell Translation:**
```yaml
spec:
  action: Sync          # TRANSLATED from "secondary"
  remoteSystem: "PS-PRIMARY-001"
```

### Step 3: Promote Primary Site

```bash
# On original primary site
export KUBECONFIG=/path/to/primary-cluster-kubeconfig

kubectl patch vr database-pvc -n production \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

**Dell Translation:**
```yaml
spec:
  action: Failover      # TRANSLATED from "primary"
  remoteSystem: "PS-DR-001"
```

**Result:**
- ✅ Failback complete
- ✅ Original site is primary again
- ✅ DR site is secondary again
- ✅ Replication restored: Primary → DR

---

## Dell PowerStore Action Details

### Action: `Failover`

**Purpose:** Make this site the active primary

**What Happens in PowerStore:**
1. Breaks replication relationship to remote
2. Promotes local volume group to read-write
3. Applications can access volumes
4. This site becomes independent

**When Used:**
- Disaster recovery failover
- Planned migration to DR site
- Testing DR capabilities

**Kubernetes-CSI-Addons State:** `primary`

---

### Action: `Sync`

**Purpose:** Keep this site synchronized as secondary

**What Happens in PowerStore:**
1. Maintains replication relationship to remote primary
2. Keeps local volumes read-only (or writable but replicating)
3. Receives replicated data from primary
4. Stays in sync with primary site

**When Used:**
- Normal DR configuration
- After failback to original primary
- Steady-state replication

**Kubernetes-CSI-Addons State:** `secondary`

---

### Action: `Reprotect`

**Purpose:** Re-establish replication after failover

**What Happens in PowerStore:**
1. Re-establishes replication relationship
2. Reverses replication direction if needed
3. Syncs data to get in sync with current primary
4. Prepares for normal replication or failback

**When Used:**
- After failover, before failback
- To reverse replication direction
- To resync after extended outage

**Kubernetes-CSI-Addons State:** `resync`

---

## Dell PowerStore Failover Timeline

### Timeline: Unplanned Failover

```
T-0:    Primary site operational
        Primary VR: state=primary → Dell: action=Failover
        DR VR: state=secondary → Dell: action=Sync
        
T+5min: Primary site fails (disaster!)
        Primary: ❌ Down
        DR: ✅ Still running with data (RPO: 15min old)
        
T+10min: Decision to failover
        
T+11min: Admin runs: kubectl patch vr ... replicationState=primary
        
T+11min: Operator translates
        Input: replicationState=primary
        Output: action=Failover
        
T+11min: Dell CSI Driver acts
        - Breaks replication to (failed) primary
        - Promotes volumes to read-write
        - Updates PVC status
        
T+12min: Application fails over
        - Pod scheduled on DR cluster
        - Mounts PVC (now read-write)
        - Application resumes operations
        
T+15min: Failover complete
        DR site is now primary
        Applications operational
```

---

## Translation for Different Backends

### Ceph Failover

**User Action:**
```yaml
spec:
  replicationState: primary
```

**Backend (Ceph VolumeReplication):**
```yaml
spec:
  replicationState: primary  # No translation - direct copy
```

**What Happens:**
- Ceph RBD image promoted to primary
- Mirror peer demoted to secondary
- Volume becomes writable

---

### Trident Failover

**User Action:**
```yaml
spec:
  replicationState: primary
```

**Backend (TridentMirrorRelationship):**
```yaml
spec:
  state: established  # TRANSLATED from "primary"
```

**What Happens:**
- SnapMirror relationship updated
- Source volume becomes active
- Destination volume becomes replica

---

### Dell PowerStore Failover

**User Action:**
```yaml
spec:
  replicationState: primary
```

**Backend (DellCSIReplicationGroup):**
```yaml
spec:
  action: Failover    # TRANSLATED from "primary"
  protectionPolicy: "15min-async"
  remoteSystem: "PS-REMOTE-001"
  pvcSelector:
    matchLabels:
      replication.storage.dell.com/group: my-replication
```

**What Happens in Dell PowerStore Hardware:**

1. **Protection Policy Evaluation**
   - PowerStore checks protection policy: "15min-async"
   - Determines last successful replication point
   - Validates data consistency

2. **Replication Relationship Break**
   - Severs replication link to remote system
   - Local volume group becomes independent
   - No longer receiving replication updates

3. **Volume Promotion**
   - Local volumes changed to read-write
   - I/O operations enabled
   - Storage metadata updated

4. **Group State Update**
   - Replication group status changes to "FailedOver"
   - All volumes in group promoted atomically
   - State reported back to Kubernetes

5. **PVC Availability**
   - PVCs become usable by applications
   - Pods can mount and access data
   - Applications can resume operations

**Dell PowerStore Physical Operations:**
- Internal consistency point created
- Volume mappings updated
- Host access permissions adjusted
- Storage controller state changed

---

## Dell PowerStore-Specific Considerations

### Protection Policies

**What They Are:**
- Pre-configured replication policies on PowerStore array
- Define RPO, sync schedule, retention
- Referenced in VolumeReplicationClass

**Example:**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: powerstore-replication
spec:
  provisioner: csi-powerstore.dellemc.com
  parameters:
    protectionPolicy: "15min-async"  # Must exist on PowerStore
    remoteSystem: "PS-DR-001"        # Must be configured
```

**During Failover:**
- PowerStore uses this policy to determine failover behavior
- Policy defines what happens during failover
- Policy determines data consistency guarantees

---

### PVC Labeling (Dell-Specific)

**The operator automatically labels PVCs for Dell:**

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: critical-data-pvc
  labels:
    replication.storage.dell.com/replicated: "true"
    replication.storage.dell.com/group: critical-data-replication
```

**Why:**
- Dell uses PVCSelector to identify volumes in replication group
- Labels connect PVCs to replication groups
- Selector matches labels to include volumes

**During Failover:**
- All PVCs with matching labels are failed over together
- Atomic group operation (all or nothing)
- Ensures consistency across multiple volumes

---

### Replication Group Behavior

**Dell PowerStore uses groups:**
```yaml
spec:
  pvcSelector:
    matchLabels:
      replication.storage.dell.com/group: my-replication
```

**During Failover (action: Failover):**
- ALL volumes in the group are failed over together
- Atomic operation (entire group succeeds or fails)
- No partial failover
- Ensures application consistency

**For Single Volume:**
- Group contains one PVC
- Still uses group semantics
- Same reliability guarantees

**For Volume Groups (VolumeGroupReplication):**
- Group contains multiple PVCs (e.g., database data + logs)
- All failed over atomically
- Perfect for multi-volume applications

---

## Step-by-Step Dell PowerStore Failover

### Detailed Example

**Initial Setup:**

```bash
# Primary site
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: powerstore-prod
spec:
  provisioner: csi-powerstore.dellemc.com
  parameters:
    protectionPolicy: "15min-async"
    remoteSystem: "PS-DR-001"
    rpo: "15m"
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: app-replication
  namespace: production
spec:
  volumeReplicationClass: powerstore-prod
  pvcName: app-data
  replicationState: primary
EOF
```

**What Operator Creates:**
```yaml
apiVersion: replication.dell.com/v1
kind: DellCSIReplicationGroup
metadata:
  name: app-replication
spec:
  action: Failover  # Primary site
  pvcSelector:
    matchLabels:
      replication.storage.dell.com/group: app-replication
```

**What Dell Does:**
- Primary volumes are read-write
- Replicating to remote system "PS-DR-001"
- Protection policy "15min-async" running

---

**DR Site Setup:**

```bash
# DR site
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: app-replication
  namespace: production
spec:
  volumeReplicationClass: powerstore-dr
  pvcName: app-data-replica
  replicationState: secondary
EOF
```

**What Operator Creates:**
```yaml
apiVersion: replication.dell.com/v1
kind: DellCSIReplicationGroup
metadata:
  name: app-replication
spec:
  action: Sync      # Secondary site - TRANSLATED!
  remoteSystem: "PS-PRIMARY-001"
```

**What Dell Does:**
- Secondary volumes are read-only (or synchronized)
- Receiving replication FROM primary
- Volumes stay in sync with primary

---

**Primary Site Fails:**

```
[Primary Site] ❌ DOWN
  |
  | Replication STOPPED
  ↓
[DR Site] ✅ Has data (15min RPO)
  → Need to activate!
```

---

**Perform Failover:**

```bash
# On DR site
kubectl patch vr app-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

**Operator Translation:**
1. Input: `replicationState: secondary` → `primary`
2. Adapter: Dell PowerStore adapter selected
3. Translation: `primary` → `Failover`
4. Update: DellCSIReplicationGroup

**Before:**
```yaml
spec:
  action: Sync
  remoteSystem: "PS-PRIMARY-001"
```

**After:**
```yaml
spec:
  action: Failover     # ← CHANGED!
  remoteSystem: "PS-PRIMARY-001"
```

---

**Dell PowerStore Executes Failover:**

**Internal PowerStore Operations:**
1. ✅ Receives action: Failover
2. ✅ Evaluates protection policy
3. ✅ Finds last consistent replication point
4. ✅ Breaks replication link to PS-PRIMARY-001
5. ✅ Promotes volumes to independent/active
6. ✅ Changes volumes to read-write
7. ✅ Updates volume metadata
8. ✅ Enables I/O operations
9. ✅ Reports status: "FailedOver"

**Kubernetes Side:**
1. ✅ PVCs become ready for use
2. ✅ Pods can mount PVCs
3. ✅ Applications can read/write
4. ✅ VolumeReplication status updated

---

**Verify Failover:**

```bash
# Check VolumeReplication
kubectl get vr app-replication -n production
# STATE: primary ✅

# Check DellCSIReplicationGroup
kubectl get dellcsireplicationgroup app-replication -n production -o yaml

# Should show:
# spec:
#   action: Failover
# status:
#   state: FailedOver  # Dell's status indicating failover complete
```

**Application Failover:**
```bash
# Start application on DR site
kubectl apply -f app-deployment.yaml -n production

# Application mounts PVC
# PVC is backed by failed-over PowerStore volume
# Application has access to data (15min RPO)
# Operations resume!
```

---

## Comparison: Failover Across Backends

### User Action (Same for All)

```yaml
kubectl patch vr my-replication \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

### Backend Actions

| Backend | Translation | Backend Action | Result |
|---------|-------------|----------------|--------|
| **Ceph** | None | `replicationState: primary` | RBD image promoted |
| **Trident** | State | `state: established` | SnapMirror source active |
| **Dell** | Action | `action: Failover` | Volume group failed over |

**All achieve the same goal:** Make this site's volumes active and writable.

---

## Atomic Group Failover (Dell PowerStore)

### For VolumeGroupReplication

**Example: Database with 3 volumes**

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplication
metadata:
  name: postgresql-group
  namespace: databases
spec:
  volumeGroupReplicationClass: powerstore-group
  selector:
    matchLabels:
      app: postgresql
      instance: prod-01
  replicationState: secondary  # DR site, standby
```

**Failover:**
```bash
kubectl patch vgr postgresql-group -n databases \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

**Dell Translation:**
```yaml
# DellCSIReplicationGroup
spec:
  action: Failover    # TRANSLATED from "primary"
  pvcSelector:
    matchLabels:
      replication.storage.dell.com/group: postgresql-group
```

**Result:**
- ✅ ALL 3 PVCs failed over TOGETHER (atomic)
  - postgresql-data-pvc
  - postgresql-logs-pvc
  - postgresql-config-pvc
- ✅ Crash-consistent group failover
- ✅ No partial state
- ✅ Database integrity maintained

**Why This Matters:**
- Database data and WAL logs must be consistent
- Partial failover would corrupt database
- Dell PowerStore ensures atomic group operations
- All volumes promoted together or none

---

## Troubleshooting Failover

### Issue: Failover Not Working

**Check VolumeReplication status:**
```bash
kubectl describe vr my-replication -n production
```

**Look for:**
```yaml
Status:
  Conditions:
  - Type: Ready
    Status: False
    Reason: ReconcileError
    Message: <error details>
```

**Common Issues:**

1. **Protection Policy Not Found**
   ```
   Message: protectionPolicy "15min-async" not found on PowerStore
   ```
   **Fix:** Create or specify correct policy name

2. **Remote System Not Configured**
   ```
   Message: remoteSystem "PS-DR-001" not configured
   ```
   **Fix:** Configure remote system in PowerStore

3. **Invalid Action**
   ```
   Message: action "failover" invalid (must be "Failover")
   ```
   **Fix:** Operator handles this - shouldn't see this error

---

### Issue: PVC Not Writable After Failover

**Check PVC labels:**
```bash
kubectl get pvc my-pvc -o yaml | grep -A 5 labels:
```

**Should have:**
```yaml
labels:
  replication.storage.dell.com/replicated: "true"
  replication.storage.dell.com/group: my-replication
```

**If missing:**
- Operator should add automatically
- Check operator logs for errors

---

## Summary

### How to Perform Failover

**One command:**
```bash
kubectl patch vr <name> -n <namespace> \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

**Works for:**
- ✅ Ceph (passthrough)
- ✅ Trident (translates to "established")
- ✅ Dell PowerStore (translates to "Failover")

---

### Dell PowerStore Failover Translation

**Input (kubernetes-csi-addons):**
```yaml
spec:
  replicationState: primary
```

**Translation:**
```
primary → Failover
```

**Dell PowerStore Actions:**
1. Breaks replication relationship
2. Promotes volumes to read-write
3. Makes site independent
4. Enables application access

**Result:**
- ✅ DR site becomes primary
- ✅ Volumes writable
- ✅ Applications can fail over
- ✅ Data accessible (within RPO)

---

### Key Points for Dell PowerStore

1. **Uses Actions** (not states) - different model
2. **Failover action** = promote to primary
3. **Sync action** = keep as secondary
4. **Reprotect action** = re-establish after failover
5. **Group operations** = atomic (all volumes together)
6. **PVC labeling** = automatic by operator
7. **Protection policies** = must exist on array

---

## Files Reference

**Translation Logic:**
- `pkg/adapters/powerstore_v1alpha2.go` - Dell translation implementation
- `pkg/adapters/trident_v1alpha2.go` - Trident translation implementation
- `pkg/adapters/ceph_v1alpha2.go` - Ceph passthrough implementation

**Tests:**
- `pkg/adapters/powerstore_v1alpha2_test.go` - Dell translation tests
- `pkg/adapters/trident_v1alpha2_test.go` - Trident translation tests

**Documentation:**
- `docs/api-reference/API_REFERENCE.md` - API reference
- `TRANSLATION_MAPS_GUIDE.md` - This document
- `demo/V2_TRIDENT_DEMO_GUIDE.md` - Trident demo

---

**Last Updated:** October 29, 2024  
**Verified:** All translations tested against actual backend APIs ✅


