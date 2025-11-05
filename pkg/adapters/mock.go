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
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

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

	return &MockAdapter{
		BaseAdapter:  baseAdapter,
		config:       mockConfig,
		replications: make(map[string]*MockReplication),
		failureRate:  mockConfig.FailureRate,
	}
}

// Note: MockAdapter v1alpha1 methods have been removed.
// Mock adapters are test-only helpers. Use v1alpha2 adapters for production.
