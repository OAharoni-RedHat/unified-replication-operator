# Demo Files Overview

This directory contains comprehensive demonstration materials for the Unified Replication Operator.

## 🎬 **Interactive Demo**

**Run the complete demo:**
```bash
./run-demo.sh
```

This interactive script demonstrates all operator capabilities in sequence.

## 📚 **Demo Documentation**

### **Main Demo Guide**
- **[COMPREHENSIVE_DEMO.md](COMPREHENSIVE_DEMO.md)** - Complete step-by-step demo
  - Part 1: Deploy the operator
  - Part 2: Create Trident replication
  - Part 3: Update and verify propagation
  - Part 4: Switch to Ceph backend

### **Supporting Documentation**
- **[VALIDATION_GUIDE.md](VALIDATION_GUIDE.md)** - How to validate replications work
- **[BACKEND_SWITCHING_DEMO.md](BACKEND_SWITCHING_DEMO.md)** - Backend switching details
- **[INSTALLATION_COMPLETE.md](INSTALLATION_COMPLETE.md)** - Installation summary

## 🧪 **Demo Scripts**

### **Automated Scripts:**
```bash
# Complete demo (all 4 parts)
./run-demo.sh

# Backend switching test
./test-backend-switching.sh

# Validate specific replication
./scripts/validate-replication.sh <replication-name>

# Build and deploy operator
./scripts/build-and-push.sh
```

## 📄 **Example YAMLs**

### **Trident Replication:**
- **File:** `trident-replication.yaml`
- **Backend:** NetApp Trident
- **Storage Class:** `trident-ontap-san`
- **State:** `source`
- **Mode:** `asynchronous`

### **Ceph Replication:**
- **File:** `ceph-replication.yaml`
- **Backend:** Ceph-CSI
- **Storage Class:** `ceph-rbd`
- **State:** `replica`
- **Mode:** `asynchronous`

## 🎯 **What Each Demo Shows**

### **run-demo.sh** - Complete Interactive Demo
- Verifies operator deployment
- Creates Trident replication
- Shows CRD auto-creation
- Demonstrates update propagation
- Switches to Ceph backend
- Proves no operator restart needed

### **test-backend-switching.sh** - Backend Switching Focus
- Creates multiple backends
- Verifies no restart
- Shows logs for different adapters
- Compares detection logic

### **validate-replication.sh** - Detailed Validation
- Checks resource existence
- Validates Ready status
- Verifies backend detection
- Confirms CRD creation
- Checks finalizers
- Reviews events and logs

## 📊 **Demo Flow**

```
1. Deploy Operator
      ↓
2. Create Unified CR (Trident)
      ↓
3. Validate TridentMirrorRelationship created
      ↓
4. Update Unified CR
      ↓
5. Verify Trident CR updated automatically
      ↓
6. Create Unified CR (Ceph)
      ↓
7. Validate VolumeReplication created
      ↓
8. Confirm no operator restart occurred
      ↓
   ✅ Demo Complete
```

## 🚀 **Quick Demo (5 minutes)**

```bash
# 1. Deploy (if not already)
./scripts/build-and-push.sh

# 2. Create Trident replication
kubectl apply -f trident-replication.yaml
sleep 5
kubectl get uvr,tridentmirrorrelationship -n default

# 3. Update it
kubectl patch uvr trident-volume-replication -n default --type=merge \
  -p '{"spec":{"schedule":{"rpo":"10m"}}}'
sleep 15
kubectl get tridentmirrorrelationship trident-volume-replication -n default \
  -o jsonpath='{.spec.replicationSchedule}'

# 4. Add Ceph
kubectl apply -f ceph-replication.yaml
kubectl get uvr -n default

# Done!
```

## 📝 **Demo Checklist**

When running a demo, validate:

- [ ] Operator pod is Running (1/1)
- [ ] UnifiedVolumeReplication created
- [ ] TridentMirrorRelationship created (same name)
- [ ] States translated correctly (source → established)
- [ ] Modes translated correctly (asynchronous → Async)
- [ ] Updates propagate to backend CR
- [ ] Ceph backend detected
- [ ] No operator restart during switching
- [ ] Logs show different adapters used

## 🎓 **Learning Path**

1. **Start here:** Run `./run-demo.sh` to see everything in action
2. **Read:** `COMPREHENSIVE_DEMO.md` for detailed explanations
3. **Validate:** Use `./scripts/validate-replication.sh` to check your replications
4. **Deep dive:** Read `BACKEND_SWITCHING_DEMO.md` for architecture details

## 🔗 **Related Documentation**

- [README.md](README.md) - Main operator documentation
- [QUICK_START.md](QUICK_START.md) - Fast setup guide
- [BUILD_AND_DEPLOY.md](BUILD_AND_DEPLOY.md) - Build instructions
- [OPENSHIFT_INSTALL.md](OPENSHIFT_INSTALL.md) - OpenShift setup

---

**Ready to see the operator in action? Run:**
```bash
./run-demo.sh
```
