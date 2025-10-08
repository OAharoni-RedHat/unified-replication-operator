# Security Policy

## Overview

The Unified Replication Operator follows security best practices and implements multiple layers of defense to protect Kubernetes clusters and storage systems.

## Security Features

### 1. Admission Webhooks

**Validating Webhook:**
- Validates all UnifiedVolumeReplication resources before admission
- Prevents invalid configurations from entering the cluster
- Enforces business rules and state transitions
- Performance: < 100ms validation time

**TLS Security:**
- Self-signed certificates with 1-year validity
- Automatic certificate rotation support
- Secure webhook endpoint (port 9443)
- mTLS between API server and webhook

### 2. RBAC (Role-Based Access Control)

**Principle of Least Privilege:**
- Minimal required permissions only
- Separate roles for operator and viewers
- No cluster-admin required
- Resource-specific permissions

**Required Permissions:**
- UnifiedVolumeReplication: Full CRUD
- Backend CRDs: Full CRUD (Ceph, Trident, PowerStore)
- Core resources: Read-only (PVCs, PVs, StorageClasses)
- Events: Create/Patch only
- Secrets: Read-only for webhook certs (named resource)

**Viewer Role:**
- Read-only access to UnifiedVolumeReplication resources
- Suitable for monitoring and operators
- No write permissions

### 3. Input Validation and Sanitization

**All user inputs are validated for:**
- Length limits (253 characters for names, 1024 for values)
- Format compliance (DNS-compatible names)
- No script injection (`<script>`, `javascript:`, etc.)
- No path traversal (`../`, `/etc/`, etc.)
- No SQL injection (defensive, though not applicable)
- Control character removal

**Validation Points:**
- Resource names
- Namespace names
- Cluster names
- Storage class names
- Schedule expressions (RPO/RTO)
- All string fields

### 4. Audit Logging

**All security-relevant events are logged:**
- CREATE, UPDATE, DELETE operations
- Validation successes and failures
- State changes
- Authentication/authorization failures
- Policy violations

**Audit Log Contents:**
- Event type and timestamp
- User and service account
- Resource namespace and name
- Operation and result
- Request ID (correlation)
- Detailed context

**Audit Log Retention:**
- Last 1000 events in memory
- Export capability for external systems
- Query by type, time, user

### 5. Network Policies

**Ingress Rules:**
- Webhook traffic from API server only
- Metrics scraping from Prometheus namespace
- Health checks from any namespace

**Egress Rules:**
- DNS resolution (kube-system)
- Kubernetes API access
- Storage backend access (all namespaces)
- No unrestricted egress

**Default Deny:**
- Deny-all policy as baseline
- Explicit allows only

### 6. Pod Security

**Pod Security Policy (PSP):**
- No privileged containers
- No privilege escalation
- Drop all capabilities
- Run as non-root user
- Read-only root filesystem
- No host namespaces (network, IPC, PID)

**Pod Security Standards (PSS):**
- Namespace labeled with `pod-security.kubernetes.io/enforce: restricted`
- Strictest security profile
- Audit and warn modes enabled

### 7. Container Security

**Image Security:**
- Minimal base image (distroless recommended)
- No secrets in image layers
- Regular vulnerability scanning
- Image signing and verification

**Runtime Security:**
- Non-root user (UID > 1000)
- Read-only filesystem
- No new privileges
- Security context enforced

### 8. Secret Management

**Best Practices:**
- Secrets mounted as volumes (not environment variables)
- Minimal secret access (named resources only)
- No secrets in logs or events
- Encryption at rest (etcd encryption)

**Webhook Certificates:**
- Stored in Kubernetes Secret
- Automatic generation and rotation
- Scoped access (only operator service account)

### 9. Data Protection

**Encryption:**
- TLS for all network communication
- etcd encryption at rest (Kubernetes feature)
- No plaintext sensitive data in logs
- Secure secret handling

**Data Minimization:**
- Only necessary data collected
- PII avoided
- Audit logs rotated

### 10. Compliance

