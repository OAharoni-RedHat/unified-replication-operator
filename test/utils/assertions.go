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

package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

// AssertionHelper provides comprehensive assertion utilities for Kubernetes resources
type AssertionHelper struct {
	t *testing.T
}

// NewAssertionHelper creates a new assertion helper
func NewAssertionHelper(t *testing.T) *AssertionHelper {
	return &AssertionHelper{t: t}
}

// AssertCRDExists verifies that a CRD exists and has expected basic properties
func (h *AssertionHelper) AssertCRDExists(uvr *replicationv1alpha1.UnifiedVolumeReplication, expectedName, expectedNamespace string) {
	require.NotNil(h.t, uvr, "CRD should not be nil")
	assert.Equal(h.t, expectedName, uvr.Name, "CRD name should match")
	assert.Equal(h.t, expectedNamespace, uvr.Namespace, "CRD namespace should match")
	assert.NotEmpty(h.t, uvr.UID, "CRD should have a UID")
	assert.False(h.t, uvr.CreationTimestamp.IsZero(), "CRD should have a creation timestamp")
}

// AssertCRDSpec verifies the spec of a CRD matches expected values
func (h *AssertionHelper) AssertCRDSpec(uvr *replicationv1alpha1.UnifiedVolumeReplication, expectedSpec replicationv1alpha1.UnifiedVolumeReplicationSpec) {
	require.NotNil(h.t, uvr, "CRD should not be nil")

	// Assert endpoints
	assert.Equal(h.t, expectedSpec.SourceEndpoint, uvr.Spec.SourceEndpoint, "Source endpoint should match")
	assert.Equal(h.t, expectedSpec.DestinationEndpoint, uvr.Spec.DestinationEndpoint, "Destination endpoint should match")

	// Assert volume mapping
	assert.Equal(h.t, expectedSpec.VolumeMapping, uvr.Spec.VolumeMapping, "Volume mapping should match")

	// Assert replication configuration
	assert.Equal(h.t, expectedSpec.ReplicationState, uvr.Spec.ReplicationState, "Replication state should match")
	assert.Equal(h.t, expectedSpec.ReplicationMode, uvr.Spec.ReplicationMode, "Replication mode should match")
	assert.Equal(h.t, expectedSpec.Schedule, uvr.Spec.Schedule, "Schedule should match")

	// Assert extensions if present
	if expectedSpec.Extensions != nil {
		require.NotNil(h.t, uvr.Spec.Extensions, "Extensions should not be nil")
		h.assertExtensions(uvr.Spec.Extensions, expectedSpec.Extensions)
	} else {
		assert.Nil(h.t, uvr.Spec.Extensions, "Extensions should be nil")
	}
}

// assertExtensions verifies extensions match expected values
func (h *AssertionHelper) assertExtensions(actual, expected *replicationv1alpha1.Extensions) {
	// Assert Ceph extensions
	if expected.Ceph != nil {
		require.NotNil(h.t, actual.Ceph, "Ceph extensions should not be nil")
		if expected.Ceph.MirroringMode != nil {
			require.NotNil(h.t, actual.Ceph.MirroringMode, "Ceph mirroring mode should not be nil")
			assert.Equal(h.t, *expected.Ceph.MirroringMode, *actual.Ceph.MirroringMode, "Ceph mirroring mode should match")
		}
	} else {
		assert.Nil(h.t, actual.Ceph, "Ceph extensions should be nil")
	}

	// Assert Trident extensions
	if expected.Trident != nil {
		require.NotNil(h.t, actual.Trident, "Trident extensions should not be nil")
		assert.Equal(h.t, expected.Trident.Actions, actual.Trident.Actions, "Trident actions should match")
	} else {
		assert.Nil(h.t, actual.Trident, "Trident extensions should be nil")
	}

	// Assert PowerStore extensions
	if expected.Powerstore != nil {
		require.NotNil(h.t, actual.Powerstore, "PowerStore extensions should not be nil")
		if expected.Powerstore.RpoSettings != nil {
			require.NotNil(h.t, actual.Powerstore.RpoSettings, "PowerStore RPO settings should not be nil")
			assert.Equal(h.t, *expected.Powerstore.RpoSettings, *actual.Powerstore.RpoSettings, "PowerStore RPO settings should match")
		}
	} else {
		assert.Nil(h.t, actual.Powerstore, "PowerStore extensions should be nil")
	}
}

// AssertCRDStatus verifies the status of a CRD
func (h *AssertionHelper) AssertCRDStatus(uvr *replicationv1alpha1.UnifiedVolumeReplication, expectedObservedGeneration int64) {
	require.NotNil(h.t, uvr, "CRD should not be nil")
	assert.Equal(h.t, expectedObservedGeneration, uvr.Status.ObservedGeneration, "Observed generation should match")
}

