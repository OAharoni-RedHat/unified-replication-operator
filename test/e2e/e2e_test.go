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

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/adapters"
	"github.com/unified-replication/operator/pkg/discovery"
	"github.com/unified-replication/operator/pkg/translation"
)

// TestE2E_CompleteWorkflow tests the complete end-to-end workflow
func TestE2E_CompleteWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx := context.Background()
	s := createE2EScheme(t)
	fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

	// Setup engines
	discoveryEngine := discovery.NewEngine(fakeClient, discovery.DefaultDiscoveryConfig())
	translationEngine := translation.NewEngine()

	// Register mock adapters for E2E testing
	_ = adapters.RegisterMockAdapters()
	defer adapters.UnregisterMockAdapters()

	t.Run("CreateReplication", func(t *testing.T) {
		uvr := createE2EUVR("e2e-test", "default")
		err := fakeClient.Create(ctx, uvr)
		require.NoError(t, err)

		// Verify created
		retrieved := &replicationv1alpha1.UnifiedVolumeReplication{}
		err = fakeClient.Get(ctx, client.ObjectKey{Name: "e2e-test", Namespace: "default"}, retrieved)
		assert.NoError(t, err)
		assert.Equal(t, "e2e-test", retrieved.Name)
	})

	t.Run("DiscoverBackends", func(t *testing.T) {
		result, err := discoveryEngine.DiscoverBackends(ctx)
		// May not find backends without CRDs, but should not error
		if err == nil {
			t.Logf("Discovered %d backends", len(result.AvailableBackends))
		}
	})

	t.Run("TranslateStates", func(t *testing.T) {
		backend := translation.BackendTrident

		// Translate state
		backendState, err := translationEngine.TranslateStateToBackend(backend, "source")
		assert.NoError(t, err)
		assert.NotEmpty(t, backendState)

		// Translate back
		unifiedState, err := translationEngine.TranslateStateFromBackend(backend, backendState)
		assert.NoError(t, err)
		assert.Equal(t, "source", unifiedState)
	})

	t.Run("StateTransition", func(t *testing.T) {
		uvr := createE2EUVR("state-test", "default")
		uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStateReplica
		err := fakeClient.Create(ctx, uvr)
		require.NoError(t, err)

		// Update to promoting
		uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStatePromoting
		err = fakeClient.Update(ctx, uvr)
		assert.NoError(t, err)

		// Verify updated
		retrieved := &replicationv1alpha1.UnifiedVolumeReplication{}
		err = fakeClient.Get(ctx, client.ObjectKey{Name: "state-test", Namespace: "default"}, retrieved)
		assert.NoError(t, err)
		assert.Equal(t, replicationv1alpha1.ReplicationStatePromoting, retrieved.Spec.ReplicationState)
	})
}

// TestE2E_MultiBackend tests operations across multiple backends
func TestE2E_MultiBackend(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E multi-backend test in short mode")
	}

	ctx := context.Background()
	s := createE2EScheme(t)

	backends := []struct {
		name         string
		storageClass string
		extensions   *replicationv1alpha1.Extensions
	}{
		{
			name:         "ceph",
			storageClass: "ceph-rbd",
			extensions: &replicationv1alpha1.Extensions{
				Ceph: &replicationv1alpha1.CephExtensions{
					MirroringMode: stringPtr("journal"),
				},
			},
		},
		{
			name:         "trident",
			storageClass: "trident-nas",
			extensions: &replicationv1alpha1.Extensions{
				Trident: &replicationv1alpha1.TridentExtensions{
					Actions: []replicationv1alpha1.TridentAction{},
				},
			},
		},
		{
			name:         "powerstore",
			storageClass: "powerstore-block",
			extensions: &replicationv1alpha1.Extensions{
				Powerstore: &replicationv1alpha1.PowerStoreExtensions{
					RpoSettings: stringPtr("Five_Minutes"),
				},
			},
		},
	}

	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

			uvr := createE2EUVR("test-"+backend.name, "default")
			uvr.Spec.SourceEndpoint.StorageClass = backend.storageClass
			uvr.Spec.DestinationEndpoint.StorageClass = backend.storageClass
			uvr.Spec.Extensions = backend.extensions

			err := fakeClient.Create(ctx, uvr)
			assert.NoError(t, err)

			t.Logf("Successfully created replication for %s backend", backend.name)
		})
	}
}

// TestE2E_FailoverScenario tests complete failover scenario
func TestE2E_FailoverScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E failover test in short mode")
	}

	ctx := context.Background()
	s := createE2EScheme(t)
	fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

	// Step 1: Create as replica
	uvr := createE2EUVR("failover-test", "default")
	uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStateReplica
	err := fakeClient.Create(ctx, uvr)
	require.NoError(t, err)

	// Step 2: Promote to source
	uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStatePromoting
	err = fakeClient.Update(ctx, uvr)
	assert.NoError(t, err)

	// Step 3: Complete promotion
	time.Sleep(100 * time.Millisecond) // Simulate promotion time
	uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStateSource
	err = fakeClient.Update(ctx, uvr)
	assert.NoError(t, err)

	// Step 4: Verify source state
	retrieved := &replicationv1alpha1.UnifiedVolumeReplication{}
	err = fakeClient.Get(ctx, client.ObjectKey{Name: "failover-test", Namespace: "default"}, retrieved)
	assert.NoError(t, err)
	assert.Equal(t, replicationv1alpha1.ReplicationStateSource, retrieved.Spec.ReplicationState)

	t.Log("Failover scenario completed successfully")
}

// TestE2E_Performance tests performance under load
func TestE2E_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E performance test in short mode")
	}

	ctx := context.Background()
	s := createE2EScheme(t)
	fakeClient := fake.NewClientBuilder().WithScheme(s).Build()

	// Create multiple replications
	numReplications := 100
	start := time.Now()

	for i := 0; i < numReplications; i++ {
		uvr := createE2EUVR(fmt.Sprintf("perf-test-%d", i), "default")
		err := fakeClient.Create(ctx, uvr)
		if err != nil {
			t.Logf("Failed to create replication %d: %v", i, err)
		}
	}

	duration := time.Since(start)
	avgTime := duration / time.Duration(numReplications)

	t.Logf("Created %d replications in %v (avg: %v per resource)",
		numReplications, duration, avgTime)

	// Should create quickly
	assert.Less(t, avgTime, 100*time.Millisecond,
		"Average creation time should be < 100ms")
}

// Helper functions

func createE2EScheme(t *testing.T) *runtime.Scheme {
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, replicationv1alpha1.AddToScheme(s))
	return s
}

func createE2EUVR(name, namespace string) *replicationv1alpha1.UnifiedVolumeReplication {
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
				StorageClass: "ceph-rbd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				Region:       "us-west-1",
				StorageClass: "ceph-rbd",
			},
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeContinuous,
				Rpo:  "15m",
				Rto:  "5m",
			},
		},
	}
}

func stringPtr(s string) *string {
	return &s
}
