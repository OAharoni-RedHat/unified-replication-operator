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
// DO NOT add custom fields to VolumeReplicationSpec or VolumeReplicationStatus.
// Use VolumeReplicationClass.parameters for backend-specific configuration.
//
// This compatibility ensures future migration to Option A (using
// replication.storage.openshift.io API group directly) will be straightforward.

// VolumeReplicationSpec defines the desired state of VolumeReplication
type VolumeReplicationSpec struct {
	// VolumeReplicationClass is the name of the VolumeReplicationClass
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	VolumeReplicationClass string `json:"volumeReplicationClass"`

	// PvcName is the name of the PersistentVolumeClaim to be replicated
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PvcName string `json:"pvcName"`

	// ReplicationState represents the desired replication state
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=primary;secondary;resync
	ReplicationState string `json:"replicationState"`

	// DataSource is an optional data source for cloning scenarios
	// +optional
	DataSource *corev1.TypedLocalObjectReference `json:"dataSource,omitempty"`

	// AutoResync indicates if the volume should be automatically resynced
	// +optional
	AutoResync *bool `json:"autoResync,omitempty"`
}

// VolumeReplicationStatus defines the observed state of VolumeReplication
type VolumeReplicationStatus struct {
	// Conditions represent the latest available observations of the replication's current state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// State represents the current replication state
	// +optional
	State string `json:"state,omitempty"`

	// Message provides detailed information about the current state
	// +optional
	Message string `json:"message,omitempty"`

	// LastSyncTime represents the time of last successful synchronization
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// LastSyncDuration represents the duration of last sync operation
	// +optional
	LastSyncDuration *metav1.Duration `json:"lastSyncDuration,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed spec
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:resource:scope=Namespaced,shortName=vr;volrep
//+kubebuilder:printcolumn:name="State",type=string,JSONPath=`.spec.replicationState`
//+kubebuilder:printcolumn:name="PVC",type=string,JSONPath=`.spec.pvcName`
//+kubebuilder:printcolumn:name="Class",type=string,JSONPath=`.spec.volumeReplicationClass`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VolumeReplication is the Schema for the volumereplications API
// This resource is compatible with kubernetes-csi-addons VolumeReplication
type VolumeReplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VolumeReplicationSpec   `json:"spec,omitempty"`
	Status VolumeReplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VolumeReplicationList contains a list of VolumeReplication
type VolumeReplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeReplication `json:"items"`
}

// Note: init() is in groupversion_info.go to register all types together

// Default sets default values for VolumeReplication
func (vr *VolumeReplication) Default() {
	if vr.Spec.AutoResync == nil {
		autoResync := false
		vr.Spec.AutoResync = &autoResync
	}
}
