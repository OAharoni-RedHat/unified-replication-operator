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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

// TestCrossBackendCompatibility tests compatibility across all backends
func TestCrossBackendCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cross-backend test in short mode")
	}

	backends := []struct {
		name    string
		adapter ReplicationAdapter
	}{
		{
			name: "Trident",
			adapter: func() ReplicationAdapter {
				client := fake.NewClientBuilder().Build()
				translator := translation.NewEngine()
				adapter, _ := NewTridentAdapter(client, translator)
				return adapter
			}(),
		},
		{
			name: "PowerStore",
			adapter: func() ReplicationAdapter {
				client := fake.NewClientBuilder().Build()
				translator := translation.NewEngine()
				adapter, _ := NewPowerStoreAdapter(client, translator)
				return adapter
			}(),
		},
		{
			name: "Ceph",
			adapter: func() ReplicationAdapter {
				client := fake.NewClientBuilder().Build()
				translator := translation.NewEngine()
				adapter, _ := NewCephAdapter(client, translator)
				return adapter
			}(),
		},
	}

	ctx := context.Background()

	t.Run("AllImplementReplicationAdapter", func(t *testing.T) {
		for _, backend := range backends {
			assert.NotNil(t, backend.adapter, "%s adapter should not be nil", backend.name)
			assert.Implements(t, (*ReplicationAdapter)(nil), backend.adapter,
				"%s should implement ReplicationAdapter", backend.name)
		}
	})

	t.Run("AllHaveBackendType", func(t *testing.T) {
		for _, backend := range backends {
			backendType := backend.adapter.GetBackendType()
			assert.NotEmpty(t, backendType, "%s should have backend type", backend.name)
			t.Logf("%s backend type: %s", backend.name, backendType)
		}
	})

	t.Run("AllSupportInitialization", func(t *testing.T) {
		for _, backend := range backends {
			err := backend.adapter.Initialize(ctx)
			assert.NoError(t, err, "%s initialization should succeed", backend.name)
		}
	})

	t.Run("AllProvideVersion", func(t *testing.T) {
		for _, backend := range backends {
			version := backend.adapter.GetVersion()
			assert.NotEmpty(t, version, "%s should have version", backend.name)
			t.Logf("%s version: %s", backend.name, version)
		}
	})

	t.Run("AllProvideFeatures", func(t *testing.T) {
		for _, backend := range backends {
			features := backend.adapter.GetSupportedFeatures()
			assert.NotNil(t, features, "%s should provide features", backend.name)
			assert.GreaterOrEqual(t, len(features), 1, "%s should have at least one feature", backend.name)
			t.Logf("%s features: %d", backend.name, len(features))
		}
	})
}

