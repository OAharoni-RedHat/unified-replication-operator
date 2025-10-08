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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

// DellCSIReplicationGroup GVK
var DellCSIReplicationGroupGVK = schema.GroupVersionKind{
	Group:   "replication.dell.com",
	Version: "v1",
	Kind:    "DellCSIReplicationGroup",
}

// PowerStoreAdapter implements the ReplicationAdapter interface for Dell PowerStore
type PowerStoreAdapter struct {
	*BaseAdapter
}

// NewPowerStoreAdapter creates a new PowerStore adapter
func NewPowerStoreAdapter(client client.Client, translator *translation.Engine) (*PowerStoreAdapter, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}

	if translator == nil {
		translator = translation.NewEngine()
	}

	config := DefaultAdapterConfig(translation.BackendPowerStore)
	baseAdapter := NewBaseAdapter(translation.BackendPowerStore, client, translator, config)

	adapter := &PowerStoreAdapter{
		BaseAdapter: baseAdapter,
	}

	return adapter, nil
}

// CreateReplication creates a DellCSIReplicationGroup resource
func (psa *PowerStoreAdapter) CreateReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Creating PowerStore replication group")

	startTime := time.Now()

	// Validate configuration
	if err := psa.ValidateConfiguration(uvr); err != nil {
		psa.updateMetrics("create", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendPowerStore, "create", uvr.Name,
			"configuration validation failed", err)
	}

	// Translate state and mode
	powerstoreState, err := psa.TranslateState(string(uvr.Spec.ReplicationState))
	if err != nil {
		psa.updateMetrics("create", false, startTime)
		return err
	}

	powerstoreMode, err := psa.TranslateMode(string(uvr.Spec.ReplicationMode))
	if err != nil {
		psa.updateMetrics("create", false, startTime)
		return err
	}

	// Create DellCSIReplicationGroup resource
	rg := &unstructured.Unstructured{}
	rg.SetGroupVersionKind(DellCSIReplicationGroupGVK)
	rg.SetName(uvr.Name)
	rg.SetNamespace(uvr.Namespace)

	// Set labels
	labels := map[string]interface{}{
		"app.kubernetes.io/managed-by": "unified-replication-operator",
		"unified-replication.io/name":  uvr.Name,
	}
	rg.SetLabels(convertToStringMap(labels))

	// Build spec
	spec := map[string]interface{}{
		"replicationState":       powerstoreState,
		"protectionGroupName":    fmt.Sprintf("%s-pg", uvr.Name),
		"storageClassName":       uvr.Spec.SourceEndpoint.StorageClass,
		"remoteSystem":           uvr.Spec.DestinationEndpoint.Cluster,
		"remoteStorageClassName": uvr.Spec.DestinationEndpoint.StorageClass,
	}

	// Add mode-specific settings
	if powerstoreMode == "Sync" {
		// Synchronous/Metro replication
		spec["protectionPolicy"] = "Metro"
	} else {
		// Asynchronous replication
		spec["protectionPolicy"] = "Async"
	}

	// Add RPO settings from extensions
	if uvr.Spec.Extensions != nil && uvr.Spec.Extensions.Powerstore != nil {
		if uvr.Spec.Extensions.Powerstore.RpoSettings != nil {
			spec["rpoPolicy"] = *uvr.Spec.Extensions.Powerstore.RpoSettings
		}
		if len(uvr.Spec.Extensions.Powerstore.VolumeGroups) > 0 {
			spec["volumeGroupList"] = uvr.Spec.Extensions.Powerstore.VolumeGroups
		}
	}

	// Add PVC to volume group
	if uvr.Spec.VolumeMapping.Source.PvcName != "" {
		spec["pvcList"] = []interface{}{
			uvr.Spec.VolumeMapping.Source.PvcName,
		}
	}

	if err := unstructured.SetNestedMap(rg.Object, spec, "spec"); err != nil {
		psa.updateMetrics("create", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "create", uvr.Name,
			"failed to build DellCSIReplicationGroup spec", err)
	}

	// Create the resource
	if err := psa.client.Create(ctx, rg); err != nil {
		psa.updateMetrics("create", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "create", uvr.Name,
			"failed to create DellCSIReplicationGroup", err)
	}

	psa.updateMetrics("create", true, startTime)
	logger.Info("Successfully created PowerStore replication group")
	return nil
}

