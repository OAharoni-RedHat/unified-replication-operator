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

package integration

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestMain(m *testing.M) {
	ctx, cancel = context.WithCancel(context.TODO())
	defer cancel()

	// Setup test environment
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		panic(err)
	}

	scheme := runtime.NewScheme()
	err = clientgoscheme.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}
	err = replicationv1alpha1.AddToScheme(scheme)
	if err != nil {
		panic(err)
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic(err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	err = testEnv.Stop()
	if err != nil {
		panic(err)
	}

	// Exit with test result code
	if code != 0 {
		panic("Tests failed")
	}
}

func TestUnifiedVolumeReplication_CRDInstallation(t *testing.T) {
	// This test verifies that the CRD can be installed successfully
	// The CRD installation is handled by envtest in TestMain
	t.Log("CRD installation test completed successfully via envtest")
}

func TestUnifiedVolumeReplication_CreateValidResource(t *testing.T) {
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-replication",
			Namespace: "default",
		},
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "source-cluster",
				Region:       "us-east-1",
				StorageClass: "ceph-rbd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				Region:       "us-west-2",
				StorageClass: "trident-nas",
			},
			VolumeMapping: replicationv1alpha1.VolumeMapping{
				Source: replicationv1alpha1.VolumeSource{
					PvcName:   "test-pvc",
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "vol-12345",
					Namespace:    "default",
				},
			},
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
				Rpo:  "15m",
				Rto:  "5m",
			},
		},
	}

	// Create the resource
	err := k8sClient.Create(ctx, uvr)
	require.NoError(t, err, "Failed to create UnifiedVolumeReplication resource")

	// Verify the resource was created
	createdUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(uvr), createdUVR)
	require.NoError(t, err, "Failed to get created UnifiedVolumeReplication resource")

	// Verify the spec was stored correctly
	assert.Equal(t, uvr.Spec.SourceEndpoint.Cluster, createdUVR.Spec.SourceEndpoint.Cluster)
	assert.Equal(t, uvr.Spec.ReplicationState, createdUVR.Spec.ReplicationState)
	assert.Equal(t, uvr.Spec.ReplicationMode, createdUVR.Spec.ReplicationMode)

	// Cleanup
	err = k8sClient.Delete(ctx, uvr)
	require.NoError(t, err, "Failed to delete UnifiedVolumeReplication resource")
}

func TestUnifiedVolumeReplication_CreateWithExtensions(t *testing.T) {
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-replication-with-extensions",
			Namespace: "default",
		},
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "source-cluster",
				Region:       "us-east-1",
				StorageClass: "ceph-rbd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				Region:       "us-west-2",
				StorageClass: "trident-nas",
			},
			VolumeMapping: replicationv1alpha1.VolumeMapping{
				Source: replicationv1alpha1.VolumeSource{
					PvcName:   "test-pvc",
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "vol-12345",
					Namespace:    "default",
				},
			},
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
			},
			Extensions: &replicationv1alpha1.Extensions{
				Ceph: &replicationv1alpha1.CephExtensions{
					MirroringMode: stringPtr("journal"),
				},
				Trident: &replicationv1alpha1.TridentExtensions{},
			},
		},
	}

	// Create the resource
	err := k8sClient.Create(ctx, uvr)
	require.NoError(t, err, "Failed to create UnifiedVolumeReplication resource with extensions")

	// Verify the resource was created with extensions
	createdUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(uvr), createdUVR)
	require.NoError(t, err, "Failed to get created UnifiedVolumeReplication resource")

	// Verify extensions were stored correctly
	require.NotNil(t, createdUVR.Spec.Extensions)
	require.NotNil(t, createdUVR.Spec.Extensions.Ceph)
	assert.Equal(t, "journal", *createdUVR.Spec.Extensions.Ceph.MirroringMode)

	// Verify Trident extension exists (currently empty struct, reserved for future use)
	require.NotNil(t, createdUVR.Spec.Extensions.Trident)

	// Cleanup
	err = k8sClient.Delete(ctx, uvr)
	require.NoError(t, err, "Failed to delete UnifiedVolumeReplication resource")
}

func TestUnifiedVolumeReplication_StatusUpdate(t *testing.T) {
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-replication-status",
			Namespace: "default",
		},
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "source-cluster",
				Region:       "us-east-1",
				StorageClass: "ceph-rbd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				Region:       "us-west-2",
				StorageClass: "trident-nas",
			},
			VolumeMapping: replicationv1alpha1.VolumeMapping{
				Source: replicationv1alpha1.VolumeSource{
					PvcName:   "test-pvc",
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "vol-12345",
					Namespace:    "default",
				},
			},
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
			},
		},
	}

	// Create the resource
	err := k8sClient.Create(ctx, uvr)
	require.NoError(t, err, "Failed to create UnifiedVolumeReplication resource")

	// Update status
	uvr.Status = replicationv1alpha1.UnifiedVolumeReplicationStatus{
		Conditions: []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "ReplicationActive",
				Message:            "Replication is active",
				LastTransitionTime: metav1.NewTime(time.Now()),
			},
		},
		ObservedGeneration: 1,
		DiscoveredBackends: []replicationv1alpha1.BackendInfo{
			{
				Name:         "ceph-backend",
				Type:         "ceph-csi",
				Available:    true,
				Capabilities: []string{"async", "bidirectional"},
			},
		},
	}

	err = k8sClient.Status().Update(ctx, uvr)
	require.NoError(t, err, "Failed to update UnifiedVolumeReplication status")

	// Verify status was updated
	updatedUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(uvr), updatedUVR)
	require.NoError(t, err, "Failed to get updated UnifiedVolumeReplication resource")

	assert.Len(t, updatedUVR.Status.Conditions, 1)
	assert.Equal(t, "Ready", updatedUVR.Status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, updatedUVR.Status.Conditions[0].Status)
	assert.Equal(t, int64(1), updatedUVR.Status.ObservedGeneration)
	assert.Len(t, updatedUVR.Status.DiscoveredBackends, 1)

	// Cleanup
	err = k8sClient.Delete(ctx, uvr)
	require.NoError(t, err, "Failed to delete UnifiedVolumeReplication resource")
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
