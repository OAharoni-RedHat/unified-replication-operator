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
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg"
	"github.com/unified-replication/operator/pkg/adapters"
	"github.com/unified-replication/operator/pkg/discovery"
	"github.com/unified-replication/operator/pkg/translation"
)

const (
	// Finalizer name for cleanup
	unifiedReplicationFinalizer = "replication.storage.io/finalizer"

	// Requeue delays
	requeueDelaySuccess = 30 * time.Second
	requeueDelayError   = 10 * time.Second
	requeueDelayFast    = 5 * time.Second
)

// UnifiedVolumeReplicationReconciler reconciles a UnifiedVolumeReplication object
type UnifiedVolumeReplicationReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	AdapterRegistry adapters.Registry

	// Engines (Phase 4.2)
	DiscoveryEngine   *discovery.Engine
	TranslationEngine *translation.Engine
	ControllerEngine  *pkg.ControllerEngine

	// Advanced features (Phase 4.3)
	StateMachine   *StateMachine
	RetryManager   *RetryManager
	CircuitBreaker *CircuitBreaker

	// Configuration
	MaxConcurrentReconciles int
	ReconcileTimeout        time.Duration
	UseIntegratedEngine     bool // Enable engine integration (Phase 4.2)
	EnableAdvancedFeatures  bool // Enable Phase 4.3 features
}

