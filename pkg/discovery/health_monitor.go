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
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/unified-replication/operator/pkg/translation"
)

// HealthMonitor provides periodic health monitoring for backends
type HealthMonitor struct {
	engine  *EnhancedEngine
	config  *CapabilityConfig
	stopCh  chan struct{}
	running bool
	mu      sync.RWMutex
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(engine *EnhancedEngine, config *CapabilityConfig) *HealthMonitor {
	return &HealthMonitor{
		engine: engine,
		config: config,
		stopCh: make(chan struct{}),
	}
}

// Start begins periodic health monitoring
func (hm *HealthMonitor) Start(ctx context.Context) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if hm.running {
		return nil // Already running
	}

	hm.running = true
	go hm.monitorLoop(ctx)
	return nil
}

// Stop stops the health monitoring
func (hm *HealthMonitor) Stop() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if !hm.running {
		return nil // Already stopped
	}

	close(hm.stopCh)
	hm.running = false
	return nil
}

// IsRunning returns whether the health monitor is running
func (hm *HealthMonitor) IsRunning() bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.running
}

// monitorLoop runs the periodic monitoring
func (hm *HealthMonitor) monitorLoop(ctx context.Context) {
	logger := log.FromContext(ctx).WithName("health-monitor")
	logger.Info("Starting health monitoring",
		"health_check_interval", hm.config.HealthCheckInterval,
		"capability_refresh_interval", hm.config.CapabilityRefreshInterval)

	healthTicker := time.NewTicker(hm.config.HealthCheckInterval)
	defer healthTicker.Stop()

	capabilityTicker := time.NewTicker(hm.config.CapabilityRefreshInterval)
	defer capabilityTicker.Stop()

	var performanceTicker *time.Ticker
	if hm.config.EnablePerformanceMetrics {
		performanceTicker = time.NewTicker(hm.config.PerformanceMetricsInterval)
		defer performanceTicker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			logger.Info("Health monitor stopped due to context cancellation")
			return
		case <-hm.stopCh:
			logger.Info("Health monitor stopped")
			return
		case <-healthTicker.C:
			hm.performHealthChecks(ctx)
		case <-capabilityTicker.C:
			hm.refreshCapabilities(ctx)
		case <-func() <-chan time.Time {
			if performanceTicker != nil {
				return performanceTicker.C
			}
			return make(chan time.Time) // Never fires if performance monitoring is disabled
		}():
			hm.collectPerformanceMetrics(ctx)
		}
	}
}

// performHealthChecks performs health checks on all registered backends
func (hm *HealthMonitor) performHealthChecks(ctx context.Context) {
	logger := log.FromContext(ctx).WithName("health-check")
	logger.V(1).Info("Performing periodic health checks")

	// Get all registered capabilities
	allCapabilities := hm.engine.capabilityRegistry.GetAllCapabilities()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, hm.config.MaxConcurrentChecks)

	for backend := range allCapabilities {
		wg.Add(1)
		go func(backend translation.Backend) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			hm.checkBackendHealth(ctx, backend)
		}(backend)
	}

	wg.Wait()
	logger.V(1).Info("Completed periodic health checks")
}

// checkBackendHealth checks the health of a specific backend
func (hm *HealthMonitor) checkBackendHealth(ctx context.Context, backend translation.Backend) {
	logger := log.FromContext(ctx).WithName("health-check").WithValues("backend", backend)

	detector, exists := hm.engine.capabilityDetectors[backend]
	if !exists {
		logger.V(1).Info("No capability detector found for backend")
		return
	}

	// Create context with timeout
	healthCtx, cancel := context.WithTimeout(ctx, hm.config.TimeoutPerCheck)
	defer cancel()

	health, err := detector.CheckHealth(healthCtx)
	if err != nil {
		logger.Error(err, "Health check failed")
		health = &HealthStatus{
			Status:      HealthLevelUnhealthy,
			Message:     err.Error(),
			LastChecked: time.Now(),
		}
	}

	// Update health status in capabilities
	capabilities, exists := hm.engine.capabilityRegistry.GetCapabilities(backend)
	if exists && capabilities != nil {
		capabilities.Health = *health
		_ = hm.engine.capabilityRegistry.UpdateCapabilities(backend, capabilities)
	}

	// Log health status changes
	if health.Status != HealthLevelHealthy {
		logger.Info("Backend health issue detected",
			"status", health.Status,
			"message", health.Message)
	}
}

