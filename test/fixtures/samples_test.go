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

package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

func TestBasicReplicationSpec(t *testing.T) {
	spec := BasicReplicationSpec()

	assert.Equal(t, "source-cluster", spec.SourceEndpoint.Cluster)
	assert.Equal(t, "dest-cluster", spec.DestinationEndpoint.Cluster)
	assert.Equal(t, "app-data", spec.VolumeMapping.Source.PvcName)
	assert.Equal(t, replicationv1alpha1.ReplicationStateSource, spec.ReplicationState)
	assert.Equal(t, replicationv1alpha1.ReplicationModeAsynchronous, spec.ReplicationMode)
	assert.Equal(t, replicationv1alpha1.ScheduleModeInterval, spec.Schedule.Mode)

	// Validate the spec
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{Spec: spec}
	err := uvr.ValidateSpec()
	assert.NoError(t, err, "Basic spec should be valid")
}

func TestCephReplicationSpec(t *testing.T) {
	spec := CephReplicationSpec()

	require.NotNil(t, spec.Extensions)
	require.NotNil(t, spec.Extensions.Ceph)
	assert.Equal(t, "journal", *spec.Extensions.Ceph.MirroringMode)
	assert.Contains(t, spec.SourceEndpoint.StorageClass, "ceph")
	assert.Contains(t, spec.DestinationEndpoint.StorageClass, "ceph")

	// Validate the spec
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{Spec: spec}
	err := uvr.ValidateSpec()
	assert.NoError(t, err, "Ceph spec should be valid")
}

func TestTridentReplicationSpec(t *testing.T) {
	spec := TridentReplicationSpec()

	require.NotNil(t, spec.Extensions)
	require.NotNil(t, spec.Extensions.Trident)
	assert.Contains(t, spec.SourceEndpoint.StorageClass, "trident")
	assert.Contains(t, spec.DestinationEndpoint.StorageClass, "trident")

	// Validate the spec
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{Spec: spec}
	err := uvr.ValidateSpec()
	assert.NoError(t, err, "Trident spec should be valid")
}

func TestPowerStoreReplicationSpec(t *testing.T) {
	spec := PowerStoreReplicationSpec()

	require.NotNil(t, spec.Extensions)
	require.NotNil(t, spec.Extensions.Powerstore)
	assert.Contains(t, spec.SourceEndpoint.StorageClass, "powerstore")
	assert.Equal(t, replicationv1alpha1.ReplicationModeSynchronous, spec.ReplicationMode)

	// Validate the spec
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{Spec: spec}
	err := uvr.ValidateSpec()
	assert.NoError(t, err, "PowerStore spec should be valid")
}

func TestMultiVendorReplicationSpec(t *testing.T) {
	spec := MultiVendorReplicationSpec()

	require.NotNil(t, spec.Extensions)
	require.NotNil(t, spec.Extensions.Ceph)
	require.NotNil(t, spec.Extensions.Trident)
	require.NotNil(t, spec.Extensions.Powerstore)

	assert.Equal(t, "snapshot", *spec.Extensions.Ceph.MirroringMode)

	// Validate the spec
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{Spec: spec}
	err := uvr.ValidateSpec()
	assert.NoError(t, err, "Multi-vendor spec should be valid")
}

func TestValidReplicationStates(t *testing.T) {
	states := ValidReplicationStates()

	assert.Len(t, states, 6, "Should have all 6 replication states")
	assert.Contains(t, states, replicationv1alpha1.ReplicationStateSource)
	assert.Contains(t, states, replicationv1alpha1.ReplicationStateReplica)
	assert.Contains(t, states, replicationv1alpha1.ReplicationStatePromoting)
	assert.Contains(t, states, replicationv1alpha1.ReplicationStateDemoting)
	assert.Contains(t, states, replicationv1alpha1.ReplicationStateSyncing)
	assert.Contains(t, states, replicationv1alpha1.ReplicationStateFailed)
}

func TestValidReplicationModes(t *testing.T) {
	modes := ValidReplicationModes()

	assert.Len(t, modes, 3, "Should have all 3 replication modes")
	assert.Contains(t, modes, replicationv1alpha1.ReplicationModeSynchronous)
	assert.Contains(t, modes, replicationv1alpha1.ReplicationModeAsynchronous)
	assert.Contains(t, modes, replicationv1alpha1.ReplicationModeAsynchronous)
}

// TestValidScheduleModes verifies that all schedule modes are defined.
// Current modes:
// - continuous: Replication runs continuously
// - interval: Replication runs at specified intervals (RPO-based)
func TestValidScheduleModes(t *testing.T) {
	modes := ValidScheduleModes()

	assert.Len(t, modes, 2, "Should have all defined schedule modes")

	// Verify each mode is valid
	for _, mode := range modes {
		assert.NotEmpty(t, string(mode), "Schedule mode should not be empty")
	}

	// Verify the modes are the expected ones
	assert.Contains(t, modes, replicationv1alpha1.ScheduleModeContinuous)
	assert.Contains(t, modes, replicationv1alpha1.ScheduleModeInterval)
}

func TestValidTimePatterns(t *testing.T) {
	patterns := ValidTimePatterns()

	assert.Greater(t, len(patterns), 0, "Should have valid time patterns")

	// Test each pattern matches the expected regex
	timeRegex := `^[0-9]+(s|m|h|d)$`
	for _, pattern := range patterns {
		assert.Regexp(t, timeRegex, pattern, "Pattern %s should match time regex", pattern)
	}
}

