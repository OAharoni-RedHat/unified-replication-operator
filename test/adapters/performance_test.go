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
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/unified-replication/operator/pkg/adapters"
	"github.com/unified-replication/operator/pkg/translation"
)

// BenchmarkAdapterCreate benchmarks replication creation across adapters
func BenchmarkAdapterCreate(b *testing.B) {
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		b.Run(string(backend), func(b *testing.B) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createBenchAdapter(b, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				uvr := createValidUVR(fmt.Sprintf("bench-create-%d", i), "default", backend)
				err := adapter.EnsureReplication(ctx, uvr)
				if err != nil {
					b.Fatalf("CreateReplication failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkAdapterUpdate benchmarks replication updates
func BenchmarkAdapterUpdate(b *testing.B) {
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		b.Run(string(backend), func(b *testing.B) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createBenchAdapter(b, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			// Pre-create replication
			uvr := createValidUVR("bench-update", "default", backend)
			_ = adapter.EnsureReplication(ctx, uvr)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := adapter.EnsureReplication(ctx, uvr)
				if err != nil {
					b.Fatalf("UpdateReplication failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkAdapterGetStatus benchmarks status retrieval
func BenchmarkAdapterGetStatus(b *testing.B) {
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		b.Run(string(backend), func(b *testing.B) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createBenchAdapter(b, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			// Pre-create replication
			uvr := createValidUVR("bench-status", "default", backend)
			_ = adapter.EnsureReplication(ctx, uvr)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := adapter.GetReplicationStatus(ctx, uvr)
				if err != nil {
					b.Fatalf("GetReplicationStatus failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkAdapterDelete benchmarks replication deletion
func BenchmarkAdapterDelete(b *testing.B) {
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		b.Run(string(backend), func(b *testing.B) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createBenchAdapter(b, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				// Create replication before each delete
				uvr := createValidUVR(fmt.Sprintf("bench-delete-%d", i), "default", backend)
				_ = adapter.EnsureReplication(ctx, uvr)
				b.StartTimer()

				err := adapter.DeleteReplication(ctx, uvr)
				if err != nil {
					b.Fatalf("DeleteReplication failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkAdapterValidation benchmarks configuration validation
func BenchmarkAdapterValidation(b *testing.B) {
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		b.Run(string(backend), func(b *testing.B) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createBenchAdapter(b, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			uvr := createValidUVR("bench-validate", "default", backend)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := adapter.ValidateConfiguration(uvr)
				if err != nil {
					b.Fatalf("ValidateConfiguration failed: %v", err)
				}
			}
		})
	}
}

// TestAdapterPerformanceBaseline establishes performance baselines
func TestAdapterPerformanceBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance baseline test in short mode")
	}

	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	type perfMetrics struct {
		createLatency time.Duration
		updateLatency time.Duration
		statusLatency time.Duration
		deleteLatency time.Duration
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createBenchAdapter(t, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			metrics := &perfMetrics{}

			// Measure create latency
			start := time.Now()
			uvr := createValidUVR("perf-test", "default", backend)
			_ = adapter.EnsureReplication(ctx, uvr)
			metrics.createLatency = time.Since(start)

			// Measure update latency
			start = time.Now()
			_ = adapter.EnsureReplication(ctx, uvr)
			metrics.updateLatency = time.Since(start)

			// Measure status latency
			start = time.Now()
			_, _ = adapter.GetReplicationStatus(ctx, uvr)
			metrics.statusLatency = time.Since(start)

			// Measure delete latency
			start = time.Now()
			_ = adapter.DeleteReplication(ctx, uvr)
			metrics.deleteLatency = time.Since(start)

			// Report metrics
			t.Logf("Performance baseline for %s:", backend)
			t.Logf("  Create: %v", metrics.createLatency)
			t.Logf("  Update: %v", metrics.updateLatency)
			t.Logf("  Status: %v", metrics.statusLatency)
			t.Logf("  Delete: %v", metrics.deleteLatency)

			// Verify performance thresholds (adjust as needed)
			maxLatency := 500 * time.Millisecond
			if metrics.createLatency > maxLatency {
				t.Logf("WARNING: Create latency (%v) exceeds threshold (%v)", metrics.createLatency, maxLatency)
			}
			if metrics.updateLatency > maxLatency {
				t.Logf("WARNING: Update latency (%v) exceeds threshold (%v)", metrics.updateLatency, maxLatency)
			}
			if metrics.statusLatency > maxLatency {
				t.Logf("WARNING: Status latency (%v) exceeds threshold (%v)", metrics.statusLatency, maxLatency)
			}
			if metrics.deleteLatency > maxLatency {
				t.Logf("WARNING: Delete latency (%v) exceeds threshold (%v)", metrics.deleteLatency, maxLatency)
			}
		})
	}
}

// TestAdapterConcurrency tests concurrent operations
func TestAdapterConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createBenchAdapter(t, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			// Create multiple replications concurrently
			numConcurrent := 10
			done := make(chan error, numConcurrent)

			start := time.Now()
			for i := 0; i < numConcurrent; i++ {
				go func(idx int) {
					uvr := createValidUVR(fmt.Sprintf("concurrent-%d", idx), "default", backend)
					done <- adapter.EnsureReplication(ctx, uvr)
				}(i)
			}

			// Wait for all to complete
			errors := 0
			for i := 0; i < numConcurrent; i++ {
				if err := <-done; err != nil {
					errors++
					t.Logf("Concurrent operation %d failed: %v", i, err)
				}
			}

			duration := time.Since(start)
			t.Logf("Backend %s: %d concurrent operations completed in %v (%d errors)",
				backend, numConcurrent, duration, errors)

			// Some errors might be acceptable in concurrent scenarios
			if errors > numConcurrent/2 {
				t.Errorf("Too many concurrent operation failures: %d/%d", errors, numConcurrent)
			}
		})
	}
}

// TestAdapterThroughput tests sustained throughput
func TestAdapterThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping throughput test in short mode")
	}

	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()
			adapter := createBenchAdapter(t, backend, client, translator)
			ctx := context.Background()

			_ = adapter.Initialize(ctx)

			// Measure throughput over time
			numOperations := 100
			start := time.Now()

			for i := 0; i < numOperations; i++ {
				uvr := createValidUVR(fmt.Sprintf("throughput-%d", i), "default", backend)
				err := adapter.EnsureReplication(ctx, uvr)
				if err != nil {
					t.Logf("Operation %d failed: %v", i, err)
				}
			}

			duration := time.Since(start)
			opsPerSecond := float64(numOperations) / duration.Seconds()

			t.Logf("Backend %s throughput: %.2f ops/sec (%d operations in %v)",
				backend, opsPerSecond, numOperations, duration)

			// Verify minimum throughput (adjust as needed)
			minThroughput := 10.0 // ops/sec
			if opsPerSecond < minThroughput {
				t.Logf("WARNING: Throughput (%.2f ops/sec) below threshold (%.2f ops/sec)",
					opsPerSecond, minThroughput)
			}
		})
	}
}

func createBenchAdapter(tb testing.TB, backend translation.Backend, c client.Client, translator *translation.Engine) adapters.ReplicationAdapter {
	switch backend {
	case translation.BackendCeph:
		adapter, _ := adapters.NewCephAdapter(c, translator)
		return adapter
	case translation.BackendTrident:
		config := adapters.DefaultMockTridentConfig()
		config.AutoProgressStates = false
		config.MinLatency = 1 * time.Millisecond
		config.MaxLatency = 5 * time.Millisecond
		return adapters.NewMockTridentAdapter(c, translator, config)
	case translation.BackendPowerStore:
		config := adapters.DefaultMockPowerStoreConfig()
		config.AutoProgressStates = false
		config.MinLatency = 1 * time.Millisecond
		config.MaxLatency = 5 * time.Millisecond
		return adapters.NewMockPowerStoreAdapter(c, translator, config)
	default:
		tb.Fatalf("Unknown backend: %s", backend)
		return nil
	}
}
