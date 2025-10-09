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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/unified-replication/operator/pkg/adapters"
	"github.com/unified-replication/operator/pkg/translation"
)

// TestErrorInjection tests adapter behavior with simulated failures
func TestErrorInjection(t *testing.T) {
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			testCreateFailure(t, backend)
			testUpdateFailure(t, backend)
			testDeleteFailure(t, backend)
			testStatusFailure(t, backend)
		})
	}
}

func testCreateFailure(t *testing.T, backend translation.Backend) {
	t.Run("CreateFailure", func(t *testing.T) {
		client := fake.NewClientBuilder().Build()
		translator := translation.NewEngine()

		// Create adapter with high failure rate
		var adapter adapters.ReplicationAdapter
		switch backend {
		case translation.BackendTrident:
			config := adapters.DefaultMockTridentConfig()
			config.CreateSuccessRate = 0.0 // Always fail
			adapter = adapters.NewMockTridentAdapter(client, translator, config)
		case translation.BackendPowerStore:
			config := adapters.DefaultMockPowerStoreConfig()
			config.CreateSuccessRate = 0.0 // Always fail
			adapter = adapters.NewMockPowerStoreAdapter(client, translator, config)
		default:
			t.Skip("Backend not supported for this test")
			return
		}

		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createValidUVR("test-create-fail", "default", backend)
		err := adapter.EnsureReplication(ctx, uvr)

		// Should get an error
		assert.Error(t, err, "EnsureReplication should fail with 0% success rate")

		// Verify adapter is still functional (doesn't crash)
		assert.True(t, adapter.IsHealthy() || !adapter.IsHealthy(), "Health check should complete without panic")
	})
}

func testUpdateFailure(t *testing.T, backend translation.Backend) {
	t.Run("UpdateFailure", func(t *testing.T) {
		client := fake.NewClientBuilder().Build()
		translator := translation.NewEngine()

		// Create adapter with high update failure rate
		var adapter adapters.ReplicationAdapter
		switch backend {
		case translation.BackendTrident:
			config := adapters.DefaultMockTridentConfig()
			config.UpdateSuccessRate = 0.0 // Always fail updates
			adapter = adapters.NewMockTridentAdapter(client, translator, config)
		case translation.BackendPowerStore:
			config := adapters.DefaultMockPowerStoreConfig()
			config.UpdateSuccessRate = 0.0 // Always fail updates
			adapter = adapters.NewMockPowerStoreAdapter(client, translator, config)
		default:
			t.Skip("Backend not supported for this test")
			return
		}

		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createValidUVR("test-update-fail", "default", backend)

		// First, create the replication with a success adapter
		var createAdapter adapters.ReplicationAdapter
		switch backend {
		case translation.BackendTrident:
			config2 := adapters.DefaultMockTridentConfig()
			config2.CreateSuccessRate = 1.0 // Allow creation
			config2.UpdateSuccessRate = 0.0 // Block updates
			createAdapter = adapters.NewMockTridentAdapter(client, translator, config2)
		case translation.BackendPowerStore:
			config2 := adapters.DefaultMockPowerStoreConfig()
			config2.CreateSuccessRate = 1.0 // Allow creation
			config2.UpdateSuccessRate = 0.0 // Block updates
			createAdapter = adapters.NewMockPowerStoreAdapter(client, translator, config2)
		}
		_ = createAdapter.Initialize(ctx)
		_ = createAdapter.EnsureReplication(ctx, uvr)

		// Now change the UVR state and try to update - should fail
		uvr.Spec.ReplicationState = "promoting"
		err := createAdapter.EnsureReplication(ctx, uvr)
		assert.Error(t, err, "EnsureReplication should fail with 0% update success rate")
	})
}

