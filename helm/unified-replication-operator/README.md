# Unified Replication Operator Helm Chart

## Overview

This Helm chart deploys the Unified Replication Operator, providing unified storage replication management across Ceph-CSI, NetApp Trident, and Dell PowerStore backends.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.x
- At least one supported storage backend (Ceph/Trident/PowerStore) installed

## Installation

### Quick Start

```bash
# Add Helm repository (if published)
helm repo add unified-replication https://unified-replication.io/charts
helm repo update

# Install operator
helm install unified-replication-operator unified-replication/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace
```

### Install from Source

```bash
# Clone repository
git clone https://github.com/unified-replication/operator
cd operator

# Install
./scripts/install.sh
```

### Custom Installation

```bash
# With custom values
helm install unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace \
  --set controller.maxConcurrentReconciles=5 \
  --set security.audit.enabled=true \
  --set monitoring.serviceMonitor.enabled=true
```

## Configuration

### Key Configuration Options

#### Controller Settings

```yaml
controller:
  maxConcurrentReconciles: 3     # Concurrent reconciliation limit
  reconcileTimeout: "5m"          # Timeout for reconciliation
  useIntegratedEngine: true       # Enable discovery/translation engines
  enableAdvancedFeatures: true    # Enable retry/circuit breaker/metrics
  logLevel: info                  # Logging level (debug, info, warn, error)
```

#### Resource Limits

```yaml
resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

#### Security

```yaml
security:
  audit:
    enabled: true                 # Enable audit logging
    maxEvents: 1000              # Audit event buffer size
  networkPolicy:
    enabled: true                 # Enable network policies
  podSecurityStandards:
    enforce: restricted           # Pod security level
```

#### Backend Adapters

```yaml
backends:
  ceph:
    enabled: true                 # Enable Ceph adapter
  trident:
    enabled: true                 # Enable Trident adapter
  powerstore:
    enabled: true                 # Enable PowerStore adapter
  mock:
    enabled: false                # Mock adapters (testing only)
```

#### Monitoring

```yaml
monitoring:
  serviceMonitor:
    enabled: true                 # Create Prometheus ServiceMonitor
    interval: 30s                 # Scrape interval
  grafanaDashboard:
    enabled: true                 # Create Grafana dashboard
```

### Complete Values Reference

See [values.yaml](values.yaml) for all configuration options.

## Upgrade

```bash
# Using script
./scripts/upgrade.sh

# Or via Helm
helm upgrade unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --wait
```

### Upgrade with Value Changes

```bash
helm upgrade unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --set controller.maxConcurrentReconciles=5 \
  --wait
```

### Rollback

```bash
# List revisions
helm history unified-replication-operator -n unified-replication-system

# Rollback to previous
helm rollback unified-replication-operator -n unified-replication-system

# Rollback to specific revision
helm rollback unified-replication-operator 3 -n unified-replication-system
```

## Uninstallation

```bash
# Using script (with prompts)
./scripts/uninstall.sh

# Or via Helm
helm uninstall unified-replication-operator -n unified-replication-system

# Delete CRDs (WARNING: deletes all replication resources)
kubectl delete crd unifiedvolumereplications.replication.unified.io

# Delete namespace
kubectl delete namespace unified-replication-system
```

## Testing

### Validate Installation

```bash
# Check pods
kubectl get pods -n unified-replication-system

# Check webhook
kubectl get validatingwebhookconfiguration

# Check CRDs
kubectl get crd | grep replication

# Create test resource
kubectl apply -f examples/sample-replication.yaml

# Check status
kubectl get uvr -A
```

### Dry Run

```bash
# Test installation without applying
helm install unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --dry-run --debug
```

### Template Rendering

```bash
# Render templates
helm template unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  > rendered.yaml
```

## Advanced Configuration

### Enable All Features

```yaml
# values-production.yaml
controller:
  maxConcurrentReconciles: 5
  enableAdvancedFeatures: true
  useIntegratedEngine: true

