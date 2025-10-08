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

package pkg

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/adapters"
	"github.com/unified-replication/operator/pkg/discovery"
	"github.com/unified-replication/operator/pkg/translation"
)

// ControllerEngine coordinates discovery, translation, and adapter operations
type ControllerEngine struct {
	client            client.Client
	discoveryEngine   *discovery.Engine
	translationEngine *translation.Engine
	adapterRegistry   adapters.Registry

	// Caching
	discoveryCache      map[string]*discovery.DiscoveryResult
	discoveryCacheMutex sync.RWMutex
	cacheExpiry         time.Duration
	lastDiscoveryTime   time.Time

	// Configuration
	enableCaching   bool
	batchOperations bool

	// Metrics
	operationCount int64
	cacheHits      int64
	cacheMisses    int64
}

// ControllerEngineConfig configures the controller engine
type ControllerEngineConfig struct {
	EnableCaching     bool
	CacheExpiry       time.Duration
	BatchOperations   bool
	DiscoveryInterval time.Duration
}

// DefaultControllerEngineConfig returns default configuration
func DefaultControllerEngineConfig() *ControllerEngineConfig {
	return &ControllerEngineConfig{
		EnableCaching:     true,
		CacheExpiry:       5 * time.Minute,
		BatchOperations:   false, // Enable in future for optimization
		DiscoveryInterval: 1 * time.Minute,
	}
}

// NewControllerEngine creates a new controller engine
func NewControllerEngine(
	client client.Client,
	discoveryEngine *discovery.Engine,
	translationEngine *translation.Engine,
	adapterRegistry adapters.Registry,
	config *ControllerEngineConfig,
) *ControllerEngine {
	if config == nil {
		config = DefaultControllerEngineConfig()
	}

	return &ControllerEngine{
		client:            client,
		discoveryEngine:   discoveryEngine,
		translationEngine: translationEngine,
		adapterRegistry:   adapterRegistry,
		discoveryCache:    make(map[string]*discovery.DiscoveryResult),
		enableCaching:     config.EnableCaching,
		cacheExpiry:       config.CacheExpiry,
		batchOperations:   config.BatchOperations,
	}
}

// ProcessReplication executes the complete workflow for a replication
// Discovery → Validation → Translation → Adapter Selection → Backend Operation
func (ce *ControllerEngine) ProcessReplication(
	ctx context.Context,
	uvr *replicationv1alpha1.UnifiedVolumeReplication,
	operation string,
	log logr.Logger,
) error {
	log.Info("Processing replication with integrated engines",
		"operation", operation,
		"backend_hint", ce.getBackendHint(uvr))

	ce.operationCount++

	// Step 1: Discovery - Find available backends
	backends, err := ce.discoverBackends(ctx, log)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	log.V(1).Info("Discovered backends", "count", len(backends))

	// Step 2: Backend Selection - Choose appropriate backend
	selectedBackend, err := ce.selectBackend(ctx, uvr, backends, log)
	if err != nil {
		return fmt.Errorf("backend selection failed: %w", err)
	}

	log.Info("Selected backend", "backend", selectedBackend)

	// Step 3: Validation - Validate configuration against backend capabilities
	if err := ce.validateConfiguration(ctx, uvr, selectedBackend, log); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Step 4: Translation - Translate unified state to backend state
	translatedState, translatedMode, err := ce.translateToBackend(uvr, selectedBackend, log)
	if err != nil {
		return fmt.Errorf("translation failed: %w", err)
	}

	log.V(1).Info("Translated states",
		"state", translatedState,
		"mode", translatedMode,
		"backend", selectedBackend)

	// Step 5: Adapter Selection - Get the appropriate adapter
	adapter, err := ce.getAdapter(ctx, selectedBackend, log)
	if err != nil {
		return fmt.Errorf("adapter selection failed: %w", err)
	}

	// Step 6: Backend Operation - Execute the operation
	if err := ce.executeOperation(ctx, adapter, uvr, operation, log); err != nil {
		return fmt.Errorf("operation execution failed: %w", err)
	}

	log.Info("Successfully processed replication")
	return nil
}

