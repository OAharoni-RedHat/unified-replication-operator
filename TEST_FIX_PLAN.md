# Test Fix Plan - Production Readiness Checklist

**Date:** 2024-10-22  
**Objective:** Fix all failing tests to achieve 100% pass rate  
**Current Status:** 97.5% passing → Target: 100%  
**Priority:** Pre-Production Critical

---

## Overview of Failing Tests

| Priority | Issue | Package | Impact | Effort |
|----------|-------|---------|--------|--------|
| 🔴 HIGH | Integration tests blocked | test/integration, pkg/discovery | Cannot test with real K8s | 30min |
| 🟡 MEDIUM | E2E state translation | test/e2e | Workflow validation incomplete | 1hr |
| 🟡 MEDIUM | Ceph adapter update test | pkg/adapters | Mock client issue | 1hr |
| 🟢 LOW | Schedule mode test | test/fixtures | Test/code mismatch | 15min |
| 🟢 LOW | Test utility cleanup | test/utils | Test isolation issue | 30min |

**Total Estimated Effort:** ~3.5 hours

---

## Fix Prompt #1: Setup Integration Test Environment

### 🔴 PRIORITY: HIGH
### 📦 PACKAGE: `test/integration`, `pkg/discovery`
### ⏱️ ESTIMATED TIME: 30 minutes

### Problem Statement
Integration tests are completely blocked and cannot run because the kubebuilder test environment is not set up. Tests fail with:
```
fork/exec /usr/local/kubebuilder/bin/etcd: no such file or directory
```

This affects:
- `TestCapabilityIntegration` in `pkg/discovery/capabilities_integration_test.go`
- All tests in `test/integration/unifiedvolumereplication_test.go`

### Root Cause
The test environment requires kubebuilder binaries (etcd, kube-apiserver) to create a temporary control plane for testing. These binaries are not present in `/usr/local/kubebuilder/bin/`.

### Fix Plan

**Step 1: Install envtest binaries**

Set up the kubebuilder test environment using the controller-runtime envtest tool:

```bash
# Install setup-envtest tool
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Download and setup test binaries
setup-envtest use --arch amd64 --os linux

# Get the path to the binaries
export KUBEBUILDER_ASSETS="$(setup-envtest use -p path --arch amd64 --os linux)"

# Verify installation
ls -la "$KUBEBUILDER_ASSETS"
# Should show: etcd, kube-apiserver, kubectl
```

**Step 2: Update test execution**

Modify how tests are run to use the KUBEBUILDER_ASSETS environment variable. Update the Makefile target or CI/CD scripts to include:

```makefile
.PHONY: test-integration
test-integration: envtest
	KUBEBUILDER_ASSETS="$(shell setup-envtest use -p path)" go test ./test/integration/... -v

.PHONY: envtest
envtest: setup-envtest
	@echo "Setting up test environment..."
	@setup-envtest use --arch amd64 --os linux

setup-envtest:
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
```

**Step 3: Verify integration tests pass**

Run the integration tests to confirm they now work:

```bash
export KUBEBUILDER_ASSETS="$(setup-envtest use -p path)"
go test ./pkg/discovery -v -run TestCapabilityIntegration
go test ./test/integration/... -v
```

**Step 4: Document the requirement**

Add to README.md or CONTRIBUTING.md:

```markdown
### Running Integration Tests

Integration tests require kubebuilder test binaries:

\`\`\`bash
# Install envtest
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Setup and run tests
export KUBEBUILDER_ASSETS="$(setup-envtest use -p path)"
make test-integration
\`\`\`
```

**Step 5: Add to CI/CD**

Update `.github/workflows/*.yml` or CI configuration to install envtest before running tests.

### Acceptance Criteria
- [ ] `TestCapabilityIntegration` passes
- [ ] All tests in `test/integration/` pass
- [ ] Makefile has `test-integration` target
- [ ] README documents integration test requirements
- [ ] CI/CD installs and uses envtest

### Files to Modify
- `Makefile` - Add envtest setup
- `README.md` or `CONTRIBUTING.md` - Document requirements
- `.github/workflows/test.yml` (if exists) - Add envtest setup
- None of the test files themselves need changes

