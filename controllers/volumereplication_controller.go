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
	volumeReplicationFinalizer = "replication.unified.io/volumereplication-finalizer"
)

// VolumeReplicationReconciler reconciles a VolumeReplication object
type VolumeReplicationReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	AdapterRegistry adapters.Registry
}

//+kubebuilder:rbac:groups=replication.unified.io,resources=volumereplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=replication.unified.io,resources=volumereplications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=replication.unified.io,resources=volumereplications/finalizers,verbs=update
//+kubebuilder:rbac:groups=replication.unified.io,resources=volumereplicationclasses,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *VolumeReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("volumereplication-controller")

	// 1. Fetch VolumeReplication resource
	vr := &replicationv1alpha2.VolumeReplication{}
	if err := r.Get(ctx, req.NamespacedName, vr); err != nil {
		if errors.IsNotFound(err) {
			log.Info("VolumeReplication resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get VolumeReplication")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling VolumeReplication",
		"name", vr.Name,
		"namespace", vr.Namespace,
		"state", vr.Spec.ReplicationState)

	// 2. Handle deletion
	if !vr.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, vr, log)
	}

	// 3. Add finalizer if missing
	if !controllerutil.ContainsFinalizer(vr, volumeReplicationFinalizer) {
		controllerutil.AddFinalizer(vr, volumeReplicationFinalizer)
		if err := r.Update(ctx, vr); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		log.Info("Added finalizer to VolumeReplication")
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. Fetch VolumeReplicationClass
	vrc, err := r.fetchVolumeReplicationClass(ctx, vr.Spec.VolumeReplicationClass, log)
	if err != nil {
		return r.handleError(ctx, vr, err, log)
	}

	// 5. Detect backend from provisioner
	backend, err := r.detectBackend(vrc.Spec.Provisioner, log)
	if err != nil {
		return r.handleError(ctx, vr, err, log)
	}

	log.Info("Detected backend", "backend", backend, "provisioner", vrc.Spec.Provisioner)

	// 6. Get adapter from registry
	adapter := r.AdapterRegistry.GetVolumeReplicationAdapter(backend)
	if adapter == nil {
		err := fmt.Errorf("no adapter available for backend: %s", backend)
		log.Error(err, "Adapter not found")
		return r.handleError(ctx, vr, err, log)
	}

	// 7. Reconcile with adapter
	result, err := adapter.ReconcileVolumeReplication(ctx, vr, vrc)
	if err != nil {
		log.Error(err, "Adapter reconciliation failed")
		return r.handleError(ctx, vr, err, log)
	}

	// 8. Update status to Ready
	if err := r.updateStatus(ctx, vr, "Ready", metav1.ConditionTrue, "ReconcileComplete",
		"Replication configured successfully", log); err != nil {
		return ctrl.Result{}, err
	}

	return result, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *VolumeReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&replicationv1alpha2.VolumeReplication{}).
		Owns(&replicationv1alpha2.VolumeReplication{}).
		Complete(r)
}

// Helper methods (to be implemented in subsequent prompts)

func (r *VolumeReplicationReconciler) handleDeletion(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Info("Handling deletion - finalizer cleanup", "finalizer", volumeReplicationFinalizer)

	if controllerutil.ContainsFinalizer(vr, volumeReplicationFinalizer) {
		// Fetch class to detect backend
		vrc, err := r.fetchVolumeReplicationClass(ctx, vr.Spec.VolumeReplicationClass, log)
		if err != nil {
			// Class might be deleted already, log and continue
			log.Info("VolumeReplicationClass not found during deletion (may be deleted), continuing", "class", vr.Spec.VolumeReplicationClass)
		} else {
			// Detect backend and call adapter deletion
			backend, err := r.detectBackend(vrc.Spec.Provisioner, log)
			if err != nil {
				log.Error(err, "Failed to detect backend during deletion, continuing anyway")
			} else {
				adapter := r.AdapterRegistry.GetVolumeReplicationAdapter(backend)
				if adapter != nil {
					if err := adapter.DeleteVolumeReplication(ctx, vr); err != nil {
						log.Error(err, "Failed to delete backend resources")
						return ctrl.Result{}, err
					}
					log.Info("Successfully deleted backend resources", "backend", backend)
				}
			}
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(vr, volumeReplicationFinalizer)
		if err := r.Update(ctx, vr); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("Finalizer removed, resource will be deleted")
	}

	return ctrl.Result{}, nil
}

func (r *VolumeReplicationReconciler) handleError(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
	err error,
	log logr.Logger,
) (ctrl.Result, error) {
	log.Error(err, "Error during reconciliation")

	// Update status condition to indicate error
	_ = r.updateStatus(ctx, vr, "Ready", metav1.ConditionFalse, "ReconcileError",
		err.Error(), log)

	// Return error for requeue
	return ctrl.Result{}, err
}

func (r *VolumeReplicationReconciler) updateStatus(
	ctx context.Context,
	vr *replicationv1alpha2.VolumeReplication,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
	log logr.Logger,
) error {
	// Update condition
	meta.SetStatusCondition(&vr.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: vr.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	})

	// Update observedGeneration
	vr.Status.ObservedGeneration = vr.Generation

	// Update state to match spec
	vr.Status.State = vr.Spec.ReplicationState

	// Save status
	if err := r.Status().Update(ctx, vr); err != nil {
		log.Error(err, "Failed to update VolumeReplication status")
		return err
	}

	log.V(1).Info("Updated VolumeReplication status", "condition", conditionType, "status", status)
	return nil
}

// fetchVolumeReplicationClass fetches and validates a VolumeReplicationClass (Prompt 3.3)
func (r *VolumeReplicationReconciler) fetchVolumeReplicationClass(
	ctx context.Context,
	className string,
	log logr.Logger,
) (*replicationv1alpha2.VolumeReplicationClass, error) {
	log.V(1).Info("Fetching VolumeReplicationClass", "class", className)

	// Fetch VolumeReplicationClass (cluster-scoped)
	vrc := &replicationv1alpha2.VolumeReplicationClass{}
	if err := r.Get(ctx, types.NamespacedName{Name: className}, vrc); err != nil {
		if errors.IsNotFound(err) {
			log.Error(err, "VolumeReplicationClass not found", "class", className)
			return nil, fmt.Errorf("VolumeReplicationClass %q not found", className)
		}
		log.Error(err, "Failed to fetch VolumeReplicationClass", "class", className)
		return nil, err
	}

	// Validate required fields
	if vrc.Spec.Provisioner == "" {
		err := fmt.Errorf("VolumeReplicationClass %q has empty provisioner", className)
		log.Error(err, "Invalid VolumeReplicationClass")
		return nil, err
	}

	log.Info("Successfully fetched VolumeReplicationClass",
		"class", className,
		"provisioner", vrc.Spec.Provisioner)

	return vrc, nil
}

// detectBackend detects the backend type from provisioner string (Prompt 3.2)
func (r *VolumeReplicationReconciler) detectBackend(
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
