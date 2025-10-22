# Security Policy

This document outlines the security features, policies, and best practices for the Unified Replication Operator.

## Security Features

### 1. Validation

**API Validation:**
- Server-side validation of all UnifiedVolumeReplication resources
- Prevents invalid configurations from being created
- Schema validation via OpenAPI v3
- Custom validation rules in CRD

**Validation Rules:**
- Endpoint validation (source ≠ destination)
- Volume mapping validation
- Schedule pattern validation (e.g., "5m", "1h")
- Extension validation (backend-specific)
- State transition validation

### 2. RBAC (Role-Based Access Control)

**Operator Service Account:**
- Limited to required permissions only
- Separate namespace isolation
- No cluster-admin privileges

**Permissions:**
- CRDs: Full access to UnifiedVolumeReplication
- Backend CRDs: Full access (VolumeReplication, TridentMirrorRelationship, DellCSIReplicationGroup)
- Core resources: Read-only (PVCs, PVs, StorageClasses)
- Events: Create/Patch only

**Viewer Role:**
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: unified-replication-viewer
rules:
- apiGroups: ["replication.unified.io"]
  resources: ["unifiedvolumereplications"]
  verbs: ["get", "list", "watch"]
```

### 3. Pod Security

**Security Context:**
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  fsGroup: 65532
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL
  seccompProfile:
    type: RuntimeDefault
```

**Pod Security Standards:**
- Compliant with "restricted" profile
- No privileged containers
- No host network/PID/IPC
- Limited volume types

### 4. Network Policies

**Ingress Rules:**
- API server access only

**Egress Rules:**
- Kubernetes API server
- DNS resolution
- Backend operator services

**Example:**
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: unified-replication-operator
spec:
  podSelector:
    matchLabels:
      control-plane: controller-manager
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: kube-system
  egress:
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
```

### 5. Audit Logging

**Audit Events:**
- Resource creation/update/deletion
- State transitions
- Backend operations
- Failures and errors

**Configuration:**
```yaml
security:
  audit:
    enabled: true
    logLevel: info
    includeMetadata: true
```

**Log Format:**
```json
{
  "timestamp": "2024-10-07T12:34:56Z",
  "level": "info",
  "msg": "Replication created",
  "resource": "my-replication",
  "namespace": "default",
  "backend": "ceph",
  "user": "system:serviceaccount:default:operator"
}
```

### 6. Secrets Management

**Backend Credentials:**
- Stored in Kubernetes Secrets
- Mounted as volumes (not environment variables)
- Minimal secret access (named resources only)

**Best Practices:**
- Use external secret management (e.g., Vault, AWS Secrets Manager)
- Rotate credentials regularly
- Enable encryption at rest (etcd encryption)

## Threat Model

### Threats and Mitigations

**1. Malicious Resource Creation**
- **Threat:** Attacker creates resources to disrupt system
- **Mitigation:** API validation, RBAC, audit logging

**2. Privilege Escalation**
- **Threat:** Operator gains excessive permissions
- **Mitigation:** Minimal RBAC, pod security context, no cluster-admin

**3. Data Exfiltration**
- **Threat:** Sensitive data accessed via operator
- **Mitigation:** Read-only access to core resources, network policies

**4. Denial of Service**
- **Threat:** Resource exhaustion or disruption
- **Mitigation:** Resource limits, rate limiting, validation

**5. Supply Chain Attack**
- **Threat:** Compromised dependencies or container images
- **Mitigation:** Image scanning, signed images, vendored dependencies

**6. Configuration Tampering**
- **Threat:** Unauthorized modification of CRDs
- **Mitigation:** RBAC, audit logging, validation

**7. Man-in-the-Middle**
- **Threat:** Interception of communication
- **Mitigation:** TLS for API communication, certificate validation

**8. Unauthorized Access**
- **Threat:** Access to resources without permission
- **Mitigation:** RBAC, API validation, audit logging

## Security Configuration

### RBAC Configuration

See `config/security/rbac.yaml` for complete RBAC manifests.

### Network Policy

Enable network policies in your cluster:
```bash
kubectl apply -f config/security/network-policy.yaml
```

### Pod Security Standards

Apply pod security policy:
```bash
kubectl label namespace unified-replication-system \
  pod-security.kubernetes.io/enforce=restricted \
  pod-security.kubernetes.io/audit=restricted \
  pod-security.kubernetes.io/warn=restricted
```

## Security Testing

### Run Security Tests

```bash
# Run all security tests
go test -v ./pkg/security/...

# Benchmark validation performance
go test -bench=. ./api/v1alpha1/...

# Test RBAC policies
kubectl auth can-i --list --as=system:serviceaccount:unified-replication-system:operator
```

### Vulnerability Scanning

```bash
# Scan container image
trivy image unified-replication-operator:latest

# Scan Go dependencies
govulncheck ./...

# Scan Kubernetes manifests
kubesec scan config/security/*.yaml
```

## Incident Response

### Security Incident Process

1. **Detection:**
   - Monitor audit logs
   - Check for unusual resource creation
   - Review RBAC denials

2. **Investigation:**
   - Review operator logs
   - Check resource changes
   - Verify backend operations

3. **Containment:**
   - Pause operator (scale to 0)
   - Block network access
   - Revoke compromised credentials

4. **Remediation:**
   - Apply security patches
   - Rotate credentials
   - Update RBAC policies

5. **Recovery:**
   - Verify fixes
   - Scale operator back up
   - Monitor for issues

### Contact

Report security vulnerabilities to: security@unified-replication.io

## Security Best Practices

### Deployment

1. **Use dedicated namespace:**
   ```bash
   kubectl create namespace unified-replication-system
   ```

2. **Enable pod security:**
   ```bash
   kubectl label namespace unified-replication-system \
     pod-security.kubernetes.io/enforce=restricted
   ```

3. **Apply network policies:**
   ```bash
   kubectl apply -f config/security/network-policy.yaml
   ```

4. **Use minimal RBAC:**
   ```bash
   kubectl apply -f config/security/rbac.yaml
   ```

5. **Enable audit logging:**
   ```yaml
   security:
     audit:
       enabled: true
   ```

### Operations

1. **Regular updates:** Keep operator and dependencies up to date
2. **Monitor logs:** Review audit logs regularly
3. **Rotate credentials:** Update backend credentials periodically
4. **Review RBAC:** Audit permissions quarterly
5. **Scan images:** Use vulnerability scanning in CI/CD

### Development

1. **Code review:** All changes reviewed by security-aware developers
2. **Static analysis:** Use gosec, staticcheck
3. **Dependency scanning:** Monitor for vulnerable dependencies
4. **Testing:** Include security tests in test suite
5. **Documentation:** Keep security docs updated

## Compliance Checklist

- [x] API validation implemented
- [x] RBAC with minimal permissions
- [x] Pod security context (restricted)
- [x] Network policies defined
- [x] Audit logging available
- [x] Secret management documented
- [x] Threat model documented
- [x] Security tests included
- [x] Incident response process defined
- [x] Regular security updates

## References

- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/)
- [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
- [RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Network Policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/)
- [Secrets Management](https://kubernetes.io/docs/concepts/configuration/secret/)

---

**Last Updated:** 2024-10-22  
**Version:** 1.0.0
