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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	replicationv1alpha2 "github.com/unified-replication/operator/api/v1alpha2"
)

// Dell DellCSIReplicationGroup CRD details
var DellCSIReplicationGroupGVKV1Alpha2 = schema.GroupVersionKind{
	Group:   "replication.dell.com",
	Version: "v1",
	Kind:    "DellCSIReplicationGroup",
}

// PowerStoreV1Alpha2Adapter implements VolumeReplicationAdapter for Dell PowerStore backend
// Translates kubernetes-csi-addons states to Dell actions
type PowerStoreV1Alpha2Adapter struct {
	client client.Client
}

// NewPowerStoreV1Alpha2Adapter creates a new Dell PowerStore adapter for v1alpha2
func NewPowerStoreV1Alpha2Adapter(client client.Client) *PowerStoreV1Alpha2Adapter {
	return &PowerStoreV1Alpha2Adapter{
		client: client,
	}
}

// ReconcileVolumeReplication reconciles a VolumeReplication for Dell PowerStore
func (a *PowerStoreV1Alpha2Adapter) ReconcileVolumeReplication(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
	vrc *replicationv1alpha2.VolumeReplicationClass,
) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("powerstore-adapter").WithValues(
		"volumereplication", vr.Name,
		"namespace", vr.Namespace)

	log.Info("Reconciling VolumeReplication with Dell PowerStore backend (with action translation)")

	// Translate state to Dell action
	dellAction := a.translateStateToDellAction(vr.Spec.ReplicationState)
	log.Info("Translated state to action", "vrState", vr.Spec.ReplicationState, "dellAction", dellAction)

	// Extract parameters
	protectionPolicy := vrc.Spec.Parameters["protectionPolicy"]
	if protectionPolicy == "" {
		return ctrl.Result{}, fmt.Errorf("protectionPolicy parameter required for Dell PowerStore")
	}

	remoteSystem := vrc.Spec.Parameters["remoteSystem"]
	if remoteSystem == "" {
		return ctrl.Result{}, fmt.Errorf("remoteSystem parameter required for Dell PowerStore")
	}

	rpo := vrc.Spec.Parameters["rpo"]
	if rpo == "" {
		rpo = "15m" // Default RPO
	}

	// Label PVC for Dell selector
	if err := a.labelPVCForReplication(ctx, vr.Spec.PvcName, vr.Namespace, vr.Name, log); err != nil {
		return ctrl.Result{}, err
	}

	// Create DellCSIReplicationGroup
	drg := &unstructured.Unstructured{}
	drg.SetGroupVersionKind(DellCSIReplicationGroupGVKV1Alpha2)
	drg.SetName(vr.Name)
	drg.SetNamespace(vr.Namespace)

	// Set owner reference
	if err := controllerutil.SetControllerReference(vr, drg, a.client.Scheme()); err != nil {
		log.Error(err, "Failed to set owner reference")
		return ctrl.Result{}, err
	}

	// Build spec
	spec := map[string]interface{}{
		"driverName":       "csi-powerstore.dellemc.com",
		"action":           dellAction, // Translated!
		"protectionPolicy": protectionPolicy,
		"remoteSystem":     remoteSystem,
		"remoteRPO":        rpo,
		"pvcSelector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"replication.storage.dell.com/group": vr.Name,
			},
		},
	}

	if err := unstructured.SetNestedMap(drg.Object, spec, "spec"); err != nil {
		log.Error(err, "Failed to build DellCSIReplicationGroup spec")
		return ctrl.Result{}, err
	}

	// Create or update
	if err := a.client.Patch(ctx, drg, client.Apply, client.FieldOwner("unified-replication-operator")); err != nil {
		log.Error(err, "Failed to create/update DellCSIReplicationGroup")
		return ctrl.Result{}, err
	}

	log.Info("Successfully created/updated DellCSIReplicationGroup with action translation")
	return ctrl.Result{}, nil
}

// DeleteVolumeReplication deletes the Dell backend resources
func (a *PowerStoreV1Alpha2Adapter) DeleteVolumeReplication(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
) error {
	log := log.FromContext(ctx).WithName("powerstore-adapter").WithValues(
		"volumereplication", vr.Name,
		"namespace", vr.Namespace)

	log.Info("Deleting DellCSIReplicationGroup and removing PVC labels")

	// Delete DellCSIReplicationGroup
	drg := &unstructured.Unstructured{}
	drg.SetGroupVersionKind(DellCSIReplicationGroupGVKV1Alpha2)
	drg.SetName(vr.Name)
	drg.SetNamespace(vr.Namespace)

	if err := a.client.Delete(ctx, drg); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("DellCSIReplicationGroup already deleted")
		} else {
			log.Error(err, "Failed to delete DellCSIReplicationGroup")
			return err
		}
	}

	// Remove labels from PVC
	if err := a.removePVCLabels(ctx, vr.Spec.PvcName, vr.Namespace, log); err != nil {
		log.Error(err, "Failed to remove PVC labels (non-fatal)")
		// Non-fatal - PVC might be deleted already
	}

	log.Info("Successfully deleted DellCSIReplicationGroup")
	return nil
}

