# Documentation Index

Complete guide to all Unified Replication Operator documentation.

---

## 🎬 **Start Here: Demo Materials**

Perfect for first-time users and demonstrations:

| Document | Purpose | Duration |
|----------|---------|----------|
| **[COMPREHENSIVE_DEMO.md](COMPREHENSIVE_DEMO.md)** ⭐ | Complete 4-part walkthrough | 30 min read |
| **[run-demo.sh](run-demo.sh)** | Interactive demo script | 10 min run |
| **[DEMO_SUMMARY.md](DEMO_SUMMARY.md)** | Demo package overview | 5 min read |
| **[DEMO_README.md](DEMO_README.md)** | Demo materials guide | 5 min read |

**Quick start the demo:**
```bash
./run-demo.sh
```

---

## 📖 **Installation & Setup**

| Document | Purpose | Audience |
|----------|---------|----------|
| **[QUICK_START.md](QUICK_START.md)** | Fast setup and validation | All users |
| **[BUILD_AND_DEPLOY.md](BUILD_AND_DEPLOY.md)** | Build from source | Developers |
| **[OPENSHIFT_INSTALL.md](OPENSHIFT_INSTALL.md)** | OpenShift-specific setup | OpenShift users |
| **[INSTALLATION_COMPLETE.md](INSTALLATION_COMPLETE.md)** | Installation summary | Reference |

**Installation scripts:**
- `scripts/install-openshift.sh` - OpenShift installer
- `scripts/build-and-push.sh` - Build and deploy
- `scripts/create-webhook-cert.sh` - Manual cert creation

---

## ✅ **Validation & Testing**

| Document | Purpose | Use When |
|----------|---------|----------|
| **[VALIDATION_GUIDE.md](VALIDATION_GUIDE.md)** | How to validate replications | After creating CRs |
| **[BACKEND_SWITCHING_DEMO.md](BACKEND_SWITCHING_DEMO.md)** | Multi-backend architecture | Understanding design |

**Validation scripts:**
```bash
# Validate specific replication
../scripts/validate-replication.sh <name>

# Test backend switching
./test-backend-switching.sh
```

---

## 📚 **API & Architecture**

Original documentation (in `docs/` directory):

| Document | Location | Purpose |
|----------|----------|---------|
| **API Reference** | `docs/api-reference/API_REFERENCE.md` | Complete API spec |
| **Operations Guide** | `docs/operations/OPERATIONS_GUIDE.md` | Production operations |
| **Getting Started** | `docs/user-guide/GETTING_STARTED.md` | Detailed user guide |
| **Troubleshooting** | `docs/user-guide/TROUBLESHOOTING.md` | Common issues |
| **Failover Tutorial** | `docs/tutorials/FAILOVER_TUTORIAL.md` | Disaster recovery |

---

## 🛠️ **Developer Documentation**

| Document | Location | Purpose |
|----------|----------|---------|
| **Controller README** | `controllers/README.md` | Controller architecture |
| **Contributing Guide** | `CONTRIBUTING.md` | How to contribute |
| **Test Documentation** | `test/README.md` | Testing guide |

---

## 📄 **Example Resources**

| File | Backend | Description |
|------|---------|-------------|
| `trident-replication.yaml` | Trident | NetApp Trident replication example |
| `ceph-replication.yaml` | Ceph | Ceph-CSI replication example |
| `config/samples/replication_v1alpha1_unifiedvolumereplication.yaml` | Generic | Basic sample |

---

## 🔧 **Scripts & Tools**

### **Installation:**
- `scripts/install.sh` - Standard installation
- `scripts/install-openshift.sh` - OpenShift installation
- `scripts/upgrade.sh` - Upgrade operator
- `scripts/uninstall.sh` - Uninstall operator

### **Build & Deploy:**
- `scripts/build-and-push.sh` - Build and deploy to registry
- `scripts/create-webhook-cert.sh` - Generate webhook certificates

### **Validation & Testing:**
- `scripts/validate-replication.sh` - Validate replication resources
- `test-backend-switching.sh` - Backend switching test
- `run-demo.sh` - Complete interactive demo

