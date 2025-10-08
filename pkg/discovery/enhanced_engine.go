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

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/unified-replication/operator/pkg/translation"
)

// EnhancedEngine extends the basic discovery engine with capability detection
type EnhancedEngine struct {
	*Engine             // Embed the basic engine
	capabilityConfig    *CapabilityConfig
	capabilityRegistry  CapabilityRegistry
	capabilityDetectors map[translation.Backend]CapabilityDetector
	healthMonitor       *HealthMonitor
}

// NewEnhancedEngine creates a new enhanced discovery engine
func NewEnhancedEngine(client client.Client, config *DiscoveryConfig, capConfig *CapabilityConfig) *EnhancedEngine {
	if capConfig == nil {
		capConfig = DefaultCapabilityConfig()
	}

	baseEngine := NewEngine(client, config)

	enhanced := &EnhancedEngine{
		Engine:              baseEngine,
		capabilityConfig:    capConfig,
		capabilityRegistry:  NewInMemoryCapabilityRegistry(),
		capabilityDetectors: make(map[translation.Backend]CapabilityDetector),
	}

	// Initialize capability detectors
	enhanced.initializeCapabilityDetectors()

	// Initialize health monitor if enabled
	if capConfig.EnableHealthChecking {
		enhanced.healthMonitor = NewHealthMonitor(enhanced, capConfig)
	}

	return enhanced
}

// initializeCapabilityDetectors creates capability detectors for each backend
func (e *EnhancedEngine) initializeCapabilityDetectors() {
	e.capabilityDetectors[translation.BackendCeph] = NewCephCapabilityDetector(e.client)
	e.capabilityDetectors[translation.BackendTrident] = NewTridentCapabilityDetector(e.client)
	e.capabilityDetectors[translation.BackendPowerStore] = NewPowerStoreCapabilityDetector(e.client)
}

// DiscoverBackendsWithCapabilities discovers backends with full capability detection
func (e *EnhancedEngine) DiscoverBackendsWithCapabilities(ctx context.Context) (*EnhancedDiscoveryResult, error) {
	logger := log.FromContext(ctx).WithName("enhanced-discovery")
	logger.Info("Starting enhanced backend discovery with capabilities")

	// First perform basic discovery
	basicResult, err := e.DiscoverBackends(ctx)
	if err != nil {
		return nil, fmt.Errorf("basic discovery failed: %w", err)
	}

	// Create enhanced result
	enhancedResult := &EnhancedDiscoveryResult{
		DiscoveryResult: basicResult,
		Capabilities:    make(map[translation.Backend]*BackendCapabilities),
		Performance:     make(map[translation.Backend]*PerformanceCharacteristics),
		Versions:        make(map[translation.Backend]*VersionInfo),
	}

	// Only detect capabilities for available backends
	if len(basicResult.AvailableBackends) == 0 {
		logger.Info("No available backends found, skipping capability detection")
		return enhancedResult, nil
	}

	// Detect capabilities for available backends in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, backend := range basicResult.AvailableBackends {
		wg.Add(1)
		go func(backend translation.Backend) {
			defer wg.Done()

			capCtx, cancel := context.WithTimeout(ctx, e.capabilityConfig.TimeoutPerCheck)
			defer cancel()

			capabilities, err := e.detectBackendCapabilities(capCtx, backend)

			mu.Lock()
			if err != nil {
				logger.Error(err, "Failed to detect capabilities", "backend", backend)
			} else {
				enhancedResult.Capabilities[backend] = capabilities
				// Register capabilities in registry
				_ = e.capabilityRegistry.RegisterCapabilities(backend, capabilities)
			}

			// Detect performance characteristics if enabled
			if e.capabilityConfig.EnablePerformanceMetrics {
				perf, err := e.detectPerformanceCharacteristics(capCtx, backend)
				if err != nil {
					logger.V(1).Info("Failed to detect performance characteristics", "backend", backend, "error", err)
				} else {
					enhancedResult.Performance[backend] = perf
				}
			}

			// Detect version information if enabled
			if e.capabilityConfig.EnableVersionDetection {
				version, err := e.detectVersionInfo(capCtx, backend)
				if err != nil {
					logger.V(1).Info("Failed to detect version info", "backend", backend, "error", err)
				} else {
					enhancedResult.Versions[backend] = version
				}
			}
			mu.Unlock()
		}(backend)
	}

	wg.Wait()

	logger.Info("Enhanced discovery completed",
		"available_backends", len(basicResult.AvailableBackends),
		"capabilities_detected", len(enhancedResult.Capabilities))

	return enhancedResult, nil
}

// detectBackendCapabilities detects capabilities for a specific backend
func (e *EnhancedEngine) detectBackendCapabilities(ctx context.Context, backend translation.Backend) (*BackendCapabilities, error) {
	detector, exists := e.capabilityDetectors[backend]
	if !exists {
		return nil, fmt.Errorf("no capability detector for backend %s", backend)
	}

	capabilities, err := detector.DetectCapabilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("capability detection failed for %s: %w", backend, err)
	}

	// Perform health check if enabled
	if e.capabilityConfig.EnableHealthChecking {
		health, err := detector.CheckHealth(ctx)
		if err != nil {
			// Don't fail capability detection if health check fails
			health = &HealthStatus{
				Status:      HealthLevelUnknown,
				Message:     fmt.Sprintf("Health check failed: %v", err),
				LastChecked: time.Now(),
			}
		}
		capabilities.Health = *health
	}

	return capabilities, nil
}

// detectPerformanceCharacteristics detects performance characteristics for a backend
func (e *EnhancedEngine) detectPerformanceCharacteristics(ctx context.Context, backend translation.Backend) (*PerformanceCharacteristics, error) {
	detector, exists := e.capabilityDetectors[backend]
	if !exists {
		return nil, fmt.Errorf("no capability detector for backend %s", backend)
	}

	return detector.GetPerformanceCharacteristics(ctx)
}

