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

// Package benchmarks provides comprehensive performance benchmarking for the unified replication operator
package benchmarks

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/test/fixtures"
	"github.com/unified-replication/operator/test/utils"
)

// BenchmarkSuite provides comprehensive benchmarking utilities
type BenchmarkSuite struct {
	TestClient    *utils.TestClient
	DataGenerator *utils.MockDataGenerator
	Tracker       *utils.PerformanceTracker
}

// NewBenchmarkSuite creates a new benchmark suite
func NewBenchmarkSuite() *BenchmarkSuite {
	return &BenchmarkSuite{
		TestClient:    utils.NewTestClient(),
		DataGenerator: utils.NewMockDataGenerator(time.Now().UnixNano()),
		Tracker:       utils.NewPerformanceTracker(),
	}
}

// BenchmarkValidation benchmarks the validation performance
func BenchmarkValidation(b *testing.B) {
	testCases := []struct {
		name string
		spec replicationv1alpha1.UnifiedVolumeReplicationSpec
	}{
		{"basic-spec", fixtures.BasicReplicationSpec()},
		{"ceph-spec", fixtures.CephReplicationSpec()},
		{"trident-spec", fixtures.TridentReplicationSpec()},
		{"powerstore-spec", fixtures.PowerStoreReplicationSpec()},
		{"multi-vendor-spec", fixtures.MultiVendorReplicationSpec()},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			uvr := &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "benchmark-test",
					Namespace: "default",
				},
				Spec: tc.spec,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				err := uvr.ValidateSpec()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCRDCreation benchmarks CRD creation performance
func BenchmarkCRDCreation(b *testing.B) {
	suite := NewBenchmarkSuite()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		uvr := utils.NewCRDBuilder().
			WithName(fmt.Sprintf("benchmark-crd-%d", i)).
			WithNamespace("default").
			Build()

		err := suite.TestClient.Client.Create(ctx, uvr)
		if err != nil {
			b.Fatal(err)
		}

		// Clean up to avoid accumulating resources
		_ = suite.TestClient.Client.Delete(ctx, uvr)
	}
}

// BenchmarkCRDUpdate benchmarks CRD update performance
func BenchmarkCRDUpdate(b *testing.B) {
	suite := NewBenchmarkSuite()
	ctx := context.Background()

	// Create a base CRD
	uvr := utils.NewCRDBuilder().
		WithName("benchmark-update-test").
		WithNamespace("default").
		Build()

	err := suite.TestClient.Client.Create(ctx, uvr)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	states := fixtures.ValidReplicationStates()

	for i := 0; i < b.N; i++ {
		// Update to a different state
		uvr.Spec.ReplicationState = states[i%len(states)]

		err := suite.TestClient.Client.Update(ctx, uvr)
		if err != nil {
			b.Fatal(err)
		}
	}

	// Cleanup
	_ = suite.TestClient.Client.Delete(ctx, uvr)
}

// BenchmarkCRDStatusUpdate benchmarks status update performance
func BenchmarkCRDStatusUpdate(b *testing.B) {
	suite := NewBenchmarkSuite()
	ctx := context.Background()

	// Create a base CRD
	uvr := utils.NewCRDBuilder().
		WithName("benchmark-status-test").
		WithNamespace("default").
		Build()

	err := suite.TestClient.Client.Create(ctx, uvr)
	require.NoError(b, err)

	conditions := fixtures.SampleConditions()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Update status with different observed generation
		uvr.Status = replicationv1alpha1.UnifiedVolumeReplicationStatus{
			Conditions:         conditions,
			ObservedGeneration: int64(i),
		}

		err := suite.TestClient.Client.Status().Update(ctx, uvr)
		if err != nil {
			b.Fatal(err)
		}
	}

	// Cleanup
	_ = suite.TestClient.Client.Delete(ctx, uvr)
}

