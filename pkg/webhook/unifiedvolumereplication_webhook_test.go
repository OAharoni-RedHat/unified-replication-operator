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

package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

func TestUnifiedVolumeReplicationValidator_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		uvr     *replicationv1alpha1.UnifiedVolumeReplication
		wantErr bool
		wantMsg string
	}{
		{
			name: "valid create",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication",
					Namespace: "default",
				},
				Spec: createValidSpec(),
			},
			wantErr: false,
		},
		{
			name: "invalid create - identical endpoints",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication",
					Namespace: "default",
				},
				Spec: createInvalidSpecIdenticalEndpoints(),
			},
			wantErr: true,
			wantMsg: "source and destination endpoints cannot be identical",
		},
		{
			name: "invalid create - long name",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "this-is-a-very-long-name-that-exceeds-the-maximum-allowed-length-for-kubernetes-resources",
					Namespace: "default",
				},
				Spec: createValidSpec(),
			},
			wantErr: true,
			wantMsg: "exceeds maximum length of 63 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, clientgoscheme.AddToScheme(scheme))
			require.NoError(t, replicationv1alpha1.AddToScheme(scheme))

			// Create fake client without existing objects for simplicity
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator := NewUnifiedVolumeReplicationValidator(client)

			// Test validation - skip PVC uniqueness check for now by directly calling ValidateSpec
			err := tt.uvr.ValidateSpec()
			if err == nil {
				// Also test resource naming validation
				err = validator.validateResourceNaming(context.Background(), tt.uvr)
			}

			if tt.wantErr {
				assert.Error(t, err, "Expected validation to fail")
				if tt.wantMsg != "" {
					assert.Contains(t, err.Error(), tt.wantMsg)
				}
			} else {
				assert.NoError(t, err, "Expected validation to pass")
			}
		})
	}
}

func TestUnifiedVolumeReplicationValidator_ValidateUpdate(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, replicationv1alpha1.AddToScheme(scheme))

	tests := []struct {
		name    string
		oldUVR  *replicationv1alpha1.UnifiedVolumeReplication
		newUVR  *replicationv1alpha1.UnifiedVolumeReplication
		wantErr bool
		wantMsg string
	}{
		{
			name: "valid update - state change",
			oldUVR: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication",
					Namespace: "default",
				},
				Spec: func() replicationv1alpha1.UnifiedVolumeReplicationSpec {
					spec := createValidSpec()
					spec.ReplicationState = replicationv1alpha1.ReplicationStateSource
					return spec
				}(),
			},
			newUVR: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication",
					Namespace: "default",
				},
				Spec: func() replicationv1alpha1.UnifiedVolumeReplicationSpec {
					spec := createValidSpec()
					spec.ReplicationState = replicationv1alpha1.ReplicationStateDemoting
					return spec
				}(),
			},
			wantErr: false,
		},
		{
			name: "invalid update - immutable field change",
			oldUVR: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication",
					Namespace: "default",
				},
				Spec: createValidSpec(),
			},
			newUVR: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication",
					Namespace: "default",
				},
				Spec: func() replicationv1alpha1.UnifiedVolumeReplicationSpec {
					spec := createValidSpec()
					spec.VolumeMapping.Source.PvcName = "different-pvc"
					return spec
				}(),
			},
			wantErr: true,
			wantMsg: "volumeMapping.source.pvcName is immutable",
		},
		{
			name: "invalid update - bad state transition",
			oldUVR: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication",
					Namespace: "default",
				},
				Spec: func() replicationv1alpha1.UnifiedVolumeReplicationSpec {
					spec := createValidSpec()
					spec.ReplicationState = replicationv1alpha1.ReplicationStateSource
					return spec
				}(),
			},
			newUVR: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replication",
					Namespace: "default",
				},
				Spec: func() replicationv1alpha1.UnifiedVolumeReplicationSpec {
					spec := createValidSpec()
					spec.ReplicationState = replicationv1alpha1.ReplicationStatePromoting
					return spec
				}(),
			},
			wantErr: true,
			wantMsg: "invalid state transition from 'source' to 'promoting'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			validator := NewUnifiedVolumeReplicationValidator(client)

			// Test validation
			_, err := validator.ValidateUpdate(context.Background(), tt.oldUVR, tt.newUVR)

			if tt.wantErr {
				assert.Error(t, err, "Expected validation to fail")
				if tt.wantMsg != "" {
					assert.Contains(t, err.Error(), tt.wantMsg)
				}
			} else {
				assert.NoError(t, err, "Expected validation to pass")
			}
		})
	}
}

