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

package webhook

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/security"
)

// log is for logging in this package.
var unifiedvolumereplicationlog = logf.Log.WithName("unifiedvolumereplication-webhook")

// UnifiedVolumeReplicationValidator validates UnifiedVolumeReplication resources
type UnifiedVolumeReplicationValidator struct {
	Client            client.Client
	SecurityValidator *security.SecurityValidator
	AuditLogger       *security.AuditLogger
	EnableAudit       bool
	// validationCount   int64 // TODO: Implement validation metrics
	// lastValidation    time.Time // TODO: Implement validation tracking
}

// NewUnifiedVolumeReplicationValidator creates a new validator
func NewUnifiedVolumeReplicationValidator(client client.Client) *UnifiedVolumeReplicationValidator {
	return &UnifiedVolumeReplicationValidator{
		Client:            client,
		SecurityValidator: security.NewSecurityValidator(),
		AuditLogger:       security.NewAuditLogger(unifiedvolumereplicationlog, true),
		EnableAudit:       true,
	}
}

// NewUnifiedVolumeReplicationValidatorWithSecurity creates a validator with custom security config
func NewUnifiedVolumeReplicationValidatorWithSecurity(
	client client.Client,
	secValidator *security.SecurityValidator,
	auditLogger *security.AuditLogger,
) *UnifiedVolumeReplicationValidator {
	return &UnifiedVolumeReplicationValidator{
		Client:            client,
		SecurityValidator: secValidator,
		AuditLogger:       auditLogger,
		EnableAudit:       auditLogger != nil,
	}
}

//+kubebuilder:webhook:path=/validate-replication-unified-io-v1alpha1-unifiedvolumereplication,mutating=false,failurePolicy=fail,sideEffects=None,groups=replication.unified.io,resources=unifiedvolumereplications,verbs=create;update,versions=v1alpha1,name=vunifiedvolumereplication.kb.io,admissionReviewVersions=v1

// ValidateCreate implements admission.CustomValidator interface
func (v *UnifiedVolumeReplicationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	uvr, ok := obj.(*replicationv1alpha1.UnifiedVolumeReplication)
	if !ok {
		return nil, fmt.Errorf("expected UnifiedVolumeReplication, got %T", obj)
	}

	unifiedvolumereplicationlog.Info("validating create", "name", uvr.Name)

	// Perform spec validation
	if err := uvr.ValidateSpec(); err != nil {
		unifiedvolumereplicationlog.Info("create validation failed", "name", uvr.Name, "error", err)
		return nil, fmt.Errorf("UnifiedVolumeReplication validation failed: %v", err)
	}

	// Additional create-specific validations
	if err := v.validateCreateSpecific(ctx, uvr); err != nil {
		unifiedvolumereplicationlog.Info("create-specific validation failed", "name", uvr.Name, "error", err)
		return nil, fmt.Errorf("UnifiedVolumeReplication create validation failed: %v", err)
	}

	unifiedvolumereplicationlog.Info("create validation passed", "name", uvr.Name)
	return nil, nil
}

// ValidateUpdate implements admission.CustomValidator interface
func (v *UnifiedVolumeReplicationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newUVR, ok := newObj.(*replicationv1alpha1.UnifiedVolumeReplication)
	if !ok {
		return nil, fmt.Errorf("expected UnifiedVolumeReplication, got %T", newObj)
	}

	oldUVR, ok := oldObj.(*replicationv1alpha1.UnifiedVolumeReplication)
	if !ok {
		return nil, fmt.Errorf("expected UnifiedVolumeReplication, got %T", oldObj)
	}

	unifiedvolumereplicationlog.Info("validating update", "name", newUVR.Name)

	// Perform spec validation on new object
	if err := newUVR.ValidateSpec(); err != nil {
		unifiedvolumereplicationlog.Info("update validation failed", "name", newUVR.Name, "error", err)
		return nil, fmt.Errorf("UnifiedVolumeReplication validation failed: %v", err)
	}

	// Additional update-specific validations
	if err := v.validateUpdateSpecific(ctx, oldUVR, newUVR); err != nil {
		unifiedvolumereplicationlog.Info("update-specific validation failed", "name", newUVR.Name, "error", err)
		return nil, fmt.Errorf("UnifiedVolumeReplication update validation failed: %v", err)
	}

	unifiedvolumereplicationlog.Info("update validation passed", "name", newUVR.Name)
	return nil, nil
}

