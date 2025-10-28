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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// COMPATIBILITY NOTICE:
// This API version (v1alpha2) is designed to be binary-compatible with
// kubernetes-csi-addons replication.storage.openshift.io/v1alpha1.
//
// VolumeGroupReplication enables replication of multiple PVCs together as a
// single unit for application consistency. This is critical for multi-volume
// applications like databases that require crash-consistent group snapshots.
//
// DO NOT add custom fields to VolumeGroupReplicationSpec or VolumeGroupReplicationStatus.
// Use VolumeGroupReplicationClass.parameters for backend-specific configuration.
//
// This compatibility ensures future migration to Option A (using
// replication.storage.openshift.io API group directly) will be straightforward.

// VolumeGroupReplicationSpec defines the desired state of VolumeGroupReplication
type VolumeGroupReplicationSpec struct {
	// VolumeGroupReplicationClass is the name of the VolumeGroupReplicationClass
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	VolumeGroupReplicationClass string `json:"volumeGroupReplicationClass"`

	// Selector is a label query to select PVCs that are part of this replication group
	// All PVCs matching this selector will be replicated together as a group
	// +kubebuilder:validation:Required
	Selector *metav1.LabelSelector `json:"selector"`

	// ReplicationState represents the desired replication state for the entire group
	// All volumes in the group will be transitioned to this state together
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=primary;secondary;resync
	ReplicationState string `json:"replicationState"`

	// AutoResync indicates if the volume group should be automatically resynced
	// +optional
	AutoResync *bool `json:"autoResync,omitempty"`

	// Source is an optional reference to the source volume group
	// +optional
	Source *corev1.TypedLocalObjectReference `json:"source,omitempty"`
}

// VolumeGroupReplicationStatus defines the observed state of VolumeGroupReplication
type VolumeGroupReplicationStatus struct {
	// Conditions represent the latest available observations of the group replication's current state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// State represents the current replication state of the volume group
	// +optional
	State string `json:"state,omitempty"`

	// Message provides detailed information about the current state
	// +optional
	Message string `json:"message,omitempty"`

	// LastSyncTime represents the time of last successful group synchronization
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// LastSyncDuration represents the duration of last group sync operation
	// +optional
	LastSyncDuration *metav1.Duration `json:"lastSyncDuration,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// PersistentVolumeClaimsRefList is the list of PVCs that are part of this volume group
	// +optional
	PersistentVolumeClaimsRefList []corev1.LocalObjectReference `json:"persistentVolumeClaimsRefList,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:resource:scope=Namespaced,shortName=vgr;volgrouprep
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.spec.replicationState`
//+kubebuilder:printcolumn:name="Class",type=string,JSONPath=`.spec.volumeGroupReplicationClass`
//+kubebuilder:printcolumn:name="PVCs",type=string,JSONPath=`.status.persistentVolumeClaimsRefList[*].name`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VolumeGroupReplication is the Schema for the volumegroupreplications API
// This resource enables replication of multiple PVCs together for application consistency
type VolumeGroupReplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumeGroupReplicationSpec   `json:"spec,omitempty"`
	Status VolumeGroupReplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VolumeGroupReplicationList contains a list of VolumeGroupReplication
type VolumeGroupReplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeGroupReplication `json:"items"`
}

// Note: init() is in groupversion_info.go to register all types together

// Default sets default values for VolumeGroupReplication
func (vgr *VolumeGroupReplication) Default() {
	if vgr.Spec.AutoResync == nil {
		autoResync := false
		vgr.Spec.AutoResync = &autoResync
	}
}