// discoverBackends discovers available backends in the cluster
func (ce *ControllerEngine) discoverBackends(ctx context.Context, log logr.Logger) ([]translation.Backend, error) {
	// Check cache first
	if ce.enableCaching && time.Since(ce.lastDiscoveryTime) < ce.cacheExpiry {
		ce.discoveryCacheMutex.RLock()
		if len(ce.discoveryCache) > 0 {
			ce.cacheHits++
			backends := make([]translation.Backend, 0, len(ce.discoveryCache))
			for backend := range ce.discoveryCache {
				backends = append(backends, translation.Backend(backend))
			}
			ce.discoveryCacheMutex.RUnlock()
			log.V(1).Info("Using cached discovery results", "backends", backends)
			return backends, nil
		}
		ce.discoveryCacheMutex.RUnlock()
	}

	ce.cacheMisses++

	// Perform discovery
	result, err := ce.discoveryEngine.DiscoverBackends(ctx)
	if err != nil {
		return nil, err
	}

	// Update cache
	if ce.enableCaching {
		ce.discoveryCacheMutex.Lock()
		defer ce.discoveryCacheMutex.Unlock()

		ce.discoveryCache = make(map[string]*discovery.DiscoveryResult)
		for _, backend := range result.AvailableBackends {
			ce.discoveryCache[string(backend)] = result
		}
		ce.lastDiscoveryTime = time.Now()
	}

	return result.AvailableBackends, nil
}

// selectBackend chooses the appropriate backend for a replication
func (ce *ControllerEngine) selectBackend(
	ctx context.Context,
	uvr *replicationv1alpha1.UnifiedVolumeReplication,
	availableBackends []translation.Backend,
	log logr.Logger,
) (translation.Backend, error) {
	// Strategy 1: Use explicitly configured backend from extensions
	if uvr.Spec.Extensions != nil {
		if uvr.Spec.Extensions.Ceph != nil {
			return ce.validateBackendAvailable(translation.BackendCeph, availableBackends)
		}
		if uvr.Spec.Extensions.Trident != nil {
			return ce.validateBackendAvailable(translation.BackendTrident, availableBackends)
		}
		if uvr.Spec.Extensions.Powerstore != nil {
			return ce.validateBackendAvailable(translation.BackendPowerStore, availableBackends)
		}
	}

	// Strategy 2: Detect from storage class name
	storageClass := uvr.Spec.SourceEndpoint.StorageClass
	if storageClass != "" {
		backend, err := ce.detectBackendFromStorageClass(storageClass, availableBackends, log)
		if err == nil {
			return backend, nil
		}
		log.V(1).Info("Could not detect backend from storage class", "storageClass", storageClass)
	}

	// Strategy 3: Use first available backend
	if len(availableBackends) > 0 {
		log.Info("No explicit backend configured, using first available", "backend", availableBackends[0])
		return availableBackends[0], nil
	}

	return "", fmt.Errorf("no backends available and no explicit backend configured")
}

// validateBackendAvailable checks if a backend is in the available list
func (ce *ControllerEngine) validateBackendAvailable(backend translation.Backend, availableBackends []translation.Backend) (translation.Backend, error) {
	for _, available := range availableBackends {
		if available == backend {
			return backend, nil
		}
	}
	return "", fmt.Errorf("backend %s not available in cluster", backend)
}

// detectBackendFromStorageClass attempts to detect backend from storage class name
func (ce *ControllerEngine) detectBackendFromStorageClass(
	storageClass string,
	availableBackends []translation.Backend,
	log logr.Logger,
) (translation.Backend, error) {
	// Simple heuristic based on storage class naming conventions
	for _, backend := range availableBackends {
		switch backend {
		case translation.BackendCeph:
			if contains(storageClass, "ceph") || contains(storageClass, "rbd") {
				return backend, nil
			}
		case translation.BackendTrident:
			if contains(storageClass, "trident") || contains(storageClass, "netapp") || contains(storageClass, "ontap") {
				return backend, nil
			}
		case translation.BackendPowerStore:
			if contains(storageClass, "powerstore") || contains(storageClass, "dell") {
				return backend, nil
			}
		}
	}

	return "", fmt.Errorf("could not detect backend from storage class: %s", storageClass)
}

// validateConfiguration validates the configuration against backend capabilities
func (ce *ControllerEngine) validateConfiguration(
	ctx context.Context,
	uvr *replicationv1alpha1.UnifiedVolumeReplication,
	backend translation.Backend,
	log logr.Logger,
) error {
	// Get backend capabilities from discovery result
	result, err := ce.discoveryEngine.DiscoverBackend(ctx, backend)
	if err != nil {
		log.V(1).Info("Could not discover backend for validation", "backend", backend, "error", err)
		// Continue without capability validation
		return nil
	}

	if result == nil {
		return fmt.Errorf("backend %s discovery returned no result", backend)
	}

	if result.Status != "Ready" && result.Status != "Available" {
		log.V(1).Info("Backend not fully ready", "backend", backend, "status", result.Status)
		// Continue anyway - backend may still work
	}

	// For now, skip detailed capability validation
	// Full capability validation will be added when capability detection is enhanced
	log.V(1).Info("Configuration validated (basic check)", "backend", backend)
	return nil
}

