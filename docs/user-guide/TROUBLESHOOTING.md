# Troubleshooting Guide

## Common Issues and Solutions

### Installation Issues

#### Issue: Operator Pod Won't Start

**Symptoms:**
```bash
$ kubectl get pods -n unified-replication-system
NAME                                         READY   STATUS             RESTARTS   AGE
unified-replication-operator-xxx-yyy         0/1     CrashLoopBackOff   5          5m
```

**Diagnosis:**
```bash
# Check logs
kubectl logs -n unified-replication-system pod/unified-replication-operator-xxx-yyy

# Check events
kubectl describe pod -n unified-replication-system unified-replication-operator-xxx-yyy
```

**Common Causes & Solutions:**

1. **Image Pull Error**
   ```bash
   # Check image pull secrets
   kubectl get secrets -n unified-replication-system
   
   # Verify image exists and is accessible
   docker pull unified-replication-operator:v0.1.0
   ```

2. **Insufficient RBAC Permissions**
   ```bash
   # Verify RBAC
   kubectl get clusterrole | grep unified-replication
   kubectl get clusterrolebinding | grep unified-replication
   
   # Reapply RBAC
   helm upgrade unified-replication-operator ... --set rbac.create=true
   ```

3. **Webhook Certificate Missing**
   ```bash
   # Check certificate secret
   kubectl get secret -n unified-replication-system | grep webhook-cert
   
   # Regenerate if missing
   helm upgrade unified-replication-operator ... --set webhook.certificate.generate=true
   ```

#### Issue: Helm Install Fails

**Error:** `Error: INSTALLATION FAILED`

**Solutions:**

1. **Kubernetes Version Incompatibility**
   ```bash
   # Check K8s version
   kubectl version --short
   
   # Operator requires 1.24+
   # Upgrade cluster if needed
   ```

2. **CRD Already Exists**
   ```bash
   # Delete existing CRD
   kubectl delete crd unifiedvolumereplications.replication.unified.io
   
   # Reinstall
   helm install ...
   ```

3. **Namespace Issues**
   ```bash
   # Delete and recreate namespace
   kubectl delete namespace unified-replication-system
   kubectl create namespace unified-replication-system
   
   # Reinstall
   helm install ...
   ```

---

### Replication Issues

#### Issue: Replication Stuck in "Promoting"

**Symptoms:**
```yaml
status:
  conditions:
  - type: Ready
    status: False
    reason: OperationFailed
```

**Diagnosis:**
```bash
# Check replication status
kubectl describe uvr my-replication -n default

# Check backend resource
# For Ceph:
kubectl get volumereplication -n default

# For Trident:
kubectl get tridentmirrorrelationship -n default

# For PowerStore:
kubectl get dellcsireplicationgroup -n default
```

**Solutions:**

1. **Backend Resource Issue**
   ```bash
   # Check backend controller logs
   kubectl logs -n <backend-namespace> -l app=<backend-controller>
   
   # Manually update backend resource if needed
   ```

2. **State Transition Not Supported**
   ```bash
   # Check valid transitions in state machine
   # Some transitions require intermediate states
   
   # For promoting: replica → promoting → source
   # Update to source after promoting completes
   ```

3. **Timeout**
   ```bash
   # Increase reconcile timeout
   helm upgrade ... --set controller.reconcileTimeout=10m
   ```

#### Issue: Backend Not Detected

**Symptoms:**
```
Error: no backend adapter found for this configuration
```

**Diagnosis:**
```bash
# Check if backend CRDs are installed
kubectl get crd | grep -E "volumereplication|trident|dell"

# Check storage class
kubectl describe uvr my-replication | grep storageClass
```

**Solutions:**

1. **Storage Class Name**
   ```yaml
   # Use backend-specific storage class names:
   storageClass: ceph-rbd        # For Ceph
   storageClass: trident-nas     # For Trident
   storageClass: powerstore-block # For PowerStore
   ```

