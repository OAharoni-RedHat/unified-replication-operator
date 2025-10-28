# Quick Start Guide

## Prerequisites

- Kubernetes 1.24+
- kubectl configured
- At least one supported storage backend:
  - Ceph-CSI with RBD
  - NetApp Trident
  - Dell PowerStore CSI

---

## Installation

### Option 1: Using Helm

```bash
# Add helm repository (if available)
helm repo add unified-replication https://unified-replication.io/charts
helm repo update

# Install operator
helm install unified-replication-operator \
  unified-replication/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace

# Or install from local chart
helm install unified-replication-operator \
  ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace
```

### Option 2: Using Kustomize

```bash
# Development
kubectl apply -k config/overlays/development

# Production
kubectl apply -k config/overlays/production
```

### Option 3: Using Installation Script

```bash
./scripts/install.sh
```

### Verify Installation

```bash
# Check operator is running
kubectl get pods -n unified-replication-system

# Check CRDs are installed
kubectl get crd | grep replication.unified.io
```

**Expected output:**
```
volumereplicationclasses.replication.unified.io
volumereplications.replication.unified.io
volumegroupreplicationclasses.replication.unified.io
volumegroupreplications.replication.unified.io
```

---

## Example 1: Single Volume Replication (Ceph)

### Step 1: Create Storage Class and PVC

```bash
# Create storage class (if not exists)
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-rbd
provisioner: rbd.csi.ceph.com
parameters:
  clusterID: rook-ceph
  pool: replicapool
  imageFormat: "2"
  imageFeatures: layering
EOF

# Create PVC
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-app-data
  namespace: production
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: ceph-rbd
EOF
```

### Step 2: Create VolumeReplicationClass

```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: ceph-rbd-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    mirroringMode: "snapshot"
    schedulingInterval: "5m"
    replication.storage.openshift.io/replication-secret-name: "rbd-secret"
    replication.storage.openshift.io/replication-secret-namespace: "rook-ceph"
EOF
```

### Step 3: Create VolumeReplication

```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: my-app-replication
  namespace: production
spec:
  volumeReplicationClass: ceph-rbd-replication
  pvcName: my-app-data
  replicationState: primary
  autoResync: true
EOF
```

### Step 4: Verify Replication

```bash
# Check VolumeReplication status
kubectl get vr -n production
kubectl describe vr my-app-replication -n production

# Check backend Ceph VolumeReplication created
kubectl get volumereplication.replication.storage.openshift.io -n production

# Expected output shows:
# - Ready: True
# - Backend CR created with same name
# - State: primary
```

---

## Example 2: Trident Replication (With Translation)

### Step 1: Create VolumeReplicationClass

```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: trident-san-replication
spec:
  provisioner: csi.trident.netapp.io
  parameters:
    replicationPolicy: "Async"
    replicationSchedule: "15m"
    remoteCluster: "dr-cluster"
    remoteSVM: "svm-dr"
    remoteVolume: "remote-volume-handle"
EOF
```

### Step 2: Create VolumeReplication

```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: trident-app-replication
  namespace: applications
spec:
  volumeReplicationClass: trident-san-replication
  pvcName: app-data-pvc
  replicationState: primary  # Will be translated to "established"
  autoResync: true
EOF
```

### Step 3: Verify Translation

```bash
# Check TridentMirrorRelationship created
kubectl get tridentmirrorrelationship -n applications

# Check spec shows translated state
kubectl get tridentmirrorrelationship trident-app-replication -n applications -o yaml | grep state:
# Should show: state: established
```

---

## Example 3: Dell PowerStore Replication

### Step 1: Create VolumeReplicationClass

```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplicationClass
metadata:
  name: powerstore-replication
spec:
  provisioner: csi-powerstore.dellemc.com
  parameters:
    protectionPolicy: "15min-async"
    remoteSystem: "PS-DR-001"
    rpo: "15m"
EOF
```

### Step 2: Create VolumeReplication

```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeReplication
metadata:
  name: critical-data-replication
  namespace: production
spec:
  volumeReplicationClass: powerstore-replication
  pvcName: critical-data-pvc
  replicationState: primary  # Will be translated to action="Failover"
  autoResync: true
EOF
```

### Step 3: Verify Dell Resources

```bash
# Check DellCSIReplicationGroup created
kubectl get dellcsireplicationgroup -n production

# Check PVC labels added
kubectl get pvc critical-data-pvc -n production --show-labels

# Should show labels:
# replication.storage.dell.com/replicated=true
# replication.storage.dell.com/group=critical-data-replication
```

---

## Example 4: Volume Group Replication (PostgreSQL)

### Step 1: Create PVCs with Labels

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgresql-data-pvc
  namespace: databases
  labels:
    app: postgresql
    instance: prod-01
    component: data
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 100Gi
  storageClassName: ceph-rbd
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgresql-logs-pvc
  namespace: databases
  labels:
    app: postgresql
    instance: prod-01
    component: logs
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 50Gi
  storageClassName: ceph-rbd
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgresql-config-pvc
  namespace: databases
  labels:
    app: postgresql
    instance: prod-01
    component: config
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 1Gi
  storageClassName: ceph-rbd