// ValidateDelete implements admission.CustomValidator interface
func (v *UnifiedVolumeReplicationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No special validation needed for delete operations
	return nil, nil
}

// validateCreateSpecific performs create-specific validations
func (v *UnifiedVolumeReplicationValidator) validateCreateSpecific(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	// Check for naming conflicts
	if err := v.validateResourceNaming(ctx, uvr); err != nil {
		return fmt.Errorf("resource naming validation failed: %w", err)
	}

	// Validate that only one replication state is allowed per PVC
	if err := v.validatePVCUniqueness(ctx, uvr); err != nil {
		return fmt.Errorf("PVC uniqueness validation failed: %w", err)
	}

	return nil
}

// validateUpdateSpecific performs update-specific validations
func (v *UnifiedVolumeReplicationValidator) validateUpdateSpecific(ctx context.Context, oldUVR, newUVR *replicationv1alpha1.UnifiedVolumeReplication) error {
	// Validate immutable fields
	if err := v.validateImmutableFields(oldUVR, newUVR); err != nil {
		return fmt.Errorf("immutable field validation failed: %w", err)
	}

	// Validate state transitions
	if err := v.validateStateTransitions(oldUVR.Spec.ReplicationState, newUVR.Spec.ReplicationState); err != nil {
		return fmt.Errorf("state transition validation failed: %w", err)
	}

	return nil
}

// validateResourceNaming validates resource naming conventions
func (v *UnifiedVolumeReplicationValidator) validateResourceNaming(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	// Check if the name follows conventions (optional, but good practice)
	if len(uvr.Name) > 63 {
		return fmt.Errorf("resource name '%s' exceeds maximum length of 63 characters", uvr.Name)
	}

	return nil
}

// validatePVCUniqueness ensures each PVC has only one active replication configuration
func (v *UnifiedVolumeReplicationValidator) validatePVCUniqueness(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	// List all UnifiedVolumeReplications in the same namespace
	uvrList := &replicationv1alpha1.UnifiedVolumeReplicationList{}
	if err := v.Client.List(ctx, uvrList, client.InNamespace(uvr.Namespace)); err != nil {
		return fmt.Errorf("failed to list existing UnifiedVolumeReplications: %w", err)
	}

	// Check for conflicts with the same source PVC
	for _, existingUVR := range uvrList.Items {
		if existingUVR.Name == uvr.Name {
			continue // Skip self
		}

		// Check if both refer to the same source PVC
		if existingUVR.Spec.VolumeMapping.Source.PvcName == uvr.Spec.VolumeMapping.Source.PvcName &&
			existingUVR.Spec.VolumeMapping.Source.Namespace == uvr.Spec.VolumeMapping.Source.Namespace {
			return fmt.Errorf("PVC '%s/%s' is already referenced by UnifiedVolumeReplication '%s'",
				uvr.Spec.VolumeMapping.Source.Namespace,
				uvr.Spec.VolumeMapping.Source.PvcName,
				existingUVR.Name)
		}
	}

	return nil
}

