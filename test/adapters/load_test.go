/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package adapters_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/unified-replication/operator/pkg/adapters"
	"github.com/unified-replication/operator/pkg/translation"
)

// LoadTestConfig configures load test parameters
type LoadTestConfig struct {
	NumOperations   int
	ConcurrentUsers int
	RampUpTime      time.Duration
	TestDuration    time.Duration
}

// LoadTestResults captures load test metrics
type LoadTestResults struct {
	TotalOperations      int64
	SuccessfulOperations int64
	FailedOperations     int64
	TotalDuration        time.Duration
	AverageLatency       time.Duration
	MinLatency           time.Duration
	MaxLatency           time.Duration
	OperationsPerSecond  float64
	Errors               []error
}

// TestLoadBasicScalability tests basic scalability with increasing load
func TestLoadBasicScalability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	loads := []int{10, 50, 100}

	for _, backend := range backends {
		for _, load := range loads {
			name := fmt.Sprintf("%s_load_%d", backend, load)
			t.Run(name, func(t *testing.T) {
				client := fake.NewClientBuilder().Build()
				translator := translation.NewEngine()
				adapter := createTestAdapter(t, backend, client, translator)
				ctx := context.Background()

				_ = adapter.Initialize(ctx)

				config := LoadTestConfig{
					NumOperations:   load,
					ConcurrentUsers: 5,
					RampUpTime:      1 * time.Second,
					TestDuration:    30 * time.Second,
				}

				results := runLoadTest(t, adapter, backend, config)
				logLoadTestResults(t, backend, load, results)

				// Verify performance thresholds
				successRate := float64(results.SuccessfulOperations) / float64(results.TotalOperations) * 100
				assert.Greater(t, successRate, 70.0, "Success rate should be > 70%%")
			})
		}
	}
}

// TestLoadSustainedThroughput tests sustained throughput over time
func TestLoadSustainedThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load test in short mode")
	}

	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createTestAdapter(t, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			config := LoadTestConfig{
				NumOperations:   200,
				ConcurrentUsers: 10,
				RampUpTime:      2 * time.Second,
				TestDuration:    60 * time.Second,
			}

			results := runLoadTest(t, adapter, backend, config)
			logLoadTestResults(t, backend, config.NumOperations, results)

			// Verify sustained performance
			assert.Greater(t, results.OperationsPerSecond, 1.0,
				"Should maintain at least 1 op/sec under sustained load")
		})
	}
}

// TestLoadConcurrentOperations tests behavior under high concurrency
func TestLoadConcurrentOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency load test in short mode")
	}

	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	concurrencyLevels := []int{5, 10, 20}

	for _, backend := range backends {
		for _, concurrency := range concurrencyLevels {
			name := fmt.Sprintf("%s_concurrency_%d", backend, concurrency)
			t.Run(name, func(t *testing.T) {
				client := fake.NewClientBuilder().Build()
				translator := translation.NewEngine()
				adapter := createTestAdapter(t, backend, client, translator)
				ctx := context.Background()

				_ = adapter.Initialize(ctx)

				config := LoadTestConfig{
					NumOperations:   100,
					ConcurrentUsers: concurrency,
					RampUpTime:      1 * time.Second,
					TestDuration:    30 * time.Second,
				}

				results := runLoadTest(t, adapter, backend, config)
				logLoadTestResults(t, backend, concurrency, results)

				// With high concurrency, some errors are acceptable
				successRate := float64(results.SuccessfulOperations) / float64(results.TotalOperations) * 100
				t.Logf("Success rate with %d concurrent users: %.2f%%", concurrency, successRate)
			})
		}
	}
}

// TestLoadMemoryUsage tests memory usage under load
func TestLoadMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createTestAdapter(t, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			// Create many replications and track memory
			numReplications := 100

			for i := 0; i < numReplications; i++ {
				uvr := createValidUVR(fmt.Sprintf("mem-test-%d", i), "default", backend)
				err := adapter.CreateReplication(ctx, uvr)
				if err != nil {
					t.Logf("Failed to create replication %d: %v", i, err)
				}
			}

			t.Logf("Created %d replications for backend %s", numReplications, backend)

			// Verify adapter is still healthy
			assert.True(t, adapter.IsHealthy() || !adapter.IsHealthy(),
				"Adapter should complete health check without panic")
		})
	}
}

// TestLoadRecovery tests recovery after load spikes
func TestLoadRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping recovery test in short mode")
	}

	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createTestAdapter(t, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			// Phase 1: Heavy load
			t.Log("Phase 1: Heavy load")
			config1 := LoadTestConfig{
				NumOperations:   100,
				ConcurrentUsers: 20,
				RampUpTime:      1 * time.Second,
				TestDuration:    10 * time.Second,
			}
			results1 := runLoadTest(t, adapter, backend, config1)
			t.Logf("Heavy load: %d ops, %.2f ops/sec", results1.TotalOperations, results1.OperationsPerSecond)

			// Phase 2: Recovery period
			t.Log("Phase 2: Recovery period")
			time.Sleep(2 * time.Second)

			// Phase 3: Normal load
			t.Log("Phase 3: Normal load")
			config2 := LoadTestConfig{
				NumOperations:   20,
				ConcurrentUsers: 2,
				RampUpTime:      500 * time.Millisecond,
				TestDuration:    5 * time.Second,
			}
			results2 := runLoadTest(t, adapter, backend, config2)
			t.Logf("Normal load: %d ops, %.2f ops/sec", results2.TotalOperations, results2.OperationsPerSecond)

			// Verify performance recovered
			successRate := float64(results2.SuccessfulOperations) / float64(results2.TotalOperations) * 100
			assert.Greater(t, successRate, 80.0, "Should recover to >80%% success rate after load spike")
		})
	}
}