// UpdateReplication updates a DellCSIReplicationGroup resource
func (psa *PowerStoreAdapter) UpdateReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Updating PowerStore replication group")

	startTime := time.Now()

	// Get existing resource
	rg := &unstructured.Unstructured{}
	rg.SetGroupVersionKind(DellCSIReplicationGroupGVK)
	key := client.ObjectKey{Name: uvr.Name, Namespace: uvr.Namespace}

	if err := psa.client.Get(ctx, key, rg); err != nil {
		if errors.IsNotFound(err) {
			// Resource doesn't exist, create it
			return psa.CreateReplication(ctx, uvr)
		}
		psa.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "update", uvr.Name,
			"failed to get DellCSIReplicationGroup", err)
	}

	// Translate new state and mode
	powerstoreState, err := psa.TranslateState(string(uvr.Spec.ReplicationState))
	if err != nil {
		psa.updateMetrics("update", false, startTime)
		return err
	}

	powerstoreMode, err := psa.TranslateMode(string(uvr.Spec.ReplicationMode))
	if err != nil {
		psa.updateMetrics("update", false, startTime)
		return err
	}

	// Update spec
	spec, _, _ := unstructured.NestedMap(rg.Object, "spec")
	if spec == nil {
		spec = make(map[string]interface{})
	}

	spec["replicationState"] = powerstoreState

	// Update protection policy based on mode
	if powerstoreMode == "Sync" {
		spec["protectionPolicy"] = "Metro"
	} else {
		spec["protectionPolicy"] = "Async"
	}

	// Update RPO if provided
	if uvr.Spec.Extensions != nil && uvr.Spec.Extensions.Powerstore != nil {
		if uvr.Spec.Extensions.Powerstore.RpoSettings != nil {
			spec["rpoPolicy"] = *uvr.Spec.Extensions.Powerstore.RpoSettings
		}
	}

	if err := unstructured.SetNestedMap(rg.Object, spec, "spec"); err != nil {
		psa.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "update", uvr.Name,
			"failed to update spec", err)
	}

	// Update the resource
	if err := psa.client.Update(ctx, rg); err != nil {
		psa.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "update", uvr.Name,
			"failed to update DellCSIReplicationGroup", err)
	}

	psa.updateMetrics("update", true, startTime)
	logger.Info("Successfully updated PowerStore replication group")
	return nil
}

// DeleteReplication deletes a DellCSIReplicationGroup resource
func (psa *PowerStoreAdapter) DeleteReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Deleting PowerStore replication group")

	startTime := time.Now()

	rg := &unstructured.Unstructured{}
	rg.SetGroupVersionKind(DellCSIReplicationGroupGVK)
	rg.SetName(uvr.Name)
	rg.SetNamespace(uvr.Namespace)

	if err := psa.client.Delete(ctx, rg); err != nil {
		if errors.IsNotFound(err) {
			// Already deleted, success
			logger.Info("DellCSIReplicationGroup already deleted")
			psa.updateMetrics("delete", true, startTime)
			return nil
		}
		psa.updateMetrics("delete", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "delete", uvr.Name,
			"failed to delete DellCSIReplicationGroup", err)
	}

	psa.updateMetrics("delete", true, startTime)
	logger.Info("Successfully deleted PowerStore replication group")
	return nil
}

// GetReplicationStatus gets the status of a DellCSIReplicationGroup
func (psa *PowerStoreAdapter) GetReplicationStatus(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) (*ReplicationStatus, error) {
	logger := log.FromContext(ctx).WithName("powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.V(1).Info("Getting PowerStore replication group status")

	startTime := time.Now()

	rg := &unstructured.Unstructured{}
	rg.SetGroupVersionKind(DellCSIReplicationGroupGVK)
	key := client.ObjectKey{Name: uvr.Name, Namespace: uvr.Namespace}

	if err := psa.client.Get(ctx, key, rg); err != nil {
		if errors.IsNotFound(err) {
			psa.updateMetrics("status", false, startTime)
			return nil, NewAdapterError(ErrorTypeResource, translation.BackendPowerStore, "status", uvr.Name,
				"DellCSIReplicationGroup not found")
		}
		psa.updateMetrics("status", false, startTime)
		return nil, NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "status", uvr.Name,
			"failed to get DellCSIReplicationGroup", err)
	}

	// Extract status
	statusMap, found, err := unstructured.NestedMap(rg.Object, "status")
	if err != nil || !found {
		psa.updateMetrics("status", false, startTime)
		return nil, NewAdapterError(ErrorTypeOperation, translation.BackendPowerStore, "status", uvr.Name,
			"status not available yet")
	}

	// Get state
	powerstoreState, _, _ := unstructured.NestedString(statusMap, "state")
	if powerstoreState == "" {
		powerstoreState, _, _ = unstructured.NestedString(rg.Object, "spec", "replicationState")
	}

	// Translate back to unified
	unifiedState, err := psa.TranslateBackendState(powerstoreState)
	if err != nil {
		unifiedState = powerstoreState
	}

	// Get mode from protection policy
	protectionPolicy, _, _ := unstructured.NestedString(rg.Object, "spec", "protectionPolicy")
	unifiedMode := "asynchronous"
	if protectionPolicy == "Metro" {
		unifiedMode = "synchronous"
	}

	// Determine health
	health := ReplicationHealthHealthy
	replicationStatus, _, _ := unstructured.NestedString(statusMap, "replicationLinkState")
	switch replicationStatus {
	case "Synchronized":
		health = ReplicationHealthHealthy
	case "Synchronizing":
		health = ReplicationHealthDegraded
	case "Failed", "Error":
		health = ReplicationHealthUnhealthy
	default:
		health = ReplicationHealthUnknown
	}

	// Get sync information
	var lastSyncTime *time.Time
	lastSyncStr, found, _ := unstructured.NestedString(statusMap, "lastSyncTime")
	if found && lastSyncStr != "" {
		if t, err := time.Parse(time.RFC3339, lastSyncStr); err == nil {
			lastSyncTime = &t
		}
	}

	// Get sync progress
	var syncProgress *SyncProgress
	syncPercent, found, _ := unstructured.NestedFloat64(statusMap, "syncProgress")
	if found {
		syncProgress = &SyncProgress{
			PercentComplete: syncPercent,
		}
	}

	// Build status
	status := &ReplicationStatus{
		State:              unifiedState,
		Mode:               unifiedMode,
		Health:             health,
		LastSyncTime:       lastSyncTime,
		SyncProgress:       syncProgress,
		ObservedGeneration: uvr.Generation,
		BackendSpecific:    statusMap,
	}

	psa.updateMetrics("status", true, startTime)
	return status, nil
}