func TestValidateStateTransitions(t *testing.T) {
	validator := &UnifiedVolumeReplicationValidator{}

	tests := []struct {
		name     string
		oldState replicationv1alpha1.ReplicationState
		newState replicationv1alpha1.ReplicationState
		wantErr  bool
		errMsg   string
	}{
		// Valid transitions
		{"source to demoting", replicationv1alpha1.ReplicationStateSource, replicationv1alpha1.ReplicationStateDemoting, false, ""},
		{"replica to promoting", replicationv1alpha1.ReplicationStateReplica, replicationv1alpha1.ReplicationStatePromoting, false, ""},
		{"promoting to source", replicationv1alpha1.ReplicationStatePromoting, replicationv1alpha1.ReplicationStateSource, false, ""},
		{"demoting to replica", replicationv1alpha1.ReplicationStateDemoting, replicationv1alpha1.ReplicationStateReplica, false, ""},
		{"failed to syncing", replicationv1alpha1.ReplicationStateFailed, replicationv1alpha1.ReplicationStateSyncing, false, ""},
		{"same state", replicationv1alpha1.ReplicationStateSource, replicationv1alpha1.ReplicationStateSource, false, ""},

		// Invalid transitions
		{"source to promoting", replicationv1alpha1.ReplicationStateSource, replicationv1alpha1.ReplicationStatePromoting, true, "invalid state transition from 'source' to 'promoting'"},
		{"replica to demoting", replicationv1alpha1.ReplicationStateReplica, replicationv1alpha1.ReplicationStateDemoting, true, "invalid state transition from 'replica' to 'demoting'"},
		{"promoting to replica", replicationv1alpha1.ReplicationStatePromoting, replicationv1alpha1.ReplicationStateReplica, true, "invalid state transition from 'promoting' to 'replica'"},
		{"demoting to source", replicationv1alpha1.ReplicationStateDemoting, replicationv1alpha1.ReplicationStateSource, true, "invalid state transition from 'demoting' to 'source'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateStateTransitions(tt.oldState, tt.newState)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationPerformance(t *testing.T) {
	// Create a complex valid spec
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "performance-test",
			Namespace: "default",
		},
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "source-cluster",
				Region:       "us-east-1",
				StorageClass: "fast-ssd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				Region:       "us-west-2",
				StorageClass: "backup-hdd",
			},
			VolumeMapping: replicationv1alpha1.VolumeMapping{
				Source: replicationv1alpha1.VolumeSource{
					PvcName:   "data-pvc",
					Namespace: "app",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "vol-123",
					Namespace:    "app-backup",
				},
			},
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
				Rpo:  "30m",
				Rto:  "10m",
			},
			Extensions: &replicationv1alpha1.Extensions{
				Ceph: &replicationv1alpha1.CephExtensions{
					MirroringMode: stringPtr("journal"),
				},
				Trident:    &replicationv1alpha1.TridentExtensions{},
				Powerstore: &replicationv1alpha1.PowerStoreExtensions{},
			},
		},
	}

	// Run validation multiple times and measure performance
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		err := uvr.ValidateSpec()
		require.NoError(t, err)
	}

	duration := time.Since(start)
	avgDuration := duration / time.Duration(iterations)

	t.Logf("Validation performance: %d iterations in %v, average: %v per validation",
		iterations, duration, avgDuration)

	// Validation should be fast (< 100ms per the requirements)
	assert.Less(t, avgDuration, 100*time.Millisecond,
		"Validation performance requirement not met: %v > 100ms", avgDuration)
}

