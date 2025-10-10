# API Reference

## UnifiedVolumeReplication API

### API Group
`replication.unified.io/v1alpha1`

### Resource
`UnifiedVolumeReplication` (short name: `uvr`)

---

## Spec

### ReplicationState

**Type:** `enum`  
**Required:** Yes  
**Values:**
- `source` - Active/primary volume
- `replica` - Passive/secondary volume
- `promoting` - Transitioning replica → source (failover in progress)
- `demoting` - Transitioning source → replica (failback in progress)
- `syncing` - Synchronization in progress
- `failed` - Replication failed

**State Transitions:**
```
replica → promoting → source (Failover)
source → demoting → replica (Failback)
replica → syncing → replica (Resync)
failed → syncing → replica (Recovery)
```

### ReplicationMode

**Type:** `enum`  
**Required:** Yes  
**Values:**
- `synchronous` - Real-time replication (RPO ~0)
- `asynchronous` - Scheduled replication (RPO based on schedule)

### VolumeMapping

**Type:** `object`  
**Required:** Yes

**Fields:**
- `source` (VolumeSource) - Source volume information
  - `pvcName` (string, required) - PVC name
  - `namespace` (string, required) - PVC namespace
- `destination` (VolumeDestination) - Destination volume information
  - `volumeHandle` (string, required) - Backend volume ID
  - `namespace` (string, required) - Destination namespace

### Endpoints

**SourceEndpoint, DestinationEndpoint**

**Type:** `object`  
**Required:** Yes

**Fields:**
- `cluster` (string, required) - Cluster identifier
- `region` (string, required) - Region/availability zone
- `storageClass` (string, required) - Storage class name

### Schedule

**Type:** `object`  
**Required:** Yes

**Fields:**
- `mode` (enum, required) - `continuous` or `scheduled`
- `rpo` (string, optional) - Recovery Point Objective (e.g., "15m", "1h")
- `rto` (string, optional) - Recovery Time Objective (e.g., "5m", "30m")

**Format:** `<number><unit>` where unit is `s`, `m`, `h`, or `d`

### Extensions

**Type:** `object`  
**Optional:** Yes

**Backend-Specific Extensions:**

#### Ceph Extensions
```yaml
extensions:
  ceph:
    mirroringMode: journal|snapshot
    schedulingInterval: "1m"
    autoResync: true
```

#### Trident Extensions
```yaml
extensions:
  trident: {}  # Reserved for future Trident-specific settings
```

#### PowerStore Extensions
```yaml
extensions:
  powerstore: {}  # Reserved for future PowerStore-specific settings
```

---

## Status

### Conditions

**Type:** `[]metav1.Condition`

**Condition Types:**
- `Ready` - Overall replication health
- `Synced` - Status synchronized from backend

**Condition Fields:**
- `type` (string) - Condition type
- `status` (string) - True, False, Unknown
- `reason` (string) - Machine-readable reason
- `message` (string) - Human-readable message
- `lastTransitionTime` (timestamp) - When status changed
- `observedGeneration` (int64) - Spec generation observed

### ObservedGeneration

**Type:** `int64`  
**Description:** The generation most recently observed by the controller

### DiscoveredBackends

**Type:** `[]BackendInfo`  
**Description:** Storage backends discovered in the cluster

---

## Examples

### Basic Ceph Replication

```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: ceph-basic-replication
  namespace: production
spec:
  replicationState: replica
  replicationMode: asynchronous
  volumeMapping:
    source:
      pvcName: app-data
      namespace: production
    destination:
      volumeHandle: pvc-dest-12345
      namespace: production
  sourceEndpoint:
    cluster: prod-east
    region: us-east-1
    storageClass: ceph-rbd
  destinationEndpoint:
    cluster: prod-west
    region: us-west-1
    storageClass: ceph-rbd
  schedule:
    mode: continuous
    rpo: "15m"
    rto: "5m"
```

### Trident with Actions