### Verification Command
```bash
export KUBEBUILDER_ASSETS="$(setup-envtest use -p path)"
go test ./pkg/discovery -v -run TestCapabilityIntegration
go test ./test/integration/... -v
```

---

## Fix Prompt #2: Fix E2E State Translation Test

### 🟡 PRIORITY: MEDIUM
### 📦 PACKAGE: `test/e2e`
### ⏱️ ESTIMATED TIME: 1 hour

### Problem Statement
The E2E complete workflow test fails during state translation validation. The test expects the state to be "source" but receives "failed":

```
Test: TestE2E_CompleteWorkflow/TranslateStates
Expected: "source"
Actual:   "failed"
Location: test/e2e/e2e_test.go:88
```

### Root Cause Analysis

The mock Trident adapter is not properly maintaining state during the test workflow. When a replication is created with state "replica" and then updated to "source", the mock adapter's `GetReplicationStatus()` method returns "failed" instead of the expected "source" state.

Possible reasons:
1. Mock adapter's internal state tracking is incorrect
2. State transition simulation logic has a bug
3. The mock doesn't properly update state when `EnsureReplication` is called
4. State translation between unified → backend → unified is failing

### Fix Plan

**Step 1: Identify the exact flow**

Examine the test to understand what it's doing:
```go
// File: test/e2e/e2e_test.go around line 80-90
// Find where the test creates a replication, updates it, and checks state
```

The test likely:
1. Creates replication with state "replica"
2. Updates to state "source"  
3. Calls GetReplicationStatus()
4. Expects status.State == "source"
5. Gets "failed" instead

**Step 2: Debug the mock adapter**

Add debug logging to understand state flow in the mock adapter:

File: `pkg/adapters/mock_trident.go`

In the `EnsureReplication` method:
- Log when state is set: `fmt.Printf("Setting state: %s\n", uvr.Spec.ReplicationState)`
- Log internal mock state after update

In the `GetReplicationStatus` method:
- Log what state is being returned: `fmt.Printf("Returning state: %s\n", status.State)`
- Log the internal mock replication state

**Step 3: Fix the state tracking**

The issue is likely in one of these places:

**Option A:** Mock adapter not storing state correctly

In `pkg/adapters/mock_trident.go`, method `EnsureReplication`:
- Ensure when a replication is updated, its state in the mock storage is updated
- The mock should have an internal map storing replications by name
- When `EnsureReplication` is called, it should update the stored state

**Option B:** State translation returning "failed"

In `pkg/adapters/mock_trident.go`, method `GetReplicationStatus`:
- Check the state translation logic
- Ensure it translates Trident states → unified states correctly
- May need to fix the translation map or logic

**Option C:** Failure simulation interfering

In `pkg/adapters/mock.go`, check if failure simulation is enabled:
- The mock config may have `SimulateFailures: true`
- This could be causing random "failed" states
- Ensure E2E tests disable failure simulation

**Step 4: Implement the fix**

Based on investigation, the likely fix is:

Update `pkg/adapters/mock_trident.go`:

In `EnsureReplication`, ensure the mock storage updates:
```go
func (mta *MockTridentAdapter) EnsureReplication(ctx context.Context, uvr *UnifiedVolumeReplication) error {
    // ... existing validation ...
    
    // Store/update the replication with current state
    mta.mockReplications[key] = MockReplication{
        Name:      uvr.Name,
        Namespace: uvr.Namespace,
        State:     string(uvr.Spec.ReplicationState),  // ← Ensure this is set
        Mode:      string(uvr.Spec.ReplicationMode),
        // ... other fields
    }
    
    return nil
}
```

In `GetReplicationStatus`, ensure state is read correctly:
```go
func (mta *MockTridentAdapter) GetReplicationStatus(ctx context.Context, uvr *UnifiedVolumeReplication) (*ReplicationStatus, error) {
    // Get stored replication
    mockRepl := mta.mockReplications[key]
    
    // Return status with correct state (don't default to "failed")
    return &ReplicationStatus{
        State:  mockRepl.State,  // ← Use stored state, not hardcoded "failed"
        Mode:   mockRepl.Mode,
        Health: ReplicationHealthHealthy,
        // ...
    }, nil
}
```

**Step 5: Add test case for state persistence**

Add a unit test to verify state persistence in mock adapter:

File: `pkg/adapters/mock_test.go`

