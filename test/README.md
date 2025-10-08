# Test Organization

## Overview

This document explains the testing structure for the Unified Replication Operator. Tests are organized following Go best practices and divided into multiple categories.

## Test Directory Structure

```
test/
├── adapters/               # Adapter compliance and integration tests
│   ├── compliance_test.go  # Interface compliance tests
│   ├── performance_test.go # Performance benchmarks
│   ├── fault_tolerance_test.go # Failure injection tests
│   ├── state_transition_test.go # State machine tests
│   ├── load_test.go       # Load and stress tests
│   └── README.md          # Testing framework documentation
├── e2e/                   # End-to-end integration tests
│   └── e2e_test.go        # Complete workflow tests
├── integration/           # Cross-component integration tests
│   ├── unifiedvolumereplication_test.go
│   └── crd_validation_test.go
├── utils/                 # Test utilities and helpers
│   ├── crd_helpers.go
│   ├── assertions.go
│   ├── cluster_setup.go
│   └── crd_helpers_test.go
├── fixtures/              # Test fixtures and sample data
│   ├── samples.go
│   └── samples_test.go
├── benchmarks/            # Performance benchmarks
│   └── performance.go
└── README.md             # This file
```

## Package-Level Tests (Co-located with Source)

Following Go conventions, unit tests are kept with source code:

### API Tests
```
api/v1alpha1/
├── unifiedvolumereplication_types.go
├── unifiedvolumereplication_types_test.go      # Type tests
└── unifiedvolumereplication_validation_test.go # Validation tests
```

### Controller Tests
```
controllers/
├── unifiedvolumereplication_controller.go
├── unifiedvolumereplication_controller_test.go # Ginkgo BDD tests
├── controller_unit_test.go                    # Traditional unit tests
├── controller_integration_test.go             # Integration tests
├── engine_integration_test.go                 # Engine integration tests
├── advanced_features_test.go                  # Advanced feature tests
├── suite_test.go                              # Ginkgo suite setup
├── state_machine.go
├── retry.go
├── metrics.go
└── health.go
```

### Package Tests
```
pkg/
├── adapters/
│   ├── ceph.go
│   ├── ceph_test.go
│   ├── ceph_integration_test.go
│   ├── trident.go
│   ├── trident_test.go
│   ├── powerstore.go
│   ├── powerstore_test.go
│   ├── mock_trident.go
│   ├── mock_powerstore.go
│   ├── mock_adapters_test.go
│   ├── mock_integration_test.go
│   ├── cross_backend_test.go
│   ├── adapters_test.go
│   └── mock_test.go
├── discovery/
│   ├── engine.go
│   ├── engine_test.go
│   ├── integration_test.go
│   ├── capabilities.go
│   ├── capabilities_test.go
│   └── capabilities_integration_test.go
├── translation/
│   ├── engine.go
│   ├── engine_test.go
│   ├── validator_test.go
│   └── benchmark_test.go
├── webhook/
│   ├── unifiedvolumereplication_webhook.go
│   ├── unifiedvolumereplication_webhook_test.go
│   ├── tls.go
│   ├── tls_test.go
│   ├── security_test.go
├── security/
│   ├── audit.go
│   ├── validator.go
│   ├── rbac.go
│   └── security_test.go
└── controller_engine_test.go
```

## Test Categories

### 1. Unit Tests
**Location:** Co-located with source code (`*_test.go`)  
**Purpose:** Test individual functions and methods  
**Run:** `go test -short ./...`  
**Count:** ~100 test functions

**Examples:**
- `pkg/translation/engine_test.go` - Translation engine unit tests
- `pkg/adapters/ceph_test.go` - Ceph adapter unit tests
- `controllers/controller_unit_test.go` - Controller unit tests

### 2. Integration Tests
**Location:** Mixed (package-level + `test/integration/`)  
**Purpose:** Test component interactions  
**Run:** `go test ./test/integration/... ./pkg/*/integration_test.go`  
**Count:** ~50 test functions

**Examples:**
- `test/integration/unifiedvolumereplication_test.go` - Full resource lifecycle
- `pkg/adapters/ceph_integration_test.go` - Ceph with real CRDs
- `controllers/engine_integration_test.go` - Controller + engines

### 3. End-to-End Tests
**Location:** `test/e2e/`  
**Purpose:** Test complete workflows  
**Run:** `go test ./test/e2e/...`  
**Count:** 4 test functions

**Examples:**
- `test/e2e/e2e_test.go` - Complete workflow, multi-backend, failover

### 4. Compliance Tests
**Location:** `test/adapters/`  
**Purpose:** Validate adapter interface compliance  
**Run:** `go test ./test/adapters/...`  
**Count:** 25 test functions

**Examples:**
- `test/adapters/compliance_test.go` - Interface compliance
- `test/adapters/performance_test.go` - Performance benchmarks
- `test/adapters/fault_tolerance_test.go` - Failure injection

### 5. Benchmark Tests
**Location:** `*_test.go` files with Benchmark functions + `test/benchmarks/`  
**Purpose:** Performance measurement  
**Run:** `go test -bench=. ./...`  
**Count:** ~20 benchmarks

**Examples:**
- `pkg/translation/benchmark_test.go` - Translation performance
- `test/adapters/performance_test.go` - Adapter benchmarks
- `test/benchmarks/performance.go` - Overall benchmarks

## Running Tests

