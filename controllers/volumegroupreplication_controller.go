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
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	replicationv1alpha2 "github.com/unified-replication/operator/api/v1alpha2"
	"github.com/unified-replication/operator/pkg/adapters"
	"github.com/unified-replication/operator/pkg/translation"
)

const (
	volumeGroupReplicationFinalizer = "replication.unified.io/volumegroupreplication-finalizer"
)

// VolumeGroupReplicationReconciler reconciles a VolumeGroupReplication object
type VolumeGroupReplicationReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	AdapterRegistry adapters.Registry
}

//+kubebuilder:rbac:groups=replication.unified.io,resources=volumegroupreplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=replication.unified.io,resources=volumegroupreplications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=replication.unified.io,resources=volumegroupreplications/finalizers,verbs=update
//+kubebuilder:rbac:groups=replication.unified.io,resources=volumegroupreplicationclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *VolumeGroupReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("volumegroupreplication-controller")

	// 1. Fetch VolumeGroupReplication resource
	vgr := &replicationv1alpha2.VolumeGroupReplication{}
	if err := r.Get(ctx, req.NamespacedName, vgr); err != nil {
		if errors.IsNotFound(err) {
			log.Info("VolumeGroupReplication resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get VolumeGroupReplication")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling VolumeGroupReplication",
		"name", vgr.Name,
		"namespace", vgr.Namespace,
		"state", vgr.Spec.ReplicationState)

	// 2. Handle deletion
	if !vgr.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, vgr, log)
	}

	// 3. Add finalizer if missing
	if !controllerutil.ContainsFinalizer(vgr, volumeGroupReplicationFinalizer) {
		controllerutil.AddFinalizer(vgr, volumeGroupReplicationFinalizer)
		if err := r.Update(ctx, vgr); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to VolumeGroupReplication")
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Fetch VolumeGroupReplicationClass
	vgrc, err := r.fetchVolumeGroupReplicationClass(ctx, vgr.Spec.VolumeGroupReplicationClass, log)
	if err != nil {
		return r.handleError(ctx, vgr, err, log)
	}

	// 5. Find PVCs matching selector
	pvcList, err := r.findMatchingPVCs(ctx, vgr, log)
	if err != nil {
		return r.handleError(ctx, vgr, err, log)
	}

	if len(pvcList.Items) == 0 {
		err := fmt.Errorf("no PVCs match selector in namespace %s", vgr.Namespace)
		return r.handleError(ctx, vgr, err, log)
	}

	log.Info("Found matching PVCs", "count", len(pvcList.Items))

	// 6. Detect backend from provisioner
	backend, err := r.detectBackend(vgrc.Spec.Provisioner, log)
	if err != nil {
		return r.handleError(ctx, vgr, err, log)
	}

	log.Info("Detected backend for group", "backend", backend, "provisioner", vgrc.Spec.Provisioner)

	// 7. Get adapter and reconcile
	adapter := r.AdapterRegistry.GetVolumeGroupReplicationAdapter(backend)
	if adapter == nil {
		err := fmt.Errorf("no volume group adapter available for backend: %s", backend)
		log.Error(err, "Adapter not found")
		return r.handleError(ctx, vgr, err, log)
	}

	// 8. Reconcile with adapter
	result, err := adapter.ReconcileVolumeGroupReplication(ctx, vgr, vgrc, pvcList.Items)
	if err != nil {
		log.Error(err, "Adapter reconciliation failed")
		return r.handleError(ctx, vgr, err, log)
	}

	// 9. Update PVC list
	vgr.Status.PersistentVolumeClaimsRefList = make([]corev1.LocalObjectReference, len(pvcList.Items))
	for i, pvc := range pvcList.Items {
		vgr.Status.PersistentVolumeClaimsRefList[i] = corev1.LocalObjectReference{
			Name: pvc.Name,
		}
	}

	// 10. Fetch backend status and update VolumeGroupReplication status
	backendStatus, err := adapter.GetGroupStatus(ctx, vgr)
	if err != nil {
		// Log warning but don't fail reconciliation - use partial status
		log.V(1).Info("Failed to fetch backend group status, using partial status", "error", err)
		// Still update Ready condition to indicate reconciliation succeeded
		if err := r.updateStatusFromBackend(ctx, vgr, nil, "Ready", metav1.ConditionTrue, "ReconcileComplete",
			fmt.Sprintf("Group replication configured for %d volumes, but status fetch failed", len(pvcList.Items)), log); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// Update status with backend status
		if err := r.updateStatusFromBackend(ctx, vgr, backendStatus, "Ready", metav1.ConditionTrue, "ReconcileComplete",
			fmt.Sprintf("Group replication configured for %d volumes", len(pvcList.Items)), log); err != nil {
			return ctrl.Result{}, err
		}
	}

	return result, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *VolumeGroupReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Basic setup - watches for VolumeGroupReplicationClass and PVC changes can be added later
	// TODO: Add watches for VolumeGroupReplicationClass changes (requeue dependent VGRs)
	// TODO: Add watches for PVC label changes (requeue VGRs with matching selectors)
	return ctrl.NewControllerManagedBy(mgr).
		For(&replicationv1alpha2.VolumeGroupReplication{}).
		Complete(r)
}

