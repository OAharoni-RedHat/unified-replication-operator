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

package utils

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

func TestNewTestClient(t *testing.T) {
	client := NewTestClient()

	assert.NotNil(t, client)
	assert.NotNil(t, client.Client)
	assert.NotNil(t, client.Scheme)
}

func TestCRDBuilder(t *testing.T) {
	builder := NewCRDBuilder()

	t.Run("default values", func(t *testing.T) {
		uvr := builder.Build()

		assert.Equal(t, "test-replication", uvr.Name)
		assert.Equal(t, "default", uvr.Namespace)
		assert.Equal(t, "source-cluster", uvr.Spec.SourceEndpoint.Cluster)
		assert.Equal(t, replicationv1alpha1.ReplicationStateSource, uvr.Spec.ReplicationState)
	})

	t.Run("with custom values", func(t *testing.T) {
		uvr := NewCRDBuilder().
			WithName("custom-replication").
			WithNamespace("custom-ns").
			WithSourceEndpoint("custom-source", "eu-west-1", "custom-storage").
			WithReplicationState(replicationv1alpha1.ReplicationStateReplica).
			Build()

		assert.Equal(t, "custom-replication", uvr.Name)
		assert.Equal(t, "custom-ns", uvr.Namespace)
		assert.Equal(t, "custom-source", uvr.Spec.SourceEndpoint.Cluster)
		assert.Equal(t, "eu-west-1", uvr.Spec.SourceEndpoint.Region)
		assert.Equal(t, "custom-storage", uvr.Spec.SourceEndpoint.StorageClass)
		assert.Equal(t, replicationv1alpha1.ReplicationStateReplica, uvr.Spec.ReplicationState)
	})

	t.Run("with extensions", func(t *testing.T) {
		startTime := metav1.Time{Time: time.Now()}

		uvr := NewCRDBuilder().
			WithCephExtensions("journal", &startTime).
			WithTridentExtensions().
			WithPowerStoreExtensions("Five_Minutes", []string{"group1"}).
			Build()

		require.NotNil(t, uvr.Spec.Extensions)
		require.NotNil(t, uvr.Spec.Extensions.Ceph)
		require.NotNil(t, uvr.Spec.Extensions.Trident)
		require.NotNil(t, uvr.Spec.Extensions.Powerstore)

		assert.Equal(t, "journal", *uvr.Spec.Extensions.Ceph.MirroringMode)
	})

	t.Run("with labels and annotations", func(t *testing.T) {
		labels := map[string]string{"app": "test", "env": "dev"}
		annotations := map[string]string{"version": "1.0", "description": "test crd"}

		uvr := NewCRDBuilder().
			WithLabels(labels).
			WithAnnotations(annotations).
			Build()

		assert.Equal(t, labels, uvr.Labels)
		assert.Equal(t, annotations, uvr.Annotations)
	})
}

func TestMockDataGenerator(t *testing.T) {
	generator := NewMockDataGenerator(12345) // Fixed seed for reproducible tests

	t.Run("random name generation", func(t *testing.T) {
		name1 := generator.RandomName("test")
		name2 := generator.RandomName("test")

		assert.Contains(t, name1, "test-")
		assert.Contains(t, name2, "test-")
		assert.NotEqual(t, name1, name2, "Names should be different")
	})

	t.Run("random values generation", func(t *testing.T) {
		cluster := generator.RandomClusterName()
		region := generator.RandomRegion()
		storageClass := generator.RandomStorageClass()
		state := generator.RandomReplicationState()
		mode := generator.RandomReplicationMode()
		scheduleMode := generator.RandomScheduleMode()
		timePattern := generator.RandomTimePattern()

		assert.NotEmpty(t, cluster)
		assert.NotEmpty(t, region)
		assert.NotEmpty(t, storageClass)
		assert.NotEmpty(t, state)
		assert.NotEmpty(t, mode)
		assert.NotEmpty(t, scheduleMode)
		assert.Regexp(t, `^\d+(s|m|h)$`, timePattern)
	})

	t.Run("random condition generation", func(t *testing.T) {
		condition := generator.RandomCondition()

		assert.NotEmpty(t, condition.Type)
		assert.NotEmpty(t, condition.Status)
		assert.NotEmpty(t, condition.Reason)
		assert.NotEmpty(t, condition.Message)
		assert.False(t, condition.LastTransitionTime.IsZero())
	})

	t.Run("random CRD generation", func(t *testing.T) {
		uvr := generator.GenerateRandomCRD()

		assert.NotNil(t, uvr)
		assert.NotEmpty(t, uvr.Name)
		assert.NotEmpty(t, uvr.Namespace)
		assert.NotEmpty(t, uvr.Spec.SourceEndpoint.Cluster)
		assert.NotEmpty(t, uvr.Spec.DestinationEndpoint.Cluster)
		assert.NotEqual(t, uvr.Spec.SourceEndpoint, uvr.Spec.DestinationEndpoint)

		// Validate the generated CRD
		err := uvr.ValidateSpec()
		assert.NoError(t, err, "Generated CRD should be valid")
	})
}