```go
func TestMockAdapter_StatePersistence(t *testing.T) {
    // Create mock adapter
    adapter := NewMockTridentAdapter(...)
    
    // Create replication with state "replica"
    uvr := &UnifiedVolumeReplication{
        Spec: UnifiedVolumeReplicationSpec{
            ReplicationState: "replica",
        },
    }
    adapter.EnsureReplication(ctx, uvr)
    
    // Verify state is "replica"
    status, _ := adapter.GetReplicationStatus(ctx, uvr)
    assert.Equal(t, "replica", status.State)
    
    // Update to state "source"
    uvr.Spec.ReplicationState = "source"
    adapter.EnsureReplication(ctx, uvr)
    
    // Verify state is now "source", NOT "failed"
    status, _ = adapter.GetReplicationStatus(ctx, uvr)
    assert.Equal(t, "source", status.State)
}
```

**Step 6: Run and verify**

```bash
# Run the new unit test
go test ./pkg/adapters -v -run TestMockAdapter_StatePersistence

# Run the E2E test
go test ./test/e2e -v -run TestE2E_CompleteWorkflow
```

### Acceptance Criteria
- [ ] `TestE2E_CompleteWorkflow/TranslateStates` passes
- [ ] Mock adapter correctly stores and returns replication state
- [ ] State updates are properly persisted in mock storage
- [ ] New unit test validates state persistence
- [ ] No "failed" state unless explicitly set or simulated

### Files to Modify
- `pkg/adapters/mock_trident.go` - Fix state tracking in `EnsureReplication` and `GetReplicationStatus`
- `pkg/adapters/mock_powerstore.go` - Same fix if needed
- `pkg/adapters/mock_test.go` - Add state persistence test
- `test/e2e/e2e_test.go` - Optionally add debug logging (temporary)

### Verification Command
```bash
go test ./test/e2e -v -run TestE2E_CompleteWorkflow/TranslateStates
```

---

## Fix Prompt #3: Fix Ceph Adapter Update Test

### 🟡 PRIORITY: MEDIUM
### 📦 PACKAGE: `pkg/adapters`
### ⏱️ ESTIMATED TIME: 1 hour

### Problem Statement
The Ceph adapter integration test fails when trying to update an existing replication:

```
Test: TestCephAdapter_EnsureReplication/ExistingReplication
Error: volumereplications.replication.storage.openshift.io "test-ceph-repl-update" not found
Location: pkg/adapters/ceph_integration_test.go
```

### Root Cause Analysis

The test creates a replication, then tries to update it. The update path calls `Get()` to retrieve the existing resource, but the mock Kubernetes client doesn't persist the resource from the first operation.

This is a **test infrastructure issue**, not a bug in the Ceph adapter code itself.

The problem is that the fake client used in tests may not properly persist resources between operations, or the test setup doesn't properly initialize the resource before the update test runs.

### Fix Plan

**Step 1: Understand the test structure**

Examine `pkg/adapters/ceph_integration_test.go`:
- Find `TestCephAdapter_EnsureReplication` test
- Look for the "ExistingReplication" subtest
- Identify how it creates the initial resource
- See how it tries to update

**Step 2: Identify the mock client issue**

The test likely does:
```go
// Create initial replication
adapter.EnsureReplication(ctx, uvr)

// Now try to update (this fails)
uvr.Spec.ReplicationMode = "synchronous"
adapter.EnsureReplication(ctx, uvr)  // ← Calls Get() which fails
```

The issue: The fake Kubernetes client in the test doesn't have the VolumeReplication resource that was "created" in the first call.

**Step 3: Fix approach options**

**Option A: Pre-populate the fake client**

Before calling update, explicitly create the resource in the fake client:

```go
func TestCephAdapter_EnsureReplication(t *testing.T) {
    t.Run("ExistingReplication", func(t *testing.T) {
        // Create the resource that should already exist
        existingVR := &unstructured.Unstructured{}
        existingVR.SetGroupVersionKind(CephVolumeReplicationGVK)
        existingVR.SetName("test-ceph-repl-update")
        existingVR.SetNamespace("default")
        
        // Pre-populate the client
        err := fakeClient.Create(ctx, existingVR)
        require.NoError(t, err)
        
        // Now test the update path
        uvr := createTestUVR("test-ceph-repl-update", "default")
        uvr.Spec.ReplicationMode = "synchronous"
        
        err = adapter.EnsureReplication(ctx, uvr)
        assert.NoError(t, err)
    })
}
```

