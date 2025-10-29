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

package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// Test helper functions
func createFakeClient() client.Client {
	scheme := runtime.NewScheme()
	_ = replicationv1alpha1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func createTestUVR(name, namespace string) *replicationv1alpha1.UnifiedVolumeReplication {
	return &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			ReplicationState: "replica",
			ReplicationMode:  "asynchronous",
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "source-cluster",
				Region:       "us-east-1",
				StorageClass: "ceph-rbd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				Region:       "us-west-1",
				StorageClass: "ceph-rbd",
			},
			VolumeMapping: replicationv1alpha1.VolumeMapping{
				Source: replicationv1alpha1.VolumeSource{
					PvcName:   "source-pvc",
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "dest-vol-123",
					Namespace:    "default",
				},
			},
			Schedule: replicationv1alpha1.Schedule{
				Mode: "interval",
				Rpo:  "30m",
				Rto:  "5m",
			},
		},
	}
}

func TestAdapterTypes(t *testing.T) {
	t.Run("AdapterFeature constants", func(t *testing.T) {
		features := []AdapterFeature{
			FeatureAsyncReplication,
			FeatureSyncReplication,
			FeatureEventualReplication,
			FeatureMetroReplication,
			FeaturePromotion,
			FeatureDemotion,
			FeatureResync,
			FeatureFailover,
			FeatureFailback,
			FeaturePauseResume,
			FeatureSnapshotBased,
			FeatureJournalBased,
			FeatureConsistencyGroups,
			FeatureVolumeGroups,
			FeatureAutoResync,
			FeatureScheduledSync,
			FeatureHighThroughput,
			FeatureLowLatency,
			FeatureMultiRegion,
			FeatureMultiCloud,
			FeatureMetrics,
			FeatureProgressTracking,
			FeatureRealTimeStatus,
		}

		for _, feature := range features {
			assert.NotEmpty(t, string(feature), "Feature constant should not be empty")
		}
	})

	t.Run("AdapterError functionality", func(t *testing.T) {
		err := NewAdapterError(ErrorTypeValidation, translation.BackendCeph, "validate", "test-resource", "validation failed")

		assert.Equal(t, ErrorTypeValidation, err.Type)
		assert.Equal(t, translation.BackendCeph, err.Backend)
		assert.Equal(t, "validate", err.Operation)
		assert.Equal(t, "test-resource", err.Resource)
		assert.Equal(t, "validation failed", err.Message)
		assert.False(t, err.IsRetryable())

		assert.Contains(t, err.Error(), "validation failed")
		assert.Contains(t, err.Error(), "ceph")
		assert.Contains(t, err.Error(), "validate")
	})

	t.Run("AdapterMetrics calculations", func(t *testing.T) {
		metrics := AdapterMetrics{
			TotalOperations: 100,
			SuccessfulOps:   85,
			FailedOps:       15,
		}

		assert.Equal(t, 85.0, metrics.CalculateSuccessRate())
		assert.True(t, metrics.IsHealthy()) // 85% success rate should be healthy

		metrics.SuccessfulOps = 70
		metrics.FailedOps = 30
		assert.Equal(t, 70.0, metrics.CalculateSuccessRate())
		assert.False(t, metrics.IsHealthy()) // 70% success rate should be unhealthy
	})

	t.Run("AdapterCapabilities feature checks", func(t *testing.T) {
		capabilities := AdapterCapabilities{
			Backend:         translation.BackendCeph,
			SupportedStates: []string{"source", "replica", "syncing"},
			SupportedModes:  []string{"asynchronous", "synchronous"},
			Features:        []AdapterFeature{FeatureAsyncReplication, FeaturePromotion},
		}

		assert.True(t, capabilities.SupportsFeature(FeatureAsyncReplication))
		assert.False(t, capabilities.SupportsFeature(FeatureSyncReplication))

		assert.True(t, capabilities.SupportsState("replica"))
		assert.False(t, capabilities.SupportsState("promoting"))

		assert.True(t, capabilities.SupportsMode("asynchronous"))
	})
}

