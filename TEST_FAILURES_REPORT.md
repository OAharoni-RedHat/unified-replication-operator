# Test Failures Report

Generated: October 10, 2025

## Summary

Total test packages: 12
- **Passed**: 7
- **Failed**: 5
- **Build Errors**: 2

## Failed Tests

### 1. pkg/adapters - TestMockTridentAdapter/GetReplicationStatus

**Status**: FAILED (Panic)

**Location**: `pkg/adapters/mock_adapters_test.go:189`

**Error Type**: Runtime panic - nil pointer dereference

**Details**:
```
panic: runtime error: invalid memory address or nil pointer dereference
[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x176e092]
```

**Root Cause**: 
The test uses `DefaultMockTridentConfig()` which has a `StatusSuccessRate` of 0.99 (99% success rate). This means 1% of status calls will fail randomly. The test doesn't account for this possibility.

When the simulated failure occurs, the implementation at `mock_trident.go:280` returns:
```go
return nil, NewAdapterError(...)
```

The test then tries to access fields on the nil status object at line 189, causing the panic.

**Code References**:
- `mock_trident.go:278-281`: Failure simulation check
- `mock_trident.go:84`: `StatusSuccessRate: 0.99`
- `mock_adapters_test.go:189`: Nil pointer dereference

**Fix Required**: 
Either:
1. Set `StatusSuccessRate: 1.0` in the test config to ensure success
2. Update the test to handle potential errors with error checking before accessing status fields
3. Run the test multiple times or with retries to handle the 1% failure rate

---

### 2. pkg/discovery - TestCapabilityIntegration

**Status**: FAILED (Infrastructure)

**Location**: `pkg/discovery/capabilities_integration_test.go:43`

**Error**: 
```
fork/exec /usr/local/kubebuilder/bin/etcd: no such file or directory
unable to start control plane itself: failed to start the controlplane. retried 5 times
```

**Root Cause**: 
The test requires kubebuilder's envtest infrastructure with an etcd binary at `/usr/local/kubebuilder/bin/etcd`, which is not installed or not at the expected path.

**Fix Required**: 
- Install kubebuilder test binaries
- Set `KUBEBUILDER_ASSETS` environment variable to point to test binaries
- Or skip this test when infrastructure is not available

---

### 3. test/fixtures - TestFailureConditions

**Status**: FAILED (Assertion)

**Location**: `test/fixtures/samples_test.go:180`

**Error**:
```
"Source and destination are out of sync" does not contain "fail"
```

**Root Cause**: 
The test at line 180 expects all failure condition messages to contain the word "fail":
```go
assert.Contains(t, condition.Message, "fail", "Failure condition message should indicate failure")
```

However, in `samples.go:203`, the second failure condition has the message:
```
"Source and destination are out of sync"
```

This message doesn't contain "fail" (the first condition at line 196 does contain "failed").

**Code References**:
- `samples.go:190-207`: FailureConditions() function
- `samples.go:196`: First condition message contains "failed" ✓
- `samples.go:203`: Second condition message lacks "fail" ✗
- `samples_test.go:180`: Test assertion checking for "fail"

**Fix Required**: 
Change line 203 in `samples.go` from:
```go
Message: "Source and destination are out of sync",
```
to:
```go
Message: "Source and destination sync failed - out of sync",
```
or similar message that includes the word "fail".

---

### 4. test/fixtures - TestStateTransitionScenarios (Multiple sub-tests)

**Status**: FAILED (Assertions)

**Location**: `test/fixtures/samples_test.go:251-253`

**Failed Sub-tests**:

#### a) promoting-to-source
```
Error: "Promotion completes to source state" does not contain "can"
Message: Valid transition reason should indicate permission
```

#### b) demoting-to-replica
```
Error: "Demotion completes to replica state" does not contain "can"
Message: Valid transition reason should indicate permission
```

#### c) promoting-to-replica
```
Error: "Cannot go from promoting directly to replica" does not contain "cannot"
Message: Invalid transition reason should indicate restriction
```

**Root Cause**: 
The test at `samples_test.go:251-253` expects:
- Valid transitions should contain "can" (lowercase)
- Invalid transitions should contain "cannot" (lowercase)

However, the actual reasons in `samples.go` don't follow this pattern:

**Detailed Issues**:

1. **promoting-to-source** (line 445): 
   - Reason: "Promotion completes to source state"
   - Valid: true
   - Missing: "can" keyword ✗

2. **demoting-to-replica** (line 451):
   - Reason: "Demotion completes to replica state"
   - Valid: true
   - Missing: "can" keyword ✗

3. **promoting-to-replica** (line 469):
   - Reason: "Cannot go from promoting directly to replica"
   - Valid: false
   - Has: "Cannot" with capital C, test checks for lowercase "cannot" ✗

**Code References**:
- `samples.go:417-478`: StateTransitionScenarios() function
- `samples.go:445`: promoting-to-source reason
- `samples.go:451`: demoting-to-replica reason
- `samples.go:469`: promoting-to-replica reason (case mismatch)
- `samples_test.go:251`: Assertion checking for "can"
- `samples_test.go:253`: Assertion checking for "cannot"

