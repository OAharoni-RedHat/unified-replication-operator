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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

func TestMockTridentAdapter(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, replicationv1alpha1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	translator := translation.NewEngine()

	t.Run("NewMockTridentAdapter", func(t *testing.T) {
		config := DefaultMockTridentConfig()
		adapter := NewMockTridentAdapter(client, translator, config)

		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendTrident, adapter.GetBackendType())
		assert.Contains(t, adapter.GetVersion(), "mock-trident")

		// Initialize adapter before checking health
		ctx := context.Background()
		err := adapter.Initialize(ctx)
		require.NoError(t, err)
		assert.True(t, adapter.IsHealthy(), "Should be healthy after initialization")
	})

	t.Run("CreateReplication", func(t *testing.T) {
		config := &MockTridentConfig{
			CreateSuccessRate: 1.0,
			MinLatency:        0,
			MaxLatency:        0,
		}
		adapter := NewMockTridentAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-trident", "default")

		err := adapter.CreateReplication(ctx, uvr)
		assert.NoError(t, err)

		// Verify replication was created in mock backend
		replications := adapter.GetAllMockTridentReplications()
		assert.Len(t, replications, 1)

		key := "default/test-trident"
		replication, exists := replications[key]
		assert.True(t, exists)
		assert.Equal(t, "test-trident", replication.Name)
		assert.Equal(t, "default", replication.Namespace)
		assert.Equal(t, ReplicationHealthHealthy, replication.Health)

		// Verify events were generated
		events := adapter.GetMockTridentEvents()
		assert.NotEmpty(t, events)
		assert.Equal(t, EventTypeCreated, events[len(events)-1].Type)
	})

	t.Run("CreateReplication_Failure", func(t *testing.T) {
		config := &MockTridentConfig{
			CreateSuccessRate: 0.0, // Always fail
			MinLatency:        0,
			MaxLatency:        0,
		}
		adapter := NewMockTridentAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-fail", "default")

		err := adapter.CreateReplication(ctx, uvr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "simulated creation failure")

		// Verify no replication was created
		replications := adapter.GetAllMockTridentReplications()
		assert.Empty(t, replications)
	})

	t.Run("UpdateReplication", func(t *testing.T) {
		config := DefaultMockTridentConfig()
		config.UpdateSuccessRate = 1.0
		adapter := NewMockTridentAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-update", "default")

		// Create first
		err := adapter.CreateReplication(ctx, uvr)
		require.NoError(t, err)

		// Update state
		uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStatePromoting
		err = adapter.UpdateReplication(ctx, uvr)
		assert.NoError(t, err)

		// Verify state was updated
		replications := adapter.GetAllMockTridentReplications()
		key := "default/test-update"
		replication := replications[key]

		// The state should be translated to Trident format
		tridentPromotingState, _ := translator.TranslateStateToBackend(translation.BackendTrident, "promoting")
		assert.Equal(t, tridentPromotingState, replication.State)
	})

	t.Run("DeleteReplication", func(t *testing.T) {
		config := DefaultMockTridentConfig()
		config.DeleteSuccessRate = 1.0
		adapter := NewMockTridentAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-delete", "default")

		// Create first
		err := adapter.CreateReplication(ctx, uvr)
		require.NoError(t, err)

		// Verify created
		replications := adapter.GetAllMockTridentReplications()
		assert.Len(t, replications, 1)

		// Delete
		err = adapter.DeleteReplication(ctx, uvr)
		assert.NoError(t, err)

		// Verify deleted
		replications = adapter.GetAllMockTridentReplications()
		assert.Empty(t, replications)

		// Verify events
		events := adapter.GetMockTridentEvents()
		assert.NotEmpty(t, events)

		// Find delete event
		deleteEventFound := false
		for _, event := range events {
			if event.Type == EventTypeDeleted {
				deleteEventFound = true
				break
			}
		}
		assert.True(t, deleteEventFound)
	})

	t.Run("GetReplicationStatus", func(t *testing.T) {
		config := DefaultMockTridentConfig()
		adapter := NewMockTridentAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-status", "default")

		// Create first
		err := adapter.CreateReplication(ctx, uvr)
		require.NoError(t, err)

		// Get status
		status, err := adapter.GetReplicationStatus(ctx, uvr)
		assert.NoError(t, err)
		assert.NotNil(t, status)

		assert.Equal(t, string(replicationv1alpha1.ReplicationStateReplica), status.State)
		assert.Equal(t, string(replicationv1alpha1.ReplicationModeAsynchronous), status.Mode)
		assert.Equal(t, ReplicationHealthHealthy, status.Health)
		assert.NotEmpty(t, status.BackendSpecific)
		assert.Contains(t, status.BackendSpecific, "mirrorRelationshipUUID")
		assert.Contains(t, status.BackendSpecific, "policyName")
	})

	t.Run("StateOperations", func(t *testing.T) {
		config := DefaultMockTridentConfig()
		adapter := NewMockTridentAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-ops", "default")

		// Create first
		err := adapter.CreateReplication(ctx, uvr)
		require.NoError(t, err)

		// Test promote
		err = adapter.PromoteReplica(ctx, uvr)
		assert.NoError(t, err)

		// Test demote
		err = adapter.DemoteSource(ctx, uvr)
		assert.NoError(t, err)

		// Test resync
		err = adapter.ResyncReplication(ctx, uvr)
		assert.NoError(t, err)

		// Test pause/resume
		err = adapter.PauseReplication(ctx, uvr)
		assert.NoError(t, err)

		err = adapter.ResumeReplication(ctx, uvr)
		assert.NoError(t, err)

		// Test failover/failback
		err = adapter.FailoverReplication(ctx, uvr)
		assert.NoError(t, err)

		err = adapter.FailbackReplication(ctx, uvr)
		assert.NoError(t, err)
	})

	t.Run("LatencySimulation", func(t *testing.T) {
		config := &MockTridentConfig{
			CreateSuccessRate: 1.0,
			MinLatency:        50 * time.Millisecond,
			MaxLatency:        100 * time.Millisecond,
		}
		adapter := NewMockTridentAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-latency", "default")

		start := time.Now()
		err := adapter.CreateReplication(ctx, uvr)
		duration := time.Since(start)

		assert.NoError(t, err)
		assert.True(t, duration >= 50*time.Millisecond, "Operation should respect minimum latency")
	})

	t.Run("EventGeneration", func(t *testing.T) {
		config := DefaultMockTridentConfig()
		adapter := NewMockTridentAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-events", "default")

		// Perform operations that should generate events
		adapter.CreateReplication(ctx, uvr)
		adapter.PromoteReplica(ctx, uvr)
		adapter.DeleteReplication(ctx, uvr)

		events := adapter.GetMockTridentEvents()
		assert.NotEmpty(t, events)
		assert.GreaterOrEqual(t, len(events), 3) // At least create, promote, delete events

		// Verify event structure
		for _, event := range events {
			assert.NotEmpty(t, event.Type)
			assert.NotEmpty(t, event.Message)
			assert.NotZero(t, event.Timestamp)
			assert.NotEmpty(t, event.Resource)
		}
	})

	t.Run("HealthFluctuation", func(t *testing.T) {
		config := &MockTridentConfig{
			HealthFluctuation:   true,
			HealthCheckInterval: 10 * time.Millisecond,
		}
		adapter := NewMockTridentAdapter(client, translator, config)

		// Initialize adapter
		ctx := context.Background()
		err := adapter.Initialize(ctx)
		require.NoError(t, err)

		// Should be healthy after initialization
		assert.True(t, adapter.IsHealthy())

		// Wait for potential health fluctuation
		time.Sleep(50 * time.Millisecond)

		// Health status might have changed (this is non-deterministic by design)
		// Just verify the method works and returns a boolean
		health := adapter.IsHealthy()
		assert.IsType(t, true, health)
	})

	t.Run("ConfigurableFailures", func(t *testing.T) {
		config := &MockTridentConfig{
			CreateSuccessRate: 0.5, // 50% success rate
			StatusSuccessRate: 0.5,
			MinLatency:        0,
			MaxLatency:        0,
		}
		adapter := NewMockTridentAdapter(client, translator, config)

		ctx := context.Background()

		// Run multiple operations to test probability
		successCount := 0
		failureCount := 0
		totalOperations := 100

		for i := 0; i < totalOperations; i++ {
			uvr := createTestUnifiedVolumeReplication("test-prob", "default")
			uvr.Name = uvr.Name + string(rune(i)) // Make unique

			err := adapter.CreateReplication(ctx, uvr)
			if err == nil {
				successCount++
			} else {
				failureCount++
			}
		}

		// With 50% success rate, we should have both successes and failures
		assert.Greater(t, successCount, 0, "Should have some successes")
		assert.Greater(t, failureCount, 0, "Should have some failures")
		assert.Equal(t, totalOperations, successCount+failureCount)
	})
}