**Option B: Use a properly initialized fake client**

Ensure the fake client is created with proper scheme registration:

```go
func setupTestClient(t *testing.T) client.Client {
    scheme := runtime.NewScheme()
    _ = clientgoscheme.AddToScheme(scheme)
    
    // Register VolumeReplication CRD types
    // This may require defining the types or using dynamic client
    
    fakeClient := fake.NewClientBuilder().
        WithScheme(scheme).
        WithRuntimeObjects(/* initial objects */).
        Build()
    
    return fakeClient
}
```

**Option C: Make the test use actual Create/Update**

Split the test to explicitly test create and update separately:

```go
t.Run("CreateReplication", func(t *testing.T) {
    // Test creation path
    uvr := createTestUVR("test-create", "default")
    err := adapter.EnsureReplication(ctx, uvr)
    assert.NoError(t, err)
})

t.Run("UpdateReplication", func(t *testing.T) {
    // Setup: Create initial resource in fake client
    initial := createVolumeReplication("test-update", "default")
    err := fakeClient.Create(ctx, initial)
    require.NoError(t, err)
    
    // Test: Update the resource
    uvr := createTestUVR("test-update", "default")
    uvr.Spec.ReplicationMode = "synchronous"
    
    err = adapter.EnsureReplication(ctx, uvr)
    assert.NoError(t, err)
})
```

**Step 4: Implement the fix**

The recommended approach is **Option C** - split the test and properly set up the fake client state for each scenario.

File: `pkg/adapters/ceph_integration_test.go`

1. Create a helper function to pre-create resources:
```go
func createVolumeReplicationInClient(t *testing.T, client client.Client, name, namespace string) {
    vr := &unstructured.Unstructured{}
    vr.SetGroupVersionKind(CephVolumeReplicationGVK)
    vr.SetName(name)
    vr.SetNamespace(namespace)
    
    // Set minimal required spec
    spec := map[string]interface{}{
        "replicationState": "primary",
        "volumeReplicationClass": "test-class",
        "dataSource": map[string]interface{}{
            "apiGroup": "",
            "kind": "PersistentVolumeClaim",
            "name": "test-pvc",
        },
    }
    unstructured.SetNestedMap(vr.Object, spec, "spec")
    
    err := client.Create(context.Background(), vr)
    require.NoError(t, err, "Failed to pre-create VolumeReplication")
}
```

2. Update the test to use it:
```go
func TestCephAdapter_EnsureReplication(t *testing.T) {
    // ... existing setup ...
    
    t.Run("UpdateExistingReplication", func(t *testing.T) {
        // Setup: Pre-create the resource
        createVolumeReplicationInClient(t, fakeClient, "test-ceph-repl-update", "default")
        
        // Test: Update it via adapter
        uvr := &replicationv1alpha1.UnifiedVolumeReplication{
            ObjectMeta: metav1.ObjectMeta{
                Name:      "test-ceph-repl-update",
                Namespace: "default",
            },
            Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
                ReplicationState: "replica",
                ReplicationMode:  "synchronous",  // Changed from asynchronous
                // ... rest of spec
            },
        }
        
        err := adapter.EnsureReplication(ctx, uvr)
        assert.NoError(t, err)
        
        // Verify the update was applied
        updated := &unstructured.Unstructured{}
        updated.SetGroupVersionKind(CephVolumeReplicationGVK)
        err = fakeClient.Get(ctx, client.ObjectKey{
            Name: "test-ceph-repl-update",
            Namespace: "default",
        }, updated)
        require.NoError(t, err)
        
        // Assert the mode was updated
        replicationClass, _, _ := unstructured.NestedString(updated.Object, "spec", "volumeReplicationClass")
        assert.Contains(t, replicationClass, "sync") // Verify it's synchronous
    })
}
```

**Step 5: Verify the fix**

```bash
go test ./pkg/adapters -v -run TestCephAdapter_EnsureReplication
```

**Step 6: Apply same fix to other adapters if needed**

Check if Trident or PowerStore adapters have similar tests that might need the same fix.

