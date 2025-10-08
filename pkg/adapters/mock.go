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

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"sigs.k8s.io/controller-runtime/pkg/client"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

// MockAdapter provides a mock implementation for testing
type MockAdapter struct {
	*BaseAdapter

	// Mock configuration
	config *MockConfig

	// Mock state
	replications map[string]*MockReplication
	mu           sync.RWMutex

	// Behavior simulation
	nextOperationShouldFail bool
	failureRate             float64
	// latencySimulation       time.Duration // TODO: Implement latency simulation
}

// MockConfig contains configuration for mock behavior
type MockConfig struct {
	FailureRate      float64       `json:"failure_rate"`      // Probability of operations failing (0.0-1.0)
	LatencyMin       time.Duration `json:"latency_min"`       // Minimum operation latency
	LatencyMax       time.Duration `json:"latency_max"`       // Maximum operation latency
	StateTransitions bool          `json:"state_transitions"` // Whether to simulate state transitions
	ProgressTracking bool          `json:"progress_tracking"` // Whether to simulate sync progress
	EventGeneration  bool          `json:"event_generation"`  // Whether to generate events
}

// DefaultMockConfig returns the default mock configuration
func DefaultMockConfig() *MockConfig {
	return &MockConfig{
		FailureRate:      0.0,
		LatencyMin:       10 * time.Millisecond,
		LatencyMax:       100 * time.Millisecond,
		StateTransitions: true,
		ProgressTracking: true,
		EventGeneration:  true,
	}
}

// MockReplication represents the state of a mock replication
type MockReplication struct {
	Name               string
	State              string
	Mode               string
	Health             ReplicationHealth
	CreatedAt          time.Time
	LastSyncTime       time.Time
	SyncProgress       *SyncProgress
	Events             []ReplicationEvent
	BackendSpecific    map[string]interface{}
	ObservedGeneration int64
}

// NewMockAdapter creates a new mock adapter
func NewMockAdapter(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig, mockConfig *MockConfig) *MockAdapter {
	if mockConfig == nil {
		mockConfig = DefaultMockConfig()
	}

	baseAdapter := NewBaseAdapter(backend, client, translator, config)

	// Set mock-specific capabilities
	capabilities := AdapterCapabilities{
		Backend:         backend,
		SupportedStates: []string{"source", "replica", "promoting", "demoting", "syncing", "failed"},
		SupportedModes:  []string{"synchronous", "asynchronous", "eventual"},
		Features: []AdapterFeature{
			FeatureAsyncReplication,
			FeatureSyncReplication,
			FeatureEventualReplication,
			FeaturePromotion,
			FeatureDemotion,
			FeatureResync,
			FeaturePauseResume,
			FeatureProgressTracking,
			FeatureHealthMonitoring,
		},
		MaxConcurrentOps: 100,
		MaxVolumeSize:    "1TB",
	}
	baseAdapter.SetCapabilities(capabilities)

	// Set mock-specific info
	info := AdapterInfo{
		Name:        "Mock Adapter",
		Backend:     backend,
		Version:     "1.0.0-mock",
		Description: "Mock adapter for testing purposes",
		Author:      "Unified Replication Operator",
	}
	baseAdapter.SetInfo(info)

	return &MockAdapter{
		BaseAdapter:  baseAdapter,
		config:       mockConfig,
		replications: make(map[string]*MockReplication),
		failureRate:  mockConfig.FailureRate,
	}
}

