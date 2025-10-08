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

package security

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/go-logr/logr"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	AuditEventCreate          AuditEventType = "CREATE"
	AuditEventUpdate          AuditEventType = "UPDATE"
	AuditEventDelete          AuditEventType = "DELETE"
	AuditEventValidate        AuditEventType = "VALIDATE"
	AuditEventStateChange     AuditEventType = "STATE_CHANGE"
	AuditEventAccess          AuditEventType = "ACCESS"
	AuditEventAuthFailure     AuditEventType = "AUTH_FAILURE"
	AuditEventPolicyViolation AuditEventType = "POLICY_VIOLATION"
)

// AuditEvent represents a security audit event
type AuditEvent struct {
	EventType      AuditEventType         `json:"event_type"`
	Timestamp      time.Time              `json:"timestamp"`
	User           string                 `json:"user,omitempty"`
	ServiceAccount string                 `json:"service_account,omitempty"`
	Namespace      string                 `json:"namespace"`
	ResourceName   string                 `json:"resource_name"`
	Operation      string                 `json:"operation"`
	Result         string                 `json:"result"` // success, failure, denied
	Reason         string                 `json:"reason,omitempty"`
	RequestID      string                 `json:"request_id,omitempty"`
	SourceIP       string                 `json:"source_ip,omitempty"`
	UserAgent      string                 `json:"user_agent,omitempty"`
	Details        map[string]interface{} `json:"details,omitempty"`
}

// AuditLogger provides structured audit logging
type AuditLogger struct {
	logger     logr.Logger
	events     []AuditEvent
	eventMutex sync.RWMutex
	maxEvents  int
	enabled    bool
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger logr.Logger, enabled bool) *AuditLogger {
	return &AuditLogger{
		logger:    logger.WithName("audit"),
		events:    make([]AuditEvent, 0),
		maxEvents: 1000, // Keep last 1000 events
		enabled:   enabled,
	}
}

// LogEvent logs an audit event
func (al *AuditLogger) LogEvent(event AuditEvent) {
	if !al.enabled {
		return
	}

	// Set timestamp if not already set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Log to structured logger
	al.logger.Info("Audit Event",
		"event_type", event.EventType,
		"user", event.User,
		"namespace", event.Namespace,
		"resource", event.ResourceName,
		"operation", event.Operation,
		"result", event.Result,
		"request_id", event.RequestID,
	)

	// Store event
	al.eventMutex.Lock()
	defer al.eventMutex.Unlock()

	al.events = append(al.events, event)

	// Trim if exceeds max
	if len(al.events) > al.maxEvents {
		al.events = al.events[len(al.events)-al.maxEvents:]
	}
}

// LogCreate logs a create operation
func (al *AuditLogger) LogCreate(ctx context.Context, namespace, name, user, result string) {
	al.LogEvent(AuditEvent{
		EventType:    AuditEventCreate,
		User:         user,
		Namespace:    namespace,
		ResourceName: name,
		Operation:    "create",
		Result:       result,
		RequestID:    getRequestIDFromContext(ctx),
	})
}

// LogUpdate logs an update operation
func (al *AuditLogger) LogUpdate(ctx context.Context, namespace, name, user, result string, details map[string]interface{}) {
	al.LogEvent(AuditEvent{
		EventType:    AuditEventUpdate,
		User:         user,
		Namespace:    namespace,
		ResourceName: name,
		Operation:    "update",
		Result:       result,
		RequestID:    getRequestIDFromContext(ctx),
		Details:      details,
	})
}

// LogDelete logs a delete operation
func (al *AuditLogger) LogDelete(ctx context.Context, namespace, name, user, result string) {
	al.LogEvent(AuditEvent{
		EventType:    AuditEventDelete,
		User:         user,
		Namespace:    namespace,
		ResourceName: name,
		Operation:    "delete",
		Result:       result,
		RequestID:    getRequestIDFromContext(ctx),
	})
}

