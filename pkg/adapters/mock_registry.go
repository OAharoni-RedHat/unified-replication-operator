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
	"fmt"
	"os"
	"time"

	"github.com/unified-replication/operator/pkg/translation"
)

// RegisterMockAdapters registers mock adapters with the global registry
// This is typically called during testing or when mock backends are explicitly enabled
func RegisterMockAdapters() error {
	registry := GetGlobalRegistry()

	// Register mock Trident adapter
	tridentFactory := NewMockTridentAdapterFactory(nil)
	if err := registry.RegisterFactory(tridentFactory); err != nil {
		return fmt.Errorf("failed to register mock Trident adapter: %w", err)
	}

	// Register mock PowerStore adapter
	powerstoreFactory := NewMockPowerStoreAdapterFactory(nil)
	if err := registry.RegisterFactory(powerstoreFactory); err != nil {
		return fmt.Errorf("failed to register mock PowerStore adapter: %w", err)
	}

	return nil
}

// RegisterMockAdaptersWithConfig registers mock adapters with custom configurations
func RegisterMockAdaptersWithConfig(tridentConfig *MockTridentConfig, powerstoreConfig *MockPowerStoreConfig) error {
	registry := GetGlobalRegistry()

	// Register mock Trident adapter with custom config
	tridentFactory := NewMockTridentAdapterFactory(tridentConfig)
	if err := registry.RegisterFactory(tridentFactory); err != nil {
		return fmt.Errorf("failed to register mock Trident adapter: %w", err)
	}

	// Register mock PowerStore adapter with custom config
	powerstoreFactory := NewMockPowerStoreAdapterFactory(powerstoreConfig)
	if err := registry.RegisterFactory(powerstoreFactory); err != nil {
		return fmt.Errorf("failed to register mock PowerStore adapter: %w", err)
	}

	return nil
}

// UnregisterMockAdapters removes mock adapters from the global registry
func UnregisterMockAdapters() error {
	registry := GetGlobalRegistry()

	// Unregister mock adapters
	registry.UnregisterFactory(translation.BackendTrident)
	registry.UnregisterFactory(translation.BackendPowerStore)

	return nil
}

// IsMockAdapterEnabled checks if mock adapters should be enabled
// This can be controlled via environment variables or build tags
func IsMockAdapterEnabled() bool {
	// Check environment variable
	if enabled := os.Getenv("ENABLE_MOCK_ADAPTERS"); enabled == "true" {
		return true
	}

	// Check for testing environment
	if testing := os.Getenv("GO_TEST"); testing != "" {
		return true
	}

	return false
}

// AutoRegisterMockAdapters automatically registers mock adapters if enabled
func AutoRegisterMockAdapters() error {
	if IsMockAdapterEnabled() {
		return RegisterMockAdapters()
	}
	return nil
}

// CreateMockTestEnvironment creates a testing environment with mock adapters
func CreateMockTestEnvironment() error {
	// Register mock adapters with testing-friendly configurations
	tridentConfig := &MockTridentConfig{
		CreateSuccessRate:    1.0, // Always succeed in tests
		UpdateSuccessRate:    1.0,
		DeleteSuccessRate:    1.0,
		StatusSuccessRate:    1.0,
		MinLatency:           0, // No latency in tests
		MaxLatency:           0,
		StateTransitionDelay: 100 * time.Millisecond, // Fast transitions
		AutoProgressStates:   false,                  // Disable for deterministic tests
		HealthFluctuation:    false,                  // Stable health in tests
		HealthCheckInterval:  time.Second,
		ThroughputMBps:       1000.0, // High throughput
		ErrorInjectionRate:   0.0,    // No random errors
	}

	powerstoreConfig := &MockPowerStoreConfig{
		CreateSuccessRate:    1.0,
		UpdateSuccessRate:    1.0,
		DeleteSuccessRate:    1.0,
		StatusSuccessRate:    1.0,
		MinLatency:           0,
		MaxLatency:           0,
		StateTransitionDelay: 100 * time.Millisecond,
		AutoProgressStates:   false, // Disable for deterministic tests
		HealthFluctuation:    false, // Stable for tests
		HealthCheckInterval:  time.Second,
		ThroughputMBps:       1000.0,
		ErrorInjectionRate:   0.0,
		MetroLatencyMs:       1,
		RPOComplianceMin:     99.0,
		RPOComplianceMax:     99.9,
		SessionFailureRate:   0.0,
	}

	return RegisterMockAdaptersWithConfig(tridentConfig, powerstoreConfig)
}