// CreateReplication creates a mock replication
func (m *MockAdapter) CreateReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if err := m.simulateOperation("create"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getReplicationKey(uvr)
	if _, exists := m.replications[key]; exists {
		return NewAdapterError(ErrorTypeResource, m.GetBackendType(), "create", uvr.Name, "replication already exists")
	}

	mockRepl := &MockReplication{
		Name:               uvr.Name,
		State:              string(uvr.Spec.ReplicationState),
		Mode:               string(uvr.Spec.ReplicationMode),
		Health:             ReplicationHealthHealthy,
		CreatedAt:          time.Now(),
		LastSyncTime:       time.Now(),
		BackendSpecific:    make(map[string]interface{}),
		ObservedGeneration: uvr.Generation,
	}

	// Initialize sync progress if enabled
	if m.config.ProgressTracking {
		mockRepl.SyncProgress = &SyncProgress{
			TotalBytes:      1000000, // 1MB mock data
			SyncedBytes:     0,
			PercentComplete: 0.0,
			EstimatedTime:   "5m",
		}
	}

	m.replications[key] = mockRepl

	// Generate creation event
	if m.config.EventGeneration {
		event := ReplicationEvent{
			Type:      EventTypeCreated,
			Timestamp: time.Now(),
			Resource:  uvr.Name,
			Message:   "Replication created successfully",
		}
		mockRepl.Events = append(mockRepl.Events, event)
	}

	// Start background state simulation if enabled
	if m.config.StateTransitions {
		go m.simulateStateTransitions(key)
	}

	return nil
}

// UpdateReplication updates a mock replication
func (m *MockAdapter) UpdateReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if err := m.simulateOperation("update"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getReplicationKey(uvr)
	mockRepl, exists := m.replications[key]
	if !exists {
		return NewAdapterError(ErrorTypeResource, m.GetBackendType(), "update", uvr.Name, "replication not found")
	}

	// Update mock replication
	oldState := mockRepl.State
	mockRepl.State = string(uvr.Spec.ReplicationState)
	mockRepl.Mode = string(uvr.Spec.ReplicationMode)
	mockRepl.ObservedGeneration = uvr.Generation

	// Generate update event if state changed
	if m.config.EventGeneration && oldState != mockRepl.State {
		event := ReplicationEvent{
			Type:      EventTypeUpdated,
			Timestamp: time.Now(),
			Resource:  uvr.Name,
			Message:   fmt.Sprintf("State changed from %s to %s", oldState, mockRepl.State),
		}
		mockRepl.Events = append(mockRepl.Events, event)
	}

	return nil
}

// DeleteReplication deletes a mock replication
func (m *MockAdapter) DeleteReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if err := m.simulateOperation("delete"); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getReplicationKey(uvr)
	mockRepl, exists := m.replications[key]
	if !exists {
		return NewAdapterError(ErrorTypeResource, m.GetBackendType(), "delete", uvr.Name, "replication not found")
	}

	// Generate deletion event
	if m.config.EventGeneration {
		event := ReplicationEvent{
			Type:      EventTypeDeleted,
			Timestamp: time.Now(),
			Resource:  uvr.Name,
			Message:   "Replication deleted successfully",
		}
		mockRepl.Events = append(mockRepl.Events, event)
	}

	delete(m.replications, key)
	return nil
}

// GetReplicationStatus returns the status of a mock replication
func (m *MockAdapter) GetReplicationStatus(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) (*ReplicationStatus, error) {
	if err := m.simulateOperation("get_status"); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.getReplicationKey(uvr)
	mockRepl, exists := m.replications[key]
	if !exists {
		return nil, NewAdapterError(ErrorTypeResource, m.GetBackendType(), "get_status", uvr.Name, "replication not found")
	}

	status := &ReplicationStatus{
		State:              mockRepl.State,
		Mode:               mockRepl.Mode,
		Health:             mockRepl.Health,
		LastSyncTime:       &mockRepl.LastSyncTime,
		SyncProgress:       mockRepl.SyncProgress,
		BackendSpecific:    mockRepl.BackendSpecific,
		ObservedGeneration: mockRepl.ObservedGeneration,
		Message:            "Mock replication running",
	}

	// Add conditions
	status.Conditions = []StatusCondition{
		{
			Type:               "Ready",
			Status:             "True",
			LastTransitionTime: mockRepl.CreatedAt,
			Reason:             "ReplicationReady",
			Message:            "Mock replication is ready",
		},
	}

	// Simulate next sync time for interval mode
	if mockRepl.Mode == "asynchronous" {
		nextSync := time.Now().Add(5 * time.Minute)
		status.NextSyncTime = &nextSync
	}

	return status, nil
}

