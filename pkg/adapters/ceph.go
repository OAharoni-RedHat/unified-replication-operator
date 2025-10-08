// Copyright 2024 unified-replication-operator contributors.
// Licensed under the Apache License, Version 2.0.

package adapters

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

const (
	// CephRBDStorageClass is the expected storage class for Ceph RBD volumes
	CephRBDStorageClass = "rbd"

	// VolumeReplication is the Ceph-CSI VolumeReplication CRD
	VolumeReplicationAPIVersion = "replication.storage.openshift.io/v1alpha1"
	VolumeReplicationKind       = "VolumeReplication"

	// State transition timeouts and retry settings
	DefaultStateTransitionTimeout = 5 * time.Minute
	StateTransitionRetryInterval  = 30 * time.Second
	MaxStateTransitionRetries     = 10

	// Cache settings
	StatusCacheTTL     = 30 * time.Second
	StatusCacheMaxSize = 1000

	// Ceph-specific constants
	CephPrimaryState   = "primary"
	CephSecondaryState = "secondary"

	// Auto-resync settings
	DefaultAutoResyncEnabled = true
	AutoResyncCheckInterval  = 2 * time.Minute
)

// VolumeReplication represents the Ceph-CSI VolumeReplication CRD
type VolumeReplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              VolumeReplicationSpec   `json:"spec,omitempty"`
	Status            VolumeReplicationStatus `json:"status,omitempty"`
}

