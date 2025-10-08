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

// Package fixtures provides comprehensive test fixtures and sample data
package fixtures

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

// BasicReplicationSpec returns a basic valid replication spec
func BasicReplicationSpec() replicationv1alpha1.UnifiedVolumeReplicationSpec {
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
				PvcName:   "app-data",
				Namespace: "production",
			},
			Destination: replicationv1alpha1.VolumeDestination{
				VolumeHandle: "vol-backup-12345",
				Namespace:    "disaster-recovery",
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

// CephReplicationSpec returns a spec with Ceph extensions
func CephReplicationSpec() replicationv1alpha1.UnifiedVolumeReplicationSpec {
	spec := BasicReplicationSpec()
	spec.SourceEndpoint.StorageClass = "ceph-rbd"
	spec.DestinationEndpoint.StorageClass = "ceph-rbd-backup"

	spec.Extensions = &replicationv1alpha1.Extensions{
		Ceph: &replicationv1alpha1.CephExtensions{
			MirroringMode:       stringPtr("journal"),
			SchedulingStartTime: &metav1.Time{Time: time.Now().Add(1 * time.Hour)},
		},
	}

	return spec
}

// TridentReplicationSpec returns a spec with Trident extensions
func TridentReplicationSpec() replicationv1alpha1.UnifiedVolumeReplicationSpec {
	spec := BasicReplicationSpec()
	spec.SourceEndpoint.StorageClass = "trident-nas"
	spec.DestinationEndpoint.StorageClass = "trident-nas-backup"

	spec.Extensions = &replicationv1alpha1.Extensions{
		Trident: &replicationv1alpha1.TridentExtensions{
			Actions: []replicationv1alpha1.TridentAction{
				{
					Type:           "mirror-update",
					SnapshotHandle: "snapshot-daily-001",
				},
				{
					Type:           "mirror-update",
					SnapshotHandle: "snapshot-hourly-12",
				},
			},
		},
	}

	return spec
}

// PowerStoreReplicationSpec returns a spec with PowerStore extensions
func PowerStoreReplicationSpec() replicationv1alpha1.UnifiedVolumeReplicationSpec {
	spec := BasicReplicationSpec()
	spec.SourceEndpoint.StorageClass = "powerstore-block"
	spec.DestinationEndpoint.StorageClass = "powerstore-block-backup"
	spec.ReplicationMode = replicationv1alpha1.ReplicationModeSynchronous

	spec.Extensions = &replicationv1alpha1.Extensions{
		Powerstore: &replicationv1alpha1.PowerStoreExtensions{
			RpoSettings: stringPtr("Five_Minutes"),
			VolumeGroups: []string{
				"app-consistency-group-1",
				"database-consistency-group",
			},
		},
	}

	return spec
}

// MultiVendorReplicationSpec returns a spec with all vendor extensions
func MultiVendorReplicationSpec() replicationv1alpha1.UnifiedVolumeReplicationSpec {
	spec := BasicReplicationSpec()

	spec.Extensions = &replicationv1alpha1.Extensions{
		Ceph: &replicationv1alpha1.CephExtensions{
			MirroringMode: stringPtr("snapshot"),
		},
		Trident: &replicationv1alpha1.TridentExtensions{
			Actions: []replicationv1alpha1.TridentAction{
				{Type: "mirror-update", SnapshotHandle: "snap-001"},
			},
		},
		Powerstore: &replicationv1alpha1.PowerStoreExtensions{
			RpoSettings:  stringPtr("Fifteen_Minutes"),
			VolumeGroups: []string{"consistency-group-1"},
		},
	}

	return spec
}

// ValidReplicationStates returns all valid replication states
func ValidReplicationStates() []replicationv1alpha1.ReplicationState {
	return []replicationv1alpha1.ReplicationState{
		replicationv1alpha1.ReplicationStateSource,
		replicationv1alpha1.ReplicationStateReplica,
		replicationv1alpha1.ReplicationStatePromoting,
		replicationv1alpha1.ReplicationStateDemoting,
		replicationv1alpha1.ReplicationStateSyncing,
		replicationv1alpha1.ReplicationStateFailed,
	}
}

// ValidReplicationModes returns all valid replication modes
func ValidReplicationModes() []replicationv1alpha1.ReplicationMode {
	return []replicationv1alpha1.ReplicationMode{
		replicationv1alpha1.ReplicationModeSynchronous,
		replicationv1alpha1.ReplicationModeAsynchronous,
		replicationv1alpha1.ReplicationModeEventual,
	}
}

// ValidScheduleModes returns all valid schedule modes
func ValidScheduleModes() []replicationv1alpha1.ScheduleMode {
	return []replicationv1alpha1.ScheduleMode{
		replicationv1alpha1.ScheduleModeContinuous,
		replicationv1alpha1.ScheduleModeInterval,
		replicationv1alpha1.ScheduleModeManual,
	}
}

// ValidTimePatterns returns valid time patterns for RPO/RTO
func ValidTimePatterns() []string {
	return []string{
		"5s", "30s", "1m", "5m", "15m", "30m", "1h", "2h", "6h", "12h", "1d",
	}
}

// InvalidTimePatterns returns invalid time patterns for testing validation
func InvalidTimePatterns() []string {
	return []string{
		"", "5", "5x", "5ms", "1.5h", "30sm", "invalid", "5 minutes",
	}
}

// SampleConditions returns sample Kubernetes conditions
func SampleConditions() []metav1.Condition {
	return []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "ReplicationActive",
			Message:            "Replication is active and healthy",
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
		{
			Type:               "Synced",
			Status:             metav1.ConditionTrue,
			Reason:             "InSync",
			Message:            "Source and destination are synchronized",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		},
		{
			Type:               "Available",
			Status:             metav1.ConditionTrue,
			Reason:             "BackendAvailable",
			Message:            "Storage backend is available",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-10 * time.Minute)),
		},
	}
}

