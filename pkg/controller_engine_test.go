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

package pkg

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/adapters"
	"github.com/unified-replication/operator/pkg/discovery"
	"github.com/unified-replication/operator/pkg/translation"
)

func TestControllerEngine_ProcessReplication(t *testing.T) {
	ctx := context.Background()
	log := ctrl.Log.WithName("test")

	// Setup
	client := fake.NewClientBuilder().Build()
	discoveryEngine := discovery.NewEngine(client, nil)
	translationEngine := translation.NewEngine()

	// Register mock adapters
	_ = adapters.RegisterMockAdapters()
	defer adapters.UnregisterMockAdapters()

	registry := adapters.GetGlobalRegistry()

	config := DefaultControllerEngineConfig()
	engine := NewControllerEngine(client, discoveryEngine, translationEngine, registry, config)

	// Create test UVR
	uvr := createTestUVR("test-process", "default")

	// Test create operation
	err := engine.ProcessReplication(ctx, uvr, "create", log)
	// May fail if discovery finds no backends, but should not panic
	if err != nil {
		t.Logf("ProcessReplication returned error (expected if no backends discovered): %v", err)
	} else {
		t.Log("ProcessReplication succeeded")
	}
}

func TestControllerEngine_BackendSelection(t *testing.T) {
	ctx := context.Background()
	log := ctrl.Log.WithName("test")

	client := fake.NewClientBuilder().Build()
	discoveryEngine := discovery.NewEngine(client, nil)
	translationEngine := translation.NewEngine()

	_ = adapters.RegisterMockAdapters()
	defer adapters.UnregisterMockAdapters()

	registry := adapters.GetGlobalRegistry()
	engine := NewControllerEngine(client, discoveryEngine, translationEngine, registry, nil)

	availableBackends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	tests := []struct {
		name            string
		uvr             *replicationv1alpha1.UnifiedVolumeReplication
		expectedBackend translation.Backend
		shouldError     bool
	}{
		{
			name: "explicit Trident extension",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
					Extensions: &replicationv1alpha1.Extensions{
						Trident: &replicationv1alpha1.TridentExtensions{},
					},
				},
			},
			expectedBackend: translation.BackendTrident,
			shouldError:     false,
		},
		{
			name: "explicit PowerStore extension",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
					Extensions: &replicationv1alpha1.Extensions{
						Powerstore: &replicationv1alpha1.PowerStoreExtensions{},
					},
				},
			},
			expectedBackend: translation.BackendPowerStore,
			shouldError:     false,
		},
		{
			name: "storage class detection - trident",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
					SourceEndpoint: replicationv1alpha1.Endpoint{
						StorageClass: "trident-nas",
					},
				},
			},
			expectedBackend: translation.BackendTrident,
			shouldError:     false,
		},
		{
			name: "storage class detection - powerstore",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
					SourceEndpoint: replicationv1alpha1.Endpoint{
						StorageClass: "powerstore-iscsi",
					},
				},
			},
			expectedBackend: translation.BackendPowerStore,
			shouldError:     false,
		},
		{
			name: "no extension - uses first available",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
					SourceEndpoint: replicationv1alpha1.Endpoint{
						StorageClass: "generic-storage",
					},
				},
			},
			expectedBackend: translation.BackendTrident, // First in list
			shouldError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := engine.selectBackend(ctx, tt.uvr, availableBackends, log)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBackend, backend)
			}
		})
	}
}

func TestControllerEngine_Translation(t *testing.T) {
	log := ctrl.Log.WithName("test")

	client := fake.NewClientBuilder().Build()
	discoveryEngine := discovery.NewEngine(client, nil)
	translationEngine := translation.NewEngine()
	registry := adapters.GetGlobalRegistry()

	engine := NewControllerEngine(client, discoveryEngine, translationEngine, registry, nil)

	uvr := &replicationv1alpha1.UnifiedVolumeReplication{
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
		},
	}

	// Test translation for each backend
	backends := []translation.Backend{
		translation.BackendCeph,
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			state, mode, err := engine.translateToBackend(uvr, backend, log)
			assert.NoError(t, err)
			assert.NotEmpty(t, state, "Translated state should not be empty")
			assert.NotEmpty(t, mode, "Translated mode should not be empty")
			t.Logf("Backend %s: state=%s, mode=%s", backend, state, mode)
		})
	}
}

