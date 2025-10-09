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
	"math/rand"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

// MockTridentReplication represents a simulated Trident replication resource
type MockTridentReplication struct {
	Name              string                 `json:"name"`
	Namespace         string                 `json:"namespace"`
	State             string                 `json:"state"`
	Mode              string                 `json:"mode"`
	SourcePVC         string                 `json:"source_pvc"`
	DestinationVolume string                 `json:"destination_volume"`
	LastSyncTime      *time.Time             `json:"last_sync_time,omitempty"`
	NextSyncTime      *time.Time             `json:"next_sync_time,omitempty"`
	SyncProgress      *SyncProgress          `json:"sync_progress,omitempty"`
	Health            ReplicationHealth      `json:"health"`
	Message           string                 `json:"message"`
	Conditions        []StatusCondition      `json:"conditions"`
	BackendSpecific   map[string]interface{} `json:"backend_specific"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	Version           int64                  `json:"version"`
}

// MockTridentConfig configures mock behavior for the Trident adapter
type MockTridentConfig struct {
	// Success/failure probabilities
	CreateSuccessRate float64 `json:"create_success_rate"`
	UpdateSuccessRate float64 `json:"update_success_rate"`
	DeleteSuccessRate float64 `json:"delete_success_rate"`
	StatusSuccessRate float64 `json:"status_success_rate"`

	// Latency simulation
	MinLatency time.Duration `json:"min_latency"`
	MaxLatency time.Duration `json:"max_latency"`

	// State transition simulation
	StateTransitionDelay time.Duration `json:"state_transition_delay"`
	AutoProgressStates   bool          `json:"auto_progress_states"`

	// Health simulation
	HealthFluctuation   bool          `json:"health_fluctuation"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`

	// Performance simulation
	ThroughputMBps     float64 `json:"throughput_mbps"`
	ErrorInjectionRate float64 `json:"error_injection_rate"`
}

// DefaultMockTridentConfig returns default configuration for mock Trident adapter
func DefaultMockTridentConfig() *MockTridentConfig {
	return &MockTridentConfig{
		CreateSuccessRate:    0.95,
		UpdateSuccessRate:    0.98,
		DeleteSuccessRate:    0.99,
		StatusSuccessRate:    0.99,
		MinLatency:           10 * time.Millisecond,
		MaxLatency:           100 * time.Millisecond,
		StateTransitionDelay: 2 * time.Second,
		AutoProgressStates:   true,
		HealthFluctuation:    false,
		HealthCheckInterval:  30 * time.Second,
		ThroughputMBps:       100.0,
		ErrorInjectionRate:   0.01,
	}
}

// MockTridentAdapter simulates a Trident backend adapter for testing
type MockTridentAdapter struct {
	*BaseAdapter
	config          *MockTridentConfig
	replications    map[string]*MockTridentReplication
	events          []ReplicationEvent
	mutex           sync.RWMutex
	lastHealthCheck time.Time
	isHealthy       bool
}

// NewMockTridentAdapter creates a new mock Trident adapter
func NewMockTridentAdapter(client client.Client, translator *translation.Engine, config *MockTridentConfig) *MockTridentAdapter {
	if config == nil {
		config = DefaultMockTridentConfig()
	}

	baseConfig := &AdapterConfig{
		Backend:             translation.BackendTrident,
		Timeout:             30 * time.Second,
		RetryAttempts:       3,
		RetryDelay:          time.Second,
		HealthCheckEnabled:  true,
		HealthCheckInterval: config.HealthCheckInterval,
	}

	adapter := &MockTridentAdapter{
		BaseAdapter:  NewBaseAdapter(translation.BackendTrident, client, translator, baseConfig),
		config:       config,
		replications: make(map[string]*MockTridentReplication),
		events:       make([]ReplicationEvent, 0),
		isHealthy:    true,
	}

	// Start background processes if auto-progression is enabled
	if config.AutoProgressStates {
		go adapter.backgroundStateProcessor()
	}

	return adapter
}

// EnsureReplication ensures the replication is in the desired state (idempotent)
func (mta *MockTridentAdapter) EnsureReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Ensuring Trident replication is in desired state")

	mta.simulateLatency()

	// Check if we should simulate failure
	if !mta.simulateSuccess(mta.config.CreateSuccessRate) {
		return NewAdapterError(ErrorTypeConnection, translation.BackendTrident, "ensure", uvr.Name, "simulated creation failure")
	}

	mta.mutex.Lock()
	defer mta.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)

	// Check if replication exists
	if mockRepl, exists := mta.replications[replicationKey]; exists {
		// Update existing replication
		tridentState, _ := mta.BaseAdapter.TranslateState(string(uvr.Spec.ReplicationState))
		tridentMode, _ := mta.BaseAdapter.TranslateMode(string(uvr.Spec.ReplicationMode))

		mockRepl.State = tridentState
		mockRepl.Mode = tridentMode
		mockRepl.Version++
		mockRepl.UpdatedAt = time.Now()
		now := time.Now()
		mockRepl.LastSyncTime = &now

		logger.Info("Updated Trident replication")
		return nil
	}

	// Create new replication
	tridentState, err := mta.BaseAdapter.TranslateState(string(uvr.Spec.ReplicationState))
	if err != nil {
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendTrident, "ensure", uvr.Name, "state translation failed", err)
	}

	tridentMode, err := mta.BaseAdapter.TranslateMode(string(uvr.Spec.ReplicationMode))
	if err != nil {
		return NewAdapterErrorWithCause(ErrorTypeValidation, translation.BackendTrident, "ensure", uvr.Name, "mode translation failed", err)
	}

	now := time.Now()
	mockRepl := &MockTridentReplication{
		Name:              uvr.Name,
		Namespace:         uvr.Namespace,
		State:             tridentState,
		Mode:              tridentMode,
		SourcePVC:         uvr.Spec.VolumeMapping.Source.PvcName,
		DestinationVolume: uvr.Spec.VolumeMapping.Destination.VolumeHandle,
		Health:            ReplicationHealthHealthy,
		Message:           "Replication created successfully",
		Conditions: []StatusCondition{
			{
				Type:               "Ready",
				Status:             "True",
				LastTransitionTime: now,
				Reason:             "Created",
				Message:            "Trident replication created",
			},
		},
		BackendSpecific: map[string]interface{}{
			"mirrorRelationshipUUID": fmt.Sprintf("uuid-%d", rand.Int63()),
			"policyName":             mta.getTridentPolicyName(uvr),
			"actionType":             "create",
			"lastActionTime":         now.Format(time.RFC3339),
		},
		CreatedAt:    now,
		UpdatedAt:    now,
		LastSyncTime: &now,
		Version:      1,
	}

	mta.replications[replicationKey] = mockRepl

	// Add creation event
	mta.addEvent(ReplicationEvent{
		Type:      EventTypeCreated,
		Message:   "Mock Trident replication created successfully",
		Timestamp: now,
		Resource:  replicationKey,
	})

	logger.Info("Created Trident replication")
	return nil
}

// DeleteReplication deletes a replication from the mock backend
func (mta *MockTridentAdapter) DeleteReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Deleting mock Trident replication")

	startTime := time.Now()
	mta.simulateLatency()

	if !mta.simulateSuccess(mta.config.DeleteSuccessRate) {
		mta.BaseAdapter.updateMetrics("delete", false, startTime)
		return NewAdapterError(ErrorTypeConnection, translation.BackendTrident, "delete", uvr.Name, "simulated deletion failure")
	}

	mta.mutex.Lock()
	defer mta.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	if _, exists := mta.replications[replicationKey]; !exists {
		// Deletion is idempotent - not an error
		mta.BaseAdapter.updateMetrics("delete", true, startTime)
		logger.Info("Mock Trident replication already deleted or not found")
		return nil
	}

	delete(mta.replications, replicationKey)
	mta.addEvent(ReplicationEvent{
		Type:      EventTypeDeleted,
		Message:   "Mock Trident replication deleted successfully",
		Timestamp: time.Now(),
		Resource:  replicationKey,
	})

	mta.BaseAdapter.updateMetrics("delete", true, startTime)
	logger.Info("Successfully deleted mock Trident replication")
	return nil
}

// GetReplicationStatus returns the status of a replication from the mock backend
func (mta *MockTridentAdapter) GetReplicationStatus(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) (*ReplicationStatus, error) {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.V(1).Info("Getting mock Trident replication status")

	startTime := time.Now()
	mta.simulateLatency()

	if !mta.simulateSuccess(mta.config.StatusSuccessRate) {
		mta.BaseAdapter.updateMetrics("status", false, startTime)
		return nil, NewAdapterError(ErrorTypeConnection, translation.BackendTrident, "status", uvr.Name, "simulated status retrieval failure")
	}

	mta.mutex.RLock()
	defer mta.mutex.RUnlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	replication, exists := mta.replications[replicationKey]
	if !exists {
		mta.BaseAdapter.updateMetrics("status", false, startTime)
		return nil, NewAdapterError(ErrorTypeResource, translation.BackendTrident, "status", uvr.Name, "replication not found")
	}

	// Translate Trident state back to unified state
	unifiedState, err := mta.BaseAdapter.translator.TranslateStateFromBackend(translation.BackendTrident, replication.State)
	if err != nil {
		mta.BaseAdapter.updateMetrics("status", false, startTime)
		return nil, NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "status", uvr.Name, "failed to translate state", err)
	}

	unifiedMode, err := mta.BaseAdapter.translator.TranslateModeFromBackend(translation.BackendTrident, replication.Mode)
	if err != nil {
		mta.BaseAdapter.updateMetrics("status", false, startTime)
		return nil, NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "status", uvr.Name, "failed to translate mode", err)
	}

	status := &ReplicationStatus{
		State:              unifiedState,
		Mode:               unifiedMode,
		Health:             replication.Health,
		LastSyncTime:       replication.LastSyncTime,
		NextSyncTime:       replication.NextSyncTime,
		SyncProgress:       replication.SyncProgress,
		BackendSpecific:    replication.BackendSpecific,
		Message:            replication.Message,
		ObservedGeneration: replication.Version,
		Conditions:         replication.Conditions,
	}

	mta.BaseAdapter.updateMetrics("status", true, startTime)
	return status, nil
}

// ValidateConfiguration validates the configuration for mock Trident adapter
func (mta *MockTridentAdapter) ValidateConfiguration(uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	// Always validate successfully for mock adapter
	return nil
}

// SupportsConfiguration checks if the mock Trident adapter supports the given configuration
func (mta *MockTridentAdapter) SupportsConfiguration(uvr *replicationv1alpha1.UnifiedVolumeReplication) (bool, error) {
	// Mock adapter supports all configurations
	return true, nil
}

// PromoteReplica promotes a replica to primary in the mock backend
func (mta *MockTridentAdapter) PromoteReplica(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Promoting mock Trident replica")

	return mta.simulateStateOperation(ctx, uvr, "promoting", "Promoting replica to primary")
}

// DemoteSource demotes a primary to replica in the mock backend
func (mta *MockTridentAdapter) DemoteSource(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Demoting mock Trident source")

	return mta.simulateStateOperation(ctx, uvr, "demoting", "Demoting primary to replica")
}

// ResyncReplication triggers a resync operation in the mock backend
func (mta *MockTridentAdapter) ResyncReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Resyncing mock Trident replication")

	return mta.simulateStateOperation(ctx, uvr, "syncing", "Resynchronizing replication")
}

// PauseReplication pauses replication operations in the mock backend
func (mta *MockTridentAdapter) PauseReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Pausing mock Trident replication")

	mta.mutex.Lock()
	defer mta.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	replication, exists := mta.replications[replicationKey]
	if !exists {
		return NewAdapterError(ErrorTypeResource, translation.BackendTrident, "pause", uvr.Name, "replication not found")
	}

	replication.BackendSpecific["paused"] = true
	replication.BackendSpecific["actionType"] = "pause"
	replication.BackendSpecific["lastActionTime"] = time.Now().Format(time.RFC3339)
	replication.Message = "Replication paused"

	return nil
}

// ResumeReplication resumes paused replication operations in the mock backend
func (mta *MockTridentAdapter) ResumeReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Resuming mock Trident replication")

	mta.mutex.Lock()
	defer mta.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	replication, exists := mta.replications[replicationKey]
	if !exists {
		return NewAdapterError(ErrorTypeResource, translation.BackendTrident, "resume", uvr.Name, "replication not found")
	}

	replication.BackendSpecific["paused"] = false
	replication.BackendSpecific["actionType"] = "resume"
	replication.BackendSpecific["lastActionTime"] = time.Now().Format(time.RFC3339)
	replication.Message = "Replication resumed"

	return nil
}

// FailoverReplication performs a failover operation in the mock backend
func (mta *MockTridentAdapter) FailoverReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Performing mock Trident failover")

	// Failover is essentially a promotion
	return mta.PromoteReplica(ctx, uvr)
}

// FailbackReplication performs a failback operation in the mock backend
func (mta *MockTridentAdapter) FailbackReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Performing mock Trident failback")

	// Failback is essentially a demotion followed by resync
	if err := mta.DemoteSource(ctx, uvr); err != nil {
		return err
	}
	return mta.ResyncReplication(ctx, uvr)
}

// GetBackendType returns the backend type for this adapter
func (mta *MockTridentAdapter) GetBackendType() translation.Backend {
	return translation.BackendTrident
}

// GetSupportedFeatures returns the features supported by this mock adapter
func (mta *MockTridentAdapter) GetSupportedFeatures() []AdapterFeature {
	return []AdapterFeature{
		FeatureAsyncReplication,
		FeatureSyncReplication,
		FeaturePromotion,
		FeatureDemotion,
		FeatureResync,
		FeatureFailover,
		FeatureFailback,
		FeaturePauseResume,
		FeatureAutoResync,
		FeatureMetrics,
		FeatureProgressTracking,
		FeatureRealTimeStatus,
	}
}

// GetVersion returns the adapter version
func (mta *MockTridentAdapter) GetVersion() string {
	return "v1.0.0-mock-trident"
}

// IsHealthy checks if the mock adapter is healthy
func (mta *MockTridentAdapter) IsHealthy() bool {
	mta.mutex.RLock()
	defer mta.mutex.RUnlock()
	return mta.isHealthy && mta.BaseAdapter.IsHealthy()
}

// Initialize performs adapter initialization
func (mta *MockTridentAdapter) Initialize(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter")
	logger.Info("Initializing mock Trident adapter")

	mta.mutex.Lock()
	mta.isHealthy = true
	mta.lastHealthCheck = time.Now()
	mta.mutex.Unlock()

	return mta.BaseAdapter.Initialize(ctx)
}

// Cleanup performs adapter cleanup
func (mta *MockTridentAdapter) Cleanup(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter")
	logger.Info("Cleaning up mock Trident adapter")

	mta.mutex.Lock()
	mta.replications = make(map[string]*MockTridentReplication)
	mta.events = make([]ReplicationEvent, 0)
	mta.mutex.Unlock()

	return mta.BaseAdapter.Cleanup(ctx)
}

// Reconcile performs adapter reconciliation
func (mta *MockTridentAdapter) Reconcile(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-trident-adapter").WithValues("uvr", uvr.Name)
	logger.V(1).Info("Reconciling mock Trident replication")

	// Simulate occasional reconciliation issues
	if mta.simulateSuccess(mta.config.ErrorInjectionRate) {
		return NewAdapterError(ErrorTypeConnection, translation.BackendTrident, "reconcile", uvr.Name, "simulated reconciliation failure")
	}

	return nil
}

// Helper methods for mock behavior simulation

func (mta *MockTridentAdapter) simulateLatency() {
	if mta.config.MinLatency > 0 || mta.config.MaxLatency > 0 {
		min := mta.config.MinLatency
		max := mta.config.MaxLatency
		if max <= min {
			max = min + time.Millisecond
		}

		latency := min + time.Duration(rand.Int63n(int64(max-min)))
		time.Sleep(latency)
	}
}

func (mta *MockTridentAdapter) simulateSuccess(successRate float64) bool {
	return rand.Float64() < successRate
}

func (mta *MockTridentAdapter) simulateStateTransition(replication *MockTridentReplication, newState string) {
	// Simulate state transition with delay
	go func() {
		time.Sleep(mta.config.StateTransitionDelay)

		mta.mutex.Lock()
		defer mta.mutex.Unlock()

		replication.State = newState
		replication.UpdatedAt = time.Now()

		// Update sync progress based on state
		mta.updateSyncProgress(replication)
	}()
}

func (mta *MockTridentAdapter) simulateStateOperation(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication, targetState, message string) error {
	mta.mutex.Lock()
	defer mta.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	replication, exists := mta.replications[replicationKey]
	if !exists {
		return NewAdapterError(ErrorTypeResource, translation.BackendTrident, "state-operation", uvr.Name, "replication not found")
	}

	// Translate to Trident state
	tridentState, err := mta.BaseAdapter.translator.TranslateStateToBackend(translation.BackendTrident, targetState)
	if err != nil {
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendTrident, "state-operation", uvr.Name, "failed to translate state", err)
	}

	replication.State = tridentState
	replication.Message = message
	replication.UpdatedAt = time.Now()
	replication.BackendSpecific["actionType"] = targetState
	replication.BackendSpecific["lastActionTime"] = time.Now().Format(time.RFC3339)

	mta.addEvent(ReplicationEvent{
		Type:      EventTypeUpdated,
		Message:   message,
		Timestamp: time.Now(),
		Resource:  replicationKey,
	})

	return nil
}

func (mta *MockTridentAdapter) updateSyncProgress(replication *MockTridentReplication) {
	now := time.Now()

	// Simulate sync progress based on throughput
	if replication.SyncProgress == nil {
		replication.SyncProgress = &SyncProgress{
			TotalBytes:      1024 * 1024 * 1024, // 1GB
			SyncedBytes:     0,
			PercentComplete: 0.0,
		}
	}

	if replication.State == "replicating" || replication.State == "syncing" {
		if replication.LastSyncTime != nil {
			duration := now.Sub(*replication.LastSyncTime)
			bytesToSync := int64(mta.config.ThroughputMBps * 1024 * 1024 * duration.Seconds())

			replication.SyncProgress.SyncedBytes = min(
				replication.SyncProgress.SyncedBytes+bytesToSync,
				replication.SyncProgress.TotalBytes,
			)
			replication.SyncProgress.PercentComplete = float64(replication.SyncProgress.SyncedBytes) / float64(replication.SyncProgress.TotalBytes) * 100.0

			if replication.SyncProgress.PercentComplete < 100.0 {
				remaining := replication.SyncProgress.TotalBytes - replication.SyncProgress.SyncedBytes
				estimatedSeconds := float64(remaining) / (mta.config.ThroughputMBps * 1024 * 1024)
				estimatedTime := now.Add(time.Duration(estimatedSeconds) * time.Second)
				replication.NextSyncTime = &estimatedTime
				replication.SyncProgress.EstimatedTime = fmt.Sprintf("%.0fs", estimatedSeconds)
			}
		}
		replication.LastSyncTime = &now
	}
}

func (mta *MockTridentAdapter) getTridentPolicyName(uvr *replicationv1alpha1.UnifiedVolumeReplication) string {
	// Return a default policy name based on replication mode
	if uvr.Spec.ReplicationMode == replicationv1alpha1.ReplicationModeSynchronous {
		return "sync-mirror-policy"
	}
	return "async-mirror-policy"
}

func (mta *MockTridentAdapter) addEvent(event ReplicationEvent) {
	// Keep only the last 100 events
	if len(mta.events) >= 100 {
		mta.events = mta.events[1:]
	}
	mta.events = append(mta.events, event)
}

func (mta *MockTridentAdapter) backgroundStateProcessor() {
	ticker := time.NewTicker(mta.config.StateTransitionDelay)
	defer ticker.Stop()

	for range ticker.C {
		mta.mutex.Lock()
		for _, replication := range mta.replications {
			mta.updateSyncProgress(replication)

			// Simulate state transitions
			if replication.State == "promoting" {
				replication.State = "established-source"
				replication.Message = "Promotion completed"
			} else if replication.State == "demoting" {
				replication.State = "established-replica"
				replication.Message = "Demotion completed"
			} else if replication.State == "syncing" {
				if replication.SyncProgress != nil && replication.SyncProgress.PercentComplete >= 100.0 {
					replication.State = "established-replica"
					replication.Message = "Synchronization completed"
				}
			}
		}
		mta.mutex.Unlock()
	}
}

// GetAllMockTridentReplications returns all mock replications (for testing)
func (mta *MockTridentAdapter) GetAllMockTridentReplications() map[string]*MockTridentReplication {
	mta.mutex.RLock()
	defer mta.mutex.RUnlock()

	result := make(map[string]*MockTridentReplication)
	for k, v := range mta.replications {
		result[k] = v
	}
	return result
}

// GetMockTridentEvents returns all mock events (for testing)
func (mta *MockTridentAdapter) GetMockTridentEvents() []ReplicationEvent {
	mta.mutex.RLock()
	defer mta.mutex.RUnlock()

	result := make([]ReplicationEvent, len(mta.events))
	copy(result, mta.events)
	return result
}

// SetMockTridentHealth manually sets the health status (for testing)
func (mta *MockTridentAdapter) SetMockTridentHealth(healthy bool) {
	mta.mutex.Lock()
	defer mta.mutex.Unlock()
	mta.isHealthy = healthy
}

// MockTridentAdapterFactory creates mock Trident adapter instances
type MockTridentAdapterFactory struct {
	info   AdapterFactoryInfo
	config *MockTridentConfig
}

// NewMockTridentAdapterFactory creates a new factory for mock Trident adapters
func NewMockTridentAdapterFactory(config *MockTridentConfig) *MockTridentAdapterFactory {
	if config == nil {
		config = DefaultMockTridentConfig()
	}

	return &MockTridentAdapterFactory{
		info: AdapterFactoryInfo{
			Name:    "Mock Trident Adapter",
			Backend: translation.BackendTrident,
			Version: "v1.0.0-mock",
		},
		config: config,
	}
}

// Create creates a new mock Trident adapter instance
func (factory *MockTridentAdapterFactory) Create(client client.Client, translator *translation.Engine) (ReplicationAdapter, error) {
	return NewMockTridentAdapter(client, translator, factory.config), nil
}

// CreateAdapter creates a new mock Trident adapter instance (implements AdapterFactory interface)
func (factory *MockTridentAdapterFactory) CreateAdapter(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig) (ReplicationAdapter, error) {
	return NewMockTridentAdapter(client, translator, factory.config), nil
}

// GetBackendType returns the backend type
func (factory *MockTridentAdapterFactory) GetBackendType() translation.Backend {
	return translation.BackendTrident
}

// GetInfo returns factory information
func (factory *MockTridentAdapterFactory) GetInfo() AdapterFactoryInfo {
	return factory.info
}

// Supports checks if this factory supports the given configuration
func (factory *MockTridentAdapterFactory) Supports(uvr *replicationv1alpha1.UnifiedVolumeReplication) bool {
	// Mock factory supports all configurations
	return true
}

// ValidateConfig validates the factory configuration
func (factory *MockTridentAdapterFactory) ValidateConfig(config *AdapterConfig) error {
	// Always valid for mock factory
	return nil
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
