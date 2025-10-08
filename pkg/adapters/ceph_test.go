// Copyright 2024 unified-replication-operator contributors.
// Licensed under the Apache License, Version 2.0.

package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

// createUnifiedVolumeReplication creates a test UnifiedVolumeReplication
func createUnifiedVolumeReplication() *replicationv1alpha1.UnifiedVolumeReplication {
	return &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-uvr",
			Namespace: "default",
		},
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			ReplicationState: "source",
			ReplicationMode:  "synchronous",
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "source-cluster",
				StorageClass: "ceph-rbd",
				Region:       "us-east-1",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				StorageClass: "ceph-rbd",
				Region:       "us-west-1",
			},
			VolumeMapping: replicationv1alpha1.VolumeMapping{
				Source: replicationv1alpha1.VolumeSource{
					PvcName:   "test-pvc",
					Namespace: "default",
				},
			},
			Schedule: replicationv1alpha1.Schedule{
				Rpo:  "5m",
				Rto:  "15m",
				Mode: "continuous",
			},
			Extensions: &replicationv1alpha1.Extensions{
				Ceph: &replicationv1alpha1.CephExtensions{
					MirroringMode: stringPtr("journal"),
				},
			},
		},
	}
}

func TestCephAdapter(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = replicationv1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	t.Run("NewCephAdapter", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		translator := translation.NewEngine()

		adapter, err := NewCephAdapter(client, translator)
		require.NoError(t, err, "Failed to create CephAdapter")

		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendCeph, adapter.GetBackendType())
	})

	t.Run("ValidateConfiguration", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		translator := translation.NewEngine()
		adapter, err := NewCephAdapter(client, translator)
		require.NoError(t, err)

		// Test valid configuration
		uvr := createUnifiedVolumeReplication()
		err = adapter.ValidateConfiguration(uvr)
		assert.NoError(t, err)

		// Test invalid storage class
		uvr.Spec.SourceEndpoint.StorageClass = "invalid-class"
		err = adapter.ValidateConfiguration(uvr)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not compatible with Ceph RBD")
	})

	t.Run("SupportsConfiguration", func(t *testing.T) {
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		translator := translation.NewEngine()
		adapter, err := NewCephAdapter(client, translator)
		require.NoError(t, err)

		// Test supported configuration
		uvr := createUnifiedVolumeReplication()
		supported, err := adapter.SupportsConfiguration(uvr)
		assert.NoError(t, err)
		assert.True(t, supported)

		// Test unsupported configuration
		uvr.Spec.SourceEndpoint.StorageClass = "nfs"
		supported, err = adapter.SupportsConfiguration(uvr)
		assert.NoError(t, err)
		assert.False(t, supported)
	})
}

func TestCephAdapterFactory(t *testing.T) {
	t.Run("NewCephAdapterFactory", func(t *testing.T) {
		factory := NewCephAdapterFactory()
		assert.NotNil(t, factory)
		assert.Equal(t, translation.BackendCeph, factory.GetBackendType())
		assert.Equal(t, "Ceph Adapter", factory.GetInfo().Name)
	})

	t.Run("CreateAdapter", func(t *testing.T) {
		factory := NewCephAdapterFactory()
		scheme := runtime.NewScheme()
		_ = replicationv1alpha1.AddToScheme(scheme)
		client := fake.NewClientBuilder().WithScheme(scheme).Build()
		translator := translation.NewEngine()

		adapter, err := factory.CreateAdapter(translation.BackendCeph, client, translator, nil)
		assert.NoError(t, err)
		assert.NotNil(t, adapter)
		assert.Equal(t, translation.BackendCeph, adapter.GetBackendType())
	})

	t.Run("Supports", func(t *testing.T) {
		factory := NewCephAdapterFactory()
		uvr := createUnifiedVolumeReplication()

		// Test supported configuration
		supported := factory.Supports(uvr)
		assert.True(t, supported)

		// Test unsupported configuration
		uvr.Spec.SourceEndpoint.StorageClass = "nfs"
		supported = factory.Supports(uvr)
		assert.False(t, supported)
	})
}
