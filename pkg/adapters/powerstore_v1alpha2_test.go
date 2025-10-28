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
)

func TestDellActionTranslationToAction(t *testing.T) {
	adapter := &PowerStoreV1Alpha2Adapter{}

	tests := []struct {
		name       string
		vrState    string
		dellAction string
	}{
		{
			name:       "primary to Failover",
			vrState:    "primary",
			dellAction: "Failover",
		},
		{
			name:       "secondary to Sync",
			vrState:    "secondary",
			dellAction: "Sync",
		},
		{
			name:       "resync to Reprotect",
			vrState:    "resync",
			dellAction: "Reprotect",
		},
		{
			name:       "unknown state defaults to Sync",
			vrState:    "unknown",
			dellAction: "Sync",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.translateStateToDellAction(tt.vrState)

			if result != tt.dellAction {
				t.Errorf("Translation failed: input=%s, got=%s, want=%s",
					tt.vrState, result, tt.dellAction)
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

func TestDellActionTranslationMappings(t *testing.T) {
	adapter := &PowerStoreV1Alpha2Adapter{}

	// Test the translation mappings are consistent
	t.Run("all kubernetes-csi-addons states have Dell actions", func(t *testing.T) {
		states := []string{"primary", "secondary", "resync"}

		for _, state := range states {
			action := adapter.translateStateToDellAction(state)
			if action == "" {
				t.Errorf("State %s should have a Dell action mapping", state)
			}
		}
	})

	t.Run("Dell actions map to meaningful states", func(t *testing.T) {
		// Verify that we can translate back from common Dell states
		dellStates := []string{"Synchronized", "Syncing", "FailedOver"}

		for _, dellState := range dellStates {
			vrState := adapter.translateStateFromDell(dellState)
			if vrState == "" {
				t.Errorf("Dell state %s should map to a VR state", dellState)
			}
		}
	})
}

func TestDellTranslationSemantics(t *testing.T) {
	adapter := &PowerStoreV1Alpha2Adapter{}

	t.Run("primary means failover to this site", func(t *testing.T) {
		action := adapter.translateStateToDellAction("primary")
		if action != "Failover" {
			t.Errorf("Primary should translate to Failover (make this site active), got %s", action)
		}
	})

	t.Run("secondary means sync from remote", func(t *testing.T) {
		action := adapter.translateStateToDellAction("secondary")
		if action != "Sync" {
			t.Errorf("Secondary should translate to Sync (receive from primary), got %s", action)
		}
	})

	t.Run("resync means re-establish protection", func(t *testing.T) {
		action := adapter.translateStateToDellAction("resync")
		if action != "Reprotect" {
			t.Errorf("Resync should translate to Reprotect (re-establish replication), got %s", action)
		}
	})
}