func TestBaseAdapter(t *testing.T) {
	client := createFakeClient()
	translator := translation.NewEngine()
	config := DefaultAdapterConfig(translation.BackendCeph)

	t.Run("NewBaseAdapter", func(t *testing.T) {
		adapter := NewBaseAdapter(translation.BackendCeph, client, translator, config)
		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendCeph, adapter.GetBackendType())
		assert.Equal(t, "1.0.0", adapter.GetVersion())
		assert.False(t, adapter.IsHealthy()) // Not initialized yet
	})

	t.Run("Initialize and Cleanup", func(t *testing.T) {
		adapter := NewBaseAdapter(translation.BackendCeph, client, translator, config)
		ctx := context.Background()

		// Initialize
		err := adapter.Initialize(ctx)
		assert.NoError(t, err)
		assert.True(t, adapter.IsHealthy())

		// Initialize again should be no-op
		err = adapter.Initialize(ctx)
		assert.NoError(t, err)

		// Cleanup
		err = adapter.Cleanup(ctx)
		assert.NoError(t, err)
		assert.False(t, adapter.IsHealthy())
	})

	t.Run("ValidateConfiguration", func(t *testing.T) {
		adapter := NewBaseAdapter(translation.BackendCeph, client, translator, config)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-uvr", "default")

		// Valid configuration
		err := adapter.ValidateConfiguration(uvr)
		assert.NoError(t, err)

		// Nil UVR should fail
		err = adapter.ValidateConfiguration(nil)
		assert.Error(t, err)
		assert.True(t, IsAdapterError(err))
	})

	t.Run("SupportsConfiguration", func(t *testing.T) {
		adapter := NewBaseAdapter(translation.BackendCeph, client, translator, config)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-uvr", "default")

		supported, err := adapter.SupportsConfiguration(uvr)
		assert.NoError(t, err)
		assert.True(t, supported)

		// Test unsupported state
		uvr.Spec.ReplicationState = "unsupported-state"
		supported, err = adapter.SupportsConfiguration(uvr)
		assert.NoError(t, err)
		assert.False(t, supported)
	})

	t.Run("Translation methods", func(t *testing.T) {
		adapter := NewBaseAdapter(translation.BackendCeph, client, translator, config)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		// Test state translation
		backendState, err := adapter.TranslateState("source")
		assert.NoError(t, err)
		assert.Equal(t, "primary", backendState)

		// Test mode translation
		backendMode, err := adapter.TranslateMode("asynchronous")
		assert.NoError(t, err)
		assert.Equal(t, "async", backendMode)

		// Test reverse translation
		unifiedState, err := adapter.TranslateBackendState("primary")
		assert.NoError(t, err)
		assert.Equal(t, "source", unifiedState)

		unifiedMode, err := adapter.TranslateBackendMode("async")
		assert.NoError(t, err)
		assert.Equal(t, "asynchronous", unifiedMode)
	})

	t.Run("Metrics and Stats", func(t *testing.T) {
		adapter := NewBaseAdapter(translation.BackendCeph, client, translator, config)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		// Get initial metrics
		metrics := adapter.GetMetrics()
		assert.Equal(t, int64(0), metrics.TotalOperations)

		// Get stats
		stats := adapter.GetStats()
		assert.Equal(t, translation.BackendCeph, stats.Backend)
		assert.Equal(t, "1.0.0", stats.Version)
	})

	t.Run("WithRetry functionality", func(t *testing.T) {
		adapter := NewBaseAdapter(translation.BackendCeph, client, translator, config)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		// Test successful operation
		callCount := 0
		err := adapter.WithRetry(ctx, "test", func() error {
			callCount++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, callCount)

		// Test retryable error
		callCount = 0
		err = adapter.WithRetry(ctx, "test", func() error {
			callCount++
			if callCount < 3 {
				return NewAdapterError(ErrorTypeConnection, translation.BackendCeph, "test", "", "connection failed")
			}
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, callCount)

		// Test non-retryable error
		callCount = 0
		err = adapter.WithRetry(ctx, "test", func() error {
			callCount++
			return NewAdapterError(ErrorTypeValidation, translation.BackendCeph, "test", "", "validation failed")
		})
		assert.Error(t, err)
		assert.Equal(t, 1, callCount) // Should not retry
	})

	t.Run("ExecuteWithTimeout", func(t *testing.T) {
		config := DefaultAdapterConfig(translation.BackendCeph)
		config.Timeout = 100 * time.Millisecond
		adapter := NewBaseAdapter(translation.BackendCeph, client, translator, config)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		// Test successful operation within timeout
		err := adapter.ExecuteWithTimeout(ctx, "test", func(ctx context.Context) error {
			return nil
		})
		assert.NoError(t, err)

		// Test operation that times out
		err = adapter.ExecuteWithTimeout(ctx, "test", func(ctx context.Context) error {
			time.Sleep(200 * time.Millisecond)
			return nil
		})
		assert.Error(t, err)
		assert.True(t, IsAdapterError(err))
		adapterErr, _ := GetAdapterError(err)
		assert.Equal(t, ErrorTypeTimeout, adapterErr.Type)
	})
}

func TestRegistry(t *testing.T) {
	t.Run("NewRegistry", func(t *testing.T) {
		registry := NewRegistry()
		assert.NotNil(t, registry)
		assert.Empty(t, registry.GetSupportedBackends())
	})

	t.Run("RegisterFactory", func(t *testing.T) {
		registry := NewRegistry()
		factory := NewBaseAdapterFactory(translation.BackendCeph, "Test Adapter", "1.0.0", "Test adapter")

		// Register factory
		err := registry.RegisterFactory(factory)
		assert.NoError(t, err)
		assert.True(t, registry.IsBackendSupported(translation.BackendCeph))

		// Try to register same backend again
		err = registry.RegisterFactory(factory)
		assert.Error(t, err)

		// Register nil factory
		err = registry.RegisterFactory(nil)
		assert.Error(t, err)
	})

	t.Run("GetFactory", func(t *testing.T) {
		registry := NewRegistry()
		factory := NewBaseAdapterFactory(translation.BackendCeph, "Test Adapter", "1.0.0", "Test adapter")
		_ = registry.RegisterFactory(factory)

		// Get existing factory
		retrievedFactory, err := registry.GetFactory(translation.BackendCeph)
		assert.NoError(t, err)
		assert.Equal(t, factory, retrievedFactory)

		// Get non-existing factory
		_, err = registry.GetFactory(translation.BackendTrident)
		assert.Error(t, err)
	})

	t.Run("UnregisterFactory", func(t *testing.T) {
		registry := NewRegistry()
		factory := NewBaseAdapterFactory(translation.BackendCeph, "Test Adapter", "1.0.0", "Test adapter")
		_ = registry.RegisterFactory(factory)

		// Unregister existing factory
		err := registry.UnregisterFactory(translation.BackendCeph)
		assert.NoError(t, err)
		assert.False(t, registry.IsBackendSupported(translation.BackendCeph))

		// Unregister non-existing factory
		err = registry.UnregisterFactory(translation.BackendCeph)
		assert.Error(t, err)
	})

	t.Run("CreateAdapter", func(t *testing.T) {
		registry := NewRegistry()
		factory := NewBaseAdapterFactory(translation.BackendCeph, "Test Adapter", "1.0.0", "Test adapter")
		_ = registry.RegisterFactory(factory)

		client := createFakeClient()
		translator := translation.NewEngine()

		// Create adapter with valid backend
		adapter, err := registry.CreateAdapter(translation.BackendCeph, client, translator, nil)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendCeph, adapter.GetBackendType())

		// Try to create adapter for unsupported backend
		_, err = registry.CreateAdapter(translation.BackendTrident, client, translator, nil)
		assert.Error(t, err)
	})

	t.Run("ListFactories", func(t *testing.T) {
		registry := NewRegistry()
		factory1 := NewBaseAdapterFactory(translation.BackendCeph, "Ceph Adapter", "1.0.0", "Ceph adapter")
		factory2 := NewBaseAdapterFactory(translation.BackendTrident, "Trident Adapter", "1.0.0", "Trident adapter")

		_ = registry.RegisterFactory(factory1)
		_ = registry.RegisterFactory(factory2)

		factories := registry.ListFactories()
		assert.Len(t, factories, 2)
	})

	t.Run("GetAdapterInfo", func(t *testing.T) {
		registry := NewRegistry()
		factory := NewBaseAdapterFactory(translation.BackendCeph, "Test Adapter", "1.0.0", "Test adapter")
		_ = registry.RegisterFactory(factory)

		info, err := registry.GetAdapterInfo(translation.BackendCeph)
		assert.NoError(t, err)
		assert.Equal(t, "Test Adapter", info.Name)
		assert.Equal(t, translation.BackendCeph, info.Backend)
	})

	t.Run("Initialize and Shutdown", func(t *testing.T) {
		registry := NewRegistry()
		factory := NewBaseAdapterFactory(translation.BackendCeph, "Test Adapter", "1.0.0", "Test adapter")
		_ = registry.RegisterFactory(factory)

		ctx := context.Background()

		// Initialize
		err := registry.Initialize(ctx)
		assert.NoError(t, err)

		// Initialize again should be no-op
		err = registry.Initialize(ctx)
		assert.NoError(t, err)

		// Shutdown
		err = registry.Shutdown(ctx)
		assert.NoError(t, err)
	})
}