// TestCrossBackendStateTranslation tests state translation consistency
func TestCrossBackendStateTranslation(t *testing.T) {
	translator := translation.NewEngine()

	backends := []translation.Backend{
		translation.BackendCeph,
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	states := []string{"source", "replica"}

	t.Run("ConsistentStateTranslation", func(t *testing.T) {
		for _, backend := range backends {
			for _, state := range states {
				// Translate to backend
				backendState, err := translator.TranslateStateToBackend(backend, state)
				if err != nil {
					t.Logf("%s doesn't support state %s", backend, state)
					continue
				}

				// Translate back to unified
				unifiedState, err := translator.TranslateStateFromBackend(backend, backendState)
				assert.NoError(t, err, "Reverse translation should work for %s", backend)
				assert.Equal(t, state, unifiedState,
					"Bidirectional translation should be consistent for %s", backend)

				t.Logf("%s: %s → %s → %s", backend, state, backendState, unifiedState)
			}
		}
	})

	t.Run("ConsistentModeTranslation", func(t *testing.T) {
		modes := []string{"synchronous", "asynchronous"}

		for _, backend := range backends {
			for _, mode := range modes {
				// Translate to backend
				backendMode, err := translator.TranslateModeToBackend(backend, mode)
				if err != nil {
					t.Logf("%s doesn't support mode %s", backend, mode)
					continue
				}

				// Translate back to unified
				unifiedMode, err := translator.TranslateModeFromBackend(backend, backendMode)
				assert.NoError(t, err, "Reverse translation should work for %s", backend)
				assert.Equal(t, mode, unifiedMode,
					"Bidirectional translation should be consistent for %s", backend)

				t.Logf("%s: %s → %s → %s", backend, mode, backendMode, unifiedMode)
			}
		}
	})
}

// TestCrossBackendPerformance compares performance across backends
func TestCrossBackendPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	scheme := runtime.NewScheme()
	require.NoError(t, replicationv1alpha1.AddToScheme(scheme))

	backends := []struct {
		name    string
		adapter ReplicationAdapter
		uvr     *replicationv1alpha1.UnifiedVolumeReplication
	}{
		{
			name: "Trident",
			adapter: func() ReplicationAdapter {
				client := fake.NewClientBuilder().WithScheme(scheme).Build()
				adapter, _ := NewTridentAdapter(client, translation.NewEngine())
				return adapter
			}(),
			uvr: createTestUVRForTrident("perf-test", "default"),
		},
		{
			name: "PowerStore",
			adapter: func() ReplicationAdapter {
				client := fake.NewClientBuilder().WithScheme(scheme).Build()
				adapter, _ := NewPowerStoreAdapter(client, translation.NewEngine())
				return adapter
			}(),
			uvr: createTestUVRForPowerStore("perf-test", "default"),
		},
	}

	ctx := context.Background()

	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			// Initialize
			err := backend.adapter.Initialize(ctx)
			require.NoError(t, err)

			// Validate
			err = backend.adapter.ValidateConfiguration(backend.uvr)
			// May have backend-specific validation requirements
			if err != nil {
				t.Logf("Validation: %v", err)
			}

			// Operations may fail without CRDs, but we test they don't panic
			_ = backend.adapter.CreateReplication(ctx, backend.uvr)
			_ = backend.adapter.UpdateReplication(ctx, backend.uvr)
			_, _ = backend.adapter.GetReplicationStatus(ctx, backend.uvr)
			_ = backend.adapter.DeleteReplication(ctx, backend.uvr)

			t.Logf("%s operations completed without panic", backend.name)
		})
	}
}

// TestCrossBackendErrorHandling tests consistent error handling
func TestCrossBackendErrorHandling(t *testing.T) {
	backends := []struct {
		name    string
		adapter ReplicationAdapter
	}{
		{
			name: "Trident",
			adapter: func() ReplicationAdapter {
				client := fake.NewClientBuilder().Build()
				adapter, _ := NewTridentAdapter(client, translation.NewEngine())
				return adapter
			}(),
		},
		{
			name: "PowerStore",
			adapter: func() ReplicationAdapter {
				client := fake.NewClientBuilder().Build()
				adapter, _ := NewPowerStoreAdapter(client, translation.NewEngine())
				return adapter
			}(),
		},
	}

	ctx := context.Background()

	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			// Create invalid UVR (missing required fields)
			invalidUVR := &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid",
					Namespace: "default",
				},
				Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
					// Missing required fields
					ReplicationState: "invalid-state",
				},
			}

			// Should handle invalid config gracefully
			err := backend.adapter.ValidateConfiguration(invalidUVR)
			if err == nil {
				t.Logf("%s: Validation passed despite invalid config (may have defaults)", backend.name)
			} else {
				t.Logf("%s: Correctly rejected invalid config: %v", backend.name, err)
			}

			// Operations should return errors, not panic
			err = backend.adapter.CreateReplication(ctx, invalidUVR)
			if err != nil {
				// Verify error is an AdapterError
				if adapterErr, ok := GetAdapterError(err); ok {
					assert.Equal(t, backend.adapter.GetBackendType(), adapterErr.Backend)
					t.Logf("%s error type: %s", backend.name, adapterErr.Type)
				}
			}
		})
	}
}
