# Getting Started with Unified Replication Operator

## Overview

The Unified Replication Operator provides a unified API for managing storage replication across Ceph-CSI, NetApp Trident, and Dell PowerStore backends. This guide will help you get started quickly.

## Prerequisites

- Kubernetes cluster (1.24+)
- kubectl configured
- Helm 3.x installed
- At least one supported storage backend:
  - Ceph-CSI with RBD
  - NetApp Trident
  - Dell PowerStore CSI

## Quick Start

### 1. Install the Operator

```bash
# Using installation script
git clone https://github.com/unified-replication/operator
cd operator
./scripts/install.sh

# Or using Helm directly
helm install unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace
```

### 2. Verify Installation

```bash
# Check operator is running
kubectl get pods -n unified-replication-system

# Check CRDs are installed
kubectl get crd | grep unifiedvolumereplications

# Check webhook is configured
kubectl get validatingwebhookconfiguration
```

### 3. Create Your First Replication

```yaml
# my-first-replication.yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: my-first-replication
  namespace: default
spec:
  replicationState: replica
  replicationMode: asynchronous
  volumeMapping:
    source:
      pvcName: source-pvc
      namespace: default
    destination:
      volumeHandle: destination-volume-id
      namespace: default
  sourceEndpoint:
    cluster: source-cluster
    region: us-east-1
    storageClass: ceph-rbd  # Automatically detects Ceph backend
  destinationEndpoint:
    cluster: destination-cluster
    region: us-west-1
    storageClass: ceph-rbd
  schedule:
    mode: continuous
    rpo: "15m"
    rto: "5m"
```

```bash
kubectl apply -f my-first-replication.yaml
```

### 4. Check Status

```bash
# Get all replications
kubectl get unifiedvolumereplications -A

# Check specific replication
kubectl describe uvr my-first-replication -n default

# Watch status changes
kubectl get uvr my-first-replication -n default -w
```

## Basic Operations

### Create Replication

```bash
# From YAML
kubectl apply -f replication.yaml

# Using kubectl create (not recommended - use YAML)
```

### Update Replication

```yaml
# Edit replication.yaml - change replicationState to 'promoting'
spec:
  replicationState: promoting  # Changed from replica
```

```bash
kubectl apply -f replication.yaml
```

### Delete Replication

```bash
kubectl delete uvr my-first-replication -n default

# With cleanup wait
kubectl delete uvr my-first-replication -n default --wait
```

### Failover (Promote Replica to Source)

```yaml
spec:
  replicationState: promoting  # Step 1: Start promotion
```

Wait for promotion to complete, then:

```yaml
spec:
  replicationState: source  # Step 2: Confirm as source
```

### Failback (Demote Source to Replica)

```yaml
spec:
  replicationState: demoting  # Step 1: Start demotion
```

Wait for demotion to complete, then:

```yaml
spec:
  replicationState: replica  # Step 2: Confirm as replica
```

## Backend-Specific Examples

### Ceph-CSI

```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: ceph-replication
spec:
  replicationState: replica
  replicationMode: asynchronous
  sourceEndpoint:
    storageClass: ceph-rbd
  # ...
  extensions:
    ceph:
      mirroringMode: journal  # or snapshot
      schedulingInterval: "1m"
```

### NetApp Trident

```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: trident-replication
spec:
  replicationState: replica
  replicationMode: asynchronous
  sourceEndpoint:
    storageClass: trident-nas
  # ...
  extensions:
    trident:
      actions:
      - type: mirror-update
        snapshotHandle: snap-12345
```

### Dell PowerStore

```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: powerstore-replication
spec:
  replicationState: replica
  replicationMode: synchronous  # Metro replication
  sourceEndpoint:
    storageClass: powerstore-block
  # ...
  extensions:
    powerstore:
      rpoSettings: Five_Minutes
      volumeGroups:
      - production-vg
```

## Monitoring

### View Metrics

```bash
# Port-forward to metrics endpoint
kubectl port-forward -n unified-replication-system \
  svc/unified-replication-operator-metrics 8080:8080

# View metrics
curl http://localhost:8080/metrics
```

### Check Health

```bash
# Port-forward to health endpoint
kubectl port-forward -n unified-replication-system \
  deployment/unified-replication-operator 8081:8081

# Liveness
curl http://localhost:8081/healthz

# Readiness
curl http://localhost:8081/readyz
```

### View Logs

```bash
# Follow logs
kubectl logs -n unified-replication-system \
  -l control-plane=controller-manager -f

# View specific pod
kubectl logs -n unified-replication-system \
  pod/unified-replication-operator-xxx-yyy -f

# Filter by log level
kubectl logs -n unified-replication-system \
  -l control-plane=controller-manager | grep ERROR
```

## Troubleshooting

### Replication Stuck

```bash
# Check conditions
kubectl describe uvr my-replication -n default

# Check events
kubectl get events -n default --field-selector involvedObject.name=my-replication

# Check operator logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=100
```

### Webhook Issues

```bash
# Check webhook configuration
kubectl get validatingwebhookconfiguration

# Check webhook service
kubectl get svc -n unified-replication-system

# Test webhook
kubectl apply -f my-replication.yaml --dry-run=server
```

### Common Issues

**Issue:** Resource validation fails
**Solution:** Check `kubectl describe uvr` for validation errors

**Issue:** Backend not detected
**Solution:** Verify storage class name matches backend pattern

**Issue:** Pods not starting
**Solution:** Check `kubectl get events` and `kubectl logs`

## Next Steps

- Read the [User Guide](USER_GUIDE.md) for detailed information
- Check [Tutorials](../tutorials/) for step-by-step guides
- Review [API Reference](../api-reference/) for complete API docs
- See [Operations Guide](../operations/) for production operations

## Support

- Documentation: https://unified-replication.io/docs
- Issues: https://github.com/unified-replication/operator/issues
- Community: https://unified-replication.io/community

