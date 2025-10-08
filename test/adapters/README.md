# Adapter Testing Framework

This directory contains comprehensive testing and validation framework for all replication adapters in the Unified Replication Operator.

## Test Structure

### 1. Compliance Tests (`compliance_test.go`)
Verifies that all adapters correctly implement the `ReplicationAdapter` interface and behave consistently.

**Tests:**
- Interface method implementations
- Cross-adapter behavior consistency
- Configuration validation
- Resource cleanup verification

**Usage:**
```bash
go test -v ./test/adapters -run TestAdapterInterfaceCompliance
go test -v ./test/adapters -run TestCrossAdapterConsistency
go test -v ./test/adapters -run TestAdapterValidation
go test -v ./test/adapters -run TestAdapterResourceCleanup
```

### 2. Performance Tests (`performance_test.go`)
Establishes performance baselines and benchmarks adapter operations.

**Benchmarks:**
- `BenchmarkAdapterCreate` - Replication creation performance
- `BenchmarkAdapterUpdate` - Replication update performance
- `BenchmarkAdapterGetStatus` - Status retrieval performance
- `BenchmarkAdapterDelete` - Replication deletion performance
- `BenchmarkAdapterValidation` - Configuration validation performance

**Performance Tests:**
- Baseline latency measurements
- Concurrent operation handling
- Sustained throughput testing

**Usage:**
```bash
# Run all benchmarks
go test -bench=. ./test/adapters

# Run specific benchmark
go test -bench=BenchmarkAdapterCreate ./test/adapters

# Run with memory profiling
go test -bench=. -benchmem ./test/adapters

# Run performance baseline tests
go test -v ./test/adapters -run TestAdapterPerformanceBaseline
go test -v ./test/adapters -run TestAdapterConcurrency
go test -v ./test/adapters -run TestAdapterThroughput
```

### 3. Fault Tolerance Tests (`fault_tolerance_test.go`)
Tests adapter resilience under various failure conditions.

**Tests:**
- Error injection (create, update, delete, status failures)
- Intermittent failure handling
- Recovery from failure
- Error propagation
- Partial failure scenarios

**Usage:**
```bash
go test -v ./test/adapters -run TestErrorInjection
go test -v ./test/adapters -run TestIntermittentFailures
go test -v ./test/adapters -run TestRecoveryFromFailure
go test -v ./test/adapters -run TestErrorPropagation
go test -v ./test/adapters -run TestPartialFailureScenarios
```

### 4. State Transition Tests (`state_transition_test.go`)
Validates state management and transition logic.

**Tests:**
- Valid state transitions (replica → promoting → source)
- Invalid state transitions detection
- State consistency verification
- Transition timing
- Concurrent state changes
- Status reporting accuracy

**Usage:**
```bash
go test -v ./test/adapters -run TestStateTransitions
go test -v ./test/adapters -run TestStateTransitionConsistency
go test -v ./test/adapters -run TestStateValidation
go test -v ./test/adapters -run TestStatusReporting
go test -v ./test/adapters -run TestConcurrentStateTransitions
```

### 5. Load Tests (`load_test.go`)
Verifies adapter scalability and performance under load.

**Tests:**
- Basic scalability (10, 50, 100 operations)
- Sustained throughput over time
- High concurrency handling (5, 10, 20 concurrent users)
- Memory usage under load
- Recovery after load spikes
- Comprehensive stress test

**Usage:**
```bash
# Run all load tests (may take several minutes)
go test -v ./test/adapters -run TestLoad

# Run specific load test
go test -v ./test/adapters -run TestLoadBasicScalability
go test -v ./test/adapters -run TestLoadSustainedThroughput
go test -v ./test/adapters -run TestLoadConcurrentOperations
go test -v ./test/adapters -run TestLoadMemoryUsage
go test -v ./test/adapters -run TestLoadRecovery

# Run comprehensive stress test
go test -v ./test/adapters -run TestLoadStressTest -timeout 30m
```

## Running All Tests

### Quick Test Suite
```bash
# Run only fast tests
go test -v -short ./test/adapters
```

### Full Test Suite
```bash
# Run all tests including load and performance tests
go test -v ./test/adapters -timeout 30m
```

### With Coverage
```bash
go test -v -coverprofile=coverage.out ./test/adapters
go tool cover -html=coverage.out -o coverage.html
```

## Test Configuration

### Environment Variables
- `ENABLE_MOCK_ADAPTERS=true` - Enable mock adapters for testing
- `TEST_TIMEOUT=30m` - Set custom test timeout

### Test Flags
- `-short` - Run only fast tests, skip load and performance tests
- `-v` - Verbose output
- `-run <pattern>` - Run specific tests matching pattern
- `-bench <pattern>` - Run benchmarks matching pattern
- `-timeout <duration>` - Set test timeout (default: 10m)
- `-count <n>` - Run each test n times

## Performance Baselines

Expected performance thresholds (adjust based on hardware):

### Latency
- Create: < 500ms
- Update: < 500ms
- Status: < 500ms
- Delete: < 500ms

### Throughput
- Minimum: > 10 ops/sec sustained

### Success Rates
- Normal operations: > 80%
- Under load: > 70%
- Under stress: > 50%

### Concurrency
- Should handle 10+ concurrent operations
- Should maintain > 70% success rate with 10 concurrent users

## Adding New Tests

### For a New Backend Adapter

1. Add backend to the `backends` slice in test files:
```go
backends := []translation.Backend{
    translation.BackendCeph,
    translation.BackendTrident,
    translation.BackendPowerStore,
    translation.BackendYourNew, // Add here
}
```

2. Update `createTestAdapter` helper function:
```go
func createTestAdapter(...) adapters.ReplicationAdapter {
    switch backend {
    case translation.BackendYourNew:
        return adapters.NewYourNewAdapter(client, translator, nil)
    // ... existing cases
    }
}
```

3. Run the full test suite to verify compliance

### For New Test Scenarios

1. Add test function following existing patterns
2. Use existing helper functions for consistency
3. Include in appropriate test file based on test type
4. Document expected behavior and thresholds

## Test Best Practices

1. **Isolation**: Each test should be independent and not rely on test order
2. **Cleanup**: Always clean up resources after tests
3. **Timeouts**: Use context with timeout for operations
4. **Error Handling**: Check errors but don't fail for expected failures
5. **Logging**: Use `t.Log` and `t.Logf` for diagnostic information
6. **Benchmarking**: Use `b.ResetTimer()` to exclude setup time

## Continuous Integration

These tests are designed to run in CI environments:

```yaml
# Example GitHub Actions configuration
- name: Run Adapter Tests
  run: |
    go test -v -short ./test/adapters
    
- name: Run Full Test Suite
  if: github.event_name == 'push' && github.ref == 'refs/heads/main'
  run: |
    go test -v ./test/adapters -timeout 30m
```

## Troubleshooting

### Tests Timing Out
- Use `-short` flag to skip long-running tests
- Increase timeout with `-timeout` flag
- Check for deadlocks or infinite loops

### High Failure Rates
- Review error messages for patterns
- Check if test expectations match implementation
- Verify test environment has sufficient resources

### Inconsistent Results
- Some variance is expected with concurrent tests
- Run tests multiple times: `go test -count=10`
- Check for race conditions: `go test -race`

## Support

For issues or questions about the testing framework:
1. Review test output and error messages
2. Check if issue reproduces with `-short` flag
3. Review implementation-specific documentation
4. File an issue with test output and configuration

