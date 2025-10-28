# Unified Replication Operator

[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)]()
[![Go Report](https://img.shields.io/badge/go%20report-A+-brightgreen)]()
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)]()
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.24%2B-blue)]()
[![kubernetes-csi-addons](https://img.shields.io/badge/kubernetes--csi--addons-compatible-blue)]()

A Kubernetes operator that provides kubernetes-csi-addons compatible storage replication API with multi-backend translation support for Ceph, NetApp Trident, and Dell PowerStore.

**Key Value:** Use the standard kubernetes-csi-addons `VolumeReplication` API, and the operator automatically translates to Trident and Dell PowerStore backends!

## Features

- **kubernetes-csi-addons Compatible** - 100% compatible with kubernetes-csi-addons VolumeReplication API
- **Multi-Backend Translation** - Automatically translates to Ceph (passthrough), Trident, and Dell PowerStore
- **Volume Group Support** - Crash-consistent multi-volume replication for databases
- **Simple API** - Just 3 required fields (class, pvcName, state)
- **Automatic Backend Detection** - Detects backend from VolumeReplicationClass provisioner
- **State Translation** - Automatic translation for Trident and Dell backends
- **Standard Compliant** - Uses kubernetes-csi-addons standard (primary, secondary, resync states)
- **Production Ready** - Tested, documented, with comprehensive examples

## 🎬 **Live Demo**

Want to see it in action? Run the comprehensive demo:

```bash
cd demo
./run-demo.sh
```

Or read the step-by-step guide: **[demo/COMPREHENSIVE_DEMO.md](demo/COMPREHENSIVE_DEMO.md)**

See **[demo/README.md](demo/README.md)** for complete demo materials.

## Quick Start

```bash
# Install via Helm
helm install unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace

# Create a VolumeReplicationClass
cat <<EOF | kubectl apply -f -
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: ceph-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    mirroringMode: "snapshot"
    schedulingInterval: "5m"
EOF

# Create a VolumeReplication
cat <<EOF | kubectl apply -f -
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: my-replication
  namespace: default
spec:
  volumeReplicationClass: ceph-replication
  pvcName: my-data-pvc
  replicationState: primary
  autoResync: true
EOF

# Check status
kubectl get vr my-replication -n default
kubectl describe vr my-replication -n default
```

## Documentation

### **Getting Started**
- **[🎬 Comprehensive Demo](demo/COMPREHENSIVE_DEMO.md)** - ⭐ Complete walkthrough with all features
- **[Demo Materials](demo/)** - All demo scripts and examples
- **[Quick Start Guide](QUICK_START.md)** - Fast setup and validation
- **[Getting Started](docs/user-guide/GETTING_STARTED.md)** - Detailed guide

### **Installation**
- **[Build & Deploy Guide](BUILD_AND_DEPLOY.md)** - Build from source
- **[OpenShift Installation](OPENSHIFT_INSTALL.md)** - OpenShift-specific setup

### **Advanced Topics**
- **[Dell Workflow Comparison](docs/DELL_WORKFLOW_COMPARISON.md)** - Native Dell CSI vs Unified Operator
- **[Backend Switching Demo](demo/BACKEND_SWITCHING_DEMO.md)** - Multi-backend support
- **[Validation Guide](demo/VALIDATION_GUIDE.md)** - How to validate replications
- **[API Reference](docs/api-reference/API_REFERENCE.md)** - Full API specification
- **[Operations Guide](docs/operations/OPERATIONS_GUIDE.md)** - Production operations
- **[Tutorials](docs/tutorials/)** - Step-by-step guides
- **[Troubleshooting](docs/user-guide/TROUBLESHOOTING.md)** - Common issues and solutions

## Architecture

```
User
  ↓
VolumeReplication (kubernetes-csi-addons compatible)
  ↓
VolumeReplicationClass (backend config + detection)
  ↓
Controller (backend detection from provisioner)
  ↓
Adapter Registry
  ├→ Ceph Adapter (passthrough) → VolumeReplication CRD
  ├→ Trident Adapter (state translation) → TridentMirrorRelationship CRD
  └→ Dell Adapter (action translation) → DellCSIReplicationGroup CRD
```

**How It Works:**
1. User creates `VolumeReplication` with standard kubernetes-csi-addons API
2. Controller reads `VolumeReplicationClass` to detect backend
3. Appropriate adapter translates to backend-specific CR
4. Backend CR managed automatically with owner references

## Supported Backends

| Backend | CRD | Features | Status |
|---------|-----|----------|--------|
| **Ceph-CSI** | VolumeReplication | Journal/Snapshot mirroring | ✅ Production |
| **NetApp Trident** | TridentMirrorRelationship | Actions, volume groups | ✅ Production |
| **Dell PowerStore** | DellCSIReplicationGroup | Metro, RPO policies, pause/resume | ✅ Production |

## Requirements

- Kubernetes 1.24+
- Helm 3.8+
- At least one supported storage backend
- Network connectivity between clusters (for cross-cluster replication)

## Installation

### Using Helm

```bash
helm repo add unified-replication https://unified-replication.io/charts
helm install unified-replication-operator unified-replication/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace
```

### Using Installation Script

```bash
./scripts/install.sh
```

### Using Kustomize

```bash
# Development
kubectl apply -k config/overlays/development

# Production
kubectl apply -k config/overlays/production
```

## Configuration

### Basic Configuration

```yaml
# values.yaml
controller:
  maxConcurrentReconciles: 3
  enableAdvancedFeatures: true

backends:
  ceph: {enabled: true}
  trident: {enabled: true}
  powerstore: {enabled: true}
```

### Advanced Configuration

See [values.yaml](helm/unified-replication-operator/values.yaml) for all options.


## Development

### Build

```bash
# Build binary
make build

# Build container image
make docker-build

# Run tests
make test

# Run specific tests
go test -v ./pkg/adapters/...
go test -v ./controllers/...
```

### Testing

```bash
# Unit tests
go test -short ./...

# Integration tests (requires envtest)
make test-integration

# All tests
go test ./...

# Benchmark
go test -bench=. ./pkg/translation/...
```

#### Running Integration Tests

Integration tests require kubebuilder test binaries (envtest):

```bash
# The Makefile handles envtest setup automatically
make test-integration

# Or manually:
export KUBEBUILDER_ASSETS="$(./bin/setup-envtest use -p path)"
go test ./test/integration/... -v
```

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Security

- **RBAC:** Minimal permissions
- **Audit:** All operations logged
- **Pod Security:** Restricted profile
- **Network Policies:** Ingress/egress controls

See [Security Policy](config/security/SECURITY_POLICY.md) for details.

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

## Acknowledgments

Built with:
- [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder)
- [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)

---

**Maintained by:** Ohad Aharoni (written by AI)  
**Last Updated:** 2024-10-07
