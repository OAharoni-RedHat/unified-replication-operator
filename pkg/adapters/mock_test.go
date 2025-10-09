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
	"github.com/stretchr/testify/require"

	"github.com/unified-replication/operator/pkg/translation"
)

func TestMockAdapter(t *testing.T) {
	client := createFakeClient()
	translator := translation.NewEngine()
	config := DefaultAdapterConfig(translation.BackendCeph)
	mockConfig := DefaultMockConfig()

	t.Run("NewMockAdapter", func(t *testing.T) {
		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendCeph, adapter.GetBackendType())
		assert.Contains(t, adapter.GetVersion(), "mock")

		features := adapter.GetSupportedFeatures()
		assert.Contains(t, features, FeatureAsyncReplication)
		assert.Contains(t, features, FeaturePromotion)
		assert.Contains(t, features, FeatureProgressTracking)
	})

	t.Run("CreateReplication", func(t *testing.T) {
		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-repl", "default")

		// Create replication
		err := adapter.EnsureReplication(ctx, uvr)
		assert.NoError(t, err)

		// Verify mock replication was created
		mockRepl, exists := adapter.GetMockReplication(uvr)
		assert.True(t, exists)
		assert.Equal(t, "test-repl", mockRepl.Name)
		assert.Equal(t, "replica", mockRepl.State)
		assert.Equal(t, "asynchronous", mockRepl.Mode)
		assert.Equal(t, ReplicationHealthHealthy, mockRepl.Health)

		// Try to ensure same replication again - should succeed (idempotent)
		err = adapter.EnsureReplication(ctx, uvr)
		assert.NoError(t, err) // EnsureReplication is idempotent, should succeed
	})

	t.Run("UpdateReplication", func(t *testing.T) {
		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-repl", "default")

		// Create replication first
		err := adapter.EnsureReplication(ctx, uvr)
		require.NoError(t, err)

		// Update replication state
		uvr.Spec.ReplicationState = "source"
		uvr.Generation = 2

		// Update using EnsureReplication (idempotent)
		err = adapter.EnsureReplication(ctx, uvr)
		assert.NoError(t, err)

		// Verify update
		mockRepl, exists := adapter.GetMockReplication(uvr)
		assert.True(t, exists)
		assert.Equal(t, "source", mockRepl.State)
		assert.Equal(t, int64(2), mockRepl.ObservedGeneration)

		// Ensure non-existing replication should create it (idempotent)
		uvr2 := createTestUVR("non-existing", "default")
		err = adapter.EnsureReplication(ctx, uvr2)
		assert.NoError(t, err) // EnsureReplication creates if doesn't exist
	})

	t.Run("DeleteReplication", func(t *testing.T) {
		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-repl", "default")

		// Create replication first
		err := adapter.EnsureReplication(ctx, uvr)
		require.NoError(t, err)

		// Delete replication
		err = adapter.DeleteReplication(ctx, uvr)
		assert.NoError(t, err)

		// Verify deletion
		_, exists := adapter.GetMockReplication(uvr)
		assert.False(t, exists)

		// Delete non-existing replication should fail
		err = adapter.DeleteReplication(ctx, uvr)
		assert.Error(t, err)
		assert.True(t, IsAdapterError(err))
	})

	t.Run("GetReplicationStatus", func(t *testing.T) {
		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-repl", "default")

		// Create replication first
		err := adapter.EnsureReplication(ctx, uvr)
		require.NoError(t, err)

		// Get status
		status, err := adapter.GetReplicationStatus(ctx, uvr)
		assert.NoError(t, err)
		assert.NotNil(t, status)
		assert.Equal(t, "replica", status.State)
		assert.Equal(t, "asynchronous", status.Mode)
		assert.Equal(t, ReplicationHealthHealthy, status.Health)
		assert.NotNil(t, status.LastSyncTime)
		assert.NotNil(t, status.SyncProgress)
		assert.NotEmpty(t, status.Conditions)

		// Get status for non-existing replication should fail
		uvr2 := createTestUVR("non-existing", "default")
		_, err = adapter.GetReplicationStatus(ctx, uvr2)
		assert.Error(t, err)
		assert.True(t, IsAdapterError(err))
	})

	t.Run("State management operations", func(t *testing.T) {
		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-repl", "default")

		// Create replication first
		err := adapter.EnsureReplication(ctx, uvr)
		require.NoError(t, err)

		// Test promotion
		err = adapter.PromoteReplica(ctx, uvr)
		assert.NoError(t, err)

		mockRepl, _ := adapter.GetMockReplication(uvr)
		assert.Equal(t, "promoting", mockRepl.State)

		// Test demotion
		err = adapter.DemoteSource(ctx, uvr)
		assert.NoError(t, err)

		mockRepl, _ = adapter.GetMockReplication(uvr)
		assert.Equal(t, "demoting", mockRepl.State)

		// Test resync
		err = adapter.ResyncReplication(ctx, uvr)
		assert.NoError(t, err)

		mockRepl, _ = adapter.GetMockReplication(uvr)
		assert.Equal(t, "syncing", mockRepl.State)

		// Test pause/resume
		err = adapter.PauseReplication(ctx, uvr)
		assert.NoError(t, err)

		mockRepl, _ = adapter.GetMockReplication(uvr)
		assert.Equal(t, "paused", mockRepl.State)

		err = adapter.ResumeReplication(ctx, uvr)
		assert.NoError(t, err)

		mockRepl, _ = adapter.GetMockReplication(uvr)
		assert.Equal(t, "syncing", mockRepl.State)

		// Test failover/failback
		err = adapter.FailoverReplication(ctx, uvr)
		assert.NoError(t, err)

		mockRepl, _ = adapter.GetMockReplication(uvr)
		assert.Equal(t, "source", mockRepl.State)

		err = adapter.FailbackReplication(ctx, uvr)
		assert.NoError(t, err)

		mockRepl, _ = adapter.GetMockReplication(uvr)
		assert.Equal(t, "replica", mockRepl.State)
	})

	t.Run("Failure simulation", func(t *testing.T) {
		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-repl", "default")

		// Set next operation to fail
		adapter.SetNextOperationShouldFail(true)

		err := adapter.EnsureReplication(ctx, uvr)
		assert.Error(t, err)
		assert.True(t, IsAdapterError(err))

		// Next operation should succeed (failure flag reset)
		err = adapter.EnsureReplication(ctx, uvr)
		assert.NoError(t, err)

		// Test failure rate
		adapter.SetFailureRate(1.0) // 100% failure rate

		// EnsureReplication should fail due to high failure rate
		uvr3 := createTestUVR("test-repl-3", "default")
		err = adapter.EnsureReplication(ctx, uvr3)
		assert.Error(t, err)
		assert.True(t, IsAdapterError(err))

		// Reset failure rate
		adapter.SetFailureRate(0.0)

		// Now EnsureReplication should succeed
		err = adapter.EnsureReplication(ctx, uvr3)
		assert.NoError(t, err)
	})

	t.Run("GetAllMockReplications", func(t *testing.T) {
		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		// Create multiple replications
		uvr1 := createTestUVR("test-repl-1", "default")
		uvr2 := createTestUVR("test-repl-2", "default")

		err := adapter.EnsureReplication(ctx, uvr1)
		require.NoError(t, err)

		err = adapter.EnsureReplication(ctx, uvr2)
		require.NoError(t, err)

		// Get all replications
		replications := adapter.GetAllMockReplications()
		assert.Len(t, replications, 2)
		assert.Contains(t, replications, "default/test-repl-1")
		assert.Contains(t, replications, "default/test-repl-2")
	})

	t.Run("Event generation", func(t *testing.T) {
		mockConfig := DefaultMockConfig()
		mockConfig.EventGeneration = true

		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-repl", "default")

		// Create replication
		err := adapter.EnsureReplication(ctx, uvr)
		require.NoError(t, err)

		// Check that creation event was generated
		mockRepl, _ := adapter.GetMockReplication(uvr)
		assert.Len(t, mockRepl.Events, 1)
		assert.Equal(t, EventTypeCreated, mockRepl.Events[0].Type)

		// Update replication state to generate event
		uvr.Spec.ReplicationState = "source"
		err = adapter.EnsureReplication(ctx, uvr)
		require.NoError(t, err)

		// Check that update event was generated
		mockRepl, _ = adapter.GetMockReplication(uvr)
		assert.Len(t, mockRepl.Events, 2)
		assert.Equal(t, EventTypeUpdated, mockRepl.Events[1].Type)

		// Promote replica to generate promotion event
		err = adapter.PromoteReplica(ctx, uvr)
		require.NoError(t, err)

		mockRepl, _ = adapter.GetMockReplication(uvr)
		assert.Len(t, mockRepl.Events, 3)
		assert.Equal(t, EventTypePromoted, mockRepl.Events[2].Type)
	})

	t.Run("Progress tracking", func(t *testing.T) {
		mockConfig := DefaultMockConfig()
		mockConfig.ProgressTracking = true
		mockConfig.StateTransitions = false // Disable auto transitions for this test

		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-repl", "default")

		err := adapter.EnsureReplication(ctx, uvr)
		require.NoError(t, err)

		mockRepl, _ := adapter.GetMockReplication(uvr)
		assert.NotNil(t, mockRepl.SyncProgress)
		assert.Equal(t, int64(1000000), mockRepl.SyncProgress.TotalBytes)
		assert.Equal(t, int64(0), mockRepl.SyncProgress.SyncedBytes)
		assert.Equal(t, 0.0, mockRepl.SyncProgress.PercentComplete)
		assert.Equal(t, "5m", mockRepl.SyncProgress.EstimatedTime)
	})

	t.Run("Latency simulation", func(t *testing.T) {
		mockConfig := DefaultMockConfig()
		mockConfig.LatencyMin = 50 * time.Millisecond
		mockConfig.LatencyMax = 100 * time.Millisecond

		adapter := NewMockAdapter(translation.BackendCeph, client, translator, config, mockConfig)
		ctx := context.Background()
		_ = adapter.Initialize(ctx)

		uvr := createTestUVR("test-repl", "default")

		// Measure operation time
		start := time.Now()
		err := adapter.EnsureReplication(ctx, uvr)
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.True(t, duration >= mockConfig.LatencyMin, "Operation should take at least minimum latency")
		assert.True(t, duration <= mockConfig.LatencyMax+10*time.Millisecond, "Operation should not exceed maximum latency significantly")
	})
}