### **Utilities:**
- `scripts/diagnostics.sh` - Diagnostic information
- `scripts/test-helm-chart.sh` - Test Helm chart

---

## 📊 **Documentation by Use Case**

### **"I want to install the operator"**
1. [QUICK_START.md](QUICK_START.md)
2. [BUILD_AND_DEPLOY.md](BUILD_AND_DEPLOY.md)
3. [OPENSHIFT_INSTALL.md](OPENSHIFT_INSTALL.md) (if on OpenShift)

### **"I want to see it working"**
1. Run `./run-demo.sh`
2. Read [COMPREHENSIVE_DEMO.md](COMPREHENSIVE_DEMO.md)

### **"I want to create a replication"**
1. Copy `trident-replication.yaml` or `ceph-replication.yaml`
2. Modify for your needs
3. `kubectl apply -f your-replication.yaml`
4. Validate with `../scripts/validate-replication.sh`

### **"I want to understand backend switching"**
1. [BACKEND_SWITCHING_DEMO.md](BACKEND_SWITCHING_DEMO.md)
2. Run `./test-backend-switching.sh`

### **"I want to validate my setup"**
1. [VALIDATION_GUIDE.md](VALIDATION_GUIDE.md)
2. Run `../scripts/validate-replication.sh <name>`

### **"I want to troubleshoot"**
1. [VALIDATION_GUIDE.md](VALIDATION_GUIDE.md) - Common Issues section
2. [docs/user-guide/TROUBLESHOOTING.md](docs/user-guide/TROUBLESHOOTING.md)
3. Check operator logs

---

## 🎯 **Recommended Reading Order**

### **For New Users:**
1. README.md (this file)
2. QUICK_START.md
3. COMPREHENSIVE_DEMO.md
4. Run `./run-demo.sh`
5. VALIDATION_GUIDE.md

### **For Operators/SREs:**
1. OPENSHIFT_INSTALL.md (if OpenShift)
2. BUILD_AND_DEPLOY.md
3. docs/operations/OPERATIONS_GUIDE.md
4. docs/user-guide/TROUBLESHOOTING.md

### **For Developers:**
1. CONTRIBUTING.md
2. controllers/README.md
3. API Reference
4. test/README.md

---

## 📦 **Complete File List**

### **Demo & Validation:**
- COMPREHENSIVE_DEMO.md ⭐
- DEMO_SUMMARY.md
- DEMO_README.md
- VALIDATION_GUIDE.md
- BACKEND_SWITCHING_DEMO.md
- INSTALLATION_COMPLETE.md

### **Setup & Installation:**
- QUICK_START.md
- BUILD_AND_DEPLOY.md
- OPENSHIFT_INSTALL.md

### **Scripts:**
- run-demo.sh
- test-backend-switching.sh
- scripts/validate-replication.sh
- scripts/build-and-push.sh
- scripts/install-openshift.sh
- scripts/create-webhook-cert.sh

### **Examples:**
- trident-replication.yaml
- ceph-replication.yaml
- values-openshift.yaml
- openshift-scc.yaml

---

## 🔗 **External Resources**

- **Kubebuilder:** https://book.kubebuilder.io/
- **controller-runtime:** https://github.com/kubernetes-sigs/controller-runtime
- **Ceph-CSI:** https://github.com/ceph/ceph-csi
- **NetApp Trident:** https://docs.netapp.com/us-en/trident/
- **Dell PowerStore CSI:** https://github.com/dell/csi-powerstore

---

## ✨ **Quick Links**

| What do you want to do? | Go to |
|--------------------------|-------|
| **See demo** | Run `./run-demo.sh` |
| **Install operator** | [QUICK_START.md](QUICK_START.md) |
| **Build from source** | [BUILD_AND_DEPLOY.md](BUILD_AND_DEPLOY.md) |
| **Validate replication** | Run `../scripts/validate-replication.sh` |
| **Troubleshoot** | [VALIDATION_GUIDE.md](VALIDATION_GUIDE.md) |
| **Understand architecture** | [BACKEND_SWITCHING_DEMO.md](BACKEND_SWITCHING_DEMO.md) |

---

*Last Updated: 2025-10-14*  
*Operator Version: 0.2.1*
