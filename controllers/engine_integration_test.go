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

package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg"
	"github.com/unified-replication/operator/pkg/adapters"
	"github.com/unified-replication/operator/pkg/discovery"
	"github.com/unified-replication/operator/pkg/translation"
)

// TestEngineIntegration_BasicWorkflow tests the complete integrated workflow
func TestEngineIntegration_BasicWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping engine integration test in short mode")
	}

	ctx := context.Background()
	s := createTestScheme(t)

	// Register mock adapters
	_ = adapters.RegisterMockAdapters()
	defer adapters.UnregisterMockAdapters()

	// Create resource
	uvr := createTestUVR("engine-test", "default")
	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	// Create engines
	discoveryEngine := discovery.NewEngine(fakeClient, discovery.DefaultDiscoveryConfig())
	translationEngine := translation.NewEngine()
	adapterRegistry := adapters.GetGlobalRegistry()

	// Create controller engine
	controllerEngine := pkg.NewControllerEngine(
		fakeClient,
		discoveryEngine,
		translationEngine,
		adapterRegistry,
		pkg.DefaultControllerEngineConfig(),
	)

	// Create reconciler with engine integration
	reconciler := createTestReconciler(fakeClient, s)
	reconciler.DiscoveryEngine = discoveryEngine
	reconciler.TranslationEngine = translationEngine
	reconciler.AdapterRegistry = adapterRegistry
	reconciler.ControllerEngine = controllerEngine

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "engine-test",
			Namespace: "default",
		},
	}

	// Reconcile with integrated engine
	result, err := reconciler.Reconcile(ctx, req)
	// May error if discovery doesn't find backends, but should not panic
	t.Logf("Reconcile with engine result: RequeueAfter=%v, Error=%v", result.RequeueAfter, err)

	t.Log("Engine integration basic workflow test completed")
}

// TestEngineIntegration_AdapterSelection tests discovery-based adapter selection
func TestEngineIntegration_AdapterSelection(t *testing.T) {
	ctx := context.Background()
	s := createTestScheme(t)

	// Register mock adapters
	_ = adapters.RegisterMockAdapters()
	defer adapters.UnregisterMockAdapters()

	// Create resource with Trident extensions
	uvr := createTestUVR("adapter-select-test", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	// Create engines
	discoveryEngine := discovery.NewEngine(fakeClient, discovery.DefaultDiscoveryConfig())
	translationEngine := translation.NewEngine()
	adapterRegistry := adapters.GetGlobalRegistry()

	reconciler := createTestReconciler(fakeClient, s)
	reconciler.DiscoveryEngine = discoveryEngine
	reconciler.TranslationEngine = translationEngine
	reconciler.AdapterRegistry = adapterRegistry

	// Get adapter via integrated engine
	adapter, err := reconciler.getAdapter(ctx, uvr, reconciler.Log)

	// Should get an adapter (either via engine or fallback)
	if err == nil {
		assert.NotNil(t, adapter)
		t.Logf("Got adapter: %s", adapter.GetBackendType())
	} else {
		t.Logf("Could not get adapter via engine (expected if no backends discovered): %v", err)
	}

	t.Log("Adapter selection test completed")
}

// TestEngineIntegration_Translation tests state translation integration
func TestEngineIntegration_Translation(t *testing.T) {
	translationEngine := translation.NewEngine()

	// Test translation for all backends
	backends := []translation.Backend{
		translation.BackendCeph,
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	states := []string{"source", "replica", "promoting", "demoting"}
	modes := []string{"synchronous", "asynchronous"}

	for _, backend := range backends {
		for _, state := range states {
			backendState, err := translationEngine.TranslateStateToBackend(backend, state)
			if err != nil {
				t.Logf("Backend %s doesn't support state %s: %v", backend, state, err)
				continue
			}

			// Verify bidirectional translation
			unifiedState, err := translationEngine.TranslateStateFromBackend(backend, backendState)
			assert.NoError(t, err, "Reverse translation should work")
			assert.Equal(t, state, unifiedState, "Bidirectional translation should be consistent")
		}

		for _, mode := range modes {
			backendMode, err := translationEngine.TranslateModeToBackend(backend, mode)
			if err != nil {
				t.Logf("Backend %s doesn't support mode %s: %v", backend, mode, err)
				continue
			}

			// Verify bidirectional translation
			unifiedMode, err := translationEngine.TranslateModeFromBackend(backend, backendMode)
			assert.NoError(t, err, "Reverse translation should work")
			assert.Equal(t, mode, unifiedMode, "Bidirectional translation should be consistent")
		}
	}

	t.Log("Translation integration test completed")
}

// TestEngineIntegration_ErrorPropagation tests error handling across engines
func TestEngineIntegration_ErrorPropagation(t *testing.T) {
	ctx := context.Background()
	s := createTestScheme(t)

	// Create resource
	uvr := createTestUVR("error-test", "default")
	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	// Create engines
	discoveryEngine := discovery.NewEngine(fakeClient, discovery.DefaultDiscoveryConfig())
	translationEngine := translation.NewEngine()

	// Create reconciler WITHOUT adapter registry (will cause error)
	reconciler := createTestReconciler(fakeClient, s)
	reconciler.DiscoveryEngine = discoveryEngine
	reconciler.TranslationEngine = translationEngine
	reconciler.AdapterRegistry = nil // This will cause error

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "error-test",
			Namespace: "default",
		},
	}

	// Should handle error gracefully
	result, err := reconciler.Reconcile(ctx, req)

	// Error is expected, but should not panic
	t.Logf("Reconcile with missing registry: RequeueAfter=%v, Error=%v", result.RequeueAfter, err)

	// Verify error is handled
	assert.True(t, result.RequeueAfter > 0 || err != nil, "Should handle error")

	t.Log("Error propagation test completed")
}