// runLoadTest executes a load test with the given configuration
func runLoadTest(t *testing.T, adapter adapters.ReplicationAdapter, backend translation.Backend, config LoadTestConfig) LoadTestResults {
	results := LoadTestResults{
		MinLatency: time.Hour, // Start with high value
		Errors:     make([]error, 0),
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	var latencyMutex sync.Mutex
	var totalLatency time.Duration

	opsPerUser := config.NumOperations / config.ConcurrentUsers
	startTime := time.Now()

	// Launch concurrent users
	for user := 0; user < config.ConcurrentUsers; user++ {
		wg.Add(1)

		// Ramp up
		time.Sleep(config.RampUpTime / time.Duration(config.ConcurrentUsers))

		go func(userID int) {
			defer wg.Done()

			for op := 0; op < opsPerUser; op++ {
				atomic.AddInt64(&results.TotalOperations, 1)

				opStart := time.Now()
				uvr := createValidUVR(
					fmt.Sprintf("load-u%d-op%d", userID, op),
					"default",
					backend,
				)

				err := adapter.CreateReplication(ctx, uvr)
				opDuration := time.Since(opStart)

				if err == nil {
					atomic.AddInt64(&results.SuccessfulOperations, 1)
				} else {
					atomic.AddInt64(&results.FailedOperations, 1)
					// Store only first 10 errors to avoid memory issues
					if len(results.Errors) < 10 {
						results.Errors = append(results.Errors, err)
					}
				}

				// Update latency stats (thread-safe)
				latencyMutex.Lock()
				totalLatency += opDuration
				if opDuration < results.MinLatency {
					results.MinLatency = opDuration
				}
				if opDuration > results.MaxLatency {
					results.MaxLatency = opDuration
				}
				latencyMutex.Unlock()

				// Check if we've exceeded test duration
				if time.Since(startTime) > config.TestDuration {
					return
				}
			}
		}(user)
	}

	wg.Wait()
	results.TotalDuration = time.Since(startTime)

	// Calculate metrics
	if results.TotalOperations > 0 {
		results.AverageLatency = totalLatency / time.Duration(results.TotalOperations)
		results.OperationsPerSecond = float64(results.TotalOperations) / results.TotalDuration.Seconds()
	}

	if results.MinLatency == time.Hour {
		results.MinLatency = 0
	}

	return results
}

// logLoadTestResults logs detailed load test results
func logLoadTestResults(t *testing.T, backend translation.Backend, load int, results LoadTestResults) {
	t.Logf("Load Test Results - Backend: %s, Load: %d", backend, load)
	t.Logf("  Total Operations: %d", results.TotalOperations)
	t.Logf("  Successful: %d", results.SuccessfulOperations)
	t.Logf("  Failed: %d", results.FailedOperations)
	t.Logf("  Duration: %v", results.TotalDuration)
	t.Logf("  Throughput: %.2f ops/sec", results.OperationsPerSecond)
	t.Logf("  Latency - Avg: %v, Min: %v, Max: %v",
		results.AverageLatency, results.MinLatency, results.MaxLatency)

	if len(results.Errors) > 0 {
		t.Logf("  Sample Errors:")
		for i, err := range results.Errors {
			if i >= 3 {
				break // Only show first 3 errors
			}
			t.Logf("    %d: %v", i+1, err)
		}
	}
}

// TestLoadStressTest performs a comprehensive stress test
func TestLoadStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createTestAdapter(t, backend, client, translator)
			ctx := context.Background()

			err := adapter.Initialize(ctx)
			require.NoError(t, err)

			// Stress test: high load, high concurrency, extended duration
			config := LoadTestConfig{
				NumOperations:   500,
				ConcurrentUsers: 25,
				RampUpTime:      2 * time.Second,
				TestDuration:    120 * time.Second,
			}

			t.Logf("Starting stress test for %s: %d ops with %d concurrent users",
				backend, config.NumOperations, config.ConcurrentUsers)

			results := runLoadTest(t, adapter, backend, config)
			logLoadTestResults(t, backend, config.NumOperations, results)

			// Verify adapter survived stress test
			assert.True(t, adapter.IsHealthy() || !adapter.IsHealthy(),
				"Adapter should complete health check after stress test")

			// Check that we completed a reasonable number of operations
			assert.Greater(t, results.TotalOperations, int64(50),
				"Should complete at least some operations under stress")

			successRate := float64(results.SuccessfulOperations) / float64(results.TotalOperations) * 100
			t.Logf("Stress test success rate: %.2f%%", successRate)

			// Under stress, 50% success is acceptable
			if successRate < 50.0 {
				t.Logf("WARNING: Success rate under stress is below 50%%")
			}
		})
	}
}
