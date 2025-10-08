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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/unified-replication/operator/pkg/translation"
)

// Engine implements the Discoverer interface with Kubernetes API integration
type Engine struct {
	client    client.Client
	config    *DiscoveryConfig
	cache     *discoveryCache
	mu        sync.RWMutex
	stopCh    chan struct{}
	running   bool
	detectors map[translation.Backend]BackendDetector
}

// discoveryCache holds cached discovery results
type discoveryCache struct {
	result    *DiscoveryResult
	timestamp time.Time
	mu        sync.RWMutex
}

// NewEngine creates a new discovery engine with the specified client and configuration
func NewEngine(client client.Client, config *DiscoveryConfig) *Engine {
	if config == nil {
		config = DefaultDiscoveryConfig()
	}

	engine := &Engine{
		client:    client,
		config:    config,
		cache:     &discoveryCache{},
		stopCh:    make(chan struct{}),
		detectors: make(map[translation.Backend]BackendDetector),
	}

	// Initialize backend detectors
	engine.initializeDetectors()

	return engine
}

// initializeDetectors creates detectors for each supported backend
func (e *Engine) initializeDetectors() {
	e.detectors[translation.BackendCeph] = NewCephDetector(e.client)
	e.detectors[translation.BackendTrident] = NewTridentDetector(e.client)
	e.detectors[translation.BackendPowerStore] = NewPowerStoreDetector(e.client)
}

// DiscoverBackends discovers all available backends
func (e *Engine) DiscoverBackends(ctx context.Context) (*DiscoveryResult, error) {
	logger := log.FromContext(ctx).WithName("discovery-engine")
	logger.Info("Starting backend discovery")

	result := &DiscoveryResult{
		Backends:          make(map[translation.Backend]BackendDiscoveryResult),
		AvailableBackends: make([]translation.Backend, 0),
		Timestamp:         time.Now(),
	}

	// Discover each backend in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	var discoveryErrors []error

	for _, backend := range translation.GetSupportedBackends() {
		wg.Add(1)
		go func(backend translation.Backend) {
			defer wg.Done()

			// Create context with timeout for each backend
			backendCtx, cancel := context.WithTimeout(ctx, e.config.TimeoutPerBackend)
			defer cancel()

			backendResult, err := e.discoverBackendWithRetry(backendCtx, backend)

			mu.Lock()
			if err != nil {
				logger.Error(err, "Failed to discover backend", "backend", backend)
				discoveryErrors = append(discoveryErrors, err)
				// Still add a result with error status
				backendResult = &BackendDiscoveryResult{
					Backend:     backend,
					Status:      BackendStatusUnavailable,
					Message:     err.Error(),
					LastUpdated: time.Now(),
				}
			}

			result.Backends[backend] = *backendResult
			if backendResult.Status == BackendStatusAvailable {
				result.AvailableBackends = append(result.AvailableBackends, backend)
			}
			mu.Unlock()
		}(backend)
	}

	wg.Wait()

	// Set overall error if any backend discovery failed
	if len(discoveryErrors) > 0 {
		result.Error = fmt.Sprintf("Failed to discover %d backends", len(discoveryErrors))
	}

	// Update cache
	e.updateCache(result)

	logger.Info("Backend discovery completed",
		"available", len(result.AvailableBackends),
		"total", len(result.Backends))

	return result, nil
}

// DiscoverBackend discovers a specific backend
func (e *Engine) DiscoverBackend(ctx context.Context, backend translation.Backend) (*BackendDiscoveryResult, error) {
	detector, exists := e.detectors[backend]
	if !exists {
		return nil, NewDiscoveryError(ErrorTypeUnknown, backend, "",
			fmt.Sprintf("no detector registered for backend %s", backend))
	}

	return detector.DetectBackend(ctx)
}

// discoverBackendWithRetry discovers a backend with retry logic
func (e *Engine) discoverBackendWithRetry(ctx context.Context, backend translation.Backend) (*BackendDiscoveryResult, error) {
	var lastErr error

	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(e.config.RetryDelay):
				// Continue with retry
			}
		}

		result, err := e.DiscoverBackend(ctx, backend)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't retry for certain error types
		if discoveryErr, ok := err.(*DiscoveryError); ok {
			if discoveryErr.Type == ErrorTypePermissionDenied {
				break // Don't retry permission errors
			}
		}
	}

	return nil, lastErr
}

// IsBackendAvailable checks if a specific backend is available
func (e *Engine) IsBackendAvailable(ctx context.Context, backend translation.Backend) (bool, error) {
	result, err := e.DiscoverBackend(ctx, backend)
	if err != nil {
		return false, err
	}
	return result.Status == BackendStatusAvailable, nil
}

// GetAvailableBackends returns a list of available backends
func (e *Engine) GetAvailableBackends(ctx context.Context) ([]translation.Backend, error) {
	result, err := e.DiscoverBackends(ctx)
	if err != nil {
		return nil, err
	}
	return result.AvailableBackends, nil
}