// CreateMockFailureTestEnvironment creates a testing environment with failure simulation
func CreateMockFailureTestEnvironment() error {
	// Register mock adapters with failure-prone configurations
	tridentConfig := &MockTridentConfig{
		CreateSuccessRate:    0.8, // Some failures
		UpdateSuccessRate:    0.9,
		DeleteSuccessRate:    0.95,
		StatusSuccessRate:    0.85,
		MinLatency:           50 * time.Millisecond,
		MaxLatency:           200 * time.Millisecond,
		StateTransitionDelay: 500 * time.Millisecond, // Faster for tests
		AutoProgressStates:   false,                  // Disable for controlled testing
		HealthFluctuation:    false,                  // Stable for tests
		HealthCheckInterval:  5 * time.Second,
		ThroughputMBps:       50.0,
		ErrorInjectionRate:   0.1, // 10% error injection
	}

	powerstoreConfig := &MockPowerStoreConfig{
		CreateSuccessRate:    0.85,
		UpdateSuccessRate:    0.9,
		DeleteSuccessRate:    0.95,
		StatusSuccessRate:    0.8,
		MinLatency:           30 * time.Millisecond,
		MaxLatency:           300 * time.Millisecond,
		StateTransitionDelay: 500 * time.Millisecond, // Faster for tests
		AutoProgressStates:   false,                  // Disable for controlled testing
		HealthFluctuation:    false,                  // Stable for tests
		HealthCheckInterval:  5 * time.Second,
		ThroughputMBps:       75.0,
		ErrorInjectionRate:   0.15,
		MetroLatencyMs:       10,
		RPOComplianceMin:     85.0,
		RPOComplianceMax:     95.0,
		SessionFailureRate:   0.05,
	}

	return RegisterMockAdaptersWithConfig(tridentConfig, powerstoreConfig)
}

// CreateMockPerformanceTestEnvironment creates a testing environment for performance testing
func CreateMockPerformanceTestEnvironment() error {
	// Register mock adapters with performance-focused configurations
	tridentConfig := &MockTridentConfig{
		CreateSuccessRate:    0.99,
		UpdateSuccessRate:    0.99,
		DeleteSuccessRate:    0.99,
		StatusSuccessRate:    0.99,
		MinLatency:           1 * time.Microsecond, // Very low latency
		MaxLatency:           10 * time.Microsecond,
		StateTransitionDelay: 50 * time.Millisecond,
		AutoProgressStates:   false, // Disable for predictable benchmarks
		HealthFluctuation:    false,
		HealthCheckInterval:  100 * time.Millisecond,
		ThroughputMBps:       5000.0, // Very high throughput
		ErrorInjectionRate:   0.001,  // Minimal errors
	}

	powerstoreConfig := &MockPowerStoreConfig{
		CreateSuccessRate:    0.99,
		UpdateSuccessRate:    0.99,
		DeleteSuccessRate:    0.99,
		StatusSuccessRate:    0.99,
		MinLatency:           1 * time.Microsecond,
		MaxLatency:           5 * time.Microsecond,
		StateTransitionDelay: 25 * time.Millisecond,
		AutoProgressStates:   false, // Disable for predictable benchmarks
		HealthFluctuation:    false,
		HealthCheckInterval:  100 * time.Millisecond,
		ThroughputMBps:       10000.0,
		ErrorInjectionRate:   0.001,
		MetroLatencyMs:       1,
		RPOComplianceMin:     99.5,
		RPOComplianceMax:     99.99,
		SessionFailureRate:   0.0001,
	}

	return RegisterMockAdaptersWithConfig(tridentConfig, powerstoreConfig)
}