// BenchmarkRandomCRDGeneration benchmarks random CRD generation
func BenchmarkRandomCRDGeneration(b *testing.B) {
	suite := NewBenchmarkSuite()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = suite.DataGenerator.GenerateRandomCRD()
	}
}

// BenchmarkComplexValidation benchmarks validation with all extensions
func BenchmarkComplexValidation(b *testing.B) {
	// Create a complex spec with all extensions
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "complex-validation-test",
			Namespace:   "default",
			Labels:      fixtures.SampleLabels(),
			Annotations: fixtures.SampleAnnotations(),
		},
		Spec: fixtures.MultiVendorReplicationSpec(),
		Status: replicationv1alpha1.UnifiedVolumeReplicationStatus{
			Conditions:         fixtures.SampleConditions(),
			ObservedGeneration: 1,
			DiscoveredBackends: fixtures.SampleBackends(),
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := uvr.ValidateSpec()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMemoryUsage benchmarks memory usage during operations
func BenchmarkMemoryUsage(b *testing.B) {
	suite := NewBenchmarkSuite()
	ctx := context.Background()

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		uvr := suite.DataGenerator.GenerateRandomCRD()

		err := suite.TestClient.Client.Create(ctx, uvr)
		if err != nil {
			b.Fatal(err)
		}

		err = uvr.ValidateSpec()
		if err != nil {
			b.Fatal(err)
		}

		_ = suite.TestClient.Client.Delete(ctx, uvr)
	}

	runtime.GC()
	runtime.ReadMemStats(&m2)

	b.ReportMetric(float64(m2.Alloc-m1.Alloc)/float64(b.N), "bytes/op")
}

// BenchmarkConcurrentOperations benchmarks concurrent CRD operations
func BenchmarkConcurrentOperations(b *testing.B) {
	suite := NewBenchmarkSuite()
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			uvr := utils.NewCRDBuilder().
				WithName(fmt.Sprintf("concurrent-test-%d-%d", b.N, i)).
				WithNamespace("default").
				Build()

			err := suite.TestClient.Client.Create(ctx, uvr)
			if err != nil {
				b.Fatal(err)
			}

			err = uvr.ValidateSpec()
			if err != nil {
				b.Fatal(err)
			}

			_ = suite.TestClient.Client.Delete(ctx, uvr)
			i++
		}
	})
}

// PerformanceTest represents a single performance test
type PerformanceTest struct {
	Name        string
	Description string
	Setup       func() interface{}
	Test        func(data interface{}) error
	Cleanup     func(data interface{})
	Iterations  int
	Timeout     time.Duration
}

// PerformanceTestSuite manages a collection of performance tests
type PerformanceTestSuite struct {
	tests   []PerformanceTest
	results map[string]PerformanceResult
}

// PerformanceResult holds the results of a performance test
type PerformanceResult struct {
	Name            string
	Iterations      int
	TotalDuration   time.Duration
	AverageDuration time.Duration
	MinDuration     time.Duration
	MaxDuration     time.Duration
	SuccessRate     float64
	ErrorCount      int
}

// NewPerformanceTestSuite creates a new performance test suite
func NewPerformanceTestSuite() *PerformanceTestSuite {
	return &PerformanceTestSuite{
		tests:   make([]PerformanceTest, 0),
		results: make(map[string]PerformanceResult),
	}
}

// AddTest adds a performance test to the suite
func (pts *PerformanceTestSuite) AddTest(test PerformanceTest) {
	pts.tests = append(pts.tests, test)
}

// RunTests runs all performance tests in the suite
func (pts *PerformanceTestSuite) RunTests(b *testing.B) {
	for _, test := range pts.tests {
		b.Run(test.Name, func(b *testing.B) {
			pts.runSingleTest(b, test)
		})
	}
}

