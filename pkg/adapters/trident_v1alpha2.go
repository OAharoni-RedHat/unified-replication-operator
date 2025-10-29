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

// Trident TridentMirrorRelationship CRD details
var TridentMirrorRelationshipGVKV1Alpha2 = schema.GroupVersionKind{
	Group:   "trident.netapp.io",
	Version: "v1",
	Kind:    "TridentMirrorRelationship",
}

// TridentV1Alpha2Adapter implements VolumeReplicationAdapter for Trident backend
// Translates kubernetes-csi-addons states to Trident states
type TridentV1Alpha2Adapter struct {
	client client.Client
}

// NewTridentV1Alpha2Adapter creates a new Trident adapter for v1alpha2
func NewTridentV1Alpha2Adapter(client client.Client) *TridentV1Alpha2Adapter {
	return &TridentV1Alpha2Adapter{
		client: client,
	}
}

// ReconcileVolumeReplication reconciles a VolumeReplication for Trident (with state translation)
func (a *TridentV1Alpha2Adapter) ReconcileVolumeReplication(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
	vrc *replicationv1alpha2.VolumeReplicationClass,
) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("trident-adapter").WithValues(
		"volumereplication", vr.Name,
		"namespace", vr.Namespace)

	log.Info("Reconciling VolumeReplication with Trident backend (with state translation)")

	// Translate state from kubernetes-csi-addons to Trident
	tridentState := a.translateStateToTrident(vr.Spec.ReplicationState)
	log.Info("Translated state", "vrState", vr.Spec.ReplicationState, "tridentState", tridentState)

	// Extract parameters from VolumeReplicationClass
	replicationPolicy := vrc.Spec.Parameters["replicationPolicy"]
	if replicationPolicy == "" {
		replicationPolicy = "Async" // Default to Async
	}

	replicationSchedule := vrc.Spec.Parameters["replicationSchedule"]
	if replicationSchedule == "" {
		replicationSchedule = "15m" // Default schedule
	}

	remoteVolume := vrc.Spec.Parameters["remoteVolume"]
	if remoteVolume == "" {
		remoteVolume = fmt.Sprintf("remote-%s", vr.Spec.PvcName) // Generate default
	}

	// Create TridentMirrorRelationship
	tmr := &unstructured.Unstructured{}
	tmr.SetGroupVersionKind(TridentMirrorRelationshipGVKV1Alpha2)
	tmr.SetName(vr.Name)
	tmr.SetNamespace(vr.Namespace)

	// Set owner reference
	if err := controllerutil.SetControllerReference(vr, tmr, a.client.Scheme()); err != nil {
		log.Error(err, "Failed to set owner reference")
		return ctrl.Result{}, err
	}

	// Build spec with translations
	spec := map[string]interface{}{
		"state":               tridentState, // Translated!
		"replicationPolicy":   replicationPolicy,
		"replicationSchedule": replicationSchedule,
		"volumeMappings": []interface{}{
			map[string]interface{}{
				"localPVCName":       vr.Spec.PvcName,
				"remoteVolumeHandle": remoteVolume,
			},
		},
	}

	if err := unstructured.SetNestedMap(tmr.Object, spec, "spec"); err != nil {
		log.Error(err, "Failed to build TridentMirrorRelationship spec")
		return ctrl.Result{}, err
	}

	// Create or update
	if err := a.client.Patch(ctx, tmr, client.Apply, client.FieldOwner("unified-replication-operator")); err != nil {
		log.Error(err, "Failed to create/update TridentMirrorRelationship")
		return ctrl.Result{}, err
	}

	log.Info("Successfully created/updated TridentMirrorRelationship with state translation")
	return ctrl.Result{}, nil
}

// DeleteVolumeReplication deletes the backend TridentMirrorRelationship
func (a *TridentV1Alpha2Adapter) DeleteVolumeReplication(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
) error {
	log := log.FromContext(ctx).WithName("trident-adapter").WithValues(
		"volumereplication", vr.Name,
		"namespace", vr.Namespace)

	log.Info("Deleting TridentMirrorRelationship")

	// Delete TridentMirrorRelationship
	tmr := &unstructured.Unstructured{}
	tmr.SetGroupVersionKind(TridentMirrorRelationshipGVKV1Alpha2)
	tmr.SetName(vr.Name)
	tmr.SetNamespace(vr.Namespace)

	if err := a.client.Delete(ctx, tmr); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("TridentMirrorRelationship already deleted")
			return nil
		}
		log.Error(err, "Failed to delete TridentMirrorRelationship")
		return err
	}

	log.Info("Successfully deleted TridentMirrorRelationship")
	return nil
}

// GetStatus fetches status from TridentMirrorRelationship
func (a *TridentV1Alpha2Adapter) GetStatus(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
) (*V1Alpha2ReplicationStatus, error) {
	log := log.FromContext(ctx).WithName("trident-adapter")

	// Fetch TridentMirrorRelationship
	tmr := &unstructured.Unstructured{}
	tmr.SetGroupVersionKind(TridentMirrorRelationshipGVKV1Alpha2)

	if err := a.client.Get(ctx, client.ObjectKey{
		Name:      vr.Name,
		Namespace: vr.Namespace,
	}, tmr); err != nil {
		log.Error(err, "Failed to get TridentMirrorRelationship status")
		return nil, err
	}

	// Extract and translate status
	status := &V1Alpha2ReplicationStatus{}

	// Get Trident state and translate back to kubernetes-csi-addons
	if tridentState, found, err := unstructured.NestedString(tmr.Object, "status", "state"); found && err == nil {
		status.State = a.translateStateFromTrident(tridentState)
	}

	// Get message
	if message, found, err := unstructured.NestedString(tmr.Object, "status", "message"); found && err == nil {
		status.Message = message
	}

	return status, nil
}