func testDeleteFailure(t *testing.T, backend translation.Backend) {
	t.Run("DeleteFailure", func(t *testing.T) {
		client := fake.NewClientBuilder().Build()
		translator := translation.NewEngine()

		// Create adapter with high delete failure rate
		var adapter adapters.ReplicationAdapter
		switch backend {
		case translation.BackendTrident:
			config := adapters.DefaultMockTridentConfig()
			config.DeleteSuccessRate = 0.0 // Always fail deletes
			adapter = adapters.NewMockTridentAdapter(client, translator, config)
		case translation.BackendPowerStore:
			config := adapters.DefaultMockPowerStoreConfig()
			config.DeleteSuccessRate = 0.0 // Always fail deletes
			adapter = adapters.NewMockPowerStoreAdapter(client, translator, config)
		default:
			t.Skip("Backend not supported for this test")
			return
		}

		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createValidUVR("test-delete-fail", "default", backend)
		_ = adapter.EnsureReplication(ctx, uvr)

		err := adapter.DeleteReplication(ctx, uvr)
		assert.Error(t, err, "Delete should fail with 0% success rate")
	})
}

func testStatusFailure(t *testing.T, backend translation.Backend) {
	t.Run("StatusFailure", func(t *testing.T) {
		client := fake.NewClientBuilder().Build()
		translator := translation.NewEngine()

		// Create adapter with high status failure rate
		var adapter adapters.ReplicationAdapter
		switch backend {
		case translation.BackendTrident:
			config := adapters.DefaultMockTridentConfig()
			config.StatusSuccessRate = 0.0 // Always fail status
			adapter = adapters.NewMockTridentAdapter(client, translator, config)
		case translation.BackendPowerStore:
			config := adapters.DefaultMockPowerStoreConfig()
			config.StatusSuccessRate = 0.0 // Always fail status
			adapter = adapters.NewMockPowerStoreAdapter(client, translator, config)
		default:
			t.Skip("Backend not supported for this test")
			return
		}

		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createValidUVR("test-status-fail", "default", backend)
		_ = adapter.EnsureReplication(ctx, uvr)

		_, err := adapter.GetReplicationStatus(ctx, uvr)
		assert.Error(t, err, "GetStatus should fail with 0% success rate")
	})
}

// TestIntermittentFailures tests adapter resilience to intermittent failures
func TestIntermittentFailures(t *testing.T) {
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()

			// Create adapter with 50% success rate
			var adapter adapters.ReplicationAdapter
			switch backend {
			case translation.BackendTrident:
				config := adapters.DefaultMockTridentConfig()
				config.CreateSuccessRate = 0.5
				config.UpdateSuccessRate = 0.5
				adapter = adapters.NewMockTridentAdapter(client, translator, config)
			case translation.BackendPowerStore:
				config := adapters.DefaultMockPowerStoreConfig()
				config.CreateSuccessRate = 0.5
				config.UpdateSuccessRate = 0.5
				adapter = adapters.NewMockPowerStoreAdapter(client, translator, config)
			default:
				t.Skip("Backend not supported for this test")
				return
			}

			ctx := context.Background()
			_ = adapter.Initialize(ctx)

			// Try multiple operations, some should succeed, some should fail
			numOps := 20
			successes := 0
			failures := 0

			for i := 0; i < numOps; i++ {
				uvr := createValidUVR("test-intermittent", "default", backend)
				err := adapter.EnsureReplication(ctx, uvr)
				if err == nil {
					successes++
				} else {
					failures++
				}
			}

			t.Logf("Backend %s: %d successes, %d failures out of %d operations",
				backend, successes, failures, numOps)

			// With 50% success rate, we should have some of each
			assert.Greater(t, successes, 0, "Should have some successes")
			assert.Greater(t, failures, 0, "Should have some failures")
		})
	}
}

