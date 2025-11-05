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

	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/unified-replication/operator/pkg/translation"
)

// BaseAdapter provides common functionality for all adapters
type BaseAdapter struct {
	backend    translation.Backend
	client     client.Client
	config     *AdapterConfig
	translator *translation.Engine

	// Runtime state
	initialized bool
	mu          sync.RWMutex

	// Adapter info
	info         AdapterInfo
	capabilities AdapterCapabilities
}

// NewBaseAdapter creates a new base adapter
func NewBaseAdapter(backend translation.Backend, client client.Client, translator *translation.Engine, config *AdapterConfig) *BaseAdapter {
	if config == nil {
		config = DefaultAdapterConfig(backend)
	}

	return &BaseAdapter{
		backend:    backend,
		client:     client,
		config:     config,
		translator: translator,
		info: AdapterInfo{
			Backend: backend,
			Version: "1.0.0",
		},
		capabilities: AdapterCapabilities{
			Backend:          backend,
			MaxConcurrentOps: 10,
			SupportedStates:  []string{"source", "replica", "promoting", "demoting", "syncing", "failed"},
			SupportedModes:   []string{"synchronous", "asynchronous"},
			Features:         []AdapterFeature{FeatureAsyncReplication, FeatureSyncReplication},
		},
	}
}

// GetBackendType returns the backend type
func (ba *BaseAdapter) GetBackendType() translation.Backend {
	return ba.backend
}

// GetSupportedFeatures returns the features supported by this adapter
func (ba *BaseAdapter) GetSupportedFeatures() []AdapterFeature {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.capabilities.Features
}

// GetVersion returns the adapter version
func (ba *BaseAdapter) GetVersion() string {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.info.Version
}

// IsHealthy returns the current health status
func (ba *BaseAdapter) IsHealthy() bool {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.initialized
}

// Initialize initializes the adapter
func (ba *BaseAdapter) Initialize(ctx context.Context) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if ba.initialized {
		return nil
	}

	logger := log.FromContext(ctx).WithName("base-adapter").WithValues("backend", ba.backend)
	logger.Info("Initializing adapter")

	// Validate configuration
	if err := ba.validateConfig(); err != nil {
		return NewAdapterErrorWithCause(ErrorTypeConfiguration, ba.backend, "initialize", "", "configuration validation failed", err)
	}

	ba.initialized = true

	logger.Info("Adapter initialized successfully")
	return nil
}

// Cleanup cleans up the adapter resources
func (ba *BaseAdapter) Cleanup(ctx context.Context) error {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if !ba.initialized {
		return nil
	}

	logger := log.FromContext(ctx).WithName("base-adapter").WithValues("backend", ba.backend)
	logger.Info("Cleaning up adapter")

	ba.initialized = false

	logger.Info("Adapter cleanup completed")
	return nil
}

// TranslateState translates unified state to backend-specific state
func (ba *BaseAdapter) TranslateState(unifiedState string) (string, error) {
	backendState, err := ba.translator.TranslateStateToBackend(ba.backend, unifiedState)
	if err != nil {
		return "", NewAdapterErrorWithCause(ErrorTypeOperation, ba.backend, "translate_state", "",
			fmt.Sprintf("failed to translate state '%s'", unifiedState), err)
	}
	return backendState, nil
}

// TranslateMode translates unified mode to backend-specific mode
func (ba *BaseAdapter) TranslateMode(unifiedMode string) (string, error) {
	backendMode, err := ba.translator.TranslateModeToBackend(ba.backend, unifiedMode)
	if err != nil {
		return "", NewAdapterErrorWithCause(ErrorTypeOperation, ba.backend, "translate_mode", "",
			fmt.Sprintf("failed to translate mode '%s'", unifiedMode), err)
	}
	return backendMode, nil
}

// TranslateBackendState translates backend-specific state to unified state
func (ba *BaseAdapter) TranslateBackendState(backendState string) (string, error) {
	unifiedState, err := ba.translator.TranslateStateFromBackend(ba.backend, backendState)
	if err != nil {
		return "", NewAdapterErrorWithCause(ErrorTypeOperation, ba.backend, "translate_backend_state", "",
			fmt.Sprintf("failed to translate backend state '%s'", backendState), err)
	}
	return unifiedState, nil
}

// TranslateBackendMode translates backend-specific mode to unified mode
func (ba *BaseAdapter) TranslateBackendMode(backendMode string) (string, error) {
	unifiedMode, err := ba.translator.TranslateModeFromBackend(ba.backend, backendMode)
	if err != nil {
		return "", NewAdapterErrorWithCause(ErrorTypeOperation, ba.backend, "translate_backend_mode", "",
			fmt.Sprintf("failed to translate backend mode '%s'", backendMode), err)
	}
	return unifiedMode, nil
}

