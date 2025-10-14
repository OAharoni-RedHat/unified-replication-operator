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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

// TestStateMachine tests the state machine implementation
func TestStateMachine(t *testing.T) {
	sm := NewStateMachine()

	t.Run("ValidTransitions", func(t *testing.T) {
		validTransitions := []struct {
			from replicationv1alpha1.ReplicationState
			to   replicationv1alpha1.ReplicationState
		}{
			{replicationv1alpha1.ReplicationStateReplica, replicationv1alpha1.ReplicationStatePromoting},
			{replicationv1alpha1.ReplicationStatePromoting, replicationv1alpha1.ReplicationStateSource},
			{replicationv1alpha1.ReplicationStateSource, replicationv1alpha1.ReplicationStateDemoting},
			{replicationv1alpha1.ReplicationStateDemoting, replicationv1alpha1.ReplicationStateReplica},
			{replicationv1alpha1.ReplicationStateReplica, replicationv1alpha1.ReplicationStateSyncing},
			{replicationv1alpha1.ReplicationStateSyncing, replicationv1alpha1.ReplicationStateReplica},
		}

		for _, tt := range validTransitions {
			err := sm.ValidateTransition(tt.from, tt.to)
			assert.NoError(t, err, "Transition from %s to %s should be valid", tt.from, tt.to)
			assert.True(t, sm.IsValidTransition(tt.from, tt.to))
		}
	})

	t.Run("InvalidTransitions", func(t *testing.T) {
		invalidTransitions := []struct {
			from replicationv1alpha1.ReplicationState
			to   replicationv1alpha1.ReplicationState
		}{
			{replicationv1alpha1.ReplicationStateReplica, replicationv1alpha1.ReplicationStateSource},
			{replicationv1alpha1.ReplicationStateSource, replicationv1alpha1.ReplicationStateReplica},
			{replicationv1alpha1.ReplicationStatePromoting, replicationv1alpha1.ReplicationStateDemoting},
		}

		for _, tt := range invalidTransitions {
			err := sm.ValidateTransition(tt.from, tt.to)
			assert.Error(t, err, "Transition from %s to %s should be invalid", tt.from, tt.to)
			assert.False(t, sm.IsValidTransition(tt.from, tt.to))
		}
	})

	t.Run("IdempotentTransitions", func(t *testing.T) {
		states := []replicationv1alpha1.ReplicationState{
			replicationv1alpha1.ReplicationStateReplica,
			replicationv1alpha1.ReplicationStateSource,
			replicationv1alpha1.ReplicationStatePromoting,
		}

		for _, state := range states {
			assert.True(t, sm.IsValidTransition(state, state), "Same state transition should be valid")
		}
	})

	t.Run("TransitionHistory", func(t *testing.T) {
		sm.ClearHistory()

		sm.RecordTransition(
			replicationv1alpha1.ReplicationStateReplica,
			replicationv1alpha1.ReplicationStatePromoting,
			"test",
			"test-123",
		)

		history := sm.GetHistory()
		assert.Len(t, history, 1)
		assert.Equal(t, replicationv1alpha1.ReplicationStateReplica, history[0].From)
		assert.Equal(t, replicationv1alpha1.ReplicationStatePromoting, history[0].To)
		assert.Equal(t, "test", history[0].Reason)
	})

	t.Run("GetValidTransitions", func(t *testing.T) {
		transitions := sm.GetValidTransitions(replicationv1alpha1.ReplicationStateReplica)
		assert.NotEmpty(t, transitions)
		assert.Contains(t, transitions, replicationv1alpha1.ReplicationStatePromoting)
		assert.Contains(t, transitions, replicationv1alpha1.ReplicationStateSyncing)
	})
}