func TestCRDManipulator(t *testing.T) {
	client := NewTestClient()
	manipulator := NewCRDManipulator(client)
	ctx := context.Background()

	t.Run("create and retrieve CRD", func(t *testing.T) {
		uvr := NewCRDBuilder().
			WithName("test-manipulator").
			WithNamespace("default").
			Build()

		// Create CRD
		err := manipulator.CreateCRD(ctx, uvr)
		require.NoError(t, err)

		// Retrieve CRD
		retrieved, err := manipulator.GetCRD(ctx, "test-manipulator", "default")
		require.NoError(t, err)

		assert.Equal(t, uvr.Name, retrieved.Name)
		assert.Equal(t, uvr.Namespace, retrieved.Namespace)
		assert.Equal(t, uvr.Spec, retrieved.Spec)
	})

	t.Run("update CRD", func(t *testing.T) {
		uvr := NewCRDBuilder().
			WithName("test-update").
			WithNamespace("default").
			Build()

		// Create CRD
		err := manipulator.CreateCRD(ctx, uvr)
		require.NoError(t, err)

		// Update CRD
		uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStateReplica
		err = manipulator.UpdateCRD(ctx, uvr)
		require.NoError(t, err)

		// Verify update
		retrieved, err := manipulator.GetCRD(ctx, "test-update", "default")
		require.NoError(t, err)
		assert.Equal(t, replicationv1alpha1.ReplicationStateReplica, retrieved.Spec.ReplicationState)
	})

	t.Run("update CRD status", func(t *testing.T) {
		uvr := NewCRDBuilder().
			WithName("test-status-update").
			WithNamespace("default").
			Build()

		// Create CRD
		err := manipulator.CreateCRD(ctx, uvr)
		require.NoError(t, err)

		// Retrieve the created CRD to get the latest version
		created, err := manipulator.GetCRD(ctx, "test-status-update", "default")
		require.NoError(t, err, "Failed to retrieve created CRD before status update")

		// Update status on the retrieved resource
		created.Status = replicationv1alpha1.UnifiedVolumeReplicationStatus{
			ObservedGeneration: 1,
			Conditions: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					Reason:             "TestReason",
					Message:            "Test message",
					LastTransitionTime: metav1.NewTime(time.Now()),
				},
			},
		}
		err = manipulator.UpdateCRDStatus(ctx, created)
		require.NoError(t, err)

		// Verify status update
		retrieved, err := manipulator.GetCRD(ctx, "test-status-update", "default")
		require.NoError(t, err)
		assert.Equal(t, int64(1), retrieved.Status.ObservedGeneration)
		assert.Len(t, retrieved.Status.Conditions, 1)
		assert.Equal(t, "Ready", retrieved.Status.Conditions[0].Type)
	})

	t.Run("list CRDs", func(t *testing.T) {
		// Create multiple CRDs
		for i := 0; i < 3; i++ {
			uvr := NewCRDBuilder().
				WithName(fmt.Sprintf("test-list-%d", i)).
				WithNamespace("test-list-ns").
				Build()

			err := manipulator.CreateCRD(ctx, uvr)
			require.NoError(t, err)
		}

		// List CRDs
		list, err := manipulator.ListCRDs(ctx, "test-list-ns")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list.Items), 3)

		// Verify all CRDs are in the correct namespace
		for _, item := range list.Items {
			assert.Equal(t, "test-list-ns", item.Namespace)
		}
	})

	t.Run("delete CRD", func(t *testing.T) {
		uvr := NewCRDBuilder().
			WithName("test-delete").
			WithNamespace("default").
			Build()

		// Create CRD
		err := manipulator.CreateCRD(ctx, uvr)
		require.NoError(t, err)

		// Verify it exists
		_, err = manipulator.GetCRD(ctx, "test-delete", "default")
		require.NoError(t, err)

		// Delete CRD
		err = manipulator.DeleteCRD(ctx, uvr)
		require.NoError(t, err)

		// Verify it's deleted
		_, err = manipulator.GetCRD(ctx, "test-delete", "default")
		assert.Error(t, err, "CRD should be deleted")
	})
}

func TestPerformanceTracker(t *testing.T) {
	t.Run("track single operation", func(t *testing.T) {
		tracker := NewPerformanceTracker() // Fresh instance per subtest

		tracker.StartOperation("test-op")
		time.Sleep(10 * time.Millisecond) // Simulate work
		duration := tracker.EndOperation("test-op")

		assert.GreaterOrEqual(t, duration, 10*time.Millisecond)
		assert.Equal(t, duration, tracker.GetOperationDuration("test-op"))
	})

	t.Run("track multiple operations", func(t *testing.T) {
		tracker := NewPerformanceTracker() // Fresh instance per subtest

		operations := []string{"op1", "op2", "op3"}

		for _, op := range operations {
			tracker.StartOperation(op)
			time.Sleep(5 * time.Millisecond)
			tracker.EndOperation(op)
		}

		allOps := tracker.GetAllOperations()
		assert.Len(t, allOps, len(operations), "Should have exactly 3 operations")

		for _, op := range operations {
			duration, exists := allOps[op]
			assert.True(t, exists)
			assert.GreaterOrEqual(t, duration, 5*time.Millisecond)
		}
	})

	t.Run("reset tracker", func(t *testing.T) {
		tracker := NewPerformanceTracker() // Fresh instance per subtest

		tracker.StartOperation("reset-test")
		tracker.EndOperation("reset-test")

		assert.Len(t, tracker.GetAllOperations(), 1, "Should have 1 operation before reset")

		tracker.Reset()
		assert.Len(t, tracker.GetAllOperations(), 0, "Should have 0 operations after reset")
	})

	t.Run("non-existent operation", func(t *testing.T) {
		tracker := NewPerformanceTracker() // Fresh instance per subtest

		duration := tracker.EndOperation("non-existent")
		assert.Equal(t, time.Duration(0), duration)
	})
}
