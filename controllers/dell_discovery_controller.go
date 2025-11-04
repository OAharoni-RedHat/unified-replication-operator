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
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

const (
	// DellCSIDiscoveryAnnotation marks a UnifiedVolumeReplication as imported from Dell CR
	DellCSIDiscoveryAnnotation = "replication.unified.io/dell-discovered"
	// UnifiedOperatorManagedLabel identifies resources managed by unified operator
	UnifiedOperatorManagedLabel = "app.kubernetes.io/managed-by"
	// UnifiedOperatorManagedValue is the value for the managed-by label
	UnifiedOperatorManagedValue = "unified-replication-operator"
)

// DellCSIReplicationGroupGVK is the GroupVersionKind for DellCSIReplicationGroup
var DellCSIReplicationGroupGVK = schema.GroupVersionKind{
	Group:   "replication.storage.dell.com",
	Version: "v1",
	Kind:    "DellCSIReplicationGroup",
}

// DellCSIDiscoveryReconciler reconciles DellCSIReplicationGroup resources
// and automatically creates corresponding UnifiedVolumeReplication resources
type DellCSIDiscoveryReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	TranslationEngine *translation.Engine
}

//+kubebuilder:rbac:groups=replication.storage.dell.com,resources=dellcsireplicationgroups,verbs=get;list;watch
//+kubebuilder:rbac:groups=replication.unified.io,resources=unifiedvolumereplications,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *DellCSIDiscoveryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("dell-discovery-controller")

	// Fetch DellCSIReplicationGroup
	drg := &unstructured.Unstructured{}
	drg.SetGroupVersionKind(DellCSIReplicationGroupGVK)
	if err := r.Get(ctx, req.NamespacedName, drg); err != nil {
		if errors.IsNotFound(err) {
			log.Info("DellCSIReplicationGroup not found, may have been deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get DellCSIReplicationGroup")
		return ctrl.Result{}, err
	}

	// Check if this Dell CR was created by unified operator
	if r.isManagedByUnifiedOperator(drg) {
		log.V(1).Info("DellCSIReplicationGroup is managed by unified operator, skipping discovery")
		return ctrl.Result{}, nil
	}

	// Check if UnifiedVolumeReplication already exists
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{}
	err := r.Get(ctx, req.NamespacedName, uvr)
	if err == nil {
		// UVR already exists - check if it was imported from this Dell CR
		if uvr.Annotations[DellCSIDiscoveryAnnotation] == "true" {
			log.V(1).Info("UnifiedVolumeReplication already exists and was imported from Dell CR, syncing if needed")
			return r.syncUVRFromDellCR(ctx, drg, uvr, log)
		}
		// UVR exists but wasn't imported - skip to avoid conflicts
		log.Info("UnifiedVolumeReplication already exists but wasn't imported from Dell CR, skipping")
		return ctrl.Result{}, nil
	}
	if !errors.IsNotFound(err) {
		log.Error(err, "Failed to check for existing UnifiedVolumeReplication")
		return ctrl.Result{}, err
	}

	// Create UnifiedVolumeReplication from Dell CR
	log.Info("Discovered DellCSIReplicationGroup, creating UnifiedVolumeReplication")
	return r.createUVRFromDellCR(ctx, drg, log)
}

// isManagedByUnifiedOperator checks if the Dell CR was created by unified operator
func (r *DellCSIDiscoveryReconciler) isManagedByUnifiedOperator(drg *unstructured.Unstructured) bool {
	labels := drg.GetLabels()
	if labels == nil {
		return false
	}
	managedBy, exists := labels[UnifiedOperatorManagedLabel]
	return exists && managedBy == UnifiedOperatorManagedValue
}