// detectVersionInfo detects version information for a backend
func (e *EnhancedEngine) detectVersionInfo(ctx context.Context, backend translation.Backend) (*VersionInfo, error) {
	detector, exists := e.capabilityDetectors[backend]
	if !exists {
		return nil, fmt.Errorf("no capability detector for backend %s", backend)
	}

	return detector.GetVersionInfo(ctx)
}

// GetBackendCapabilities returns capabilities for a specific backend
func (e *EnhancedEngine) GetBackendCapabilities(backend translation.Backend) (*BackendCapabilities, bool) {
	return e.capabilityRegistry.GetCapabilities(backend)
}

// QueryBackendsByCapabilities finds backends that match capability requirements
func (e *EnhancedEngine) QueryBackendsByCapabilities(query CapabilityQuery) ([]CapabilityQueryResult, error) {
	allCapabilities := e.capabilityRegistry.GetAllCapabilities()
	var results []CapabilityQueryResult

	for backend, capabilities := range allCapabilities {
		if capabilities == nil {
			continue
		}

		result := e.evaluateBackendForQuery(backend, capabilities, query)
		if result.Score > 0 {
			results = append(results, result)
		}
	}

	// Sort by score (highest first)
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results, nil
}

// evaluateBackendForQuery evaluates how well a backend matches a capability query
func (e *EnhancedEngine) evaluateBackendForQuery(backend translation.Backend, capabilities *BackendCapabilities, query CapabilityQuery) CapabilityQueryResult {
	result := CapabilityQueryResult{
		Backend:      backend,
		Capabilities: capabilities,
		Score:        0,
		Reasons:      make([]string, 0),
	}

	// Check health requirement
	if query.RequireHealthy && capabilities.Health.Status != HealthLevelHealthy {
		result.Reasons = append(result.Reasons, fmt.Sprintf("backend is not healthy: %s", capabilities.Health.Status))
		return result
	}

	// Check required capabilities
	requiredScore := 0.0
	maxRequiredScore := float64(len(query.RequiredCapabilities))

	for _, reqCap := range query.RequiredCapabilities {
		capInfo, exists := capabilities.Capabilities[reqCap]
		if !exists || capInfo.Level == CapabilityLevelNone {
			result.Reasons = append(result.Reasons, fmt.Sprintf("missing required capability: %s", reqCap))
			return result // Fail immediately if required capability is missing
		}

		// Score based on capability level
		switch capInfo.Level {
		case CapabilityLevelFull:
			requiredScore += 1.0
		case CapabilityLevelPartial:
			requiredScore += 0.7
		case CapabilityLevelBasic:
			requiredScore += 0.5
		}
	}

	// Check optional capabilities (bonus points)
	optionalScore := 0.0
	maxOptionalScore := float64(len(query.OptionalCapabilities))

	for _, optCap := range query.OptionalCapabilities {
		capInfo, exists := capabilities.Capabilities[optCap]
		if exists && capInfo.Level != CapabilityLevelNone {
			switch capInfo.Level {
			case CapabilityLevelFull:
				optionalScore += 1.0
			case CapabilityLevelPartial:
				optionalScore += 0.7
			case CapabilityLevelBasic:
				optionalScore += 0.5
			}
		}
	}

	// Calculate final score
	baseScore := 0.0
	if maxRequiredScore > 0 {
		baseScore = requiredScore / maxRequiredScore
	} else {
		baseScore = 1.0 // No required capabilities
	}

	bonusScore := 0.0
	if maxOptionalScore > 0 {
		bonusScore = (optionalScore / maxOptionalScore) * 0.3 // Bonus up to 30%
	}

	result.Score = baseScore + bonusScore

	if result.Score > 0 {
		result.Reasons = append(result.Reasons, fmt.Sprintf("matches with score %.2f", result.Score))
	}

	return result
}

// ValidateBackendConfiguration validates if a configuration is supported by a backend
func (e *EnhancedEngine) ValidateBackendConfiguration(backend translation.Backend, config map[string]interface{}) error {
	return e.capabilityRegistry.ValidateConfiguration(backend, config)
}

// StartCapabilityMonitoring starts background capability and health monitoring
func (e *EnhancedEngine) StartCapabilityMonitoring(ctx context.Context) error {
	if e.healthMonitor != nil {
		return e.healthMonitor.Start(ctx)
	}
	return nil
}

// StopCapabilityMonitoring stops background monitoring
func (e *EnhancedEngine) StopCapabilityMonitoring() error {
	if e.healthMonitor != nil {
		return e.healthMonitor.Stop()
	}
	return nil
}

// GetCapabilityRegistry returns the capability registry
func (e *EnhancedEngine) GetCapabilityRegistry() CapabilityRegistry {
	return e.capabilityRegistry
}

// RefreshCapabilities refreshes capabilities for all available backends
func (e *EnhancedEngine) RefreshCapabilities(ctx context.Context) error {
	return e.capabilityRegistry.RefreshAllCapabilities(ctx)
}

// Close cleans up the enhanced discovery engine
func (e *EnhancedEngine) Close() error {
	var errs []error

	if err := e.StopCapabilityMonitoring(); err != nil {
		errs = append(errs, err)
	}

	if err := e.Engine.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("multiple errors during close: %v", errs)
	}

	return nil
}

// NewEnhancedDiscoverer creates a new enhanced discoverer instance
func NewEnhancedDiscoverer(client client.Client, config *DiscoveryConfig, capConfig *CapabilityConfig) *EnhancedEngine {
	return NewEnhancedEngine(client, config, capConfig)
}
