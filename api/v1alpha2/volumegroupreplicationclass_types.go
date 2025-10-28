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

// Parameters for volume group replication:
//
// Common parameters across backends:
// - consistencyGroup: "enabled" - ensure crash consistency across volumes in group
// - groupSnapshots: "true" - use volume group snapshots for consistency guarantees
// - replication.storage.openshift.io/replication-secret-name: Secret for authentication
// - replication.storage.openshift.io/replication-secret-namespace: Secret namespace
//
// Ceph-specific parameters:
// - groupConsistency: "application" or "crash" - consistency level for group
// - groupMirroringMode: "snapshot" or "journal" - mirroring mode for the group
// - schedulingInterval: "5m", "15m", etc. - group snapshot schedule
//
// Trident-specific parameters:
// - consistencyGroupPolicy: Policy name for consistency group
// - replicationPolicy: "Async" or "Sync" - replication mode for the group
// - groupReplicationSchedule: "15m", "1h", etc. - schedule for group replication
// - remoteCluster: Name of remote cluster
// - remoteSVM: SVM name for remote cluster
//
// Dell PowerStore-specific parameters:
// - consistencyType: "Metro" or "Async" - consistency type for group
// - protectionPolicy: Policy name for group protection (e.g., "group-15min-async")
// - remoteSystem: Remote system ID
// - rpo: RPO value for the group (e.g., "15m")
// - remoteClusterId: Remote cluster identifier

// VolumeGroupReplicationClassSpec defines the desired state of VolumeGroupReplicationClass
type VolumeGroupReplicationClassSpec struct {
	// Provisioner is the name of storage provisioner that handles group replication
	// This is used to determine which backend adapter to use
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Provisioner string `json:"provisioner"`

	// Parameters is a key-value map with storage provisioner specific configurations for group replication
	// +optional
	Parameters map[string]string `json:"parameters,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Cluster,shortName=vgrc;volgrouprepclass
//+kubebuilder:printcolumn:name="Provisioner",type=string,JSONPath=`.spec.provisioner`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// VolumeGroupReplicationClass is the Schema for the volumegroupreplicationclasses API
//
// COMPATIBILITY NOTICE:
// This structure matches kubernetes-csi-addons VolumeGroupReplicationClass.
// The provisioner field is used to detect which backend adapter to use for group replication.
type VolumeGroupReplicationClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VolumeGroupReplicationClassSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// VolumeGroupReplicationClassList contains a list of VolumeGroupReplicationClass
type VolumeGroupReplicationClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeGroupReplicationClass `json:"items"`
}

// Note: init() is in groupversion_info.go to register all types together
