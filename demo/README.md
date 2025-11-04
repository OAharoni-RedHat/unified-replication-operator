# Unified Replication Operator - Demo Materials

Complete demonstration package showcasing all operator capabilities.

---

## 🎬 **Quick Start**

Run the complete interactive demo:

```bash
cd demo
./run-demo.sh
```

---

## 📚 **What's in This Folder**

### **Main Demo Guides**
- **[V2_DEMOS_README.md](V2_DEMOS_README.md)** ⭐ - Complete guide to v1alpha2 demos
- **[V2_TRIDENT_DEMO_GUIDE.md](V2_TRIDENT_DEMO_GUIDE.md)** - Detailed Trident demo walkthrough
- **[DEPRECATION_NOTICE.md](DEPRECATION_NOTICE.md)** - Migration guide from v1alpha1 (removed)

### **Demo Scripts**
- **[run-demo.sh](run-demo.sh)** - Interactive 4-part demo (with pauses)
- **[run-v2-trident-demo.sh](run-v2-trident-demo.sh)** - Trident-specific demo script
- **[test-backend-switching.sh](test-backend-switching.sh)** - Backend switching validation

### **Example Resources (v1alpha2)**
- **[v2-trident-demo.yaml](v2-trident-demo.yaml)** - Complete Trident demo example
- **[trident-replication.yaml](trident-replication.yaml)** - Trident backend example
- **[ceph-replication.yaml](ceph-replication.yaml)** - Ceph backend example
- **[test-invalid-replication.yaml](test-invalid-replication.yaml)** - Invalid resource for testing validation

---

## 🚀 **Running the Demo**

### **Option 1: Interactive Demo (Recommended)**

```bash
cd demo
./run-demo.sh
```

**Features:**
- Pauses between steps
- Explains each action
- Shows expected outputs
- Validates results

**Duration:** ~10 minutes

### **Option 2: Quick Backend Switching Test**

```bash
cd demo
./test-backend-switching.sh
```

**Features:**
- Automated (no pauses)
- Focuses on backend switching
- Quick validation

**Duration:** ~2 minutes

### **Option 3: Manual Step-by-Step**

Follow the steps in `V2_DEMOS_README.md` or `V2_TRIDENT_DEMO_GUIDE.md` manually.

---

## 📋 **Demo Parts Overview**

### **Part 1: Deploy the Operator**
- Verify operator is running
- Check pod status
- View operator logs

### **Part 2: Create Trident Replication**
- Apply `trident-replication.yaml` (VolumeReplicationClass + VolumeReplication)
- Validate VolumeReplication created (v1alpha2 API)
- ⭐ Verify TridentMirrorRelationship auto-created
- Compare translation (primary → established)

### **Part 3: Update and Verify Propagation**
- Update VolumeReplication (change replicationState)
- Wait for reconciliation
- ⭐ Verify Trident CR updated automatically
- Prove bidirectional sync

### **Part 4: Switch to Ceph Backend**
- Apply `ceph-replication.yaml`
- Verify both backends running
- ⭐ Confirm no operator restart
- Show different adapters used

---

## ✅ **Validation**

After running the demo, validate with:

```bash
# Validate Trident replication
.../scripts/validate-replication.sh trident-volume-replication

# Check both replications
kubectl get vr -n default

# Check backend-specific CRDs
kubectl get tridentmirrorrelationship -n default
kubectl get volumereplication.replication.storage.openshift.io -n default
```

---

## 📊 **Expected Results**

### **After Part 2 (Trident):**
```
NAME                         STATE     PVC           READY
trident-volume-replication   primary   my-app-data   True  ✅

NAME (TridentMirrorRelationship)  DESIRED STATE   LOCAL PVC
trident-volume-replication         established     my-app-data  ✅
```

### **After Part 3 (Update):**
```
VolumeReplication:
  spec.replicationState: secondary  ← Updated

TridentMirrorRelationship:
  spec.state: reestablished  ← Also updated! ✅
```

### **After Part 4 (Backend Switch):**
```
NAME                         CLASS                    STATE     READY
trident-volume-replication   trident-async-replication primary   True   ✅
ceph-volume-replication      ceph-rbd-replication     primary   True   ✅

Operator Restarts: 0  ← NO RESTART! ✅
```

---

## 🎯 **Quick Commands**

```bash
# Run full demo
cd demo && ./run-demo.sh

# Quick validation
cd demo && ./test-backend-switching.sh

# Check current state
kubectl get vr -n default
kubectl get vrc -n default
kubectl get tridentmirrorrelationship -n default

# Validate specific resource
.../scripts/validate-replication.sh trident-volume-replication

# View operator logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f

# Clean up after demo
kubectl delete vr --all -n default
kubectl delete vrc --all -n default
```

---

## 📖 **Related Documentation**

- **[../README.md](../README.md)** - Main operator documentation
- **[../QUICK_START.md](../QUICK_START.md)** - Quick setup guide
- **[../BUILD_AND_DEPLOY.md](../BUILD_AND_DEPLOY.md)** - Build instructions
- **[../OPENSHIFT_INSTALL.md](../OPENSHIFT_INSTALL.md)** - OpenShift setup

---

## 🎓 **Learning Path**

1. **Start:** Run `./run-demo.sh`
2. **Read:** `COMPREHENSIVE_DEMO.md`
3. **Validate:** Use `.../scripts/validate-replication.sh`
4. **Understand:** Read `BACKEND_SWITCHING_DEMO.md`
5. **Reference:** Use `VALIDATION_GUIDE.md` as needed

---

## 🎉 **Ready to Demo!**

Your comprehensive demo package includes:
- ✅ Complete documentation
- ✅ Interactive scripts
- ✅ Example resources
- ✅ Validation tools

**Start the demo:**
```bash
./run-demo.sh
```

---

*Demo Package Version: 1.0*  
*Operator Version: 0.2.1*  
*Last Updated: 2025-10-14*