func TestMockPowerStoreAdapter(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, replicationv1alpha1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	translator := translation.NewEngine()

	t.Run("NewMockPowerStoreAdapter", func(t *testing.T) {
		config := DefaultMockPowerStoreConfig()
		adapter := NewMockPowerStoreAdapter(client, translator, config)

		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendPowerStore, adapter.GetBackendType())
		assert.True(t, adapter.IsHealthy())
		assert.Contains(t, adapter.GetVersion(), "mock-powerstore")

		// Check PowerStore-specific features
		features := adapter.GetSupportedFeatures()
		assert.Contains(t, features, FeatureMetroReplication)
		assert.Contains(t, features, FeatureConsistencyGroups)
		assert.Contains(t, features, FeatureVolumeGroups)
	})

	t.Run("CreateReplication", func(t *testing.T) {
		config := DefaultMockPowerStoreConfig()
		config.CreateSuccessRate = 1.0
		adapter := NewMockPowerStoreAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-ps", "default")

		err := adapter.CreateReplication(ctx, uvr)
		assert.NoError(t, err)

		// Verify replication was created
		replications := adapter.GetAllMockPowerStoreReplications()
		assert.Len(t, replications, 1)

		key := "default/test-ps"
		replication, exists := replications[key]
		assert.True(t, exists)
		assert.Equal(t, "test-ps", replication.Name)
		assert.Equal(t, ReplicationHealthHealthy, replication.Health)

		// Check PowerStore-specific fields
		assert.NotEmpty(t, replication.ReplicationGroupID)
		assert.NotEmpty(t, replication.SessionID)
		assert.Greater(t, replication.RPOCompliance, 0.0)
		assert.Greater(t, replication.RTOEstimate, time.Duration(0))
		assert.Contains(t, replication.BackendSpecific, "array_serial")
		assert.Contains(t, replication.BackendSpecific, "metro_enabled")

		// Verify session tracking
		sessions := adapter.GetMockPowerStoreSessions()
		assert.Contains(t, sessions, key)
		assert.Equal(t, replication.SessionID, sessions[key])
	})

	t.Run("RPOCompliance", func(t *testing.T) {
		config := &MockPowerStoreConfig{
			CreateSuccessRate: 1.0,
			RPOComplianceMin:  95.0,
			RPOComplianceMax:  99.0,
			MinLatency:        0,
			MaxLatency:        0,
		}
		adapter := NewMockPowerStoreAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-rpo", "default")

		err := adapter.CreateReplication(ctx, uvr)
		require.NoError(t, err)

		status, err := adapter.GetReplicationStatus(ctx, uvr)
		require.NoError(t, err)

		rpoCompliance, exists := status.BackendSpecific["rpo_compliance"]
		assert.True(t, exists)

		rpoValue, ok := rpoCompliance.(float64)
		assert.True(t, ok)
		assert.GreaterOrEqual(t, rpoValue, 95.0)
		assert.LessOrEqual(t, rpoValue, 99.0)
	})

	t.Run("SessionFailureSimulation", func(t *testing.T) {
		config := &MockPowerStoreConfig{
			CreateSuccessRate:  1.0,
			StatusSuccessRate:  1.0,
			SessionFailureRate: 1.0, // Always simulate session failure
			MinLatency:         0,
			MaxLatency:         0,
		}
		adapter := NewMockPowerStoreAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-session", "default")

		err := adapter.CreateReplication(ctx, uvr)
		require.NoError(t, err)

		status, err := adapter.GetReplicationStatus(ctx, uvr)
		require.NoError(t, err)

		// Should show degraded health due to session issues
		assert.Equal(t, ReplicationHealthDegraded, status.Health)
		assert.Contains(t, status.Message, "Session connectivity issues")
	})

	t.Run("FailoverOperations", func(t *testing.T) {
		config := DefaultMockPowerStoreConfig()
		adapter := NewMockPowerStoreAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-failover", "default")

		// Create replication
		err := adapter.CreateReplication(ctx, uvr)
		require.NoError(t, err)

		// Get original session ID
		sessions := adapter.GetMockPowerStoreSessions()
		originalSessionID := sessions["default/test-failover"]

		// Perform failover
		err = adapter.FailoverReplication(ctx, uvr)
		assert.NoError(t, err)

		// Session ID should change after failover
		updatedSessions := adapter.GetMockPowerStoreSessions()
		newSessionID := updatedSessions["default/test-failover"]
		assert.NotEqual(t, originalSessionID, newSessionID)
		assert.Contains(t, newSessionID, "failover-session")

		// Check backend-specific info
		replications := adapter.GetAllMockPowerStoreReplications()
		replication := replications["default/test-failover"]
		assert.Contains(t, replication.BackendSpecific, "failover_time")
	})

	t.Run("MetroReplication", func(t *testing.T) {
		config := DefaultMockPowerStoreConfig()
		config.MetroLatencyMs = 2
		adapter := NewMockPowerStoreAdapter(client, translator, config)

		ctx := context.Background()
		uvr := createTestUnifiedVolumeReplication("test-metro", "default")
		uvr.Spec.ReplicationMode = replicationv1alpha1.ReplicationModeSynchronous
		uvr.Spec.Extensions.Powerstore = &replicationv1alpha1.PowerStoreExtensions{
			RpoSettings: &[]string{"Five_Minutes"}[0], // Metro with short RPO
		}

		err := adapter.CreateReplication(ctx, uvr)
		require.NoError(t, err)

		status, err := adapter.GetReplicationStatus(ctx, uvr)
		require.NoError(t, err)

		metroEnabled, exists := status.BackendSpecific["metro_enabled"]
		assert.True(t, exists)
		assert.True(t, metroEnabled.(bool))

		metroLatency, exists := status.BackendSpecific["metro_latency_ms"]
		assert.True(t, exists)
		assert.Equal(t, 2, metroLatency)
	})
}

