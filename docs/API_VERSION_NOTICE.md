# API Version Notice

## Current API Status

**Primary API:** v1alpha2 (kubernetes-csi-addons compatible)  
**Legacy API:** v1alpha1 (can be removed - no production users)  
**Status Date:** October 28, 2024

---

## Recommended API: v1alpha2

### Why v1alpha2?

1. **kubernetes-csi-addons Compatible** - Standard API used by Ceph and other CSI drivers
2. **Simpler** - Only 3 required fields vs. 7+ in v1alpha1
3. **Better Separation** - VolumeReplicationClass separates "what" from "how"
4. **Volume Group Support** - Native support for multi-volume consistency
5. **Future-Proof** - Aligned with kubernetes-csi-addons ecosystem

### v1alpha2 Resources

**Single Volume:**
- `VolumeReplication` - Replicate a single PVC
- `VolumeReplicationClass` - Define how to replicate

**Volume Groups:**
- `VolumeGroupReplication` - Replicate multiple PVCs together
- `VolumeGroupReplicationClass` - Define how to replicate groups

**Example:**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: my-replication
spec:
  volumeReplicationClass: ceph-replication
  pvcName: my-data
  replicationState: primary  # Simple!
```

---

## Legacy API: v1alpha1

### Status: Optional (Can Be Removed)

Since this operator is not currently in production and has no active users, the v1alpha1 API can be safely removed without impact.

**v1alpha1 Resource:**
- `UnifiedVolumeReplication` - Complex multi-endpoint specification

**Characteristics:**
- ❌ Complex spec with source/destination endpoints
- ❌ More fields to configure
- ❌ Not kubernetes-csi-addons compatible
- ✅ Maintained for development continuity

### Removal Plan (If Desired)

**If you want to remove v1alpha1:**

1. **Remove from main.go:**
   ```go
   // Comment out or remove:
   // utilruntime.Must(replicationv1alpha1.AddToScheme(scheme))
   ```

2. **Remove controller setup:**
   ```go
   // Comment out UnifiedVolumeReplicationReconciler setup in main.go
   ```

3. **Remove CRD:**
   ```bash
   kubectl delete crd unifiedvolumereplications.replication.unified.io
   ```

4. **Clean up code:**
   - Remove `api/v1alpha1/` directory (optional - can keep for reference)
   - Remove `controllers/unifiedvolumereplication_controller.go` (optional)
   - Remove v1alpha1 adapter factories (optional)

**Benefit:** Simpler codebase, fewer CRDs, clearer focus on v1alpha2

**Drawback:** None (no users to impact)

---

## Recommendation

### Option A: Keep Both APIs (Current State)

**Pros:**
- ✅ Shows evolution of the project
- ✅ Reference implementation available
- ✅ No breaking changes if someone starts using v1alpha1

**Cons:**
- ⚠️ More CRDs to maintain
- ⚠️ Confusing which API to use
- ⚠️ Extra code to maintain

**When:** If uncertain about future needs

### Option B: Remove v1alpha1 (Recommended)

**Pros:**
- ✅ Cleaner codebase
- ✅ Single clear API
- ✅ Reduced maintenance
- ✅ No confusion for users

**Cons:**
- ❌ Lose development history
- ❌ Can't reference old implementation

**When:** Since no production users exist

### Decision

**For this operator:** 

Since there are no production users, **Option B (remove v1alpha1) is recommended** to:
- Keep codebase clean and focused
- Reduce confusion about which API to use
- Eliminate deprecated code before first users
- Simplify documentation

However, **keeping v1alpha1 for now** is also fine as it:
- Preserves development history
- Shows API evolution
- Can be removed anytime without user impact

---

## API Comparison

### v1alpha1 (Legacy)

```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
spec:
  sourceEndpoint: {...}
  destinationEndpoint: {...}
  volumeMapping: {...}
  replicationState: source  # Custom state names
  replicationMode: asynchronous
  schedule: {...}
  extensions: {...}
```

**Complexity:** High (7 top-level fields, nested structures)

### v1alpha2 (Recommended)

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
spec:
  volumeReplicationClass: my-class
  pvcName: my-pvc
  replicationState: primary  # Standard state names
```

**Complexity:** Low (3 required fields, simple structure)

---

## Migration (If Needed)

**Note:** Since there are no production users, migration is not required.

If you do have test/dev resources in v1alpha1 format:

### Manual Migration

1. **Identify backend** from `sourceEndpoint.storageClass`
2. **Create VolumeReplicationClass** with appropriate provisioner
3. **Create VolumeReplication** with:
   - `pvcName` from `volumeMapping.source.pvcName`
   - `replicationState`: `source` → `primary`, `replica` → `secondary`
4. **Delete old UnifiedVolumeReplication**

### Example

**Before (v1alpha1):**
```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: my-replication
spec:
  sourceEndpoint:
    storageClass: ceph-rbd
  volumeMapping:
    source:
      pvcName: my-data
  replicationState: source
  replicationMode: asynchronous
  schedule:
    rpo: "15m"
```

**After (v1alpha2):**
```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: ceph-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    schedulingInterval: "15m"  # from schedule.rpo
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: my-replication
spec:
  volumeReplicationClass: ceph-replication
  pvcName: my-data  # from volumeMapping.source.pvcName
  replicationState: primary  # source → primary
```

---

## Summary

- ✅ **Use v1alpha2** for all new deployments
- ✅ **v1alpha2 is kubernetes-csi-addons compatible**
- ✅ **v1alpha2 supports volume groups**
- ⚠️ **v1alpha1 can be removed** (no users)
- ℹ️ **Both APIs currently supported** (can coexist)

**For new users:** Start with v1alpha2 - it's simpler, standard-compliant, and the future of this operator.

