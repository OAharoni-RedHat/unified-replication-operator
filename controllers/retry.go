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
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// RetryStrategy defines retry behavior
type RetryStrategy struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	Multiplier      float64
	Jitter          float64
	RetryableErrors []string
}

// DefaultRetryStrategy returns default retry configuration
func DefaultRetryStrategy() *RetryStrategy {
	return &RetryStrategy{
		MaxAttempts:  5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Minute,
		Multiplier:   2.0,
		Jitter:       0.1,
		RetryableErrors: []string{
			"connection refused",
			"timeout",
			"temporary failure",
			"service unavailable",
		},
	}
}

// RetryManager manages retry logic with exponential backoff
type RetryManager struct {
	strategy      *RetryStrategy
	attempts      map[string]int
	lastAttempt   map[string]time.Time
	attemptsMutex sync.RWMutex
}

// NewRetryManager creates a new retry manager
func NewRetryManager(strategy *RetryStrategy) *RetryManager {
	if strategy == nil {
		strategy = DefaultRetryStrategy()
	}

	return &RetryManager{
		strategy:    strategy,
		attempts:    make(map[string]int),
		lastAttempt: make(map[string]time.Time),
	}
}

// ShouldRetry determines if an operation should be retried
func (rm *RetryManager) ShouldRetry(resourceKey string, err error) bool {
	if err == nil {
		return false
	}

	rm.attemptsMutex.RLock()
	attempts := rm.attempts[resourceKey]
	rm.attemptsMutex.RUnlock()

	if attempts >= rm.strategy.MaxAttempts {
		return false
	}

	// Check if error is retryable
	return rm.isRetryableError(err)
}

// RecordAttempt records a retry attempt
func (rm *RetryManager) RecordAttempt(resourceKey string) {
	rm.attemptsMutex.Lock()
	defer rm.attemptsMutex.Unlock()

	rm.attempts[resourceKey]++
	rm.lastAttempt[resourceKey] = time.Now()
}

// ResetAttempts resets retry attempts for a resource
func (rm *RetryManager) ResetAttempts(resourceKey string) {
	rm.attemptsMutex.Lock()
	defer rm.attemptsMutex.Unlock()

	delete(rm.attempts, resourceKey)
	delete(rm.lastAttempt, resourceKey)
}

// GetAttemptCount returns the number of attempts for a resource
func (rm *RetryManager) GetAttemptCount(resourceKey string) int {
	rm.attemptsMutex.RLock()
	defer rm.attemptsMutex.RUnlock()

	return rm.attempts[resourceKey]
}

// GetNextDelay calculates the next retry delay with exponential backoff
func (rm *RetryManager) GetNextDelay(resourceKey string) time.Duration {
	rm.attemptsMutex.RLock()
	attempts := rm.attempts[resourceKey]
	rm.attemptsMutex.RUnlock()

	if attempts == 0 {
		return rm.strategy.InitialDelay
	}

	// Exponential backoff
	delay := time.Duration(float64(rm.strategy.InitialDelay) * 
		pow(rm.strategy.Multiplier, float64(attempts-1)))

	// Cap at max delay
	if delay > rm.strategy.MaxDelay {
		delay = rm.strategy.MaxDelay
	}

	// Add jitter
	if rm.strategy.Jitter > 0 {
		jitter := time.Duration(float64(delay) * rm.strategy.Jitter)
		delay += time.Duration(randomInt63n(int64(jitter)))
	}

	return delay
}

// WithRetry executes a function with retry logic
func (rm *RetryManager) WithRetry(ctx context.Context, resourceKey string, fn func() error) error {
	backoff := wait.Backoff{
		Duration: rm.strategy.InitialDelay,
		Factor:   rm.strategy.Multiplier,
		Jitter:   rm.strategy.Jitter,
		Steps:    rm.strategy.MaxAttempts,
		Cap:      rm.strategy.MaxDelay,
	}

	err := wait.ExponentialBackoffWithContext(ctx, backoff, func(ctx context.Context) (bool, error) {
		rm.RecordAttempt(resourceKey)

		err := fn()
		if err == nil {
			rm.ResetAttempts(resourceKey)
			return true, nil // Success
		}

		if !rm.isRetryableError(err) {
			return false, err // Non-retryable error
		}

		return false, nil // Retryable error, continue
	})

	return err
}

