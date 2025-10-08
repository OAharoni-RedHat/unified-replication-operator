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
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/translation"
)

// AdapterFactory defines the interface for creating adapters
type AdapterFactory interface {
	CreateAdapter(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig) (ReplicationAdapter, error)
	GetBackendType() translation.Backend
	GetInfo() AdapterFactoryInfo
	ValidateConfig(config *AdapterConfig) error
}

// AdapterFactoryInfo provides information about an adapter factory
type AdapterFactoryInfo struct {
	Name        string              `json:"name"`
	Backend     translation.Backend `json:"backend"`
	Version     string              `json:"version"`
	Description string              `json:"description"`
	Author      string              `json:"author,omitempty"`
}

// Registry manages adapter factories and provides adapter creation services
type Registry interface {
	// Factory management
	RegisterFactory(factory AdapterFactory) error
	UnregisterFactory(backend translation.Backend) error
	GetFactory(backend translation.Backend) (AdapterFactory, error)
	ListFactories() []AdapterFactory

	// Adapter management
	CreateAdapter(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig) (ReplicationAdapter, error)
	GetAdapterInfo(backend translation.Backend) (*AdapterFactoryInfo, error)
	IsBackendSupported(backend translation.Backend) bool
	GetSupportedBackends() []translation.Backend

	// Lifecycle management
	Initialize(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// DefaultRegistry implements the Registry interface
type DefaultRegistry struct {
	factories   map[translation.Backend]AdapterFactory
	mu          sync.RWMutex
	initialized bool
}

// NewRegistry creates a new adapter registry
func NewRegistry() Registry {
	return &DefaultRegistry{
		factories: make(map[translation.Backend]AdapterFactory),
	}
}

// RegisterFactory registers an adapter factory
func (r *DefaultRegistry) RegisterFactory(factory AdapterFactory) error {
	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}

	backend := factory.GetBackendType()
	if backend == "" {
		return fmt.Errorf("factory backend type cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[backend]; exists {
		return fmt.Errorf("factory for backend %s already registered", backend)
	}

	r.factories[backend] = factory
	return nil
}

// UnregisterFactory removes an adapter factory
func (r *DefaultRegistry) UnregisterFactory(backend translation.Backend) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[backend]; !exists {
		return fmt.Errorf("no factory registered for backend %s", backend)
	}

	delete(r.factories, backend)
	return nil
}

// GetFactory returns the factory for a specific backend
func (r *DefaultRegistry) GetFactory(backend translation.Backend) (AdapterFactory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[backend]
	if !exists {
		return nil, fmt.Errorf("no factory registered for backend %s", backend)
	}

	return factory, nil
}

// ListFactories returns all registered factories
func (r *DefaultRegistry) ListFactories() []AdapterFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factories := make([]AdapterFactory, 0, len(r.factories))
	for _, factory := range r.factories {
		factories = append(factories, factory)
	}

	return factories
}

// CreateAdapter creates an adapter for the specified backend
func (r *DefaultRegistry) CreateAdapter(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig) (ReplicationAdapter, error) {
	factory, err := r.GetFactory(backend)
	if err != nil {
		return nil, fmt.Errorf("failed to get factory for backend %s: %w", backend, err)
	}

	if config == nil {
		config = DefaultAdapterConfig(backend)
	}

	// Validate configuration using the factory
	if err := factory.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed for backend %s: %w", backend, err)
	}

	adapter, err := factory.CreateAdapter(backend, client, translator, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter for backend %s: %w", backend, err)
	}

	return adapter, nil
}

// GetAdapterInfo returns information about an adapter factory
func (r *DefaultRegistry) GetAdapterInfo(backend translation.Backend) (*AdapterFactoryInfo, error) {
	factory, err := r.GetFactory(backend)
	if err != nil {
		return nil, err
	}

	info := factory.GetInfo()
	return &info, nil
}

// IsBackendSupported checks if a backend is supported
func (r *DefaultRegistry) IsBackendSupported(backend translation.Backend) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.factories[backend]
	return exists
}

// GetSupportedBackends returns all supported backends
func (r *DefaultRegistry) GetSupportedBackends() []translation.Backend {
	r.mu.RLock()
	defer r.mu.RUnlock()

	backends := make([]translation.Backend, 0, len(r.factories))
	for backend := range r.factories {
		backends = append(backends, backend)
	}

	return backends
}