// AssertConditionExists verifies that a specific condition exists in the CRD status
func (h *AssertionHelper) AssertConditionExists(uvr *replicationv1alpha1.UnifiedVolumeReplication, conditionType string) metav1.Condition {
	require.NotNil(h.t, uvr, "CRD should not be nil")

	for _, condition := range uvr.Status.Conditions {
		if condition.Type == conditionType {
			return condition
		}
	}

	h.t.Fatalf("Condition %s not found in CRD %s/%s", conditionType, uvr.Namespace, uvr.Name)
	return metav1.Condition{} // This line will never be reached
}

// AssertConditionStatus verifies that a condition has the expected status
func (h *AssertionHelper) AssertConditionStatus(uvr *replicationv1alpha1.UnifiedVolumeReplication, conditionType string, expectedStatus metav1.ConditionStatus) {
	condition := h.AssertConditionExists(uvr, conditionType)
	assert.Equal(h.t, expectedStatus, condition.Status, "Condition %s status should be %s", conditionType, expectedStatus)
}

// AssertConditionReason verifies that a condition has the expected reason
func (h *AssertionHelper) AssertConditionReason(uvr *replicationv1alpha1.UnifiedVolumeReplication, conditionType, expectedReason string) {
	condition := h.AssertConditionExists(uvr, conditionType)
	assert.Equal(h.t, expectedReason, condition.Reason, "Condition %s reason should be %s", conditionType, expectedReason)
}

// AssertConditionMessage verifies that a condition has the expected message
func (h *AssertionHelper) AssertConditionMessage(uvr *replicationv1alpha1.UnifiedVolumeReplication, conditionType, expectedMessage string) {
	condition := h.AssertConditionExists(uvr, conditionType)
	assert.Contains(h.t, condition.Message, expectedMessage, "Condition %s message should contain '%s'", conditionType, expectedMessage)
}

// AssertConditionUpdatedRecently verifies that a condition was updated recently
func (h *AssertionHelper) AssertConditionUpdatedRecently(uvr *replicationv1alpha1.UnifiedVolumeReplication, conditionType string, within time.Duration) {
	condition := h.AssertConditionExists(uvr, conditionType)
	timeSinceUpdate := time.Since(condition.LastTransitionTime.Time)
	assert.True(h.t, timeSinceUpdate <= within, "Condition %s should have been updated within %v (was %v ago)", conditionType, within, timeSinceUpdate)
}

// AssertBackendDiscovered verifies that a backend is discovered in the status
func (h *AssertionHelper) AssertBackendDiscovered(uvr *replicationv1alpha1.UnifiedVolumeReplication, backendName, backendType string, available bool) {
	require.NotNil(h.t, uvr, "CRD should not be nil")

	for _, backend := range uvr.Status.DiscoveredBackends {
		if backend.Name == backendName {
			assert.Equal(h.t, backendType, backend.Type, "Backend %s type should be %s", backendName, backendType)
			assert.Equal(h.t, available, backend.Available, "Backend %s availability should be %v", backendName, available)
			return
		}
	}

	h.t.Fatalf("Backend %s not found in discovered backends", backendName)
}

// AssertLabelsContain verifies that CRD contains expected labels
func (h *AssertionHelper) AssertLabelsContain(uvr *replicationv1alpha1.UnifiedVolumeReplication, expectedLabels map[string]string) {
	require.NotNil(h.t, uvr, "CRD should not be nil")

	for expectedKey, expectedValue := range expectedLabels {
		actualValue, exists := uvr.Labels[expectedKey]
		assert.True(h.t, exists, "Label %s should exist", expectedKey)
		assert.Equal(h.t, expectedValue, actualValue, "Label %s should have value %s", expectedKey, expectedValue)
	}
}

// AssertAnnotationsContain verifies that CRD contains expected annotations
func (h *AssertionHelper) AssertAnnotationsContain(uvr *replicationv1alpha1.UnifiedVolumeReplication, expectedAnnotations map[string]string) {
	require.NotNil(h.t, uvr, "CRD should not be nil")

	for expectedKey, expectedValue := range expectedAnnotations {
		actualValue, exists := uvr.Annotations[expectedKey]
		assert.True(h.t, exists, "Annotation %s should exist", expectedKey)
		assert.Equal(h.t, expectedValue, actualValue, "Annotation %s should have value %s", expectedKey, expectedValue)
	}
}

// AssertValidationError verifies that a validation error occurred with expected message
func (h *AssertionHelper) AssertValidationError(err error, expectedMessage string) {
	require.Error(h.t, err, "Validation should have failed")
	assert.Contains(h.t, err.Error(), expectedMessage, "Validation error should contain expected message")
}

// AssertValidationSuccess verifies that validation succeeded
func (h *AssertionHelper) AssertValidationSuccess(err error) {
	assert.NoError(h.t, err, "Validation should have succeeded")
}