// GetCapabilities returns the adapter capabilities
func (ba *BaseAdapter) GetCapabilities() AdapterCapabilities {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.capabilities
}

// GetInfo returns the adapter information
func (ba *BaseAdapter) GetInfo() AdapterInfo {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	return ba.info
}

// SetCapabilities updates the adapter capabilities
func (ba *BaseAdapter) SetCapabilities(capabilities AdapterCapabilities) {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	ba.capabilities = capabilities
}

// SetInfo updates the adapter information
func (ba *BaseAdapter) SetInfo(info AdapterInfo) {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	ba.info = info
}

// WithRetry executes a function with retry logic
func (ba *BaseAdapter) WithRetry(ctx context.Context, operation string, fn func() error) error {
	err := wait.ExponentialBackoff(wait.Backoff{
		Duration: ba.config.RetryDelay,
		Factor:   2.0,
		Jitter:   0.1,
		Steps:    ba.config.RetryAttempts,
		Cap:      30 * time.Second,
	}, func() (bool, error) {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
		}

		err := fn()
		if err == nil {
			return true, nil // Success, stop retrying
		}

		// Check if error is retryable
		if adapterErr, ok := GetAdapterError(err); ok && !adapterErr.IsRetryable() {
			return false, err // Non-retryable error, stop with error
		}

		return false, nil // Retryable error, continue trying
	})

	if err != nil {
		return err // Return error from ExponentialBackoff (either context error or non-retryable error)
	}

	// If ExponentialBackoff succeeded (err == nil), the operation succeeded
	return nil
}

// ExecuteWithTimeout executes a function with timeout
func (ba *BaseAdapter) ExecuteWithTimeout(ctx context.Context, operation string, fn func(context.Context) error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, ba.config.Timeout)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- fn(timeoutCtx)
	}()

	select {
	case err := <-errCh:
		return err
	case <-timeoutCtx.Done():
		return NewAdapterError(ErrorTypeTimeout, ba.backend, operation, "",
			fmt.Sprintf("operation timed out after %s", ba.config.Timeout))
	}
}

// CreateCondition creates a status condition
func (ba *BaseAdapter) CreateCondition(condType, status, reason, message string) StatusCondition {
	return StatusCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: time.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// validateConfig validates the adapter configuration
func (ba *BaseAdapter) validateConfig() error {
	if ba.config == nil {
		return fmt.Errorf("adapter configuration cannot be nil")
	}

	if ba.config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if ba.config.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}

	if ba.config.RetryDelay <= 0 {
		return fmt.Errorf("retry delay must be positive")
	}

	return nil
}

// NotImplementedError returns an error for operations not implemented by the specific adapter
func (ba *BaseAdapter) NotImplementedError(operation string) error {
	return NewAdapterError(ErrorTypeOperation, ba.backend, operation, "",
		fmt.Sprintf("operation %s not implemented by %s adapter", operation, ba.backend))
}

// ResourceNotFoundError returns an error when a resource is not found
func (ba *BaseAdapter) ResourceNotFoundError(resource string) error {
	return NewAdapterError(ErrorTypeResource, ba.backend, "get", resource, "resource not found")
}

// ConfigurationError returns a configuration error
func (ba *BaseAdapter) ConfigurationError(message string) error {
	return NewAdapterError(ErrorTypeConfiguration, ba.backend, "configure", "", message)
}

// ConnectionError returns a connection error
func (ba *BaseAdapter) ConnectionError(message string, cause error) error {
	return NewAdapterErrorWithCause(ErrorTypeConnection, ba.backend, "connect", "", message, cause)
}

// TimeoutError returns a timeout error
func (ba *BaseAdapter) TimeoutError(operation string, timeout time.Duration) error {
	return NewAdapterError(ErrorTypeTimeout, ba.backend, operation, "",
		fmt.Sprintf("operation timed out after %s", timeout))
}

// updateMetrics updates internal metrics (stub implementation)
func (ba *BaseAdapter) updateMetrics(operation string, success bool, startTime time.Time) {
	// Stub implementation - metrics tracking can be added later if needed
	// This prevents errors but doesn't actually track metrics
}

// GetMetrics returns adapter metrics (stub implementation)
func (ba *BaseAdapter) GetMetrics() AdapterMetrics {
	return AdapterMetrics{
		TotalOperations: 0,
		SuccessfulOps:   0,
		FailedOps:       0,
	}
}

// GetStats returns adapter statistics (stub implementation)
func (ba *BaseAdapter) GetStats() AdapterStats {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	return AdapterStats{
		Backend:            ba.backend,
		Uptime:             0,
		ActiveReplications: 0,
		TotalReplications:  0,
		Metrics:            ba.GetMetrics(),
		LastHealthCheck:    time.Now(),
		SupportedFeatures:  ba.capabilities.Features,
		Version:            ba.info.Version,
	}
}
