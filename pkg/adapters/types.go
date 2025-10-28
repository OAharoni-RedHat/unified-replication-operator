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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	replicationv1alpha2 "github.com/unified-replication/operator/api/v1alpha2"
	"github.com/unified-replication/operator/pkg/translation"
)

// VolumeReplicationAdapter handles reconciliation for v1alpha2 VolumeReplication resources
// This is the NEW interface for kubernetes-csi-addons compatible single volume replication
type VolumeReplicationAdapter interface {
	// ReconcileVolumeReplication reconciles a VolumeReplication resource
	ReconcileVolumeReplication(
		ctx context.Context,
		vr *replicationv1alpha2.VolumeReplication,
		vrc *replicationv1alpha2.VolumeReplicationClass,
	) (ctrl.Result, error)

	// DeleteVolumeReplication cleans up backend resources for a VolumeReplication
	DeleteVolumeReplication(
		ctx context.Context,
		vr *replicationv1alpha2.VolumeReplication,
	) error

	// GetStatus fetches current replication status from backend
	GetStatus(
		ctx context.Context,
		vr *replicationv1alpha2.VolumeReplication,
	) (*V1Alpha2ReplicationStatus, error)
}

// VolumeGroupReplicationAdapter handles reconciliation for v1alpha2 VolumeGroupReplication resources
// This enables multi-volume crash-consistent group replication
type VolumeGroupReplicationAdapter interface {
	// ReconcileVolumeGroupReplication reconciles a group of volumes
	ReconcileVolumeGroupReplication(
		ctx context.Context,
		vgr *replicationv1alpha2.VolumeGroupReplication,
		vgrc *replicationv1alpha2.VolumeGroupReplicationClass,
		pvcs []corev1.PersistentVolumeClaim,
	) (ctrl.Result, error)

	// DeleteVolumeGroupReplication cleans up backend resources for a volume group
	DeleteVolumeGroupReplication(
		ctx context.Context,
		vgr *replicationv1alpha2.VolumeGroupReplication,
	) error

	// GetGroupStatus fetches current group replication status from backend
	GetGroupStatus(
		ctx context.Context,
		vgr *replicationv1alpha2.VolumeGroupReplication,
	) (*V1Alpha2ReplicationStatus, error)
}

// V1Alpha2ReplicationStatus represents status for v1alpha2 resources
type V1Alpha2ReplicationStatus struct {
	State            string
	Message          string
	LastSyncTime     *metav1.Time
	LastSyncDuration *metav1.Duration
	Conditions       []metav1.Condition
}

