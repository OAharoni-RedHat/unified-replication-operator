# Tutorial: Performing a Failover

## Overview

This tutorial walks through performing a complete failover operation, promoting a replica volume to become the source.

## Scenario

- **Primary Site:** us-east-1 (currently source)
- **DR Site:** us-west-1 (currently replica)
- **Goal:** Failover to DR site due to primary site issue

## Prerequisites

- Unified Replication Operator installed
- Existing replication configured and healthy
- Access to both source and destination clusters

## Step-by-Step Failover

### Step 1: Verify Current State

```bash
# Check current replication status
kubectl get uvr production-db -n database

# Output should show:
# NAME            STATE     MODE    AGE
# production-db   replica   async   24h
```

```bash
# Check detailed status
kubectl describe uvr production-db -n database

# Verify conditions show Ready=True
```

### Step 2: Initiate Promotion

```bash
# Edit the replication resource
kubectl edit uvr production-db -n database

# Change replicationState from 'replica' to 'promoting'
spec:
  replicationState: promoting  # Changed from: replica
```

Or use kubectl patch:

```bash
kubectl patch uvr production-db -n database --type='merge' -p '
spec:
  replicationState: promoting
'
```

### Step 3: Monitor Promotion Progress

```bash
# Watch status changes
kubectl get uvr production-db -n database -w

# Check conditions
kubectl get uvr production-db -n database -o jsonpath='{.status.conditions[?(@.type=="Ready")]}'
```

Expected progression:
```
replica → promoting (in progress) → promoting (near complete)
```

### Step 4: Complete Promotion

Once backend reports promotion is complete:

```bash
kubectl patch uvr production-db -n database --type='merge' -p '
spec:
  replicationState: source
'
```

### Step 5: Verify New Source

```bash
# Check final state
kubectl get uvr production-db -n database

# Output should show:
# NAME            STATE    MODE    AGE
# production-db   source   async   24h

# Verify application can access volume
kubectl get pvc -n database
kubectl describe pvc production-db-pvc -n database
```

### Step 6: Update Application

```bash
# If application was pointing to old primary:
# 1. Update application to use new primary (DR site)
# 2. Restart application pods
kubectl rollout restart deployment/production-db -n database
```

## Rollback (If Needed)

If failover needs to be cancelled during promotion:

```bash
# Return to replica state
kubectl patch uvr production-db -n database --type='merge' -p '
spec:
  replicationState: replica
'
```

**Note:** This may require manual intervention on the backend storage system.

## Post-Failover

### Establish Reverse Replication

After failover, establish replication in reverse direction:

```yaml
# Update replication to replicate from new source (DR) to old source (recovery)
spec:
  sourceEndpoint:
    cluster: dr-cluster      # Now source
    region: us-west-1
  destinationEndpoint:
    cluster: prod-cluster    # Now replica
    region: us-east-1
```

## Automated Failover Script

```bash
#!/bin/bash
# failover.sh - Automated failover script

UVR_NAME="${1}"
NAMESPACE="${2:-default}"

echo "Starting failover for $UVR_NAME in $NAMESPACE"

# Step 1: Get current state
CURRENT_STATE=$(kubectl get uvr $UVR_NAME -n $NAMESPACE -o jsonpath='{.spec.replicationState}')
echo "Current state: $CURRENT_STATE"

if [ "$CURRENT_STATE" != "replica" ]; then
    echo "Error: Can only failover from replica state"
    exit 1
fi

# Step 2: Start promotion
echo "Starting promotion..."
kubectl patch uvr $UVR_NAME -n $NAMESPACE --type='merge' -p '{"spec":{"replicationState":"promoting"}}'

# Step 3: Wait for promotion
echo "Waiting for promotion to complete..."
kubectl wait --for=condition=Ready \
  uvr/$UVR_NAME -n $NAMESPACE \
  --timeout=300s

# Step 4: Complete promotion
echo "Completing promotion..."
kubectl patch uvr $UVR_NAME -n $NAMESPACE --type='merge' -p '{"spec":{"replicationState":"source"}}'

# Step 5: Verify
NEW_STATE=$(kubectl get uvr $UVR_NAME -n $NAMESPACE -o jsonpath='{.spec.replicationState}')
echo "Failover complete. New state: $NEW_STATE"
```

## Best Practices

1. **Test failover regularly** - Validate DR procedures
2. **Document runbooks** - Clear step-by-step procedures
3. **Automate where possible** - Reduce human error
4. **Monitor during failover** - Watch metrics and logs
5. **Communicate with stakeholders** - Keep teams informed
6. **Verify application functionality** - Test after failover
7. **Establish reverse replication** - Prepare for failback

## Troubleshooting Failover

### Promotion Stuck

**Check backend logs:**
```bash
# For Ceph
kubectl logs -n rook-ceph -l app=rook-ceph-operator

# For Trident
kubectl logs -n trident -l app=controller

# For PowerStore
kubectl logs -n dell-csi -l app=dell-csi-controller
```

### Application Can't Access Volume

**Verify PVC binding:**
```bash
kubectl get pvc -n database
kubectl describe pvc production-db-pvc -n database
```

**Check volume attachment:**
```bash
kubectl get volumeattachment | grep production-db
```

---

**Tutorial Version:** 1.0  
**Difficulty:** Intermediate  
**Time:** 15-30 minutes

