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

func TestTridentStateTranslationToTrident(t *testing.T) {
	adapter := &TridentV1Alpha2Adapter{}

	tests := []struct {
		name         string
		vrState      string
		tridentState string
	}{
		{
			name:         "primary to established",
			vrState:      "primary",
			tridentState: "established",
		},
		{
			name:         "secondary to reestablished",
			vrState:      "secondary",
			tridentState: "reestablished", // Note: reestablisheD with 'd'
		},
		{
			name:         "resync to reestablished",
			vrState:      "resync",
			tridentState: "reestablished", // Note: reestablisheD with 'd'
		},
		{
			name:         "unknown state defaults to established",
			vrState:      "unknown",
			tridentState: "established",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.translateStateToTrident(tt.vrState)

			if result != tt.tridentState {
				t.Errorf("Translation failed: input=%s, got=%s, want=%s",
					tt.vrState, result, tt.tridentState)
			}
		})
	}
}

func TestTridentStateTranslationFromTrident(t *testing.T) {
	adapter := &TridentV1Alpha2Adapter{}

	tests := []struct {
		name         string
		tridentState string
		vrState      string
	}{
		{
			name:         "established to primary",
			tridentState: "established",
			vrState:      "primary",
		},
		{
			name:         "reestablished to secondary",
			tridentState: "reestablished", // Note: reestablisheD with 'd'
			vrState:      "secondary",
		},
		{
			name:         "promoted to primary",
			tridentState: "promoted",
			vrState:      "primary",
		},
		{
			name:         "unknown state passthrough",
			tridentState: "unknown",
			vrState:      "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.translateStateFromTrident(tt.tridentState)

			if result != tt.vrState {
				t.Errorf("Translation failed: input=%s, got=%s, want=%s",
					tt.tridentState, result, tt.vrState)
			}
		})
	}
}

func TestTridentStateRoundTrip(t *testing.T) {
	adapter := &TridentV1Alpha2Adapter{}

	tests := []struct {
		name    string
		vrState string
	}{
		{name: "primary roundtrip", vrState: "primary"},
		{name: "secondary roundtrip", vrState: "secondary"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Translate to Trident
			tridentState := adapter.translateStateToTrident(tt.vrState)

			// Translate back
			backToVR := adapter.translateStateFromTrident(tridentState)

			// For primary and secondary, we should get back the same state
			if tt.vrState == "primary" || tt.vrState == "secondary" {
				if backToVR != tt.vrState {
					t.Errorf("Roundtrip failed: started with %s, got back %s (via %s)",
						tt.vrState, backToVR, tridentState)
				}
			}
		})
	}
}
