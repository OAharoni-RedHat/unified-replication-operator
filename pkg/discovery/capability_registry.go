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

package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/unified-replication/operator/pkg/translation"
)

// InMemoryCapabilityRegistry implements CapabilityRegistry using in-memory storage
type InMemoryCapabilityRegistry struct {
	capabilities map[translation.Backend]*BackendCapabilities
	mu           sync.RWMutex
	detectors    map[translation.Backend]CapabilityDetector
}

// NewInMemoryCapabilityRegistry creates a new in-memory capability registry
func NewInMemoryCapabilityRegistry() *InMemoryCapabilityRegistry {
	return &InMemoryCapabilityRegistry{
		capabilities: make(map[translation.Backend]*BackendCapabilities),
		detectors:    make(map[translation.Backend]CapabilityDetector),
	}
}

// RegisterCapabilities registers capabilities for a backend
func (r *InMemoryCapabilityRegistry) RegisterCapabilities(backend translation.Backend, capabilities *BackendCapabilities) error {
	if capabilities == nil {
		return fmt.Errorf("capabilities cannot be nil for backend %s", backend)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	capabilities.LastUpdated = time.Now()
	r.capabilities[backend] = capabilities
	return nil
}

// GetCapabilities returns capabilities for a backend
func (r *InMemoryCapabilityRegistry) GetCapabilities(backend translation.Backend) (*BackendCapabilities, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	capabilities, exists := r.capabilities[backend]
	return capabilities, exists
}

// GetAllCapabilities returns capabilities for all registered backends
func (r *InMemoryCapabilityRegistry) GetAllCapabilities() map[translation.Backend]*BackendCapabilities {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[translation.Backend]*BackendCapabilities)
	for backend, capabilities := range r.capabilities {
		result[backend] = capabilities
	}
	return result
}

// UpdateCapabilities updates capabilities for a backend
func (r *InMemoryCapabilityRegistry) UpdateCapabilities(backend translation.Backend, capabilities *BackendCapabilities) error {
	if capabilities == nil {
		return fmt.Errorf("capabilities cannot be nil for backend %s", backend)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Merge with existing capabilities if they exist
	if existing, exists := r.capabilities[backend]; exists {
		// Keep some existing metadata if not provided in update
		if capabilities.Version == "" {
			capabilities.Version = existing.Version
		}
		// Merge capabilities that aren't explicitly updated
		for cap, info := range existing.Capabilities {
			if _, exists := capabilities.Capabilities[cap]; !exists {
				if capabilities.Capabilities == nil {
					capabilities.Capabilities = make(map[BackendCapability]CapabilityInfo)
				}
				capabilities.Capabilities[cap] = info
			}
		}
	}

	capabilities.LastUpdated = time.Now()
	r.capabilities[backend] = capabilities
	return nil
}

// RefreshCapabilities refreshes capabilities for a backend
func (r *InMemoryCapabilityRegistry) RefreshCapabilities(ctx context.Context, backend translation.Backend) error {
	r.mu.RLock()
	detector, exists := r.detectors[backend]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no detector registered for backend %s", backend)
	}

	capabilities, err := detector.DetectCapabilities(ctx)
	if err != nil {
		return fmt.Errorf("failed to refresh capabilities for %s: %w", backend, err)
	}

	return r.UpdateCapabilities(backend, capabilities)
}

// RefreshAllCapabilities refreshes capabilities for all backends
func (r *InMemoryCapabilityRegistry) RefreshAllCapabilities(ctx context.Context) error {
	r.mu.RLock()
	backends := make([]translation.Backend, 0, len(r.detectors))
	for backend := range r.detectors {
		backends = append(backends, backend)
	}
	r.mu.RUnlock()

	var errors []error
	for _, backend := range backends {
		if err := r.RefreshCapabilities(ctx, backend); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to refresh capabilities for some backends: %v", errors)
	}

	return nil
}

// IsCapabilitySupported checks if a backend supports a capability
func (r *InMemoryCapabilityRegistry) IsCapabilitySupported(backend translation.Backend, capability BackendCapability) (CapabilityLevel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	capabilities, exists := r.capabilities[backend]
	if !exists {
		return CapabilityLevelUnknown, false
	}

	capInfo, exists := capabilities.Capabilities[capability]
	if !exists {
		return CapabilityLevelNone, true
	}

	return capInfo.Level, true
}