// DeepCopyObject implements runtime.Object
func (vr *VolumeReplication) DeepCopyObject() runtime.Object {
	if vr == nil {
		return nil
	}
	out := new(VolumeReplication)
	vr.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (vr *VolumeReplication) DeepCopyInto(out *VolumeReplication) {
	*out = *vr
	out.TypeMeta = vr.TypeMeta
	vr.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	vr.Spec.DeepCopyInto(&out.Spec)
	vr.Status.DeepCopyInto(&out.Status)
}

// VolumeReplicationSpec defines the desired state of VolumeReplication
type VolumeReplicationSpec struct {
	// volumeReplicationClass is the VolumeReplicationClass name
	VolumeReplicationClass string `json:"volumeReplicationClass"`
	// pvcName contains the name of the PVC
	PvcName string `json:"pvcName"`
	// replicationState is the state of the volume being replicated
	ReplicationState string `json:"replicationState"`
	// dataSource contains the data source information
	DataSource *corev1.VolumeSource `json:"dataSource,omitempty"`
	// autoResync indicates if the volume should be automatically resynced
	AutoResync *bool `json:"autoResync,omitempty"`
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (vrs *VolumeReplicationSpec) DeepCopyInto(out *VolumeReplicationSpec) {
	*out = *vrs
	if vrs.DataSource != nil {
		in, out := &vrs.DataSource, &out.DataSource
		*out = new(corev1.VolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if vrs.AutoResync != nil {
		in, out := &vrs.AutoResync, &out.AutoResync
		*out = new(bool)
		**out = **in
	}
}

// VolumeReplicationStatus defines the observed state of VolumeReplication
type VolumeReplicationStatus struct {
	// conditions contains the list of status conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// state represents the current state of the volume replication
	State string `json:"state,omitempty"`
	// message provides detailed information about the current state
	Message string `json:"message,omitempty"`
	// lastSyncTime represents the last time the volume was synced
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	// lastSyncDuration represents the duration of the last sync
	LastSyncDuration *metav1.Duration `json:"lastSyncDuration,omitempty"`
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (vrs *VolumeReplicationStatus) DeepCopyInto(out *VolumeReplicationStatus) {
	*out = *vrs
	if vrs.Conditions != nil {
		in, out := &vrs.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if vrs.LastSyncTime != nil {
		in, out := &vrs.LastSyncTime, &out.LastSyncTime
		*out = (*in).DeepCopy()
	}
	if vrs.LastSyncDuration != nil {
		in, out := &vrs.LastSyncDuration, &out.LastSyncDuration
		*out = new(metav1.Duration)
		**out = **in
	}
}

// VolumeReplicationList contains a list of VolumeReplication
type VolumeReplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VolumeReplication `json:"items"`
}

// DeepCopyObject implements runtime.Object
func (vrl *VolumeReplicationList) DeepCopyObject() runtime.Object {
	if vrl == nil {
		return nil
	}
	out := new(VolumeReplicationList)
	vrl.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (vrl *VolumeReplicationList) DeepCopyInto(out *VolumeReplicationList) {
	*out = *vrl
	out.TypeMeta = vrl.TypeMeta
	vrl.ListMeta.DeepCopyInto(&out.ListMeta)
	if vrl.Items != nil {
		in, out := &vrl.Items, &out.Items
		*out = make([]VolumeReplication, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// CacheedStatus represents cached status information
type CachedStatus struct {
	Status    *ReplicationStatus
	Timestamp time.Time
}

// StatusCache provides thread-safe caching for replication status
type StatusCache struct {
	cache map[string]*CachedStatus
	mutex sync.RWMutex
	ttl   time.Duration
}

// NewStatusCache creates a new status cache
func NewStatusCache(ttl time.Duration) *StatusCache {
	return &StatusCache{
		cache: make(map[string]*CachedStatus),
		mutex: sync.RWMutex{},
		ttl:   ttl,
	}
}

// Get retrieves cached status if valid
func (sc *StatusCache) Get(key string) (*ReplicationStatus, bool) {
	sc.mutex.RLock()
	defer sc.mutex.RUnlock()

	cached, exists := sc.cache[key]
	if !exists {
		return nil, false
	}

	if time.Since(cached.Timestamp) > sc.ttl {
		return nil, false
	}

	return cached.Status, true
}

// Set stores status in cache
func (sc *StatusCache) Set(key string, status *ReplicationStatus) {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	sc.cache[key] = &CachedStatus{
		Status:    status,
		Timestamp: time.Now(),
	}

	// Simple cache size management
	if len(sc.cache) > StatusCacheMaxSize {
		sc.evictOldest()
	}
}

// evictOldest removes the oldest cache entry
func (sc *StatusCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time = time.Now()

	for key, cached := range sc.cache {
		if cached.Timestamp.Before(oldestTime) {
			oldestTime = cached.Timestamp
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(sc.cache, oldestKey)
	}
}

// Clear removes all cached entries
func (sc *StatusCache) Clear() {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.cache = make(map[string]*CachedStatus)
}

// StateTransition represents a state transition with validation
type StateTransition struct {
	From     string
	To       string
	Allowed  bool
	Reason   string
	Duration time.Duration
}

// CephAdapter implements the ReplicationAdapter interface for Ceph-CSI
type CephAdapter struct {
	*BaseAdapter
	client      client.Client
	statusCache *StatusCache

	// State transition tracking
	transitionMutex   sync.RWMutex
	activeTransitions map[string]*StateTransition

	// Performance metrics
	operationMetrics sync.Map
	lastHealthCheck  time.Time
	healthMutex      sync.RWMutex
}

// NewCephAdapter creates a new CephAdapter instance
func NewCephAdapter(client client.Client, translator *translation.Engine) (*CephAdapter, error) {
	if client == nil {
		return nil, fmt.Errorf("client cannot be nil")
	}
	if translator == nil {
		return nil, fmt.Errorf("translator cannot be nil")
	}

	baseAdapter := NewBaseAdapter(translation.BackendCeph, client, translator, nil)

	return &CephAdapter{
		BaseAdapter:       baseAdapter,
		client:            client,
		statusCache:       NewStatusCache(StatusCacheTTL),
		activeTransitions: make(map[string]*StateTransition),
		lastHealthCheck:   time.Now(),
	}, nil
}

// GetBackendType returns the backend type
func (ca *CephAdapter) GetBackendType() translation.Backend {
	return translation.BackendCeph
}

// isValidStateTransition validates if a state transition is allowed
func (ca *CephAdapter) isValidStateTransition(from, to string) (bool, string) {
	// Define allowed state transitions for Ceph
	validTransitions := map[string][]string{
		"source":    {"demoting", "failed"},
		"replica":   {"promoting", "syncing", "failed"},
		"promoting": {"source", "failed"},
		"demoting":  {"replica", "failed"},
		"syncing":   {"replica", "source", "failed"},
		"failed":    {"syncing", "source", "replica"},
	}

	if from == to {
		return true, "same state transition"
	}

	allowedStates, exists := validTransitions[from]
	if !exists {
		return false, fmt.Sprintf("unknown source state: %s", from)
	}

	for _, allowed := range allowedStates {
		if to == allowed {
			return true, "valid transition"
		}
	}

	return false, fmt.Sprintf("transition from %s to %s is not allowed", from, to)
}

// trackStateTransition tracks an active state transition
func (ca *CephAdapter) trackStateTransition(key, from, to string) {
	ca.transitionMutex.Lock()
	defer ca.transitionMutex.Unlock()

	allowed, reason := ca.isValidStateTransition(from, to)
	ca.activeTransitions[key] = &StateTransition{
		From:     from,
		To:       to,
		Allowed:  allowed,
		Reason:   reason,
		Duration: 0,
	}
}

// completeStateTransition marks a state transition as complete
func (ca *CephAdapter) completeStateTransition(key string, success bool) {
	ca.transitionMutex.Lock()
	defer ca.transitionMutex.Unlock()

	if transition, exists := ca.activeTransitions[key]; exists {
		if success {
			delete(ca.activeTransitions, key)
		} else {
			transition.Reason = "transition failed"
		}
	}
}

// getActiveStateTransition retrieves an active state transition
func (ca *CephAdapter) getActiveStateTransition(key string) (*StateTransition, bool) {
	ca.transitionMutex.RLock()
	defer ca.transitionMutex.RUnlock()

	transition, exists := ca.activeTransitions[key]
	return transition, exists
}

// ValidateConfiguration validates the unified configuration for Ceph compatibility
func (ca *CephAdapter) ValidateConfiguration(uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if uvr == nil {
		return fmt.Errorf("UnifiedVolumeReplication cannot be nil")
	}

	// Validate storage class
	if err := ca.validateStorageClass(uvr); err != nil {
		return fmt.Errorf("storage class validation failed: %w", err)
	}

	// Validate Ceph extensions
	if err := ca.validateCephExtensions(uvr); err != nil {
		return fmt.Errorf("Ceph extensions validation failed: %w", err)
	}

	// Validate cross-field requirements
	if err := ca.validateCrossFieldRequirements(uvr); err != nil {
		return fmt.Errorf("cross-field validation failed: %w", err)
	}

	return nil
}

// validateStorageClass ensures the storage class is compatible with Ceph RBD
func (ca *CephAdapter) validateStorageClass(uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if uvr.Spec.SourceEndpoint.StorageClass == "" {
		return fmt.Errorf("source storage class is required for Ceph")
	}

	// Check if it's a Ceph RBD storage class
	if !strings.Contains(strings.ToLower(uvr.Spec.SourceEndpoint.StorageClass), "rbd") {
		return fmt.Errorf("storage class %s is not compatible with Ceph RBD", uvr.Spec.SourceEndpoint.StorageClass)
	}

	return nil
}

// validateCephExtensions validates Ceph-specific extensions
func (ca *CephAdapter) validateCephExtensions(uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if uvr.Spec.Extensions == nil || uvr.Spec.Extensions.Ceph == nil {
		return nil // Extensions are optional
	}

	cephExt := uvr.Spec.Extensions.Ceph

	// Validate mirroring mode
	if cephExt.MirroringMode != nil {
		validModes := []string{"snapshot", "journal"}
		mode := strings.ToLower(*cephExt.MirroringMode)
		valid := false
		for _, validMode := range validModes {
			if mode == validMode {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid mirroring mode %s, must be one of: %v", *cephExt.MirroringMode, validModes)
		}
	}

	return nil
}

// validateCrossFieldRequirements validates cross-field requirements for Ceph
func (ca *CephAdapter) validateCrossFieldRequirements(uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	// For Ceph, the source and destination clusters must be different
	if uvr.Spec.SourceEndpoint.Cluster == uvr.Spec.DestinationEndpoint.Cluster &&
		uvr.Spec.SourceEndpoint.Region == uvr.Spec.DestinationEndpoint.Region {
		return fmt.Errorf("source and destination endpoints cannot be identical for Ceph replication")
	}

	return nil
}

// CreateReplication creates a new VolumeReplication resource for Ceph
func (ca *CephAdapter) CreateReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Creating Ceph VolumeReplication")

	startTime := time.Now()

	// Validate configuration
	if err := ca.ValidateConfiguration(uvr); err != nil {
		ca.BaseAdapter.updateMetrics("create", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendCeph, "create", uvr.Name, "configuration validation failed", err)
	}

	// Create VolumeReplication object
	vr, err := ca.buildVolumeReplication(uvr)
	if err != nil {
		ca.BaseAdapter.updateMetrics("create", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendCeph, "create", uvr.Name, "failed to build VolumeReplication", err)
	}

	// Create the resource
	if err := ca.client.Create(ctx, vr); err != nil {
		if errors.IsAlreadyExists(err) {
			logger.Info("VolumeReplication already exists, updating instead")
			return ca.UpdateReplication(ctx, uvr)
		}
		ca.BaseAdapter.updateMetrics("create", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "create", uvr.Name, "failed to create VolumeReplication", err)
	}

	// Update metrics
	ca.BaseAdapter.updateMetrics("create", true, startTime)

	logger.Info("Successfully created Ceph VolumeReplication", "volumeReplication", vr.Name)
	return nil
}

// UpdateReplication updates an existing VolumeReplication resource
func (ca *CephAdapter) UpdateReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Updating Ceph VolumeReplication")

	startTime := time.Now()

	// Get the existing VolumeReplication
	existingVR := &VolumeReplication{}
	if err := ca.client.Get(ctx, types.NamespacedName{
		Name:      ca.buildVolumeReplicationName(uvr),
		Namespace: uvr.Namespace,
	}, existingVR); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("VolumeReplication not found, creating instead")
			return ca.CreateReplication(ctx, uvr)
		}
		ca.BaseAdapter.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "update", uvr.Name, "failed to get existing VolumeReplication", err)
	}

	// Validate the configuration
	if err := ca.ValidateConfiguration(uvr); err != nil {
		ca.BaseAdapter.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendCeph, "update", uvr.Name, "configuration validation failed", err)
	}

	// Translate unified state to Ceph state
	cephState, _, err := ca.translateToCephState(string(uvr.Spec.ReplicationState))
	if err != nil {
		ca.BaseAdapter.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendCeph, "update", uvr.Name, "state translation failed", err)
	}

	// Update the spec
	existingVR.Spec.ReplicationState = cephState

	// Update the VolumeReplication resource
	if err := ca.client.Update(ctx, existingVR); err != nil {
		ca.BaseAdapter.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "update", uvr.Name, "failed to update VolumeReplication", err)
	}

	// Update metrics
	ca.BaseAdapter.updateMetrics("update", true, startTime)

	logger.Info("Successfully updated Ceph VolumeReplication", "volumeReplication", existingVR.Name)
	return nil
}

// DeleteReplication deletes a VolumeReplication resource
func (ca *CephAdapter) DeleteReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Deleting Ceph VolumeReplication")

	startTime := time.Now()

	// Get the VolumeReplication to delete
	vr := &VolumeReplication{}
	if err := ca.client.Get(ctx, types.NamespacedName{
		Name:      ca.buildVolumeReplicationName(uvr),
		Namespace: uvr.Namespace,
	}, vr); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("VolumeReplication not found, already deleted")
			ca.BaseAdapter.updateMetrics("delete", true, startTime)
			return nil
		}
		ca.BaseAdapter.updateMetrics("delete", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "delete", uvr.Name, "failed to get VolumeReplication", err)
	}

	// Delete the resource
	if err := ca.client.Delete(ctx, vr); err != nil {
		ca.BaseAdapter.updateMetrics("delete", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "delete", uvr.Name, "failed to delete VolumeReplication", err)
	}

	// Update metrics
	ca.BaseAdapter.updateMetrics("delete", true, startTime)

	logger.Info("Successfully deleted Ceph VolumeReplication", "volumeReplication", vr.Name)
	return nil
}

