# Operations Guide

## Production Operations

This guide covers operational procedures for running the Unified Replication Operator in production.

## Health Monitoring

### Health Check Endpoints

**Liveness Probe:** `/healthz` on port 8081
- Verifies operator is running
- Checks recent reconciliation activity
- Validates error rate < 50%

**Readiness Probe:** `/readyz` on port 8081
- Verifies operator is ready to handle requests
- Checks all engines initialized
- Validates dependencies available

### Prometheus Metrics

**Key Metrics to Monitor:**

```prometheus
# Reconciliation success rate (should be > 95%)
sum(rate(unified_replication_reconcile_total{result="success"}[5m])) /
sum(rate(unified_replication_reconcile_total[5m]))

# P95 reconciliation latency (should be < 1s)
histogram_quantile(0.95, 
  sum(rate(unified_replication_reconcile_duration_seconds_bucket[5m])) by (le))

# Error rate (should be < 5%)
sum(rate(unified_replication_reconcile_errors_total[5m]))

# Circuit breaker state (alert when open)
unified_replication_circuit_breaker_state > 0

# Active replications
unified_replication_active_total

# Discovery cache hit rate (should be > 80%)
unified_replication_discovery_cache_hits_total /
(unified_replication_discovery_cache_hits_total + unified_replication_discovery_cache_misses_total)
```

### Alerting Rules

```yaml
groups:
- name: unified-replication
  rules:
  - alert: HighReconciliationErrorRate
    expr: rate(unified_replication_reconcile_errors_total[5m]) > 0.1
    for: 5m
    annotations:
      summary: "High error rate in reconciliations"
      
  - alert: CircuitBreakerOpen
    expr: unified_replication_circuit_breaker_state == 1
    for: 1m
    annotations:
      summary: "Circuit breaker is open"
      
  - alert: HighReconciliationLatency
    expr: histogram_quantile(0.95, unified_replication_reconcile_duration_seconds) > 5
    for: 5m
    annotations:
      summary: "P95 latency above 5 seconds"
      
  - alert: OperatorDown
    expr: up{job="unified-replication-operator"} == 0
    for: 5m
    annotations:
      summary: "Operator is down"
```

## Capacity Planning

### Resource Requirements

**Per Replication Resource:**
- Memory: ~1KB
- CPU: Negligible (state in etcd)

**Operator Resource Usage:**
- Base: 128Mi RAM, 100m CPU
- Per 100 replications: +64Mi RAM, +50m CPU
- Recommended: 512Mi RAM, 500m CPU for 1000+ replications

### Scaling Guidelines

**Vertical Scaling:**
- Increase `resources.limits` in values.yaml
- Increase `maxConcurrentReconciles` for more parallelism

**Horizontal Scaling:**
- Increase `replicaCount` for HA
- Leader election ensures only one active controller
- Multiple replicas improve availability, not throughput

## Backup and Recovery

### Backup Procedures

**1. Backup CRD Definitions:**
```bash
kubectl get crd unifiedvolumereplications.replication.unified.io -o yaml > uvr-crd-backup.yaml
```

**2. Backup Replication Resources:**
```bash
kubectl get uvr -A -o yaml > all-replications-backup.yaml
```

**3. Backup Operator Configuration:**
```bash
helm get values unified-replication-operator -n unified-replication-system \
  > operator-values-backup.yaml
```

**4. Backup Audit Logs:**
```bash
kubectl logs -n unified-replication-system \
  -l control-plane=controller-manager > operator-logs-backup.txt
```

### Recovery Procedures

**1. Restore Operator:**
```bash
helm install unified-replication-operator ./helm/unified-replication-operator \
  -f operator-values-backup.yaml \
  --namespace unified-replication-system
```

**2. Restore CRDs:**
```bash
kubectl apply -f uvr-crd-backup.yaml
```

**3. Restore Replications:**
```bash
kubectl apply -f all-replications-backup.yaml
```