// runSingleTest runs a single performance test
func (pts *PerformanceTestSuite) runSingleTest(b *testing.B, test PerformanceTest) {
	var data interface{}
	if test.Setup != nil {
		data = test.Setup()
	}

	defer func() {
		if test.Cleanup != nil {
			test.Cleanup(data)
		}
	}()

	iterations := test.Iterations
	if iterations == 0 {
		iterations = b.N
	}

	var (
		totalDuration time.Duration
		minDuration   = time.Duration(int64(^uint64(0) >> 1)) // Max duration
		maxDuration   time.Duration
		errorCount    int
	)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < iterations; i++ {
		start := time.Now()

		err := test.Test(data)
		if err != nil {
			errorCount++
		}

		duration := time.Since(start)
		totalDuration += duration

		if duration < minDuration {
			minDuration = duration
		}
		if duration > maxDuration {
			maxDuration = duration
		}
	}

	result := PerformanceResult{
		Name:            test.Name,
		Iterations:      iterations,
		TotalDuration:   totalDuration,
		AverageDuration: totalDuration / time.Duration(iterations),
		MinDuration:     minDuration,
		MaxDuration:     maxDuration,
		SuccessRate:     float64(iterations-errorCount) / float64(iterations) * 100,
		ErrorCount:      errorCount,
	}

	pts.results[test.Name] = result

	// Report custom metrics
	b.ReportMetric(float64(result.AverageDuration.Nanoseconds()), "ns/op")
	b.ReportMetric(result.SuccessRate, "%success")
}

// GetResults returns all performance test results
func (pts *PerformanceTestSuite) GetResults() map[string]PerformanceResult {
	return pts.results
}

// PrintResults prints performance test results
func (pts *PerformanceTestSuite) PrintResults() {
	fmt.Println("\n=== Performance Test Results ===")
	for name, result := range pts.results {
		fmt.Printf("\nTest: %s\n", name)
		fmt.Printf("  Iterations: %d\n", result.Iterations)
		fmt.Printf("  Total Duration: %v\n", result.TotalDuration)
		fmt.Printf("  Average Duration: %v\n", result.AverageDuration)
		fmt.Printf("  Min Duration: %v\n", result.MinDuration)
		fmt.Printf("  Max Duration: %v\n", result.MaxDuration)
		fmt.Printf("  Success Rate: %.2f%%\n", result.SuccessRate)
		fmt.Printf("  Error Count: %d\n", result.ErrorCount)
	}
}

// DefaultPerformanceTests returns a set of default performance tests
func DefaultPerformanceTests() []PerformanceTest {
	return []PerformanceTest{
		{
			Name:        "validation-performance",
			Description: "Tests validation performance with various specs",
			Setup: func() interface{} {
				return fixtures.BasicReplicationSpec()
			},
			Test: func(data interface{}) error {
				spec := data.(replicationv1alpha1.UnifiedVolumeReplicationSpec)
				uvr := &replicationv1alpha1.UnifiedVolumeReplication{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "perf-test",
						Namespace: "default",
					},
					Spec: spec,
				}
				return uvr.ValidateSpec()
			},
			Iterations: 1000,
			Timeout:    10 * time.Second,
		},
		{
			Name:        "crd-builder-performance",
			Description: "Tests CRD builder performance",
			Setup: func() interface{} {
				return utils.NewMockDataGenerator(time.Now().UnixNano())
			},
			Test: func(data interface{}) error {
				generator := data.(*utils.MockDataGenerator)
				_ = generator.GenerateRandomCRD()
				return nil
			},
			Iterations: 5000,
			Timeout:    5 * time.Second,
		},
		{
			Name:        "complex-validation-performance",
			Description: "Tests validation performance with complex specs",
			Setup: func() interface{} {
				return fixtures.MultiVendorReplicationSpec()
			},
			Test: func(data interface{}) error {
				spec := data.(replicationv1alpha1.UnifiedVolumeReplicationSpec)
				uvr := &replicationv1alpha1.UnifiedVolumeReplication{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "complex-perf-test",
						Namespace: "default",
					},
					Spec: spec,
				}
				return uvr.ValidateSpec()
			},
			Iterations: 1000,
			Timeout:    15 * time.Second,
		},
	}
}