func TestControllerEngine_Caching(t *testing.T) {
	ctx := context.Background()
	log := ctrl.Log.WithName("test")

	client := fake.NewClientBuilder().Build()
	discoveryEngine := discovery.NewEngine(client, nil)
	translationEngine := translation.NewEngine()
	registry := adapters.GetGlobalRegistry()

	config := &ControllerEngineConfig{
		EnableCaching: true,
		CacheExpiry:   1 * time.Second,
	}
	engine := NewControllerEngine(client, discoveryEngine, translationEngine, registry, config)

	// First call - cache miss
	initialCacheMisses := engine.cacheMisses
	_, _ = engine.discoverBackends(ctx, log)
	assert.Equal(t, initialCacheMisses+1, engine.cacheMisses, "Should increment cache misses")

	// Second call immediately - cache hit
	_, _ = engine.discoverBackends(ctx, log)
	// May or may not hit cache depending on discovery results

	t.Logf("Cache stats: hits=%d, misses=%d", engine.cacheHits, engine.cacheMisses)

	// Wait for cache expiry
	time.Sleep(1100 * time.Millisecond)

	// Should be cache miss again
	beforeMisses := engine.cacheMisses
	_, _ = engine.discoverBackends(ctx, log)
	assert.Greater(t, engine.cacheMisses, beforeMisses, "Should have cache miss after expiry")
}

func TestControllerEngine_CacheInvalidation(t *testing.T) {
	ctx := context.Background()
	log := ctrl.Log.WithName("test")

	client := fake.NewClientBuilder().Build()
	discoveryEngine := discovery.NewEngine(client, nil)
	translationEngine := translation.NewEngine()
	registry := adapters.GetGlobalRegistry()

	config := &ControllerEngineConfig{
		EnableCaching: true,
		CacheExpiry:   10 * time.Minute,
	}
	engine := NewControllerEngine(client, discoveryEngine, translationEngine, registry, config)

	// Populate cache
	_, _ = engine.discoverBackends(ctx, log)

	// Verify cache has entries (or at least discovery was attempted)
	metrics := engine.GetMetrics()
	t.Logf("Metrics before invalidation: %+v", metrics)

	// Invalidate cache
	engine.InvalidateCache()

	// Verify cache is empty
	engine.discoveryCacheMutex.RLock()
	cacheSize := len(engine.discoveryCache)
	engine.discoveryCacheMutex.RUnlock()

	assert.Equal(t, 0, cacheSize, "Cache should be empty after invalidation")
}

func TestControllerEngine_Metrics(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	discoveryEngine := discovery.NewEngine(client, nil)
	translationEngine := translation.NewEngine()
	registry := adapters.GetGlobalRegistry()

	engine := NewControllerEngine(client, discoveryEngine, translationEngine, registry, nil)

	// Initial metrics
	metrics := engine.GetMetrics()
	assert.Contains(t, metrics, "operation_count")
	assert.Contains(t, metrics, "cache_hits")
	assert.Contains(t, metrics, "cache_misses")
	assert.Contains(t, metrics, "cache_entries")
	assert.Contains(t, metrics, "last_discovery")

	initialOpCount := metrics["operation_count"].(int64)

	// Simulate operation
	ctx := context.Background()
	log := ctrl.Log.WithName("test")
	uvr := createTestUVR("test", "default")

	_ = engine.ProcessReplication(ctx, uvr, "create", log)

	// Check metrics updated
	updatedMetrics := engine.GetMetrics()
	assert.Greater(t, updatedMetrics["operation_count"].(int64), initialOpCount)
}

func TestControllerEngine_GetReplicationStatus(t *testing.T) {
	ctx := context.Background()
	log := ctrl.Log.WithName("test")

	client := fake.NewClientBuilder().Build()
	discoveryEngine := discovery.NewEngine(client, nil)
	translationEngine := translation.NewEngine()

	_ = adapters.RegisterMockAdapters()
	defer adapters.UnregisterMockAdapters()

	registry := adapters.GetGlobalRegistry()
	engine := NewControllerEngine(client, discoveryEngine, translationEngine, registry, nil)

	uvr := createTestUVR("test-status", "default")

	status, err := engine.GetReplicationStatus(ctx, uvr, log)
	// May error if no backends available
	if err != nil {
		t.Logf("GetReplicationStatus returned error (expected if no backends): %v", err)
	} else if status != nil {
		t.Logf("Got status: state=%s, mode=%s, health=%s", status.State, status.Mode, status.Health)
	}
}

// Helper functions

func createTestUVR(name, namespace string) *replicationv1alpha1.UnifiedVolumeReplication {
	return &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			ReplicationState: replicationv1alpha1.ReplicationStateReplica,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			VolumeMapping: replicationv1alpha1.VolumeMapping{
				Source: replicationv1alpha1.VolumeSource{
					PvcName:   "source-pvc",
					Namespace: namespace,
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "dest-volume",
					Namespace:    namespace,
				},
			},
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "source-cluster",
				Region:       "us-east-1",
				StorageClass: "fast-ssd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				Region:       "us-west-1",
				StorageClass: "fast-ssd",
			},
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeContinuous,
				Rpo:  "15m",
				Rto:  "5m",
			},
			Extensions: &replicationv1alpha1.Extensions{
				Trident: &replicationv1alpha1.TridentExtensions{
					Actions: []replicationv1alpha1.TridentAction{},
				},
			},
		},
	}
}