### Acceptance Criteria
- [ ] `TestCephAdapter_EnsureReplication/ExistingReplication` passes
- [ ] Test properly sets up existing resource before testing update
- [ ] Similar tests in other adapters are also fixed
- [ ] Helper function is reusable for other tests
- [ ] Test clearly separates create vs update scenarios

### Files to Modify
- `pkg/adapters/ceph_integration_test.go` - Fix test setup and add helper
- Possibly `pkg/adapters/trident_integration_test.go` - Same fix if needed
- Possibly `pkg/adapters/powerstore_integration_test.go` - Same fix if needed

### Verification Command
```bash
go test ./pkg/adapters -v -run TestCephAdapter_EnsureReplication/ExistingReplication
```

---

## Fix Prompt #4: Fix Schedule Mode Test

### 🟢 PRIORITY: LOW
### 📦 PACKAGE: `test/fixtures`
### ⏱️ ESTIMATED TIME: 15 minutes

### Problem Statement
The fixtures test expects 3 schedule modes but the code only defines 2:

```
Test: TestValidScheduleModes
Error: "[continuous interval]" should have 3 item(s), but has 2
Expected: 3 schedule modes
Actual: 2 schedule modes
Location: test/fixtures/samples_test.go:127
```

### Root Cause Analysis

The test was written expecting three schedule modes, but the CRD only defines two:

```go
// api/v1alpha1/unifiedvolumereplication_types.go
type ScheduleMode string

const (
    ScheduleModeContinuous ScheduleMode = "continuous"
    ScheduleModeInterval   ScheduleMode = "interval"
)
```

This is a **test/documentation mismatch** issue. Either:
1. A third mode was planned but never implemented, OR
2. The test is incorrect

### Fix Plan

**Step 1: Determine if a third mode is needed**

Review requirements and documentation:
- Check `docs/api-reference/API_REFERENCE.md` - How many modes are documented?
- Check `config/samples/*.yaml` - Which modes are used in examples?
- Check use cases - Is there a need for a third mode like "manual" or "on-demand"?

**Step 2: Decision point**

**Option A: Test is wrong (most likely)**

If only 2 modes are needed:
- Fix the test to expect 2 modes
- Update test documentation

**Option B: Add a third mode**

If requirements show a need for a third mode:
- Add the mode to the CRD types
- Update validation
- Add examples
- Update documentation

### Recommended Fix: Option A (Fix the Test)

**Step 1: Update the test**

File: `test/fixtures/samples_test.go` around line 127

Change from:
```go
func TestValidScheduleModes(t *testing.T) {
    modes := []ScheduleMode{
        ScheduleModeContinuous,
        ScheduleModeInterval,
    }
    assert.Len(t, modes, 3, "Should have all 3 schedule modes")  // ← WRONG
}
```

To:
```go
func TestValidScheduleModes(t *testing.T) {
    modes := []ScheduleMode{
        ScheduleModeContinuous,
        ScheduleModeInterval,
    }
    assert.Len(t, modes, 2, "Should have all defined schedule modes")
    
    // Verify each mode is valid
    for _, mode := range modes {
        assert.NotEmpty(t, string(mode), "Schedule mode should not be empty")
    }
    
    // Verify the modes are the expected ones
    assert.Contains(t, modes, ScheduleModeContinuous)
    assert.Contains(t, modes, ScheduleModeInterval)
}
```

**Step 2: Add documentation comment**

Add a comment explaining the two modes:

```go
// TestValidScheduleModes verifies that all schedule modes are defined.
// Current modes:
// - continuous: Replication runs continuously
// - interval: Replication runs at specified intervals (RPO-based)
func TestValidScheduleModes(t *testing.T) {
    // ...
}
```

**Step 3: Verify no other places expect 3 modes**

Search for other references:
```bash
grep -r "3.*schedule.*mode" --include="*.go" --include="*.md"
grep -r "third.*mode" --include="*.go" --include="*.md"
```

**Step 4: Run the test**

```bash
go test ./test/fixtures -v -run TestValidScheduleModes
```

### Alternative Fix: Option B (Add Third Mode)

If after review you determine a third mode IS needed:

**Step 1: Add the mode to types**

File: `api/v1alpha1/unifiedvolumereplication_types.go`