// LogValidation logs a validation event
func (al *AuditLogger) LogValidation(ctx context.Context, namespace, name, result, reason string) {
	al.LogEvent(AuditEvent{
		EventType:    AuditEventValidate,
		Namespace:    namespace,
		ResourceName: name,
		Operation:    "validate",
		Result:       result,
		Reason:       reason,
		RequestID:    getRequestIDFromContext(ctx),
	})
}

// LogStateChange logs a state change event
func (al *AuditLogger) LogStateChange(ctx context.Context, namespace, name, fromState, toState string) {
	al.LogEvent(AuditEvent{
		EventType:    AuditEventStateChange,
		Namespace:    namespace,
		ResourceName: name,
		Operation:    "state_change",
		Result:       "success",
		RequestID:    getRequestIDFromContext(ctx),
		Details: map[string]interface{}{
			"from_state": fromState,
			"to_state":   toState,
		},
	})
}

// LogAuthFailure logs an authentication/authorization failure
func (al *AuditLogger) LogAuthFailure(ctx context.Context, user, operation, reason string) {
	al.LogEvent(AuditEvent{
		EventType: AuditEventAuthFailure,
		User:      user,
		Operation: operation,
		Result:    "denied",
		Reason:    reason,
		RequestID: getRequestIDFromContext(ctx),
	})
}

// LogPolicyViolation logs a policy violation
func (al *AuditLogger) LogPolicyViolation(ctx context.Context, namespace, name, policy, reason string) {
	al.LogEvent(AuditEvent{
		EventType:    AuditEventPolicyViolation,
		Namespace:    namespace,
		ResourceName: name,
		Operation:    "policy_check",
		Result:       "violation",
		Reason:       reason,
		RequestID:    getRequestIDFromContext(ctx),
		Details: map[string]interface{}{
			"policy": policy,
		},
	})
}

// GetEvents returns all audit events
func (al *AuditLogger) GetEvents() []AuditEvent {
	al.eventMutex.RLock()
	defer al.eventMutex.RUnlock()

	eventsCopy := make([]AuditEvent, len(al.events))
	copy(eventsCopy, al.events)
	return eventsCopy
}

// GetEventsSince returns events since a timestamp
func (al *AuditLogger) GetEventsSince(since time.Time) []AuditEvent {
	al.eventMutex.RLock()
	defer al.eventMutex.RUnlock()

	result := make([]AuditEvent, 0)
	for _, event := range al.events {
		if event.Timestamp.After(since) {
			result = append(result, event)
		}
	}
	return result
}

// GetEventsByType returns events of a specific type
func (al *AuditLogger) GetEventsByType(eventType AuditEventType) []AuditEvent {
	al.eventMutex.RLock()
	defer al.eventMutex.RUnlock()

	result := make([]AuditEvent, 0)
	for _, event := range al.events {
		if event.EventType == eventType {
			result = append(result, event)
		}
	}
	return result
}

// ExportEvents exports events as JSON
func (al *AuditLogger) ExportEvents() ([]byte, error) {
	al.eventMutex.RLock()
	defer al.eventMutex.RUnlock()

	return json.MarshalIndent(al.events, "", "  ")
}

// ClearEvents clears all stored events
func (al *AuditLogger) ClearEvents() {
	al.eventMutex.Lock()
	defer al.eventMutex.Unlock()
	al.events = make([]AuditEvent, 0)
}

// GetEventCount returns the number of events
func (al *AuditLogger) GetEventCount() int {
	al.eventMutex.RLock()
	defer al.eventMutex.RUnlock()
	return len(al.events)
}

// GetEventCountByType returns the count of events by type
func (al *AuditLogger) GetEventCountByType(eventType AuditEventType) int {
	al.eventMutex.RLock()
	defer al.eventMutex.RUnlock()

	count := 0
	for _, event := range al.events {
		if event.EventType == eventType {
			count++
		}
	}
	return count
}

// Helper function to get request ID from context
func getRequestIDFromContext(ctx context.Context) string {
	// Try to get correlation ID from context
	if reqID, ok := ctx.Value("correlation-id").(string); ok {
		return reqID
	}
	// Try to get from request info
	if reqID, ok := ctx.Value("request-id").(string); ok {
		return reqID
	}
	return ""
}