2. **Explicit Backend Extension**
   ```yaml
   spec:
     # ... other fields
     extensions:
       ceph: {}  # Explicitly select Ceph
   ```

3. **Install Backend**
   ```bash
   # Install required backend CSI driver and CRDs
   # See backend-specific installation guides
   ```

#### Issue: Status Not Updating

**Symptoms:**
```bash
$ kubectl get uvr my-replication
NAME             STATE     MODE    AGE
my-replication   replica   async   1h
# Status not changing
```

**Diagnosis:**
```bash
# Check controller logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager --tail=50 | grep my-replication

# Check reconciliation
kubectl get events -n default --field-selector involvedObject.name=my-replication
```

**Solutions:**

1. **Controller Not Reconciling**
   ```bash
   # Restart controller
   kubectl delete pod -n unified-replication-system -l control-plane=controller-manager
   ```

2. **Backend Resource Not Created**
   ```bash
   # Check if backend resource exists
   kubectl get volumereplication,tridentmirrorrelationship,dellcsireplicationgroup -A
   
   # If missing, check operator logs for creation errors
   ```

---

### Webhook Issues

#### Issue: Admission Webhook Denies All Requests

**Symptoms:**
```
Error from server: admission webhook "vunifiedvolumereplication.kb.io" denied the request
```

**Diagnosis:**
```bash
# Check webhook configuration
kubectl get validatingwebhookconfiguration

# Check webhook pod
kubectl get pods -n unified-replication-system

# Check webhook logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager | grep webhook
```

**Solutions:**

1. **Certificate Issue**
   ```bash
   # Check certificate validity
   kubectl get secret unified-replication-operator-webhook-cert \
     -n unified-replication-system \
     -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text
   
   # Regenerate if expired
   helm upgrade ... --set webhook.certificate.generate=true
   ```

2. **Webhook Service Not Reachable**
   ```bash
   # Check service
   kubectl get svc -n unified-replication-system
   
   # Test connectivity
   kubectl run test-pod --rm -it --image=curlimages/curl -- \
     curl -k https://unified-replication-operator-webhook-service.unified-replication-system:443
   ```

3. **Disable Webhook Temporarily**
   ```bash
   # Delete webhook configuration
   kubectl delete validatingwebhookconfiguration \
     unified-replication-operator-validating-webhook
   
   # Or disable via Helm
   helm upgrade ... --set webhook.enabled=false
   ```

---

### Performance Issues

#### Issue: Slow Reconciliation

**Symptoms:**
- High P95 latency in metrics
- Long time to process changes

**Diagnosis:**
```bash
# Check metrics
kubectl port-forward -n unified-replication-system \
  svc/unified-replication-operator-metrics 8080:8080

curl http://localhost:8080/metrics | grep reconcile_duration
```

**Solutions:**

1. **Increase Concurrency**
   ```yaml
   controller:
     maxConcurrentReconciles: 5  # Increase from default 3
   ```

2. **Optimize Cache**
   ```yaml
   engines:
     discovery:
       cacheExpiry: "10m"  # Longer cache
   ```

3. **Reduce Timeout**
   ```yaml
   controller:
     reconcileTimeout: "3m"  # Reduce if operations are fast
   ```

#### Issue: High Memory Usage

**Symptoms:**
- Pod OOMKilled
- Memory usage climbing

**Diagnosis:**
```bash
# Check memory usage
kubectl top pod -n unified-replication-system

# Check number of resources
kubectl get uvr -A --no-headers | wc -l
```

**Solutions:**

1. **Increase Memory Limits**
   ```yaml
   resources:
     limits:
       memory: 1Gi  # Increase from 512Mi
   ```

2. **Reduce Cache Sizes**
   ```yaml
   engines:
     discovery:
       cacheExpiry: "2m"  # Shorter cache
   
   advancedFeatures:
     stateMachine:
       maxHistorySize: 50  # Reduce from 100
   
   security:
     audit:
       maxEvents: 500  # Reduce from 1000
   ```