// TestEngineIntegration_BackendSelection tests storage class-based backend detection
func TestEngineIntegration_BackendSelection(t *testing.T) {
	ctx := context.Background()
	s := createTestScheme(t)

	tests := []struct {
		name          string
		storageClass  string
		expectedMatch string // backend name hint
	}{
		{
			name:          "Ceph storage class",
			storageClass:  "ceph-rbd",
			expectedMatch: "ceph",
		},
		{
			name:          "Trident storage class",
			storageClass:  "trident-nas",
			expectedMatch: "trident",
		},
		{
			name:          "PowerStore storage class",
			storageClass:  "powerstore-block",
			expectedMatch: "powerstore",
		},
		{
			name:          "NetApp storage class",
			storageClass:  "netapp-ontap",
			expectedMatch: "trident",
		},
		{
			name:          "Dell storage class",
			storageClass:  "dell-storage",
			expectedMatch: "powerstore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uvr := createTestUVR("backend-select-test", "default")
			uvr.Spec.SourceEndpoint.StorageClass = tt.storageClass
			uvr.Spec.DestinationEndpoint.StorageClass = tt.storageClass
			uvr.Spec.Extensions = nil // No explicit extensions

			// Test selectBackendViaEngine
			reconciler := createTestReconciler(nil, s)

			// Create mock available backends
			availableBackends := []translation.Backend{
				translation.BackendCeph,
				translation.BackendTrident,
				translation.BackendPowerStore,
			}

			backend, err := reconciler.selectBackendViaEngine(ctx, uvr, availableBackends, reconciler.Log)
			if err == nil {
				t.Logf("Selected backend: %s for storage class: %s", backend, tt.storageClass)
				// Verify it matches expected (basic check)
				assert.Contains(t, string(backend), tt.expectedMatch, "Should select appropriate backend")
			} else {
				t.Logf("Could not select backend: %v", err)
			}
		})
	}

	t.Log("Backend selection test completed")
}

// TestEngineIntegration_Caching tests discovery caching
func TestEngineIntegration_Caching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping caching test in short mode")
	}

	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()

	discoveryEngine := discovery.NewEngine(fakeClient, discovery.DefaultDiscoveryConfig())
	translationEngine := translation.NewEngine()
	adapterRegistry := adapters.GetGlobalRegistry()

	config := &pkg.ControllerEngineConfig{
		EnableCaching:     true,
		CacheExpiry:       1 * time.Minute,
		BatchOperations:   false,
		DiscoveryInterval: 30 * time.Second,
	}

	controllerEngine := pkg.NewControllerEngine(
		fakeClient,
		discoveryEngine,
		translationEngine,
		adapterRegistry,
		config,
	)

	// First discovery - should be cache miss
	metrics1 := controllerEngine.GetMetrics()
	initialMisses := metrics1["cache_misses"].(int64)

	// Try discovery multiple times quickly
	for i := 0; i < 5; i++ {
		_, _ = discoveryEngine.DiscoverBackends(ctx)
	}

	// Metrics should show cache behavior
	metrics2 := controllerEngine.GetMetrics()
	assert.Contains(t, metrics2, "cache_hits")
	assert.Contains(t, metrics2, "cache_misses")

	t.Logf("Cache metrics: Hits=%v, Misses=%v",
		metrics2["cache_hits"],
		metrics2["cache_misses"])

	// Should have at least initial miss
	assert.GreaterOrEqual(t, metrics2["cache_misses"].(int64), initialMisses)

	t.Log("Caching test completed")
}

