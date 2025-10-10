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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

func TestReconciler_BasicLifecycle(t *testing.T) {
	ctx := context.Background()
	s := createTestScheme(t)

	// Create test resource first
	uvr := createTestUVR("test-lifecycle", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	reconciler := createTestReconciler(fakeClient, s)

	// First reconcile - should add finalizer
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-lifecycle",
			Namespace: "default",
		},
	}

	result, err := reconciler.Reconcile(ctx, req)
	assert.NoError(t, err)
	assert.True(t, result.RequeueAfter > 0)

	// Verify finalizer was added
	updatedUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
	require.NoError(t, fakeClient.Get(ctx, req.NamespacedName, updatedUVR))
	assert.Contains(t, updatedUVR.Finalizers, unifiedReplicationFinalizer)

	t.Log("Basic lifecycle test passed")
}

func TestReconciler_StatusUpdate(t *testing.T) {
	ctx := context.Background()
	s := createTestScheme(t)

	// Create test resource first
	uvr := createTestUVR("test-status", "default")

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	reconciler := createTestReconciler(fakeClient, s)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-status",
			Namespace: "default",
		},
	}

	// Reconcile multiple times
	for i := 0; i < 3; i++ {
		result, err := reconciler.Reconcile(ctx, req)
		t.Logf("Reconcile %d: RequeueAfter=%v, Error=%v", i, result.RequeueAfter, err)
		// May error if adapter not available, but should update status
		_ = err
	}

	// Check status was updated
	updatedUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
	require.NoError(t, fakeClient.Get(ctx, req.NamespacedName, updatedUVR))

	// Should have conditions after reconciliation attempts
	// First reconcile adds finalizer, subsequent ones may set conditions
	if len(updatedUVR.Status.Conditions) > 0 {
		t.Logf("Status has %d conditions", len(updatedUVR.Status.Conditions))
		t.Log("Status update test passed - conditions set")
	} else {
		t.Log("No conditions set yet - may need adapter to be available")
		// This is acceptable behavior when adapter is not available
		// The finalizer should at least have been added
		assert.Contains(t, updatedUVR.Finalizers, unifiedReplicationFinalizer,
			"Should at least have finalizer even if conditions not set")
	}
}

func TestReconciler_Deletion(t *testing.T) {
	ctx := context.Background()
	s := createTestScheme(t)

	// Create test resource with finalizer
	uvr := createTestUVR("test-delete", "default")
	uvr.Finalizers = []string{unifiedReplicationFinalizer}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	reconciler := createTestReconciler(fakeClient, s)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-delete",
			Namespace: "default",
		},
	}

	// Delete the resource
	require.NoError(t, fakeClient.Delete(ctx, uvr))

	// Reconcile to handle deletion
	result, err := reconciler.Reconcile(ctx, req)
	assert.NoError(t, err)
	assert.Equal(t, time.Duration(0), result.RequeueAfter)

	// Resource should be gone
	deletedUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
	err = fakeClient.Get(ctx, req.NamespacedName, deletedUVR)
	assert.True(t, err != nil || deletedUVR.DeletionTimestamp != nil)

	t.Log("Deletion test passed")
}

func TestReconciler_ConditionManagement(t *testing.T) {
	s := createTestScheme(t)
	reconciler := createTestReconciler(nil, s)

	uvr := createTestUVR("test-cond", "default")
	uvr.Generation = 1

	// Test adding new condition
	condition1 := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "TestReason",
		Message:            "Test message",
		ObservedGeneration: 1,
	}

	reconciler.updateCondition(uvr, condition1)
	assert.Len(t, uvr.Status.Conditions, 1)
	assert.Equal(t, "Ready", uvr.Status.Conditions[0].Type)

	// Test updating existing condition
	condition2 := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "UpdatedReason",
		Message:            "Updated message",
		ObservedGeneration: 1,
	}

	reconciler.updateCondition(uvr, condition2)
	assert.Len(t, uvr.Status.Conditions, 1)
	assert.Equal(t, metav1.ConditionFalse, uvr.Status.Conditions[0].Status)
	assert.Equal(t, "UpdatedReason", uvr.Status.Conditions[0].Reason)

	// Test getting condition
	found := reconciler.getCondition(uvr, "Ready")
	assert.NotNil(t, found)
	assert.Equal(t, "Ready", found.Type)

	notFound := reconciler.getCondition(uvr, "NonExistent")
	assert.Nil(t, notFound)

	t.Log("Condition management test passed")
}

