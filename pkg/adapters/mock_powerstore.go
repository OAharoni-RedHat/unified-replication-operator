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

// MockPowerStoreReplication represents a simulated PowerStore replication resource
type MockPowerStoreReplication struct {
	Name               string                 `json:"name"`
	Namespace          string                 `json:"namespace"`
	State              string                 `json:"state"`
	Mode               string                 `json:"mode"`
	SourceVolume       string                 `json:"source_volume"`
	DestinationVolume  string                 `json:"destination_volume"`
	ReplicationGroupID string                 `json:"replication_group_id"`
	SessionID          string                 `json:"session_id"`
	LastSyncTime       *time.Time             `json:"last_sync_time,omitempty"`
	NextSyncTime       *time.Time             `json:"next_sync_time,omitempty"`
	SyncProgress       *SyncProgress          `json:"sync_progress,omitempty"`
	Health             ReplicationHealth      `json:"health"`
	Message            string                 `json:"message"`
	Conditions         []StatusCondition      `json:"conditions"`
	BackendSpecific    map[string]interface{} `json:"backend_specific"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
	Version            int64                  `json:"version"`
	RPOCompliance      float64                `json:"rpo_compliance"`
	RTOEstimate        time.Duration          `json:"rto_estimate"`
}

// MockPowerStoreConfig configures mock behavior for the PowerStore adapter
type MockPowerStoreConfig struct {
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

	// PowerStore-specific simulation
	MetroLatencyMs     int     `json:"metro_latency_ms"`
	RPOComplianceMin   float64 `json:"rpo_compliance_min"`
	RPOComplianceMax   float64 `json:"rpo_compliance_max"`
	SessionFailureRate float64 `json:"session_failure_rate"`
}

// DefaultMockPowerStoreConfig returns default configuration for mock PowerStore adapter
func DefaultMockPowerStoreConfig() *MockPowerStoreConfig {
	return &MockPowerStoreConfig{
		CreateSuccessRate:    0.95,
		UpdateSuccessRate:    0.98,
		DeleteSuccessRate:    0.99,
		StatusSuccessRate:    0.99,
		MinLatency:           15 * time.Millisecond,
		MaxLatency:           150 * time.Millisecond,
		StateTransitionDelay: 3 * time.Second,
		AutoProgressStates:   true,
		HealthFluctuation:    false,
		HealthCheckInterval:  30 * time.Second,
		ThroughputMBps:       200.0,
		ErrorInjectionRate:   0.01,
		MetroLatencyMs:       5,
		RPOComplianceMin:     95.0,
		RPOComplianceMax:     99.9,
		SessionFailureRate:   0.005,
	}
}

// MockPowerStoreAdapter simulates a PowerStore backend adapter for testing
type MockPowerStoreAdapter struct {
	*BaseAdapter
	config          *MockPowerStoreConfig
	replications    map[string]*MockPowerStoreReplication
	events          []ReplicationEvent
	mutex           sync.RWMutex
	lastHealthCheck time.Time
	isHealthy       bool
	sessions        map[string]string // replication key -> session ID
}

// NewMockPowerStoreAdapter creates a new mock PowerStore adapter
func NewMockPowerStoreAdapter(client client.Client, translator *translation.Engine, config *MockPowerStoreConfig) *MockPowerStoreAdapter {
	if config == nil {
		config = DefaultMockPowerStoreConfig()
	}

	baseConfig := &AdapterConfig{
		Backend:             translation.BackendPowerStore,
		Timeout:             45 * time.Second,
		RetryAttempts:       3,
		RetryDelay:          time.Second,
		HealthCheckEnabled:  true,
		HealthCheckInterval: config.HealthCheckInterval,
	}

	adapter := &MockPowerStoreAdapter{
		BaseAdapter:  NewBaseAdapter(translation.BackendPowerStore, client, translator, baseConfig),
		config:       config,
		replications: make(map[string]*MockPowerStoreReplication),
		events:       make([]ReplicationEvent, 0),
		sessions:     make(map[string]string),
		isHealthy:    true,
	}

	// Start background processes if auto-progression is enabled
	if config.AutoProgressStates {
		go adapter.backgroundStateProcessor()
	}

	if config.HealthFluctuation {
		go adapter.backgroundHealthMonitor()
	}

	return adapter
}

// CreateReplication creates a new replication in the mock backend
func (mpa *MockPowerStoreAdapter) CreateReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Creating mock PowerStore replication")

	startTime := time.Now()
	mpa.simulateLatency()

	if !mpa.simulateSuccess(mpa.config.CreateSuccessRate) {
		mpa.BaseAdapter.updateMetrics("create", false, startTime)
		return NewAdapterError(ErrorTypeConnection, translation.BackendPowerStore, "create", uvr.Name, "simulated creation failure")
	}

	mpa.mutex.Lock()
	defer mpa.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	if _, exists := mpa.replications[replicationKey]; exists {
		mpa.BaseAdapter.updateMetrics("create", false, startTime)
		return NewAdapterError(ErrorTypeValidation, translation.BackendPowerStore, "create", uvr.Name, "replication already exists")
	}

	// Translate unified state to PowerStore state
	powerstoreState, err := mpa.BaseAdapter.translator.TranslateStateToBackend(translation.BackendPowerStore, string(uvr.Spec.ReplicationState))
	if err != nil {
		mpa.BaseAdapter.updateMetrics("create", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "create", uvr.Name, "failed to translate state", err)
	}

	powerstoreMode, err := mpa.BaseAdapter.translator.TranslateModeToBackend(translation.BackendPowerStore, string(uvr.Spec.ReplicationMode))
	if err != nil {
		mpa.BaseAdapter.updateMetrics("create", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "create", uvr.Name, "failed to translate mode", err)
	}

	now := time.Now()
	sessionID := fmt.Sprintf("session-%d", rand.Int63())
	replicationGroupID := fmt.Sprintf("rg-%s-%s", uvr.Namespace, uvr.Name)

	replication := &MockPowerStoreReplication{
		Name:               uvr.Name,
		Namespace:          uvr.Namespace,
		State:              powerstoreState,
		Mode:               powerstoreMode,
		SourceVolume:       uvr.Spec.VolumeMapping.Source.PvcName,
		DestinationVolume:  uvr.Spec.VolumeMapping.Destination.VolumeHandle,
		ReplicationGroupID: replicationGroupID,
		SessionID:          sessionID,
		Health:             ReplicationHealthHealthy,
		Message:            "Replication created successfully",
		Conditions: []StatusCondition{
			{
				Type:               "Ready",
				Status:             "True",
				LastTransitionTime: now,
				Reason:             "Created",
				Message:            "PowerStore replication created",
			},
		},
		BackendSpecific: map[string]interface{}{
			"replication_group_id": replicationGroupID,
			"session_id":           sessionID,
			"metro_enabled":        mpa.isPowerStoreMetro(uvr),
			"protection_policy":    mpa.getPowerStoreProtectionPolicy(uvr),
			"creation_type":        "API",
			"array_serial":         fmt.Sprintf("PS%d", rand.Int31()),
		},
		CreatedAt:     now,
		UpdatedAt:     now,
		Version:       1,
		RPOCompliance: mpa.generateRPOCompliance(),
		RTOEstimate:   mpa.estimateRTO(uvr),
	}

	mpa.replications[replicationKey] = replication
	mpa.sessions[replicationKey] = sessionID

	mpa.addEvent(ReplicationEvent{
		Type:      EventTypeCreated,
		Message:   "Mock PowerStore replication created successfully",
		Timestamp: now,
		Resource:  replicationKey,
	})

	mpa.BaseAdapter.updateMetrics("create", true, startTime)
	logger.Info("Successfully created mock PowerStore replication")
	return nil
}

// UpdateReplication updates an existing replication in the mock backend
func (mpa *MockPowerStoreAdapter) UpdateReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Updating mock PowerStore replication")

	startTime := time.Now()
	mpa.simulateLatency()

	if !mpa.simulateSuccess(mpa.config.UpdateSuccessRate) {
		mpa.BaseAdapter.updateMetrics("update", false, startTime)
		return NewAdapterError(ErrorTypeConnection, translation.BackendPowerStore, "update", uvr.Name, "simulated update failure")
	}

	mpa.mutex.Lock()
	defer mpa.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	replication, exists := mpa.replications[replicationKey]
	if !exists {
		mpa.BaseAdapter.updateMetrics("update", false, startTime)
		return NewAdapterError(ErrorTypeResource, translation.BackendPowerStore, "update", uvr.Name, "replication not found")
	}

	// Translate unified state to PowerStore state
	powerstoreState, err := mpa.BaseAdapter.translator.TranslateStateToBackend(translation.BackendPowerStore, string(uvr.Spec.ReplicationState))
	if err != nil {
		mpa.BaseAdapter.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "update", uvr.Name, "failed to translate state", err)
	}

	powerstoreMode, err := mpa.BaseAdapter.translator.TranslateModeToBackend(translation.BackendPowerStore, string(uvr.Spec.ReplicationMode))
	if err != nil {
		mpa.BaseAdapter.updateMetrics("update", false, startTime)
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "update", uvr.Name, "failed to translate mode", err)
	}

	now := time.Now()

	// Check for state transitions
	if replication.State != powerstoreState {
		mpa.simulateStateTransition(replication, powerstoreState)
		mpa.addEvent(ReplicationEvent{
			Type:      EventTypeUpdated,
			Message:   fmt.Sprintf("State changed from %s to %s", replication.State, powerstoreState),
			Timestamp: now,
			Resource:  replicationKey,
		})
	}

	replication.State = powerstoreState
	replication.Mode = powerstoreMode
	replication.UpdatedAt = now
	replication.Version++
	replication.RPOCompliance = mpa.generateRPOCompliance()
	replication.RTOEstimate = mpa.estimateRTO(uvr)

	mpa.BaseAdapter.updateMetrics("update", true, startTime)
	logger.Info("Successfully updated mock PowerStore replication")
	return nil
}

// DeleteReplication deletes a replication from the mock backend
func (mpa *MockPowerStoreAdapter) DeleteReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Deleting mock PowerStore replication")

	startTime := time.Now()
	mpa.simulateLatency()

	if !mpa.simulateSuccess(mpa.config.DeleteSuccessRate) {
		mpa.BaseAdapter.updateMetrics("delete", false, startTime)
		return NewAdapterError(ErrorTypeConnection, translation.BackendPowerStore, "delete", uvr.Name, "simulated deletion failure")
	}

	mpa.mutex.Lock()
	defer mpa.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	if _, exists := mpa.replications[replicationKey]; !exists {
		// Deletion is idempotent - not an error
		mpa.BaseAdapter.updateMetrics("delete", true, startTime)
		logger.Info("Mock PowerStore replication already deleted or not found")
		return nil
	}

	delete(mpa.replications, replicationKey)
	delete(mpa.sessions, replicationKey)

	mpa.addEvent(ReplicationEvent{
		Type:      EventTypeDeleted,
		Message:   "Mock PowerStore replication deleted successfully",
		Timestamp: time.Now(),
		Resource:  replicationKey,
	})

	mpa.BaseAdapter.updateMetrics("delete", true, startTime)
	logger.Info("Successfully deleted mock PowerStore replication")
	return nil
}

// GetReplicationStatus returns the status of a replication from the mock backend
func (mpa *MockPowerStoreAdapter) GetReplicationStatus(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) (*ReplicationStatus, error) {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.V(1).Info("Getting mock PowerStore replication status")

	startTime := time.Now()
	mpa.simulateLatency()

	if !mpa.simulateSuccess(mpa.config.StatusSuccessRate) {
		mpa.BaseAdapter.updateMetrics("status", false, startTime)
		return nil, NewAdapterError(ErrorTypeConnection, translation.BackendPowerStore, "status", uvr.Name, "simulated status retrieval failure")
	}

	mpa.mutex.RLock()
	defer mpa.mutex.RUnlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	replication, exists := mpa.replications[replicationKey]
	if !exists {
		mpa.BaseAdapter.updateMetrics("status", false, startTime)
		return nil, NewAdapterError(ErrorTypeResource, translation.BackendPowerStore, "status", uvr.Name, "replication not found")
	}

	// Simulate session failures
	if mpa.simulateSuccess(mpa.config.SessionFailureRate) {
		replication.Health = ReplicationHealthDegraded
		replication.Message = "Session connectivity issues"
	}

	// Translate PowerStore state back to unified state
	unifiedState, err := mpa.BaseAdapter.translator.TranslateStateFromBackend(translation.BackendPowerStore, replication.State)
	if err != nil {
		mpa.BaseAdapter.updateMetrics("status", false, startTime)
		return nil, NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "status", uvr.Name, "failed to translate state", err)
	}

	unifiedMode, err := mpa.BaseAdapter.translator.TranslateModeFromBackend(translation.BackendPowerStore, replication.Mode)
	if err != nil {
		mpa.BaseAdapter.updateMetrics("status", false, startTime)
		return nil, NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "status", uvr.Name, "failed to translate mode", err)
	}

	// Add PowerStore-specific information
	backendSpecific := make(map[string]interface{})
	for k, v := range replication.BackendSpecific {
		backendSpecific[k] = v
	}
	backendSpecific["rpo_compliance"] = replication.RPOCompliance
	backendSpecific["rto_estimate"] = replication.RTOEstimate.String()
	backendSpecific["metro_latency_ms"] = mpa.config.MetroLatencyMs

	status := &ReplicationStatus{
		State:              unifiedState,
		Mode:               unifiedMode,
		Health:             replication.Health,
		LastSyncTime:       replication.LastSyncTime,
		NextSyncTime:       replication.NextSyncTime,
		SyncProgress:       replication.SyncProgress,
		BackendSpecific:    backendSpecific,
		Message:            replication.Message,
		ObservedGeneration: replication.Version,
		Conditions:         replication.Conditions,
	}

	mpa.BaseAdapter.updateMetrics("status", true, startTime)
	return status, nil
}

// ValidateConfiguration validates the configuration for mock PowerStore adapter
func (mpa *MockPowerStoreAdapter) ValidateConfiguration(uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	// Always validate successfully for mock adapter
	return nil
}

// SupportsConfiguration checks if the mock PowerStore adapter supports the given configuration
func (mpa *MockPowerStoreAdapter) SupportsConfiguration(uvr *replicationv1alpha1.UnifiedVolumeReplication) (bool, error) {
	// Mock adapter supports all configurations
	return true, nil
}

// PromoteReplica promotes a replica to primary in the mock backend
func (mpa *MockPowerStoreAdapter) PromoteReplica(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Promoting mock PowerStore replica")

	return mpa.simulateStateOperation(ctx, uvr, "promoting", "Promoting replica to primary")
}

// DemoteSource demotes a primary to replica in the mock backend
func (mpa *MockPowerStoreAdapter) DemoteSource(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Demoting mock PowerStore source")

	return mpa.simulateStateOperation(ctx, uvr, "demoting", "Demoting primary to replica")
}

// ResyncReplication triggers a resync operation in the mock backend
func (mpa *MockPowerStoreAdapter) ResyncReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Resyncing mock PowerStore replication")

	return mpa.simulateStateOperation(ctx, uvr, "syncing", "Resynchronizing replication")
}

// PauseReplication pauses replication operations in the mock backend
func (mpa *MockPowerStoreAdapter) PauseReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Pausing mock PowerStore replication")

	mpa.mutex.Lock()
	defer mpa.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	replication, exists := mpa.replications[replicationKey]
	if !exists {
		return NewAdapterError(ErrorTypeResource, translation.BackendPowerStore, "pause", uvr.Name, "replication not found")
	}

	replication.BackendSpecific["paused"] = true
	replication.BackendSpecific["pause_reason"] = "User requested"
	replication.Message = "Replication paused"

	return nil
}

// ResumeReplication resumes paused replication operations in the mock backend
func (mpa *MockPowerStoreAdapter) ResumeReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Resuming mock PowerStore replication")

	mpa.mutex.Lock()
	defer mpa.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	replication, exists := mpa.replications[replicationKey]
	if !exists {
		return NewAdapterError(ErrorTypeResource, translation.BackendPowerStore, "resume", uvr.Name, "replication not found")
	}

	replication.BackendSpecific["paused"] = false
	delete(replication.BackendSpecific, "pause_reason")
	replication.Message = "Replication resumed"

	return nil
}

// FailoverReplication performs a failover operation in the mock backend
func (mpa *MockPowerStoreAdapter) FailoverReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Performing mock PowerStore failover")

	// Failover is essentially a promotion with session failover
	if err := mpa.PromoteReplica(ctx, uvr); err != nil {
		return err
	}

	// Update session information
	mpa.mutex.Lock()
	defer mpa.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	if replication, exists := mpa.replications[replicationKey]; exists {
		newSessionID := fmt.Sprintf("failover-session-%d", rand.Int63())
		replication.SessionID = newSessionID
		replication.BackendSpecific["session_id"] = newSessionID
		replication.BackendSpecific["failover_time"] = time.Now().Format(time.RFC3339)
		mpa.sessions[replicationKey] = newSessionID
	}

	return nil
}

// FailbackReplication performs a failback operation in the mock backend
func (mpa *MockPowerStoreAdapter) FailbackReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.Info("Performing mock PowerStore failback")

	// Failback involves resync followed by role reversal
	if err := mpa.ResyncReplication(ctx, uvr); err != nil {
		return err
	}
	return mpa.DemoteSource(ctx, uvr)
}

// GetBackendType returns the backend type for this adapter
func (mpa *MockPowerStoreAdapter) GetBackendType() translation.Backend {
	return translation.BackendPowerStore
}

// GetSupportedFeatures returns the features supported by this mock adapter
func (mpa *MockPowerStoreAdapter) GetSupportedFeatures() []AdapterFeature {
	return []AdapterFeature{
		FeatureAsyncReplication,
		FeatureSyncReplication,
		FeatureMetroReplication,
		FeaturePromotion,
		FeatureDemotion,
		FeatureResync,
		FeatureFailover,
		FeatureFailback,
		FeaturePauseResume,
		FeatureConsistencyGroups,
		FeatureVolumeGroups,
		FeatureHealthMonitoring,
		FeatureMetrics,
		FeatureProgressTracking,
		FeatureRealTimeStatus,
		FeatureMultiRegion,
	}
}

// GetVersion returns the adapter version
func (mpa *MockPowerStoreAdapter) GetVersion() string {
	return "v1.0.0-mock-powerstore"
}

// IsHealthy checks if the mock adapter is healthy
func (mpa *MockPowerStoreAdapter) IsHealthy() bool {
	mpa.mutex.RLock()
	defer mpa.mutex.RUnlock()
	return mpa.isHealthy && mpa.BaseAdapter.IsHealthy()
}

// Initialize performs adapter initialization
func (mpa *MockPowerStoreAdapter) Initialize(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter")
	logger.Info("Initializing mock PowerStore adapter")

	mpa.mutex.Lock()
	mpa.isHealthy = true
	mpa.lastHealthCheck = time.Now()
	mpa.mutex.Unlock()

	return mpa.BaseAdapter.Initialize(ctx)
}

// Cleanup performs adapter cleanup
func (mpa *MockPowerStoreAdapter) Cleanup(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter")
	logger.Info("Cleaning up mock PowerStore adapter")

	mpa.mutex.Lock()
	mpa.replications = make(map[string]*MockPowerStoreReplication)
	mpa.events = make([]ReplicationEvent, 0)
	mpa.sessions = make(map[string]string)
	mpa.mutex.Unlock()

	return mpa.BaseAdapter.Cleanup(ctx)
}

// Reconcile performs adapter reconciliation
func (mpa *MockPowerStoreAdapter) Reconcile(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	logger := log.FromContext(ctx).WithName("mock-powerstore-adapter").WithValues("uvr", uvr.Name)
	logger.V(1).Info("Reconciling mock PowerStore replication")

	// Simulate occasional reconciliation issues
	if mpa.simulateSuccess(mpa.config.ErrorInjectionRate) {
		return NewAdapterError(ErrorTypeConnection, translation.BackendPowerStore, "reconcile", uvr.Name, "simulated reconciliation failure")
	}

	return nil
}

// Helper methods for mock behavior simulation

func (mpa *MockPowerStoreAdapter) simulateLatency() {
	if mpa.config.MinLatency > 0 || mpa.config.MaxLatency > 0 {
		min := mpa.config.MinLatency
		max := mpa.config.MaxLatency
		if max <= min {
			max = min + time.Millisecond
		}

		latency := min + time.Duration(rand.Int63n(int64(max-min)))
		time.Sleep(latency)
	}
}

func (mpa *MockPowerStoreAdapter) simulateSuccess(successRate float64) bool {
	return rand.Float64() < successRate
}

func (mpa *MockPowerStoreAdapter) simulateStateTransition(replication *MockPowerStoreReplication, newState string) {
	// Simulate state transition with delay
	go func() {
		time.Sleep(mpa.config.StateTransitionDelay)

		mpa.mutex.Lock()
		defer mpa.mutex.Unlock()

		replication.State = newState
		replication.UpdatedAt = time.Now()
		replication.RPOCompliance = mpa.generateRPOCompliance()

		// Update sync progress based on state
		mpa.updateSyncProgress(replication)
	}()
}

func (mpa *MockPowerStoreAdapter) simulateStateOperation(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication, targetState, message string) error {
	mpa.mutex.Lock()
	defer mpa.mutex.Unlock()

	replicationKey := fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
	replication, exists := mpa.replications[replicationKey]
	if !exists {
		return NewAdapterError(ErrorTypeResource, translation.BackendPowerStore, "state-operation", uvr.Name, "replication not found")
	}

	// Translate to PowerStore state
	powerstoreState, err := mpa.BaseAdapter.translator.TranslateStateToBackend(translation.BackendPowerStore, targetState)
	if err != nil {
		return NewAdapterErrorWithCause(ErrorTypeOperation, translation.BackendPowerStore, "state-operation", uvr.Name, "failed to translate state", err)
	}

	replication.State = powerstoreState
	replication.Message = message
	replication.UpdatedAt = time.Now()
	replication.RPOCompliance = mpa.generateRPOCompliance()

	mpa.addEvent(ReplicationEvent{
		Type:      EventTypeUpdated,
		Message:   message,
		Timestamp: time.Now(),
		Resource:  replicationKey,
	})

	return nil
}

func (mpa *MockPowerStoreAdapter) updateSyncProgress(replication *MockPowerStoreReplication) {
	now := time.Now()

	// Simulate sync progress based on throughput
	if replication.SyncProgress == nil {
		replication.SyncProgress = &SyncProgress{
			TotalBytes:      2 * 1024 * 1024 * 1024, // 2GB
			SyncedBytes:     0,
			PercentComplete: 0.0,
		}
	}

	if replication.State == "REPLICATING" || replication.State == "SYNCING" {
		if replication.LastSyncTime != nil {
			duration := now.Sub(*replication.LastSyncTime)
			bytesToSync := int64(mpa.config.ThroughputMBps * 1024 * 1024 * duration.Seconds())

			replication.SyncProgress.SyncedBytes = min(
				replication.SyncProgress.SyncedBytes+bytesToSync,
				replication.SyncProgress.TotalBytes,
			)
			replication.SyncProgress.PercentComplete = float64(replication.SyncProgress.SyncedBytes) / float64(replication.SyncProgress.TotalBytes) * 100.0

			if replication.SyncProgress.PercentComplete < 100.0 {
				remaining := replication.SyncProgress.TotalBytes - replication.SyncProgress.SyncedBytes
				estimatedSeconds := float64(remaining) / (mpa.config.ThroughputMBps * 1024 * 1024)
				estimatedTime := now.Add(time.Duration(estimatedSeconds) * time.Second)
				replication.NextSyncTime = &estimatedTime
				replication.SyncProgress.EstimatedTime = fmt.Sprintf("%.0fs", estimatedSeconds)
			}
		}
		replication.LastSyncTime = &now
	}
}

func (mpa *MockPowerStoreAdapter) generateRPOCompliance() float64 {
	min := mpa.config.RPOComplianceMin
	max := mpa.config.RPOComplianceMax
	return min + rand.Float64()*(max-min)
}

func (mpa *MockPowerStoreAdapter) estimateRTO(uvr *replicationv1alpha1.UnifiedVolumeReplication) time.Duration {
	// Base RTO on mode and configuration
	baseRTO := 30 * time.Second

	if uvr.Spec.ReplicationMode == replicationv1alpha1.ReplicationModeSynchronous {
		baseRTO = 10 * time.Second
	} else if uvr.Spec.ReplicationMode == replicationv1alpha1.ReplicationModeEventual {
		baseRTO = 60 * time.Second
	}

	// Add some randomness
	variation := time.Duration(rand.Int63n(int64(baseRTO / 2)))
	return baseRTO + variation
}

func (mpa *MockPowerStoreAdapter) isPowerStoreMetro(uvr *replicationv1alpha1.UnifiedVolumeReplication) bool {
	// Metro replication is identified by synchronous mode
	return uvr.Spec.ReplicationMode == replicationv1alpha1.ReplicationModeSynchronous
}

func (mpa *MockPowerStoreAdapter) getPowerStoreProtectionPolicy(uvr *replicationv1alpha1.UnifiedVolumeReplication) string {
	// Use RPO settings if available
	if uvr.Spec.Extensions.Powerstore != nil && uvr.Spec.Extensions.Powerstore.RpoSettings != nil {
		return *uvr.Spec.Extensions.Powerstore.RpoSettings
	}
	return "default-protection-policy"
}

func (mpa *MockPowerStoreAdapter) addEvent(event ReplicationEvent) {
	// Keep only the last 100 events
	if len(mpa.events) >= 100 {
		mpa.events = mpa.events[1:]
	}
	mpa.events = append(mpa.events, event)
}

func (mpa *MockPowerStoreAdapter) backgroundStateProcessor() {
	ticker := time.NewTicker(mpa.config.StateTransitionDelay)
	defer ticker.Stop()

	for range ticker.C {
		mpa.mutex.Lock()
		for _, replication := range mpa.replications {
			mpa.updateSyncProgress(replication)

			// Simulate state transitions
			if replication.State == "PROMOTING" {
				replication.State = "PRIMARY"
				replication.Message = "Promotion completed"
			} else if replication.State == "DEMOTING" {
				replication.State = "SECONDARY"
				replication.Message = "Demotion completed"
			} else if replication.State == "SYNCING" {
				if replication.SyncProgress != nil && replication.SyncProgress.PercentComplete >= 100.0 {
					replication.State = "SECONDARY"
					replication.Message = "Synchronization completed"
				}
			}
		}
		mpa.mutex.Unlock()
	}
}

func (mpa *MockPowerStoreAdapter) backgroundHealthMonitor() {
	ticker := time.NewTicker(mpa.config.HealthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		mpa.mutex.Lock()
		// Simulate occasional health fluctuations
		if mpa.simulateSuccess(0.95) {
			mpa.isHealthy = true
		} else {
			mpa.isHealthy = false
		}
		mpa.lastHealthCheck = time.Now()
		mpa.mutex.Unlock()
	}
}

// GetAllMockPowerStoreReplications returns all mock replications (for testing)
func (mpa *MockPowerStoreAdapter) GetAllMockPowerStoreReplications() map[string]*MockPowerStoreReplication {
	mpa.mutex.RLock()
	defer mpa.mutex.RUnlock()

	result := make(map[string]*MockPowerStoreReplication)
	for k, v := range mpa.replications {
		result[k] = v
	}
	return result
}

// GetMockPowerStoreEvents returns all mock events (for testing)
func (mpa *MockPowerStoreAdapter) GetMockPowerStoreEvents() []ReplicationEvent {
	mpa.mutex.RLock()
	defer mpa.mutex.RUnlock()

	result := make([]ReplicationEvent, len(mpa.events))
	copy(result, mpa.events)
	return result
}

// GetMockPowerStoreSessions returns all mock sessions (for testing)
func (mpa *MockPowerStoreAdapter) GetMockPowerStoreSessions() map[string]string {
	mpa.mutex.RLock()
	defer mpa.mutex.RUnlock()

	result := make(map[string]string)
	for k, v := range mpa.sessions {
		result[k] = v
	}
	return result
}

// SetMockPowerStoreHealth manually sets the health status (for testing)
func (mpa *MockPowerStoreAdapter) SetMockPowerStoreHealth(healthy bool) {
	mpa.mutex.Lock()
	defer mpa.mutex.Unlock()
	mpa.isHealthy = healthy
}

// MockPowerStoreAdapterFactory creates mock PowerStore adapter instances
type MockPowerStoreAdapterFactory struct {
	info   AdapterFactoryInfo
	config *MockPowerStoreConfig
}

// NewMockPowerStoreAdapterFactory creates a new factory for mock PowerStore adapters
func NewMockPowerStoreAdapterFactory(config *MockPowerStoreConfig) *MockPowerStoreAdapterFactory {
	if config == nil {
		config = DefaultMockPowerStoreConfig()
	}

	return &MockPowerStoreAdapterFactory{
		info: AdapterFactoryInfo{
			Name:    "Mock PowerStore Adapter",
			Backend: translation.BackendPowerStore,
			Version: "v1.0.0-mock",
		},
		config: config,
	}
}

// Create creates a new mock PowerStore adapter instance
func (factory *MockPowerStoreAdapterFactory) Create(client client.Client, translator *translation.Engine) (ReplicationAdapter, error) {
	return NewMockPowerStoreAdapter(client, translator, factory.config), nil
}

// CreateAdapter creates a new mock PowerStore adapter instance (implements AdapterFactory interface)
func (factory *MockPowerStoreAdapterFactory) CreateAdapter(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig) (ReplicationAdapter, error) {
	return NewMockPowerStoreAdapter(client, translator, factory.config), nil
}

// GetBackendType returns the backend type
func (factory *MockPowerStoreAdapterFactory) GetBackendType() translation.Backend {
	return translation.BackendPowerStore
}

// GetInfo returns factory information
func (factory *MockPowerStoreAdapterFactory) GetInfo() AdapterFactoryInfo {
	return factory.info
}

// Supports checks if this factory supports the given configuration
func (factory *MockPowerStoreAdapterFactory) Supports(uvr *replicationv1alpha1.UnifiedVolumeReplication) bool {
	// Mock factory supports all configurations
	return true
}

// ValidateConfig validates the factory configuration
func (factory *MockPowerStoreAdapterFactory) ValidateConfig(config *AdapterConfig) error {
	// Always valid for mock factory
	return nil
}