// Initialize initializes the registry
func (r *DefaultRegistry) Initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return nil
	}

	logger := log.FromContext(ctx).WithName("adapter-registry")
	logger.Info("Initializing adapter registry", "factories", len(r.factories))

	// Validate all registered factories
	for backend, factory := range r.factories {
		if factory.GetBackendType() != backend {
			return fmt.Errorf("factory backend type mismatch: expected %s, got %s", backend, factory.GetBackendType())
		}
	}

	r.initialized = true
	logger.Info("Adapter registry initialized successfully")
	return nil
}

// Shutdown shuts down the registry
func (r *DefaultRegistry) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	logger := log.FromContext(ctx).WithName("adapter-registry")
	logger.Info("Shutting down adapter registry")

	r.initialized = false
	logger.Info("Adapter registry shutdown completed")
	return nil
}

// BaseAdapterFactory provides a base implementation for adapter factories
type BaseAdapterFactory struct {
	info AdapterFactoryInfo
}

// NewBaseAdapterFactory creates a new base adapter factory
func NewBaseAdapterFactory(backend translation.Backend, name, version, description string) *BaseAdapterFactory {
	return &BaseAdapterFactory{
		info: AdapterFactoryInfo{
			Name:        name,
			Backend:     backend,
			Version:     version,
			Description: description,
		},
	}
}

// GetBackendType returns the backend type
func (f *BaseAdapterFactory) GetBackendType() translation.Backend {
	return f.info.Backend
}

// GetInfo returns factory information
func (f *BaseAdapterFactory) GetInfo() AdapterFactoryInfo {
	return f.info
}

// ValidateConfig validates the adapter configuration
func (f *BaseAdapterFactory) ValidateConfig(config *AdapterConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.Backend != f.info.Backend {
		return fmt.Errorf("config backend %s does not match factory backend %s", config.Backend, f.info.Backend)
	}

	return nil
}

// CreateAdapter creates a base adapter (should be overridden by specific implementations)
func (f *BaseAdapterFactory) CreateAdapter(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig) (ReplicationAdapter, error) {
	return NewBaseAdapter(backend, client, translator, config), nil
}

// AdapterManager provides high-level adapter management functionality
type AdapterManager struct {
	registry Registry
	adapters map[string]ReplicationAdapter // keyed by instance identifier
	mu       sync.RWMutex
	config   *ManagerConfig
}

// ManagerConfig contains configuration for the adapter manager
type ManagerConfig struct {
	DefaultTimeout       time.Duration
	DefaultRetryAttempts int
	HealthCheckEnabled   bool
	MetricsEnabled       bool
}

// DefaultManagerConfig returns the default manager configuration
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		DefaultTimeout:       30 * time.Second,
		DefaultRetryAttempts: 3,
		HealthCheckEnabled:   true,
		MetricsEnabled:       true,
	}
}

// NewAdapterManager creates a new adapter manager
func NewAdapterManager(registry Registry, config *ManagerConfig) *AdapterManager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	return &AdapterManager{
		registry: registry,
		adapters: make(map[string]ReplicationAdapter),
		config:   config,
	}
}

// GetOrCreateAdapter gets an existing adapter or creates a new one
func (m *AdapterManager) GetOrCreateAdapter(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication, client client.Client, translator *translation.Engine) (ReplicationAdapter, error) {
	instanceKey := m.getInstanceKey(uvr)

	m.mu.RLock()
	if adapter, exists := m.adapters[instanceKey]; exists {
		m.mu.RUnlock()
		return adapter, nil
	}
	m.mu.RUnlock()

	// Determine backend type from UVR
	backend, err := m.determineBackend(uvr)
	if err != nil {
		return nil, fmt.Errorf("failed to determine backend for %s: %w", uvr.Name, err)
	}

	// Create adapter configuration
	config := m.createAdapterConfig(backend, uvr)

	// Create adapter
	adapter, err := m.registry.CreateAdapter(backend, client, translator, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter for %s: %w", uvr.Name, err)
	}

	// Initialize adapter
	if err := adapter.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize adapter for %s: %w", uvr.Name, err)
	}

	// Store adapter
	m.mu.Lock()
	m.adapters[instanceKey] = adapter
	m.mu.Unlock()

	return adapter, nil
}

