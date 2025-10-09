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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

// TridentMirrorRelationship GVK
var TridentMirrorRelationshipGVK = schema.GroupVersionKind{
	Group:   "trident.netapp.io",
	Version: "v1",
	Kind:    "TridentMirrorRelationship",
}

// TridentActionMirrorUpdate GVK
var TridentActionMirrorUpdateGVK = schema.GroupVersionKind{
	Group:   "trident.netapp.io",
	Version: "v1",
	Kind:    "TridentActionMirrorUpdate",
}

// TridentAdapter implements the ReplicationAdapter interface for NetApp Trident
type TridentAdapter struct {
	*BaseAdapter
}

// NewTridentAdapter creates a new Trident adapter
func NewTridentAdapter(client client.Client, translator *translation.Engine) (*TridentAdapter, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}

	if translator == nil {
		translator = translation.NewEngine()
	}

	config := DefaultAdapterConfig(translation.BackendTrident)
	baseAdapter := NewBaseAdapter(translation.BackendTrident, client, translator, config)

	adapter := &TridentAdapter{
		BaseAdapter: baseAdapter,
	}

	return adapter, nil
}

// EnsureReplication ensures the TridentMirrorRelationship is in the desired state (idempotent)
func (ta *TridentAdapter) EnsureReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Ensuring Trident mirror relationship is in desired state")

	startTime := time.Now()

	// Validate configuration
	if err := ta.ValidateConfiguration(uvr); err != nil {
		ta.updateMetrics("ensure", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendTrident, "ensure", uvr.Name, "configuration validation failed", err)
	}

	// Check if TridentMirrorRelationship exists
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(TridentMirrorRelationshipGVK)
	err := ta.client.Get(ctx, types.NamespacedName{
		Name:      uvr.Name,
		Namespace: uvr.Namespace,
	}, existing)

	if err != nil {
		if errors.IsNotFound(err) {
			// Resource doesn't exist, create it
			logger.Info("TridentMirrorRelationship not found, creating")
			return ta.createTridentMirrorRelationship(ctx, uvr, startTime)
		}
		// Some other error
		ta.updateMetrics("ensure", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendTrident, "ensure", uvr.Name, "failed to check existing TridentMirrorRelationship", err)
	}

	// Resource exists, update it
	logger.V(1).Info("TridentMirrorRelationship exists, updating if needed")
	return ta.updateTridentMirrorRelationship(ctx, uvr, existing, startTime)
}

// createTridentMirrorRelationship creates a new TridentMirrorRelationship resource
func (ta *TridentAdapter) createTridentMirrorRelationship(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication, startTime time.Time) error {
	logger := log.FromContext(ctx).WithName("trident-adapter").WithValues("uvr", uvr.Name)

	// Translate state and mode
	tridentState, err := ta.TranslateState(string(uvr.Spec.ReplicationState))
	if err != nil {
		ta.updateMetrics("create", false, startTime)
		return err
	}

	tridentMode, err := ta.TranslateMode(string(uvr.Spec.ReplicationMode))
	if err != nil {
		ta.updateMetrics("create", false, startTime)
		return err
	}

	// Create TridentMirrorRelationship resource
	tmr := &unstructured.Unstructured{}
	tmr.SetGroupVersionKind(TridentMirrorRelationshipGVK)
	tmr.SetName(uvr.Name)
	tmr.SetNamespace(uvr.Namespace)

	// Set labels for tracking
	labels := map[string]interface{}{
		"app.kubernetes.io/managed-by": "unified-replication-operator",
		"unified-replication.io/name":  uvr.Name,
	}
	tmr.SetLabels(convertToStringMap(labels))

	// Build spec
	spec := map[string]interface{}{
		"state":               tridentState,
		"replicationPolicy":   tridentMode,
		"volumeGroupName":     fmt.Sprintf("%s-vg", uvr.Name),
		"localPVCName":        uvr.Spec.VolumeMapping.Source.PvcName,
		"localVolumeHandle":   "", // Will be discovered
		"remoteVolumeHandle":  uvr.Spec.VolumeMapping.Destination.VolumeHandle,
		"replicationSchedule": uvr.Spec.Schedule.Rpo,
	}

	// Add Trident-specific extensions if provided
	if uvr.Spec.Extensions != nil && uvr.Spec.Extensions.Trident != nil {
		if len(uvr.Spec.Extensions.Trident.Actions) > 0 {
			actions := make([]interface{}, 0, len(uvr.Spec.Extensions.Trident.Actions))
			for _, action := range uvr.Spec.Extensions.Trident.Actions {
				actions = append(actions, map[string]interface{}{
					"type":           action.Type,
					"snapshotHandle": action.SnapshotHandle,
				})
			}
			spec["pendingActions"] = actions
		}
	}

	if err := unstructured.SetNestedMap(tmr.Object, spec, "spec"); err != nil {
		ta.updateMetrics("create", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "create", uvr.Name,
			"failed to build TridentMirrorRelationship spec", err)
	}

	// Create the resource
	if err := ta.client.Create(ctx, tmr); err != nil {
		ta.updateMetrics("create", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "create", uvr.Name,
			"failed to create TridentMirrorRelationship", err)
	}

	ta.updateMetrics("create", true, startTime)
	logger.Info("Successfully created Trident mirror relationship")
	return nil
}

