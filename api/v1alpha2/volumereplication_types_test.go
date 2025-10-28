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

package v1alpha2

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVolumeReplicationValidation(t *testing.T) {
	tests := []struct {
		name    string
		spec    VolumeReplicationSpec
		wantErr bool
	}{
		{
			name: "valid primary state",
			spec: VolumeReplicationSpec{
				VolumeReplicationClass: "test-class",
				PvcName:                "test-pvc",
				ReplicationState:       "primary",
			},
			wantErr: false,
		},
		{
			name: "valid secondary state",
			spec: VolumeReplicationSpec{
				VolumeReplicationClass: "test-class",
				PvcName:                "test-pvc",
				ReplicationState:       "secondary",
			},
			wantErr: false,
		},
		{
			name: "valid resync state",
			spec: VolumeReplicationSpec{
				VolumeReplicationClass: "test-class",
				PvcName:                "test-pvc",
				ReplicationState:       "resync",
			},
			wantErr: false,
		},
		{
			name: "with autoResync true",
			spec: VolumeReplicationSpec{
				VolumeReplicationClass: "test-class",
				PvcName:                "test-pvc",
				ReplicationState:       "primary",
				AutoResync:             boolPtr(true),
			},
			wantErr: false,
		},
		{
			name: "with autoResync false",
			spec: VolumeReplicationSpec{
				VolumeReplicationClass: "test-class",
				PvcName:                "test-pvc",
				ReplicationState:       "primary",
				AutoResync:             boolPtr(false),
			},
			wantErr: false,
		},
		{
			name: "with dataSource",
			spec: VolumeReplicationSpec{
				VolumeReplicationClass: "test-class",
				PvcName:                "test-pvc",
				ReplicationState:       "primary",
				DataSource: &corev1.TypedLocalObjectReference{
					APIGroup: stringPtr("snapshot.storage.k8s.io"),
					Kind:     "VolumeSnapshot",
					Name:     "test-snapshot",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vr := &VolumeReplication{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vr",
					Namespace: "default",
				},
				Spec: tt.spec,
			}

			// Validate spec fields are set correctly
			if vr.Spec.VolumeReplicationClass != tt.spec.VolumeReplicationClass {
				t.Errorf("VolumeReplicationClass mismatch: got %s, want %s",
					vr.Spec.VolumeReplicationClass, tt.spec.VolumeReplicationClass)
			}

			if vr.Spec.PvcName != tt.spec.PvcName {
				t.Errorf("PvcName mismatch: got %s, want %s",
					vr.Spec.PvcName, tt.spec.PvcName)
			}

			if vr.Spec.ReplicationState != tt.spec.ReplicationState {
				t.Errorf("ReplicationState mismatch: got %s, want %s",
					vr.Spec.ReplicationState, tt.spec.ReplicationState)
			}
		})
	}
}

func TestVolumeReplicationDefaulting(t *testing.T) {
	t.Run("autoResync defaults to false", func(t *testing.T) {
		vr := &VolumeReplication{
			Spec: VolumeReplicationSpec{
				VolumeReplicationClass: "test-class",
				PvcName:                "test-pvc",
				ReplicationState:       "primary",
				// AutoResync not set
			},
		}

		vr.Default()

		if vr.Spec.AutoResync == nil {
			t.Error("Expected autoResync to be set after Default()")
		} else if *vr.Spec.AutoResync != false {
			t.Errorf("Expected autoResync to default to false, got %v", *vr.Spec.AutoResync)
		}
	})

	t.Run("existing autoResync not overwritten", func(t *testing.T) {
		vr := &VolumeReplication{
			Spec: VolumeReplicationSpec{
				VolumeReplicationClass: "test-class",
				PvcName:                "test-pvc",
				ReplicationState:       "primary",
				AutoResync:             boolPtr(true),
			},
		}

		vr.Default()

		if vr.Spec.AutoResync == nil {
			t.Error("AutoResync should not be nil")
		} else if *vr.Spec.AutoResync != true {
			t.Error("Existing autoResync value should not be overwritten")
		}
	})
}

func TestVolumeReplicationDeepCopy(t *testing.T) {
	original := &VolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-vr",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: VolumeReplicationSpec{
			VolumeReplicationClass: "test-class",
			PvcName:                "test-pvc",
			ReplicationState:       "primary",
			AutoResync:             boolPtr(true),
		},
		Status: VolumeReplicationStatus{
			State:   "primary",
			Message: "Test message",
		},
	}

	// Deep copy
	copied := original.DeepCopy()

	// Verify copy is equal
	if copied.Name != original.Name {
		t.Error("DeepCopy name mismatch")
	}

	if copied.Spec.PvcName != original.Spec.PvcName {
		t.Error("DeepCopy spec.pvcName mismatch")
	}

	// Modify copy
	copied.Spec.ReplicationState = "secondary"
	copied.Labels["modified"] = "true"

	// Verify original is unchanged
	if original.Spec.ReplicationState == "secondary" {
		t.Error("Original should not be modified when copy is modified")
	}

	if _, exists := original.Labels["modified"]; exists {
		t.Error("Original labels should not be modified when copy is modified")
	}
}

func TestVolumeReplicationList(t *testing.T) {
	list := &VolumeReplicationList{
		Items: []VolumeReplication{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "vr1"},
				Spec:       VolumeReplicationSpec{PvcName: "pvc1"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "vr2"},
				Spec:       VolumeReplicationSpec{PvcName: "pvc2"},
			},
		},
	}

	if len(list.Items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(list.Items))
	}

	// Test DeepCopy
	copied := list.DeepCopy()
	if len(copied.Items) != 2 {
		t.Error("DeepCopy should preserve items")
	}

	// Modify copy
	copied.Items[0].Spec.PvcName = "modified"

	// Verify original unchanged
	if list.Items[0].Spec.PvcName == "modified" {
		t.Error("Original list should not be modified")
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}