func TestAdapterManager(t *testing.T) {
	t.Run("NewAdapterManager", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewAdapterManager(registry, nil)
		assert.NotNil(t, manager)
	})

	t.Run("GetOrCreateAdapter", func(t *testing.T) {
		registry := NewRegistry()
		mockFactory := NewMockAdapterFactory(translation.BackendCeph, DefaultMockConfig())
		_ = registry.RegisterFactory(mockFactory)

		manager := NewAdapterManager(registry, DefaultManagerConfig())
		client := createFakeClient()
		translator := translation.NewEngine()
		ctx := context.Background()

		uvr := createTestUVR("test-uvr", "default")
		uvr.Spec.Extensions = &replicationv1alpha1.Extensions{
			Ceph: &replicationv1alpha1.CephExtensions{
				MirroringMode: stringPtr("journal"),
			},
		}

		// Create adapter
		adapter, err := manager.GetOrCreateAdapter(ctx, uvr, client, translator)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendCeph, adapter.GetBackendType())

		// Get same adapter again
		adapter2, err := manager.GetOrCreateAdapter(ctx, uvr, client, translator)
		assert.NoError(t, err)
		assert.Equal(t, adapter, adapter2) // Should be the same instance
	})

	t.Run("RemoveAdapter", func(t *testing.T) {
		registry := NewRegistry()
		mockFactory := NewMockAdapterFactory(translation.BackendCeph, DefaultMockConfig())
		_ = registry.RegisterFactory(mockFactory)

		manager := NewAdapterManager(registry, DefaultManagerConfig())
		client := createFakeClient()
		translator := translation.NewEngine()
		ctx := context.Background()

		uvr := createTestUVR("test-uvr", "default")
		uvr.Spec.Extensions = &replicationv1alpha1.Extensions{
			Ceph: &replicationv1alpha1.CephExtensions{
				MirroringMode: stringPtr("journal"),
			},
		}

		// Create adapter
		adapter, err := manager.GetOrCreateAdapter(ctx, uvr, client, translator)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)

		// Remove adapter
		err = manager.RemoveAdapter(ctx, uvr)
		assert.NoError(t, err)

		// Verify adapter is removed
		_, exists := manager.GetAdapter(uvr)
		assert.False(t, exists)
	})

	t.Run("Shutdown", func(t *testing.T) {
		registry := NewRegistry()
		mockFactory := NewMockAdapterFactory(translation.BackendCeph, DefaultMockConfig())
		_ = registry.RegisterFactory(mockFactory)

		manager := NewAdapterManager(registry, DefaultManagerConfig())
		client := createFakeClient()
		translator := translation.NewEngine()
		ctx := context.Background()

		uvr := createTestUVR("test-uvr", "default")
		uvr.Spec.Extensions = &replicationv1alpha1.Extensions{
			Ceph: &replicationv1alpha1.CephExtensions{
				MirroringMode: stringPtr("journal"),
			},
		}

		// Create some adapters
		_, err := manager.GetOrCreateAdapter(ctx, uvr, client, translator)
		assert.NoError(t, err)

		// Shutdown should cleanup all adapters
		err = manager.Shutdown(ctx)
		assert.NoError(t, err)

		stats := manager.GetStats()
		assert.Empty(t, stats)
	})
}

