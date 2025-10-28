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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Common parameters across backends:
// - replication.storage.openshift.io/replication-secret-name: Secret for authentication
// - replication.storage.openshift.io/replication-secret-namespace: Secret namespace
//
// Ceph-specific parameters:
// - mirroringMode: "snapshot" or "journal"
// - schedulingInterval: "5m", "15m", etc.
//
// Trident-specific parameters:
// - replicationPolicy: "Async" or "Sync"
// - replicationSchedule: "15m", "1h", etc.
// - remoteCluster: Name of remote cluster
// - remoteSVM: SVM name for remote cluster
// - remoteVolume: Remote volume handle
//
// Dell PowerStore-specific parameters:
// - protectionPolicy: Policy name (e.g., "15min-async")
// - remoteSystem: Remote system ID
// - rpo: RPO value (e.g., "15m")
// - remoteClusterId: Remote cluster identifier

// VolumeReplicationClassSpec defines the desired state of VolumeReplicationClass
type VolumeReplicationClassSpec struct {
	// Provisioner is the name of storage provisioner that handles replication
	// This is used to determine which backend adapter to use
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Provisioner string `json:"provisioner"`

	// Parameters is a key-value map with storage provisioner specific configurations
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster,shortName=vrc;volrepclass
//+kubebuilder:printcolumn:name="Provisioner",type=string,JSONPath=`.spec.provisioner`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VolumeReplicationClass is the Schema for the volumereplicationclasses API
//
// COMPATIBILITY NOTICE:
// This structure matches kubernetes-csi-addons VolumeReplicationClass.
// The provisioner field is used to detect which backend adapter to use.
type VolumeReplicationClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VolumeReplicationClassSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// VolumeReplicationClassList contains a list of VolumeReplicationClass
type VolumeReplicationClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeReplicationClass `json:"items"`
}

// Note: init() is in groupversion_info.go to register all types together
