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

	log.Info("Reconciling VolumeReplication with Dell PowerStore backend")

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

	// Label PVC for Dell selector with role
	if err := a.labelPVCForReplication(ctx, vr.Spec.PvcName, vr.Namespace, vr.Name, log); err != nil {
		return ctrl.Result{}, err
	}

	// Get existing DellCSIReplicationGroup if it exists
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(DellCSIReplicationGroupGVKV1Alpha2)
	err := a.client.Get(ctx, client.ObjectKey{Name: vr.Name, Namespace: vr.Namespace}, existing)
	existingDRG := err == nil

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

	// Build spec WITHOUT action
	// Dell manages primary/secondary based on PVC read/write permissions
	// Action is only set when explicitly triggering failover or resync operations
	spec := map[string]interface{}{
		"driverName":       "csi-powerstore.dellemc.com",
		"protectionPolicy": protectionPolicy,
		"remoteSystem":     remoteSystem,
		"remoteRPO":        rpo,
		"pvcSelector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"replication.storage.dell.com/group": vr.Name,
			},
		},
	}

	// Determine if we need to set an action to trigger an explicit operation
	action := a.determineRequiredAction(vr, existingDRG, existing)
	if action != "" {
		log.Info("Triggering explicit Dell action", "action", action,
			"reason", "failover or resync operation requested")
		spec["action"] = action
	} else {
		log.Info("No action set - Dell manages replication via protection policy and PVC permissions",
			"desiredState", vr.Spec.ReplicationState)
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

	log.Info("Successfully reconciled DellCSIReplicationGroup")
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

// determineRequiredAction determines if a Dell action is needed to trigger an explicit operation
// Dell manages primary/secondary based on PVC read/write vs read-only permissions
// Actions are ONLY used to trigger explicit failover or resync operations, NOT for state management
func (a *PowerStoreV1Alpha2Adapter) determineRequiredAction(
	vr *replicationv1alpha2.VolumeReplication,
	existingDRG bool,
	existingObject *unstructured.Unstructured,
) string {
	desiredState := vr.Spec.ReplicationState
	currentState := vr.Status.State

	// Check if a previous action is still in progress
	if existingDRG && existingObject != nil {
		if previousAction, found, err := unstructured.NestedString(existingObject.Object, "spec", "action"); found && err == nil && previousAction != "" {
			// Action is still set - let it complete before setting a new one
			return ""
		}
	}

	// ONLY set action for explicit failover or resync operations
	// Do NOT set action for initial creation or steady state

	// Explicit resync requested (user wants to reprotect/resync the volume)
	if desiredState == "resync" {
		// Only trigger if not already in resync state
		if currentState != "resync" {
			return "Reprotect"
		}
		return ""
	}

	// Explicit failover: secondary -> primary transition
	// This is the ONLY case where we trigger a failover action
	if existingDRG && currentState == "secondary" && desiredState == "primary" {
		return "Failover"
	}

	// All other cases: no action
	// - Initial creation: Dell sets up replication based on protection policy
	// - Steady state: Dell maintains replication, primary/secondary determined by PVC permissions
	// - Primary -> secondary: Dell handles this via PVC permission changes, no action needed
	return ""
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

	// Extract parameters
	protectionPolicy := vgrc.Spec.Parameters["protectionPolicy"]
	if protectionPolicy == "" {
		return ctrl.Result{}, fmt.Errorf("protectionPolicy parameter required for Dell PowerStore")
	}

	remoteSystem := vgrc.Spec.Parameters["remoteSystem"]
	if remoteSystem == "" {
		return ctrl.Result{}, fmt.Errorf("remoteSystem parameter required for Dell PowerStore")
	}

	// Get existing DellCSIReplicationGroup if it exists
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(DellCSIReplicationGroupGVKV1Alpha2)
	err := a.client.Get(ctx, client.ObjectKey{Name: vgr.Name, Namespace: vgr.Namespace}, existing)
	existingDRG := err == nil

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

	// Build spec with PVCSelector WITHOUT action
	// Dell manages primary/secondary based on PVC read/write permissions
	spec := map[string]interface{}{
		"driverName":       "csi-powerstore.dellemc.com",
		"protectionPolicy": protectionPolicy,
		"remoteSystem":     remoteSystem,
		"remoteRPO":        vgrc.Spec.Parameters["rpo"],
		"pvcSelector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"replication.storage.dell.com/group": vgr.Name,
			},
		},
	}

	// Determine if we need to set an action to trigger an explicit operation
	// Create a pseudo VolumeReplication for action determination
	vrForAction := &replicationv1alpha2.VolumeReplication{
		Spec: replicationv1alpha2.VolumeReplicationSpec{
			ReplicationState: vgr.Spec.ReplicationState,
		},
		Status: replicationv1alpha2.VolumeReplicationStatus{
			State: vgr.Status.State,
		},
	}
	action := a.determineRequiredAction(vrForAction, existingDRG, existing)

	if action != "" {
		log.Info("Triggering explicit Dell action for volume group", "action", action,
			"reason", "failover or resync operation requested")
		spec["action"] = action
	} else {
		log.Info("No action set for volume group - Dell manages replication via protection policy and PVC permissions",
			"desiredState", vgr.Spec.ReplicationState)
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
