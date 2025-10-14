# Build and Deploy Guide

Quick guide for building and deploying the Unified Replication Operator.

## Prerequisites

- **Go 1.24+** - For building the operator
- **Podman or Docker** - For container builds
- **Helm 3.x** - For deployment
- **kubectl or oc CLI** - For cluster interaction
- **Access to a container registry** - quay.io, docker.io, or OpenShift internal registry

## Quick Start

### 1. Set Your Registry

```bash
export REGISTRY=quay.io/YOUR_USERNAME
# or
export REGISTRY=docker.io/YOUR_USERNAME
```

### 2. Login to Registry

```bash
podman login quay.io
# Enter your username and password
```

### 3. Build, Push, and Deploy

```bash
./scripts/build-and-push.sh
```

That's it! The script will:
- ✅ Run tests
- ✅ Build the operator binary
- ✅ Build the container image
- ✅ Push to your registry
- ✅ Deploy to your cluster

## Common Usage Examples

### Build and Push Only (No Deployment)

```bash
REGISTRY=quay.io/myuser SKIP_DEPLOY=true ./scripts/build-and-push.sh
```

### Build Local Only (No Push, No Deploy)

```bash
PUSH_IMAGE=false SKIP_DEPLOY=true ./scripts/build-and-push.sh
```

### Custom Version

```bash
VERSION=0.2.0 REGISTRY=quay.io/myuser ./scripts/build-and-push.sh
```

### Skip Tests (Faster Development)

```bash
SKIP_TESTS=true REGISTRY=quay.io/myuser ./scripts/build-and-push.sh
```

### Use Docker Instead of Podman

```bash
CONTAINER_TOOL=docker REGISTRY=quay.io/myuser ./scripts/build-and-push.sh
```

## For OpenShift Users

### Using OpenShift Internal Registry

First, expose the registry route (if not already done):

```bash
export KUBECONFIG=/path/to/your/kubeconfig
oc patch configs.imageregistry.operator.openshift.io/cluster --type merge -p '{"spec":{"defaultRoute":true}}'
```

Get the registry URL:

```bash
REGISTRY=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
echo $REGISTRY
```

Build and push:

```bash
# Login
TOKEN=$(oc whoami -t)
podman login -u $(oc whoami) -p $TOKEN $REGISTRY --tls-verify=false

# Build and push
REGISTRY=$REGISTRY/unified-replication-system \
  ./scripts/build-and-push.sh
```

### Using External Registry (Easier)

```bash
# Login to quay.io
podman login quay.io

# Build and deploy
REGISTRY=quay.io/myuser ./scripts/build-and-push.sh
```

## Environment Variables Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `VERSION` | 0.1.0 | Image version tag |
| `REGISTRY` | quay.io/YOUR_USERNAME | Container registry |
| `IMAGE_NAME` | unified-replication-operator | Image name |
| `NAMESPACE` | unified-replication-system | Kubernetes namespace |
| `CONTAINER_TOOL` | podman | Container tool (podman or docker) |
| `SKIP_TESTS` | false | Skip running tests |
| `SKIP_DEPLOY` | false | Skip deployment |
| `PUSH_IMAGE` | true | Push image to registry |

## Development Workflow

### Fast Iteration (Local Development)

```bash
# 1. Build and test locally (no push/deploy)
PUSH_IMAGE=false SKIP_DEPLOY=true ./scripts/build-and-push.sh

# 2. Run operator locally
export KUBECONFIG=/path/to/kubeconfig
./bin/manager --leader-elect=false --enable-webhooks=false

# 3. Test your changes
kubectl apply -f trident-replication.yaml
kubectl get uvr -A
```

### Deploy to Dev Cluster

```bash
# Build, push, and deploy to dev
REGISTRY=quay.io/myuser VERSION=dev ./scripts/build-and-push.sh
```

### Deploy to Production

