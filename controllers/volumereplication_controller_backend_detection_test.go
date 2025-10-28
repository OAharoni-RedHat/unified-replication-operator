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

package controllers

import (
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/unified-replication/operator/pkg/translation"
)

func TestBackendDetection(t *testing.T) {
	// Setup logger for tests
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := ctrl.Log.WithName("test")

	reconciler := &VolumeReplicationReconciler{}

	tests := []struct {
		name        string
		provisioner string
		expected    translation.Backend
		shouldError bool
	}{
		// Ceph provisioners
		{
			name:        "Ceph RBD provisioner",
			provisioner: "rbd.csi.ceph.com",
			expected:    translation.BackendCeph,
			shouldError: false,
		},
		{
			name:        "Ceph CephFS provisioner",
			provisioner: "cephfs.csi.ceph.com",
			expected:    translation.BackendCeph,
			shouldError: false,
		},
		{
			name:        "Ceph with substring",
			provisioner: "my-ceph-provisioner",
			expected:    translation.BackendCeph,
			shouldError: false,
		},
		// Trident provisioners
		{
			name:        "Trident official provisioner",
			provisioner: "csi.trident.netapp.io",
			expected:    translation.BackendTrident,
			shouldError: false,
		},
		{
			name:        "Trident with substring",
			provisioner: "trident-san",
			expected:    translation.BackendTrident,
			shouldError: false,
		},
		{
			name:        "NetApp provisioner",
			provisioner: "netapp.io/trident",
			expected:    translation.BackendTrident,
			shouldError: false,
		},
		// Dell PowerStore provisioners
		{
			name:        "Dell PowerStore official provisioner",
			provisioner: "csi-powerstore.dellemc.com",
			expected:    translation.BackendPowerStore,
			shouldError: false,
		},
		{
			name:        "Dell with substring powerstore",
			provisioner: "my-powerstore",
			expected:    translation.BackendPowerStore,
			shouldError: false,
		},
		{
			name:        "Dell with substring dellemc",
			provisioner: "dellemc.com/csi",
			expected:    translation.BackendPowerStore,
			shouldError: false,
		},
		// Unknown provisioner
		{
			name:        "Unknown provisioner",
			provisioner: "unknown.provisioner.io",
			expected:    "",
			shouldError: true,
		},
		// Case insensitivity
		{
			name:        "Ceph uppercase",
			provisioner: "RBD.CSI.CEPH.COM",
			expected:    translation.BackendCeph,
			shouldError: false,
		},
		{
			name:        "Trident mixed case",
			provisioner: "CSI.Trident.NetApp.IO",
			expected:    translation.BackendTrident,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := reconciler.detectBackend(tt.provisioner, log)

			if tt.shouldError {
				if err == nil {
					t.Error("Expected error for unknown provisioner, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if backend != tt.expected {
					t.Errorf("Backend detection failed: got %s, want %s", backend, tt.expected)
				}
			}
		})
	}
}

func TestBackendDetectionForVolumeGroup(t *testing.T) {
	// Setup logger
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := ctrl.Log.WithName("test")

	reconciler := &VolumeGroupReplicationReconciler{}

	tests := []struct {
		name        string
		provisioner string
		expected    translation.Backend
	}{
		{
			name:        "Ceph for volume group",
			provisioner: "rbd.csi.ceph.com",
			expected:    translation.BackendCeph,
		},
		{
			name:        "Trident for volume group",
			provisioner: "csi.trident.netapp.io",
			expected:    translation.BackendTrident,
		},
		{
			name:        "Dell for volume group",
			provisioner: "csi-powerstore.dellemc.com",
			expected:    translation.BackendPowerStore,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := reconciler.detectBackend(tt.provisioner, log)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if backend != tt.expected {
				t.Errorf("Backend detection failed: got %s, want %s", backend, tt.expected)
			}
		})
	}
}