---

### Error Messages

#### "invalid state transition from X to Y"

**Meaning:** Attempted state change is not allowed by state machine

**Valid Transitions:**
- replica → promoting → source (failover)
- source → demoting → replica (failback)
- replica → syncing → replica (resync)

**Solution:** Follow valid transition paths

#### "configuration validation failed"

**Meaning:** Resource spec has validation errors

**Common Issues:**
- Missing required fields
- Invalid format (RPO/RTO expressions)
- Invalid enum values

**Solution:** Check `kubectl describe uvr` for specific validation errors

#### "no backend adapter found"

**Meaning:** Cannot determine which backend to use

**Solutions:**
- Add explicit extension (`extensions.ceph`, `.trident`, or `.powerstore`)
- Use recognizable storage class name
- Verify backend CRDs are installed

---

### Debugging Tips

#### Enable Debug Logging

```yaml
# values.yaml
controller:
  logLevel: debug
```

```bash
# Via Helm upgrade
helm upgrade ... --set controller.logLevel=debug
```

#### Trace Specific Resource

```bash
# Follow logs for specific resource
kubectl logs -n unified-replication-system \
  -l control-plane=controller-manager -f | \
  grep "my-replication"
```

#### Check Correlation IDs

```bash
# All operations for a request share correlation ID
kubectl logs -n unified-replication-system \
  -l control-plane=controller-manager | \
  grep "correlationID=default-my-replication-xxx"
```

#### Inspect Backend Resources

```bash
# Ceph
kubectl get volumereplication -A -o yaml

# Trident
kubectl get tridentmirrorrelationship -A -o yaml
kubectl get tridentactionmirrorupdate -A

# PowerStore
kubectl get dellcsireplicationgroup -A -o yaml
```

---

### Getting Help

#### Self-Service

1. Check this troubleshooting guide
2. Review logs: `kubectl logs -n unified-replication-system ...`
3. Check events: `kubectl get events -n <namespace>`
4. Review documentation: docs/

#### Community Support

1. GitHub Issues: https://github.com/unified-replication/operator/issues
2. Search existing issues
3. File new issue with:
   - Kubernetes version
   - Operator version
   - Backend type
   - Resource YAML
   - Operator logs
   - Error messages

#### Enterprise Support

Contact: support@unified-replication.io

Include:
- Support contract number
- Severity level
- Complete diagnostic bundle

---

## Diagnostic Bundle Collection

```bash
#!/bin/bash
# collect-diagnostics.sh

NAMESPACE="${1:-unified-replication-system}"
OUTPUT="diagnostics-$(date +%Y%m%d-%H%M%S).tar.gz"

mkdir -p diagnostics

# Operator info
kubectl get pods -n $NAMESPACE -o yaml > diagnostics/pods.yaml
kubectl get deployment -n $NAMESPACE -o yaml > diagnostics/deployment.yaml
kubectl logs -n $NAMESPACE -l control-plane=controller-manager --tail=1000 > diagnostics/logs.txt

# Resources
kubectl get uvr -A -o yaml > diagnostics/replications.yaml
kubectl get events -A --sort-by='.lastTimestamp' | tail -100 > diagnostics/events.txt

# Configuration
helm get values unified-replication-operator -n $NAMESPACE > diagnostics/helm-values.yaml
kubectl get configmap -n $NAMESPACE -o yaml > diagnostics/configmaps.yaml

# Cluster info
kubectl version > diagnostics/version.txt
kubectl get nodes > diagnostics/nodes.txt

# Package
tar -czf $OUTPUT diagnostics/
rm -rf diagnostics/

echo "Diagnostics collected: $OUTPUT"
```

---

**Document Version:** 1.0  
**Last Updated:** 2024-10-07

