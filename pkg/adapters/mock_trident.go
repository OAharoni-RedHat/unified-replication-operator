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
	"time"
)

// MockTridentReplication represents a simulated Trident replication resource
// Note: This uses v1alpha2-compatible types only
type MockTridentReplication struct {
	Name              string    `json:"name"`
	Namespace         string    `json:"namespace"`
	State             string    `json:"state"`
	Mode              string    `json:"mode"`
	SourcePVC         string    `json:"source_pvc"`
	DestinationVolume string    `json:"destination_volume"`
	LastSyncTime      *time.Time `json:"last_sync_time,omitempty"`
	Message           string    `json:"message"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Version           int64     `json:"version"`
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
		ErrorInjectionRate:   0.0,
	}
}

// Note: MockTridentAdapter v1alpha1 methods have been removed.
// Use TridentV1Alpha2Adapter for v1alpha2 API support.