// Helper methods

func (r *VolumeGroupReplicationReconciler) handleDeletion(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Info("Handling deletion - finalizer cleanup", "finalizer", volumeGroupReplicationFinalizer)

	if controllerutil.ContainsFinalizer(vgr, volumeGroupReplicationFinalizer) {
		// Fetch class to detect backend
		vgrc, err := r.fetchVolumeGroupReplicationClass(ctx, vgr.Spec.VolumeGroupReplicationClass, log)
		if err != nil {
			// Class might be deleted already, log and continue
			log.Info("VolumeGroupReplicationClass not found during deletion (may be deleted), continuing",
				"class", vgr.Spec.VolumeGroupReplicationClass)
		} else {
			// Detect backend and call adapter deletion
			backend, err := r.detectBackend(vgrc.Spec.Provisioner, log)
			if err != nil {
				log.Error(err, "Failed to detect backend during deletion, continuing anyway")
			} else {
				adapter := r.AdapterRegistry.GetVolumeGroupReplicationAdapter(backend)
				if adapter != nil {
					if err := adapter.DeleteVolumeGroupReplication(ctx, vgr); err != nil {
						log.Error(err, "Failed to delete backend resources")
						return ctrl.Result{}, err
					}
					log.Info("Successfully deleted backend resources for group", "backend", backend)
				}
			}
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(vgr, volumeGroupReplicationFinalizer)
		if err := r.Update(ctx, vgr); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Finalizer removed, resource will be deleted")
	}

	return ctrl.Result{}, nil
}

func (r *VolumeGroupReplicationReconciler) handleError(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
	err error,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Error(err, "Error during reconciliation")

	// Update status condition to indicate error
	_ = r.updateStatus(ctx, vgr, "Ready", metav1.ConditionFalse, "ReconcileError",
		err.Error(), log)

	// Return error for requeue
	return ctrl.Result{}, err
}

// updateStatus is a legacy method for error cases - use updateStatusFromBackend for normal flow
func (r *VolumeGroupReplicationReconciler) updateStatus(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
	log logr.Logger,
) error {
	return r.updateStatusFromBackend(ctx, vgr, nil, conditionType, status, reason, message, log)
}

// updateStatusFromBackend updates VolumeGroupReplication status using backend status
func (r *VolumeGroupReplicationReconciler) updateStatusFromBackend(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
	backendStatus *adapters.V1Alpha2ReplicationStatus,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
	log logr.Logger,
) error {
	// Update controller-managed Ready condition
	meta.SetStatusCondition(&vgr.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: vgr.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})

	// Update observedGeneration
	vgr.Status.ObservedGeneration = vgr.Generation

	// Update status fields from backend if available
	if backendStatus != nil {
		// Use backend state (not spec state)
		if backendStatus.State != "" {
			vgr.Status.State = backendStatus.State
		}

		// Update message from backend
		if backendStatus.Message != "" {
			vgr.Status.Message = backendStatus.Message
		} else if message != "" {
			vgr.Status.Message = message
		}

		// Update lastSyncTime from backend
		if backendStatus.LastSyncTime != nil {
			vgr.Status.LastSyncTime = backendStatus.LastSyncTime
		}

		// Update lastSyncDuration from backend
		if backendStatus.LastSyncDuration != nil {
			vgr.Status.LastSyncDuration = backendStatus.LastSyncDuration
		}

		// Merge backend conditions with controller-managed conditions
		// Backend conditions (Syncing, Degraded) take precedence, but Ready is controller-managed
		for _, backendCond := range backendStatus.Conditions {
			// Skip Ready condition - controller manages it
			if backendCond.Type == "Ready" {
				continue
			}
			// Merge other conditions (Syncing, Degraded, etc.)
			meta.SetStatusCondition(&vgr.Status.Conditions, backendCond)
		}
	} else {
		// No backend status - fallback to spec state
		vgr.Status.State = vgr.Spec.ReplicationState
		if message != "" {
			vgr.Status.Message = message
		}
	}

	// Save status
	if err := r.Status().Update(ctx, vgr); err != nil {
		log.Error(err, "Failed to update VolumeGroupReplication status")
		return err
	}

	log.V(1).Info("Updated VolumeGroupReplication status", "condition", conditionType, "status", status, "backendState", vgr.Status.State)
	return nil
}

