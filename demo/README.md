# Unified Replication Operator - Demo Materials

Complete demonstration package showcasing all operator capabilities.

---

## 🎬 **Quick Start**

Run the complete interactive demo:

```bash
cd demo
./run-demo.sh
```

Or read the comprehensive guide first:
```bash
cat COMPREHENSIVE_DEMO.md | less
```

---

## 📚 **What's in This Folder**

### **Main Demo Guide**
- **[COMPREHENSIVE_DEMO.md](COMPREHENSIVE_DEMO.md)** ⭐ - Complete 4-part walkthrough
  - Part 1: Deploy the operator
  - Part 2: Create Trident replication & validate
  - Part 3: Update CR and verify propagation
  - Part 4: Switch to Ceph backend seamlessly

### **Demo Scripts**
- **[run-demo.sh](run-demo.sh)** - Interactive demo (with pauses)
- **[test-backend-switching.sh](test-backend-switching.sh)** - Backend switching validation

### **Example Resources**
- **[trident-replication.yaml](trident-replication.yaml)** - Trident backend example
- **[ceph-replication.yaml](ceph-replication.yaml)** - Ceph backend example

### **Supporting Documentation**
- **[VALIDATION_GUIDE.md](VALIDATION_GUIDE.md)** - How to validate replications
- **[BACKEND_SWITCHING_DEMO.md](BACKEND_SWITCHING_DEMO.md)** - Multi-backend architecture
- **[DEMO_SUMMARY.md](DEMO_SUMMARY.md)** - Demo package overview
- **[DEMO_README.md](DEMO_README.md)** - This file (detailed guide)
- **[DOCUMENTATION_INDEX.md](DOCUMENTATION_INDEX.md)** - Master documentation index

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

Follow the steps in `COMPREHENSIVE_DEMO.md` manually.

---

## 📋 **Demo Parts Overview**

### **Part 1: Deploy the Operator**
- Verify operator is running
- Check pod status
- View operator logs

### **Part 2: Create Trident Replication**
- Apply `trident-replication.yaml`
- Validate UnifiedVolumeReplication created
- ⭐ Verify TridentMirrorRelationship auto-created
- Compare translation (source → established)

### **Part 3: Update and Verify Propagation**
- Update Unified CR (change RPO)
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
kubectl get uvr -n default

# Check backend-specific CRDs
kubectl get tridentmirrorrelationship -n default
kubectl get volumereplication -n default
```

---

## 📊 **Expected Results**

### **After Part 2 (Trident):**
```
NAME                         STATE    READY
trident-volume-replication   source   True  ✅

NAME (TridentMirrorRelationship)  DESIRED STATE   LOCAL PVC
trident-volume-replication         established     my-app-data  ✅
```

### **After Part 3 (Update):**
```
UnifiedVolumeReplication:
  spec.schedule.rpo: 10m  ← Updated

TridentMirrorRelationship:
  spec.replicationSchedule: 10m  ← Also updated! ✅
```

### **After Part 4 (Backend Switch):**
```
NAME                         BACKEND            READY
trident-volume-replication   trident-ontap-san  True   ✅
ceph-volume-replication      ceph-rbd           False  ⚠️

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
kubectl get uvr -n default
kubectl get tridentmirrorrelationship -n default

# Validate specific resource
.../scripts/validate-replication.sh trident-volume-replication

# View operator logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f

# Clean up after demo
kubectl delete uvr --all -n default
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