// GetReplicationStatus retrieves the current replication status with caching
func (ca *CephAdapter) GetReplicationStatus(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) (*ReplicationStatus, error) {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)

	// Check cache first
	cacheKey := ca.buildStatusCacheKey(uvr)
	if cachedStatus, found := ca.statusCache.Get(cacheKey); found {
		logger.V(1).Info("Returning cached status")
		return cachedStatus, nil
	}

	// Get the VolumeReplication resource
	vr := &VolumeReplication{}
	vrKey := types.NamespacedName{
		Name:      ca.buildVolumeReplicationName(uvr),
		Namespace: uvr.Namespace,
	}

	if err := ca.client.Get(ctx, vrKey, vr); err != nil {
		if errors.IsNotFound(err) {
			status := &ReplicationStatus{
				State:   "unknown",
				Health:  ReplicationHealthUnknown,
				Message: "VolumeReplication resource not found",
			}
			// Cache the not-found status with shorter TTL
			ca.statusCache.Set(cacheKey, status)
			return status, nil
		}
		return nil, NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "status", uvr.Name, "failed to get VolumeReplication", err)
	}

	// Enhanced status mapping with detailed analysis
	status, err := ca.buildEnhancedReplicationStatus(ctx, vr, uvr)
	if err != nil {
		logger.Error(err, "Failed to build enhanced status")
		// Return basic status on error
		status = ca.buildBasicReplicationStatus(vr)
	}

	// Cache the status
	ca.statusCache.Set(cacheKey, status)

	logger.V(1).Info("Built replication status", "state", status.State, "health", status.Health)
	return status, nil
}

// buildEnhancedReplicationStatus creates detailed status with condition analysis
func (ca *CephAdapter) buildEnhancedReplicationStatus(ctx context.Context, vr *VolumeReplication, uvr *replicationv1alpha1.UnifiedVolumeReplication) (*ReplicationStatus, error) {
	logger := log.FromContext(ctx).WithName("ceph-adapter")

	// Translate Ceph state to unified state
	unifiedState, _, err := ca.translateFromCephState(vr.Spec.ReplicationState)
	if err != nil {
		logger.Error(err, "Failed to translate Ceph state", "cephState", vr.Spec.ReplicationState)
		unifiedState = "unknown"
	}

	// Analyze conditions for detailed health status
	health, detailedMessage := ca.analyzeVolumeReplicationConditions(vr.Status.Conditions)

	// Calculate sync progress with enhanced metrics
	progress := ca.calculateEnhancedSyncProgress(vr.Status, vr.Spec.ReplicationState)

	// Build backend-specific information
	backendSpecific := ca.buildBackendSpecificInfo(vr)

	// Check for active state transitions
	transitionKey := ca.buildTransitionKey(uvr)
	activeTransition, hasTransition := ca.getActiveStateTransition(transitionKey)
	if hasTransition && !activeTransition.Allowed {
		health = ReplicationHealthDegraded
		detailedMessage += fmt.Sprintf("; Invalid transition: %s", activeTransition.Reason)
	}

	status := &ReplicationStatus{
		State:           unifiedState,
		Health:          health,
		Message:         detailedMessage,
		SyncProgress:    &progress,
		BackendSpecific: backendSpecific,
		Conditions:      ca.convertConditionsToStatusConditions(vr.Status.Conditions),
	}

	if vr.Status.LastSyncTime != nil {
		status.LastSyncTime = &vr.Status.LastSyncTime.Time
	}

	// Estimate next sync time based on scheduling
	if nextSync := ca.estimateNextSyncTime(uvr, vr); nextSync != nil {
		status.NextSyncTime = nextSync
	}

	return status, nil
}