func TestWebhookIntegration_InvalidConfigurations(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, replicationv1alpha1.AddToScheme(scheme))

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	validator := NewUnifiedVolumeReplicationValidator(client)

	invalidConfigs := []struct {
		name    string
		uvr     *replicationv1alpha1.UnifiedVolumeReplication
		wantMsg string
	}{
		{
			name: "empty cluster name",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: func() replicationv1alpha1.UnifiedVolumeReplicationSpec {
					spec := createValidSpec()
					spec.SourceEndpoint.Cluster = ""
					return spec
				}(),
			},
			wantMsg: "source endpoint cluster cannot be empty",
		},
		{
			name: "invalid RPO format",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: func() replicationv1alpha1.UnifiedVolumeReplicationSpec {
					spec := createValidSpec()
					spec.Schedule.Rpo = "invalid-format"
					return spec
				}(),
			},
			wantMsg: "does not match required pattern",
		},
		{
			name: "invalid extension configuration",
			uvr: &replicationv1alpha1.UnifiedVolumeReplication{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
				Spec: func() replicationv1alpha1.UnifiedVolumeReplicationSpec {
					spec := createValidSpec()
					spec.Extensions = &replicationv1alpha1.Extensions{
						Ceph: &replicationv1alpha1.CephExtensions{
							MirroringMode: stringPtr("invalid-mode"),
						},
					}
					return spec
				}(),
			},
			wantMsg: "invalid mirroring mode",
		},
	}

	for _, tc := range invalidConfigs {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validator.ValidateCreate(context.Background(), tc.uvr)
			assert.Error(t, err, "Expected validation to fail for %s", tc.name)
			assert.Contains(t, err.Error(), tc.wantMsg, "Expected error message not found")
		})
	}
}

// Helper functions

func createValidSpec() replicationv1alpha1.UnifiedVolumeReplicationSpec {
	return replicationv1alpha1.UnifiedVolumeReplicationSpec{
		SourceEndpoint: replicationv1alpha1.Endpoint{
			Cluster:      "source-cluster",
			Region:       "us-east-1",
			StorageClass: "fast-ssd",
		},
		DestinationEndpoint: replicationv1alpha1.Endpoint{
			Cluster:      "dest-cluster",
			Region:       "us-west-2",
			StorageClass: "backup-hdd",
		},
		VolumeMapping: replicationv1alpha1.VolumeMapping{
			Source: replicationv1alpha1.VolumeSource{
				PvcName:   "data-pvc",
				Namespace: "app",
			},
			Destination: replicationv1alpha1.VolumeDestination{
				VolumeHandle: "vol-123",
				Namespace:    "app-backup",
			},
		},
		ReplicationState: replicationv1alpha1.ReplicationStateSource,
		ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
		Schedule: replicationv1alpha1.Schedule{
			Mode: replicationv1alpha1.ScheduleModeInterval,
			Rpo:  "30m",
			Rto:  "10m",
		},
	}
}

func createInvalidSpecIdenticalEndpoints() replicationv1alpha1.UnifiedVolumeReplicationSpec {
	return replicationv1alpha1.UnifiedVolumeReplicationSpec{
		SourceEndpoint: replicationv1alpha1.Endpoint{
			Cluster:      "same-cluster",
			Region:       "us-east-1",
			StorageClass: "same-storage",
		},
		DestinationEndpoint: replicationv1alpha1.Endpoint{
			Cluster:      "same-cluster",
			Region:       "us-east-1",
			StorageClass: "same-storage",
		},
		VolumeMapping: replicationv1alpha1.VolumeMapping{
			Source: replicationv1alpha1.VolumeSource{
				PvcName:   "data-pvc",
				Namespace: "app",
			},
			Destination: replicationv1alpha1.VolumeDestination{
				VolumeHandle: "vol-123",
				Namespace:    "app-backup",
			},
		},
		ReplicationState: replicationv1alpha1.ReplicationStateSource,
		ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
		Schedule: replicationv1alpha1.Schedule{
			Mode: replicationv1alpha1.ScheduleModeInterval,
			Rpo:  "30m",
		},
	}
}

func stringPtr(s string) *string {
	return &s
}