// GetStatus fetches status from DellCSIReplicationGroup
func (a *PowerStoreV1Alpha2Adapter) GetStatus(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
) (*V1Alpha2ReplicationStatus, error) {
	log := log.FromContext(ctx).WithName("powerstore-adapter")

	// Fetch DellCSIReplicationGroup
	drg := &unstructured.Unstructured{}
	drg.SetGroupVersionKind(DellCSIReplicationGroupGVKV1Alpha2)

	if err := a.client.Get(ctx, client.ObjectKey{
		Name:      vr.Name,
		Namespace: vr.Namespace,
	}, drg); err != nil {
		log.Error(err, "Failed to get DellCSIReplicationGroup status")
		return nil, err
	}

	// Extract and translate status
	status := &V1Alpha2ReplicationStatus{}

	// Get Dell state/status and translate back
	if dellState, found, err := unstructured.NestedString(drg.Object, "status", "state"); found && err == nil {
		status.State = a.translateStateFromDell(dellState)
	}

	// Get message
	if message, found, err := unstructured.NestedString(drg.Object, "status", "message"); found && err == nil {
		status.Message = message
	}

	return status, nil
}

// translateStateToDellAction translates kubernetes-csi-addons state to Dell action
func (a *PowerStoreV1Alpha2Adapter) translateStateToDellAction(vrState string) string {
	switch vrState {
	case "primary":
		return "Failover" // Promote to primary (failover to this site)
	case "secondary":
		return "Sync" // Operate as secondary (sync from primary)
	case "resync":
		return "Reprotect" // Re-establish replication after failover
	default:
		return "Sync" // Default to Sync
	}
}

// translateStateFromDell translates Dell state back to kubernetes-csi-addons state
func (a *PowerStoreV1Alpha2Adapter) translateStateFromDell(dellState string) string {
	switch dellState {
	case "Synchronized", "Syncing":
		return "secondary" // Syncing → secondary
	case "FailedOver":
		return "primary" // Failed over → primary
	default:
		return "secondary" // Default
	}
}

// labelPVCForReplication adds Dell-specific labels to PVC
func (a *PowerStoreV1Alpha2Adapter) labelPVCForReplication(
	ctx context.Context,
	pvcName string,
	namespace string,
	groupName string,
	log logr.Logger,
) error {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := a.client.Get(ctx, types.NamespacedName{
		Name:      pvcName,
		Namespace: namespace,
	}, pvc); err != nil {
		log.Error(err, "Failed to get PVC", "pvc", pvcName)
		return err
	}

	// Add labels
	if pvc.Labels == nil {
		pvc.Labels = make(map[string]string)
	}
	pvc.Labels["replication.storage.dell.com/replicated"] = "true"
	pvc.Labels["replication.storage.dell.com/group"] = groupName

	if err := a.client.Update(ctx, pvc); err != nil {
		log.Error(err, "Failed to update PVC labels", "pvc", pvcName)
		return err
	}

	log.V(1).Info("Labeled PVC for Dell replication", "pvc", pvcName, "group", groupName)
	return nil
}

// removePVCLabels removes Dell-specific labels from PVC
func (a *PowerStoreV1Alpha2Adapter) removePVCLabels(
	ctx context.Context,
	pvcName string,
	namespace string,
	log logr.Logger,
) error {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := a.client.Get(ctx, types.NamespacedName{
		Name:      pvcName,
		Namespace: namespace,
	}, pvc); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil // PVC already deleted
		}
		return err
	}

	// Remove labels
	if pvc.Labels != nil {
		delete(pvc.Labels, "replication.storage.dell.com/replicated")
		delete(pvc.Labels, "replication.storage.dell.com/group")
	}

	if err := a.client.Update(ctx, pvc); err != nil {
		log.Error(err, "Failed to remove PVC labels", "pvc", pvcName)
		return err
	}

	log.V(1).Info("Removed labels from PVC", "pvc", pvcName)
	return nil
}

