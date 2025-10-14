# Demo Folder - Complete Contents

All demonstration materials are organized in this folder.

## 📁 **Folder Contents**

```
demo/
├── README.md                      ← Start here!
├── COMPREHENSIVE_DEMO.md          ← Main demo guide (4 parts)
├── DEMO_SUMMARY.md                ← Package overview
├── BACKEND_SWITCHING_DEMO.md      ← Backend switching details
├── VALIDATION_GUIDE.md            ← Validation reference
├── DOCUMENTATION_INDEX.md         ← Master doc index
├── DEMO_README.md                 ← Materials guide
│
├── run-demo.sh                    ← Interactive demo script
├── test-backend-switching.sh      ← Backend switching test
│
├── trident-replication.yaml       ← Trident example
└── ceph-replication.yaml          ← Ceph example
```

## 🚀 **Quick Start**

```bash
# From the demo folder:
./run-demo.sh

# Or from project root:
cd demo && ./run-demo.sh
```

## 📖 **Documentation Guide**

| File | Purpose | Read Time |
|------|---------|-----------|
| README.md | Quick overview | 2 min |
| COMPREHENSIVE_DEMO.md | Complete 4-part guide | 30 min |
| VALIDATION_GUIDE.md | Validation reference | 15 min |
| BACKEND_SWITCHING_DEMO.md | Architecture details | 10 min |
| DEMO_SUMMARY.md | Package summary | 5 min |

## 🎯 **What Each File Does**

### **Main Demo Guide**
`COMPREHENSIVE_DEMO.md` - The complete walkthrough covering:
- Part 1: Deploy operator
- Part 2: Create Trident CR, validate TridentMirrorRelationship
- Part 3: Update CR, verify Trident CR updates
- Part 4: Switch to Ceph, verify no restart

### **Interactive Script**
`run-demo.sh` - Automated demo that:
- Runs all 4 parts sequentially
- Pauses for explanation
- Shows commands and outputs
- Validates at each step

### **Backend Switching Test**
`test-backend-switching.sh` - Focused test that:
- Creates both Trident and Ceph replications
- Verifies no operator restart
- Shows adapter selection logs
- Displays summary

### **Validation Tools**
Located in `../scripts/`:
- `validate-replication.sh` - Comprehensive validation
- Run from anywhere: `../scripts/validate-replication.sh <name>`

## 📄 **Example Resources**

### **trident-replication.yaml**
- Backend: NetApp Trident
- Storage Class: `trident-ontap-san`
- State: `source`
- Creates: `TridentMirrorRelationship`

### **ceph-replication.yaml**
- Backend: Ceph-CSI
- Storage Class: `ceph-rbd`
- State: `replica`
- Creates: `VolumeReplication` (if CRDs installed)

## 🔗 **Related Documentation**

From project root:
- `README.md` - Main docs (updated to link here)
- `QUICK_START.md` - Quick setup
- `BUILD_AND_DEPLOY.md` - Build guide
- `OPENSHIFT_INSTALL.md` - OpenShift setup

## ✅ **Files Moved to This Folder**

The following files were organized into `demo/`:

**Documentation:**
- COMPREHENSIVE_DEMO.md
- DEMO_SUMMARY.md
- DEMO_README.md
- BACKEND_SWITCHING_DEMO.md
- VALIDATION_GUIDE.md
- DOCUMENTATION_INDEX.md

**Scripts:**
- run-demo.sh
- test-backend-switching.sh

**Examples:**
- trident-replication.yaml
- ceph-replication.yaml

---

**Start the demo:** `./run-demo.sh`
