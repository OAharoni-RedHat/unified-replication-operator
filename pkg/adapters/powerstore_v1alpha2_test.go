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
	"testing"

	replicationv1alpha2 "github.com/unified-replication/operator/api/v1alpha2"
)

func TestDellActionDetermination(t *testing.T) {
	adapter := &PowerStoreV1Alpha2Adapter{}

	tests := []struct {
		name         string
		currentState string
		desiredState string
		existingDRG  bool
		wantAction   string
		description  string
	}{
		{
			name:         "initial creation as primary - no action",
			currentState: "",
			desiredState: "primary",
			existingDRG:  false,
			wantAction:   "", // Dell manages via PVC permissions, no initial action
			description:  "Initial creation doesn't need action, Dell sets up via protection policy",
		},
		{
			name:         "initial creation as secondary - no action",
			currentState: "",
			desiredState: "secondary",
			existingDRG:  false,
			wantAction:   "", // Dell manages via PVC permissions, no initial action
			description:  "Initial creation doesn't need action, Dell sets up via protection policy",
		},
		{
			name:         "transition: secondary to primary (failover)",
			currentState: "secondary",
			desiredState: "primary",
			existingDRG:  true,
			wantAction:   "Failover",
			description:  "Failover is an explicit operation that needs an action",
		},
		{
			name:         "transition: primary to secondary - no action",
			currentState: "primary",
			desiredState: "secondary",
			existingDRG:  true,
			wantAction:   "", // Dell handles demotion via PVC permission changes
			description:  "Demotion is handled by Dell via PVC permissions, no action needed",
		},
		{
			name:         "steady state primary",
			currentState: "primary",
			desiredState: "primary",
			existingDRG:  true,
			wantAction:   "", // No action for steady state
			description:  "Steady state, Dell maintains replication",
		},
		{
			name:         "steady state secondary",
			currentState: "secondary",
			desiredState: "secondary",
			existingDRG:  true,
			wantAction:   "", // No action for steady state
			description:  "Steady state, Dell maintains replication",
		},
		{
			name:         "resync requested from primary",
			currentState: "primary",
			desiredState: "resync",
			existingDRG:  true,
			wantAction:   "Reprotect",
			description:  "Resync is an explicit operation that needs an action",
		},
		{
			name:         "resync requested from secondary",
			currentState: "secondary",
			desiredState: "resync",
			existingDRG:  true,
			wantAction:   "Reprotect",
			description:  "Resync is an explicit operation that needs an action",
		},
		{
			name:         "already in resync state",
			currentState: "resync",
			desiredState: "resync",
			existingDRG:  true,
			wantAction:   "",
			description:  "Already resyncing, no need to trigger again",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vr := &replicationv1alpha2.VolumeReplication{
				Spec: replicationv1alpha2.VolumeReplicationSpec{
					ReplicationState: tt.desiredState,
				},
				Status: replicationv1alpha2.VolumeReplicationStatus{
					State: tt.currentState,
				},
			}

			result := adapter.determineRequiredAction(vr, tt.existingDRG, nil)

			if result != tt.wantAction {
				t.Errorf("Action determination failed: %s\ncurrentState=%s, desiredState=%s, got=%s, want=%s",
					tt.description, tt.currentState, tt.desiredState, result, tt.wantAction)
			}
		})
	}
}

func TestDellStateTranslationFromDell(t *testing.T) {
	adapter := &PowerStoreV1Alpha2Adapter{}

	tests := []struct {
		name      string
		dellState string
		vrState   string
	}{
		{
			name:      "Synchronized to secondary",
			dellState: "Synchronized",
			vrState:   "secondary",
		},
		{
			name:      "Syncing to secondary",
			dellState: "Syncing",
			vrState:   "secondary",
		},
		{
			name:      "FailedOver to primary",
			dellState: "FailedOver",
			vrState:   "primary",
		},
		{
			name:      "unknown state defaults to secondary",
			dellState: "Unknown",
			vrState:   "secondary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.translateStateFromDell(tt.dellState)

			if result != tt.vrState {
				t.Errorf("Translation failed: input=%s, got=%s, want=%s",
					tt.dellState, result, tt.vrState)
			}
		})
	}
}

// REMOVED: TestDellActionTranslationMappings and TestDellTranslationSemantics
// These tested the old static translation model which has been replaced
// with event-driven action determination (only set actions during transitions)
//
// The new model:
// - Actions are for operations (failover, sync, reprotect), not steady-state
// - Primary/secondary is determined by PVC read-write permissions
// - Actions are only set when state transitions are detected
//
// See TestDellActionDetermination for the new test approach