func (r *VolumeGroupReplicationReconciler) fetchVolumeGroupReplicationClass(
	ctx context.Context,
	className string,
	log logr.Logger,
) (*replicationv1alpha2.VolumeGroupReplicationClass, error) {
	log.V(1).Info("Fetching VolumeGroupReplicationClass", "class", className)

	// Fetch VolumeGroupReplicationClass (cluster-scoped)
	vgrc := &replicationv1alpha2.VolumeGroupReplicationClass{}
	if err := r.Get(ctx, types.NamespacedName{Name: className}, vgrc); err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "VolumeGroupReplicationClass not found", "class", className)
			return nil, fmt.Errorf("VolumeGroupReplicationClass %q not found", className)
		}
		log.Error(err, "Failed to fetch VolumeGroupReplicationClass", "class", className)
		return nil, err
	}

	// Validate required fields
	if vgrc.Spec.Provisioner == "" {
		err := fmt.Errorf("VolumeGroupReplicationClass %q has empty provisioner", className)
		log.Error(err, "Invalid VolumeGroupReplicationClass")
		return nil, err
	}

	log.Info("Successfully fetched VolumeGroupReplicationClass",
		"class", className,
		"provisioner", vgrc.Spec.Provisioner)

	return vgrc, nil
}

func (r *VolumeGroupReplicationReconciler) findMatchingPVCs(
	ctx context.Context,
	vgr *replicationv1alpha2.VolumeGroupReplication,
	log logr.Logger,
) (*corev1.PersistentVolumeClaimList, error) {
	log.V(1).Info("Finding PVCs matching selector", "selector", vgr.Spec.Selector)

	// Create label selector from spec
	selector, err := metav1.LabelSelectorAsSelector(vgr.Spec.Selector)
	if err != nil {
		log.Error(err, "Invalid label selector")
		return nil, fmt.Errorf("invalid selector: %w", err)
	}

	// List PVCs in the same namespace matching the selector
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := r.List(ctx, pvcList,
		client.InNamespace(vgr.Namespace),
		client.MatchingLabelsSelector{Selector: selector}); err != nil {
		log.Error(err, "Failed to list PVCs")
		return nil, err
	}

	pvcNames := make([]string, len(pvcList.Items))
	for i, pvc := range pvcList.Items {
		pvcNames[i] = pvc.Name
	}

	log.Info("Found matching PVCs", "count", len(pvcList.Items), "pvcs", pvcNames)

	return pvcList, nil
}

func (r *VolumeGroupReplicationReconciler) detectBackend(
	provisioner string,
	log logr.Logger,
) (translation.Backend, error) {
	log.V(1).Info("Detecting backend from provisioner", "provisioner", provisioner)

	prov := strings.ToLower(provisioner)

	// Ceph backend detection
	if strings.Contains(prov, "ceph") ||
		strings.Contains(prov, "rbd.csi.ceph.com") ||
		strings.Contains(prov, "cephfs.csi.ceph.com") {
		log.Info("Detected Ceph backend", "provisioner", provisioner)
		return translation.BackendCeph, nil
	}

	// Trident backend detection
	if strings.Contains(prov, "trident") ||
		strings.Contains(prov, "csi.trident.netapp.io") ||
		strings.Contains(prov, "netapp") {
		log.Info("Detected Trident backend", "provisioner", provisioner)
		return translation.BackendTrident, nil
	}

	// Dell PowerStore backend detection
	if strings.Contains(prov, "powerstore") ||
		strings.Contains(prov, "dellemc") ||
		strings.Contains(prov, "csi-powerstore.dellemc.com") {
		log.Info("Detected Dell PowerStore backend", "provisioner", provisioner)
		return translation.BackendPowerStore, nil
	}

	// Unknown backend
	err := fmt.Errorf("unable to detect backend from provisioner: %s", provisioner)
	log.Error(err, "Unknown provisioner")
	return "", err
}

// TODO: Enhanced watch configuration
// The following methods can be used to set up watches in SetupWithManager:
//
// findGroupsForClass maps VolumeGroupReplicationClass changes to VolumeGroupReplications
// findGroupsForPVC maps PVC changes to VolumeGroupReplications that might select them
//
// These will be implemented when advanced watch configuration is needed.
