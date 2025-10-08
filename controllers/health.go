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

package controllers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-logr/logr"
)

// HealthStatus represents the health status of the controller
type HealthStatus struct {
	Healthy        bool                   `json:"healthy"`
	LastCheck      time.Time              `json:"last_check"`
	LastReconcile  time.Time              `json:"last_reconcile"`
	ReconcileCount int64                  `json:"reconcile_count"`
	ErrorRate      float64                `json:"error_rate"`
	Message        string                 `json:"message"`
	Details        map[string]interface{} `json:"details,omitempty"`
}

// HealthChecker provides health check functionality
type HealthChecker struct {
	reconciler       *UnifiedVolumeReplicationReconciler
	healthMutex      sync.RWMutex
	lastHealthCheck  time.Time
	lastHealthStatus *HealthStatus
	healthCheckCount int64
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(reconciler *UnifiedVolumeReplicationReconciler) *HealthChecker {
	return &HealthChecker{
		reconciler: reconciler,
	}
}

// Check performs a health check
func (hc *HealthChecker) Check(ctx context.Context, log logr.Logger) *HealthStatus {
	hc.healthMutex.Lock()
	defer hc.healthMutex.Unlock()

	hc.healthCheckCount++
	hc.lastHealthCheck = time.Now()

	status := &HealthStatus{
		Healthy:        true,
		LastCheck:      time.Now(),
		LastReconcile:  hc.reconciler.LastReconcileTime,
		ReconcileCount: hc.reconciler.ReconcileCount,
		Details:        make(map[string]interface{}),
	}

	// Check 1: Controller has reconciled recently
	if !hc.reconciler.LastReconcileTime.IsZero() {
		timeSinceReconcile := time.Since(hc.reconciler.LastReconcileTime)
		if timeSinceReconcile > 10*time.Minute {
			status.Healthy = false
			status.Message = fmt.Sprintf("No reconciliation in %v", timeSinceReconcile)
		}
		status.Details["time_since_reconcile"] = timeSinceReconcile.String()
	}

	// Check 2: Error rate is acceptable
	if hc.reconciler.ReconcileCount > 0 {
		errorRate := float64(hc.reconciler.ReconcileErrors) / float64(hc.reconciler.ReconcileCount)
		status.ErrorRate = errorRate
		if errorRate > 0.5 {
			status.Healthy = false
			status.Message = fmt.Sprintf("High error rate: %.2f%%", errorRate*100)
		}
		status.Details["error_rate"] = fmt.Sprintf("%.2f%%", errorRate*100)
	}

	// Check 3: Engines are available (if integrated)
	if hc.reconciler.UseIntegratedEngine {
		if hc.reconciler.DiscoveryEngine == nil {
			status.Healthy = false
			status.Message = "Discovery engine not available"
		}
		if hc.reconciler.TranslationEngine == nil {
			status.Healthy = false
			status.Message = "Translation engine not available"
		}
		if hc.reconciler.ControllerEngine == nil {
			status.Healthy = false
			status.Message = "Controller engine not available"
		}
		status.Details["integrated_engine"] = hc.reconciler.UseIntegratedEngine
	}

	// Check 4: Adapter registry available
	if hc.reconciler.AdapterRegistry == nil {
		status.Healthy = false
		status.Message = "Adapter registry not available"
	}

	if status.Healthy && status.Message == "" {
		status.Message = "All systems operational"
	}

	hc.lastHealthStatus = status
	return status
}

// GetLastStatus returns the last health check status
func (hc *HealthChecker) GetLastStatus() *HealthStatus {
	hc.healthMutex.RLock()
	defer hc.healthMutex.RUnlock()
	return hc.lastHealthStatus
}

// HTTPHandler returns an HTTP handler for health checks
func (hc *HealthChecker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		status := hc.Check(ctx, hc.reconciler.Log)

		if status.Healthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"healthy":%v,"message":"%s","last_reconcile":"%s","error_rate":%.2f}`,
			status.Healthy,
			status.Message,
			status.LastReconcile.Format(time.RFC3339),
			status.ErrorRate,
		)
	}
}

// ReadinessChecker provides readiness check functionality
type ReadinessChecker struct {
	reconciler *UnifiedVolumeReplicationReconciler
	ready      bool
	readyMutex sync.RWMutex
}

// NewReadinessChecker creates a new readiness checker
func NewReadinessChecker(reconciler *UnifiedVolumeReplicationReconciler) *ReadinessChecker {
	return &ReadinessChecker{
		reconciler: reconciler,
		ready:      false,
	}
}

// SetReady marks the controller as ready
func (rc *ReadinessChecker) SetReady(ready bool) {
	rc.readyMutex.Lock()
	defer rc.readyMutex.Unlock()
	rc.ready = ready
}

// IsReady checks if the controller is ready
func (rc *ReadinessChecker) IsReady() bool {
	rc.readyMutex.RLock()
	defer rc.readyMutex.RUnlock()
	return rc.ready
}

// Check performs a readiness check
func (rc *ReadinessChecker) Check(ctx context.Context) bool {
	// Controller is ready if:
	// 1. It has been marked ready
	// 2. All engines are initialized (if using integrated mode)

	if !rc.IsReady() {
		return false
	}

	if rc.reconciler.UseIntegratedEngine {
		if rc.reconciler.DiscoveryEngine == nil ||
			rc.reconciler.TranslationEngine == nil ||
			rc.reconciler.ControllerEngine == nil {
			return false
		}
	}

	return true
}

// HTTPHandler returns an HTTP handler for readiness checks
func (rc *ReadinessChecker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if rc.Check(ctx) {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"ready":true}`)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"ready":false}`)
		}
	}
}

// CorrelationIDKey is the context key for correlation IDs
type contextKey string

const CorrelationIDKey contextKey = "correlation-id"

// GenerateCorrelationID generates a unique correlation ID
func GenerateCorrelationID(namespace, name string) string {
	return fmt.Sprintf("%s-%s-%d", namespace, name, time.Now().UnixNano())
}

// WithCorrelationID adds a correlation ID to the context
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// GetCorrelationID retrieves the correlation ID from context
func GetCorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	return ""
}