// GetSupportedBackends returns backends that support a specific capability
func (r *InMemoryCapabilityRegistry) GetSupportedBackends(capability BackendCapability, minLevel CapabilityLevel) []translation.Backend {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var supportedBackends []translation.Backend

	for backend, capabilities := range r.capabilities {
		if capabilities == nil {
			continue
		}

		capInfo, exists := capabilities.Capabilities[capability]
		if !exists {
			continue
		}

		if r.isCapabilityLevelSufficient(capInfo.Level, minLevel) {
			supportedBackends = append(supportedBackends, backend)
		}
	}

	return supportedBackends
}

// isCapabilityLevelSufficient checks if a capability level meets the minimum requirement
func (r *InMemoryCapabilityRegistry) isCapabilityLevelSufficient(actual, required CapabilityLevel) bool {
	levelOrder := map[CapabilityLevel]int{
		CapabilityLevelNone:    0,
		CapabilityLevelUnknown: 1,
		CapabilityLevelBasic:   2,
		CapabilityLevelPartial: 3,
		CapabilityLevelFull:    4,
	}

	actualScore, actualExists := levelOrder[actual]
	requiredScore, requiredExists := levelOrder[required]

	if !actualExists || !requiredExists {
		return false
	}

	return actualScore >= requiredScore
}

// ValidateConfiguration validates if a configuration is supported by backend capabilities
func (r *InMemoryCapabilityRegistry) ValidateConfiguration(backend translation.Backend, config map[string]interface{}) error {
	r.mu.RLock()
	capabilities, exists := r.capabilities[backend]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no capabilities registered for backend %s", backend)
	}

	// Validate based on configuration requirements
	return r.validateConfigurationAgainstCapabilities(config, capabilities)
}

// validateConfigurationAgainstCapabilities validates a configuration against backend capabilities
func (r *InMemoryCapabilityRegistry) validateConfigurationAgainstCapabilities(config map[string]interface{}, capabilities *BackendCapabilities) error {
	// Check replication mode requirements
	if mode, exists := config["replicationMode"]; exists {
		if err := r.validateReplicationMode(mode, capabilities); err != nil {
			return err
		}
	}

	// Check state requirements
	if state, exists := config["replicationState"]; exists {
		if err := r.validateReplicationState(state, capabilities); err != nil {
			return err
		}
	}

	// Check extension requirements
	if extensions, exists := config["extensions"]; exists {
		if err := r.validateExtensions(extensions, capabilities); err != nil {
			return err
		}
	}

	return nil
}

// validateReplicationMode validates replication mode against capabilities
func (r *InMemoryCapabilityRegistry) validateReplicationMode(mode interface{}, capabilities *BackendCapabilities) error {
	modeStr, ok := mode.(string)
	if !ok {
		return fmt.Errorf("replication mode must be a string")
	}

	var requiredCapability BackendCapability
	switch modeStr {
	case "synchronous":
		requiredCapability = CapabilitySyncReplication
	case "asynchronous":
		requiredCapability = CapabilityAsyncReplication
	default:
		return fmt.Errorf("unknown replication mode: %s", modeStr)
	}

	capInfo, exists := capabilities.Capabilities[requiredCapability]
	if !exists || capInfo.Level == CapabilityLevelNone {
		return fmt.Errorf("backend %s does not support replication mode %s", capabilities.Backend, modeStr)
	}

	return nil
}

// validateReplicationState validates replication state against capabilities
func (r *InMemoryCapabilityRegistry) validateReplicationState(state interface{}, capabilities *BackendCapabilities) error {
	stateStr, ok := state.(string)
	if !ok {
		return fmt.Errorf("replication state must be a string")
	}

	var requiredCapability BackendCapability
	switch stateStr {
	case "promoting":
		requiredCapability = CapabilitySourcePromotion
	case "demoting":
		requiredCapability = CapabilityReplicaDemotion
	case "syncing":
		requiredCapability = CapabilityResync
	default:
		// Basic states like "source" and "replica" are assumed to be supported
		return nil
	}

	capInfo, exists := capabilities.Capabilities[requiredCapability]
	if !exists || capInfo.Level == CapabilityLevelNone {
		return fmt.Errorf("backend %s does not support replication state %s", capabilities.Backend, stateStr)
	}

	return nil
}