// updateTridentMirrorRelationship updates an existing TridentMirrorRelationship resource
func (ta *TridentAdapter) updateTridentMirrorRelationship(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication, existing *unstructured.Unstructured, startTime time.Time) error {
	logger := log.FromContext(ctx).WithName("trident-adapter").WithValues("uvr", uvr.Name)

	// Translate state and mode
	tridentState, err := ta.TranslateState(string(uvr.Spec.ReplicationState))
	if err != nil {
		ta.updateMetrics("update", false, startTime)
		return err
	}

	tridentMode, err := ta.TranslateMode(string(uvr.Spec.ReplicationMode))
	if err != nil {
		ta.updateMetrics("update", false, startTime)
		return err
	}

	// Update spec fields
	spec := map[string]interface{}{
		"state":               tridentState,
		"replicationPolicy":   tridentMode,
		"volumeGroupName":     fmt.Sprintf("%s-vg", uvr.Name),
		"localPVCName":        uvr.Spec.VolumeMapping.Source.PvcName,
		"localVolumeHandle":   "", // Will be discovered
		"remoteVolumeHandle":  uvr.Spec.VolumeMapping.Destination.VolumeHandle,
		"replicationSchedule": uvr.Spec.Schedule.Rpo,
	}

	// Add Trident-specific extensions if provided
	if uvr.Spec.Extensions != nil && uvr.Spec.Extensions.Trident != nil {
		if len(uvr.Spec.Extensions.Trident.Actions) > 0 {
			actions := make([]interface{}, 0, len(uvr.Spec.Extensions.Trident.Actions))
			for _, action := range uvr.Spec.Extensions.Trident.Actions {
				actions = append(actions, map[string]interface{}{
					"type":           action.Type,
					"snapshotHandle": action.SnapshotHandle,
				})
			}
			spec["pendingActions"] = actions
		}
	}

	if err := unstructured.SetNestedMap(existing.Object, spec, "spec"); err != nil {
		ta.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "update", uvr.Name,
			"failed to update TridentMirrorRelationship spec", err)
	}

	// Update the resource
	if err := ta.client.Update(ctx, existing); err != nil {
		ta.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "update", uvr.Name,
			"failed to update TridentMirrorRelationship", err)
	}

	ta.updateMetrics("update", true, startTime)
	logger.Info("Successfully updated Trident mirror relationship")
	return nil
}

// updateMetrics is a helper that delegates to BaseAdapter
func (ta *TridentAdapter) updateMetrics(operation string, success bool, startTime time.Time) {
	ta.BaseAdapter.updateMetrics(operation, success, startTime)
}

// DeleteReplication deletes a TridentMirrorRelationship resource
func (ta *TridentAdapter) DeleteReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Deleting Trident mirror relationship")

	startTime := time.Now()

	tmr := &unstructured.Unstructured{}
	tmr.SetGroupVersionKind(TridentMirrorRelationshipGVK)
	tmr.SetName(uvr.Name)
	tmr.SetNamespace(uvr.Namespace)

	if err := ta.client.Delete(ctx, tmr); err != nil {
		if errors.IsNotFound(err) {
			// Already deleted, success
			logger.Info("TridentMirrorRelationship already deleted")
			ta.updateMetrics("delete", true, startTime)
			return nil
		}
		ta.updateMetrics("delete", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "delete", uvr.Name,
			"failed to delete TridentMirrorRelationship", err)
	}

	ta.updateMetrics("delete", true, startTime)
	logger.Info("Successfully deleted Trident mirror relationship")
	return nil
}