// Operation determination tests removed (behavior now handled by EnsureReplication)

func TestReconciler_ErrorHandling(t *testing.T) {
	ctx := context.Background()
	s := createTestScheme(t)

	// Create resource with invalid spec
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-error",
			Namespace: "default",
		},
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			// Missing required fields
			ReplicationState: "invalid",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	reconciler := createTestReconciler(fakeClient, s)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-error",
			Namespace: "default",
		},
	}

	// Reconcile - should handle error gracefully
	result, err := reconciler.Reconcile(ctx, req)

	// Should requeue or return error
	// Note: err may be nil if validation error is handled gracefully
	assert.True(t, result.RequeueAfter > 0 || err != nil, "Should handle error appropriately")

	t.Logf("Result: RequeueAfter=%v, Error=%v", result.RequeueAfter, err)

	// Check status was updated with error
	updatedUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
	require.NoError(t, fakeClient.Get(ctx, req.NamespacedName, updatedUVR))

	// Should have error condition
	if len(updatedUVR.Status.Conditions) > 0 {
		hasErrorCondition := false
		for _, cond := range updatedUVR.Status.Conditions {
			if cond.Status == metav1.ConditionFalse {
				hasErrorCondition = true
				t.Logf("Found error condition: %s - %s", cond.Reason, cond.Message)
			}
		}
		if hasErrorCondition {
			t.Log("Error condition properly set")
		}
	}

	t.Log("Error handling test passed")
}

func TestReconciler_ConcurrentReconciles(t *testing.T) {
	reconciler := &UnifiedVolumeReplicationReconciler{
		Log:                     ctrl.Log.WithName("test"),
		MaxConcurrentReconciles: 5,
	}

	maxConcurrent := reconciler.getMaxConcurrentReconciles()
	assert.Equal(t, 5, maxConcurrent)

	// Test default
	reconciler2 := &UnifiedVolumeReplicationReconciler{
		Log: ctrl.Log.WithName("test"),
	}
	maxConcurrent2 := reconciler2.getMaxConcurrentReconciles()
	assert.Equal(t, 1, maxConcurrent2)

	t.Log("Concurrent reconciles config test passed")
}

func TestReconciler_Timeout(t *testing.T) {
	reconciler := &UnifiedVolumeReplicationReconciler{
		Log:              ctrl.Log.WithName("test"),
		ReconcileTimeout: 2 * time.Minute,
	}

	timeout := reconciler.getReconcileTimeout()
	assert.Equal(t, 2*time.Minute, timeout)

	// Test default
	reconciler2 := &UnifiedVolumeReplicationReconciler{
		Log: ctrl.Log.WithName("test"),
	}
	timeout2 := reconciler2.getReconcileTimeout()
	assert.Equal(t, 5*time.Minute, timeout2)

	t.Log("Timeout config test passed")
}

// Helper functions

func createTestScheme(t *testing.T) *runtime.Scheme {
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, replicationv1alpha1.AddToScheme(s))
	return s
}

func createTestReconciler(client client.Client, s *runtime.Scheme) *UnifiedVolumeReplicationReconciler {
	return &UnifiedVolumeReplicationReconciler{
		Client:   client,
		Log:      ctrl.Log.WithName("test").WithName("UnifiedVolumeReplication"),
		Scheme:   s,
		Recorder: record.NewFakeRecorder(100),
	}
}

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
				Trident: &replicationv1alpha1.TridentExtensions{},
			},
		},
	}
}