```bash
# Clean build with tests
REGISTRY=quay.io/myuser VERSION=1.0.0 ./scripts/build-and-push.sh

# Or tag existing image
podman tag quay.io/myuser/unified-replication-operator:dev \
  quay.io/myuser/unified-replication-operator:1.0.0
podman push quay.io/myuser/unified-replication-operator:1.0.0
```

## Troubleshooting

### Image Pull Errors

If pods show `ImagePullBackOff`:

1. **Check registry authentication:**
   ```bash
   # Create image pull secret
   kubectl create secret docker-registry regcred \
     --docker-server=quay.io \
     --docker-username=YOUR_USERNAME \
     --docker-password=YOUR_PASSWORD \
     -n unified-replication-system
   
   # Update Helm values
   helm upgrade unified-replication-operator ./helm/unified-replication-operator \
     -n unified-replication-system \
     --set imagePullSecrets[0].name=regcred
   ```

2. **Verify image exists:**
   ```bash
   podman pull quay.io/myuser/unified-replication-operator:0.1.0
   ```

3. **Check image is public or credentials are correct**

### Build Failures

If the build fails:

```bash
# Check Go version
go version  # Should be 1.24+

# Clean and rebuild
make clean
make build

# Try building image separately
podman build -t test .
```

### Registry Login Issues

For OpenShift internal registry:

```bash
# Get token
TOKEN=$(oc create token unified-replication-operator -n unified-replication-system --duration=1h)

# Login
REGISTRY=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
podman login -u unused -p $TOKEN $REGISTRY --tls-verify=false
```

### Webhook Certificate Issues

If you see webhook cert errors, the script skips webhook cert generation. To fix:

```bash
# Create certificates manually
./scripts/create-webhook-cert.sh

# Or disable webhooks temporarily
helm upgrade unified-replication-operator ./helm/unified-replication-operator \
  -n unified-replication-system \
  --set webhook.enabled=false
```

## Manual Build Steps

If you prefer manual control:

```bash
# 1. Build binary
make build

# 2. Run tests
make test-unit

# 3. Build image
podman build -t quay.io/myuser/unified-replication-operator:0.1.0 .

# 4. Push image
podman push quay.io/myuser/unified-replication-operator:0.1.0

# 5. Deploy
export KUBECONFIG=/path/to/kubeconfig
helm upgrade --install unified-replication-operator ./helm/unified-replication-operator \
  --namespace unified-replication-system \
  --create-namespace \
  --set image.repository=quay.io/myuser/unified-replication-operator \
  --set image.tag=0.1.0 \
  --set openshift.compatibleSecurity=true
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build and Push

on:
  push:
    branches: [ main ]
    tags: [ 'v*' ]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.24'
    
    - name: Login to Quay.io
      run: echo "${{ secrets.QUAY_PASSWORD }}" | podman login quay.io -u "${{ secrets.QUAY_USERNAME }}" --password-stdin
    
    - name: Build and Push
      env:
        REGISTRY: quay.io/${{ secrets.QUAY_USERNAME }}
        VERSION: ${{ github.ref_name }}
      run: ./scripts/build-and-push.sh
```

### GitLab CI Example

```yaml
build:
  image: golang:1.24
  services:
    - docker:dind
  script:
    - apt-get update && apt-get install -y podman
    - echo "$CI_REGISTRY_PASSWORD" | podman login -u "$CI_REGISTRY_USER" --password-stdin "$CI_REGISTRY"
    - REGISTRY=$CI_REGISTRY_IMAGE VERSION=$CI_COMMIT_TAG ./scripts/build-and-push.sh
  only:
    - tags
```

## Getting Help

```bash
# Show help
./scripts/build-and-push.sh --help

# Check prerequisites
./scripts/build-and-push.sh  # Will show what's missing
```

## See Also

- [README.md](README.md) - General operator documentation
- [OPENSHIFT_INSTALL.md](OPENSHIFT_INSTALL.md) - OpenShift-specific installation
- [Makefile](Makefile) - Available make targets

