# v2.0.0-beta Demos

## Overview

This directory contains demos for the Unified Replication Operator v2.0.0-beta.

**New in v2.0.0-beta:** kubernetes-csi-addons compatible API with automatic multi-backend translation!

---

## Quick Start

### v2 Demo (kubernetes-csi-addons API) - **RECOMMENDED**

**Demonstrates:** Trident backend with automatic state translation

```bash
# Run interactive demo
./demo/run-v2-trident-demo.sh

# Or apply manually
kubectl apply -f demo/v2-trident-demo.yaml
```

**What It Shows:**
- ✅ kubernetes-csi-addons standard `VolumeReplication` API
- ✅ Simple 3-field spec (class, pvcName, state)
- ✅ Automatic backend detection from provisioner
- ✅ State translation: primary → established, secondary → reestablishing
- ✅ Backend CR creation (TridentMirrorRelationship)
- ✅ Clean lifecycle management

**Time:** 5-10 minutes

---

## Available Demos

### v2 Demos (kubernetes-csi-addons Compatible)

**1. Trident Translation Demo**
- **File:** `v2-trident-demo.yaml`
- **Script:** `run-v2-trident-demo.sh` (interactive)
- **Guide:** `V2_TRIDENT_DEMO_GUIDE.md`
- **Shows:** State translation in action
- **Backend:** NetApp Trident
- **API:** v1alpha2 (kubernetes-csi-addons)

**2. Sample YAMLs (All Backends)**
- **Location:** `../config/samples/`
- **Files:** 10 sample YAMLs
  - `volumereplicationclass_ceph.yaml`
  - `volumereplication_ceph_primary.yaml`
  - `volumereplicationclass_trident.yaml`
  - `volumereplication_trident_secondary.yaml`
  - `volumereplicationclass_powerstore.yaml`
  - `volumereplication_powerstore_primary.yaml`
  - `volumegroupreplicationclass_*.yaml` (3 files)
  - `volumegroupreplication_postgresql.yaml`

## Comparison

### v1alpha2 API (Current - kubernetes-csi-addons Compatible)

```yaml
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: trident-replication
spec:
  provisioner: csi.trident.netapp.io
  parameters: {replicationPolicy, schedule, etc.}
---
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
spec:
  volumeReplicationClass: trident-replication
  pvcName: my-data
  replicationState: primary
```

**Complexity:** Low (3 required fields, simple structure)

**Note:** v1alpha1 has been removed from the operator. All demos now use v1alpha2 API.

---

## Translation Table

### Trident Backend

| kubernetes-csi-addons (Input) | Trident (Output) | Verified |
|-------------------------------|------------------|----------|
| `primary` | `established` | ✅ |
| `secondary` | `reestablishing` | ✅ |
| `resync` | `reestablishing` | ✅ |

### Ceph Backend

| kubernetes-csi-addons (Input) | Ceph (Output) | Verified |
|-------------------------------|---------------|----------|
| `primary` | `primary` | ✅ |
| `secondary` | `secondary` | ✅ |
| `resync` | `resync` | ✅ |

*No translation needed - Ceph uses kubernetes-csi-addons natively!*

### Dell PowerStore Backend

| kubernetes-csi-addons (Input) | Dell (Output) | Verified |
|-------------------------------|---------------|----------|
| `primary` | `Failover` | ✅ |
| `secondary` | `Sync` | ✅ |
| `resync` | `Reprotect` | ✅ |

---

## Running the Demos

### Interactive v2 Demo (Recommended)

```bash
cd /path/to/unified-replication-operator
./demo/run-v2-trident-demo.sh
```

**Features:**
- Step-by-step progression
- Pauses between steps
- Shows translation in action
- Verifies backend resources
- Tests state transitions
- Clean cleanup

### Manual v2 Demo

```bash
# 1. Apply the demo YAML
kubectl apply -f demo/v2-trident-demo.yaml

# 2. Check VolumeReplication
kubectl get vr -n applications

# 3. Check backend TridentMirrorRelationship
kubectl get tridentmirrorrelationship -n applications

# 4. Verify translation
kubectl get vr trident-app-replication -n applications -o jsonpath='{.spec.replicationState}'
kubectl get tridentmirrorrelationship trident-app-replication -n applications -o jsonpath='{.spec.state}'

# Should show:
# Input: primary
# Output: established
```

### Use Sample YAMLs

```bash
# Ceph example
kubectl apply -f config/samples/volumereplicationclass_ceph.yaml
kubectl apply -f config/samples/volumereplication_ceph_primary.yaml

# Trident example
kubectl apply -f config/samples/volumereplicationclass_trident.yaml
kubectl apply -f config/samples/volumereplication_trident_secondary.yaml

# Dell example
kubectl apply -f config/samples/volumereplicationclass_powerstore.yaml
kubectl apply -f config/samples/volumereplication_powerstore_primary.yaml
```

---

## Documentation

**v2 Documentation:**
- **Quick Start:** `../QUICK_START.md`
- **API Reference:** `../docs/api-reference/API_REFERENCE.md`
- **Architecture:** `../docs/architecture/MIGRATION_ARCHITECTURE.md`

**General Documentation:**
- **Demo Guide:** `README.md`
- **Comprehensive Demo:** `COMPREHENSIVE_DEMO.md`
- **Backend Switching:** `test-backend-switching.sh`

---

## Which Demo Should I Use?

**All demos now use v1alpha2 API (kubernetes-csi-addons compatible):**
- ✅ Simple API (3 required fields)
- ✅ Standard state names (primary, secondary, resync)
- ✅ Compatible with kubernetes-csi-addons
- ✅ Works with all backends (Ceph, Trident, Dell PowerStore)

**Recommendation:** Use any demo - they all use v1alpha2!

---

## Files in This Directory

**Demo YAMLs (v1alpha2):**
- `v2-trident-demo.yaml` - Trident demo YAML (v1alpha2)
- `ceph-replication.yaml` - Ceph demo (v1alpha2)
- `trident-replication.yaml` - Trident demo (v1alpha2)
- `test-invalid-replication.yaml` - Validation demo (v1alpha2)

**Demo Scripts:**
- `run-demo.sh` - Comprehensive interactive demo
- `run-v2-trident-demo.sh` - Trident-specific demo script
- `test-backend-switching.sh` - Backend switching validation

**Documentation:**
- `V2_DEMOS_README.md` - This file
- `V2_TRIDENT_DEMO_GUIDE.md` - Complete Trident walkthrough
- `README.md` - General demo guide
- `COMPREHENSIVE_DEMO.md` - Detailed 4-part demo

**Note:** v1alpha1 has been completely removed. All demos and examples use v1alpha2.

---

## Support

**Issues?** See:
- `../QUICK_START.md` - Troubleshooting section
- `../docs/api-reference/API_REFERENCE.md` - Complete API docs
- `V2_TRIDENT_DEMO_GUIDE.md` - Detailed demo guide

**Questions about translation?**
- See `../docs/architecture/MIGRATION_ARCHITECTURE.md`
- Section: "Translation Strategy"

---

## Summary

**Recommended Demo:** `./demo/run-v2-trident-demo.sh`

This demonstrates:
- kubernetes-csi-addons standard API
- Automatic Trident translation
- State transitions
- Backend verification
- Clean lifecycle management

**Time:** 5-10 minutes  
**Difficulty:** Easy  
**Requirements:** Operator installed, kubectl access

**Happy replicating!** 🚀