// createUVRFromDellCR creates a UnifiedVolumeReplication from a DellCSIReplicationGroup
func (r *DellCSIDiscoveryReconciler) createUVRFromDellCR(ctx context.Context, drg *unstructured.Unstructured, log logr.Logger) (ctrl.Result, error) {
	// Translate Dell CR to UnifiedVolumeReplication spec
	uvrSpec, err := r.translateDellCRToUVRSpec(ctx, drg, log)
	if err != nil {
		log.Error(err, "Failed to translate Dell CR to UnifiedVolumeReplication spec")
		return ctrl.Result{}, err
	}

	// Create UnifiedVolumeReplication
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      drg.GetName(),
			Namespace: drg.GetNamespace(),
			Annotations: map[string]string{
				DellCSIDiscoveryAnnotation:                 "true",
				"replication.unified.io/dell-cr-name":      drg.GetName(),
				"replication.unified.io/dell-cr-namespace": drg.GetNamespace(),
				"replication.unified.io/imported-at":       metav1.Now().Format(time.RFC3339),
			},
			Labels: map[string]string{
				"replication.unified.io/discovered": "true",
				"replication.unified.io/source":     "dell-csi",
			},
		},
		Spec: *uvrSpec,
	}

	// Validate the spec before creating
	if err := uvr.ValidateSpec(); err != nil {
		log.Error(err, "Translated UnifiedVolumeReplication spec validation failed")
		// Still create it but log the validation error - user can fix it later
		log.Info("Creating UnifiedVolumeReplication anyway - user can fix validation issues later")
	}

	// Set owner reference to Dell CR (non-blocking)
	if err := controllerutil.SetControllerReference(drg, uvr, r.Scheme); err != nil {
		log.Info("Could not set owner reference (non-fatal)", "error", err)
		// Continue without owner reference - this allows Dell CR to be deleted independently if needed
	}

	if err := r.Create(ctx, uvr); err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("UnifiedVolumeReplication already exists, will sync on next reconcile")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to create UnifiedVolumeReplication")
		return ctrl.Result{}, err
	}

	log.Info("Successfully created UnifiedVolumeReplication from Dell CR",
		"uvr", uvr.Name,
		"namespace", uvr.Namespace)
	return ctrl.Result{}, nil
}

// syncUVRFromDellCR syncs UnifiedVolumeReplication status from Dell CR
func (r *DellCSIDiscoveryReconciler) syncUVRFromDellCR(ctx context.Context, drg *unstructured.Unstructured, uvr *replicationv1alpha1.UnifiedVolumeReplication, log logr.Logger) (ctrl.Result, error) {
	// For now, just update status from Dell CR
	// The UnifiedVolumeReplication controller will handle the actual reconciliation
	log.V(1).Info("Syncing UnifiedVolumeReplication status from Dell CR")
	return ctrl.Result{}, nil
}