// PromoteReplica promotes a replica to source (failover)
func (psa *PowerStoreAdapter) PromoteReplica(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Promoting PowerStore replica (failover)")

	// Update state to active/source
	uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStateSource
	return psa.UpdateReplication(ctx, uvr)
}

// DemoteSource demotes a source to replica (failback)
func (psa *PowerStoreAdapter) DemoteSource(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Demoting PowerStore source (failback)")

	// Update state to passive/replica
	uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStateReplica
	return psa.UpdateReplication(ctx, uvr)
}

// ResyncReplication triggers a resync operation
func (psa *PowerStoreAdapter) ResyncReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Resyncing PowerStore replication group")

	// For PowerStore, resync is done by updating to syncing state then back to replica
	// Get current resource
	rg := &unstructured.Unstructured{}
	rg.SetGroupVersionKind(DellCSIReplicationGroupGVK)
	key := client.ObjectKey{Name: uvr.Name, Namespace: uvr.Namespace}

	if err := psa.client.Get(ctx, key, rg); err != nil {
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "resync", uvr.Name,
			"failed to get DellCSIReplicationGroup", err)
	}

	// Update annotation to trigger resync
	annotations := rg.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["replication.dell.com/resync-requested"] = time.Now().Format(time.RFC3339)
	rg.SetAnnotations(annotations)

	if err := psa.client.Update(ctx, rg); err != nil {
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "resync", uvr.Name,
			"failed to trigger resync", err)
	}

	logger.Info("Successfully triggered resync operation")
	return nil
}

// PauseReplication pauses replication
func (psa *PowerStoreAdapter) PauseReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Pausing PowerStore replication")

	rg := &unstructured.Unstructured{}
	rg.SetGroupVersionKind(DellCSIReplicationGroupGVK)
	key := client.ObjectKey{Name: uvr.Name, Namespace: uvr.Namespace}

	if err := psa.client.Get(ctx, key, rg); err != nil {
		return err
	}

	spec, _, _ := unstructured.NestedMap(rg.Object, "spec")
	if spec == nil {
		spec = make(map[string]interface{})
	}
	spec["action"] = "Pause"

	if err := unstructured.SetNestedMap(rg.Object, spec, "spec"); err != nil {
		return err
	}

	return psa.client.Update(ctx, rg)
}

// ResumeReplication resumes paused replication
func (psa *PowerStoreAdapter) ResumeReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Resuming PowerStore replication")

	rg := &unstructured.Unstructured{}
	rg.SetGroupVersionKind(DellCSIReplicationGroupGVK)
	key := client.ObjectKey{Name: uvr.Name, Namespace: uvr.Namespace}

	if err := psa.client.Get(ctx, key, rg); err != nil {
		return err
	}

	spec, _, _ := unstructured.NestedMap(rg.Object, "spec")
	if spec == nil {
		spec = make(map[string]interface{})
	}
	spec["action"] = "Resume"

	if err := unstructured.SetNestedMap(rg.Object, spec, "spec"); err != nil {
		return err
	}

	return psa.client.Update(ctx, rg)
}