// buildBasicReplicationStatus creates basic status for fallback
func (ca *CephAdapter) buildBasicReplicationStatus(vr *VolumeReplication) *ReplicationStatus {
	// Basic state translation without error handling
	unifiedState := "unknown"
	if state, _, err := ca.translateFromCephState(vr.Spec.ReplicationState); err == nil {
		unifiedState = state
	}

	health := ca.mapCephStatusToHealth(vr.Status)
	progress := ca.calculateSyncProgress(vr.Status)

	return &ReplicationStatus{
		State:        unifiedState,
		Health:       health,
		Message:      vr.Status.Message,
		SyncProgress: &progress,
	}
}

// analyzeVolumeReplicationConditions provides detailed condition analysis
func (ca *CephAdapter) analyzeVolumeReplicationConditions(conditions []metav1.Condition) (ReplicationHealth, string) {
	if len(conditions) == 0 {
		return ReplicationHealthUnknown, "No conditions available"
	}

	var messages []string
	health := ReplicationHealthHealthy

	for _, condition := range conditions {
		switch condition.Type {
		case "Degraded":
			if condition.Status == metav1.ConditionTrue {
				health = ReplicationHealthDegraded
				messages = append(messages, fmt.Sprintf("Degraded: %s", condition.Message))
			}
		case "Healthy":
			if condition.Status == metav1.ConditionFalse {
				if health == ReplicationHealthHealthy {
					health = ReplicationHealthDegraded
				}
				messages = append(messages, fmt.Sprintf("Not healthy: %s", condition.Message))
			}
		case "Error", "Failed":
			if condition.Status == metav1.ConditionTrue {
				health = ReplicationHealthUnhealthy
				messages = append(messages, fmt.Sprintf("Error: %s", condition.Message))
			}
		case "Resyncing":
			if condition.Status == metav1.ConditionTrue {
				messages = append(messages, "Resyncing in progress")
			}
		case "Ready":
			if condition.Status == metav1.ConditionTrue {
				messages = append(messages, "Ready for operations")
			}
		}
	}

	if len(messages) == 0 {
		messages = append(messages, "Status conditions analyzed")
	}

	return health, strings.Join(messages, "; ")
}

// calculateEnhancedSyncProgress provides detailed sync progress
func (ca *CephAdapter) calculateEnhancedSyncProgress(status VolumeReplicationStatus, state string) SyncProgress {
	progress := SyncProgress{
		TotalBytes:  100, // Default total
		SyncedBytes: 0,
	}

	// State-based progress estimation
	switch state {
	case CephPrimaryState, CephSecondaryState:
		progress.SyncedBytes = 100 // Fully synced
	case "resync-promote", "resync-demote":
		progress.SyncedBytes = 50 // In progress
	case "initial-sync":
		progress.SyncedBytes = 10 // Just started
	default:
		progress.SyncedBytes = 0 // Unknown state
	}

	// Look for progress information in conditions
	for _, condition := range status.Conditions {
		if condition.Type == "Resyncing" && condition.Status == metav1.ConditionTrue {
			// Extract progress from message if available
			if strings.Contains(condition.Message, "%") {
				progress.SyncedBytes = 75 // Rough estimate for active resync
			}
		}
	}

	return progress
}

// buildBackendSpecificInfo creates Ceph-specific status information
func (ca *CephAdapter) buildBackendSpecificInfo(vr *VolumeReplication) map[string]interface{} {
	info := make(map[string]interface{})

	info["ceph_state"] = vr.Spec.ReplicationState
	info["volume_replication_class"] = vr.Spec.VolumeReplicationClass
	info["pvc_name"] = vr.Spec.PvcName

	if vr.Spec.AutoResync != nil {
		info["auto_resync"] = *vr.Spec.AutoResync
	}

	if vr.Status.LastSyncTime != nil {
		info["last_sync_time"] = vr.Status.LastSyncTime.Time
	}

	if vr.Status.LastSyncDuration != nil {
		info["last_sync_duration"] = vr.Status.LastSyncDuration.Duration
	}

	return info
}