## Upgrade Procedures

### Minor Version Upgrade

```bash
# 1. Backup current installation
helm get values unified-replication-operator -n unified-replication-system \
  > values-backup.yaml

# 2. Run upgrade
./scripts/upgrade.sh

# 3. Verify
kubectl get pods -n unified-replication-system
kubectl get uvr -A
```

### Major Version Upgrade

```bash
# 1. Read release notes and migration guide
# 2. Test in staging environment first
# 3. Backup all resources
# 4. Schedule maintenance window
# 5. Perform upgrade with --atomic flag
helm upgrade unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --atomic \
  --timeout 10m

# 6. Verify functionality
# 7. Monitor for 24 hours
```

### Rollback

```bash
# Automatic rollback (if upgrade failed)
# Already handled by --atomic flag

# Manual rollback
helm rollback unified-replication-operator -n unified-replication-system

# Rollback to specific revision
helm history unified-replication-operator -n unified-replication-system
helm rollback unified-replication-operator 3 -n unified-replication-system
```

## Disaster Recovery

### Scenario 1: Operator Pod Crashes

**Detection:** Pod in CrashLoopBackOff

**Recovery:**
```bash
# Check logs
kubectl logs -n unified-replication-system pod/xxx --previous

# Check events
kubectl get events -n unified-replication-system

# If configuration issue, fix and restart
kubectl delete pod -n unified-replication-system -l control-plane=controller-manager
```

### Scenario 2: Webhook Not Responding

**Detection:** Admission requests timing out

**Recovery:**
```bash
# Check webhook pod
kubectl get pods -n unified-replication-system

# Check webhook service
kubectl get svc -n unified-replication-system

# Check certificates
kubectl get secret -n unified-replication-system | grep webhook-cert

# Regenerate certificates if needed
./scripts/regenerate-webhook-certs.sh
```

### Scenario 3: All Replications Failing

**Detection:** High error rate in metrics

**Recovery:**
```bash
# Check operator health
kubectl get pods -n unified-replication-system

# Check backend availability
kubectl get crd | grep -E "volumereplication|trident|dell"

# Check adapter registry
kubectl logs -n unified-replication-system -l control-plane=controller-manager | grep "adapter"

# Restart operator if needed
kubectl rollout restart deployment/unified-replication-operator \
  -n unified-replication-system
```

## Performance Tuning

### Optimize Reconciliation

```yaml
# values.yaml
controller:
  maxConcurrentReconciles: 5  # Increase for more parallelism
  reconcileTimeout: "3m"       # Reduce if operations are fast
```

### Optimize Caching

```yaml
engines:
  discovery:
    cacheExpiry: "10m"  # Longer cache for stable environments
  controllerEngine:
    enableCaching: true
    cacheExpiry: "10m"
```

### Resource Tuning

```yaml
resources:
  limits:
    cpu: 1000m      # Increase for heavy load
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi
```

## Security Operations

### Certificate Rotation

```bash
# Check certificate expiry
kubectl get secret unified-replication-operator-webhook-cert \
  -n unified-replication-system -o jsonpath='{.data.tls\.crt}' | \
  base64 -d | openssl x509 -noout -dates

# Rotate certificates
# 1. Generate new certificates
# 2. Update secret
# 3. Restart operator
kubectl rollout restart deployment/unified-replication-operator \
  -n unified-replication-system
```

### Audit Log Management

```bash
# Export audit logs
kubectl exec -n unified-replication-system \
  deployment/unified-replication-operator -- \
  curl http://localhost:8081/audit/export > audit-logs.json

# Query audit logs
kubectl logs -n unified-replication-system \
  -l control-plane=controller-manager | grep "Audit Event"
```

### RBAC Validation

```bash
# Check operator permissions
kubectl auth can-i --list \
  --as=system:serviceaccount:unified-replication-system:unified-replication-operator

# Test specific permission
kubectl auth can-i create unifiedvolumereplications \
  --as=system:serviceaccount:unified-replication-system:unified-replication-operator
```

