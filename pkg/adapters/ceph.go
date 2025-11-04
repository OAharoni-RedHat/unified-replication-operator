// Copyright 2024 unified-replication-operator contributors.
// Licensed under the Apache License, Version 2.0.

package adapters

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// CephRBDStorageClass is the expected storage class for Ceph RBD volumes
	CephRBDStorageClass = "rbd"

	// VolumeReplication is the Ceph-CSI VolumeReplication CRD
	VolumeReplicationAPIVersion = "replication.storage.openshift.io/v1alpha1"
	VolumeReplicationKind       = "VolumeReplication"

	// Ceph-specific constants
	CephPrimaryState   = "primary"
	CephSecondaryState = "secondary"
)

// VolumeReplication represents the Ceph-CSI VolumeReplication CRD
type VolumeReplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              VolumeReplicationSpec   `json:"spec,omitempty"`
	Status            VolumeReplicationStatus `json:"status,omitempty"`
}

// DeepCopyObject implements runtime.Object
func (vr *VolumeReplication) DeepCopyObject() runtime.Object {
	if vr == nil {
		return nil
	}
	out := new(VolumeReplication)
	vr.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (vr *VolumeReplication) DeepCopyInto(out *VolumeReplication) {
	*out = *vr
	out.TypeMeta = vr.TypeMeta
	vr.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	vr.Spec.DeepCopyInto(&out.Spec)
	vr.Status.DeepCopyInto(&out.Status)
}

// VolumeReplicationSpec defines the desired state of VolumeReplication
type VolumeReplicationSpec struct {
	// volumeReplicationClass is the VolumeReplicationClass name
	VolumeReplicationClass string `json:"volumeReplicationClass"`
	// pvcName contains the name of the PVC
	PvcName string `json:"pvcName"`
	// replicationState is the state of the volume being replicated
	ReplicationState string `json:"replicationState"`
	// dataSource contains the data source information
	DataSource *corev1.VolumeSource `json:"dataSource,omitempty"`
	// autoResync indicates if the volume should be automatically resynced
	AutoResync *bool `json:"autoResync,omitempty"`
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (vrs *VolumeReplicationSpec) DeepCopyInto(out *VolumeReplicationSpec) {
	*out = *vrs
	if vrs.DataSource != nil {
		in, out := &vrs.DataSource, &out.DataSource
		*out = new(corev1.VolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if vrs.AutoResync != nil {
		in, out := &vrs.AutoResync, &out.AutoResync
		*out = new(bool)
		**out = **in
	}
}

// VolumeReplicationStatus defines the observed state of VolumeReplication
type VolumeReplicationStatus struct {
	// conditions contains the list of status conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// state represents the current state of the volume replication
	State string `json:"state,omitempty"`
	// message provides detailed information about the current state
	Message string `json:"message,omitempty"`
	// lastSyncTime represents the last time the volume was synced
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	// lastSyncDuration represents the duration of the last sync
	LastSyncDuration *metav1.Duration `json:"lastSyncDuration,omitempty"`
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (vrs *VolumeReplicationStatus) DeepCopyInto(out *VolumeReplicationStatus) {
	*out = *vrs
	if vrs.Conditions != nil {
		in, out := &vrs.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if vrs.LastSyncTime != nil {
		in, out := &vrs.LastSyncTime, &out.LastSyncTime
		*out = (*in).DeepCopy()
	}
	if vrs.LastSyncDuration != nil {
		in, out := &vrs.LastSyncDuration, &out.LastSyncDuration
		*out = new(metav1.Duration)
		**out = **in
	}
}

// VolumeReplicationList contains a list of VolumeReplication
type VolumeReplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeReplication `json:"items"`
}

// DeepCopyObject implements runtime.Object
func (vrl *VolumeReplicationList) DeepCopyObject() runtime.Object {
	if vrl == nil {
		return nil
	}
	out := new(VolumeReplicationList)
	vrl.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (vrl *VolumeReplicationList) DeepCopyInto(out *VolumeReplicationList) {
	*out = *vrl
	out.TypeMeta = vrl.TypeMeta
	vrl.ListMeta.DeepCopyInto(&out.ListMeta)
	if vrl.Items != nil {
		in, out := &vrl.Items, &out.Items
		*out = make([]VolumeReplication, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// Note: CephAdapter v1alpha1 implementation has been removed.
// Use CephV1Alpha2Adapter in ceph_v1alpha2.go for v1alpha2 API support.
