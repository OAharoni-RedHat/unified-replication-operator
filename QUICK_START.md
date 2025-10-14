# Quick Start Guide

> 🎬 **Want a complete demo?** See the **[demo/](demo/)** folder for comprehensive walkthrough!

## ✅ What's Working

Your operator build environment is now fully configured:

- ✅ **Go 1.24** - Installed and working
- ✅ **controller-gen v0.16.5** - Updated to be compatible with Go 1.24
- ✅ **Dockerfile** - Fixed to use Go 1.24
- ✅ **Build script** - Created and tested
- ✅ **Binary builds** - Working (50.2 MB image)
- ✅ **Webhook certificates** - Fixed and working
- ✅ **OpenShift compatibility** - Configured

## 🚀 Next Steps

### 1. Login to Quay.io

```bash
podman login quay.io
# Enter username: rh-ee-oaharoni
# Enter password: (your quay.io password or robot token)
```

### 2. Build, Push, and Deploy

```bash
export KUBECONFIG=/home/oaharoni/aws-gpfs-playground/ocp_install_files/auth/kubeconfig
./scripts/build-and-push.sh
```

This will automatically:
- Build the operator
- Push to `quay.io/rh-ee-oaharoni/unified-replication-operator:0.1.0`
- Deploy to your OpenShift cluster
- Verify the deployment

### 3. Test Your Operator

```bash
# Apply your Trident replication
kubectl apply -f trident-replication.yaml

# Check status
kubectl get unifiedvolumereplications -A
kubectl describe uvr trident-volume-replication -n default

# View operator logs
kubectl logs -n unified-replication-system -l control-plane=controller-manager -f
```

## 🔧 Fixed Issues

1. **controller-gen panic** - Updated from v0.13.0 to v0.16.5 (Go 1.24 compatible)
2. **Dockerfile Go version** - Updated from 1.21 to 1.24
3. **Webhook certificates** - Created missing certificate generation and service
4. **OpenShift SCC** - Configured security context constraints
5. **RBAC permissions** - Added webhook cert creation permissions
6. **Build script typo** - Fixed `SKIP_TESTS` variable

## 📝 Configuration

Your current settings in `scripts/build-and-push.sh`:

```bash
REGISTRY="quay.io/rh-ee-oaharoni"
VERSION="0.1.0"
SKIP_TESTS="true"  # Tests are skipped by default
SKIP_DEPLOY="false"  # Will deploy to cluster
PUSH_IMAGE="true"  # Will push to registry
```

## 🎯 Common Commands

```bash
# Build only (no push/deploy)
PUSH_IMAGE=false SKIP_DEPLOY=true ./scripts/build-and-push.sh

# Build and push (no deploy)
SKIP_DEPLOY=true ./scripts/build-and-push.sh

# Custom version
VERSION=0.2.0 ./scripts/build-and-push.sh

# Run tests
SKIP_TESTS=false ./scripts/build-and-push.sh

# View help
./scripts/build-and-push.sh --help
```

## 🐛 Troubleshooting

### Image Pull Errors

If pods show `ImagePullBackOff`, make sure your image is public on quay.io:

1. Go to https://quay.io/repository/rh-ee-oaharoni/unified-replication-operator
2. Settings → Make Public

Or create an image pull secret:

```bash
kubectl create secret docker-registry quay-secret \
  --docker-server=quay.io \
  --docker-username=rh-ee-oaharoni \
  --docker-password=YOUR_PASSWORD \
  -n unified-replication-system

helm upgrade unified-replication-operator ./helm/unified-replication-operator \
  -n unified-replication-system \
  --set imagePullSecrets[0].name=quay-secret
```

### Webhook Certificate Errors

If you see certificate errors:

```bash
./scripts/create-webhook-cert.sh
kubectl rollout restart deployment unified-replication-operator -n unified-replication-system
```

### Build Failures

If controller-gen fails again:

```bash
# Clean and rebuild
go clean -cache -modcache -testcache
rm -f bin/controller-gen
make controller-gen
make build
```

## 📚 Documentation

- [BUILD_AND_DEPLOY.md](BUILD_AND_DEPLOY.md) - Detailed build instructions
- [OPENSHIFT_INSTALL.md](OPENSHIFT_INSTALL.md) - OpenShift-specific setup
- [README.md](README.md) - Main operator documentation

## 🎉 You're Ready!

Your operator is now ready to build and deploy. Just run:

```bash
podman login quay.io
./scripts/build-and-push.sh
```

Then test with your `trident-replication.yaml`!