**Security Standards:**
- CIS Kubernetes Benchmark compliant
- NIST guidelines followed
- Principle of least privilege
- Defense in depth

**Audit Trail:**
- All security events logged
- Immutable audit log
- Export capability for compliance
- Retention policy configurable

## Threat Model

### Threats Mitigated

**1. Malicious Resource Creation**
- **Threat:** Attacker creates resources to disrupt system
- **Mitigation:** Webhook validation, RBAC, audit logging

**2. Privilege Escalation**
- **Threat:** Attacker gains elevated permissions
- **Mitigation:** Minimal RBAC, no privileged containers, PSP/PSS

**3. Injection Attacks**
- **Threat:** Script/SQL/Command injection via input fields
- **Mitigation:** Input sanitization, validation, pattern blocking

**4. Path Traversal**
- **Threat:** Access to unauthorized files/directories
- **Mitigation:** Path validation, read-only filesystem

**5. Data Exfiltration**
- **Threat:** Sensitive data leaked
- **Mitigation:** Network policies, no secrets in logs, encryption

**6. Denial of Service**
- **Threat:** Resource exhaustion
- **Mitigation:** Rate limiting, circuit breakers, retry backoff

**7. Man-in-the-Middle**
- **Threat:** Interception of communication
- **Mitigation:** TLS for webhook, mTLS, certificate validation

**8. Unauthorized Access**
- **Threat:** Access to resources without permission
- **Mitigation:** RBAC, admission webhooks, audit logging

## Security Configuration

### Deployment Security Context
```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
    - ALL
```

### Required Secrets
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: unified-replication-webhook-server-cert
  namespace: unified-replication-system
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded-cert>
  tls.key: <base64-encoded-key>
  ca.crt: <base64-encoded-ca>
```

### Webhook Configuration
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: unified-replication-validating-webhook
webhooks:
- name: vunifiedvolumereplication.kb.io
  clientConfig:
    service:
      name: webhook-service
      namespace: unified-replication-system
      path: /validate-replication-unified-io-v1alpha1-unifiedvolumereplication
    caBundle: <base64-encoded-ca-bundle>
  rules:
  - apiGroups: ["replication.unified.io"]
    apiVersions: ["v1alpha1"]
    operations: ["CREATE", "UPDATE"]
    resources: ["unifiedvolumereplications"]
  failurePolicy: Fail
  sideEffects: None
  admissionReviewVersions: ["v1"]
```

## Security Testing

### Validation Tests
```bash
# Run security tests
go test -v ./pkg/security/...

# Run webhook security tests
go test -v ./pkg/webhook/... -run Security

# Benchmark validation performance
go test -bench=. ./pkg/security/...
```

### Vulnerability Scanning
```bash
# Scan Go dependencies
go list -json -m all | nancy sleuth

# Scan container image
trivy image unified-replication-operator:latest

# Security audit
gosec ./...
```

### RBAC Validation
```bash
# Verify permissions
kubectl auth can-i create unifiedvolumereplications \
  --as=system:serviceaccount:unified-replication-system:unified-replication-operator

# Test with minimal permissions
kubectl apply -f config/security/rbac.yaml
```

## Incident Response

### Security Issue Reporting
- Report to: security@example.com
- Include: Detailed description, reproduction steps, impact
- Response Time: 24 hours for critical, 72 hours for others

### Security Updates
- Regular dependency updates
- CVE monitoring
- Security patch releases
- Coordinated disclosure

## Compliance Checklist

- [x] Admission webhooks implemented
- [x] TLS certificates managed
- [x] RBAC with minimal permissions
- [x] Input sanitization and validation
- [x] Audit logging enabled
- [x] Network policies defined
- [x] Pod security policies/standards
- [x] Container security hardened
- [x] Secrets properly handled
- [x] Security documentation complete

## References

- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [NIST Guidelines](https://www.nist.gov/)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)

---

**Last Updated:** 2024-10-07  
**Version:** 1.0  
**Status:** Production Ready

