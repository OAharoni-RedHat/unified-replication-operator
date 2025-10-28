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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	replicationv1alpha2 "github.com/unified-replication/operator/api/v1alpha2"
)

// Ceph VolumeReplication CRD details
var CephVolumeReplicationGVK = schema.GroupVersionKind{
	Group:   "replication.storage.openshift.io",
	Version: "v1alpha1",
	Kind:    "VolumeReplication",
}

// CephV1Alpha2Adapter implements VolumeReplicationAdapter for Ceph backend
// Since Ceph uses kubernetes-csi-addons natively, this is mostly passthrough
type CephV1Alpha2Adapter struct {
	client client.Client
}

// NewCephV1Alpha2Adapter creates a new Ceph adapter for v1alpha2
func NewCephV1Alpha2Adapter(client client.Client) *CephV1Alpha2Adapter {
	return &CephV1Alpha2Adapter{
		client: client,
	}
}

// ReconcileVolumeReplication reconciles a VolumeReplication for Ceph (Passthrough)
func (a *CephV1Alpha2Adapter) ReconcileVolumeReplication(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
	vrc *replicationv1alpha2.VolumeReplicationClass,
) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("ceph-adapter").WithValues(
		"volumereplication", vr.Name,
		"namespace", vr.Namespace)

	log.Info("Reconciling VolumeReplication with Ceph backend (passthrough)")

	// Ceph adapter is mostly passthrough since Ceph uses kubernetes-csi-addons natively.
	// We create a backend VolumeReplication CR in replication.storage.openshift.io/v1alpha1
	// and mirror its status back to our CR.

	// Create backend Ceph VolumeReplication CR
	cephVR := &unstructured.Unstructured{}
	cephVR.SetGroupVersionKind(CephVolumeReplicationGVK)
	cephVR.SetName(vr.Name)
	cephVR.SetNamespace(vr.Namespace)

	// Set owner reference
	if err := controllerutil.SetControllerReference(vr, cephVR, a.client.Scheme()); err != nil {
		log.Error(err, "Failed to set owner reference")
		return ctrl.Result{}, err
	}

	// Build spec - direct mapping (no translation needed!)
	spec := map[string]interface{}{
		"volumeReplicationClass": vr.Spec.VolumeReplicationClass,
		"pvcName":                vr.Spec.PvcName,
		"replicationState":       vr.Spec.ReplicationState, // No translation!
	}

	if vr.Spec.AutoResync != nil {
		spec["autoResync"] = *vr.Spec.AutoResync
	}

	if vr.Spec.DataSource != nil {
		spec["dataSource"] = map[string]interface{}{
			"apiGroup": vr.Spec.DataSource.APIGroup,
			"kind":     vr.Spec.DataSource.Kind,
			"name":     vr.Spec.DataSource.Name,
		}
	}

	if err := unstructured.SetNestedMap(cephVR.Object, spec, "spec"); err != nil {
		log.Error(err, "Failed to build Ceph VolumeReplication spec")
		return ctrl.Result{}, err
	}

	// Create or update the backend Ceph VolumeReplication
	if err := a.client.Patch(ctx, cephVR, client.Apply, client.FieldOwner("unified-replication-operator")); err != nil {
		log.Error(err, "Failed to create/update Ceph VolumeReplication")
		return ctrl.Result{}, err
	}

	log.Info("Successfully created/updated Ceph VolumeReplication (passthrough)")

	// TODO: Sync status from Ceph VolumeReplication back to our VolumeReplication
	// This will read the Ceph VR status and copy conditions, state, lastSyncTime, etc.

	return ctrl.Result{}, nil
}

// DeleteVolumeReplication deletes the backend Ceph VolumeReplication
func (a *CephV1Alpha2Adapter) DeleteVolumeReplication(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
) error {
	log := log.FromContext(ctx).WithName("ceph-adapter").WithValues(
		"volumereplication", vr.Name,
		"namespace", vr.Namespace)

	log.Info("Deleting Ceph VolumeReplication")

	// Delete backend Ceph VolumeReplication CR
	cephVR := &unstructured.Unstructured{}
	cephVR.SetGroupVersionKind(CephVolumeReplicationGVK)
	cephVR.SetName(vr.Name)
	cephVR.SetNamespace(vr.Namespace)

	if err := a.client.Delete(ctx, cephVR); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("Ceph VolumeReplication already deleted")
			return nil
		}
		log.Error(err, "Failed to delete Ceph VolumeReplication")
		return err
	}

	log.Info("Successfully deleted Ceph VolumeReplication")
	return nil
}

// GetStatus fetches status from backend Ceph VolumeReplication
func (a *CephV1Alpha2Adapter) GetStatus(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
) (*V1Alpha2ReplicationStatus, error) {
	log := log.FromContext(ctx).WithName("ceph-adapter")

	// Fetch backend Ceph VolumeReplication
	cephVR := &unstructured.Unstructured{}
	cephVR.SetGroupVersionKind(CephVolumeReplicationGVK)

	if err := a.client.Get(ctx, client.ObjectKey{
		Name:      vr.Name,
		Namespace: vr.Namespace,
	}, cephVR); err != nil {
		log.Error(err, "Failed to get Ceph VolumeReplication status")
		return nil, err
	}

	// Extract status fields
	status := &V1Alpha2ReplicationStatus{}

	// Get state
	if state, found, err := unstructured.NestedString(cephVR.Object, "status", "state"); found && err == nil {
		status.State = state
	}

	// Get message
	if message, found, err := unstructured.NestedString(cephVR.Object, "status", "message"); found && err == nil {
		status.Message = message
	}

	// TODO: Parse lastSyncTime, lastSyncDuration, conditions from Ceph VR status

	return status, nil
}

