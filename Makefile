# Image URL to use all building/pushing image targets
IMG ?= unified-replication-operator:latest
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.30.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd paths="./..." output:crd:artifacts:config=config/crd/bases
	@echo "✅ Generated CRDs for both v1alpha1 and v1alpha2"

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	@echo "✅ Generated deepcopy code for both v1alpha1 and v1alpha2"

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint against code.
	$(GOLANGCI_LINT) run

.PHONY: test
test: test-unit ## Run unit tests (default)

##@ Testing

.PHONY: test-unit
test-unit: fmt vet ## Run unit tests with coverage
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./api/... ./pkg/... ./controllers/...
	@echo "Coverage report generated: coverage.out"

.PHONY: test-integration
test-integration: manifests generate envtest ## Run integration tests
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test -v ./test/integration/...

.PHONY: test-e2e
test-e2e: fmt vet generate ## Run end-to-end tests
	@echo "E2E tests not yet implemented"

.PHONY: test-all
test-all: test-unit test-integration ## Run all tests

.PHONY: test-fixtures
test-fixtures: ## Validate test fixtures
	go test -v ./test/fixtures/...

.PHONY: test-utils
test-utils: ## Test utility functions
	go test -v ./test/utils/...

.PHONY: test-setup
test-setup: $(ENVTEST) ## Setup test environment
	$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN)
	@echo "Test environment setup complete"

.PHONY: test-v1alpha2
test-v1alpha2: fmt vet ## Run v1alpha2 specific tests
	go test -v ./api/v1alpha2/...
	go test -v ./controllers/... -run "VolumeReplication|VolumeGroupReplication|BackendDetection"
	go test -v ./pkg/adapters/... -run "V1Alpha2|Translation"
	@echo "✅ v1alpha2 tests passed"

.PHONY: test-translation
test-translation: ## Run translation logic tests
	go test -v ./pkg/adapters/... -run "Translation"
	@echo "✅ Translation tests passed"

.PHONY: test-backend-detection
test-backend-detection: ## Run backend detection tests
	go test -v ./controllers/... -run "BackendDetection"
	@echo "✅ Backend detection tests passed"

##@ Coverage Reporting

.PHONY: coverage
coverage: test-unit ## Generate coverage report
	go tool cover -func=coverage.out

.PHONY: coverage-html
coverage-html: test-unit ## Generate HTML coverage report
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report generated: coverage.html"

.PHONY: coverage-report
coverage-report: test-unit ## Generate detailed coverage report with quality gate
	@go tool cover -func=coverage.out | grep "total:" | awk '{print "Total coverage: " $$3}' | tee coverage-summary.txt
	@COVERAGE=$$(go tool cover -func=coverage.out | grep "total:" | awk '{print $$3}' | sed 's/%//'); \
	echo "Coverage: $$COVERAGE%"; \
	if [ $$(echo "$$COVERAGE" | cut -d. -f1) -lt 80 ]; then \
		echo "❌ Coverage $$COVERAGE% is below the required 80% threshold"; \
		exit 1; \
	else \
		echo "✅ Coverage $$COVERAGE% meets the required 80% threshold"; \
	fi

##@ Benchmarking

.PHONY: benchmark
benchmark: benchmark-validation ## Run performance benchmarks

.PHONY: benchmark-validation
benchmark-validation: ## Run validation performance benchmarks
	go test -bench=BenchmarkValidation -benchmem ./test/benchmarks/...

.PHONY: benchmark-crud
benchmark-crud: ## Run CRUD performance benchmarks  
	go test -bench=BenchmarkCRD -benchmem ./test/benchmarks/...

.PHONY: benchmark-all
benchmark-all: ## Run all performance benchmarks
	go test -bench=. -benchmem ./test/benchmarks/...

.PHONY: smoke-test
smoke-test: ## Run basic smoke test to verify project builds.
	@echo "Running smoke test..."
	go build -o /tmp/unified-replication-operator main.go
	@echo "✅ Project builds successfully"

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/dev-best-practices/
.PHONY: docker-build
docker-build: test ## Build docker image with the manager.
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have a multi-arch builder. More info: https://docs.docker.com/build/building/multi-platform/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: test ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- docker buildx create --name project-v3-builder
	docker buildx use project-v3-builder
	- docker buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- docker buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Samples (v1alpha2)

.PHONY: deploy-samples-v1alpha2
deploy-samples-v1alpha2: ## Deploy v1alpha2 sample resources
	@echo "Deploying v1alpha2 sample resources..."
	kubectl apply -f config/samples/volumereplicationclass_ceph.yaml
	kubectl apply -f config/samples/volumereplication_ceph_primary.yaml
	@echo "✅ v1alpha2 samples deployed"

.PHONY: deploy-samples-all
deploy-samples-all: ## Deploy all v1alpha2 sample resources (all backends)
	@echo "Deploying all v1alpha2 samples..."
	kubectl apply -f config/samples/volumereplicationclass_ceph.yaml
	kubectl apply -f config/samples/volumereplicationclass_trident.yaml
	kubectl apply -f config/samples/volumereplicationclass_powerstore.yaml
	kubectl apply -f config/samples/volumereplication_ceph_primary.yaml
	kubectl apply -f config/samples/volumereplication_trident_secondary.yaml
	kubectl apply -f config/samples/volumereplication_powerstore_primary.yaml
	@echo "✅ All single volume samples deployed"

.PHONY: deploy-samples-groups
deploy-samples-groups: ## Deploy volume group sample resources
	@echo "Deploying volume group samples..."
	kubectl apply -f config/samples/volumegroupreplicationclass_ceph_group.yaml
	kubectl apply -f config/samples/volumegroupreplication_postgresql.yaml
	@echo "✅ Volume group samples deployed"

.PHONY: undeploy-samples
undeploy-samples: ## Remove all sample resources
	@echo "Removing sample resources..."
	kubectl delete vr --all --all-namespaces --ignore-not-found=true
	kubectl delete vgr --all --all-namespaces --ignore-not-found=true
	kubectl delete vrc --all --ignore-not-found=true
	kubectl delete vgrc --all --ignore-not-found=true
	@echo "✅ Samples removed"

##@ Security

.PHONY: security-scan
security-scan: gosec ## Run security scan with gosec.
	$(GOSEC) ./...

.PHONY: dependency-check
dependency-check: ## Check for known vulnerabilities in dependencies.
	go list -json -deps ./... | nancy sleuth

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GOSEC ?= $(LOCALBIN)/gosec

## Tool Versions
KUSTOMIZE_VERSION ?= v5.0.4-0.20230601165947-6ce0bf390ce3
CONTROLLER_TOOLS_VERSION ?= v0.16.5
GOLANGCI_LINT_VERSION ?= v1.54.2
GOSEC_VERSION ?= v2.18.2

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.17

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	test -s $(LOCALBIN)/golangci-lint && $(LOCALBIN)/golangci-lint --version | grep -q $(GOLANGCI_LINT_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: gosec
gosec: $(GOSEC) ## Download gosec locally if necessary.
$(GOSEC): $(LOCALBIN)
	test -s $(LOCALBIN)/gosec && $(LOCALBIN)/gosec --version | grep -q $(GOSEC_VERSION) || \
	GOBIN=$(LOCALBIN) go install github.com/securecodewarrior/gosec/v2/cmd/gosec@$(GOSEC_VERSION)