func TestInvalidTimePatterns(t *testing.T) {
	patterns := InvalidTimePatterns()

	assert.Greater(t, len(patterns), 0, "Should have invalid time patterns")

	// These patterns should NOT match the time regex
	timeRegex := `^[0-9]+(s|m|h|d)$`
	for _, pattern := range patterns {
		assert.NotRegexp(t, timeRegex, pattern, "Pattern %s should NOT match time regex", pattern)
	}
}

func TestSampleConditions(t *testing.T) {
	conditions := SampleConditions()

	assert.Greater(t, len(conditions), 0, "Should have sample conditions")

	for _, condition := range conditions {
		assert.NotEmpty(t, condition.Type, "Condition type should not be empty")
		assert.NotEmpty(t, condition.Status, "Condition status should not be empty")
		assert.NotEmpty(t, condition.Reason, "Condition reason should not be empty")
		assert.NotEmpty(t, condition.Message, "Condition message should not be empty")
		assert.False(t, condition.LastTransitionTime.IsZero(), "LastTransitionTime should not be zero")
	}
}

func TestFailureConditions(t *testing.T) {
	conditions := FailureConditions()

	assert.Greater(t, len(conditions), 0, "Should have failure conditions")

	// All failure conditions should have False status
	for _, condition := range conditions {
		assert.Equal(t, "False", string(condition.Status), "Failure condition should have False status")
		assert.NotEmpty(t, condition.Reason, "Failure condition should have a reason")
		assert.Contains(t, condition.Message, "fail", "Failure condition message should indicate failure")
	}
}

func TestProgressingConditions(t *testing.T) {
	conditions := ProgressingConditions()

	assert.Greater(t, len(conditions), 0, "Should have progressing conditions")

	// Should have at least one progressing condition
	hasProgressingCondition := false
	for _, condition := range conditions {
		if condition.Type == "Progressing" && condition.Status == "True" {
			hasProgressingCondition = true
			break
		}
	}
	assert.True(t, hasProgressingCondition, "Should have at least one progressing condition")
}

func TestSampleBackends(t *testing.T) {
	backends := SampleBackends()

	assert.Greater(t, len(backends), 0, "Should have sample backends")

	expectedBackends := map[string]string{
		"ceph-rbd-backend":         "ceph-csi",
		"trident-ontap-backend":    "trident",
		"powerstore-block-backend": "powerstore",
	}

	for _, backend := range backends {
		assert.NotEmpty(t, backend.Name, "Backend name should not be empty")
		assert.NotEmpty(t, backend.Type, "Backend type should not be empty")

		expectedType, exists := expectedBackends[backend.Name]
		if exists {
			assert.Equal(t, expectedType, backend.Type, "Backend %s should have type %s", backend.Name, expectedType)
		}
	}
}

func TestInvalidSpecs(t *testing.T) {
	specs := InvalidSpecs()

	assert.Greater(t, len(specs), 0, "Should have invalid specs")

	for name, spec := range specs {
		t.Run(name, func(t *testing.T) {
			uvr := &replicationv1alpha1.UnifiedVolumeReplication{Spec: spec}
			err := uvr.ValidateSpec()
			assert.Error(t, err, "Spec %s should be invalid", name)
		})
	}
}

func TestStateTransitionScenarios(t *testing.T) {
	scenarios := StateTransitionScenarios()

	assert.Greater(t, len(scenarios), 0, "Should have state transition scenarios")

	for name, scenario := range scenarios {
		t.Run(name, func(t *testing.T) {
			// Test the actual transition logic would be here
			// For now, just verify the scenario structure
			assert.NotEmpty(t, scenario.From, "From state should not be empty")
			assert.NotEmpty(t, scenario.To, "To state should not be empty")
			assert.NotEmpty(t, scenario.Reason, "Reason should not be empty")

			// Verify the expected validity matches our validation logic
			if scenario.Valid {
				assert.Contains(t, scenario.Reason, "can", "Valid transition reason should indicate permission")
			} else {
				assert.Contains(t, scenario.Reason, "cannot", "Invalid transition reason should indicate restriction")
			}
		})
	}
}

func TestSampleLabels(t *testing.T) {
	labels := SampleLabels()

	assert.Greater(t, len(labels), 0, "Should have sample labels")
	assert.Contains(t, labels, "app.kubernetes.io/name")
	assert.Contains(t, labels, "app.kubernetes.io/instance")
	assert.Contains(t, labels, "app.kubernetes.io/version")

	// Verify standard Kubernetes labels format
	for key, value := range labels {
		assert.NotEmpty(t, key, "Label key should not be empty")
		assert.NotEmpty(t, value, "Label value should not be empty")
	}
}

func TestSampleAnnotations(t *testing.T) {
	annotations := SampleAnnotations()

	assert.Greater(t, len(annotations), 0, "Should have sample annotations")

	// Verify annotations have meaningful content
	for key, value := range annotations {
		assert.NotEmpty(t, key, "Annotation key should not be empty")
		assert.NotEmpty(t, value, "Annotation value should not be empty")
	}
}

func TestCrossRegionScenarios(t *testing.T) {
	scenarios := CrossRegionScenarios()

	assert.Greater(t, len(scenarios), 0, "Should have cross-region scenarios")

	for name, spec := range scenarios {
		t.Run(name, func(t *testing.T) {
			// Verify it's actually cross-region
			assert.NotEqual(t, spec.SourceEndpoint.Region, spec.DestinationEndpoint.Region,
				"Scenario %s should be cross-region", name)

			// Validate the spec
			uvr := &replicationv1alpha1.UnifiedVolumeReplication{Spec: spec}
			err := uvr.ValidateSpec()
			assert.NoError(t, err, "Cross-region scenario %s should be valid", name)
		})
	}
}