// FailureConditions returns sample failure conditions
func FailureConditions() []metav1.Condition {
	return []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "ReplicationFailed",
			Message:            "Replication has failed due to network connectivity issues",
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
		{
			Type:               "Synced",
			Status:             metav1.ConditionFalse,
			Reason:             "OutOfSync",
			Message:            "Source and destination are out of sync",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
		},
	}
}

// ProgressingConditions returns sample progressing conditions
func ProgressingConditions() []metav1.Condition {
	return []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "ReplicationProgressing",
			Message:            "Replication is being established",
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
		{
			Type:               "Progressing",
			Status:             metav1.ConditionTrue,
			Reason:             "InitialSync",
			Message:            "Performing initial synchronization",
			LastTransitionTime: metav1.NewTime(time.Now()),
		},
	}
}

// SampleBackends returns sample discovered backends
func SampleBackends() []replicationv1alpha1.BackendInfo {
	return []replicationv1alpha1.BackendInfo{
		{
			Name:      "ceph-rbd-backend",
			Type:      "ceph-csi",
			Available: true,
			Capabilities: []string{
				"async-replication",
				"snapshot-based",
				"bidirectional",
			},
		},
		{
			Name:      "trident-ontap-backend",
			Type:      "trident",
			Available: true,
			Capabilities: []string{
				"mirror-update",
				"scheduled-replication",
				"cross-cluster",
			},
		},
		{
			Name:      "powerstore-block-backend",
			Type:      "powerstore",
			Available: false,
			Capabilities: []string{
				"metro-replication",
				"consistency-groups",
				"rpo-based",
			},
		},
	}
}

// InvalidSpecs returns various invalid specs for validation testing
func InvalidSpecs() map[string]replicationv1alpha1.UnifiedVolumeReplicationSpec {
	return map[string]replicationv1alpha1.UnifiedVolumeReplicationSpec{
		"identical-endpoints": {
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
					PvcName:   "test-pvc",
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "vol-123",
					Namespace:    "default",
				},
			},
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
				Rpo:  "30m",
			},
		},
		"empty-cluster": {
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "", // Invalid: empty cluster
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
					PvcName:   "test-pvc",
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "vol-123",
					Namespace:    "default",
				},
			},
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
				Rpo:  "30m",
			},
		},
		"invalid-rpo-pattern": {
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
					PvcName:   "test-pvc",
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "vol-123",
					Namespace:    "default",
				},
			},
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
				Rpo:  "invalid-time", // Invalid RPO pattern
			},
		},
		"interval-without-rpo": {
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
					PvcName:   "test-pvc",
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "vol-123",
					Namespace:    "default",
				},
			},
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
				// Missing RPO for interval mode
			},
		},
		"invalid-ceph-mirroring": {
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "source-cluster",
				Region:       "us-east-1",
				StorageClass: "ceph-rbd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				Region:       "us-west-2",
				StorageClass: "ceph-rbd-backup",
			},
			VolumeMapping: replicationv1alpha1.VolumeMapping{
				Source: replicationv1alpha1.VolumeSource{
					PvcName:   "test-pvc",
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "vol-123",
					Namespace:    "default",
				},
			},
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
				Rpo:  "30m",
			},
			Extensions: &replicationv1alpha1.Extensions{
				Ceph: &replicationv1alpha1.CephExtensions{
					MirroringMode: stringPtr("invalid-mode"), // Invalid mirroring mode
				},
			},
		},
	}
}