```go
// ScheduleMode defines the replication scheduling mode
// +kubebuilder:validation:Enum=continuous;interval;manual
type ScheduleMode string

const (
    // ScheduleModeContinuous provides continuous replication
    ScheduleModeContinuous ScheduleMode = "continuous"
    // ScheduleModeInterval provides interval-based replication
    ScheduleModeInterval ScheduleMode = "interval"
    // ScheduleModeManual requires explicit trigger for replication
    ScheduleModeManual ScheduleMode = "manual"
)
```

**Step 2: Update validation**

File: `api/v1alpha1/unifiedvolumereplication_types.go`

In the `validateSchedule()` function, add handling for manual mode:

```go
func (uvr *UnifiedVolumeReplication) validateSchedule() error {
    schedule := uvr.Spec.Schedule
    
    switch schedule.Mode {
    case ScheduleModeInterval:
        if schedule.Rpo == "" {
            return fmt.Errorf("schedule RPO is required when mode is 'interval'")
        }
    case ScheduleModeContinuous:
        // RPO/RTO are optional objectives
    case ScheduleModeManual:
        // Manual mode - no RPO/RTO requirements
    default:
        return fmt.Errorf("invalid schedule mode '%s', must be one of: continuous, interval, manual", schedule.Mode)
    }
    
    return nil
}
```

**Step 3: Update CRD**

```bash
make manifests
```

**Step 4: Update documentation**

Update `docs/api-reference/API_REFERENCE.md` to document the new mode.

**Step 5: Add sample**

Create `config/samples/manual_schedule_example.yaml`

### Acceptance Criteria
- [ ] `TestValidScheduleModes` passes
- [ ] Test accurately reflects the number of modes in the code
- [ ] Documentation matches implementation
- [ ] No other tests fail due to the change

### Files to Modify
**Option A (Fix Test):**
- `test/fixtures/samples_test.go` - Update test expectation to 2

**Option B (Add Mode):**
- `api/v1alpha1/unifiedvolumereplication_types.go` - Add third mode
- `api/v1alpha1/unifiedvolumereplication_types.go` - Update validation
- `docs/api-reference/API_REFERENCE.md` - Document new mode
- `config/samples/` - Add example
- Run `make manifests` to update CRD

### Verification Command
```bash
go test ./test/fixtures -v -run TestValidScheduleModes
```

---

## Fix Prompt #5: Fix Test Utility Cleanup Issues

### 🟢 PRIORITY: LOW
### 📦 PACKAGE: `test/utils`
### ⏱️ ESTIMATED TIME: 30 minutes

### Problem Statement
The performance tracker tests fail due to improper cleanup between tests:

```
Test: TestPerformanceTracker/track_multiple_operations
Error: map should have 3 item(s), but has 4
Actual: map[op1:5.145ms op2:5.134ms op3:5.135ms test-op:10.095ms]
        ^^^^^^^^^^^^^ leftover from previous test

Test: TestPerformanceTracker/reset_tracker
Error: map should have 1 item(s), but has 5
Actual: map[op1:5.145ms op2:5.134ms op3:5.135ms reset-test:228ns test-op:10.095ms]
        ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^ not properly reset

Location: test/utils/crd_helpers_test.go:303, 316
```

Also:
```
Test: TestCRDManipulator/update_CRD_status
Error: unifiedvolumereplications.replication.unified.io "test-status-update" not found
```

### Root Cause Analysis

Two separate issues:

**Issue 1: Performance Tracker State Pollution**
- The performance tracker is using a global/shared instance
- Tests don't properly reset the tracker between runs
- Operations from one test leak into the next test
- The `Reset()` method may not be working correctly

**Issue 2: CRD Manipulator Resource Cleanup**
- Resources created in one test are not cleaned up
- Subsequent tests expect a clean state
- The "test-status-update" resource may not exist when the test expects it

### Fix Plan

#### Part A: Fix Performance Tracker

**Step 1: Identify the problem**

File: `test/utils/crd_helpers_test.go`

Look for how the performance tracker is created:
```go
func TestPerformanceTracker(t *testing.T) {
    tracker := NewPerformanceTracker()  // ← Is this shared?
    
    t.Run("test1", func(t *testing.T) {
        tracker.Track("op1", ...)
    })
    
    t.Run("test2", func(t *testing.T) {
        // tracker still has "op1" from test1
        tracker.Track("op2", ...)
    })
}
```