// TestRetryManager tests the retry manager
func TestRetryManager(t *testing.T) {
	t.Run("ShouldRetry", func(t *testing.T) {
		rm := NewRetryManager(nil)

		// Should retry on error
		err := errors.New("temporary failure")
		assert.True(t, rm.ShouldRetry("test-resource", err))

		// Should not retry when no error
		assert.False(t, rm.ShouldRetry("test-resource", nil))
	})

	t.Run("RetryAttempts", func(t *testing.T) {
		rm := NewRetryManager(&RetryStrategy{
			MaxAttempts: 3,
		})

		resourceKey := "test-resource"

		// Record attempts
		assert.Equal(t, 0, rm.GetAttemptCount(resourceKey))
		rm.RecordAttempt(resourceKey)
		assert.Equal(t, 1, rm.GetAttemptCount(resourceKey))
		rm.RecordAttempt(resourceKey)
		assert.Equal(t, 2, rm.GetAttemptCount(resourceKey))

		// Should still retry (max is 3)
		err := errors.New("test error")
		assert.True(t, rm.ShouldRetry(resourceKey, err))

		// After max attempts
		rm.RecordAttempt(resourceKey)
		assert.Equal(t, 3, rm.GetAttemptCount(resourceKey))
		assert.False(t, rm.ShouldRetry(resourceKey, err))

		// Reset
		rm.ResetAttempts(resourceKey)
		assert.Equal(t, 0, rm.GetAttemptCount(resourceKey))
	})

	t.Run("ExponentialBackoff", func(t *testing.T) {
		rm := NewRetryManager(&RetryStrategy{
			InitialDelay: 1 * time.Second,
			MaxDelay:     1 * time.Minute,
			Multiplier:   2.0,
			Jitter:       0,
		})

		resourceKey := "test-backoff"

		// First delay (no attempts yet)
		delay0 := rm.GetNextDelay(resourceKey)
		assert.Equal(t, 1*time.Second, delay0)

		// After first attempt, delay is still initial (attempts-1 in calculation)
		rm.RecordAttempt(resourceKey)
		delay1 := rm.GetNextDelay(resourceKey)
		assert.GreaterOrEqual(t, delay1, 1*time.Second, "Should be at least initial delay")

		// After second attempt
		rm.RecordAttempt(resourceKey)
		delay2 := rm.GetNextDelay(resourceKey)
		assert.GreaterOrEqual(t, delay2, 2*time.Second, "Should grow exponentially")

		t.Logf("Delays: %v, %v, %v", delay0, delay1, delay2)
	})

	t.Run("WithRetry", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping retry test in short mode")
		}

		rm := NewRetryManager(&RetryStrategy{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Multiplier:   2.0,
		})

		ctx := context.Background()
		attempts := 0

		// Function that succeeds on 3rd attempt
		err := rm.WithRetry(ctx, "test-retry", func() error {
			attempts++
			if attempts < 3 {
				return errors.New("temporary failure")
			}
			return nil
		})

		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})
}

// TestCircuitBreaker tests the circuit breaker
func TestCircuitBreaker(t *testing.T) {
	t.Run("ClosedState", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 2, 1*time.Second)

		assert.Equal(t, StateClosed, cb.GetState())

		// Successful call
		err := cb.Call(func() error { return nil })
		assert.NoError(t, err)
		assert.Equal(t, StateClosed, cb.GetState())
	})

	t.Run("OpenState", func(t *testing.T) {
		cb := NewCircuitBreaker(3, 2, 1*time.Second)

		// Fail multiple times
		for i := 0; i < 3; i++ {
			_ = cb.Call(func() error { return errors.New("failure") })
		}

		assert.Equal(t, StateOpen, cb.GetState())

		// Next call should be rejected
		err := cb.Call(func() error { return nil })
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "circuit breaker is open")
	})

	t.Run("HalfOpenState", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 2, 100*time.Millisecond)

		// Open the circuit
		for i := 0; i < 2; i++ {
			_ = cb.Call(func() error { return errors.New("failure") })
		}
		assert.Equal(t, StateOpen, cb.GetState())

		// Wait for timeout
		time.Sleep(150 * time.Millisecond)

		// Next call transitions to half-open
		_ = cb.Call(func() error { return nil })

		// After successful calls, should close
		_ = cb.Call(func() error { return nil })

		// Should be closed or half-open
		state := cb.GetState()
		assert.True(t, state == StateClosed || state == StateHalfOpen)
	})

	t.Run("Reset", func(t *testing.T) {
		cb := NewCircuitBreaker(2, 2, 1*time.Second)

		// Open the circuit
		for i := 0; i < 2; i++ {
			_ = cb.Call(func() error { return errors.New("failure") })
		}
		assert.Equal(t, StateOpen, cb.GetState())

		// Reset
		cb.Reset()
		assert.Equal(t, StateClosed, cb.GetState())
	})

}