// isRetryableError checks if an error is retryable
func (rm *RetryManager) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	for _, retryableErr := range rm.strategy.RetryableErrors {
		if contains(errMsg, retryableErr) {
			return true
		}
	}

	// Default: retry on unknown errors
	return true
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState string

const (
	StateClosed    CircuitBreakerState = "closed"     // Normal operation
	StateOpen      CircuitBreakerState = "open"       // Failing, reject requests
	StateHalfOpen  CircuitBreakerState = "half-open"  // Testing if recovered
)

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	state          CircuitBreakerState
	failureCount   int
	successCount   int
	lastFailure    time.Time
	lastSuccess    time.Time
	openedAt       time.Time
	stateMutex     sync.RWMutex

	// Configuration
	failureThreshold int
	successThreshold int
	timeout          time.Duration
	halfOpenTimeout  time.Duration
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		halfOpenTimeout:  timeout / 2,
	}
}

// Call executes a function through the circuit breaker
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.stateMutex.Lock()
	
	// Check circuit state
	switch cb.state {
	case StateOpen:
		// Check if timeout has passed
		if time.Since(cb.openedAt) > cb.timeout {
			cb.state = StateHalfOpen
			cb.successCount = 0
			cb.stateMutex.Unlock()
		} else {
			cb.stateMutex.Unlock()
			return fmt.Errorf("circuit breaker is open")
		}
	case StateHalfOpen:
		// Limited calls allowed in half-open state
		cb.stateMutex.Unlock()
	default: // StateClosed
		cb.stateMutex.Unlock()
	}

	// Execute function
	err := fn()

	cb.stateMutex.Lock()
	defer cb.stateMutex.Unlock()

	if err == nil {
		cb.onSuccess()
		return nil
	}

	cb.onFailure()
	return err
}

// onSuccess handles successful execution
func (cb *CircuitBreaker) onSuccess() {
	cb.lastSuccess = time.Now()
	cb.failureCount = 0
	cb.successCount++

	switch cb.state {
	case StateHalfOpen:
		if cb.successCount >= cb.successThreshold {
			cb.state = StateClosed
			cb.successCount = 0
		}
	}
}

// onFailure handles failed execution
func (cb *CircuitBreaker) onFailure() {
	cb.lastFailure = time.Now()
	cb.failureCount++
	cb.successCount = 0

	if cb.failureCount >= cb.failureThreshold {
		cb.state = StateOpen
		cb.openedAt = time.Now()
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.stateMutex.RLock()
	defer cb.stateMutex.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.stateMutex.Lock()
	defer cb.stateMutex.Unlock()

	cb.state = StateClosed
	cb.failureCount = 0
	cb.successCount = 0
}

// GetMetrics returns circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	cb.stateMutex.RLock()
	defer cb.stateMutex.RUnlock()

	return map[string]interface{}{
		"state":          string(cb.state),
		"failure_count":  cb.failureCount,
		"success_count":  cb.successCount,
		"last_failure":   cb.lastFailure,
		"last_success":   cb.lastSuccess,
		"opened_at":      cb.openedAt,
	}
}

// Helper functions

func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

// Simple random for jitter (not cryptographically secure, but fine for backoff)
var randMutex sync.Mutex
var randSeed = time.Now().UnixNano()

func randomInt63n(n int64) int64 {
	randMutex.Lock()
	defer randMutex.Unlock()
	
	// Simple LCG (Linear Congruential Generator)
	randSeed = (randSeed*1103515245 + 12345) & 0x7fffffff
	return randSeed % n
}