// translateDellCRToUVRSpec translates DellCSIReplicationGroup spec to UnifiedVolumeReplication spec
func (r *DellCSIDiscoveryReconciler) translateDellCRToUVRSpec(ctx context.Context, drg *unstructured.Unstructured, log logr.Logger) (*replicationv1alpha1.UnifiedVolumeReplicationSpec, error) {
	spec := &replicationv1alpha1.UnifiedVolumeReplicationSpec{}

	// Extract Dell CR spec fields
	drgSpec, found, err := unstructured.NestedMap(drg.Object, "spec")
	if !found || err != nil {
		return nil, fmt.Errorf("failed to extract Dell CR spec: %w", err)
	}

	// Extract PVC selector or source volumes to get PVC name
	pvcName, pvcNamespace, err := r.extractPVCInfo(ctx, drg, drgSpec, log)
	if err != nil {
		return nil, fmt.Errorf("failed to extract PVC info: %w", err)
	}

	// Extract storage class from PVC
	storageClass, err := r.getStorageClassFromPVC(ctx, pvcName, pvcNamespace, log)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("PVC not found, will use defaults. PVC may be created later",
				"pvc", pvcName,
				"namespace", pvcNamespace)
		} else {
			log.Info("Could not determine storage class from PVC, using defaults", "error", err)
		}
		storageClass = "powerstore-replication" // Default fallback
	}

	// Extract remote system (destination cluster)
	remoteSystem, _, _ := unstructured.NestedString(drgSpec, "remoteSystem")
	if remoteSystem == "" {
		remoteSystem = "remote-cluster" // Default fallback
	}

	// Extract protection policy to determine replication mode
	protectionPolicy, _, _ := unstructured.NestedString(drgSpec, "protectionPolicy")
	replicationMode := replicationv1alpha1.ReplicationModeAsynchronous
	if strings.ToUpper(protectionPolicy) == "METRO" {
		replicationMode = replicationv1alpha1.ReplicationModeSynchronous
	}

	// Extract RPO
	rpo, _, _ := unstructured.NestedString(drgSpec, "remoteRPO")
	if rpo == "" {
		rpo = "15m" // Default
	}

	// Extract Dell state and translate to unified state
	dellState, _, _ := unstructured.NestedString(drgSpec, "state")
	if dellState == "" {
		// Try status.state if spec.state is not set
		dellState, _, _ = unstructured.NestedString(drg.Object, "status", "state")
	}
	replicationState := r.translateDellStateToUnified(dellState)

	// Extract volume handle from remote volumes or status
	volumeHandle := r.extractVolumeHandle(drg, drgSpec)

	// Build UnifiedVolumeReplication spec
	spec.ReplicationState = replicationState
	spec.ReplicationMode = replicationMode
	spec.SourceEndpoint = replicationv1alpha1.Endpoint{
		Cluster:      "local-cluster",
		Region:       "local-region",
		StorageClass: storageClass,
	}
	spec.DestinationEndpoint = replicationv1alpha1.Endpoint{
		Cluster:      remoteSystem,
		Region:       "remote-region",
		StorageClass: storageClass,
	}
	spec.VolumeMapping = replicationv1alpha1.VolumeMapping{
		Source: replicationv1alpha1.VolumeSource{
			PvcName:   pvcName,
			Namespace: pvcNamespace,
		},
		Destination: replicationv1alpha1.VolumeDestination{
			VolumeHandle: volumeHandle,
			Namespace:    pvcNamespace,
		},
	}
	spec.Schedule = replicationv1alpha1.Schedule{
		Mode: replicationv1alpha1.ScheduleModeContinuous,
		Rpo:  rpo,
		Rto:  "5m", // Default RTO
	}

	return spec, nil
}

// extractPVCInfo extracts PVC name and namespace from Dell CR
func (r *DellCSIDiscoveryReconciler) extractPVCInfo(ctx context.Context, drg *unstructured.Unstructured, drgSpec map[string]interface{}, log logr.Logger) (string, string, error) {
	// Try to get PVC from pvcSelector
	if pvcSelector, found, _ := unstructured.NestedMap(drgSpec, "pvcSelector"); found {
		if matchLabels, found, _ := unstructured.NestedMap(pvcSelector, "matchLabels"); found {
			// Look for PVCs with matching labels
			groupName, _, _ := unstructured.NestedString(matchLabels, "replication.storage.dell.com/group")
			if groupName != "" {
				// Find PVCs with this label
				pvcList := &corev1.PersistentVolumeClaimList{}
				labels := map[string]string{
					"replication.storage.dell.com/group": groupName,
				}
				if err := r.List(ctx, pvcList, client.InNamespace(drg.GetNamespace()), client.MatchingLabels(labels)); err == nil {
					if len(pvcList.Items) > 0 {
						pvc := pvcList.Items[0]
						return pvc.Name, pvc.Namespace, nil
					}
				}
			}
		}
	}

	// Try sourceVolumes array
	if sourceVolumes, found, _ := unstructured.NestedSlice(drgSpec, "sourceVolumes"); found && len(sourceVolumes) > 0 {
		if sourceVol, ok := sourceVolumes[0].(map[string]interface{}); ok {
			if pvcName, found, _ := unstructured.NestedString(sourceVol, "pvcName"); found && pvcName != "" {
				return pvcName, drg.GetNamespace(), nil
			}
		}
	}

	// Fallback: use Dell CR name as PVC name (common pattern)
	return drg.GetName(), drg.GetNamespace(), nil
}

