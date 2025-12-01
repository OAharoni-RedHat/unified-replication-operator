# Demo Directory

Complete demonstration package for the Unified Replication Operator.

---

## 🚀 **Quick Start**

### Step 1: Build and Deploy Operator

Use the build script to build, push, and deploy in one step:

```bash
# For OpenShift internal registry
export KUBECONFIG=/path/to/your/kubeconfig
oc patch configs.imageregistry.operator.openshift.io/cluster --type merge -p '{"spec":{"defaultRoute":true}}'
REGISTRY=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
TOKEN=$(oc whoami -t)
podman login -u $(oc whoami) -p $TOKEN $REGISTRY --tls-verify=false

cd /path/to/unified-replication-operator
REGISTRY=$REGISTRY/unified-replication-system VERSION=2.0.0-beta ./scripts/build-and-push.sh

# For external registry (Quay.io, Docker Hub, etc.)
podman login quay.io
REGISTRY=quay.io/YOUR_USERNAME VERSION=2.0.0-beta ./scripts/build-and-push.sh
```

The script handles:
- ✅ Running tests
- ✅ Building operator binary
- ✅ Building container image
- ✅ Pushing to registry
- ✅ Installing CRDs
- ✅ Deploying operator via Helm

### Step 2: Run Demo

```bash
cd demo
./run-demo.sh
```

---

## 📚 **Demo Guides**

- **[V2_TRIDENT_DEMO_GUIDE.md](V2_TRIDENT_DEMO_GUIDE.md)** ⭐ - Complete Trident demo with installation steps
- **[V2_DELL_POWERSTORE_DEMO_GUIDE.md](V2_DELL_POWERSTORE_DEMO_GUIDE.md)** ⭐ - Complete Dell PowerStore demo with operator integration
- **[DEPRECATION_NOTICE.md](DEPRECATION_NOTICE.md)** - Historical reference (v1alpha1 removed)

---

## 📋 **Demo Workflow**

1. **Build & Deploy** - Use `../scripts/build-and-push.sh`
2. **Create VolumeReplicationClass** - Configure backend settings
3. **Create VolumeReplication** - Set up replication relationship
4. **Monitor** - Watch replication status
5. **Test Failover** - Promote secondary to primary

---

## 🎯 **Quick Commands**

```bash
# Check operator status
kubectl get pods -n unified-replication-system

# View operator logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f

# Check replications
kubectl get vr,vrc -A

# Clean up demo resources (keeps operator installed)
../scripts/cleanup-demo.sh

# Clean up everything including operator
../scripts/cleanup-demo.sh --operator
```

---

## 📖 **Related Documentation**

- **[../README.md](../README.md)** - Main operator documentation
- **[../BUILD_AND_DEPLOY.md](../BUILD_AND_DEPLOY.md)** - Detailed build instructions
- **[../QUICK_START.md](../QUICK_START.md)** - Quick setup guide

---

*Demo Package Version: 2.0*  
*Operator Version: 2.0.0-beta*  
*Last Updated: 2025-11-05*