// StateTransitionScenarios returns valid and invalid state transition scenarios
func StateTransitionScenarios() map[string]struct {
	From   replicationv1alpha1.ReplicationState
	To     replicationv1alpha1.ReplicationState
	Valid  bool
	Reason string
} {
	return map[string]struct {
		From   replicationv1alpha1.ReplicationState
		To     replicationv1alpha1.ReplicationState
		Valid  bool
		Reason string
	}{
		"source-to-demoting": {
			From:   replicationv1alpha1.ReplicationStateSource,
			To:     replicationv1alpha1.ReplicationStateDemoting,
			Valid:  true,
			Reason: "Source can be demoted to replica",
		},
		"replica-to-promoting": {
			From:   replicationv1alpha1.ReplicationStateReplica,
			To:     replicationv1alpha1.ReplicationStatePromoting,
			Valid:  true,
			Reason: "Replica can be promoted to source",
		},
		"promoting-to-source": {
			From:   replicationv1alpha1.ReplicationStatePromoting,
			To:     replicationv1alpha1.ReplicationStateSource,
			Valid:  true,
			Reason: "Promotion completes to source state",
		},
		"demoting-to-replica": {
			From:   replicationv1alpha1.ReplicationStateDemoting,
			To:     replicationv1alpha1.ReplicationStateReplica,
			Valid:  true,
			Reason: "Demotion completes to replica state",
		},
		"source-to-promoting": {
			From:   replicationv1alpha1.ReplicationStateSource,
			To:     replicationv1alpha1.ReplicationStatePromoting,
			Valid:  false,
			Reason: "Source cannot directly promote (already source)",
		},
		"replica-to-demoting": {
			From:   replicationv1alpha1.ReplicationStateReplica,
			To:     replicationv1alpha1.ReplicationStateDemoting,
			Valid:  false,
			Reason: "Replica cannot demote (already replica)",
		},
		"promoting-to-replica": {
			From:   replicationv1alpha1.ReplicationStatePromoting,
			To:     replicationv1alpha1.ReplicationStateReplica,
			Valid:  false,
			Reason: "Cannot go from promoting directly to replica",
		},
		"failed-to-syncing": {
			From:   replicationv1alpha1.ReplicationStateFailed,
			To:     replicationv1alpha1.ReplicationStateSyncing,
			Valid:  true,
			Reason: "Failed state can recover through syncing",
		},
	}
}

// SampleLabels returns sample labels for testing
func SampleLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       "unified-replication-operator",
		"app.kubernetes.io/instance":   "test-instance",
		"app.kubernetes.io/version":    "v1alpha1",
		"app.kubernetes.io/component":  "replication",
		"app.kubernetes.io/managed-by": "unified-replication-operator",
		"environment":                  "test",
		"replication.unified.io/type":  "cross-region",
	}
}

// SampleAnnotations returns sample annotations for testing
func SampleAnnotations() map[string]string {
	return map[string]string{
		"replication.unified.io/last-sync":                 time.Now().Format(time.RFC3339),
		"replication.unified.io/source-size":               "100Gi",
		"replication.unified.io/backend-type":              "ceph-csi",
		"kubectl.kubernetes.io/last-applied-configuration": `{"apiVersion":"replication.unified.io/v1alpha1","kind":"UnifiedVolumeReplication"}`,
	}
}

// CrossRegionScenarios returns various cross-region replication scenarios
func CrossRegionScenarios() map[string]replicationv1alpha1.UnifiedVolumeReplicationSpec {
	return map[string]replicationv1alpha1.UnifiedVolumeReplicationSpec{
		"us-east-to-west": {
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "prod-us-east-1",
				Region:       "us-east-1",
				StorageClass: "fast-ssd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dr-us-west-2",
				Region:       "us-west-2",
				StorageClass: "backup-ssd",
			},
			VolumeMapping:    BasicReplicationSpec().VolumeMapping,
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
				Rpo:  "1h",
				Rto:  "15m",
			},
		},
		"eu-to-us": {
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "prod-eu-west-1",
				Region:       "eu-west-1",
				StorageClass: "premium-ssd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dr-us-east-1",
				Region:       "us-east-1",
				StorageClass: "standard-ssd",
			},
			VolumeMapping:    BasicReplicationSpec().VolumeMapping,
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
				Rpo:  "4h",
				Rto:  "30m",
			},
		},
		"multi-region-sync": {
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "primary-ap-southeast-1",
				Region:       "ap-southeast-1",
				StorageClass: "powerstore-block",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "secondary-ap-northeast-1",
				Region:       "ap-northeast-1",
				StorageClass: "powerstore-block",
			},
			VolumeMapping:    BasicReplicationSpec().VolumeMapping,
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeSynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeContinuous,
				Rpo:  "5m",
				Rto:  "1m",
			},
			Extensions: &replicationv1alpha1.Extensions{
				Powerstore: &replicationv1alpha1.PowerStoreExtensions{
					RpoSettings:  stringPtr("Five_Minutes"),
					VolumeGroups: []string{"critical-apps-cg"},
				},
			},
		},
	}
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}