// GetReplicationStatus gets the status of a TridentMirrorRelationship
func (ta *TridentAdapter) GetReplicationStatus(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) (*ReplicationStatus, error) {
	logger := log.FromContext(ctx).WithName("trident-adapter").WithValues("uvr", uvr.Name)
	logger.V(1).Info("Getting Trident mirror relationship status")

	startTime := time.Now()

	tmr := &unstructured.Unstructured{}
	tmr.SetGroupVersionKind(TridentMirrorRelationshipGVK)
	key := client.ObjectKey{Name: uvr.Name, Namespace: uvr.Namespace}

	if err := ta.client.Get(ctx, key, tmr); err != nil {
		if errors.IsNotFound(err) {
			ta.updateMetrics("status", false, startTime)
			return nil, NewAdapterError(ErrorTypeResource, translation.BackendTrident, "status", uvr.Name,
				"TridentMirrorRelationship not found")
		}
		ta.updateMetrics("status", false, startTime)
		return nil, NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "status", uvr.Name,
			"failed to get TridentMirrorRelationship", err)
	}

	// Extract status
	statusMap, found, err := unstructured.NestedMap(tmr.Object, "status")
	if err != nil || !found {
		ta.updateMetrics("status", false, startTime)
		return nil, NewAdapterError(ErrorTypeOperation, translation.BackendTrident, "status", uvr.Name,
			"status not available yet")
	}

	// Get state
	tridentState, _, _ := unstructured.NestedString(statusMap, "state")
	if tridentState == "" {
		tridentState, _, _ = unstructured.NestedString(tmr.Object, "spec", "state")
	}

	// Translate back to unified
	unifiedState, err := ta.TranslateBackendState(tridentState)
	if err != nil {
		unifiedState = tridentState // Use as-is if translation fails
	}

	// Get mode
	tridentMode, _, _ := unstructured.NestedString(tmr.Object, "spec", "replicationPolicy")
	unifiedMode, err := ta.TranslateBackendMode(tridentMode)
	if err != nil {
		unifiedMode = tridentMode
	}

	// Determine health
	health := ReplicationHealthHealthy
	conditions, _, _ := unstructured.NestedSlice(statusMap, "conditions")
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _, _ := unstructured.NestedString(condMap, "type")
		condStatus, _, _ := unstructured.NestedString(condMap, "status")

		if condType == "Ready" && condStatus != "True" {
			health = ReplicationHealthDegraded
		}
		if condType == "Error" {
			health = ReplicationHealthUnhealthy
		}
	}

	// Get last sync time
	var lastSyncTime *time.Time
	lastSyncStr, found, _ := unstructured.NestedString(statusMap, "lastTransferTime")
	if found && lastSyncStr != "" {
		if t, err := time.Parse(time.RFC3339, lastSyncStr); err == nil {
			lastSyncTime = &t
		}
	}

	// Build status
	status := &ReplicationStatus{
		State:              unifiedState,
		Mode:               unifiedMode,
		Health:             health,
		LastSyncTime:       lastSyncTime,
		ObservedGeneration: uvr.Generation,
		BackendSpecific:    statusMap,
	}

	ta.updateMetrics("status", true, startTime)
	return status, nil
}

// PromoteReplica promotes a replica to source
func (ta *TridentAdapter) PromoteReplica(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Promoting Trident replica")

	// For Trident, promotion is done by updating state to "established" (source)
	uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStateSource
	return ta.EnsureReplication(ctx, uvr)
}

// DemoteSource demotes a source to replica
func (ta *TridentAdapter) DemoteSource(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Demoting Trident source")

	// For Trident, demotion is done by updating state to "snapmirrored" (replica)
	uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStateReplica
	return ta.EnsureReplication(ctx, uvr)
}

// ResyncReplication triggers a resync operation
func (ta *TridentAdapter) ResyncReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Resyncing Trident mirror relationship")

	// Create TridentActionMirrorUpdate for resync
	action := &unstructured.Unstructured{}
	action.SetGroupVersionKind(TridentActionMirrorUpdateGVK)
	action.SetName(fmt.Sprintf("%s-resync-%d", uvr.Name, time.Now().Unix()))
	action.SetNamespace(uvr.Namespace)

	spec := map[string]interface{}{
		"mirrorRelationshipName": uvr.Name,
		"snapshotHandle":         "", // Latest snapshot
	}

	if err := unstructured.SetNestedMap(action.Object, spec, "spec"); err != nil {
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "resync", uvr.Name,
			"failed to build action spec", err)
	}

	if err := ta.client.Create(ctx, action); err != nil {
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "resync", uvr.Name,
			"failed to create resync action", err)
	}

	logger.Info("Successfully triggered resync action")
	return nil
}

// Helper functions

func convertToStringMap(m map[string]interface{}) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}
