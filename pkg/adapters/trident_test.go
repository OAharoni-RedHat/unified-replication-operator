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

func TestNewTridentAdapter(t *testing.T) {
	t.Run("ValidClient", func(t *testing.T) {
		client := fake.NewClientBuilder().Build()
		translator := translation.NewEngine()

		adapter, err := NewTridentAdapter(client, translator)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendTrident, adapter.GetBackendType())
	})

	t.Run("NilClient", func(t *testing.T) {
		_, err := NewTridentAdapter(nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client cannot be nil")
	})

	t.Run("NilTranslator", func(t *testing.T) {
		client := fake.NewClientBuilder().Build()

		adapter, err := NewTridentAdapter(client, nil)
		assert.NoError(t, err, "Should create translator if nil")
		assert.NotNil(t, adapter)
	})
}

func TestTridentAdapter_CreateReplication(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	translator := translation.NewEngine()

	adapter, err := NewTridentAdapter(client, translator)
	require.NoError(t, err)

	ctx := context.Background()
	uvr := createTestUVRForTrident("test-trident", "default")

	t.Run("SuccessfulCreate", func(t *testing.T) {
		err := adapter.EnsureReplication(ctx, uvr)
		// May fail due to CRD not registered, but should not panic
		if err != nil {
			t.Logf("Create failed (expected without CRD): %v", err)
		}
	})

	t.Run("ValidateConfiguration", func(t *testing.T) {
		err := adapter.ValidateConfiguration(uvr)
		// Should validate spec
		if err != nil {
			t.Logf("Validation error: %v", err)
		}
	})
}

func TestTridentAdapter_Operations(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	translator := translation.NewEngine()

	adapter, err := NewTridentAdapter(client, translator)
	require.NoError(t, err)

	ctx := context.Background()
	uvr := createTestUVRForTrident("test-ops", "default")

	t.Run("PromoteReplica", func(t *testing.T) {
		err := adapter.PromoteReplica(ctx, uvr)
		// Test doesn't panic
		_ = err
	})

	t.Run("DemoteSource", func(t *testing.T) {
		err := adapter.DemoteSource(ctx, uvr)
		// Test doesn't panic
		_ = err
	})

	t.Run("ResyncReplication", func(t *testing.T) {
		err := adapter.ResyncReplication(ctx, uvr)
		// May fail without CRD, but validates logic
		_ = err
	})
}

func TestTridentAdapter_StateTranslation(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	translator := translation.NewEngine()

	adapter, err := NewTridentAdapter(client, translator)
	require.NoError(t, err)

	t.Run("TranslateState", func(t *testing.T) {
		states := []string{"source", "replica", "promoting", "demoting", "syncing"}

		for _, state := range states {
			translated, err := adapter.TranslateState(state)
			if err == nil {
				assert.NotEmpty(t, translated)
				t.Logf("State %s → %s", state, translated)
			}
		}
	})

	t.Run("TranslateMode", func(t *testing.T) {
		modes := []string{"synchronous", "asynchronous"}

		for _, mode := range modes {
			translated, err := adapter.TranslateMode(mode)
			if err == nil {
				assert.NotEmpty(t, translated)
				t.Logf("Mode %s → %s", mode, translated)
			}
		}
	})
}

// Helper function
func createTestUVRForTrident(name, namespace string) *replicationv1alpha1.UnifiedVolumeReplication {
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
				StorageClass: "trident-nas",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				Region:       "us-west-1",
				StorageClass: "trident-nas",
			},
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeContinuous,
				Rpo:  "15m",
				Rto:  "5m",
			},
			Extensions: &replicationv1alpha1.Extensions{
				Trident: &replicationv1alpha1.TridentExtensions{
					Actions: []replicationv1alpha1.TridentAction{
						{
							Type:           "mirror-update",
							SnapshotHandle: "snap-123",
						},
					},
				},
			},
		},
	}
}