func TestGlobalRegistry(t *testing.T) {
	t.Run("GetGlobalRegistry", func(t *testing.T) {
		registry1 := GetGlobalRegistry()
		registry2 := GetGlobalRegistry()
		assert.Equal(t, registry1, registry2) // Should be the same instance
	})

	t.Run("RegisterAdapter", func(t *testing.T) {
		// FIX: Use new registry instance instead of global to avoid singleton conflicts
		registry := NewRegistry()
		factory := NewBaseAdapterFactory(translation.BackendPowerStore, "Test PowerStore Adapter", "1.0.0", "Test adapter")

		err := registry.RegisterFactory(factory)
		assert.NoError(t, err, "RegisterFactory should succeed on fresh registry")

		assert.True(t, registry.IsBackendSupported(translation.BackendPowerStore), "Backend should be supported after registration")
	})

	t.Run("CreateAdapterForBackend", func(t *testing.T) {
		// FIX: Use new registry instance for test isolation
		registry := NewRegistry()
		factory := NewBaseAdapterFactory(translation.BackendTrident, "Test Trident Adapter", "1.0.0", "Test adapter")
		err := registry.RegisterFactory(factory)
		assert.NoError(t, err, "RegisterFactory should succeed")

		client := createFakeClient()
		translator := translation.NewEngine()

		adapter, err := registry.CreateAdapter(translation.BackendTrident, client, translator, nil)
		assert.NoError(t, err, "CreateAdapter should succeed")
		assert.NotNil(t, adapter, "Adapter should not be nil")
		assert.Equal(t, translation.BackendTrident, adapter.GetBackendType(), "Adapter should have correct backend type")
	})
}

func TestAdapterConfig(t *testing.T) {
	t.Run("DefaultAdapterConfig", func(t *testing.T) {
		config := DefaultAdapterConfig(translation.BackendCeph)
		assert.NotNil(t, config)
		assert.Equal(t, translation.BackendCeph, config.Backend)
		assert.Equal(t, 30*time.Second, config.Timeout)
		assert.Equal(t, 3, config.RetryAttempts)
		assert.Equal(t, 5*time.Second, config.RetryDelay)
		assert.True(t, config.HealthCheckEnabled)
		assert.Equal(t, 1*time.Minute, config.HealthCheckInterval)
		assert.False(t, config.MetricsEnabled)
		assert.NotNil(t, config.CustomSettings)
	})
}

func TestDefaultManagerConfig(t *testing.T) {
	config := DefaultManagerConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 30*time.Second, config.DefaultTimeout)
	assert.Equal(t, 3, config.DefaultRetryAttempts)
	assert.True(t, config.HealthCheckEnabled)
	assert.True(t, config.MetricsEnabled)
}
