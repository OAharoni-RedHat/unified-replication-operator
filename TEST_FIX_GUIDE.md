# Test Fix Guide

This document provides step-by-step instructions to fix all failing tests.

---

## Fix #1: Mock Adapter Nil Pointer Panic (Critical)

**File**: `pkg/adapters/mock_adapters_test.go`  
**Lines**: 174-175  
**Failure**: Random panic due to simulated failure in GetReplicationStatus

### Current Code (Line 174):
```go
config := DefaultMockTridentConfig()
adapter := NewMockTridentAdapter(client, translator, config)
```

### Fixed Code:
```go
config := &MockTridentConfig{
    CreateSuccessRate: 1.0,
    UpdateSuccessRate: 1.0,
    DeleteSuccessRate: 1.0,
    StatusSuccessRate: 1.0,  // Changed from 0.99 to ensure 100% success
    MinLatency:        0,
    MaxLatency:        0,
}
adapter := NewMockTridentAdapter(client, translator, config)
```

### Why:
The `DefaultMockTridentConfig()` sets `StatusSuccessRate: 0.99`, meaning 1% of calls fail. When the failure occurs, it returns `nil` for status, and the test tries to access fields on the nil object, causing a panic. Setting it to 1.0 ensures deterministic test behavior.

---

## Fix #2: Integration Test Compilation Error (Critical)

**File**: `test/integration/unifiedvolumereplication_test.go`  
**Lines**: 209-210  
**Failure**: Undefined field `Actions` in TridentExtensions

### Current Code (Lines 208-210):
```go
require.NotNil(t, createdUVR.Spec.Extensions.Trident)
require.Len(t, createdUVR.Spec.Extensions.Trident.Actions, 1)
assert.Equal(t, "mirror-update", createdUVR.Spec.Extensions.Trident.Actions[0].Type)
```

### Fixed Code:
```go
require.NotNil(t, createdUVR.Spec.Extensions.Trident)
// Actions field has been removed from TridentExtensions as it's now empty
// If Trident-specific fields are needed in the future, they can be added to TridentExtensions
```

### Why:
The `TridentExtensions` struct is now empty (see `api/v1alpha1/unifiedvolumereplication_types.go:157-158`). The `Actions` field no longer exists. Since TridentExtensions is currently just a placeholder for future extensions, we should remove the assertions about Actions.

### Alternative Fix (if Actions are still needed):
If the test data creation also includes Actions, you'll need to remove that too. Check around line 180-200 for where the test UVR is created and remove any Actions initialization.

---

## Fix #3: Utils Test Compilation Error (Critical)

**File**: `test/utils/crd_helpers_test.go`  
**Line**: 73  
**Failure**: Undefined variable `actions`

### Current Code (Lines 71-73):
```go
uvr := NewCRDBuilder().
    WithCephExtensions("journal", &startTime).
    WithTridentExtensions(actions).
    WithPowerStoreExtensions("Five_Minutes", []string{"group1"}).
    Build()
```

### Fixed Code:
```go
uvr := NewCRDBuilder().
    WithCephExtensions("journal", &startTime).
    WithTridentExtensions().  // Remove the actions parameter
    WithPowerStoreExtensions("Five_Minutes", []string{"group1"}).
    Build()
```

### Additionally Fix (File: `test/utils/crd_helpers.go`, Line 187):

**Current Code:**
```go
func (b *CRDBuilder) WithTridentExtensions(actions interface{}) *CRDBuilder {
    if b.uvr.Spec.Extensions == nil {
        b.uvr.Spec.Extensions = &replicationv1alpha1.Extensions{}
    }
    b.uvr.Spec.Extensions.Trident = &replicationv1alpha1.TridentExtensions{}
    return b
}
```

**Fixed Code:**
```go
func (b *CRDBuilder) WithTridentExtensions() *CRDBuilder {  // Remove actions parameter
    if b.uvr.Spec.Extensions == nil {
        b.uvr.Spec.Extensions = &replicationv1alpha1.Extensions{}
    }
    b.uvr.Spec.Extensions.Trident = &replicationv1alpha1.TridentExtensions{}
    return b
}
```

### Why:
Since `TridentExtensions` is now empty, the `actions` parameter is no longer needed or used. The function signature should be updated to remove this unused parameter, and all calls to this function should be updated accordingly.

---

## Fix #4a: Failure Condition Message (Medium Priority)

**File**: `test/fixtures/samples.go`  
**Line**: 203  
**Failure**: Message doesn't contain "fail"

### Current Code (Line 203):
```go
Message: "Source and destination are out of sync",
```

### Fixed Code:
```go
Message: "Synchronization failed - source and destination are out of sync",
```