// translateToBackend translates unified state/mode to backend-specific values
func (ce *ControllerEngine) translateToBackend(
	uvr *replicationv1alpha1.UnifiedVolumeReplication,
	backend translation.Backend,
	log logr.Logger,
) (string, string, error) {
	state, err := ce.translationEngine.TranslateStateToBackend(backend, string(uvr.Spec.ReplicationState))
	if err != nil {
		return "", "", fmt.Errorf("state translation failed: %w", err)
	}

	mode, err := ce.translationEngine.TranslateModeToBackend(backend, string(uvr.Spec.ReplicationMode))
	if err != nil {
		return "", "", fmt.Errorf("mode translation failed: %w", err)
	}

	log.V(1).Info("Translation successful",
		"unifiedState", uvr.Spec.ReplicationState,
		"backendState", state,
		"unifiedMode", uvr.Spec.ReplicationMode,
		"backendMode", mode)

	return state, mode, nil
}

// getAdapter retrieves the appropriate adapter for a backend
func (ce *ControllerEngine) getAdapter(
	ctx context.Context,
	backend translation.Backend,
	log logr.Logger,
) (adapters.ReplicationAdapter, error) {
	factory, err := ce.adapterRegistry.GetFactory(backend)
	if err != nil {
		return nil, fmt.Errorf("no factory found for backend %s: %w", backend, err)
	}

	adapter, err := factory.CreateAdapter(backend, ce.client, ce.translationEngine, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter for backend %s: %w", backend, err)
	}

	// Initialize adapter
	if err := adapter.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize adapter: %w", err)
	}

	log.V(1).Info("Adapter created and initialized", "backend", backend)
	return adapter, nil
}

// executeOperation executes the appropriate operation on the adapter
func (ce *ControllerEngine) executeOperation(
	ctx context.Context,
	adapter adapters.ReplicationAdapter,
	uvr *replicationv1alpha1.UnifiedVolumeReplication,
	operation string,
	log logr.Logger,
) error {
	switch operation {
	case "create":
		return adapter.CreateReplication(ctx, uvr)
	case "update":
		return adapter.UpdateReplication(ctx, uvr)
	case "delete":
		return adapter.DeleteReplication(ctx, uvr)
	case "sync":
		// Just get status, don't modify
		_, err := adapter.GetReplicationStatus(ctx, uvr)
		return err
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}
}

// GetReplicationStatus retrieves status from the backend with translation
func (ce *ControllerEngine) GetReplicationStatus(
	ctx context.Context,
	uvr *replicationv1alpha1.UnifiedVolumeReplication,
	log logr.Logger,
) (*adapters.ReplicationStatus, error) {
	// Discover backends
	backends, err := ce.discoverBackends(ctx, log)
	if err != nil {
		return nil, err
	}

	// Select backend
	backend, err := ce.selectBackend(ctx, uvr, backends, log)
	if err != nil {
		return nil, err
	}

	// Get adapter
	adapter, err := ce.getAdapter(ctx, backend, log)
	if err != nil {
		return nil, err
	}

	// Get status from adapter
	status, err := adapter.GetReplicationStatus(ctx, uvr)
	if err != nil {
		return nil, err
	}

	// Translate status back to unified format
	if status != nil && ce.translationEngine != nil {
		unifiedState, err := ce.translationEngine.TranslateStateFromBackend(backend, status.State)
		if err == nil {
			status.State = unifiedState
		}

		unifiedMode, err := ce.translationEngine.TranslateModeFromBackend(backend, status.Mode)
		if err == nil {
			status.Mode = unifiedMode
		}
	}

	return status, nil
}

// InvalidateCache invalidates the discovery cache
func (ce *ControllerEngine) InvalidateCache() {
	ce.discoveryCacheMutex.Lock()
	defer ce.discoveryCacheMutex.Unlock()

	ce.discoveryCache = make(map[string]*discovery.DiscoveryResult)
	ce.lastDiscoveryTime = time.Time{}
}

// GetMetrics returns engine metrics
func (ce *ControllerEngine) GetMetrics() map[string]interface{} {
	ce.discoveryCacheMutex.RLock()
	defer ce.discoveryCacheMutex.RUnlock()

	return map[string]interface{}{
		"operation_count": ce.operationCount,
		"cache_hits":      ce.cacheHits,
		"cache_misses":    ce.cacheMisses,
		"cache_entries":   len(ce.discoveryCache),
		"last_discovery":  ce.lastDiscoveryTime,
	}
}

// getBackendHint extracts a backend hint from the UVR
func (ce *ControllerEngine) getBackendHint(uvr *replicationv1alpha1.UnifiedVolumeReplication) string {
	if uvr.Spec.Extensions == nil {
		return "auto"
	}
	if uvr.Spec.Extensions.Ceph != nil {
		return "ceph"
	}
	if uvr.Spec.Extensions.Trident != nil {
		return "trident"
	}
	if uvr.Spec.Extensions.Powerstore != nil {
		return "powerstore"
	}
	return "auto"
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstr(s, substr)))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