// translateStateToTrident translates kubernetes-csi-addons state to Trident state
func (a *TridentV1Alpha2Adapter) translateStateToTrident(vrState string) string {
	switch vrState {
	case "primary":
		return "established" // Trident primary state
	case "secondary":
		return "reestablished" // Trident secondary/replica state (note: reestablisheD with 'd')
	case "resync":
		return "reestablished" // Re-establish mirror
	default:
		return "established" // Default to established
	}
}

// translateStateFromTrident translates Trident state back to kubernetes-csi-addons state
func (a *TridentV1Alpha2Adapter) translateStateFromTrident(tridentState string) string {
	switch tridentState {
	case "established":
		return "primary" // Trident primary → kubernetes-csi-addons primary
	case "reestablished":
		return "secondary" // Trident secondary → kubernetes-csi-addons secondary (note: reestablisheD with 'd')
	case "promoted":
		return "primary" // Promoted volume is now primary
	default:
		return tridentState // Pass through unknown states
	}
}

// ReconcileVolumeGroupReplication reconciles a volume group for Trident
// Trident supports multiple volumes natively via volumeMappings array
func (a *TridentV1Alpha2Adapter) ReconcileVolumeGroupReplication(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
	vgrc *replicationv1alpha2.VolumeGroupReplicationClass,
	pvcs []corev1.PersistentVolumeClaim,
) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("trident-adapter").WithValues(
		"volumegroupreplication", vgr.Name,
		"namespace", vgr.Namespace,
		"pvcCount", len(pvcs))

	log.Info("Reconciling VolumeGroupReplication with Trident backend (native array support)")

	// Translate state
	tridentState := a.translateStateToTrident(vgr.Spec.ReplicationState)

	// Extract parameters
	replicationPolicy := vgrc.Spec.Parameters["replicationPolicy"]
	if replicationPolicy == "" {
		replicationPolicy = "Async"
	}

	// Build volumeMappings for all PVCs
	volumeMappings := make([]interface{}, len(pvcs))
	for i, pvc := range pvcs {
		remoteVolume := vgrc.Spec.Parameters[fmt.Sprintf("remoteVolume-%s", pvc.Name)]
		if remoteVolume == "" {
			remoteVolume = fmt.Sprintf("remote-%s", pvc.Name)
		}

		volumeMappings[i] = map[string]interface{}{
			"localPVCName":       pvc.Name,
			"remoteVolumeHandle": remoteVolume,
		}
	}

	// Create TridentMirrorRelationship with multiple volumes
	tmr := &unstructured.Unstructured{}
	tmr.SetGroupVersionKind(TridentMirrorRelationshipGVKV1Alpha2)
	tmr.SetName(vgr.Name)
	tmr.SetNamespace(vgr.Namespace)

	// Set owner reference
	if err := controllerutil.SetControllerReference(vgr, tmr, a.client.Scheme()); err != nil {
		log.Error(err, "Failed to set owner reference")
		return ctrl.Result{}, err
	}

	// Build spec
	spec := map[string]interface{}{
		"state":               tridentState,
		"replicationPolicy":   replicationPolicy,
		"replicationSchedule": vgrc.Spec.Parameters["groupReplicationSchedule"],
		"volumeMappings":      volumeMappings, // Multiple volumes!
	}

	if err := unstructured.SetNestedMap(tmr.Object, spec, "spec"); err != nil {
		log.Error(err, "Failed to build TridentMirrorRelationship spec")
		return ctrl.Result{}, err
	}

	// Create or update
	if err := a.client.Patch(ctx, tmr, client.Apply, client.FieldOwner("unified-replication-operator")); err != nil {
		log.Error(err, "Failed to create/update TridentMirrorRelationship")
		return ctrl.Result{}, err
	}

	log.Info("Successfully created/updated TridentMirrorRelationship for volume group", "volumeCount", len(pvcs))
	return ctrl.Result{}, nil
}

// DeleteVolumeGroupReplication deletes the TridentMirrorRelationship for the group
func (a *TridentV1Alpha2Adapter) DeleteVolumeGroupReplication(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
) error {
	log := log.FromContext(ctx).WithName("trident-adapter").WithValues(
		"volumegroupreplication", vgr.Name,
		"namespace", vgr.Namespace)

	log.Info("Deleting TridentMirrorRelationship for volume group")

	// Delete TridentMirrorRelationship
	tmr := &unstructured.Unstructured{}
	tmr.SetGroupVersionKind(TridentMirrorRelationshipGVKV1Alpha2)
	tmr.SetName(vgr.Name)
	tmr.SetNamespace(vgr.Namespace)

	if err := a.client.Delete(ctx, tmr); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Info("TridentMirrorRelationship already deleted")
			return nil
		}
		log.Error(err, "Failed to delete TridentMirrorRelationship")
		return err
	}

	log.Info("Successfully deleted TridentMirrorRelationship")
	return nil
}

// GetGroupStatus fetches group status from TridentMirrorRelationship
func (a *TridentV1Alpha2Adapter) GetGroupStatus(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
) (*V1Alpha2ReplicationStatus, error) {
	// TODO: Fetch and translate status from TridentMirrorRelationship
	return &V1Alpha2ReplicationStatus{
		State:   vgr.Spec.ReplicationState,
		Message: "Group status pending",
	}, nil
}
