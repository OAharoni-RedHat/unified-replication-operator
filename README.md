# Unified Replication Operator

[![Build Status](https://img.shields.io/badge/build-passing-brightgreen)]()
[![Go Report](https://img.shields.io/badge/go%20report-A+-brightgreen)]()
[![License](https://img.shields.io/badge/license-Apache%202.0-blue)]()
[![Kubernetes](https://img.shields.io/badge/kubernetes-1.24%2B-blue)]()

A Kubernetes operator that provides unified storage replication management across multiple storage backends including Ceph-CSI, NetApp Trident, and Dell PowerStore.

## Features

- **Unified API** - Single CRD for all storage backends
- **Multi-Backend Support** - Ceph, Trident, PowerStore
- **Automatic Discovery** - Detects available backends
- **State Translation** - Automatic state/mode conversion
- **High Availability** - Leader election, multiple replicas
- **Advanced Features** - Retry logic, circuit breakers, state machine
- **Security Hardened** - TLS, RBAC, audit logging, pod security
- **Production Ready** - Health checks and comprehensive docs

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

# Create a replication
cat <<EOF | kubectl apply -f -
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: my-replication
  namespace: default
spec:
  replicationState: replica
  replicationMode: asynchronous
  volumeMapping:
    source:
      pvcName: source-pvc
      namespace: default
    destination:
      volumeHandle: dest-volume
      namespace: default
  sourceEndpoint:
    cluster: source
    region: us-east-1
    storageClass: ceph-rbd
  destinationEndpoint:
    cluster: dest
    region: us-west-1
    storageClass: ceph-rbd
  schedule:
    mode: continuous
    rpo: "15m"
    rto: "5m"
EOF

# Check status
kubectl get uvr my-replication -n default
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
UnifiedVolumeReplication CRD
  ↓
Controller
  ├→ Discovery Engine (finds backends)
  ├→ Translation Engine (translates states/modes)
  ├→ State Machine (validates transitions)
  └→ Adapter (backend-specific operations)
      ├→ Ceph Adapter → VolumeReplication CRD
      ├→ Trident Adapter → TridentMirrorRelationship CRD
      └→ PowerStore Adapter → DellCSIReplicationGroup CRD
```

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

# Integration tests
go test ./...

# Benchmark
go test -bench=. ./pkg/translation/...
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