EOF
```

### Step 2: Create VolumeGroupReplicationClass

```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplicationClass
metadata:
  name: postgresql-group-replication
spec:
  provisioner: rbd.csi.ceph.com
  parameters:
    groupMirroringMode: "snapshot"
    groupConsistency: "crash"
    schedulingInterval: "5m"
EOF
```

### Step 3: Create VolumeGroupReplication

```bash
kubectl apply -f - <<EOF
apiVersion: replication.unified.io/v1alpha2
kind: VolumeGroupReplication
metadata:
  name: postgresql-prod-group
  namespace: databases
spec:
  volumeGroupReplicationClass: postgresql-group-replication
  selector:
    matchLabels:
      app: postgresql
      instance: prod-01
  replicationState: primary
  autoResync: true
EOF
```

### Step 4: Verify Group Replication

```bash
# Check VolumeGroupReplication
kubectl get vgr -n databases
kubectl describe vgr postgresql-prod-group -n databases

# Check which PVCs are in the group
kubectl get vgr postgresql-prod-group -n databases \
  -o jsonpath='{.status.persistentVolumeClaimsRefList[*].name}'

# Should output: postgresql-data-pvc postgresql-logs-pvc postgresql-config-pvc

# All 3 volumes are now replicated as a crash-consistent group!
```

---

## Common Operations

### Promote Secondary to Primary (Failover)

```bash
# Update state to primary
kubectl patch vr my-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'

# For volume groups
kubectl patch vgr my-group -n production \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

### Demote Primary to Secondary (Failback)

```bash
kubectl patch vr my-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"secondary"}}'
```

### Force Resynchronization

```bash
kubectl patch vr my-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"resync"}}'

# After resync, set to desired final state
kubectl patch vr my-replication -n production \
  --type merge \
  -p '{"spec":{"replicationState":"primary"}}'
```

### Delete Replication

```bash
# Delete VolumeReplication
kubectl delete vr my-replication -n production

# Backend resources are automatically cleaned up
# PVC remains intact
```

---

## Verification

### Check Status

```bash
# List all replications
kubectl get vr --all-namespaces

# Show with custom columns
kubectl get vr -A -o custom-columns=\
NAME:.metadata.name,\
NAMESPACE:.metadata.namespace,\
STATE:.spec.replicationState,\
PVC:.spec.pvcName,\
CLASS:.spec.volumeReplicationClass,\
READY:.status.conditions[?(@.type==\"Ready\")].status
```

### Check Backend Resources

```bash
# For Ceph backend
kubectl get volumereplication.replication.storage.openshift.io --all-namespaces

# For Trident backend
kubectl get tridentmirrorrelationship --all-namespaces

# For Dell backend
kubectl get dellcsireplicationgroup --all-namespaces
```

### Check Operator Logs

```bash
kubectl logs -n unified-replication-system \
  deployment/unified-replication-operator \
  --tail=100 \
  --follow
```

---

## Troubleshooting

### Issue: VolumeReplicationClass Not Found

**Symptom:**
```
Ready: False
Reason: VolumeReplicationClassNotFound
```

**Solution:**
```bash
# Check class exists
kubectl get vrc

# Create the class
kubectl apply -f volumereplicationclass.yaml
```

### Issue: Backend Not Detected

**Symptom:**
```
Ready: False
Reason: UnknownBackend
```

**Solution:**
- Verify provisioner in VolumeReplicationClass
- Supported: `ceph`, `trident`, `netapp`, `powerstore`, `dellemc`
- Check for typos

### Issue: PVC Not Found

**Symptom:**
```
Ready: False  
Reason: ReconcileError
Message: PVC "my-pvc" not found
```

**Solution:**
```bash
# Check PVC exists in same namespace
kubectl get pvc -n <namespace>

# Create PVC if missing
```

### Issue: No PVCs Match Selector (Volume Groups)

**Symptom:**
```
Ready: False
Reason: ReconcileError
Message: no PVCs match selector
```

**Solution:**
```bash
# Check PVC labels
kubectl get pvc -n <namespace> --show-labels

# Add labels to PVCs
kubectl label pvc my-pvc app=myapp instance=prod-01 -n <namespace>
```

---

## Next Steps

- **[API Reference](docs/api-reference/API_REFERENCE.md)** - Complete API documentation
- **[Examples](config/samples/)** - More example YAMLs
- **[Architecture](docs/architecture/MIGRATION_ARCHITECTURE.md)** - Detailed architecture
- **[Troubleshooting](docs/user-guide/TROUBLESHOOTING.md)** - Common issues

---

## Support

- **GitHub Issues:** [Report bugs or request features](https://github.com/unified-replication/operator/issues)
- **Documentation:** [Full documentation](docs/)
- **Examples:** [Sample configurations](config/samples/)