// PromoteReplica promotes a replica to source
func (m *MockAdapter) PromoteReplica(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if err := m.simulateOperation("promote"); err != nil {
		return err
	}

	return m.changeState(uvr, "promoting", EventTypePromoted, "Replica promoted to source")
}

// DemoteSource demotes a source to replica
func (m *MockAdapter) DemoteSource(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if err := m.simulateOperation("demote"); err != nil {
		return err
	}

	return m.changeState(uvr, "demoting", EventTypeDemoted, "Source demoted to replica")
}

// ResyncReplication resyncs a replication
func (m *MockAdapter) ResyncReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if err := m.simulateOperation("resync"); err != nil {
		return err
	}

	return m.changeState(uvr, "syncing", EventTypeResynced, "Replication resync initiated")
}

// PauseReplication pauses a replication
func (m *MockAdapter) PauseReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if err := m.simulateOperation("pause"); err != nil {
		return err
	}

	return m.changeState(uvr, "paused", EventTypePaused, "Replication paused")
}

// ResumeReplication resumes a paused replication
func (m *MockAdapter) ResumeReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if err := m.simulateOperation("resume"); err != nil {
		return err
	}

	return m.changeState(uvr, "syncing", EventTypeResumed, "Replication resumed")
}

// FailoverReplication performs failover
func (m *MockAdapter) FailoverReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if err := m.simulateOperation("failover"); err != nil {
		return err
	}

	return m.changeState(uvr, "source", EventTypeFailedOver, "Failover completed")
}

// FailbackReplication performs failback
func (m *MockAdapter) FailbackReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	if err := m.simulateOperation("failback"); err != nil {
		return err
	}

	return m.changeState(uvr, "replica", EventTypeFailedBack, "Failback completed")
}

// SetFailureRate sets the mock failure rate
func (m *MockAdapter) SetFailureRate(rate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureRate = rate
}

// SetNextOperationShouldFail forces the next operation to fail
func (m *MockAdapter) SetNextOperationShouldFail(shouldFail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextOperationShouldFail = shouldFail
}

// GetMockReplication returns mock replication data for testing
func (m *MockAdapter) GetMockReplication(uvr *replicationv1alpha1.UnifiedVolumeReplication) (*MockReplication, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.getReplicationKey(uvr)
	mockRepl, exists := m.replications[key]
	return mockRepl, exists
}

// GetAllMockReplications returns all mock replications
func (m *MockAdapter) GetAllMockReplications() map[string]*MockReplication {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	replications := make(map[string]*MockReplication)
	for k, v := range m.replications {
		replications[k] = v
	}
	return replications
}

// simulateOperation simulates operation latency and failure
func (m *MockAdapter) simulateOperation(operation string) error {
	// Simulate latency
	latency := m.calculateLatency()
	time.Sleep(latency)

	// Simulate failure
	m.mu.RLock()
	shouldFail := m.nextOperationShouldFail || (rand.Float64() < m.failureRate)
	m.mu.RUnlock()

	if shouldFail {
		m.mu.Lock()
		m.nextOperationShouldFail = false // Reset after use
		m.mu.Unlock()

		return NewAdapterError(ErrorTypeOperation, m.GetBackendType(), operation, "",
			fmt.Sprintf("mock operation %s failed (simulated)", operation))
	}

	return nil
}

// calculateLatency calculates simulated latency
func (m *MockAdapter) calculateLatency() time.Duration {
	min := int64(m.config.LatencyMin)
	max := int64(m.config.LatencyMax)
	if min >= max {
		return m.config.LatencyMin
	}
	return time.Duration(min + rand.Int63n(max-min))
}