// UnifiedVolumeReplicationAdapter defines the interface for v1alpha1 backend adapters
// DEPRECATED: This interface is for v1alpha1 API. Use VolumeReplicationAdapter for v1alpha2.
// This will be removed in v3.0.0 when v1alpha1 support is dropped.
type UnifiedVolumeReplicationAdapter interface {
	// Core operations - use EnsureReplication for reconciliation
	EnsureReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error
	DeleteReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error
	GetReplicationStatus(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) (*ReplicationStatus, error)

	// Configuration and validation
	ValidateConfiguration(uvr *replicationv1alpha1.UnifiedVolumeReplication) error
	SupportsConfiguration(uvr *replicationv1alpha1.UnifiedVolumeReplication) (bool, error)

	// State management
	PromoteReplica(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error
	DemoteSource(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error
	ResyncReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error

	// Advanced operations
	PauseReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error
	ResumeReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error
	FailoverReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error
	FailbackReplication(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error

	// Metadata and information
	GetBackendType() translation.Backend
	GetSupportedFeatures() []AdapterFeature
	GetVersion() string
	IsHealthy() bool

	// Lifecycle management
	Initialize(ctx context.Context) error
	Cleanup(ctx context.Context) error
	Reconcile(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error
}

// ReplicationAdapter is an alias for backward compatibility
// DEPRECATED: Use UnifiedVolumeReplicationAdapter explicitly
type ReplicationAdapter = UnifiedVolumeReplicationAdapter

// ReplicationStatus represents the status of a replication relationship
type ReplicationStatus struct {
	State              string                 `json:"state"`
	Mode               string                 `json:"mode"`
	Health             ReplicationHealth      `json:"health"`
	LastSyncTime       *time.Time             `json:"last_sync_time,omitempty"`
	NextSyncTime       *time.Time             `json:"next_sync_time,omitempty"`
	SyncProgress       *SyncProgress          `json:"sync_progress,omitempty"`
	BackendSpecific    map[string]interface{} `json:"backend_specific,omitempty"`
	Message            string                 `json:"message,omitempty"`
	ObservedGeneration int64                  `json:"observed_generation"`
	Conditions         []StatusCondition      `json:"conditions,omitempty"`
}

// ReplicationHealth represents the health of a replication relationship
type ReplicationHealth string

const (
	ReplicationHealthHealthy   ReplicationHealth = "Healthy"
	ReplicationHealthDegraded  ReplicationHealth = "Degraded"
	ReplicationHealthUnhealthy ReplicationHealth = "Unhealthy"
	ReplicationHealthUnknown   ReplicationHealth = "Unknown"
)

// SyncProgress represents the progress of synchronization
type SyncProgress struct {
	TotalBytes      int64   `json:"total_bytes"`
	SyncedBytes     int64   `json:"synced_bytes"`
	PercentComplete float64 `json:"percent_complete"`
	EstimatedTime   string  `json:"estimated_time,omitempty"`
}

// StatusCondition represents a condition of the replication status
type StatusCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastTransitionTime time.Time `json:"last_transition_time"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// AdapterFeature represents a feature supported by an adapter
type AdapterFeature string

const (
	// Core replication features
	FeatureAsyncReplication    AdapterFeature = "AsyncReplication"
	FeatureSyncReplication     AdapterFeature = "SyncReplication"
	FeatureEventualReplication AdapterFeature = "EventualReplication"
	FeatureMetroReplication    AdapterFeature = "MetroReplication"

	// State management features
	FeaturePromotion   AdapterFeature = "Promotion"
	FeatureDemotion    AdapterFeature = "Demotion"
	FeatureResync      AdapterFeature = "Resync"
	FeatureFailover    AdapterFeature = "Failover"
	FeatureFailback    AdapterFeature = "Failback"
	FeaturePauseResume AdapterFeature = "PauseResume"

	// Advanced features
	FeatureSnapshotBased     AdapterFeature = "SnapshotBased"
	FeatureJournalBased      AdapterFeature = "JournalBased"
	FeatureConsistencyGroups AdapterFeature = "ConsistencyGroups"
	FeatureVolumeGroups      AdapterFeature = "VolumeGroups"
	FeatureAutoResync        AdapterFeature = "AutoResync"
	FeatureScheduledSync     AdapterFeature = "ScheduledSync"

	// Performance features
	FeatureHighThroughput AdapterFeature = "HighThroughput"
	FeatureLowLatency     AdapterFeature = "LowLatency"
	FeatureMultiRegion    AdapterFeature = "MultiRegion"
	FeatureMultiCloud     AdapterFeature = "MultiCloud"

	// Management features
	FeatureMetrics          AdapterFeature = "Metrics"
	FeatureProgressTracking AdapterFeature = "ProgressTracking"
	FeatureRealTimeStatus   AdapterFeature = "RealTimeStatus"
)

// AdapterConfig contains configuration for adapters
type AdapterConfig struct {
	Backend             translation.Backend    `json:"backend"`
	Timeout             time.Duration          `json:"timeout"`
	RetryAttempts       int                    `json:"retry_attempts"`
	RetryDelay          time.Duration          `json:"retry_delay"`
	HealthCheckEnabled  bool                   `json:"health_check_enabled"`
	HealthCheckInterval time.Duration          `json:"health_check_interval"`
	MetricsEnabled      bool                   `json:"metrics_enabled"`
	CustomSettings      map[string]interface{} `json:"custom_settings,omitempty"`
}

// DefaultAdapterConfig returns the default configuration for adapters
func DefaultAdapterConfig(backend translation.Backend) *AdapterConfig {
	return &AdapterConfig{
		Backend:             backend,
		Timeout:             30 * time.Second,
		RetryAttempts:       3,
		RetryDelay:          5 * time.Second,
		HealthCheckEnabled:  true,
		HealthCheckInterval: 1 * time.Minute,
		MetricsEnabled:      false,
		CustomSettings:      make(map[string]interface{}),
	}
}

// AdapterError represents errors that can occur in adapters
type AdapterError struct {
	Type       AdapterErrorType
	Backend    translation.Backend
	Operation  string
	Resource   string
	Message    string
	Cause      error
	Retryable  bool
	Suggestion string
}

// AdapterErrorType defines categories of adapter errors
type AdapterErrorType string

const (
	ErrorTypeConfiguration AdapterErrorType = "Configuration"
	ErrorTypeConnection    AdapterErrorType = "Connection"
	ErrorTypeValidation    AdapterErrorType = "Validation"
	ErrorTypeOperation     AdapterErrorType = "Operation"
	ErrorTypeTimeout       AdapterErrorType = "Timeout"
	ErrorTypePermission    AdapterErrorType = "Permission"
	ErrorTypeResource      AdapterErrorType = "Resource"
	ErrorTypeUnknown       AdapterErrorType = "Unknown"
)

// Error implements the error interface
func (e *AdapterError) Error() string {
	msg := "adapter error"
	if e.Backend != "" {
		msg += " (" + string(e.Backend) + ")"
	}
	if e.Operation != "" {
		msg += " [" + e.Operation + "]"
	}
	if e.Resource != "" {
		msg += " {" + e.Resource + "}"
	}
	msg += ": " + e.Message
	if e.Cause != nil {
		msg += " (caused by: " + e.Cause.Error() + ")"
	}
	if e.Suggestion != "" {
		msg += " - " + e.Suggestion
	}
	return msg
}

// Unwrap returns the underlying cause for error wrapping support
func (e *AdapterError) Unwrap() error {
	return e.Cause
}

// IsRetryable returns whether the error is retryable
func (e *AdapterError) IsRetryable() bool {
	return e.Retryable
}

// NewAdapterError creates a new adapter error
func NewAdapterError(errType AdapterErrorType, backend translation.Backend, operation, resource, message string) *AdapterError {
	return &AdapterError{
		Type:      errType,
		Backend:   backend,
		Operation: operation,
		Resource:  resource,
		Message:   message,
		Retryable: isRetryableError(errType),
	}
}

// NewAdapterErrorWithCause creates a new adapter error with an underlying cause
func NewAdapterErrorWithCause(errType AdapterErrorType, backend translation.Backend, operation, resource, message string, cause error) *AdapterError {
	return &AdapterError{
		Type:      errType,
		Backend:   backend,
		Operation: operation,
		Resource:  resource,
		Message:   message,
		Cause:     cause,
		Retryable: isRetryableError(errType),
	}
}

// isRetryableError determines if an error type is generally retryable
func isRetryableError(errType AdapterErrorType) bool {
	switch errType {
	case ErrorTypeConnection, ErrorTypeTimeout, ErrorTypeResource:
		return true
	case ErrorTypeConfiguration, ErrorTypeValidation, ErrorTypePermission:
		return false
	default:
		return false
	}
}

// IsAdapterError checks if an error is an AdapterError
func IsAdapterError(err error) bool {
	_, ok := err.(*AdapterError)
	return ok
}

// GetAdapterError extracts an AdapterError from an error
func GetAdapterError(err error) (*AdapterError, bool) {
	ae, ok := err.(*AdapterError)
	return ae, ok
}

// AdapterMetrics contains metrics for adapter operations
type AdapterMetrics struct {
	TotalOperations     int64         `json:"total_operations"`
	SuccessfulOps       int64         `json:"successful_operations"`
	FailedOps           int64         `json:"failed_operations"`
	AverageLatency      time.Duration `json:"average_latency"`
	LastOperationTime   time.Time     `json:"last_operation_time"`
	HealthCheckCount    int64         `json:"health_check_count"`
	HealthCheckFailures int64         `json:"health_check_failures"`
	ConnectionErrors    int64         `json:"connection_errors"`
	TimeoutErrors       int64         `json:"timeout_errors"`
}

// CalculateSuccessRate returns the success rate as a percentage
func (m *AdapterMetrics) CalculateSuccessRate() float64 {
	if m.TotalOperations == 0 {
		return 100.0 // New adapters start with 100% success rate
	}
	return float64(m.SuccessfulOps) / float64(m.TotalOperations) * 100.0
}

// IsHealthy returns true if the adapter is considered healthy based on metrics
func (m *AdapterMetrics) IsHealthy() bool {
	successRate := m.CalculateSuccessRate()
	recentFailures := m.HealthCheckFailures > 0 && m.HealthCheckCount > 0
	tooManyErrors := float64(m.ConnectionErrors+m.TimeoutErrors)/float64(m.TotalOperations) > 0.1

	return successRate > 80.0 && !recentFailures && !tooManyErrors
}

// ReplicationEvent represents events that can occur during replication operations
type ReplicationEvent struct {
	Type      ReplicationEventType `json:"type"`
	Timestamp time.Time            `json:"timestamp"`
	Resource  string               `json:"resource"`
	Message   string               `json:"message"`
	Metadata  map[string]string    `json:"metadata,omitempty"`
}

// ReplicationEventType defines types of replication events
type ReplicationEventType string

const (
	EventTypeCreated    ReplicationEventType = "Created"
	EventTypeUpdated    ReplicationEventType = "Updated"
	EventTypeDeleted    ReplicationEventType = "Deleted"
	EventTypePromoted   ReplicationEventType = "Promoted"
	EventTypeDemoted    ReplicationEventType = "Demoted"
	EventTypeResynced   ReplicationEventType = "Resynced"
	EventTypePaused     ReplicationEventType = "Paused"
	EventTypeResumed    ReplicationEventType = "Resumed"
	EventTypeFailedOver ReplicationEventType = "FailedOver"
	EventTypeFailedBack ReplicationEventType = "FailedBack"
	EventTypeHealthy    ReplicationEventType = "Healthy"
	EventTypeDegraded   ReplicationEventType = "Degraded"
	EventTypeUnhealthy  ReplicationEventType = "Unhealthy"
	EventTypeError      ReplicationEventType = "Error"
)

// AdapterStats provides statistics about adapter operations
type AdapterStats struct {
	Backend            translation.Backend `json:"backend"`
	Uptime             time.Duration       `json:"uptime"`
	ActiveReplications int                 `json:"active_replications"`
	TotalReplications  int                 `json:"total_replications"`
	Metrics            AdapterMetrics      `json:"metrics"`
	LastHealthCheck    time.Time           `json:"last_health_check"`
	SupportedFeatures  []AdapterFeature    `json:"supported_features"`
	Version            string              `json:"version"`
}

// AdapterCapabilities represents the capabilities of an adapter
type AdapterCapabilities struct {
	Backend             translation.Backend    `json:"backend"`
	SupportedStates     []string               `json:"supported_states"`
	SupportedModes      []string               `json:"supported_modes"`
	Features            []AdapterFeature       `json:"features"`
	MaxConcurrentOps    int                    `json:"max_concurrent_operations"`
	MaxVolumeSize       string                 `json:"max_volume_size,omitempty"`
	SupportedRegions    []string               `json:"supported_regions,omitempty"`
	RequiredPermissions []string               `json:"required_permissions,omitempty"`
	ConfigurationSchema map[string]interface{} `json:"configuration_schema,omitempty"`
}

// SupportsFeature checks if the adapter supports a specific feature
func (c *AdapterCapabilities) SupportsFeature(feature AdapterFeature) bool {
	for _, f := range c.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// SupportsState checks if the adapter supports a specific replication state
func (c *AdapterCapabilities) SupportsState(state string) bool {
	for _, s := range c.SupportedStates {
		if s == state {
			return true
		}
	}
	return false
}

// SupportsMode checks if the adapter supports a specific replication mode
func (c *AdapterCapabilities) SupportsMode(mode string) bool {
	for _, m := range c.SupportedModes {
		if m == mode {
			return true
		}
	}
	return false
}

// AdapterInfo provides information about an adapter implementation
type AdapterInfo struct {
	Name        string              `json:"name"`
	Backend     translation.Backend `json:"backend"`
	Version     string              `json:"version"`
	Description string              `json:"description"`
	Author      string              `json:"author,omitempty"`
	Homepage    string              `json:"homepage,omitempty"`
	License     string              `json:"license,omitempty"`
}

// String returns a string representation of the adapter info
func (ai *AdapterInfo) String() string {
	return ai.Name + " v" + ai.Version + " (" + string(ai.Backend) + ")"
}