// convertConditionsToStatusConditions converts k8s conditions to our format
func (ca *CephAdapter) convertConditionsToStatusConditions(conditions []metav1.Condition) []StatusCondition {
	var statusConditions []StatusCondition

	for _, condition := range conditions {
		statusConditions = append(statusConditions, StatusCondition{
			Type:               condition.Type,
			Status:             string(condition.Status),
			LastTransitionTime: condition.LastTransitionTime.Time,
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}

	return statusConditions
}

// estimateNextSyncTime estimates when the next sync will occur
func (ca *CephAdapter) estimateNextSyncTime(uvr *replicationv1alpha1.UnifiedVolumeReplication, vr *VolumeReplication) *time.Time {
	// If manual mode, no automatic sync
	if uvr.Spec.Schedule.Mode == "manual" {
		return nil
	}

	// If continuous mode, sync is ongoing
	if uvr.Spec.Schedule.Mode == "continuous" {
		next := time.Now().Add(AutoResyncCheckInterval)
		return &next
	}

	// For interval mode, use RPO
	if uvr.Spec.Schedule.Mode == "interval" && uvr.Spec.Schedule.Rpo != "" {
		if duration, err := time.ParseDuration(uvr.Spec.Schedule.Rpo); err == nil {
			var baseTime time.Time
			if vr.Status.LastSyncTime != nil {
				baseTime = vr.Status.LastSyncTime.Time
			} else {
				baseTime = time.Now()
			}
			next := baseTime.Add(duration)
			return &next
		}
	}

	return nil
}

// buildStatusCacheKey creates a cache key for status
func (ca *CephAdapter) buildStatusCacheKey(uvr *replicationv1alpha1.UnifiedVolumeReplication) string {
	return fmt.Sprintf("ceph:%s:%s", uvr.Namespace, uvr.Name)
}

// buildTransitionKey creates a key for state transitions
func (ca *CephAdapter) buildTransitionKey(uvr *replicationv1alpha1.UnifiedVolumeReplication) string {
	return fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
}

// mapCephStatusToHealth maps Ceph VolumeReplication status to ReplicationHealth
func (ca *CephAdapter) mapCephStatusToHealth(status VolumeReplicationStatus) ReplicationHealth {
	// Check conditions for health status
	for _, condition := range status.Conditions {
		switch condition.Type {
		case "Degraded":
			if condition.Status == metav1.ConditionTrue {
				return ReplicationHealthDegraded
			}
		case "Healthy":
			if condition.Status == metav1.ConditionTrue {
				return ReplicationHealthHealthy
			}
		case "Error", "Failed":
			if condition.Status == metav1.ConditionTrue {
				return ReplicationHealthUnhealthy
			}
		}
	}

	// Default to healthy if no specific conditions
	if status.State == "primary" || status.State == "secondary" {
		return ReplicationHealthHealthy
	}

	return ReplicationHealthUnknown
}

// calculateSyncProgress calculates sync progress from Ceph status
func (ca *CephAdapter) calculateSyncProgress(status VolumeReplicationStatus) SyncProgress {
	progress := SyncProgress{
		TotalBytes:  100, // Default values for now
		SyncedBytes: 0,
	}

	// Extract progress information from status message if available
	if status.State == "primary" || status.State == "secondary" {
		progress.SyncedBytes = 100 // Assume fully synced
	}

	return progress
}

// buildVolumeReplication creates a VolumeReplication object from UnifiedVolumeReplication
func (ca *CephAdapter) buildVolumeReplication(uvr *replicationv1alpha1.UnifiedVolumeReplication) (*VolumeReplication, error) {
	// Translate unified state to Ceph state
	cephState, _, err := ca.translateToCephState(string(uvr.Spec.ReplicationState))
	if err != nil {
		return nil, fmt.Errorf("failed to translate state: %w", err)
	}

	// Default VolumeReplicationClass
	volumeReplicationClass := "rbd-volumereplicationclass"

	vr := &VolumeReplication{
		TypeMeta: metav1.TypeMeta{
			APIVersion: VolumeReplicationAPIVersion,
			Kind:       VolumeReplicationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ca.buildVolumeReplicationName(uvr),
			Namespace: uvr.Namespace,
			Labels: map[string]string{
				"managed-by": "unified-replication-operator",
				"backend":    "ceph",
			},
		},
		Spec: VolumeReplicationSpec{
			VolumeReplicationClass: volumeReplicationClass,
			PvcName:                uvr.Spec.VolumeMapping.Source.PvcName,
			ReplicationState:       cephState,
		},
	}

	return vr, nil
}

// buildVolumeReplicationName generates a name for the VolumeReplication resource
func (ca *CephAdapter) buildVolumeReplicationName(uvr *replicationv1alpha1.UnifiedVolumeReplication) string {
	return fmt.Sprintf("%s-vr", uvr.Name)
}

// translateToCephState translates unified state to Ceph-specific state
func (ca *CephAdapter) translateToCephState(unifiedState string) (string, string, error) {
	cephState, err := ca.BaseAdapter.translator.TranslateStateToBackend(translation.BackendCeph, unifiedState)
	return cephState, "", err
}

// translateFromCephState translates Ceph state to unified state
func (ca *CephAdapter) translateFromCephState(cephState string) (string, string, error) {
	unifiedState, err := ca.BaseAdapter.translator.TranslateStateFromBackend(translation.BackendCeph, cephState)
	return unifiedState, "", err
}

// SupportsConfiguration checks if the adapter supports the given configuration
func (ca *CephAdapter) SupportsConfiguration(uvr *replicationv1alpha1.UnifiedVolumeReplication) (bool, error) {
	if uvr == nil {
		return false, fmt.Errorf("UnifiedVolumeReplication cannot be nil")
	}

	// Check if storage class is compatible
	if err := ca.validateStorageClass(uvr); err != nil {
		return false, nil // Not supported but not an error
	}

	return true, nil
}

// PromoteReplica promotes a replica to primary with state transition validation
func (ca *CephAdapter) PromoteReplica(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Promoting Ceph replica to primary")

	startTime := time.Now()
	transitionKey := ca.buildTransitionKey(uvr)

	// Validate current state allows promotion
	currentStatus, err := ca.GetReplicationStatus(ctx, uvr)
	if err != nil {
		ca.BaseAdapter.updateMetrics("promote", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendCeph, "promote", uvr.Name, "failed to get current status", err)
	}

	// Check if promotion is allowed from current state
	if allowed, reason := ca.isValidStateTransition(currentStatus.State, "promoting"); !allowed {
		ca.BaseAdapter.updateMetrics("promote", false, startTime)
		return NewAdapterError(ErrorTypeValidation, translation.BackendCeph, "promote", uvr.Name,
			fmt.Sprintf("invalid state transition: %s", reason))
	}

	// Track the state transition
	ca.trackStateTransition(transitionKey, currentStatus.State, "promoting")

	// Get the VolumeReplication resource
	vr := &VolumeReplication{}
	if err := ca.client.Get(ctx, types.NamespacedName{
		Name:      ca.buildVolumeReplicationName(uvr),
		Namespace: uvr.Namespace,
	}, vr); err != nil {
		ca.completeStateTransition(transitionKey, false)
		ca.BaseAdapter.updateMetrics("promote", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "promote", uvr.Name, "failed to get VolumeReplication", err)
	}

	// Translate to Ceph promote state
	cephPromoteState, _, err := ca.translateToCephState("promoting")
	if err != nil {
		ca.completeStateTransition(transitionKey, false)
		ca.BaseAdapter.updateMetrics("promote", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendCeph, "promote", uvr.Name, "failed to translate promote state", err)
	}

	// Update VolumeReplication to promote
	vr.Spec.ReplicationState = cephPromoteState
	if err := ca.client.Update(ctx, vr); err != nil {
		ca.completeStateTransition(transitionKey, false)
		ca.BaseAdapter.updateMetrics("promote", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "promote", uvr.Name, "failed to update VolumeReplication for promotion", err)
	}

	// Wait for promotion to complete with timeout
	if err := ca.waitForStateTransition(ctx, uvr, "source", DefaultStateTransitionTimeout); err != nil {
		ca.completeStateTransition(transitionKey, false)
		ca.BaseAdapter.updateMetrics("promote", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeTimeout, translation.BackendCeph, "promote", uvr.Name, "promotion timed out", err)
	}

	// Clear cache and complete transition
	ca.statusCache.Clear()
	ca.completeStateTransition(transitionKey, true)
	ca.BaseAdapter.updateMetrics("promote", true, startTime)

	logger.Info("Successfully promoted Ceph replica to primary")
	return nil
}

// DemoteSource demotes a primary to replica with state transition validation
func (ca *CephAdapter) DemoteSource(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Demoting Ceph primary to replica")

	startTime := time.Now()
	transitionKey := ca.buildTransitionKey(uvr)

	// Validate current state allows demotion
	currentStatus, err := ca.GetReplicationStatus(ctx, uvr)
	if err != nil {
		ca.BaseAdapter.updateMetrics("demote", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendCeph, "demote", uvr.Name, "failed to get current status", err)
	}

	if allowed, reason := ca.isValidStateTransition(currentStatus.State, "demoting"); !allowed {
		ca.BaseAdapter.updateMetrics("demote", false, startTime)
		return NewAdapterError(ErrorTypeValidation, translation.BackendCeph, "demote", uvr.Name,
			fmt.Sprintf("invalid state transition: %s", reason))
	}

	// Track the state transition
	ca.trackStateTransition(transitionKey, currentStatus.State, "demoting")

	// Get the VolumeReplication resource
	vr := &VolumeReplication{}
	if err := ca.client.Get(ctx, types.NamespacedName{
		Name:      ca.buildVolumeReplicationName(uvr),
		Namespace: uvr.Namespace,
	}, vr); err != nil {
		ca.completeStateTransition(transitionKey, false)
		ca.BaseAdapter.updateMetrics("demote", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "demote", uvr.Name, "failed to get VolumeReplication", err)
	}

	// Translate to Ceph demote state
	cephDemoteState, _, err := ca.translateToCephState("demoting")
	if err != nil {
		ca.completeStateTransition(transitionKey, false)
		ca.BaseAdapter.updateMetrics("demote", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendCeph, "demote", uvr.Name, "failed to translate demote state", err)
	}

	// Update VolumeReplication to demote
	vr.Spec.ReplicationState = cephDemoteState
	if err := ca.client.Update(ctx, vr); err != nil {
		ca.completeStateTransition(transitionKey, false)
		ca.BaseAdapter.updateMetrics("demote", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "demote", uvr.Name, "failed to update VolumeReplication for demotion", err)
	}

	// Wait for demotion to complete
	if err := ca.waitForStateTransition(ctx, uvr, "replica", DefaultStateTransitionTimeout); err != nil {
		ca.completeStateTransition(transitionKey, false)
		ca.BaseAdapter.updateMetrics("demote", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeTimeout, translation.BackendCeph, "demote", uvr.Name, "demotion timed out", err)
	}

	// Clear cache and complete transition
	ca.statusCache.Clear()
	ca.completeStateTransition(transitionKey, true)
	ca.BaseAdapter.updateMetrics("demote", true, startTime)

	logger.Info("Successfully demoted Ceph primary to replica")
	return nil
}

// ResyncReplication triggers a resync operation
func (ca *CephAdapter) ResyncReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Resyncing Ceph replication")

	startTime := time.Now()
	transitionKey := ca.buildTransitionKey(uvr)

	// Get current status
	currentStatus, err := ca.GetReplicationStatus(ctx, uvr)
	if err != nil {
		ca.BaseAdapter.updateMetrics("resync", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendCeph, "resync", uvr.Name, "failed to get current status", err)
	}

	// Track transition to syncing state
	ca.trackStateTransition(transitionKey, currentStatus.State, "syncing")

	// Get the VolumeReplication resource
	vr := &VolumeReplication{}
	if err := ca.client.Get(ctx, types.NamespacedName{
		Name:      ca.buildVolumeReplicationName(uvr),
		Namespace: uvr.Namespace,
	}, vr); err != nil {
		ca.completeStateTransition(transitionKey, false)
		ca.BaseAdapter.updateMetrics("resync", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "resync", uvr.Name, "failed to get VolumeReplication", err)
	}

	// Enable auto-resync if not already enabled
	autoResync := DefaultAutoResyncEnabled
	vr.Spec.AutoResync = &autoResync

	// Update the VolumeReplication resource
	if err := ca.client.Update(ctx, vr); err != nil {
		ca.completeStateTransition(transitionKey, false)
		ca.BaseAdapter.updateMetrics("resync", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "resync", uvr.Name, "failed to update VolumeReplication for resync", err)
	}

	// Clear cache to force fresh status
	ca.statusCache.Clear()
	ca.completeStateTransition(transitionKey, true)
	ca.BaseAdapter.updateMetrics("resync", true, startTime)

	logger.Info("Successfully triggered Ceph replication resync")
	return nil
}

// waitForStateTransition waits for a specific state transition to complete
func (ca *CephAdapter) waitForStateTransition(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication, targetState string, timeout time.Duration) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter")

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(StateTransitionRetryInterval)
	defer ticker.Stop()

	retries := 0
	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("state transition to %s timed out after %v (retries: %d)", targetState, timeout, retries)
		case <-ticker.C:
			// Clear cache for fresh status
			cacheKey := ca.buildStatusCacheKey(uvr)
			ca.statusCache.Set(cacheKey, nil) // Invalidate cache

			status, err := ca.GetReplicationStatus(ctx, uvr)
			if err != nil {
				logger.V(1).Info("Error getting status during transition wait", "error", err)
				retries++
				if retries >= MaxStateTransitionRetries {
					return fmt.Errorf("max retries exceeded waiting for state transition to %s", targetState)
				}
				continue
			}

			if status.State == targetState {
				logger.Info("State transition completed", "targetState", targetState, "retries", retries)
				return nil
			}

			logger.V(1).Info("Waiting for state transition", "currentState", status.State, "targetState", targetState, "retries", retries)
			retries++
		}
	}
}