func TestMockAdapterFactory(t *testing.T) {
	t.Run("NewMockAdapterFactory", func(t *testing.T) {
		mockConfig := DefaultMockConfig()
		factory := NewMockAdapterFactory(translation.BackendCeph, mockConfig)

		assert.NotNil(t, factory)
		assert.Equal(t, translation.BackendCeph, factory.GetBackendType())

		info := factory.GetInfo()
		assert.Contains(t, info.Name, "Mock")
		assert.Contains(t, info.Name, "Ceph")
		assert.Equal(t, translation.BackendCeph, info.Backend)
		assert.Contains(t, info.Version, "mock")
	})

	t.Run("CreateAdapter", func(t *testing.T) {
		mockConfig := DefaultMockConfig()
		factory := NewMockAdapterFactory(translation.BackendCeph, mockConfig)

		client := createFakeClient()
		translator := translation.NewEngine()
		config := DefaultAdapterConfig(translation.BackendCeph)

		adapter, err := factory.CreateAdapter(translation.BackendCeph, client, translator, config)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)

		// Verify it's a mock adapter
		mockAdapter, ok := adapter.(*MockAdapter)
		assert.True(t, ok)
		assert.Equal(t, translation.BackendCeph, mockAdapter.GetBackendType())
	})

	t.Run("ValidateConfig", func(t *testing.T) {
		mockConfig := DefaultMockConfig()
		factory := NewMockAdapterFactory(translation.BackendCeph, mockConfig)

		// Valid config
		config := DefaultAdapterConfig(translation.BackendCeph)
		err := factory.ValidateConfig(config)
		assert.NoError(t, err)

		// Invalid backend
		config.Backend = translation.BackendTrident
		err = factory.ValidateConfig(config)
		assert.Error(t, err)

		// Nil config
		err = factory.ValidateConfig(nil)
		assert.Error(t, err)
	})

	t.Run("Invalid mock config", func(t *testing.T) {
		// Invalid failure rate
		mockConfig := DefaultMockConfig()
		mockConfig.FailureRate = 1.5 // > 1.0
		factory := NewMockAdapterFactory(translation.BackendCeph, mockConfig)

		config := DefaultAdapterConfig(translation.BackendCeph)
		err := factory.ValidateConfig(config)
		assert.Error(t, err)

		// Invalid latency
		mockConfig = DefaultMockConfig()
		mockConfig.LatencyMin = 100 * time.Millisecond
		mockConfig.LatencyMax = 50 * time.Millisecond // Less than min
		factory = NewMockAdapterFactory(translation.BackendCeph, mockConfig)

		err = factory.ValidateConfig(config)
		assert.Error(t, err)
	})
}