// RefreshCache refreshes the discovery cache
func (e *Engine) RefreshCache(ctx context.Context) error {
	_, err := e.DiscoverBackends(ctx)
	return err
}

// GetCachedResult returns cached discovery results if available and valid
func (e *Engine) GetCachedResult() (*DiscoveryResult, bool) {
	e.cache.mu.RLock()
	defer e.cache.mu.RUnlock()

	if e.cache.result == nil {
		return nil, false
	}

	// Check if cache is still valid
	if time.Since(e.cache.timestamp) > e.config.CacheTTL {
		return nil, false
	}

	return e.cache.result, true
}

// updateCache updates the discovery cache with thread safety
func (e *Engine) updateCache(result *DiscoveryResult) {
	e.cache.mu.Lock()
	defer e.cache.mu.Unlock()

	e.cache.result = result
	e.cache.timestamp = time.Now()
}

// StartAutoRefresh starts automatic background refresh if enabled
func (e *Engine) StartAutoRefresh(ctx context.Context) error {
	if !e.config.EnableAutoRefresh {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("auto refresh is already running")
	}

	e.running = true
	go e.autoRefreshLoop(ctx)
	return nil
}

// StopAutoRefresh stops automatic background refresh
func (e *Engine) StopAutoRefresh() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	close(e.stopCh)
	e.running = false
	return nil
}

// autoRefreshLoop runs the automatic refresh loop
func (e *Engine) autoRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(e.config.RefreshInterval)
	defer ticker.Stop()

	logger := log.FromContext(ctx).WithName("discovery-auto-refresh")

	for {
		select {
		case <-ctx.Done():
			logger.Info("Auto refresh stopped due to context cancellation")
			return
		case <-e.stopCh:
			logger.Info("Auto refresh stopped")
			return
		case <-ticker.C:
			logger.V(1).Info("Performing automatic discovery refresh")
			if err := e.RefreshCache(ctx); err != nil {
				logger.Error(err, "Auto refresh failed")
			}
		}
	}
}

// CheckCRDExists checks if a CRD exists in the cluster
func (e *Engine) CheckCRDExists(ctx context.Context, crdName string) (bool, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := e.client.Get(ctx, client.ObjectKey{Name: crdName}, crd)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CheckCRDReady checks if a CRD exists and is ready (established)
func (e *Engine) CheckCRDReady(ctx context.Context, crdName string) (bool, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := e.client.Get(ctx, client.ObjectKey{Name: crdName}, crd)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	// Check if CRD is established
	for _, condition := range crd.Status.Conditions {
		if condition.Type == apiextensionsv1.Established {
			return condition.Status == apiextensionsv1.ConditionTrue, nil
		}
	}

	return false, nil
}

// GetCRDInfo retrieves detailed information about a CRD
func (e *Engine) GetCRDInfo(ctx context.Context, crdName string) (*CRDInfo, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := e.client.Get(ctx, client.ObjectKey{Name: crdName}, crd)
	if err != nil {
		if errors.IsNotFound(err) {
			return &CRDInfo{
				Name:      crdName,
				Available: false,
			}, nil
		}
		return nil, err
	}

	// Extract CRD information
	info := &CRDInfo{
		Name:      crd.Name,
		Group:     crd.Spec.Group,
		Kind:      crd.Spec.Names.Kind,
		Available: true,
	}

	// Get version (use the latest storage version)
	for _, version := range crd.Spec.Versions {
		if version.Storage {
			info.Version = version.Name
			break
		}
	}
	if info.Version == "" && len(crd.Spec.Versions) > 0 {
		info.Version = crd.Spec.Versions[0].Name
	}

	// Check if established
	for _, condition := range crd.Status.Conditions {
		if condition.Type == apiextensionsv1.Established {
			info.Controller = condition.Status == apiextensionsv1.ConditionTrue
			break
		}
	}

	return info, nil
}

// ListCRDs lists all CRDs in the cluster matching a specific group
func (e *Engine) ListCRDs(ctx context.Context, group string) ([]CRDInfo, error) {
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	err := e.client.List(ctx, crdList)
	if err != nil {
		return nil, err
	}

	var infos []CRDInfo
	for _, crd := range crdList.Items {
		if group == "" || crd.Spec.Group == group {
			info, err := e.GetCRDInfo(ctx, crd.Name)
			if err != nil {
				continue // Skip CRDs we can't read
			}
			infos = append(infos, *info)
		}
	}

	return infos, nil
}

// ValidateClientPermissions checks if the client has required permissions
func (e *Engine) ValidateClientPermissions(ctx context.Context) error {
	// Try to list CRDs to validate permissions
	_, err := e.ListCRDs(ctx, "")
	if err != nil {
		return NewDiscoveryError(ErrorTypePermissionDenied, "", "",
			"insufficient permissions to list CRDs")
	}
	return nil
}

// Close cleans up the discovery engine
func (e *Engine) Close() error {
	return e.StopAutoRefresh()
}

// NewDiscoverer creates a new Discoverer instance
func NewDiscoverer(client client.Client, config *DiscoveryConfig) Discoverer {
	return NewEngine(client, config)
}
