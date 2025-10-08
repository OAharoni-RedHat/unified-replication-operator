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
	"fmt"
	"sync"
	"time"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

// StateTransition represents a valid state transition
type StateTransition struct {
	From        replicationv1alpha1.ReplicationState
	To          replicationv1alpha1.ReplicationState
	Description string
	RequiresOp  string // Operation required for this transition
}

// StateMachine enforces valid state transitions
type StateMachine struct {
	validTransitions map[replicationv1alpha1.ReplicationState][]replicationv1alpha1.ReplicationState
	transitionRules  []StateTransition
	history          []StateHistoryEntry
	historyMutex     sync.RWMutex
	maxHistorySize   int
}

// StateHistoryEntry records a state transition
type StateHistoryEntry struct {
	From      replicationv1alpha1.ReplicationState
	To        replicationv1alpha1.ReplicationState
	Timestamp time.Time
	Reason    string
	RequestID string
}

// NewStateMachine creates a new state machine with defined transitions
func NewStateMachine() *StateMachine {
	sm := &StateMachine{
		validTransitions: make(map[replicationv1alpha1.ReplicationState][]replicationv1alpha1.ReplicationState),
		transitionRules:  make([]StateTransition, 0),
		history:          make([]StateHistoryEntry, 0),
		maxHistorySize:   100,
	}

	sm.defineTransitions()
	return sm
}

// defineTransitions defines all valid state transitions
func (sm *StateMachine) defineTransitions() {
	// Define valid transitions
	transitions := []StateTransition{
		// Initial state transitions
		{
			From:        "",
			To:          replicationv1alpha1.ReplicationStateReplica,
			Description: "Initial creation as replica",
			RequiresOp:  "create",
		},
		{
			From:        "",
			To:          replicationv1alpha1.ReplicationStateSource,
			Description: "Initial creation as source",
			RequiresOp:  "create",
		},

		// Replica transitions
		{
			From:        replicationv1alpha1.ReplicationStateReplica,
			To:          replicationv1alpha1.ReplicationStatePromoting,
			Description: "Start promotion (failover)",
			RequiresOp:  "promote",
		},
		{
			From:        replicationv1alpha1.ReplicationStateReplica,
			To:          replicationv1alpha1.ReplicationStateSyncing,
			Description: "Start resync",
			RequiresOp:  "resync",
		},
		{
			From:        replicationv1alpha1.ReplicationStateReplica,
			To:          replicationv1alpha1.ReplicationStateReplica,
			Description: "Idempotent - remain replica",
			RequiresOp:  "update",
		},

		// Promoting transitions
		{
			From:        replicationv1alpha1.ReplicationStatePromoting,
			To:          replicationv1alpha1.ReplicationStateSource,
			Description: "Complete promotion",
			RequiresOp:  "update",
		},
		{
			From:        replicationv1alpha1.ReplicationStatePromoting,
			To:          replicationv1alpha1.ReplicationStateFailed,
			Description: "Promotion failed",
			RequiresOp:  "update",
		},

		// Source transitions
		{
			From:        replicationv1alpha1.ReplicationStateSource,
			To:          replicationv1alpha1.ReplicationStateDemoting,
			Description: "Start demotion (failback)",
			RequiresOp:  "demote",
		},
		{
			From:        replicationv1alpha1.ReplicationStateSource,
			To:          replicationv1alpha1.ReplicationStateSource,
			Description: "Idempotent - remain source",
			RequiresOp:  "update",
		},

		// Demoting transitions
		{
			From:        replicationv1alpha1.ReplicationStateDemoting,
			To:          replicationv1alpha1.ReplicationStateReplica,
			Description: "Complete demotion",
			RequiresOp:  "update",
		},
		{
			From:        replicationv1alpha1.ReplicationStateDemoting,
			To:          replicationv1alpha1.ReplicationStateFailed,
			Description: "Demotion failed",
			RequiresOp:  "update",
		},

		// Syncing transitions
		{
			From:        replicationv1alpha1.ReplicationStateSyncing,
			To:          replicationv1alpha1.ReplicationStateReplica,
			Description: "Sync complete",
			RequiresOp:  "update",
		},
		{
			From:        replicationv1alpha1.ReplicationStateSyncing,
			To:          replicationv1alpha1.ReplicationStateFailed,
			Description: "Sync failed",
			RequiresOp:  "update",
		},
		{
			From:        replicationv1alpha1.ReplicationStateSyncing,
			To:          replicationv1alpha1.ReplicationStateSyncing,
			Description: "Idempotent - continue syncing",
			RequiresOp:  "update",
		},

		// Failed state transitions
		{
			From:        replicationv1alpha1.ReplicationStateFailed,
			To:          replicationv1alpha1.ReplicationStateSyncing,
			Description: "Retry from failure",
			RequiresOp:  "resync",
		},
		{
			From:        replicationv1alpha1.ReplicationStateFailed,
			To:          replicationv1alpha1.ReplicationStateReplica,
			Description: "Recover to replica",
			RequiresOp:  "update",
		},
	}

	sm.transitionRules = transitions

	// Build transition map for fast lookup
	for _, transition := range transitions {
		sm.validTransitions[transition.From] = append(
			sm.validTransitions[transition.From],
			transition.To,
		)
	}
}

