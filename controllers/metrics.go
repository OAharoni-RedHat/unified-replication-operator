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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// Reconciliation metrics
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "unified_replication_reconcile_total",
			Help: "Total number of reconciliations",
		},
		[]string{"namespace", "name", "result"},
	)

	reconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "unified_replication_reconcile_duration_seconds",
			Help:    "Duration of reconciliations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace", "name"},
	)

	reconcileErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "unified_replication_reconcile_errors_total",
			Help: "Total number of reconciliation errors",
		},
		[]string{"namespace", "name", "error_type"},
	)

	// State transition metrics
	stateTransitions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "unified_replication_state_transitions_total",
			Help: "Total number of state transitions",
		},
		[]string{"from_state", "to_state", "backend"},
	)

	stateTransitionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "unified_replication_state_transition_duration_seconds",
			Help:    "Duration of state transitions in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"from_state", "to_state"},
	)

	// Backend operation metrics
	backendOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "unified_replication_backend_operations_total",
			Help: "Total number of backend operations",
		},
		[]string{"backend", "operation", "result"},
	)

	backendOperationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "unified_replication_backend_operation_duration_seconds",
			Help:    "Duration of backend operations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"backend", "operation"},
	)

	// Replication status metrics
	replicationState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "unified_replication_state",
			Help: "Current state of replications (0=replica, 1=source, 2=promoting, 3=demoting, 4=syncing, 5=failed)",
		},
		[]string{"namespace", "name", "state"},
	)

	replicationMode = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "unified_replication_mode",
			Help: "Replication mode (0=async, 1=sync)",
		},
		[]string{"namespace", "name", "mode"},
	)

	replicationHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "unified_replication_health",
			Help: "Replication health status (1=healthy, 0=unhealthy)",
		},
		[]string{"namespace", "name"},
	)

	// Retry and circuit breaker metrics
	retryAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "unified_replication_retry_attempts_total",
			Help: "Total number of retry attempts",
		},
		[]string{"namespace", "name"},
	)

	circuitBreakerState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "unified_replication_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"operation"},
	)

	// Discovery and translation metrics
	discoveryDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "unified_replication_discovery_duration_seconds",
			Help:    "Duration of backend discovery in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)

	discoveryCacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "unified_replication_discovery_cache_hits_total",
			Help: "Total number of discovery cache hits",
		},
	)

	discoveryCacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "unified_replication_discovery_cache_misses_total",
			Help: "Total number of discovery cache misses",
		},
	)

	translationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "unified_replication_translation_duration_seconds",
			Help:    "Duration of state/mode translation in seconds",
			Buckets: []float64{.000001, .000005, .00001, .00005, .0001, .0005, .001},
		},
		[]string{"backend", "type"},
	)

	translationErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "unified_replication_translation_errors_total",
			Help: "Total number of translation errors",
		},
		[]string{"backend", "type", "value"},
	)

	// Resource metrics
	activeReplications = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "unified_replication_active_total",
			Help: "Number of active replication resources",
		},
	)

	replicationsByBackend = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "unified_replication_by_backend",
			Help: "Number of replications per backend",
		},
		[]string{"backend"},
	)
)

func init() {
	// Register metrics with controller-runtime
	metrics.Registry.MustRegister(
		reconcileTotal,
		reconcileDuration,
		reconcileErrors,
		stateTransitions,
		stateTransitionDuration,
		backendOperations,
		backendOperationDuration,
		replicationState,
		replicationMode,
		replicationHealth,
		retryAttempts,
		circuitBreakerState,
		discoveryDuration,
		discoveryCacheHits,
		discoveryCacheMisses,
		translationDuration,
		translationErrors,
		activeReplications,
		replicationsByBackend,
	)
}

// MetricsRecorder provides methods to record metrics
type MetricsRecorder struct{}

// NewMetricsRecorder creates a new metrics recorder
func NewMetricsRecorder() *MetricsRecorder {
	return &MetricsRecorder{}
}