## Multi-Cluster Operations

### Setup

1. Install operator in each cluster
2. Configure cross-cluster communication
3. Ensure storage backends are configured for remote replication

### Cross-Region Replication

```yaml
spec:
  sourceEndpoint:
    cluster: us-east-cluster
    region: us-east-1
    storageClass: ceph-rbd
  destinationEndpoint:
    cluster: us-west-cluster
    region: us-west-1
    storageClass: ceph-rbd
```

## Maintenance Windows

### Planned Maintenance

```bash
# 1. Notify users
# 2. Stop accepting new replications (optional)
# 3. Wait for in-progress operations
kubectl get uvr -A --no-headers | \
  awk '$3!="Ready"' # Check for non-ready resources

# 4. Perform maintenance
# 5. Verify functionality
# 6. Resume normal operations
```

### Zero-Downtime Updates

```bash
# With multiple replicas and leader election
# Operator can be updated without downtime
helm upgrade unified-replication-operator ... --atomic

# Kubernetes rolling update handles graceful transition
```

## Log Management

### Log Levels

- **debug**: Verbose, all operations
- **info**: Normal operations (default)
- **warn**: Warnings and errors
- **error**: Errors only

```yaml
# Set log level
controller:
  logLevel: info
```

### Structured Logging

All logs include:
- Timestamp
- Level
- Component
- Correlation ID
- Resource namespace/name
- Operation details

### Log Aggregation

```bash
# Using fluentd/fluent-bit
kubectl logs -n unified-replication-system \
  -l control-plane=controller-manager --tail=-1 | \
  fluent-bit -c fluent-bit.conf

# Using Loki
# Configure promtail to scrape operator logs
```

## Operational Runbooks

### Runbook: High Error Rate

**Symptoms:**
- Prometheus alert: HighReconciliationErrorRate
- Many failed reconciliations

**Investigation:**
1. Check operator logs for error patterns
2. Check backend availability
3. Check network connectivity
4. Review recent changes

**Resolution:**
1. Fix underlying issue (network, backend, config)
2. Operator will automatically retry
3. Monitor error rate recovery

### Runbook: Circuit Breaker Open

**Symptoms:**
- Prometheus alert: CircuitBreakerOpen
- Operations being rejected

**Investigation:**
1. Check what triggered circuit breaker
2. Review error rate and types
3. Check backend health

**Resolution:**
1. Fix backend issues
2. Wait for circuit breaker timeout (1m default)
3. Circuit auto-transitions to half-open
4. Monitor for successful operations
5. Circuit auto-closes after success threshold

### Runbook: Memory Pressure

**Symptoms:**
- Pod OOMKilled
- High memory usage metrics

**Investigation:**
1. Check number of managed resources
2. Review cache sizes
3. Check for memory leaks (rare)

**Resolution:**
1. Increase memory limits
2. Reduce cache expiry times
3. Reduce maxConcurrentReconciles if needed

## Best Practices

1. **Always use Helm for deployment** - Not kubectl apply
2. **Test upgrades in staging first** - Before production
3. **Monitor metrics continuously** - Set up alerts
4. **Regular backups** - Daily backup of resources
5. **Certificate rotation** - Before expiry
6. **Resource limits** - Always set limits and requests
7. **High availability** - Use 3 replicas in production
8. **Network policies** - Enable in production
9. **Audit logging** - Enable for compliance
10. **Regular updates** - Apply security patches

## Support Escalation

### Level 1: Self-Service
- Check this operations guide
- Review troubleshooting docs
- Search community forums

### Level 2: Community Support
- File GitHub issue
- Community Slack/Discord
- Stack Overflow

### Level 3: Enterprise Support
- Support ticket
- Priority response
- Direct engineering access

---

**Document Version:** 1.0  
**Last Updated:** 2024-10-07

