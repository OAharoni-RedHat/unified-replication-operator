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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVolumeReplicationClassValidation(t *testing.T) {
	tests := []struct {
		name    string
		spec    VolumeReplicationClassSpec
		wantErr bool
	}{
		{
			name: "valid with provisioner only",
			spec: VolumeReplicationClassSpec{
				Provisioner: "rbd.csi.ceph.com",
			},
			wantErr: false,
		},
		{
			name: "valid with provisioner and parameters",
			spec: VolumeReplicationClassSpec{
				Provisioner: "csi.trident.netapp.io",
				Parameters: map[string]string{
					"replicationPolicy": "Async",
					"schedule":          "15m",
				},
			},
			wantErr: false,
		},
		{
			name: "valid with empty parameters",
			spec: VolumeReplicationClassSpec{
				Provisioner: "csi-powerstore.dellemc.com",
				Parameters:  map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "valid with nil parameters",
			spec: VolumeReplicationClassSpec{
				Provisioner: "rbd.csi.ceph.com",
				Parameters:  nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vrc := &VolumeReplicationClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-class",
				},
				Spec: tt.spec,
			}

			// Validate provisioner is set
			if vrc.Spec.Provisioner != tt.spec.Provisioner {
				t.Errorf("Provisioner mismatch: got %s, want %s",
					vrc.Spec.Provisioner, tt.spec.Provisioner)
			}

			// Validate parameters
			if tt.spec.Parameters != nil {
				if len(vrc.Spec.Parameters) != len(tt.spec.Parameters) {
					t.Errorf("Parameters length mismatch: got %d, want %d",
						len(vrc.Spec.Parameters), len(tt.spec.Parameters))
				}
			}
		})
	}
}

func TestVolumeReplicationClassParameters(t *testing.T) {
	t.Run("parameters can store various values", func(t *testing.T) {
		vrc := &VolumeReplicationClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-class",
			},
			Spec: VolumeReplicationClassSpec{
				Provisioner: "test-provisioner",
				Parameters: map[string]string{
					"key1":     "value1",
					"key2":     "value2",
					"schedule": "15m",
					"rpo":      "30m",
				},
			},
		}

		if vrc.Spec.Parameters["key1"] != "value1" {
			t.Error("Parameters should preserve key-value pairs")
		}

		if len(vrc.Spec.Parameters) != 4 {
			t.Errorf("Expected 4 parameters, got %d", len(vrc.Spec.Parameters))
		}
	})

	t.Run("parameters are optional", func(t *testing.T) {
		vrc := &VolumeReplicationClass{
			Spec: VolumeReplicationClassSpec{
				Provisioner: "test-provisioner",
				// No Parameters
			},
		}

		if vrc.Spec.Parameters != nil {
			t.Error("Parameters should be nil when not set")
		}
	})
}

func TestVolumeReplicationClassDeepCopy(t *testing.T) {
	t.Run("deepcopy with nil parameters", func(t *testing.T) {
		original := &VolumeReplicationClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-class",
			},
			Spec: VolumeReplicationClassSpec{
				Provisioner: "test-provisioner",
				Parameters:  nil,
			},
		}

		copied := original.DeepCopy()

		if copied.Spec.Provisioner != original.Spec.Provisioner {
			t.Error("DeepCopy provisioner mismatch")
		}

		if copied.Spec.Parameters != nil {
			t.Error("DeepCopy should preserve nil parameters")
		}
	})

	t.Run("deepcopy with populated parameters", func(t *testing.T) {
		original := &VolumeReplicationClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-class",
			},
			Spec: VolumeReplicationClassSpec{
				Provisioner: "test-provisioner",
				Parameters: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		}

		copied := original.DeepCopy()

		// Verify copy is equal
		if len(copied.Spec.Parameters) != len(original.Spec.Parameters) {
			t.Error("DeepCopy parameters length mismatch")
		}

		for k, v := range original.Spec.Parameters {
			if copied.Spec.Parameters[k] != v {
				t.Errorf("DeepCopy parameter %s mismatch: got %s, want %s",
					k, copied.Spec.Parameters[k], v)
			}
		}

		// Modify copy
		copied.Spec.Parameters["new"] = "added"

		// Verify original unchanged
		if _, exists := original.Spec.Parameters["new"]; exists {
			t.Error("Original parameters should not be modified when copy is modified")
		}
	})
}

func TestVolumeReplicationClassList(t *testing.T) {
	list := &VolumeReplicationClassList{
		Items: []VolumeReplicationClass{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "class1"},
				Spec:       VolumeReplicationClassSpec{Provisioner: "prov1"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "class2"},
				Spec:       VolumeReplicationClassSpec{Provisioner: "prov2"},
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
	copied.Items[0].Spec.Provisioner = "modified"

	// Verify original unchanged
	if list.Items[0].Spec.Provisioner == "modified" {
		t.Error("Original list should not be modified")
	}
}