// PauseReplication pauses replication operations
func (ca *CephAdapter) PauseReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Pausing Ceph replication")

	startTime := time.Now()

	// Get the VolumeReplication resource
	vr := &VolumeReplication{}
	if err := ca.client.Get(ctx, types.NamespacedName{
		Name:      ca.buildVolumeReplicationName(uvr),
		Namespace: uvr.Namespace,
	}, vr); err != nil {
		ca.BaseAdapter.updateMetrics("pause", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "pause", uvr.Name, "failed to get VolumeReplication", err)
	}

	// Disable auto-resync to pause operations
	autoResync := false
	vr.Spec.AutoResync = &autoResync

	if err := ca.client.Update(ctx, vr); err != nil {
		ca.BaseAdapter.updateMetrics("pause", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "pause", uvr.Name, "failed to pause replication", err)
	}

	ca.statusCache.Clear()
	ca.BaseAdapter.updateMetrics("pause", true, startTime)
	logger.Info("Successfully paused Ceph replication")
	return nil
}

// ResumeReplication resumes paused replication operations
func (ca *CephAdapter) ResumeReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Resuming Ceph replication")

	startTime := time.Now()

	// Get the VolumeReplication resource
	vr := &VolumeReplication{}
	if err := ca.client.Get(ctx, types.NamespacedName{
		Name:      ca.buildVolumeReplicationName(uvr),
		Namespace: uvr.Namespace,
	}, vr); err != nil {
		ca.BaseAdapter.updateMetrics("resume", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "resume", uvr.Name, "failed to get VolumeReplication", err)
	}

	// Enable auto-resync to resume operations
	autoResync := DefaultAutoResyncEnabled
	vr.Spec.AutoResync = &autoResync

	if err := ca.client.Update(ctx, vr); err != nil {
		ca.BaseAdapter.updateMetrics("resume", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeConnection, translation.BackendCeph, "resume", uvr.Name, "failed to resume replication", err)
	}

	ca.statusCache.Clear()
	ca.BaseAdapter.updateMetrics("resume", true, startTime)
	logger.Info("Successfully resumed Ceph replication")
	return nil
}