**Fix Required**: 
Update `samples.go` lines:
- Line 445: Change to `"Promotion can complete to source state"`
- Line 451: Change to `"Demotion can complete to replica state"`
- Line 469: Change to `"cannot go from promoting directly to replica"` (lowercase c)

Or make the test case-insensitive by using `strings.ToLower()` before checking.

---

### 5. test/integration - Build Failed

**Status**: BUILD FAILED (Compilation Error)

**Location**: `test/integration/unifiedvolumereplication_test.go:209-210`

**Error**: 
```
test/integration/unifiedvolumereplication_test.go:209:52: 
  createdUVR.Spec.Extensions.Trident.Actions undefined 
  (type *TridentExtensions has no field or method Actions)
test/integration/unifiedvolumereplication_test.go:210:70: 
  createdUVR.Spec.Extensions.Trident.Actions undefined 
  (type *TridentExtensions has no field or method Actions)
```

**Root Cause**: 
The test code at lines 209-210 references `createdUVR.Spec.Extensions.Trident.Actions`, but the `TridentExtensions` struct in the API doesn't have an `Actions` field. This suggests:
1. The API was changed and removed the `Actions` field
2. The test wasn't updated to match the API changes
3. The field may have been renamed or restructured

**Fix Required**: 
1. Check the current structure of `TridentExtensions` in `api/v1alpha1/unifiedvolumereplication_types.go`
2. Update lines 209-210 in `test/integration/unifiedvolumereplication_test.go` to use the correct field name
3. If `Actions` was removed, update the test to remove references to it or use the replacement field

---

### 6. test/utils - Build Failed

**Status**: BUILD FAILED (Compilation Error)

**Location**: `test/utils/crd_helpers_test.go:73`

**Error**: 
```
test/utils/crd_helpers_test.go:73:26: undefined: actions
```

**Root Cause**: 
The test code at line 73 references a variable or function named `actions` that is not defined in scope. This could be:
1. A variable that was removed or renamed
2. A missing import
3. A function that was deleted
4. Part of the same API changes that affected test/integration (Actions field removal)

**Fix Required**: 
1. Examine line 73 of `test/utils/crd_helpers_test.go` to see what `actions` refers to
2. If it's related to the `Actions` field removal, update or remove the reference
3. If it's a missing variable, define it or import it from the correct location
4. Check if this is related to the TridentExtensions.Actions field issue

---

## Passing Test Packages

The following test packages passed successfully:

1. ✅ **Root package** (main) - 3 tests passed
2. ✅ **api/v1alpha1** - 6 tests passed (79 sub-tests)
3. ✅ **controllers** - 7 tests passed (82 sub-tests)
4. ✅ **pkg** (controller_engine) - 8 tests passed
5. ✅ **pkg/adapters** (partial) - Most tests passed except GetReplicationStatus
6. ✅ **pkg/discovery** (partial) - Unit tests passed, integration test failed due to infrastructure
7. ✅ **test/e2e** - 4 tests passed (10 sub-tests)

## Test Coverage by Category

### Adapter Tests
- **Status**: Mostly passing, 1 critical failure
- **Issue**: Mock adapter GetReplicationStatus panic

### Discovery Tests
- **Status**: Unit tests pass, integration tests fail
- **Issue**: Missing test infrastructure (etcd)

### Controller Tests
- **Status**: All passing ✅
- **Coverage**: State machine, retry logic, integration

### Fixture/Sample Tests
- **Status**: Partially failing
- **Issue**: String assertion mismatches in state transition messages

### Build Tests
- **Status**: Failing
- **Issue**: Compilation errors in integration and utils packages

## Recommendations

### Priority 1 (Critical)
1. Fix nil pointer dereference in `TestMockTridentAdapter/GetReplicationStatus`
2. Fix compilation errors in test/integration and test/utils packages

### Priority 2 (High)
3. Update state transition messages in samples.go to match test expectations
4. Fix failure condition message assertions

### Priority 3 (Medium)
5. Set up kubebuilder test environment or make integration tests skippable when infrastructure is unavailable

### Priority 4 (Low)
6. Review and standardize message formats across all state transitions and conditions

## Notes

- Tests marked as "DISABLED" or "SKIP" were intentionally skipped (e.g., `TestCephAdapterIntegration_DISABLED`, `TestCrossBackendCompatibility/AllSupportInitialization_DISABLED`)
- Performance and load tests all passed successfully, showing good system stability
- The adapter cross-backend compatibility tests demonstrate good abstraction design
- State machine and transition tests show the system properly handles complex state changes

## Next Steps

1. Run individual failing tests to get detailed error messages
2. Check for recent code changes that may have introduced the failures
3. Review git diff to understand what changed in the modified files
4. Fix compilation errors first (blocking other tests)
5. Then address runtime failures
6. Finally, adjust test assertions to match implementation