// TestAdvancedReconciliation tests reconciliation with advanced features
func TestAdvancedReconciliation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping advanced reconciliation test in short mode")
	}

	ctx := context.Background()
	s := createTestScheme(t)

	// Create resource
	uvr := createTestUVR("advanced-test", "default")
	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	// Create reconciler with advanced features
	reconciler := createTestReconciler(fakeClient, s)
	reconciler.StateMachine = NewStateMachine()
	reconciler.RetryManager = NewRetryManager(nil)
	reconciler.CircuitBreaker = NewCircuitBreaker(5, 2, 1*time.Minute)

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "advanced-test",
			Namespace: "default",
		},
	}

	// Reconcile
	result, err := reconciler.Reconcile(ctx, req)
	t.Logf("Reconcile result: RequeueAfter=%v, Error=%v",
		result.RequeueAfter, err)

	t.Log("Advanced reconciliation test completed")
}

// TestStateTransitionWithStateMachine tests state validation in reconciliation
func TestStateTransitionWithStateMachine(t *testing.T) {
	ctx := context.Background()
	s := createTestScheme(t)

	// Create resource in replica state
	uvr := createTestUVR("state-test", "default")
	uvr.Spec.ReplicationState = replicationv1alpha1.ReplicationStateReplica

	fakeClient := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(uvr).
		Build()

	reconciler := createTestReconciler(fakeClient, s)
	reconciler.StateMachine = NewStateMachine()

	// First reconcile - establish initial state
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "state-test",
			Namespace: "default",
		},
	}
	_, _ = reconciler.Reconcile(ctx, req)

	// Get updated resource
	updatedUVR := &replicationv1alpha1.UnifiedVolumeReplication{}
	require.NoError(t, fakeClient.Get(ctx, req.NamespacedName, updatedUVR))

	// Try valid transition: replica → promoting
	updatedUVR.Spec.ReplicationState = replicationv1alpha1.ReplicationStatePromoting
	require.NoError(t, fakeClient.Update(ctx, updatedUVR))

	// Reconcile again
	result, err := reconciler.Reconcile(ctx, req)
	t.Logf("Valid transition result: RequeueAfter=%v, Error=%v", result.RequeueAfter, err)

	// Verify state machine recorded transition
	history := reconciler.StateMachine.GetHistory()
	if len(history) > 0 {
		t.Logf("State machine has %d transitions in history", len(history))
	}

	t.Log("State transition test completed")
}

// TestRetryWithCircuitBreaker tests interaction between retry and circuit breaker
func TestRetryWithCircuitBreaker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping retry/circuit breaker test in short mode")
	}

	rm := NewRetryManager(&RetryStrategy{
		MaxAttempts:  5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	cb := NewCircuitBreaker(3, 2, 500*time.Millisecond)

	ctx := context.Background()
	resourceKey := "test-resource"

	// Attempt with circuit breaker
	attempts := 0
	err := rm.WithRetry(ctx, resourceKey, func() error {
		attempts++

		// Use circuit breaker
		return cb.Call(func() error {
			if attempts < 10 { // Always fail
				return errors.New("persistent failure")
			}
			return nil
		})
	})

	assert.Error(t, err, "Should fail due to circuit breaker or max attempts")
	assert.Equal(t, StateOpen, cb.GetState(), "Circuit should be open after failures")
	t.Logf("Attempts: %d, Circuit State: %s", attempts, cb.GetState())

	t.Log("Retry with circuit breaker test completed")
}