**Step 2: Fix approach - Create fresh tracker per subtest**

Option A: Create new tracker in each subtest:

```go
func TestPerformanceTracker(t *testing.T) {
    t.Run("track_single_operation", func(t *testing.T) {
        tracker := NewPerformanceTracker()  // ← Fresh instance
        
        // Existing test code...
        tracker.Track("test-op", 10*time.Millisecond)
        
        duration, _ := tracker.GetDuration("test-op")
        assert.Equal(t, 10*time.Millisecond, duration)
    })
    
    t.Run("track_multiple_operations", func(t *testing.T) {
        tracker := NewPerformanceTracker()  // ← Fresh instance
        
        // Track 3 operations
        tracker.Track("op1", 5*time.Millisecond)
        tracker.Track("op2", 5*time.Millisecond)
        tracker.Track("op3", 5*time.Millisecond)
        
        durations := tracker.GetAllDurations()
        assert.Len(t, durations, 3, "Should have exactly 3 operations")  // ← Now passes
    })
    
    t.Run("reset_tracker", func(t *testing.T) {
        tracker := NewPerformanceTracker()  // ← Fresh instance
        
        // Track operation
        tracker.Track("reset-test", 1*time.Nanosecond)
        
        // Reset
        tracker.Reset()
        
        durations := tracker.GetAllDurations()
        assert.Len(t, durations, 0, "Should have 0 operations after reset")  // ← Now passes
    })
}
```

Option B: Properly reset between subtests:

```go
func TestPerformanceTracker(t *testing.T) {
    var tracker *PerformanceTracker
    
    // Setup: Create fresh tracker before each subtest
    setup := func(t *testing.T) {
        tracker = NewPerformanceTracker()
    }
    
    // Teardown: Reset after each subtest
    teardown := func(t *testing.T) {
        if tracker != nil {
            tracker.Reset()
        }
    }
    
    t.Run("track_single_operation", func(t *testing.T) {
        setup(t)
        defer teardown(t)
        
        // Test code...
    })
    
    t.Run("track_multiple_operations", func(t *testing.T) {
        setup(t)
        defer teardown(t)
        
        // Test code...
    })
}
```

**Step 3: Fix the Reset() method**

File: `test/utils/crd_helpers.go`

Ensure Reset() actually clears the internal map:

```go
type PerformanceTracker struct {
    operations map[string]time.Duration
    mu         sync.RWMutex
}

func (pt *PerformanceTracker) Reset() {
    pt.mu.Lock()
    defer pt.mu.Unlock()
    
    // Clear the map completely
    pt.operations = make(map[string]time.Duration)
}
```

#### Part B: Fix CRD Manipulator

**Step 1: Add cleanup to test**

File: `test/utils/crd_helpers_test.go`

```go
func TestCRDManipulator(t *testing.T) {
    client := setupTestClient(t)
    
    t.Run("update_CRD_status", func(t *testing.T) {
        // Create the resource first
        uvr := &replicationv1alpha1.UnifiedVolumeReplication{
            ObjectMeta: metav1.ObjectMeta{
                Name:      "test-status-update",
                Namespace: "default",
            },
            Spec: createValidSpec(),
        }
        
        // Create it
        err := client.Create(context.Background(), uvr)
        require.NoError(t, err, "Failed to create resource for test")
        
        // Cleanup after test
        defer func() {
            _ = client.Delete(context.Background(), uvr)
        }()
        
        // Now test status update
        uvr.Status.ObservedGeneration = 1
        err = client.Status().Update(context.Background(), uvr)
        assert.NoError(t, err)  // ← Should now pass
        
        // Verify
        retrieved := &replicationv1alpha1.UnifiedVolumeReplication{}
        err = client.Get(context.Background(), 
            types.NamespacedName{Name: "test-status-update", Namespace: "default"},
            retrieved)
        require.NoError(t, err)
        assert.Equal(t, int64(1), retrieved.Status.ObservedGeneration)
    })
}
```

**Step 2: Add helper for test setup**