// validateImmutableFields validates that certain fields cannot be changed after creation
func (v *UnifiedVolumeReplicationValidator) validateImmutableFields(oldUVR, newUVR *replicationv1alpha1.UnifiedVolumeReplication) error {
	// Volume mapping should be immutable
	if oldUVR.Spec.VolumeMapping.Source.PvcName != newUVR.Spec.VolumeMapping.Source.PvcName {
		return fmt.Errorf("volumeMapping.source.pvcName is immutable (was: %s, now: %s)",
			oldUVR.Spec.VolumeMapping.Source.PvcName,
			newUVR.Spec.VolumeMapping.Source.PvcName)
	}

	if oldUVR.Spec.VolumeMapping.Source.Namespace != newUVR.Spec.VolumeMapping.Source.Namespace {
		return fmt.Errorf("volumeMapping.source.namespace is immutable (was: %s, now: %s)",
			oldUVR.Spec.VolumeMapping.Source.Namespace,
			newUVR.Spec.VolumeMapping.Source.Namespace)
	}

	if oldUVR.Spec.VolumeMapping.Destination.VolumeHandle != newUVR.Spec.VolumeMapping.Destination.VolumeHandle {
		return fmt.Errorf("volumeMapping.destination.volumeHandle is immutable (was: %s, now: %s)",
			oldUVR.Spec.VolumeMapping.Destination.VolumeHandle,
			newUVR.Spec.VolumeMapping.Destination.VolumeHandle)
	}

	// Endpoints should be immutable (they define the replication relationship)
	if oldUVR.Spec.SourceEndpoint != newUVR.Spec.SourceEndpoint {
		return fmt.Errorf("sourceEndpoint is immutable")
	}

	if oldUVR.Spec.DestinationEndpoint != newUVR.Spec.DestinationEndpoint {
		return fmt.Errorf("destinationEndpoint is immutable")
	}

	return nil
}

// validateStateTransitions validates allowed state transitions
func (v *UnifiedVolumeReplicationValidator) validateStateTransitions(oldState, newState replicationv1alpha1.ReplicationState) error {
	if oldState == newState {
		return nil // No transition
	}

	// Define allowed transitions
	allowedTransitions := map[replicationv1alpha1.ReplicationState][]replicationv1alpha1.ReplicationState{
		replicationv1alpha1.ReplicationStateSource: {
			replicationv1alpha1.ReplicationStateDemoting,
			replicationv1alpha1.ReplicationStateFailed,
			replicationv1alpha1.ReplicationStateSyncing,
		},
		replicationv1alpha1.ReplicationStateReplica: {
			replicationv1alpha1.ReplicationStatePromoting,
			replicationv1alpha1.ReplicationStateFailed,
			replicationv1alpha1.ReplicationStateSyncing,
		},
		replicationv1alpha1.ReplicationStatePromoting: {
			replicationv1alpha1.ReplicationStateSource,
			replicationv1alpha1.ReplicationStateFailed,
		},
		replicationv1alpha1.ReplicationStateDemoting: {
			replicationv1alpha1.ReplicationStateReplica,
			replicationv1alpha1.ReplicationStateFailed,
		},
		replicationv1alpha1.ReplicationStateSyncing: {
			replicationv1alpha1.ReplicationStateSource,
			replicationv1alpha1.ReplicationStateReplica,
			replicationv1alpha1.ReplicationStateFailed,
		},
		replicationv1alpha1.ReplicationStateFailed: {
			replicationv1alpha1.ReplicationStateSyncing,
			replicationv1alpha1.ReplicationStateSource,
			replicationv1alpha1.ReplicationStateReplica,
		},
	}

	allowed, exists := allowedTransitions[oldState]
	if !exists {
		return fmt.Errorf("unknown old state: %s", oldState)
	}

	for _, allowedState := range allowed {
		if newState == allowedState {
			return nil // Transition is allowed
		}
	}

	return fmt.Errorf("invalid state transition from '%s' to '%s'", oldState, newState)
}

// SetupWebhookWithManager sets up the webhook with the manager
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&replicationv1alpha1.UnifiedVolumeReplication{}).
		WithValidator(NewUnifiedVolumeReplicationValidator(mgr.GetClient())).
		Complete()
}