func TestMockConfig(t *testing.T) {
	t.Run("DefaultMockConfig", func(t *testing.T) {
		config := DefaultMockConfig()
		assert.NotNil(t, config)
		assert.Equal(t, 0.0, config.FailureRate)
		assert.Equal(t, 10*time.Millisecond, config.LatencyMin)
		assert.Equal(t, 100*time.Millisecond, config.LatencyMax)
		assert.True(t, config.StateTransitions)
		assert.True(t, config.ProgressTracking)
		assert.True(t, config.EventGeneration)
	})

	t.Run("Custom mock config", func(t *testing.T) {
		config := &MockConfig{
			FailureRate:      0.1,
			LatencyMin:       5 * time.Millisecond,
			LatencyMax:       50 * time.Millisecond,
			StateTransitions: false,
			ProgressTracking: false,
			EventGeneration:  false,
		}

		assert.Equal(t, 0.1, config.FailureRate)
		assert.Equal(t, 5*time.Millisecond, config.LatencyMin)
		assert.Equal(t, 50*time.Millisecond, config.LatencyMax)
		assert.False(t, config.StateTransitions)
		assert.False(t, config.ProgressTracking)
		assert.False(t, config.EventGeneration)
	})
}

func TestReplicationStatus(t *testing.T) {
	t.Run("ReplicationStatus fields", func(t *testing.T) {
		now := time.Now()
		status := &ReplicationStatus{
			State:        "replica",
			Mode:         "asynchronous",
			Health:       ReplicationHealthHealthy,
			LastSyncTime: &now,
			NextSyncTime: &now,
			SyncProgress: &SyncProgress{
				TotalBytes:      1000,
				SyncedBytes:     500,
				PercentComplete: 50.0,
				EstimatedTime:   "2m",
			},
			BackendSpecific: map[string]interface{}{
				"test_field": "test_value",
			},
			Message:            "Test message",
			ObservedGeneration: 1,
			Conditions: []StatusCondition{
				{
					Type:               "Ready",
					Status:             "True",
					LastTransitionTime: now,
					Reason:             "TestReason",
					Message:            "Test condition",
				},
			},
		}

		assert.Equal(t, "replica", status.State)
		assert.Equal(t, "asynchronous", status.Mode)
		assert.Equal(t, ReplicationHealthHealthy, status.Health)
		assert.NotNil(t, status.LastSyncTime)
		assert.NotNil(t, status.NextSyncTime)
		assert.NotNil(t, status.SyncProgress)
		assert.Equal(t, 50.0, status.SyncProgress.PercentComplete)
		assert.Equal(t, "test_value", status.BackendSpecific["test_field"])
		assert.Equal(t, "Test message", status.Message)
		assert.Equal(t, int64(1), status.ObservedGeneration)
		assert.Len(t, status.Conditions, 1)
		assert.Equal(t, "Ready", status.Conditions[0].Type)
	})
}