// refreshCapabilities refreshes capabilities for all backends
func (hm *HealthMonitor) refreshCapabilities(ctx context.Context) {
	logger := log.FromContext(ctx).WithName("capability-refresh")
	logger.V(1).Info("Refreshing backend capabilities")

	err := hm.engine.capabilityRegistry.RefreshAllCapabilities(ctx)
	if err != nil {
		logger.Error(err, "Failed to refresh capabilities")
	} else {
		logger.V(1).Info("Successfully refreshed capabilities")
	}
}

// collectPerformanceMetrics collects performance metrics for all backends
func (hm *HealthMonitor) collectPerformanceMetrics(ctx context.Context) {
	logger := log.FromContext(ctx).WithName("performance-metrics")
	logger.V(1).Info("Collecting performance metrics")

	allCapabilities := hm.engine.capabilityRegistry.GetAllCapabilities()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, hm.config.MaxConcurrentChecks)

	for backend := range allCapabilities {
		wg.Add(1)
		go func(backend translation.Backend) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			hm.collectBackendPerformanceMetrics(ctx, backend)
		}(backend)
	}

	wg.Wait()
	logger.V(1).Info("Completed performance metrics collection")
}

// collectBackendPerformanceMetrics collects performance metrics for a specific backend
func (hm *HealthMonitor) collectBackendPerformanceMetrics(ctx context.Context, backend translation.Backend) {
	logger := log.FromContext(ctx).WithName("performance-metrics").WithValues("backend", backend)

	detector, exists := hm.engine.capabilityDetectors[backend]
	if !exists {
		logger.V(1).Info("No capability detector found for backend")
		return
	}

	// Create context with timeout
	perfCtx, cancel := context.WithTimeout(ctx, hm.config.TimeoutPerCheck)
	defer cancel()

	_, err := detector.GetPerformanceCharacteristics(perfCtx)
	if err != nil {
		logger.V(1).Info("Failed to collect performance metrics", "error", err)
		return
	}

	logger.V(2).Info("Successfully collected performance metrics")
}

// GetHealthSummary returns a summary of backend health status
func (hm *HealthMonitor) GetHealthSummary() HealthSummary {
	allCapabilities := hm.engine.capabilityRegistry.GetAllCapabilities()

	summary := HealthSummary{
		TotalBackends:     len(allCapabilities),
		HealthyBackends:   0,
		DegradedBackends:  0,
		UnhealthyBackends: 0,
		UnknownBackends:   0,
		LastUpdated:       time.Now(),
		BackendStatus:     make(map[translation.Backend]HealthLevel),
	}

	for backend, capabilities := range allCapabilities {
		if capabilities == nil {
			summary.UnknownBackends++
			summary.BackendStatus[backend] = HealthLevelUnknown
			continue
		}

		status := capabilities.Health.Status
		summary.BackendStatus[backend] = status

		switch status {
		case HealthLevelHealthy:
			summary.HealthyBackends++
		case HealthLevelDegraded:
			summary.DegradedBackends++
		case HealthLevelUnhealthy:
			summary.UnhealthyBackends++
		default:
			summary.UnknownBackends++
		}
	}

	return summary
}

// HealthSummary provides a summary of backend health
type HealthSummary struct {
	TotalBackends     int                                 `json:"total_backends"`
	HealthyBackends   int                                 `json:"healthy_backends"`
	DegradedBackends  int                                 `json:"degraded_backends"`
	UnhealthyBackends int                                 `json:"unhealthy_backends"`
	UnknownBackends   int                                 `json:"unknown_backends"`
	LastUpdated       time.Time                           `json:"last_updated"`
	BackendStatus     map[translation.Backend]HealthLevel `json:"backend_status"`
}

// IsHealthy returns true if all backends are healthy
func (hs *HealthSummary) IsHealthy() bool {
	return hs.UnhealthyBackends == 0 && hs.DegradedBackends == 0 && hs.UnknownBackends == 0
}

// HealthPercentage returns the percentage of healthy backends
func (hs *HealthSummary) HealthPercentage() float64 {
	if hs.TotalBackends == 0 {
		return 100.0
	}
	return float64(hs.HealthyBackends) / float64(hs.TotalBackends) * 100.0
}

// GetUnhealthyBackends returns a list of backends that are not healthy
func (hs *HealthSummary) GetUnhealthyBackends() []translation.Backend {
	var unhealthy []translation.Backend
	for backend, status := range hs.BackendStatus {
		if status != HealthLevelHealthy {
			unhealthy = append(unhealthy, backend)
		}
	}
	return unhealthy
}