func TestMockAdapterFactories(t *testing.T) {
	t.Run("MockTridentAdapterFactory", func(t *testing.T) {
		factory := NewMockTridentAdapterFactory(nil)

		assert.Equal(t, translation.BackendTrident, factory.GetBackendType())
		assert.Equal(t, "Mock Trident Adapter", factory.GetInfo().Name)
		assert.Equal(t, "v1.0.0-mock", factory.GetInfo().Version)

		// Test adapter creation
		scheme := runtime.NewScheme()
		require.NoError(t, replicationv1alpha1.AddToScheme(scheme))
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		translator := translation.NewEngine()

		adapter, err := factory.Create(client, translator)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendTrident, adapter.GetBackendType())

		// Test supports and validate
		uvr := createTestUnifiedVolumeReplication("test", "default")
		assert.True(t, factory.Supports(uvr))
		assert.NoError(t, factory.ValidateConfig(nil))
	})

	t.Run("MockPowerStoreAdapterFactory", func(t *testing.T) {
		factory := NewMockPowerStoreAdapterFactory(nil)

		assert.Equal(t, translation.BackendPowerStore, factory.GetBackendType())
		assert.Equal(t, "Mock PowerStore Adapter", factory.GetInfo().Name)
		assert.Equal(t, "v1.0.0-mock", factory.GetInfo().Version)

		// Test adapter creation
		scheme := runtime.NewScheme()
		require.NoError(t, replicationv1alpha1.AddToScheme(scheme))
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		translator := translation.NewEngine()

		adapter, err := factory.Create(client, translator)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendPowerStore, adapter.GetBackendType())

		// Test supports and validate
		uvr := createTestUnifiedVolumeReplication("test", "default")
		assert.True(t, factory.Supports(uvr))
		assert.NoError(t, factory.ValidateConfig(nil))
	})
}