// RecordReconcile records a reconciliation
func (mr *MetricsRecorder) RecordReconcile(namespace, name, result string, duration time.Duration) {
	reconcileTotal.WithLabelValues(namespace, name, result).Inc()
	reconcileDuration.WithLabelValues(namespace, name).Observe(duration.Seconds())
}

// RecordReconcileError records a reconciliation error
func (mr *MetricsRecorder) RecordReconcileError(namespace, name, errorType string) {
	reconcileErrors.WithLabelValues(namespace, name, errorType).Inc()
}

// RecordStateTransition records a state transition
func (mr *MetricsRecorder) RecordStateTransition(fromState, toState, backend string, duration time.Duration) {
	stateTransitions.WithLabelValues(fromState, toState, backend).Inc()
	stateTransitionDuration.WithLabelValues(fromState, toState).Observe(duration.Seconds())
}

// RecordBackendOperation records a backend operation
func (mr *MetricsRecorder) RecordBackendOperation(backend, operation, result string, duration time.Duration) {
	backendOperations.WithLabelValues(backend, operation, result).Inc()
	backendOperationDuration.WithLabelValues(backend, operation).Observe(duration.Seconds())
}

// SetReplicationState sets the current state gauge
func (mr *MetricsRecorder) SetReplicationState(namespace, name, state string) {
	// Clear previous state
	replicationState.DeleteLabelValues(namespace, name, state)
	// Set new state
	replicationState.WithLabelValues(namespace, name, state).Set(1)
}

// SetReplicationMode sets the mode gauge
func (mr *MetricsRecorder) SetReplicationMode(namespace, name, mode string) {
	val := 0.0
	if mode == "synchronous" {
		val = 1.0
	}
	replicationMode.WithLabelValues(namespace, name, mode).Set(val)
}

// SetReplicationHealth sets the health gauge
func (mr *MetricsRecorder) SetReplicationHealth(namespace, name string, healthy bool) {
	val := 0.0
	if healthy {
		val = 1.0
	}
	replicationHealth.WithLabelValues(namespace, name).Set(val)
}

// RecordRetryAttempt records a retry attempt
func (mr *MetricsRecorder) RecordRetryAttempt(namespace, name string) {
	retryAttempts.WithLabelValues(namespace, name).Inc()
}

// SetCircuitBreakerState sets the circuit breaker state
func (mr *MetricsRecorder) SetCircuitBreakerState(operation string, state CircuitBreakerState) {
	val := 0.0
	switch state {
	case StateOpen:
		val = 1.0
	case StateHalfOpen:
		val = 2.0
	}
	circuitBreakerState.WithLabelValues(operation).Set(val)
}

// RecordDiscoveryDuration records discovery duration
func (mr *MetricsRecorder) RecordDiscoveryDuration(duration time.Duration) {
	discoveryDuration.Observe(duration.Seconds())
}

// RecordDiscoveryCacheHit records a cache hit
func (mr *MetricsRecorder) RecordDiscoveryCacheHit() {
	discoveryCacheHits.Inc()
}

// RecordDiscoveryCacheMiss records a cache miss
func (mr *MetricsRecorder) RecordDiscoveryCacheMiss() {
	discoveryCacheMisses.Inc()
}

// RecordTranslation records a translation operation
func (mr *MetricsRecorder) RecordTranslation(backend, translationType string, duration time.Duration) {
	translationDuration.WithLabelValues(backend, translationType).Observe(duration.Seconds())
}

// RecordTranslationError records a translation error
func (mr *MetricsRecorder) RecordTranslationError(backend, translationType, value string) {
	translationErrors.WithLabelValues(backend, translationType, value).Inc()
}

// SetActiveReplications sets the number of active replications
func (mr *MetricsRecorder) SetActiveReplications(count float64) {
	activeReplications.Set(count)
}

// SetReplicationsByBackend sets replications count per backend
func (mr *MetricsRecorder) SetReplicationsByBackend(backend string, count float64) {
	replicationsByBackend.WithLabelValues(backend).Set(count)
}