### Why:
The test at `samples_test.go:180` expects all failure condition messages to contain the word "fail". This makes messages more consistent and immediately identifiable as failure messages.

---

## Fix #4b: State Transition Reason - promoting-to-source (Medium Priority)

**File**: `test/fixtures/samples.go`  
**Line**: 445  
**Failure**: Valid transition reason doesn't contain "can"

### Current Code (Line 445):
```go
Reason: "Promotion completes to source state",
```

### Fixed Code:
```go
Reason: "Promotion can complete to source state",
```

---

## Fix #4c: State Transition Reason - demoting-to-replica (Medium Priority)

**File**: `test/fixtures/samples.go`  
**Line**: 451  
**Failure**: Valid transition reason doesn't contain "can"

### Current Code (Line 451):
```go
Reason: "Demotion completes to replica state",
```

### Fixed Code:
```go
Reason: "Demotion can complete to replica state",
```

---

## Fix #4d: State Transition Reason - promoting-to-replica (Medium Priority)

**File**: `test/fixtures/samples.go`  
**Line**: 469  
**Failure**: Invalid transition reason has uppercase "Cannot" instead of lowercase "cannot"

### Current Code (Line 469):
```go
Reason: "Cannot go from promoting directly to replica",
```

### Fixed Code:
```go
Reason: "cannot go from promoting directly to replica",
```

### Why:
The test uses case-sensitive string matching with `assert.Contains()`. Valid transitions should contain lowercase "can", and invalid transitions should contain lowercase "cannot". This provides consistent pattern matching for automated validation.

---

## Fix #5: Missing Test Infrastructure (Optional)

**Issue**: TestCapabilityIntegration fails due to missing etcd binary  
**File**: `pkg/discovery/capabilities_integration_test.go:43`

### Option 1: Install kubebuilder test tools
```bash
# Download and install kubebuilder test tools
make install-kubebuilder-test-tools
```

### Option 2: Set environment variable
```bash
export KUBEBUILDER_ASSETS=/path/to/kubebuilder/testbin/bin
```

### Option 3: Skip when unavailable (already implemented)
The test already has skip logic for when etcd is unavailable. You can leave it as-is if integration tests are not critical for your workflow.

---

## Verification Steps

After applying all fixes, run these commands to verify:

### 1. Verify compilation errors are fixed:
```bash
go test -c ./test/integration/...
go test -c ./test/utils/...
```

Both should exit with code 0 (no errors).

### 2. Run specific failing tests:
```bash
# Test the mock adapter fix
go test -v ./pkg/adapters -run TestMockTridentAdapter/GetReplicationStatus

# Test fixtures
go test -v ./test/fixtures -run TestFailureConditions
go test -v ./test/fixtures -run TestStateTransitionScenarios

# Test integration (after compilation fix)
go test -v ./test/integration -run TestUnifiedVolumeReplication_Extensions
```

### 3. Run full test suite:
```bash
go test ./... -v 2>&1 | tee test_results_after_fix.log
```

### 4. Check for remaining failures:
```bash
grep -E "(FAIL|panic)" test_results_after_fix.log
```

---

## Summary of Changes

| File | Lines | Change Type | Priority |
|------|-------|-------------|----------|
| pkg/adapters/mock_adapters_test.go | 174 | Config change | Critical |
| test/integration/unifiedvolumereplication_test.go | 209-210 | Remove code | Critical |
| test/utils/crd_helpers.go | 187 | Function signature | Critical |
| test/utils/crd_helpers_test.go | 73 | Remove parameter | Critical |
| test/fixtures/samples.go | 203 | Message text | Medium |
| test/fixtures/samples.go | 445 | Message text | Medium |
| test/fixtures/samples.go | 451 | Message text | Medium |
| test/fixtures/samples.go | 469 | Message text | Medium |

---

## Expected Results After Fixes

- ✅ All compilation errors resolved
- ✅ Mock adapter test passes consistently (no random failures)
- ✅ Fixture tests pass with correct string assertions
- ✅ Integration and utils tests compile and run
- ⚠️ Discovery integration test may still skip if etcd not installed (acceptable)

---

## Notes

1. The root cause of most failures is the API change that made `TridentExtensions` an empty struct
2. Tests were not updated when the API changed
3. The mock adapter failure was due to overly realistic simulation (including random failures)
4. String assertion failures are cosmetic but help maintain code quality standards

---

## Rollback Instructions

If any fix causes issues, you can rollback individual changes:

```bash
# Rollback specific file
git checkout HEAD -- <filepath>

# Or rollback all test changes
git checkout HEAD -- test/
git checkout HEAD -- pkg/adapters/mock_adapters_test.go
```