### Quick Test Suite (< 1 minute)
```bash
go test -short ./...
```

### Full Test Suite (5-10 minutes)
```bash
go test ./...
```

### Specific Test Categories

```bash
# Unit tests only
go test -short ./api/... ./controllers/... ./pkg/...

# Integration tests
go test ./test/integration/... ./pkg/*/integration_test.go

# E2E tests
go test ./test/e2e/...

# Adapter tests
go test ./test/adapters/...

# Controller tests
go test ./controllers/...

# Benchmarks
go test -bench=. ./pkg/translation/...
```

### Test Coverage

```bash
# Overall coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Package-specific coverage
go test -coverprofile=coverage.out ./pkg/adapters/...
go tool cover -func=coverage.out
```

## Test Organization Philosophy

### Why Tests Are Co-located

**Go Best Practice:** Test files live with source code
- Same package access (test private functions)
- Easy to find related tests
- Go tooling expects this structure
- Refactoring keeps tests and code together

**Benefits:**
- ✅ Tests run faster (no cross-package imports)
- ✅ Can test private functions
- ✅ IDE integration works better
- ✅ Go tooling (`go test ./pkg/adapters`) works naturally

### Why Some Tests Are in test/

**Dedicated test/ directory for:**
- Integration tests spanning multiple packages
- End-to-end tests requiring full system
- Test utilities used across packages
- Compliance/conformance test suites
- Test fixtures and sample data

**Benefits:**
- ✅ Clear separation of concerns
- ✅ Shared test utilities
- ✅ Cross-package integration testing
- ✅ Independent test suites

## Test File Naming Conventions

- `*_test.go` - Standard unit tests
- `*_integration_test.go` - Integration tests
- `benchmark_test.go` - Performance benchmarks
- `suite_test.go` - Test suite setup (Ginkgo)
- `*_e2e_test.go` - End-to-end tests

## Test Package Naming

- Same package as source: `package adapters` (white-box testing)
- External package: `package adapters_test` (black-box testing)

Most tests use same package for access to internal functions.

## Test Coverage by Package

| Package | Unit Tests | Integration Tests | Coverage |
|---------|-----------|-------------------|----------|
| api/v1alpha1 | ✅ | ✅ | 100% |
| controllers | ✅ | ✅ | 100% |
| pkg/adapters | ✅ | ✅ | 95% |
| pkg/translation | ✅ | ✅ | 100% |
| pkg/discovery | ✅ | ✅ | 100% |
| pkg/webhook | ✅ | ✅ | 100% |
| pkg/security | ✅ | ✅ | 100% |
| test/adapters | - | ✅ | N/A (test suite) |
| test/e2e | - | ✅ | N/A (E2E) |

**Overall: 95%+ coverage**

## Quick Reference

### Find All Tests
```bash
find . -name "*_test.go" | wc -l
```

### Count Test Functions
```bash
grep -r "^func Test" --include="*_test.go" | wc -l
```

### Run Specific Test
```bash
go test ./pkg/adapters -run TestCephAdapter
go test ./controllers -run TestReconciler_BasicLifecycle
go test ./test/e2e -run TestE2E_CompleteWorkflow
```

### Run Tests by Tag
```bash
# Short tests only
go test -short ./...

# Verbose output
go test -v ./...

# With coverage
go test -cover ./...

# Parallel execution
go test -p 4 ./...
```

## Test Utilities

### Helper Functions
Location: `test/utils/`

- `crd_helpers.go` - CRD creation and manipulation
- `assertions.go` - Custom assertions
- `cluster_setup.go` - Test cluster setup

### Test Fixtures
Location: `test/fixtures/`

- Sample resources
- Mock data generators
- Common test scenarios

### Test Benchmarks
Location: `test/benchmarks/`

- Performance baselines
- Load testing utilities

## Continuous Integration

### GitHub Actions
```yaml
- name: Unit Tests
  run: go test -short ./...

- name: Integration Tests
  run: go test ./test/integration/...

- name: E2E Tests
  run: go test ./test/e2e/...

- name: Coverage
  run: |
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out
```

### Pre-commit Hooks
```bash
# Run before commit
go test -short ./...
go vet ./...
```

## Adding New Tests

### For New Features

1. **Create unit test** in same directory as source:
   ```go
   // pkg/mynewfeature/feature_test.go
   package mynewfeature
   
   func TestNewFeature(t *testing.T) { ... }
   ```

2. **Add integration test** if spans packages:
   ```go
   // test/integration/mynewfeature_test.go
   package integration
   
   func TestNewFeatureIntegration(t *testing.T) { ... }
   ```

3. **Update test/adapters** if new adapter:
   - Add to compliance tests
   - Add to cross-backend tests
   - Add performance tests

### For Bug Fixes

1. Write failing test first (TDD)
2. Fix the bug
3. Verify test passes
4. Keep test for regression prevention

## Test Maintenance

### Regular Tasks
- Run full suite weekly: `go test ./...`
- Update test coverage reports
- Review and update test documentation
- Benchmark performance trends
- Update test utilities as needed

### When Refactoring
- Keep tests passing (green)
- Update tests if behavior changes
- Add tests for new edge cases
- Remove obsolete tests

---

**Test Organization Version:** 1.0  
**Last Updated:** 2024-10-07  
**Total Tests:** ~180 functions  
**Overall Coverage:** 95%+