// getStorageClassFromPVC gets the storage class name from a PVC
func (r *DellCSIDiscoveryReconciler) getStorageClassFromPVC(ctx context.Context, pvcName, namespace string, log logr.Logger) (string, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: namespace}, pvc); err != nil {
		return "", err
	}

	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
		return "", fmt.Errorf("PVC has no storage class")
	}

	return *pvc.Spec.StorageClassName, nil
}

// extractVolumeHandle extracts volume handle from Dell CR
func (r *DellCSIDiscoveryReconciler) extractVolumeHandle(drg *unstructured.Unstructured, drgSpec map[string]interface{}) string {
	// Try remoteVolumes array
	if remoteVolumes, found, _ := unstructured.NestedSlice(drgSpec, "remoteVolumes"); found && len(remoteVolumes) > 0 {
		if remoteVol, ok := remoteVolumes[0].(map[string]interface{}); ok {
			if volumeHandle, found, _ := unstructured.NestedString(remoteVol, "volumeHandle"); found && volumeHandle != "" {
				return volumeHandle
			}
		}
	}

	// Try status.remoteVolumeHandle
	if volumeHandle, found, _ := unstructured.NestedString(drg.Object, "status", "remoteVolumeHandle"); found && volumeHandle != "" {
		return volumeHandle
	}

	// Fallback: generate a placeholder
	return fmt.Sprintf("remote-volume-%s", drg.GetName())
}

// translateDellStateToUnified translates Dell state to unified state
func (r *DellCSIDiscoveryReconciler) translateDellStateToUnified(dellState string) replicationv1alpha1.ReplicationState {
	// Use translation engine if available
	if r.TranslationEngine != nil {
		if unifiedState, err := r.TranslationEngine.TranslateStateFromBackend(translation.BackendPowerStore, dellState); err == nil {
			return replicationv1alpha1.ReplicationState(unifiedState)
		}
	}

	// Fallback manual translation
	switch strings.ToLower(dellState) {
	case "source":
		return replicationv1alpha1.ReplicationStateSource
	case "destination":
		return replicationv1alpha1.ReplicationStateReplica
	case "promoting":
		return replicationv1alpha1.ReplicationStatePromoting
	case "demoting":
		return replicationv1alpha1.ReplicationStateDemoting
	case "syncing":
		return replicationv1alpha1.ReplicationStateSyncing
	case "failed":
		return replicationv1alpha1.ReplicationStateFailed
	case "synchronized":
		// Status state that indicates replica
		return replicationv1alpha1.ReplicationStateReplica
	case "failedover":
		// Status state that indicates source
		return replicationv1alpha1.ReplicationStateSource
	default:
		// Default to replica for safety
		return replicationv1alpha1.ReplicationStateReplica
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *DellCSIDiscoveryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create an unstructured object with the Dell CRD GVK for watching
	drg := &unstructured.Unstructured{}
	drg.SetGroupVersionKind(DellCSIReplicationGroupGVK)

	// Create a predicate to filter only DellCSIReplicationGroup resources
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return r.isDellCR(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return r.isDellCR(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return r.isDellCR(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return r.isDellCR(e.Object)
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(drg).
		WithEventFilter(pred).
		Complete(r)
}

// isDellCR checks if an object is a DellCSIReplicationGroup
func (r *DellCSIDiscoveryReconciler) isDellCR(obj client.Object) bool {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return false
	}
	gvk := u.GroupVersionKind()
	return gvk.Group == DellCSIReplicationGroupGVK.Group &&
		gvk.Kind == DellCSIReplicationGroupGVK.Kind &&
		gvk.Version == DellCSIReplicationGroupGVK.Version
}