// IsValidTransition checks if a state transition is valid
func (sm *StateMachine) IsValidTransition(from, to replicationv1alpha1.ReplicationState) bool {
	// Same state is always valid (idempotent)
	if from == to {
		return true
	}

	validTargets, exists := sm.validTransitions[from]
	if !exists {
		return false
	}

	for _, validTarget := range validTargets {
		if validTarget == to {
			return true
		}
	}

	return false
}

// ValidateTransition validates a state transition and returns error if invalid
func (sm *StateMachine) ValidateTransition(from, to replicationv1alpha1.ReplicationState) error {
	if sm.IsValidTransition(from, to) {
		return nil
	}

	return fmt.Errorf("invalid state transition from %s to %s", from, to)
}

// RecordTransition records a state transition in history
func (sm *StateMachine) RecordTransition(from, to replicationv1alpha1.ReplicationState, reason, requestID string) {
	sm.historyMutex.Lock()
	defer sm.historyMutex.Unlock()

	entry := StateHistoryEntry{
		From:      from,
		To:        to,
		Timestamp: time.Now(),
		Reason:    reason,
		RequestID: requestID,
	}

	sm.history = append(sm.history, entry)

	// Trim history if too large
	if len(sm.history) > sm.maxHistorySize {
		sm.history = sm.history[len(sm.history)-sm.maxHistorySize:]
	}
}

// GetHistory returns the state transition history
func (sm *StateMachine) GetHistory() []StateHistoryEntry {
	sm.historyMutex.RLock()
	defer sm.historyMutex.RUnlock()

	historyCopy := make([]StateHistoryEntry, len(sm.history))
	copy(historyCopy, sm.history)
	return historyCopy
}

// GetValidTransitions returns all valid transitions from a given state
func (sm *StateMachine) GetValidTransitions(from replicationv1alpha1.ReplicationState) []replicationv1alpha1.ReplicationState {
	validTargets, exists := sm.validTransitions[from]
	if !exists {
		return []replicationv1alpha1.ReplicationState{}
	}

	result := make([]replicationv1alpha1.ReplicationState, len(validTargets))
	copy(result, validTargets)
	return result
}

// GetTransitionRule returns the transition rule for a given transition
func (sm *StateMachine) GetTransitionRule(from, to replicationv1alpha1.ReplicationState) *StateTransition {
	for _, rule := range sm.transitionRules {
		if rule.From == from && rule.To == to {
			return &rule
		}
	}
	return nil
}

// ClearHistory clears the transition history
func (sm *StateMachine) ClearHistory() {
	sm.historyMutex.Lock()
	defer sm.historyMutex.Unlock()
	sm.history = make([]StateHistoryEntry, 0)
}

// GetHistoryForState returns history entries involving a specific state
func (sm *StateMachine) GetHistoryForState(state replicationv1alpha1.ReplicationState) []StateHistoryEntry {
	sm.historyMutex.RLock()
	defer sm.historyMutex.RUnlock()

	result := make([]StateHistoryEntry, 0)
	for _, entry := range sm.history {
		if entry.From == state || entry.To == state {
			result = append(result, entry)
		}
	}
	return result
}