// SetupWithManager sets up the controller with the Manager.
func (r *UnifiedVolumeReplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&replicationv1alpha1.UnifiedVolumeReplication{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.getMaxConcurrentReconciles(),
		}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=replication.storage.io,resources=unifiedvolumereplications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=replication.storage.io,resources=unifiedvolumereplications/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=replication.storage.io,resources=unifiedvolumereplications/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile implements the reconciliation loop for UnifiedVolumeReplication
func (r *UnifiedVolumeReplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues(
		"unifiedvolumereplication", req.NamespacedName,
	)
	log.Info("Starting reconciliation")

	// Create context with timeout
	reconcileCtx, cancel := context.WithTimeout(ctx, r.getReconcileTimeout())
	defer cancel()

	// Fetch the UnifiedVolumeReplication instance
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{}
	if err := r.Get(reconcileCtx, req.NamespacedName, uvr); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("UnifiedVolumeReplication resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get UnifiedVolumeReplication")
		return ctrl.Result{}, err
	}

	// Initialize status if needed
	if uvr.Status.Conditions == nil {
		uvr.Status.Conditions = []metav1.Condition{}
	}

	// Handle deletion
	if !uvr.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.handleDeletion(reconcileCtx, uvr, log)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(uvr, unifiedReplicationFinalizer) {
		log.Info("Adding finalizer")
		controllerutil.AddFinalizer(uvr, unifiedReplicationFinalizer)
		if err := r.Update(reconcileCtx, uvr); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		// Requeue to continue with reconciliation
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// Reconcile the replication
	return r.reconcileReplication(reconcileCtx, uvr, log)
}

// reconcileReplication handles the main reconciliation logic
func (r *UnifiedVolumeReplicationReconciler) reconcileReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication, log logr.Logger) (ctrl.Result, error) {
	log.Info("Reconciling replication",
		"state", uvr.Spec.ReplicationState,
		"mode", uvr.Spec.ReplicationMode,
		"generation", uvr.Generation)

	// Phase 4.3: Validate state transitions using state machine
	if r.EnableAdvancedFeatures && r.StateMachine != nil {
		// Get current state from status (if available)
		currentState := r.getCurrentState(uvr)
		desiredState := uvr.Spec.ReplicationState

		if currentState != "" && currentState != desiredState {
			if err := r.StateMachine.ValidateTransition(currentState, desiredState); err != nil {
				log.Error(err, "Invalid state transition",
					"from", currentState,
					"to", desiredState)
				r.updateCondition(uvr, metav1.Condition{
					Type:               "Ready",
					Status:             metav1.ConditionFalse,
					Reason:             "InvalidStateTransition",
					Message:            fmt.Sprintf("Invalid transition from %s to %s", currentState, desiredState),
					ObservedGeneration: uvr.Generation,
				})

				if err := r.Status().Update(ctx, uvr); err != nil {
					log.Error(err, "Failed to update status")
				}

				return ctrl.Result{RequeueAfter: requeueDelayError}, err
			}

			// Record valid transition
			r.StateMachine.RecordTransition(currentState, desiredState, "user_requested", "")
			log.Info("Valid state transition", "from", currentState, "to", desiredState)
		}
	}

	// Validate the spec
	if err := uvr.ValidateSpec(); err != nil {
		log.Error(err, "Spec validation failed")
		r.updateCondition(uvr, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "ValidationFailed",
			Message:            fmt.Sprintf("Validation failed: %v", err),
			ObservedGeneration: uvr.Generation,
		})
		r.Recorder.Event(uvr, corev1.EventTypeWarning, "ValidationFailed", err.Error())

		if err := r.Status().Update(ctx, uvr); err != nil {
			log.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}

		return ctrl.Result{RequeueAfter: requeueDelayError}, nil
	}

	// Get the appropriate adapter
	adapter, err := r.getAdapter(ctx, uvr, log)
	if err != nil {
		log.Error(err, "Failed to get adapter")
		r.updateCondition(uvr, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "AdapterError",
			Message:            fmt.Sprintf("Failed to get adapter: %v", err),
			ObservedGeneration: uvr.Generation,
		})
		r.Recorder.Event(uvr, corev1.EventTypeWarning, "AdapterError", err.Error())

		if err := r.Status().Update(ctx, uvr); err != nil {
			log.Error(err, "Failed to update status")
		}

		return ctrl.Result{RequeueAfter: requeueDelayError}, err
	}

	// Initialize adapter if needed
	if err := adapter.Initialize(ctx); err != nil {
		log.Error(err, "Failed to initialize adapter")
		r.updateCondition(uvr, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "InitializationFailed",
			Message:            fmt.Sprintf("Adapter initialization failed: %v", err),
			ObservedGeneration: uvr.Generation,
		})

		if err := r.Status().Update(ctx, uvr); err != nil {
			log.Error(err, "Failed to update status")
		}

		return ctrl.Result{RequeueAfter: requeueDelayError}, err
	}

	// Determine the operation based on status
	operation := r.determineOperation(uvr)
	log.Info("Determined operation", "operation", operation)

	// Execute the operation
	var opErr error

	// Use integrated engine if available (Phase 4.2)
	if r.UseIntegratedEngine && r.ControllerEngine != nil {
		log.Info("Using integrated controller engine")
		opErr = r.ControllerEngine.ProcessReplication(ctx, uvr, operation, log)
	} else {
		// Fallback to direct adapter operations (Phase 4.1)
		log.V(1).Info("Using direct adapter operations")
		switch operation {
		case "create":
			opErr = r.handleCreate(ctx, adapter, uvr, log)
		case "update":
			opErr = r.handleUpdate(ctx, adapter, uvr, log)
		case "sync":
			opErr = r.handleSync(ctx, adapter, uvr, log)
		default:
			log.Info("No operation needed")
		}
	}

	if opErr != nil {
		log.Error(opErr, "Operation failed", "operation", operation)
		r.updateCondition(uvr, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "OperationFailed",
			Message:            fmt.Sprintf("Operation %s failed: %v", operation, opErr),
			ObservedGeneration: uvr.Generation,
		})
		r.Recorder.Eventf(uvr, corev1.EventTypeWarning, "OperationFailed", "Operation %s failed: %v", operation, opErr)

		if err := r.Status().Update(ctx, uvr); err != nil {
			log.Error(err, "Failed to update status")
		}

		return ctrl.Result{RequeueAfter: requeueDelayError}, opErr
	}

	// Update status from adapter
	if r.UseIntegratedEngine && r.ControllerEngine != nil {
		// Use integrated engine for status with translation
		status, err := r.ControllerEngine.GetReplicationStatus(ctx, uvr, log)
		if err != nil {
			log.Error(err, "Failed to get status from integrated engine")
		} else if status != nil {
			r.updateStatusFromEngineStatus(uvr, status, log)
		}
	} else if adapter != nil {
		// Fallback to direct adapter status
		if err := r.updateStatusFromAdapter(ctx, adapter, uvr, log); err != nil {
			log.Error(err, "Failed to update status from adapter")
			// Don't fail reconciliation for status update errors
		}
	}

	// Set ready condition
	r.updateCondition(uvr, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "ReconciliationSucceeded",
		Message:            "Replication is operating normally",
		ObservedGeneration: uvr.Generation,
	})

	// Update status
	if err := r.Status().Update(ctx, uvr); err != nil {
		log.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciliation completed successfully")
	return ctrl.Result{RequeueAfter: requeueDelaySuccess}, nil
}