// changeState changes the state of a mock replication
func (m *MockAdapter) changeState(uvr *replicationv1alpha1.UnifiedVolumeReplication, newState string, eventType ReplicationEventType, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getReplicationKey(uvr)
	mockRepl, exists := m.replications[key]
	if !exists {
		return NewAdapterError(ErrorTypeResource, m.GetBackendType(), "change_state", uvr.Name, "replication not found")
	}

	oldState := mockRepl.State
	mockRepl.State = newState

	// Generate event
	if m.config.EventGeneration {
		event := ReplicationEvent{
			Type:      eventType,
			Timestamp: time.Now(),
			Resource:  uvr.Name,
			Message:   message,
			Metadata: map[string]string{
				"old_state": oldState,
				"new_state": newState,
			},
		}
		mockRepl.Events = append(mockRepl.Events, event)
	}

	return nil
}

// simulateStateTransitions simulates automatic state transitions
func (m *MockAdapter) simulateStateTransitions(key string) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		mockRepl, exists := m.replications[key]
		if !exists {
			m.mu.Unlock()
			return // Replication deleted
		}

		// Simulate state transitions
		switch mockRepl.State {
		case "promoting":
			mockRepl.State = "source"
		case "demoting":
			mockRepl.State = "replica"
		case "syncing":
			// Simulate sync progress
			if mockRepl.SyncProgress != nil && mockRepl.SyncProgress.PercentComplete < 100.0 {
				mockRepl.SyncProgress.SyncedBytes += 10000 // 10KB increment
				mockRepl.SyncProgress.PercentComplete = float64(mockRepl.SyncProgress.SyncedBytes) / float64(mockRepl.SyncProgress.TotalBytes) * 100.0
				if mockRepl.SyncProgress.PercentComplete >= 100.0 {
					mockRepl.SyncProgress.PercentComplete = 100.0
					mockRepl.State = "replica"
					mockRepl.LastSyncTime = time.Now()
				}
			}
		}

		m.mu.Unlock()
	}
}

// getReplicationKey generates a key for storing mock replications
func (m *MockAdapter) getReplicationKey(uvr *replicationv1alpha1.UnifiedVolumeReplication) string {
	return fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
}

// MockAdapterFactory creates mock adapters
type MockAdapterFactory struct {
	*BaseAdapterFactory
	mockConfig *MockConfig
}

// NewMockAdapterFactory creates a new mock adapter factory
func NewMockAdapterFactory(backend translation.Backend, mockConfig *MockConfig) *MockAdapterFactory {
	if mockConfig == nil {
		mockConfig = DefaultMockConfig()
	}

	baseFactory := NewBaseAdapterFactory(backend, fmt.Sprintf("Mock %s Adapter", cases.Title(language.English).String(string(backend))), "1.0.0-mock",
		fmt.Sprintf("Mock adapter for %s backend testing", backend))

	return &MockAdapterFactory{
		BaseAdapterFactory: baseFactory,
		mockConfig:         mockConfig,
	}
}

// CreateAdapter creates a mock adapter
func (f *MockAdapterFactory) CreateAdapter(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig) (ReplicationAdapter, error) {
	return NewMockAdapter(backend, client, translator, config, f.mockConfig), nil
}

// ValidateConfig validates mock adapter configuration
func (f *MockAdapterFactory) ValidateConfig(config *AdapterConfig) error {
	if err := f.BaseAdapterFactory.ValidateConfig(config); err != nil {
		return err
	}

	// Mock-specific validation
	if f.mockConfig.FailureRate < 0.0 || f.mockConfig.FailureRate > 1.0 {
		return fmt.Errorf("failure rate must be between 0.0 and 1.0")
	}

	if f.mockConfig.LatencyMin < 0 || f.mockConfig.LatencyMax < f.mockConfig.LatencyMin {
		return fmt.Errorf("invalid latency configuration")
	}

	return nil
}