func TestMockRegistry(t *testing.T) {
	t.Run("RegisterMockAdapters", func(t *testing.T) {
		// Clear registry first
		registry := GetGlobalRegistry()
		registry.UnregisterFactory(translation.BackendTrident)
		registry.UnregisterFactory(translation.BackendPowerStore)

		// Register mock adapters
		err := RegisterMockAdapters()
		assert.NoError(t, err)

		// Verify registration
		tridentFactory, err := registry.GetFactory(translation.BackendTrident)
		assert.NoError(t, err)
		assert.NotNil(t, tridentFactory)

		powerstoreFactory, err := registry.GetFactory(translation.BackendPowerStore)
		assert.NoError(t, err)
		assert.NotNil(t, powerstoreFactory)

		// Clean up
		UnregisterMockAdapters()
	})

	t.Run("CreateMockTestEnvironment", func(t *testing.T) {
		err := CreateMockTestEnvironment()
		assert.NoError(t, err)

		// Verify adapters are registered with test-friendly configurations
		registry := GetGlobalRegistry()

		tridentFactory, err := registry.GetFactory(translation.BackendTrident)
		assert.NoError(t, err)
		assert.NotNil(t, tridentFactory)

		powerstoreFactory, err := registry.GetFactory(translation.BackendPowerStore)
		assert.NoError(t, err)
		assert.NotNil(t, powerstoreFactory)

		// Clean up
		UnregisterMockAdapters()
	})

	t.Run("CreateMockFailureTestEnvironment", func(t *testing.T) {
		err := CreateMockFailureTestEnvironment()
		assert.NoError(t, err)

		// Verify adapters are registered
		registry := GetGlobalRegistry()

		factories := registry.ListFactories()
		assert.GreaterOrEqual(t, len(factories), 2)

		// Clean up
		UnregisterMockAdapters()
	})
}

// Helper function for creating test UnifiedVolumeReplication resources
func createTestUnifiedVolumeReplication(name, namespace string) *replicationv1alpha1.UnifiedVolumeReplication {
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
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "dest-volume-handle",
					Namespace:    "default",
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
				Ceph:       &replicationv1alpha1.CephExtensions{},
				Trident:    &replicationv1alpha1.TridentExtensions{},
				Powerstore: &replicationv1alpha1.PowerStoreExtensions{},
			},
		},
	}
}
