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
)

func TestUnifiedVolumeReplication_ValidateSpec(t *testing.T) {
	tests := []struct {
		name    string
		uvr     *UnifiedVolumeReplication
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid complete spec",
			uvr: &UnifiedVolumeReplication{
				Spec: UnifiedVolumeReplicationSpec{
					SourceEndpoint: Endpoint{
						Cluster:      "source-cluster",
						Region:       "us-east-1",
						StorageClass: "fast-ssd",
					},
					DestinationEndpoint: Endpoint{
						Cluster:      "dest-cluster",
						Region:       "us-west-2",
						StorageClass: "backup-hdd",
					},
					VolumeMapping: VolumeMapping{
						Source: VolumeSource{
							PvcName:   "data-pvc",
							Namespace: "app",
						},
						Destination: VolumeDestination{
							VolumeHandle: "vol-123",
							Namespace:    "app-backup",
						},
					},
					ReplicationState: ReplicationStateSource,
					ReplicationMode:  ReplicationModeAsynchronous,
					Schedule: Schedule{
						Mode: ScheduleModeInterval,
						Rpo:  "30m",
						Rto:  "10m",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "identical endpoints should fail",
			uvr: &UnifiedVolumeReplication{
				Spec: UnifiedVolumeReplicationSpec{
					SourceEndpoint: Endpoint{
						Cluster:      "same-cluster",
						Region:       "us-east-1",
						StorageClass: "fast-ssd",
					},
					DestinationEndpoint: Endpoint{
						Cluster:      "same-cluster",
						Region:       "us-east-1",
						StorageClass: "fast-ssd",
					},
					VolumeMapping: VolumeMapping{
						Source: VolumeSource{
							PvcName:   "data-pvc",
							Namespace: "app",
						},
						Destination: VolumeDestination{
							VolumeHandle: "vol-123",
							Namespace:    "app-backup",
						},
					},
					ReplicationState: ReplicationStateSource,
					ReplicationMode:  ReplicationModeAsynchronous,
					Schedule: Schedule{
						Mode: ScheduleModeInterval,
						Rpo:  "30m",
					},
				},
			},
			wantErr: true,
			errMsg:  "source and destination endpoints cannot be identical",
		},
		{
			name: "invalid RPO pattern should fail",
			uvr: &UnifiedVolumeReplication{
				Spec: UnifiedVolumeReplicationSpec{
					SourceEndpoint: Endpoint{
						Cluster:      "source-cluster",
						Region:       "us-east-1",
						StorageClass: "fast-ssd",
					},
					DestinationEndpoint: Endpoint{
						Cluster:      "dest-cluster",
						Region:       "us-west-2",
						StorageClass: "backup-hdd",
					},
					VolumeMapping: VolumeMapping{
						Source: VolumeSource{
							PvcName:   "data-pvc",
							Namespace: "app",
						},
						Destination: VolumeDestination{
							VolumeHandle: "vol-123",
							Namespace:    "app-backup",
						},
					},
					ReplicationState: ReplicationStateSource,
					ReplicationMode:  ReplicationModeAsynchronous,
					Schedule: Schedule{
						Mode: ScheduleModeInterval,
						Rpo:  "invalid-time",
					},
				},
			},
			wantErr: true,
			errMsg:  "does not match required pattern",
		},
		{
			name: "interval mode without RPO should fail",
			uvr: &UnifiedVolumeReplication{
				Spec: UnifiedVolumeReplicationSpec{
					SourceEndpoint: Endpoint{
						Cluster:      "source-cluster",
						Region:       "us-east-1",
						StorageClass: "fast-ssd",
					},
					DestinationEndpoint: Endpoint{
						Cluster:      "dest-cluster",
						Region:       "us-west-2",
						StorageClass: "backup-hdd",
					},
					VolumeMapping: VolumeMapping{
						Source: VolumeSource{
							PvcName:   "data-pvc",
							Namespace: "app",
						},
						Destination: VolumeDestination{
							VolumeHandle: "vol-123",
							Namespace:    "app-backup",
						},
					},
					ReplicationState: ReplicationStateSource,
					ReplicationMode:  ReplicationModeAsynchronous,
					Schedule: Schedule{
						Mode: ScheduleModeInterval,
						// Missing RPO
					},
				},
			},
			wantErr: true,
			errMsg:  "schedule RPO is required when mode is 'interval'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.uvr.ValidateSpec()
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

func TestValidateEndpoints(t *testing.T) {
	tests := []struct {
		name    string
		src     Endpoint
		dest    Endpoint
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid different endpoints",
			src:     Endpoint{Cluster: "cluster1", Region: "us-east-1", StorageClass: "fast"},
			dest:    Endpoint{Cluster: "cluster2", Region: "us-west-2", StorageClass: "slow"},
			wantErr: false,
		},
		{
			name:    "identical endpoints",
			src:     Endpoint{Cluster: "cluster1", Region: "us-east-1", StorageClass: "fast"},
			dest:    Endpoint{Cluster: "cluster1", Region: "us-east-1", StorageClass: "fast"},
			wantErr: true,
			errMsg:  "cannot be identical",
		},
		{
			name:    "empty cluster name",
			src:     Endpoint{Cluster: "", Region: "us-east-1", StorageClass: "fast"},
			dest:    Endpoint{Cluster: "cluster2", Region: "us-west-2", StorageClass: "slow"},
			wantErr: true,
			errMsg:  "source endpoint cluster cannot be empty",
		},
		{
			name:    "invalid kubernetes name",
			src:     Endpoint{Cluster: "Invalid_Cluster_Name", Region: "us-east-1", StorageClass: "fast"},
			dest:    Endpoint{Cluster: "cluster2", Region: "us-west-2", StorageClass: "slow"},
			wantErr: true,
			errMsg:  "is not a valid Kubernetes name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uvr := &UnifiedVolumeReplication{
				Spec: UnifiedVolumeReplicationSpec{
					SourceEndpoint:      tt.src,
					DestinationEndpoint: tt.dest,
				},
			}
			err := uvr.validateEndpoints()
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

func TestValidateSchedule(t *testing.T) {
	tests := []struct {
		name     string
		schedule Schedule
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid interval with RPO",
			schedule: Schedule{Mode: ScheduleModeInterval, Rpo: "15m", Rto: "5m"},
			wantErr:  false,
		},
		{
			name:     "valid continuous mode",
			schedule: Schedule{Mode: ScheduleModeContinuous, Rpo: "1h"},
			wantErr:  false,
		},
		{
			name:     "valid manual mode",
			schedule: Schedule{Mode: ScheduleModeManual},
			wantErr:  false,
		},
		{
			name:     "interval without RPO",
			schedule: Schedule{Mode: ScheduleModeInterval},
			wantErr:  true,
			errMsg:   "schedule RPO is required when mode is 'interval'",
		},
		{
			name:     "invalid RPO pattern",
			schedule: Schedule{Mode: ScheduleModeInterval, Rpo: "invalid"},
			wantErr:  true,
			errMsg:   "does not match required pattern",
		},
		{
			name:     "invalid RTO pattern",
			schedule: Schedule{Mode: ScheduleModeInterval, Rpo: "15m", Rto: "invalid"},
			wantErr:  true,
			errMsg:   "does not match required pattern",
		},
		{
			name:     "valid time patterns",
			schedule: Schedule{Mode: ScheduleModeInterval, Rpo: "5s", Rto: "1d"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uvr := &UnifiedVolumeReplication{
				Spec: UnifiedVolumeReplicationSpec{
					Schedule: tt.schedule,
				},
			}
			err := uvr.validateSchedule()
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

func TestValidateExtensions(t *testing.T) {
	tests := []struct {
		name       string
		extensions *Extensions
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "nil extensions",
			extensions: nil,
			wantErr:    false,
		},
		{
			name: "valid ceph extensions",
			extensions: &Extensions{
				Ceph: &CephExtensions{
					MirroringMode: stringPtr("journal"),
				},
			},
			wantErr: false,
		},
		{
			name: "invalid ceph mirroring mode",
			extensions: &Extensions{
				Ceph: &CephExtensions{
					MirroringMode: stringPtr("invalid"),
				},
			},
			wantErr: true,
			errMsg:  "invalid mirroring mode",
		},
		{
			name: "valid trident extensions",
			extensions: &Extensions{
				Trident: &TridentExtensions{},
			},
			wantErr: false,
		},
		{
			name: "valid powerstore extensions",
			extensions: &Extensions{
				Powerstore: &PowerStoreExtensions{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uvr := &UnifiedVolumeReplication{
				Spec: UnifiedVolumeReplicationSpec{
					Extensions: tt.extensions,
				},
			}
			err := uvr.validateExtensions()
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

func TestValidateCephExtensions(t *testing.T) {
	tests := []struct {
		name    string
		ceph    *CephExtensions
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid journal mode",
			ceph:    &CephExtensions{MirroringMode: stringPtr("journal")},
			wantErr: false,
		},
		{
			name:    "valid snapshot mode",
			ceph:    &CephExtensions{MirroringMode: stringPtr("snapshot")},
			wantErr: false,
		},
		{
			name:    "invalid mirroring mode",
			ceph:    &CephExtensions{MirroringMode: stringPtr("invalid")},
			wantErr: true,
			errMsg:  "invalid mirroring mode 'invalid'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCephExtensions(tt.ceph)
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

func TestIsValidKubernetesName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid simple name", "test", true},
		{"valid with dash", "test-name", true},
		{"valid with dot", "test.name", true},
		{"valid complex", "app-1.backend", true},
		{"empty string", "", false},
		{"too long", string(make([]byte, 254)), false},
		{"uppercase", "TestName", false},
		{"starts with dash", "-test", false},
		{"ends with dash", "test-", false},
		{"starts with dot", ".test", false},
		{"ends with dot", "test.", false},
		{"underscore not allowed", "test_name", false},
		{"special characters", "test@name", false},
		{"single character", "a", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidKubernetesName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTimePatternRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid seconds", "30s", true},
		{"valid minutes", "15m", true},
		{"valid hours", "2h", true},
		{"valid days", "1d", true},
		{"multiple digits", "123m", true},
		{"zero value", "0s", true},
		{"invalid no unit", "30", false},
		{"invalid multiple units", "30sm", false},
		{"invalid characters", "30x", false},
		{"invalid format", "s30", false},
		{"empty string", "", false},
		{"only unit", "s", false},
		{"float not allowed", "1.5h", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := timePatternRegex.MatchString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