// ReconcileVolumeGroupReplication reconciles a volume group for Ceph
// For Ceph, we create individual VolumeReplications for each PVC but coordinate them
func (a *CephV1Alpha2Adapter) ReconcileVolumeGroupReplication(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
	vgrc *replicationv1alpha2.VolumeGroupReplicationClass,
	pvcs []corev1.PersistentVolumeClaim,
) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("ceph-adapter").WithValues(
		"volumegroupreplication", vgr.Name,
		"namespace", vgr.Namespace,
		"pvcCount", len(pvcs))

	log.Info("Reconciling VolumeGroupReplication with Ceph backend (coordinated VRs)")

	// For Ceph, create one VolumeReplication per PVC in the group
	// All VRs are owned by the VolumeGroupReplication for coordinated management
	for _, pvc := range pvcs {
		vrName := fmt.Sprintf("%s-%s", vgr.Name, pvc.Name)

		cephVR := &unstructured.Unstructured{}
		cephVR.SetGroupVersionKind(CephVolumeReplicationGVK)
		cephVR.SetName(vrName)
		cephVR.SetNamespace(vgr.Namespace)

		// Set owner reference to VolumeGroupReplication
		if err := controllerutil.SetControllerReference(vgr, cephVR, a.client.Scheme()); err != nil {
			log.Error(err, "Failed to set owner reference", "pvc", pvc.Name)
			return ctrl.Result{}, err
		}

		// Add label to track group membership
		labels := map[string]string{
			"volumeGroupReplication": vgr.Name,
		}
		cephVR.SetLabels(labels)

		// Build spec
		spec := map[string]interface{}{
			"volumeReplicationClass": vgr.Spec.VolumeGroupReplicationClass,
			"pvcName":                pvc.Name,
			"replicationState":       vgr.Spec.ReplicationState, // Same state for all
		}

		if vgr.Spec.AutoResync != nil {
			spec["autoResync"] = *vgr.Spec.AutoResync
		}

		if err := unstructured.SetNestedMap(cephVR.Object, spec, "spec"); err != nil {
			log.Error(err, "Failed to build Ceph VolumeReplication spec", "pvc", pvc.Name)
			return ctrl.Result{}, err
		}

		// Create or update
		if err := a.client.Patch(ctx, cephVR, client.Apply, client.FieldOwner("unified-replication-operator")); err != nil {
			log.Error(err, "Failed to create/update Ceph VolumeReplication for PVC", "pvc", pvc.Name)
			return ctrl.Result{}, err
		}

		log.V(1).Info("Created/updated Ceph VolumeReplication for PVC", "pvc", pvc.Name, "vrName", vrName)
	}

	log.Info("Successfully created/updated Ceph VolumeReplications for volume group", "count", len(pvcs))
	return ctrl.Result{}, nil
}

// DeleteVolumeGroupReplication deletes all backend Ceph VolumeReplications for the group
func (a *CephV1Alpha2Adapter) DeleteVolumeGroupReplication(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
) error {
	log := log.FromContext(ctx).WithName("ceph-adapter").WithValues(
		"volumegroupreplication", vgr.Name,
		"namespace", vgr.Namespace)

	log.Info("Deleting Ceph VolumeReplications for volume group")

	// List all Ceph VolumeReplications with our group label
	cephVRList := &unstructured.UnstructuredList{}
	cephVRList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "replication.storage.openshift.io",
		Version: "v1alpha1",
		Kind:    "VolumeReplicationList",
	})

	if err := a.client.List(ctx, cephVRList,
		client.InNamespace(vgr.Namespace),
		client.MatchingLabels{"volumeGroupReplication": vgr.Name}); err != nil {
		log.Error(err, "Failed to list Ceph VolumeReplications")
		return err
	}

	// Delete each one
	for _, item := range cephVRList.Items {
		if err := a.client.Delete(ctx, &item); err != nil {
			if client.IgnoreNotFound(err) == nil {
				continue
			}
			log.Error(err, "Failed to delete Ceph VolumeReplication", "name", item.GetName())
			return err
		}
		log.V(1).Info("Deleted Ceph VolumeReplication", "name", item.GetName())
	}

	log.Info("Successfully deleted all Ceph VolumeReplications for group", "count", len(cephVRList.Items))
	return nil
}

// GetGroupStatus fetches group status from all Ceph VolumeReplications
func (a *CephV1Alpha2Adapter) GetGroupStatus(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
) (*V1Alpha2ReplicationStatus, error) {
	// TODO: Aggregate status from all Ceph VolumeReplications in the group
	return &V1Alpha2ReplicationStatus{
		State:   vgr.Spec.ReplicationState,
		Message: "Group status aggregation pending",
	}, nil
}