```go
// createTestResource creates a UVR for testing and returns cleanup function
func createTestResource(t *testing.T, client client.Client, name string) (*replicationv1alpha1.UnifiedVolumeReplication, func()) {
    uvr := &replicationv1alpha1.UnifiedVolumeReplication{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: "default",
        },
        Spec: createValidSpec(),
    }
    
    err := client.Create(context.Background(), uvr)
    require.NoError(t, err, "Failed to create test resource")
    
    cleanup := func() {
        _ = client.Delete(context.Background(), uvr)
    }
    
    return uvr, cleanup
}

// Usage:
t.Run("update_CRD_status", func(t *testing.T) {
    uvr, cleanup := createTestResource(t, client, "test-status-update")
    defer cleanup()
    
    // Test status update
    // ...
})
```

**Step 3: Verify all subtests have proper setup/cleanup**

Review all subtests in `TestCRDManipulator` and ensure:
- Resources are created before being used
- Resources are cleaned up after tests
- No assumptions about pre-existing state

### Acceptance Criteria
- [ ] `TestPerformanceTracker/track_multiple_operations` passes
- [ ] `TestPerformanceTracker/reset_tracker` passes
- [ ] `TestCRDManipulator/update_CRD_status` passes
- [ ] Tests don't pollute each other's state
- [ ] Reset() method properly clears tracker
- [ ] Helper functions for test setup/cleanup

### Files to Modify
- `test/utils/crd_helpers_test.go` - Fix test isolation
- `test/utils/crd_helpers.go` - Fix Reset() method if needed
- `test/utils/test_helpers.go` - Add setup/cleanup helpers

### Verification Command
```bash
go test ./test/utils -v -run TestPerformanceTracker
go test ./test/utils -v -run TestCRDManipulator
```

---

## Execution Plan

### Phase 1: Environment Setup (30 min)
1. Run Fix Prompt #1 (Integration Test Environment)
2. Verify: `go test ./test/integration/... -v`

### Phase 2: Quick Wins (45 min)
3. Run Fix Prompt #4 (Schedule Mode Test)
4. Verify: `go test ./test/fixtures -v -run TestValidScheduleModes`
5. Run Fix Prompt #5 (Test Utility Cleanup)
6. Verify: `go test ./test/utils -v`

### Phase 3: Mock Adapter Issues (2 hrs)
7. Run Fix Prompt #2 (E2E State Translation)
8. Verify: `go test ./test/e2e -v`
9. Run Fix Prompt #3 (Ceph Adapter Update)
10. Verify: `go test ./pkg/adapters -v`

### Phase 4: Final Verification (15 min)
11. Run complete test suite: `go test ./... -v`
12. Verify 100% pass rate
13. Generate coverage report: `go test ./... -coverprofile=coverage.out`

### Total Estimated Time: 3.5 hours

---

## Success Criteria

### Before Starting
- [ ] 97.5% tests passing (390/400)
- [ ] 5 failing test issues identified

### After Completion
- [ ] 100% tests passing (400/400)
- [ ] All integration tests runnable
- [ ] All E2E tests passing
- [ ] All unit tests passing
- [ ] Test coverage maintained or improved
- [ ] CI/CD updated with environment setup
- [ ] Documentation updated

---

## Risk Mitigation

### Low-Risk Changes
- Fix #4 (Schedule Mode) - Just updating test expectation
- Fix #5 (Test Utilities) - Test isolation improvements

### Medium-Risk Changes
- Fix #1 (Integration Environment) - Only affects test setup
- Fix #3 (Ceph Adapter Test) - Only affects test mocks

### Needs Careful Testing
- Fix #2 (E2E State Translation) - Touches mock adapter logic
  - Risk: Could affect other tests if mock behavior changes
  - Mitigation: Run full adapter test suite after changes
  - Verify: `go test ./pkg/adapters/... -v`

---

## Post-Fix Checklist

After completing all fixes:

- [ ] All tests pass: `go test ./... -v`
- [ ] No race conditions: `go test ./... -race`
- [ ] Coverage maintained: `go test ./... -cover`
- [ ] CI/CD pipeline updated
- [ ] README documents test requirements
- [ ] CONTRIBUTING.md explains how to run tests
- [ ] Test infrastructure documented
- [ ] Commit messages reference test failures fixed

---

**STATUS:** Ready for Implementation  
**BLOCKER:** None  
**DEPENDENCIES:** Go 1.19+, setup-envtest tool  
**OUTCOME:** 100% test pass rate, production-ready codebase