// validateExtensions validates extensions against capabilities
func (r *InMemoryCapabilityRegistry) validateExtensions(extensions interface{}, capabilities *BackendCapabilities) error {
	extMap, ok := extensions.(map[string]interface{})
	if !ok {
		return fmt.Errorf("extensions must be a map")
	}

	// Validate backend-specific extensions
	backendName := string(capabilities.Backend)
	if backendExt, exists := extMap[backendName]; exists {
		return r.validateBackendExtensions(backendExt, capabilities)
	}

	return nil
}

// validateBackendExtensions validates backend-specific extensions
func (r *InMemoryCapabilityRegistry) validateBackendExtensions(extensions interface{}, capabilities *BackendCapabilities) error {
	extMap, ok := extensions.(map[string]interface{})
	if !ok {
		return nil // Skip validation if not a map
	}

	switch capabilities.Backend {
	case translation.BackendCeph:
		return r.validateCephExtensions(extMap, capabilities)
	case translation.BackendTrident:
		return r.validateTridentExtensions(extMap, capabilities)
	case translation.BackendPowerStore:
		return r.validatePowerStoreExtensions(extMap, capabilities)
	}

	return nil
}

// validateCephExtensions validates Ceph-specific extensions
func (r *InMemoryCapabilityRegistry) validateCephExtensions(extensions map[string]interface{}, capabilities *BackendCapabilities) error {
	if mirroringMode, exists := extensions["mirroringMode"]; exists {
		mode, ok := mirroringMode.(string)
		if !ok {
			return fmt.Errorf("ceph mirroringMode must be a string")
		}

		var requiredCapability BackendCapability
		switch mode {
		case "journal":
			requiredCapability = CapabilityJournalBased
		case "snapshot":
			requiredCapability = CapabilitySnapshotBased
		default:
			return fmt.Errorf("unknown ceph mirroring mode: %s", mode)
		}

		capInfo, exists := capabilities.Capabilities[requiredCapability]
		if !exists || capInfo.Level == CapabilityLevelNone {
			return fmt.Errorf("ceph backend does not support mirroring mode %s", mode)
		}
	}

	return nil
}

// validateTridentExtensions validates Trident-specific extensions
func (r *InMemoryCapabilityRegistry) validateTridentExtensions(extensions map[string]interface{}, capabilities *BackendCapabilities) error {
	// Trident extensions are generally supported if the backend is available
	// More specific validation could be added based on Trident capabilities
	return nil
}

// validatePowerStoreExtensions validates PowerStore-specific extensions
func (r *InMemoryCapabilityRegistry) validatePowerStoreExtensions(extensions map[string]interface{}, capabilities *BackendCapabilities) error {
	return nil
}

// RegisterDetector registers a capability detector for a backend
func (r *InMemoryCapabilityRegistry) RegisterDetector(backend translation.Backend, detector CapabilityDetector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detectors[backend] = detector
}

// GetStatistics returns statistics about the registry
func (r *InMemoryCapabilityRegistry) GetStatistics() RegistryStatistics {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RegistryStatistics{
		TotalBackends:         len(r.capabilities),
		HealthyBackends:       0,
		UnhealthyBackends:     0,
		UnknownHealthBackends: 0,
		TotalCapabilities:     0,
		LastUpdated:           time.Time{},
	}

	for _, capabilities := range r.capabilities {
		if capabilities == nil {
			continue
		}

		switch capabilities.Health.Status {
		case HealthLevelHealthy:
			stats.HealthyBackends++
		case HealthLevelDegraded, HealthLevelUnhealthy:
			stats.UnhealthyBackends++
		default:
			stats.UnknownHealthBackends++
		}

		stats.TotalCapabilities += len(capabilities.Capabilities)

		if capabilities.LastUpdated.After(stats.LastUpdated) {
			stats.LastUpdated = capabilities.LastUpdated
		}
	}

	return stats
}

// RegistryStatistics provides statistics about the capability registry
type RegistryStatistics struct {
	TotalBackends         int       `json:"total_backends"`
	HealthyBackends       int       `json:"healthy_backends"`
	UnhealthyBackends     int       `json:"unhealthy_backends"`
	UnknownHealthBackends int       `json:"unknown_health_backends"`
	TotalCapabilities     int       `json:"total_capabilities"`
	LastUpdated           time.Time `json:"last_updated"`
}