```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: trident-mirror
  namespace: database
spec:
  replicationState: replica
  replicationMode: asynchronous
  volumeMapping:
    source:
      pvcName: postgres-data
      namespace: database
    destination:
      volumeHandle: dr-volume-789
      namespace: database
  sourceEndpoint:
    cluster: primary-dc
    region: datacenter-1
    storageClass: trident-nas
  destinationEndpoint:
    cluster: dr-dc
    region: datacenter-2
    storageClass: trident-nas
  schedule:
    mode: scheduled
    rpo: "1h"
    rto: "15m"
  extensions:
    trident:
      actions:
      - type: mirror-update
        snapshotHandle: snap-hourly-001
```

### PowerStore Metro Replication

```yaml
apiVersion: replication.unified.io/v1alpha1
kind: UnifiedVolumeReplication
metadata:
  name: powerstore-metro
  namespace: critical-apps
spec:
  replicationState: source
  replicationMode: synchronous  # Metro replication
  volumeMapping:
    source:
      pvcName: app-volume
      namespace: critical-apps
    destination:
      volumeHandle: metro-volume-456
      namespace: critical-apps
  sourceEndpoint:
    cluster: site-a
    region: datacenter-1
    storageClass: powerstore-block
  destinationEndpoint:
    cluster: site-b
    region: datacenter-2
    storageClass: powerstore-block
  schedule:
    mode: continuous
    rpo: "0s"  # Synchronous
    rto: "1m"
  extensions:
    powerstore: {}  # Reserved for future use
```

---

## Field Validation

### Name Validation
- Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
- Max length: 253 characters
- Must be DNS-compatible

### Schedule Expression Validation
- Pattern: `^[0-9]+(s|m|h|d)$`
- Examples: `15m`, `1h`, `30s`, `1d`

### Cluster Name Validation
- Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
- Max length: 253 characters

---

## Kubectl Commands

### List All Replications
```bash
kubectl get uvr -A
kubectl get unifiedvolumereplications --all-namespaces
```

### Get Specific Replication
```bash
kubectl get uvr my-replication -n default
kubectl get uvr my-replication -n default -o yaml
kubectl describe uvr my-replication -n default
```

### Watch Status Changes
```bash
kubectl get uvr my-replication -n default -w
kubectl get uvr -A -w
```

### Filter by State
```bash
kubectl get uvr -A -o json | \
  jq '.items[] | select(.spec.replicationState=="source") | .metadata.name'
```

### Get Conditions
```bash
kubectl get uvr my-replication -n default \
  -o jsonpath='{.status.conditions[*].type}'
```

---

## API Endpoints

### Webhook
- Path: `/validate-replication-unified-io-v1alpha1-unifiedvolumereplication`
- Port: 9443
- Protocol: HTTPS
- Purpose: Admission validation

### Metrics
- Path: `/metrics`
- Port: 8080
- Protocol: HTTP
- Purpose: Prometheus scraping

### Health
- Path: `/healthz`
- Port: 8081
- Protocol: HTTP
- Purpose: Liveness probe

### Readiness
- Path: `/readyz`
- Port: 8081
- Protocol: HTTP
- Purpose: Readiness probe

---

## Error Codes

### Validation Errors
- `ValidationFailed` - Spec validation failed
- `InvalidStateTransition` - Invalid state change
- `InvalidConfiguration` - Configuration error

### Operational Errors
- `AdapterError` - Backend adapter error
- `InitializationFailed` - Adapter initialization failed
- `OperationFailed` - Backend operation failed
- `TranslationFailed` - State/mode translation failed
- `DiscoveryFailed` - Backend discovery failed

### Resource Errors
- `ResourceNotFound` - Backend resource not found
- `ConnectionError` - Cannot connect to backend
- `TimeoutError` - Operation timed out

---

**Document Version:** 1.0  
**API Version:** replication.unified.io/v1alpha1  
**Last Updated:** 2024-10-07