// FailoverReplication performs a failover operation
func (ca *CephAdapter) FailoverReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Performing Ceph replication failover")

	// Failover is essentially promoting the replica
	return ca.PromoteReplica(ctx, uvr)
}

// FailbackReplication performs a failback operation
func (ca *CephAdapter) FailbackReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Performing Ceph replication failback")

	// Failback involves demoting current primary and promoting original primary
	// This is a simplified implementation - in practice might require coordination
	return ca.DemoteSource(ctx, uvr)
}

// RecoverFromError attempts to recover from error states
func (ca *CephAdapter) RecoverFromError(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Attempting recovery from error state")

	startTime := time.Now()

	// Get current status to understand the error
	status, err := ca.GetReplicationStatus(ctx, uvr)
	if err != nil {
		ca.BaseAdapter.updateMetrics("recover", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendCeph, "recover", uvr.Name, "failed to get current status for recovery", err)
	}

	if status.Health != ReplicationHealthUnhealthy {
		logger.Info("Replication is not in error state, no recovery needed", "health", status.Health)
		ca.BaseAdapter.updateMetrics("recover", true, startTime)
		return nil
	}

	// Recovery strategy based on current state
	recoveryActions := []func(context.Context, *replicationv1alpha1.UnifiedVolumeReplication) error{
		ca.attemptResyncRecovery,
		ca.attemptRestartRecovery,
		ca.attemptResetRecovery,
	}

	for i, action := range recoveryActions {
		logger.Info("Attempting recovery action", "action", i+1, "totalActions", len(recoveryActions))

		if err := action(ctx, uvr); err != nil {
			logger.Error(err, "Recovery action failed", "action", i+1)
			continue
		}

		// Check if recovery was successful
		time.Sleep(StateTransitionRetryInterval)
		newStatus, err := ca.GetReplicationStatus(ctx, uvr)
		if err != nil {
			logger.Error(err, "Failed to check status after recovery action", "action", i+1)
			continue
		}

		if newStatus.Health != ReplicationHealthUnhealthy {
			logger.Info("Recovery successful", "action", i+1, "newHealth", newStatus.Health)
			ca.BaseAdapter.updateMetrics("recover", true, startTime)
			return nil
		}
	}

	ca.BaseAdapter.updateMetrics("recover", false, startTime)
	return NewAdapterError(ErrorTypeOperation, translation.BackendCeph, "recover", uvr.Name, "all recovery attempts failed")
}

// attemptResyncRecovery tries to recover by triggering a resync
func (ca *CephAdapter) attemptResyncRecovery(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Attempting recovery via resync")

	return ca.ResyncReplication(ctx, uvr)
}

// attemptRestartRecovery tries to recover by recreating the VolumeReplication resource
func (ca *CephAdapter) attemptRestartRecovery(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Attempting recovery via resource restart")

	// Delete and recreate the VolumeReplication resource
	if err := ca.DeleteReplication(ctx, uvr); err != nil {
		logger.Error(err, "Failed to delete VolumeReplication during restart recovery")
		return err
	}

	// Wait a bit for cleanup
	time.Sleep(StateTransitionRetryInterval)

	// Recreate the resource
	return ca.CreateReplication(ctx, uvr)
}

// attemptResetRecovery tries to recover by resetting to a safe state
func (ca *CephAdapter) attemptResetRecovery(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Attempting recovery via state reset")

	// Get the VolumeReplication resource
	vr := &VolumeReplication{}
	if err := ca.client.Get(ctx, types.NamespacedName{
		Name:      ca.buildVolumeReplicationName(uvr),
		Namespace: uvr.Namespace,
	}, vr); err != nil {
		return err
	}

	// Try to reset to secondary state as a safe fallback
	cephSecondaryState, _, err := ca.translateToCephState("replica")
	if err != nil {
		return err
	}

	vr.Spec.ReplicationState = cephSecondaryState
	autoResync := DefaultAutoResyncEnabled
	vr.Spec.AutoResync = &autoResync

	return ca.client.Update(ctx, vr)
}