// handleCreate creates a new replication in the backend
func (r *UnifiedVolumeReplicationReconciler) handleCreate(ctx context.Context, adapter adapters.ReplicationAdapter, uvr *replicationv1alpha1.UnifiedVolumeReplication, log logr.Logger) error {
	log.Info("Creating replication in backend")

	// Use integrated engine if available (Phase 4.2)
	if r.UseIntegratedEngine && r.ControllerEngine != nil {
		log.Info("Using integrated engine for creation")
		if err := r.ControllerEngine.ProcessReplication(ctx, uvr, "create", log); err != nil {
			return fmt.Errorf("failed to create replication via engine: %w", err)
		}
	} else {
		// Fallback to direct adapter call (Phase 4.1)
		if err := adapter.CreateReplication(ctx, uvr); err != nil {
			return fmt.Errorf("failed to create replication: %w", err)
		}
	}

	r.Recorder.Event(uvr, corev1.EventTypeNormal, "Created", "Replication created successfully")
	return nil
}

// handleUpdate updates an existing replication in the backend
func (r *UnifiedVolumeReplicationReconciler) handleUpdate(ctx context.Context, adapter adapters.ReplicationAdapter, uvr *replicationv1alpha1.UnifiedVolumeReplication, log logr.Logger) error {
	log.Info("Updating replication in backend")

	// Use integrated engine if available (Phase 4.2)
	if r.UseIntegratedEngine && r.ControllerEngine != nil {
		log.Info("Using integrated engine for update")
		if err := r.ControllerEngine.ProcessReplication(ctx, uvr, "update", log); err != nil {
			return fmt.Errorf("failed to update replication via engine: %w", err)
		}
	} else {
		// Fallback to direct adapter call (Phase 4.1)
		if err := adapter.UpdateReplication(ctx, uvr); err != nil {
			return fmt.Errorf("failed to update replication: %w", err)
		}
	}

	r.Recorder.Event(uvr, corev1.EventTypeNormal, "Updated", "Replication updated successfully")
	return nil
}

// handleSync synchronizes status from the backend
func (r *UnifiedVolumeReplicationReconciler) handleSync(ctx context.Context, adapter adapters.ReplicationAdapter, uvr *replicationv1alpha1.UnifiedVolumeReplication, log logr.Logger) error {
	log.V(1).Info("Syncing status from backend")
	// Status sync happens in updateStatusFromAdapter
	return nil
}

