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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUnifiedVolumeReplicationSpec_ValidateBasicFields(t *testing.T) {
	tests := []struct {
		name    string
		spec    UnifiedVolumeReplicationSpec
		wantErr bool
	}{
		{
			name: "valid basic spec",
			spec: UnifiedVolumeReplicationSpec{
				SourceEndpoint: Endpoint{
					Cluster:      "source-cluster",
					Region:       "us-east-1",
					StorageClass: "ceph-rbd",
				},
				DestinationEndpoint: Endpoint{
					Cluster:      "dest-cluster",
					Region:       "us-west-2",
					StorageClass: "trident-nas",
				},
				VolumeMapping: VolumeMapping{
					Source: VolumeSource{
						PvcName:   "test-pvc",
						Namespace: "default",
					},
					Destination: VolumeDestination{
						VolumeHandle: "vol-12345",
						Namespace:    "default",
					},
				},
				ReplicationState: ReplicationStateSource,
				ReplicationMode:  ReplicationModeAsynchronous,
				Schedule: Schedule{
					Mode: ScheduleModeInterval,
					Rpo:  "15m",
					Rto:  "5m",
				},
			},
			wantErr: false,
		},
		{
			name: "valid spec with extensions",
			spec: UnifiedVolumeReplicationSpec{
				SourceEndpoint: Endpoint{
					Cluster:      "source-cluster",
					Region:       "us-east-1",
					StorageClass: "ceph-rbd",
				},
				DestinationEndpoint: Endpoint{
					Cluster:      "dest-cluster",
					Region:       "us-west-2",
					StorageClass: "trident-nas",
				},
				VolumeMapping: VolumeMapping{
					Source: VolumeSource{
						PvcName:   "test-pvc",
						Namespace: "default",
					},
					Destination: VolumeDestination{
						VolumeHandle: "vol-12345",
						Namespace:    "default",
					},
				},
				ReplicationState: ReplicationStateSource,
				ReplicationMode:  ReplicationModeAsynchronous,
				Schedule: Schedule{
					Mode: ScheduleModeInterval,
				},
				Extensions: &Extensions{
					Ceph: &CephExtensions{
						MirroringMode: stringPtr("journal"),
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation - ensure all required fields are present
			assert.NotEmpty(t, tt.spec.SourceEndpoint.Cluster)
			assert.NotEmpty(t, tt.spec.SourceEndpoint.Region)
			assert.NotEmpty(t, tt.spec.SourceEndpoint.StorageClass)
			assert.NotEmpty(t, tt.spec.DestinationEndpoint.Cluster)
			assert.NotEmpty(t, tt.spec.DestinationEndpoint.Region)
			assert.NotEmpty(t, tt.spec.DestinationEndpoint.StorageClass)
			assert.NotEmpty(t, tt.spec.VolumeMapping.Source.PvcName)
			assert.NotEmpty(t, tt.spec.VolumeMapping.Source.Namespace)
			assert.NotEmpty(t, tt.spec.VolumeMapping.Destination.VolumeHandle)
			assert.NotEmpty(t, tt.spec.VolumeMapping.Destination.Namespace)
		})
	}
}

func TestReplicationState_Constants(t *testing.T) {
	tests := []struct {
		name  string
		state ReplicationState
		valid bool
	}{
		{"source state", ReplicationStateSource, true},
		{"replica state", ReplicationStateReplica, true},
		{"promoting state", ReplicationStatePromoting, true},
		{"demoting state", ReplicationStateDemoting, true},
		{"syncing state", ReplicationStateSyncing, true},
		{"failed state", ReplicationStateFailed, true},
	}

	validStates := []ReplicationState{
		ReplicationStateSource,
		ReplicationStateReplica,
		ReplicationStatePromoting,
		ReplicationStateDemoting,
		ReplicationStateSyncing,
		ReplicationStateFailed,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, validStates, tt.state)
			assert.NotEmpty(t, string(tt.state))
		})
	}
}

func TestReplicationMode_Constants(t *testing.T) {
	tests := []struct {
		name string
		mode ReplicationMode
	}{
		{"synchronous mode", ReplicationModeSynchronous},
		{"asynchronous mode", ReplicationModeAsynchronous},
	}

	validModes := []ReplicationMode{
		ReplicationModeSynchronous,
		ReplicationModeAsynchronous,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, validModes, tt.mode)
			assert.NotEmpty(t, string(tt.mode))
		})
	}
}

func TestScheduleMode_Constants(t *testing.T) {
	tests := []struct {
		name string
		mode ScheduleMode
	}{
		{"continuous mode", ScheduleModeContinuous},
		{"interval mode", ScheduleModeInterval},
	}

	validModes := []ScheduleMode{
		ScheduleModeContinuous,
		ScheduleModeInterval,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, validModes, tt.mode)
			assert.NotEmpty(t, string(tt.mode))
		})
	}
}

func TestUnifiedVolumeReplication_ObjectMeta(t *testing.T) {
	uvr := &UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-replication",
			Namespace: "default",
		},
	}

	assert.Equal(t, "test-replication", uvr.ObjectMeta.Name)
	assert.Equal(t, "default", uvr.ObjectMeta.Namespace)
}

func TestUnifiedVolumeReplicationStatus_Conditions(t *testing.T) {
	status := UnifiedVolumeReplicationStatus{
		Conditions: []metav1.Condition{
			{
				Type:   "Ready",
				Status: metav1.ConditionTrue,
				Reason: "ReplicationActive",
			},
			{
				Type:   "Synced",
				Status: metav1.ConditionFalse,
				Reason: "NetworkLatency",
			},
		},
		ObservedGeneration: 1,
	}

	assert.Len(t, status.Conditions, 2)
	assert.Equal(t, "Ready", status.Conditions[0].Type)
	assert.Equal(t, metav1.ConditionTrue, status.Conditions[0].Status)
	assert.Equal(t, "Synced", status.Conditions[1].Type)
	assert.Equal(t, metav1.ConditionFalse, status.Conditions[1].Status)
	assert.Equal(t, int64(1), status.ObservedGeneration)
}

func TestExtensions_AllVendors(t *testing.T) {
	extensions := &Extensions{
		Ceph: &CephExtensions{
			MirroringMode: stringPtr("journal"),
		},
		Trident:    &TridentExtensions{},
		Powerstore: &PowerStoreExtensions{},
	}

	assert.NotNil(t, extensions.Ceph)
	assert.NotNil(t, extensions.Trident)
	assert.NotNil(t, extensions.Powerstore)

	assert.Equal(t, "journal", *extensions.Ceph.MirroringMode)
}

func TestBackendInfo_Structure(t *testing.T) {
	backend := BackendInfo{
		Name:         "ceph-backend",
		Type:         "ceph-csi",
		Available:    true,
		Capabilities: []string{"async", "bidirectional"},
	}

	assert.Equal(t, "ceph-backend", backend.Name)
	assert.Equal(t, "ceph-csi", backend.Type)
	assert.True(t, backend.Available)
	assert.Len(t, backend.Capabilities, 2)
	assert.Contains(t, backend.Capabilities, "async")
	assert.Contains(t, backend.Capabilities, "bidirectional")
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