advancedFeatures:
  stateMachine:
    enabled: true
  retryManager:
    enabled: true
    maxAttempts: 5
  circuitBreaker:
    enabled: true
  metrics:
    enabled: true
  healthChecks:
    enabled: true

security:
  audit:
    enabled: true
  networkPolicy:
    enabled: true

monitoring:
  serviceMonitor:
    enabled: true
  grafanaDashboard:
    enabled: true

backends:
  ceph: {enabled: true}
  trident: {enabled: true}
  powerstore: {enabled: true}
```

```bash
helm install unified-replication-operator ./helm/unified-replication-operator \
  -f values-production.yaml \
  --namespace unified-replication-system \
  --create-namespace
```

### Using cert-manager

```yaml
webhook:
  enabled: true
  certManager:
    enabled: true  # Use cert-manager for certificates
```

### Custom Images

```yaml
image:
  repository: my-registry.com/unified-replication-operator
  tag: v0.2.0
  pullPolicy: Always

imagePullSecrets:
- name: my-registry-secret
```

### Node Affinity

```yaml
nodeSelector:
  node-role.kubernetes.io/control-plane: ""

affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
```

### Tolerations

```yaml
tolerations:
- key: node-role.kubernetes.io/control-plane
  operator: Exists
  effect: NoSchedule
```

## Kustomize Overlays

Pre-configured overlays for different environments:

### Development
```bash
kubectl apply -k config/overlays/development
```

**Features:**
- 1 replica
- Low resource limits
- Debug logging
- Mock adapters enabled

### Staging
```bash
kubectl apply -k config/overlays/staging
```

**Features:**
- 2 replicas
- Medium resource limits
- Info logging
- Metrics enabled

### Production
```bash
kubectl apply -k config/overlays/production
```

**Features:**
- 3 replicas with anti-affinity
- High resource limits
- Info logging
- Audit + Metrics enabled
- Network policies enforced

## Monitoring

### Prometheus Metrics

The operator exposes metrics on port 8080 at `/metrics`:

```bash
# Port-forward to access metrics
kubectl port-forward -n unified-replication-system \
  svc/unified-replication-operator-metrics 8080:8080

# View metrics
curl http://localhost:8080/metrics
```

### Key Metrics

- `unified_replication_reconcile_total` - Total reconciliations
- `unified_replication_reconcile_duration_seconds` - Reconciliation latency
- `unified_replication_state_transitions_total` - State transitions
- `unified_replication_backend_operations_total` - Backend operations
- `unified_replication_circuit_breaker_state` - Circuit breaker status

### Grafana Dashboard

If enabled, a ConfigMap with Grafana dashboard JSON is created:

```bash
kubectl get configmap -n unified-replication-system \
  unified-replication-operator-grafana-dashboard -o yaml
```

## Troubleshooting

### Pods Not Starting

```bash
# Check events
kubectl get events -n unified-replication-system --sort-by='.lastTimestamp'

# Check pod logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager

# Describe pod
kubectl describe pod -n unified-replication-system -l control-plane=controller-manager
```

### Webhook Not Working

```bash
# Check webhook configuration
kubectl get validatingwebhookconfiguration

# Check webhook service
kubectl get svc -n unified-replication-system

# Check certificate secret
kubectl get secret -n unified-replication-system | grep webhook-cert

# Test webhook
kubectl apply -f examples/sample-replication.yaml --dry-run=server
```

### RBAC Issues

```bash
# Check service account
kubectl get sa -n unified-replication-system

# Check permissions
kubectl auth can-i create unifiedvolumereplications \
  --as=system:serviceaccount:unified-replication-system:unified-replication-operator

# View role bindings
kubectl get clusterrolebinding | grep unified-replication
```

## Support

- Documentation: https://unified-replication.io/docs
- Issues: https://github.com/unified-replication/operator/issues
- Community: https://unified-replication.io/community

## License

Apache License 2.0