// ReconcileVolumeGroupReplication reconciles a volume group for Dell PowerStore
// Dell natively supports groups via PVCSelector
func (a *PowerStoreV1Alpha2Adapter) ReconcileVolumeGroupReplication(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
	vgrc *replicationv1alpha2.VolumeGroupReplicationClass,
	pvcs []corev1.PersistentVolumeClaim,
) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("powerstore-adapter").WithValues(
		"volumegroupreplication", vgr.Name,
		"namespace", vgr.Namespace,
		"pvcCount", len(pvcs))

	log.Info("Reconciling VolumeGroupReplication with Dell PowerStore backend (native selector)")

	// Translate state to action
	dellAction := a.translateStateToDellAction(vgr.Spec.ReplicationState)

	// Extract parameters
	protectionPolicy := vgrc.Spec.Parameters["protectionPolicy"]
	if protectionPolicy == "" {
		return ctrl.Result{}, fmt.Errorf("protectionPolicy parameter required for Dell PowerStore")
	}

	remoteSystem := vgrc.Spec.Parameters["remoteSystem"]
	if remoteSystem == "" {
		return ctrl.Result{}, fmt.Errorf("remoteSystem parameter required for Dell PowerStore")
	}

	// Label all PVCs in the group
	for _, pvc := range pvcs {
		if err := a.labelPVCForReplication(ctx, pvc.Name, vgr.Namespace, vgr.Name, log); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Create DellCSIReplicationGroup with PVCSelector
	drg := &unstructured.Unstructured{}
	drg.SetGroupVersionKind(DellCSIReplicationGroupGVKV1Alpha2)
	drg.SetName(vgr.Name)
	drg.SetNamespace(vgr.Namespace)

	// Set owner reference
	if err := controllerutil.SetControllerReference(vgr, drg, a.client.Scheme()); err != nil {
		log.Error(err, "Failed to set owner reference")
		return ctrl.Result{}, err
	}

	// Build spec with PVCSelector
	spec := map[string]interface{}{
		"driverName":       "csi-powerstore.dellemc.com",
		"action":           dellAction,
		"protectionPolicy": protectionPolicy,
		"remoteSystem":     remoteSystem,
		"remoteRPO":        vgrc.Spec.Parameters["rpo"],
		"pvcSelector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"replication.storage.dell.com/group": vgr.Name,
			},
		},
	}

	if err := unstructured.SetNestedMap(drg.Object, spec, "spec"); err != nil {
		log.Error(err, "Failed to build DellCSIReplicationGroup spec")
		return ctrl.Result{}, err
	}

	// Create or update
	if err := a.client.Patch(ctx, drg, client.Apply, client.FieldOwner("unified-replication-operator")); err != nil {
		log.Error(err, "Failed to create/update DellCSIReplicationGroup")
		return ctrl.Result{}, err
	}

	log.Info("Successfully created/updated DellCSIReplicationGroup for volume group", "pvcCount", len(pvcs))
	return ctrl.Result{}, nil
}

// DeleteVolumeGroupReplication deletes Dell backend resources for volume group
func (a *PowerStoreV1Alpha2Adapter) DeleteVolumeGroupReplication(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
) error {
	log := log.FromContext(ctx).WithName("powerstore-adapter").WithValues(
		"volumegroupreplication", vgr.Name,
		"namespace", vgr.Namespace)

	log.Info("Deleting DellCSIReplicationGroup for volume group")

	// Delete DellCSIReplicationGroup
	drg := &unstructured.Unstructured{}
	drg.SetGroupVersionKind(DellCSIReplicationGroupGVKV1Alpha2)
	drg.SetName(vgr.Name)
	drg.SetNamespace(vgr.Namespace)

	if err := a.client.Delete(ctx, drg); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("DellCSIReplicationGroup already deleted")
		} else {
			log.Error(err, "Failed to delete DellCSIReplicationGroup")
			return err
		}
	}

	// Remove labels from all PVCs in status
	for _, pvcRef := range vgr.Status.PersistentVolumeClaimsRefList {
		if err := a.removePVCLabels(ctx, pvcRef.Name, vgr.Namespace, log); err != nil {
			log.Error(err, "Failed to remove PVC labels (non-fatal)", "pvc", pvcRef.Name)
		}
	}

	log.Info("Successfully deleted DellCSIReplicationGroup")
	return nil
}

// GetGroupStatus fetches status from DellCSIReplicationGroup
func (a *PowerStoreV1Alpha2Adapter) GetGroupStatus(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
) (*V1Alpha2ReplicationStatus, error) {
	// TODO: Fetch and translate status from DellCSIReplicationGroup
	return &V1Alpha2ReplicationStatus{
		State:   vgr.Spec.ReplicationState,
		Message: "Group status pending",
	}, nil
}