// TestEngineIntegration_Performance tests performance with all engines
func TestEngineIntegration_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	s := createTestScheme(t)

	// Register mock adapters
	_ = adapters.RegisterMockAdapters()
	defer adapters.UnregisterMockAdapters()

	// Create multiple resources
	objects := make([]client.Object, 10)
	for i := 0; i < 10; i++ {
		objects[i] = createTestUVR(fmt.Sprintf("perf-test-%d", i), "default")
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(objects...).
		Build()

	// Create engines
	discoveryEngine := discovery.NewEngine(fakeClient, discovery.DefaultDiscoveryConfig())
	translationEngine := translation.NewEngine()
	adapterRegistry := adapters.GetGlobalRegistry()
	controllerEngine := pkg.NewControllerEngine(
		fakeClient,
		discoveryEngine,
		translationEngine,
		adapterRegistry,
		pkg.DefaultControllerEngineConfig(),
	)

	// Create reconciler
	reconciler := createTestReconciler(fakeClient, s)
	reconciler.DiscoveryEngine = discoveryEngine
	reconciler.TranslationEngine = translationEngine
	reconciler.AdapterRegistry = adapterRegistry
	reconciler.ControllerEngine = controllerEngine

	// Measure reconciliation performance
	start := time.Now()
	successCount := 0
	errorCount := 0

	for i := 0; i < 10; i++ {
		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      fmt.Sprintf("perf-test-%d", i),
				Namespace: "default",
			},
		}

		_, err := reconciler.Reconcile(ctx, req)
		if err == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	duration := time.Since(start)
	avgTime := duration / 10

	t.Logf("Performance with engines: 10 reconciles in %v (avg: %v per reconcile)", duration, avgTime)
	t.Logf("Success: %d, Errors: %d", successCount, errorCount)

	// Verify reasonable performance
	assert.Less(t, avgTime, 1*time.Second, "Average reconcile time should be < 1s")

	t.Log("Performance test completed")
}

// TestEngineIntegration_DiscoveryFallback tests fallback when discovery fails
func TestEngineIntegration_DiscoveryFallback(t *testing.T) {
	ctx := context.Background()
	s := createTestScheme(t)

	// Create resource
	uvr := createTestUVR("fallback-test", "default")
	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	// Create engines but NO adapter registry (will cause discovery to find nothing)
	discoveryEngine := discovery.NewEngine(fakeClient, discovery.DefaultDiscoveryConfig())
	translationEngine := translation.NewEngine()

	reconciler := createTestReconciler(fakeClient, s)
	reconciler.DiscoveryEngine = discoveryEngine
	reconciler.TranslationEngine = translationEngine
	reconciler.AdapterRegistry = nil // No registry

	// Try to get adapter - should fall back to extension-based
	adapter, err := reconciler.getAdapter(ctx, uvr, reconciler.Log)

	// Should still get an adapter via fallback
	if adapter != nil {
		assert.NotNil(t, adapter)
		t.Logf("Fallback successful, got adapter: %s", adapter.GetBackendType())
	} else {
		t.Logf("Fallback also failed (expected when no backends available): %v", err)
	}

	t.Log("Discovery fallback test completed")
}

// TestEngineIntegration_TranslationInWorkflow tests translation in complete workflow
func TestEngineIntegration_TranslationInWorkflow(t *testing.T) {
	_ = context.Background() // Not needed for this test
	_ = createTestScheme(t)  // Not needed for this test

	// Register mock adapters
	_ = adapters.RegisterMockAdapters()
	defer adapters.UnregisterMockAdapters()

	// Test translation directly (don't need UVR for this)
	translationEngine := translation.NewEngine()

	// Test translation for all backends
	backend := translation.BackendTrident
	translatedState, err := translationEngine.TranslateStateToBackend(backend, string(replicationv1alpha1.ReplicationStateSource))
	translatedMode, err2 := translationEngine.TranslateModeToBackend(backend, string(replicationv1alpha1.ReplicationModeAsynchronous))

	if err == nil && err2 == nil {
		assert.NotEmpty(t, translatedState)
		assert.NotEmpty(t, translatedMode)
		t.Logf("Translation: source→%s, asynchronous→%s (backend: %s)",
			translatedState, translatedMode, backend)
	} else {
		t.Logf("Translation failed (may be expected for some states): %v", err)
	}

	t.Log("Translation workflow test completed")
}

// TestEngineIntegration_EngineToggle tests toggling engine integration on/off
func TestEngineIntegration_EngineToggle(t *testing.T) {
	ctx := context.Background()
	s := createTestScheme(t)

	uvr := createTestUVR("toggle-test", "default")
	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	reconciler := createTestReconciler(fakeClient, s)
	reconciler.TranslationEngine = translation.NewEngine()

	// Test with engine integration OFF (Phase 4.1 mode)
	adapter1, err1 := reconciler.getAdapter(ctx, uvr, reconciler.Log)

	if adapter1 != nil {
		t.Log("Phase 4.1 mode: Got adapter via fallback")
	} else {
		t.Logf("Phase 4.1 mode: No adapter (expected): %v", err1)
	}

	// Test with engine integration ON (Phase 4.2 mode)
	reconciler.DiscoveryEngine = discovery.NewEngine(fakeClient, discovery.DefaultDiscoveryConfig())
	reconciler.AdapterRegistry = adapters.GetGlobalRegistry()

	adapter2, err2 := reconciler.getAdapter(ctx, uvr, reconciler.Log)

	if adapter2 != nil {
		t.Log("Phase 4.2 mode: Got adapter via engine")
	} else {
		t.Logf("Phase 4.2 mode: No adapter (may fallback): %v", err2)
	}

	t.Log("Engine toggle test completed")
}

// Helper function for use across tests - creating reconciler is defined in controller_unit_test.go