// handleDeletion handles resource deletion with finalizer cleanup
func (r *UnifiedVolumeReplicationReconciler) handleDeletion(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication, log logr.Logger) (ctrl.Result, error) {
	log.Info("Handling deletion")

	if !controllerutil.ContainsFinalizer(uvr, unifiedReplicationFinalizer) {
		log.Info("Finalizer already removed, skipping cleanup")
		return ctrl.Result{}, nil
	}

	// Get adapter for cleanup
	adapter, err := r.getAdapter(ctx, uvr, log)
	if err != nil {
		log.Error(err, "Failed to get adapter for cleanup, removing finalizer anyway")
		// Remove finalizer even if we can't get adapter
		controllerutil.RemoveFinalizer(uvr, unifiedReplicationFinalizer)
		if err := r.Update(ctx, uvr); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Delete replication from backend
	log.Info("Deleting replication from backend")
	if err := adapter.DeleteReplication(ctx, uvr); err != nil {
		log.Error(err, "Failed to delete replication from backend")
		r.Recorder.Eventf(uvr, corev1.EventTypeWarning, "DeletionFailed", "Failed to delete from backend: %v", err)
		// Retry deletion
		return ctrl.Result{RequeueAfter: requeueDelayError}, err
	}

	r.Recorder.Event(uvr, corev1.EventTypeNormal, "Deleted", "Replication deleted successfully")

	// Remove finalizer
	log.Info("Removing finalizer")
	controllerutil.RemoveFinalizer(uvr, unifiedReplicationFinalizer)
	if err := r.Update(ctx, uvr); err != nil {
		log.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	log.Info("Deletion completed")
	return ctrl.Result{}, nil
}

// determineOperation determines what operation needs to be performed
func (r *UnifiedVolumeReplicationReconciler) determineOperation(uvr *replicationv1alpha1.UnifiedVolumeReplication) string {
	// If no conditions, this is a new resource
	if len(uvr.Status.Conditions) == 0 {
		return "create"
	}

	// Check if ready condition exists
	readyCondition := r.getCondition(uvr, "Ready")
	if readyCondition == nil {
		return "create"
	}

	// If observed generation is behind, need to update
	if readyCondition.ObservedGeneration < uvr.Generation {
		return "update"
	}

	// If ready but status is false, might need update
	if readyCondition.Status == metav1.ConditionFalse {
		return "update"
	}

	// Otherwise just sync status
	return "sync"
}

// getAdapter retrieves the appropriate adapter for the UVR
func (r *UnifiedVolumeReplicationReconciler) getAdapter(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication, log logr.Logger) (adapters.ReplicationAdapter, error) {
	// Phase 4.2: Use integrated engine for discovery-based adapter selection
	if r.UseIntegratedEngine && r.DiscoveryEngine != nil && r.AdapterRegistry != nil {
		log.V(1).Info("Using integrated engine for adapter selection")

		// Discover available backends
		backends, err := r.DiscoveryEngine.DiscoverBackends(ctx)
		if err != nil {
			log.Error(err, "Discovery failed, falling back to extension-based selection")
		} else if backends != nil && len(backends.AvailableBackends) > 0 {
			// Select backend using engine logic
			backend, err := r.selectBackendViaEngine(ctx, uvr, backends.AvailableBackends, log)
			if err == nil {
				// Get adapter via registry
				factory, err := r.AdapterRegistry.GetFactory(backend)
				if err == nil {
					adapter, err := factory.CreateAdapter(backend, r.Client, r.TranslationEngine, nil)
					if err == nil {
						_ = adapter.Initialize(ctx)
						log.Info("Selected adapter via engine", "backend", backend)
						return adapter, nil
					}
				}
				log.Error(err, "Failed to get adapter via registry", "backend", backend)
			}
		}
	}

	// Fallback: Phase 4.1 behavior - extension-based selection
	log.V(1).Info("Using extension-based adapter selection")

	if uvr.Spec.Extensions != nil {
		if uvr.Spec.Extensions.Ceph != nil {
			log.Info("Using Ceph adapter")
			if adapter, err := adapters.NewCephAdapter(r.Client, r.TranslationEngine); err == nil {
				return adapter, nil
			}
			return nil, fmt.Errorf("ceph adapter creation failed")
		}
		if uvr.Spec.Extensions.Trident != nil {
			log.Info("Using Trident mock adapter")
			config := adapters.DefaultMockTridentConfig()
			return adapters.NewMockTridentAdapter(r.Client, r.TranslationEngine, config), nil
		}
		if uvr.Spec.Extensions.Powerstore != nil {
			log.Info("Using PowerStore mock adapter")
			config := adapters.DefaultMockPowerStoreConfig()
			return adapters.NewMockPowerStoreAdapter(r.Client, r.TranslationEngine, config), nil
		}
	}

	return nil, fmt.Errorf("no backend adapter found for this configuration")
}

// selectBackendViaEngine uses the engine's backend selection logic
func (r *UnifiedVolumeReplicationReconciler) selectBackendViaEngine(
	ctx context.Context,
	uvr *replicationv1alpha1.UnifiedVolumeReplication,
	availableBackends []translation.Backend,
	log logr.Logger,
) (translation.Backend, error) {
	// Use extension hints first
	if uvr.Spec.Extensions != nil {
		if uvr.Spec.Extensions.Ceph != nil {
			for _, backend := range availableBackends {
				if backend == translation.BackendCeph {
					return backend, nil
				}
			}
		}
		if uvr.Spec.Extensions.Trident != nil {
			for _, backend := range availableBackends {
				if backend == translation.BackendTrident {
					return backend, nil
				}
			}
		}
		if uvr.Spec.Extensions.Powerstore != nil {
			for _, backend := range availableBackends {
				if backend == translation.BackendPowerStore {
					return backend, nil
				}
			}
		}
	}

	// Detect from storage class
	storageClass := uvr.Spec.SourceEndpoint.StorageClass
	for _, backend := range availableBackends {
		switch backend {
		case translation.BackendCeph:
			if contains(storageClass, "ceph") || contains(storageClass, "rbd") {
				return backend, nil
			}
		case translation.BackendTrident:
			if contains(storageClass, "trident") || contains(storageClass, "netapp") {
				return backend, nil
			}
		case translation.BackendPowerStore:
			if contains(storageClass, "powerstore") || contains(storageClass, "dell") {
				return backend, nil
			}
		}
	}

	// Use first available
	if len(availableBackends) > 0 {
		return availableBackends[0], nil
	}

	return "", fmt.Errorf("no available backends found")
}

// updateStatusFromAdapter updates the UVR status from the backend adapter
func (r *UnifiedVolumeReplicationReconciler) updateStatusFromAdapter(ctx context.Context, adapter adapters.ReplicationAdapter, uvr *replicationv1alpha1.UnifiedVolumeReplication, log logr.Logger) error {
	var status *adapters.ReplicationStatus
	var err error

	// Phase 4.2: Use integrated engine for status retrieval with translation
	if r.UseIntegratedEngine && r.ControllerEngine != nil {
		log.V(1).Info("Using integrated engine for status retrieval")
		status, err = r.ControllerEngine.GetReplicationStatus(ctx, uvr, log)
		if err != nil {
			return fmt.Errorf("failed to get replication status via engine: %w", err)
		}
	} else {
		// Fallback: Direct adapter call (Phase 4.1)
		status, err = adapter.GetReplicationStatus(ctx, uvr)
		if err != nil {
			return fmt.Errorf("failed to get replication status: %w", err)
		}
	}

	if status == nil {
		return nil
	}

	// Update observed generation
	uvr.Status.ObservedGeneration = uvr.Generation

	// Add status information to conditions
	if status.State != "" {
		r.updateCondition(uvr, metav1.Condition{
			Type:               "Synced",
			Status:             metav1.ConditionTrue,
			Reason:             "StatusUpdated",
			Message:            fmt.Sprintf("Current state: %s, mode: %s", status.State, status.Mode),
			ObservedGeneration: uvr.Generation,
		})
	}

	log.V(1).Info("Updated status from adapter",
		"state", status.State,
		"mode", status.Mode)
	return nil
}

// updateStatusFromEngineStatus updates status from integrated engine (with translation)
func (r *UnifiedVolumeReplicationReconciler) updateStatusFromEngineStatus(uvr *replicationv1alpha1.UnifiedVolumeReplication, status *adapters.ReplicationStatus, log logr.Logger) {
	// Update observed generation
	uvr.Status.ObservedGeneration = uvr.Generation

	// Add status information to conditions (state and mode are already in unified format)
	if status.State != "" {
		r.updateCondition(uvr, metav1.Condition{
			Type:               "Synced",
			Status:             metav1.ConditionTrue,
			Reason:             "StatusUpdated",
			Message:            fmt.Sprintf("Current state: %s, mode: %s (via integrated engine)", status.State, status.Mode),
			ObservedGeneration: uvr.Generation,
		})
	}

	log.V(1).Info("Updated status from integrated engine",
		"state", status.State,
		"mode", status.Mode)
}

// updateCondition updates or adds a condition to the status
func (r *UnifiedVolumeReplicationReconciler) updateCondition(uvr *replicationv1alpha1.UnifiedVolumeReplication, condition metav1.Condition) {
	condition.LastTransitionTime = metav1.NewTime(time.Now())

	// Find existing condition
	for i, existingCondition := range uvr.Status.Conditions {
		if existingCondition.Type == condition.Type {
			// Update if status changed
			if existingCondition.Status != condition.Status {
				uvr.Status.Conditions[i] = condition
			} else {
				// Just update message and reason
				uvr.Status.Conditions[i].Message = condition.Message
				uvr.Status.Conditions[i].Reason = condition.Reason
				uvr.Status.Conditions[i].ObservedGeneration = condition.ObservedGeneration
			}
			return
		}
	}

	// Add new condition
	uvr.Status.Conditions = append(uvr.Status.Conditions, condition)
}

// getCondition retrieves a condition by type
func (r *UnifiedVolumeReplicationReconciler) getCondition(uvr *replicationv1alpha1.UnifiedVolumeReplication, conditionType string) *metav1.Condition {
	for _, condition := range uvr.Status.Conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// getMaxConcurrentReconciles returns the configured max concurrent reconciles
func (r *UnifiedVolumeReplicationReconciler) getMaxConcurrentReconciles() int {
	if r.MaxConcurrentReconciles > 0 {
		return r.MaxConcurrentReconciles
	}
	return 1 // Default to 1
}

// getReconcileTimeout returns the configured reconcile timeout
func (r *UnifiedVolumeReplicationReconciler) getReconcileTimeout() time.Duration {
	if r.ReconcileTimeout > 0 {
		return r.ReconcileTimeout
	}
	return 5 * time.Minute // Default timeout
}

// getCurrentState extracts the current state from the UVR status
func (r *UnifiedVolumeReplicationReconciler) getCurrentState(uvr *replicationv1alpha1.UnifiedVolumeReplication) replicationv1alpha1.ReplicationState {
	// Look for Synced condition which contains current state info
	for _, cond := range uvr.Status.Conditions {
		if cond.Type == "Synced" && cond.Status == metav1.ConditionTrue {
			// State was previously set
			// For now, return empty if we can't reliably determine it
			// In production, this would parse the condition message or have explicit status field
			return ""
		}
	}

	// If this is first reconcile, no current state
	return ""
}

// Helper functions

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	return len(sLower) >= len(substrLower) && containsSubstr(sLower, substrLower)
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