// RemoveAdapter removes an adapter instance
func (m *AdapterManager) RemoveAdapter(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	instanceKey := m.getInstanceKey(uvr)

	m.mu.Lock()
	adapter, exists := m.adapters[instanceKey]
	if exists {
		delete(m.adapters, instanceKey)
	}
	m.mu.Unlock()

	if exists {
		return adapter.Cleanup(ctx)
	}

	return nil
}

// GetAdapter returns an existing adapter
func (m *AdapterManager) GetAdapter(uvr *replicationv1alpha1.UnifiedVolumeReplication) (ReplicationAdapter, bool) {
	instanceKey := m.getInstanceKey(uvr)

	m.mu.RLock()
	defer m.mu.RUnlock()

	adapter, exists := m.adapters[instanceKey]
	return adapter, exists
}

// Shutdown shuts down all adapters
func (m *AdapterManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	adapters := make([]ReplicationAdapter, 0, len(m.adapters))
	for _, adapter := range m.adapters {
		adapters = append(adapters, adapter)
	}
	m.adapters = make(map[string]ReplicationAdapter)
	m.mu.Unlock()

	// Cleanup all adapters
	for _, adapter := range adapters {
		if err := adapter.Cleanup(ctx); err != nil {
			log.FromContext(ctx).Error(err, "Failed to cleanup adapter", "backend", adapter.GetBackendType())
		}
	}

	return nil
}

// GetStats returns statistics for all managed adapters
func (m *AdapterManager) GetStats() map[string]AdapterStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]AdapterStats)
	for key, adapter := range m.adapters {
		if statsProvider, ok := adapter.(*BaseAdapter); ok {
			stats[key] = statsProvider.GetStats()
		}
	}

	return stats
}

// getInstanceKey creates a unique key for an adapter instance
func (m *AdapterManager) getInstanceKey(uvr *replicationv1alpha1.UnifiedVolumeReplication) string {
	return fmt.Sprintf("%s/%s", uvr.Namespace, uvr.Name)
}

// determineBackend determines the backend type from a UnifiedVolumeReplication
func (m *AdapterManager) determineBackend(uvr *replicationv1alpha1.UnifiedVolumeReplication) (translation.Backend, error) {
	// This is a simplified implementation - in practice, this would analyze
	// storage classes, annotations, or other indicators to determine the backend

	// Check extensions for explicit backend configuration
	if uvr.Spec.Extensions != nil {
		if uvr.Spec.Extensions.Ceph != nil {
			return translation.BackendCeph, nil
		}
		if uvr.Spec.Extensions.Trident != nil {
			return translation.BackendTrident, nil
		}
		if uvr.Spec.Extensions.Powerstore != nil {
			return translation.BackendPowerStore, nil
		}
	}

	// Default fallback - in practice, this should never be reached
	return "", fmt.Errorf("unable to determine backend type for %s", uvr.Name)
}

// createAdapterConfig creates adapter configuration based on UVR and manager settings
func (m *AdapterManager) createAdapterConfig(backend translation.Backend, uvr *replicationv1alpha1.UnifiedVolumeReplication) *AdapterConfig {
	config := DefaultAdapterConfig(backend)

	// Apply manager-level defaults
	config.Timeout = m.config.DefaultTimeout
	config.RetryAttempts = m.config.DefaultRetryAttempts
	config.HealthCheckEnabled = m.config.HealthCheckEnabled
	config.MetricsEnabled = m.config.MetricsEnabled

	// Apply UVR-specific customizations
	if uvr.Spec.Schedule.Rto != "" {
		// Parse RTO and adjust timeout accordingly
		// This is a simplified implementation
	}

	return config
}

// Global registry instance
var globalRegistry Registry
var registryOnce sync.Once

// GetGlobalRegistry returns the global adapter registry
func GetGlobalRegistry() Registry {
	registryOnce.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}

// RegisterAdapter is a convenience function to register an adapter factory globally
func RegisterAdapter(factory AdapterFactory) error {
	return GetGlobalRegistry().RegisterFactory(factory)
}

// CreateAdapterForBackend is a convenience function to create an adapter from the global registry
func CreateAdapterForBackend(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig) (ReplicationAdapter, error) {
	return GetGlobalRegistry().CreateAdapter(backend, client, translator, config)
}