// AssertCRDListLength verifies the length of a CRD list
func (h *AssertionHelper) AssertCRDListLength(list *replicationv1alpha1.UnifiedVolumeReplicationList, expectedLength int) {
	require.NotNil(h.t, list, "CRD list should not be nil")
	assert.Len(h.t, list.Items, expectedLength, "CRD list should have %d items", expectedLength)
}

// AssertCRDInList verifies that a specific CRD exists in a list
func (h *AssertionHelper) AssertCRDInList(list *replicationv1alpha1.UnifiedVolumeReplicationList, name, namespace string) *replicationv1alpha1.UnifiedVolumeReplication {
	require.NotNil(h.t, list, "CRD list should not be nil")

	for _, item := range list.Items {
		if item.Name == name && item.Namespace == namespace {
			return &item
		}
	}

	h.t.Fatalf("CRD %s/%s not found in list", namespace, name)
	return nil // This line will never be reached
}

// AssertTimeWithinRange verifies that a time is within a specific range
func (h *AssertionHelper) AssertTimeWithinRange(actual time.Time, expected time.Time, tolerance time.Duration) {
	diff := actual.Sub(expected)
	if diff < 0 {
		diff = -diff
	}
	assert.True(h.t, diff <= tolerance, "Time difference %v should be within tolerance %v", diff, tolerance)
}

// AssertReplicationStateTransition verifies a valid state transition
func (h *AssertionHelper) AssertReplicationStateTransition(fromState, toState replicationv1alpha1.ReplicationState, shouldBeValid bool) {
	// Define valid transitions (same as in webhook validation)
	validTransitions := map[replicationv1alpha1.ReplicationState][]replicationv1alpha1.ReplicationState{
		replicationv1alpha1.ReplicationStateSource: {
			replicationv1alpha1.ReplicationStateDemoting,
			replicationv1alpha1.ReplicationStateFailed,
			replicationv1alpha1.ReplicationStateSyncing,
		},
		replicationv1alpha1.ReplicationStateReplica: {
			replicationv1alpha1.ReplicationStatePromoting,
			replicationv1alpha1.ReplicationStateFailed,
			replicationv1alpha1.ReplicationStateSyncing,
		},
		replicationv1alpha1.ReplicationStatePromoting: {
			replicationv1alpha1.ReplicationStateSource,
			replicationv1alpha1.ReplicationStateFailed,
		},
		replicationv1alpha1.ReplicationStateDemoting: {
			replicationv1alpha1.ReplicationStateReplica,
			replicationv1alpha1.ReplicationStateFailed,
		},
		replicationv1alpha1.ReplicationStateSyncing: {
			replicationv1alpha1.ReplicationStateSource,
			replicationv1alpha1.ReplicationStateReplica,
			replicationv1alpha1.ReplicationStateFailed,
		},
		replicationv1alpha1.ReplicationStateFailed: {
			replicationv1alpha1.ReplicationStateSyncing,
			replicationv1alpha1.ReplicationStateSource,
			replicationv1alpha1.ReplicationStateReplica,
		},
	}

	isValid := fromState == toState // Same state is always valid
	if !isValid {
		if allowed, exists := validTransitions[fromState]; exists {
			for _, allowedState := range allowed {
				if toState == allowedState {
					isValid = true
					break
				}
			}
		}
	}

	if shouldBeValid {
		assert.True(h.t, isValid, "Transition from %s to %s should be valid", fromState, toState)
	} else {
		assert.False(h.t, isValid, "Transition from %s to %s should be invalid", fromState, toState)
	}
}

// AssertPerformance verifies that an operation completed within expected time
func (h *AssertionHelper) AssertPerformance(operation string, duration time.Duration, maxExpected time.Duration) {
	assert.True(h.t, duration <= maxExpected, "%s took %v, which exceeds maximum expected %v", operation, duration, maxExpected)
}

// AssertCRDSpecEquals compares two CRD specs for equality with detailed error messages
func (h *AssertionHelper) AssertCRDSpecEquals(actual, expected replicationv1alpha1.UnifiedVolumeReplicationSpec, message string) {
	// Compare endpoints
	assert.Equal(h.t, expected.SourceEndpoint, actual.SourceEndpoint, "%s: source endpoint mismatch", message)
	assert.Equal(h.t, expected.DestinationEndpoint, actual.DestinationEndpoint, "%s: destination endpoint mismatch", message)

	// Compare volume mapping
	assert.Equal(h.t, expected.VolumeMapping, actual.VolumeMapping, "%s: volume mapping mismatch", message)

	// Compare replication settings
	assert.Equal(h.t, expected.ReplicationState, actual.ReplicationState, "%s: replication state mismatch", message)
	assert.Equal(h.t, expected.ReplicationMode, actual.ReplicationMode, "%s: replication mode mismatch", message)
	assert.Equal(h.t, expected.Schedule, actual.Schedule, "%s: schedule mismatch", message)

	// Compare extensions
	assert.Equal(h.t, expected.Extensions, actual.Extensions, "%s: extensions mismatch", message)
}