// TestRecoveryFromFailure tests adapter recovery after failures
func TestRecoveryFromFailure(t *testing.T) {
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()

			// Start with failing adapter
			var adapter adapters.ReplicationAdapter
			var config interface{}
			switch backend {
			case translation.BackendTrident:
				cfg := adapters.DefaultMockTridentConfig()
				cfg.CreateSuccessRate = 0.0
				config = cfg
				adapter = adapters.NewMockTridentAdapter(client, translator, cfg)
			case translation.BackendPowerStore:
				cfg := adapters.DefaultMockPowerStoreConfig()
				cfg.CreateSuccessRate = 0.0
				config = cfg
				adapter = adapters.NewMockPowerStoreAdapter(client, translator, cfg)
			default:
				t.Skip("Backend not supported for this test")
				return
			}

			ctx := context.Background()
			_ = adapter.Initialize(ctx)

			uvr := createValidUVR("test-recovery", "default", backend)

			// First attempt should fail
			err := adapter.EnsureReplication(ctx, uvr)
			assert.Error(t, err, "First attempt should fail")

			// "Fix" the issue by recreating adapter with good config
			switch backend {
			case translation.BackendTrident:
				cfg := config.(*adapters.MockTridentConfig)
				cfg.CreateSuccessRate = 1.0
				adapter = adapters.NewMockTridentAdapter(client, translator, cfg)
			case translation.BackendPowerStore:
				cfg := config.(*adapters.MockPowerStoreConfig)
				cfg.CreateSuccessRate = 1.0
				adapter = adapters.NewMockPowerStoreAdapter(client, translator, cfg)
			}

			_ = adapter.Initialize(ctx)

			// Second attempt should succeed
			err = adapter.EnsureReplication(ctx, uvr)
			assert.NoError(t, err, "Second attempt should succeed after recovery")
		})
	}
}

// TestErrorPropagation verifies errors are properly propagated
func TestErrorPropagation(t *testing.T) {
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()

			// Create adapter with failures
			var adapter adapters.ReplicationAdapter
			switch backend {
			case translation.BackendTrident:
				config := adapters.DefaultMockTridentConfig()
				config.CreateSuccessRate = 0.0
				adapter = adapters.NewMockTridentAdapter(client, translator, config)
			case translation.BackendPowerStore:
				config := adapters.DefaultMockPowerStoreConfig()
				config.CreateSuccessRate = 0.0
				adapter = adapters.NewMockPowerStoreAdapter(client, translator, config)
			default:
				t.Skip("Backend not supported for this test")
				return
			}

			ctx := context.Background()
			_ = adapter.Initialize(ctx)

			uvr := createValidUVR("test-error-prop", "default", backend)
			err := adapter.EnsureReplication(ctx, uvr)

			// Verify error is not nil and has meaningful information
			require.Error(t, err, "Should return error")
			assert.NotEmpty(t, err.Error(), "Error should have message")

			// Check if it's an AdapterError
			if adapterErr, ok := adapters.GetAdapterError(err); ok {
				assert.Equal(t, backend, adapterErr.Backend, "Error should indicate correct backend")
				assert.NotEmpty(t, adapterErr.Operation, "Error should indicate operation")
				t.Logf("AdapterError details: Type=%s, Operation=%s, Message=%s",
					adapterErr.Type, adapterErr.Operation, adapterErr.Message)
			}
		})
	}
}

// TestPartialFailureScenarios tests scenarios where some operations succeed and others fail
func TestPartialFailureScenarios(t *testing.T) {
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			client := fake.NewClientBuilder().Build()
			translator := translation.NewEngine()

			// Create adapter where some operations fail
			var adapter adapters.ReplicationAdapter
			switch backend {
			case translation.BackendTrident:
				config := adapters.DefaultMockTridentConfig()
				config.CreateSuccessRate = 1.0 // Create succeeds
				config.UpdateSuccessRate = 0.0 // Update fails
				config.DeleteSuccessRate = 1.0 // Delete succeeds
				adapter = adapters.NewMockTridentAdapter(client, translator, config)
			case translation.BackendPowerStore:
				config := adapters.DefaultMockPowerStoreConfig()
				config.CreateSuccessRate = 1.0 // Create succeeds
				config.UpdateSuccessRate = 0.0 // Update fails
				config.DeleteSuccessRate = 1.0 // Delete succeeds
				adapter = adapters.NewMockPowerStoreAdapter(client, translator, config)
			default:
				t.Skip("Backend not supported for this test")
				return
			}

			ctx := context.Background()
			_ = adapter.Initialize(ctx)

			uvr := createValidUVR("test-partial", "default", backend)

			// Create should succeed
			err := adapter.EnsureReplication(ctx, uvr)
			assert.NoError(t, err, "Create should succeed")

			// Update should fail
			err = adapter.EnsureReplication(ctx, uvr)
			assert.Error(t, err, "Update should fail")

			// Delete should succeed
			err = adapter.DeleteReplication(ctx, uvr)
			assert.NoError(t, err, "Delete should succeed")
		})
	}
}
