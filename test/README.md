# Test Organization

## Overview

This document explains the testing structure for the Unified Replication Operator. Tests are organized following Go best practices and divided into multiple categories.

## Test Directory Structure

```
test/
├── adapters/               # Adapter compliance and integration tests
│   ├── compliance_test.go  # Interface compliance tests
│   ├── fault_tolerance_test.go # Failure injection tests
│   ├── state_transition_test.go # State machine tests
│   └── README.md          # Testing framework documentation
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

### 2. Compliance Tests
**Location:** `test/adapters/`  
**Purpose:** Validate adapter interface compliance  
**Run:** `go test ./test/adapters/...`  
**Count:** 25 test functions

**Examples:**
- `test/adapters/compliance_test.go` - Interface compliance
- `test/adapters/fault_tolerance_test.go` - Failure injection

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

# Adapter tests
go test ./test/adapters/...

# Controller tests
go test ./controllers/...

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

## Continuous Integration

### GitHub Actions
```yaml
- name: Unit Tests
  run: go test -short ./...


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

2. **Update test/adapters** if new adapter:
   - Add to compliance tests
   - Add to cross-backend tests

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