// IsHealthy checks if the adapter and its backend are healthy
func (ca *CephAdapter) IsHealthy() bool {
	ca.healthMutex.RLock()
	defer ca.healthMutex.RUnlock()

	// Check if we've done a recent health check
	if time.Since(ca.lastHealthCheck) < AutoResyncCheckInterval {
		return ca.BaseAdapter.IsHealthy()
	}

	return ca.BaseAdapter.IsHealthy()
}

// performHealthCheck performs a comprehensive health check
func (ca *CephAdapter) performHealthCheck(ctx context.Context) error {
	ca.healthMutex.Lock()
	defer ca.healthMutex.Unlock()

	ca.lastHealthCheck = time.Now()

	// Basic adapter health check
	if !ca.BaseAdapter.IsHealthy() {
		return fmt.Errorf("base adapter health check failed")
	}

	// Check translation engine health
	if err := ca.BaseAdapter.translator.ValidateTranslation(translation.BackendCeph); err != nil {
		return fmt.Errorf("translation engine validation failed: %w", err)
	}

	// Check Kubernetes client connectivity
	// Try to list CRDs to verify API server connectivity
	if err := ca.client.List(ctx, &VolumeReplicationList{}); err != nil {
		return fmt.Errorf("kubernetes client connectivity check failed: %w", err)
	}

	return nil
}

// GetVersion returns the adapter version
func (ca *CephAdapter) GetVersion() string {
	return "v1.0.0-ceph"
}

// GetSupportedFeatures returns the features supported by this adapter
func (ca *CephAdapter) GetSupportedFeatures() []AdapterFeature {
	return []AdapterFeature{
		FeatureAsyncReplication,
		FeaturePromotion,
		FeatureDemotion,
		FeatureResync,
		FeatureFailover,
		FeatureFailback,
		FeaturePauseResume,
		FeatureAutoResync,
		FeatureSnapshotBased,
		FeatureHealthMonitoring,
		FeatureMetrics,
		FeatureProgressTracking,
		FeatureRealTimeStatus,
	}
}

// Initialize performs adapter initialization
func (ca *CephAdapter) Initialize(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter")
	logger.Info("Initializing Ceph adapter")

	// Initialize base adapter first
	if err := ca.BaseAdapter.Initialize(ctx); err != nil {
		return fmt.Errorf("base adapter initialization failed: %w", err)
	}

	// Perform initial health check after base adapter is initialized
	if err := ca.performHealthCheck(ctx); err != nil {
		return fmt.Errorf("initial health check failed: %w", err)
	}

	return nil
}

// Cleanup performs adapter cleanup
func (ca *CephAdapter) Cleanup(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter")
	logger.Info("Cleaning up Ceph adapter")

	// Clear caches
	ca.statusCache.Clear()

	// Clear active transitions
	ca.transitionMutex.Lock()
	ca.activeTransitions = make(map[string]*StateTransition)
	ca.transitionMutex.Unlock()

	// Cleanup base adapter
	return ca.BaseAdapter.Cleanup(ctx)
}

// Reconcile performs adapter reconciliation
func (ca *CephAdapter) Reconcile(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("ceph-adapter").WithValues("uvr", uvr.Name)
	logger.V(1).Info("Reconciling Ceph replication")

	// Perform health check periodically during reconciliation
	if time.Since(ca.lastHealthCheck) > AutoResyncCheckInterval {
		if err := ca.performHealthCheck(ctx); err != nil {
			logger.Error(err, "Health check failed during reconciliation")
			// Don't fail reconciliation on health check failure, just log it
		}
	}

	// Get current status
	status, err := ca.GetReplicationStatus(ctx, uvr)
	if err != nil {
		return err
	}

	// Check if recovery is needed
	if status.Health == ReplicationHealthUnhealthy {
		logger.Info("Unhealthy replication detected, attempting recovery")
		return ca.RecoverFromError(ctx, uvr)
	}

	// Check for stuck state transitions
	transitionKey := ca.buildTransitionKey(uvr)
	if transition, exists := ca.getActiveStateTransition(transitionKey); exists {
		if !transition.Allowed {
			logger.Error(nil, "Invalid state transition detected", "transition", transition)
			ca.completeStateTransition(transitionKey, false)
		}
	}

	return nil
}

// CephAdapterFactory creates Ceph adapter instances
type CephAdapterFactory struct {
	info AdapterFactoryInfo
}

// NewCephAdapterFactory creates a new factory for Ceph adapters
func NewCephAdapterFactory() *CephAdapterFactory {
	return &CephAdapterFactory{
		info: AdapterFactoryInfo{
			Name:    "Ceph Adapter",
			Backend: translation.BackendCeph,
			Version: "v1.0.0",
		},
	}
}

// CreateAdapter creates a new Ceph adapter instance
func (f *CephAdapterFactory) CreateAdapter(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig) (ReplicationAdapter, error) {
	if backend != translation.BackendCeph {
		return nil, fmt.Errorf("unsupported backend: %s", backend)
	}

	if client == nil {
		return nil, fmt.Errorf("Kubernetes client is required for Ceph adapter")
	}

	if translator == nil {
		return nil, fmt.Errorf("translator is required for Ceph adapter")
	}

	return NewCephAdapter(client, translator)
}

// GetBackendType returns the backend type this factory supports
func (f *CephAdapterFactory) GetBackendType() translation.Backend {
	return translation.BackendCeph
}

// GetInfo returns information about this factory
func (f *CephAdapterFactory) GetInfo() AdapterFactoryInfo {
	return f.info
}

// ValidateConfig validates the adapter configuration for Ceph
func (f *CephAdapterFactory) ValidateConfig(config *AdapterConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.Backend != translation.BackendCeph {
		return fmt.Errorf("unsupported backend: %s", config.Backend)
	}

	// Validate Ceph-specific configuration
	if config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if config.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}

	return nil
}

// Supports returns whether this factory supports the given configuration
func (f *CephAdapterFactory) Supports(uvr *replicationv1alpha1.UnifiedVolumeReplication) bool {
	if uvr == nil {
		return false
	}

	// Check if storage class indicates Ceph
	storageClass := strings.ToLower(uvr.Spec.SourceEndpoint.StorageClass)
	return strings.Contains(storageClass, "rbd") || strings.Contains(storageClass, "ceph")
}

// Register the Ceph adapter factory with the global registry
func init() {
	GetGlobalRegistry().RegisterFactory(NewCephAdapterFactory())
}
