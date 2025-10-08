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
	"time"

	"github.com/unified-replication/operator/pkg/translation"
)

// BackendStatus represents the status of a discovered backend
type BackendStatus string

const (
	// BackendStatusAvailable indicates the backend is available and ready
	BackendStatusAvailable BackendStatus = "Available"
	// BackendStatusUnavailable indicates the backend is not available
	BackendStatusUnavailable BackendStatus = "Unavailable"
	// BackendStatusPartial indicates the backend is partially available (some CRDs missing)
	BackendStatusPartial BackendStatus = "Partial"
	// BackendStatusUnknown indicates the backend status could not be determined
	BackendStatusUnknown BackendStatus = "Unknown"
)

// CRDInfo represents information about a Custom Resource Definition
type CRDInfo struct {
	// Name is the name of the CRD
	Name string `json:"name"`
	// Group is the API group of the CRD
	Group string `json:"group"`
	// Version is the version of the CRD
	Version string `json:"version"`
	// Kind is the kind of the CRD
	Kind string `json:"kind"`
	// Available indicates whether the CRD is installed and available
	Available bool `json:"available"`
	// Controller indicates whether the controller for this CRD is running
	Controller bool `json:"controller"`
}

// BackendDiscoveryResult represents the discovery result for a backend
type BackendDiscoveryResult struct {
	// Backend is the backend type
	Backend translation.Backend `json:"backend"`
	// Status is the overall status of the backend
	Status BackendStatus `json:"status"`
	// CRDs contains information about the required CRDs for this backend
	CRDs []CRDInfo `json:"crds"`
	// Version is the detected version of the backend
	Version string `json:"version,omitempty"`
	// Message provides additional information about the discovery result
	Message string `json:"message,omitempty"`
	// LastUpdated is the timestamp when this result was last updated
	LastUpdated time.Time `json:"last_updated"`
}

// DiscoveryResult represents the complete discovery result for all backends
type DiscoveryResult struct {
	// Backends contains the discovery results for each backend
	Backends map[translation.Backend]BackendDiscoveryResult `json:"backends"`
	// AvailableBackends is a list of backends that are available
	AvailableBackends []translation.Backend `json:"available_backends"`
	// Timestamp is when the discovery was performed
	Timestamp time.Time `json:"timestamp"`
	// Error contains any error that occurred during discovery
	Error string `json:"error,omitempty"`
}

// DiscoveryConfig configures the discovery engine behavior
type DiscoveryConfig struct {
	// CacheTTL is how long discovery results are cached
	CacheTTL time.Duration `json:"cache_ttl"`
	// RefreshInterval is how often discovery is refreshed automatically
	RefreshInterval time.Duration `json:"refresh_interval"`
	// TimeoutPerBackend is the timeout for discovering each backend
	TimeoutPerBackend time.Duration `json:"timeout_per_backend"`
	// EnableAutoRefresh enables automatic background refresh
	EnableAutoRefresh bool `json:"enable_auto_refresh"`
	// MaxRetries is the maximum number of retry attempts for failed discoveries
	MaxRetries int `json:"max_retries"`
	// RetryDelay is the delay between retry attempts
	RetryDelay time.Duration `json:"retry_delay"`
}

// DefaultDiscoveryConfig returns the default discovery configuration
func DefaultDiscoveryConfig() *DiscoveryConfig {
	return &DiscoveryConfig{
		CacheTTL:          5 * time.Minute,
		RefreshInterval:   30 * time.Second,
		TimeoutPerBackend: 10 * time.Second,
		EnableAutoRefresh: false,
		MaxRetries:        3,
		RetryDelay:        1 * time.Second,
	}
}

// Discoverer interface defines the contract for backend discovery
type Discoverer interface {
	// DiscoverBackends discovers all available backends
	DiscoverBackends(ctx context.Context) (*DiscoveryResult, error)

	// DiscoverBackend discovers a specific backend
	DiscoverBackend(ctx context.Context, backend translation.Backend) (*BackendDiscoveryResult, error)

	// IsBackendAvailable checks if a specific backend is available
	IsBackendAvailable(ctx context.Context, backend translation.Backend) (bool, error)

	// GetAvailableBackends returns a list of available backends
	GetAvailableBackends(ctx context.Context) ([]translation.Backend, error)

	// RefreshCache refreshes the discovery cache
	RefreshCache(ctx context.Context) error

	// GetCachedResult returns cached discovery results if available
	GetCachedResult() (*DiscoveryResult, bool)

	// StartAutoRefresh starts automatic background refresh (if enabled)
	StartAutoRefresh(ctx context.Context) error

	// StopAutoRefresh stops automatic background refresh
	StopAutoRefresh() error
}

// BackendDetector interface defines the contract for detecting specific backends
type BackendDetector interface {
	// DetectBackend detects if the backend is available
	DetectBackend(ctx context.Context) (*BackendDiscoveryResult, error)

	// GetRequiredCRDs returns the list of CRDs required for this backend
	GetRequiredCRDs() []CRDInfo

	// GetBackendType returns the backend type this detector handles
	GetBackendType() translation.Backend

	// ValidateBackend performs additional validation beyond CRD existence
	ValidateBackend(ctx context.Context) error
}

// DiscoveryError represents various types of discovery failures
type DiscoveryError struct {
	Type     DiscoveryErrorType
	Backend  translation.Backend
	CRD      string
	Message  string
	Original error
}

// DiscoveryErrorType defines the types of discovery errors
type DiscoveryErrorType string

const (
	// ErrorTypeCRDNotFound indicates a required CRD was not found
	ErrorTypeCRDNotFound DiscoveryErrorType = "crd_not_found"
	// ErrorTypeControllerNotFound indicates the controller is not running
	ErrorTypeControllerNotFound DiscoveryErrorType = "controller_not_found"
	// ErrorTypeTimeout indicates discovery timed out
	ErrorTypeTimeout DiscoveryErrorType = "timeout"
	// ErrorTypePermissionDenied indicates insufficient permissions
	ErrorTypePermissionDenied DiscoveryErrorType = "permission_denied"
	// ErrorTypeUnknown indicates an unknown error occurred
	ErrorTypeUnknown DiscoveryErrorType = "unknown"
)

// Error implements the error interface
func (e *DiscoveryError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("discovery error (%s) for backend %s CRD %s: %s (caused by: %v)",
			e.Type, e.Backend, e.CRD, e.Message, e.Original)
	}
	return fmt.Sprintf("discovery error (%s) for backend %s CRD %s: %s",
		e.Type, e.Backend, e.CRD, e.Message)
}

// Unwrap returns the original error for error wrapping support
func (e *DiscoveryError) Unwrap() error {
	return e.Original
}

// IsDiscoveryError checks if an error is a DiscoveryError
func IsDiscoveryError(err error) bool {
	_, ok := err.(*DiscoveryError)
	return ok
}

// GetDiscoveryError extracts a DiscoveryError from an error
func GetDiscoveryError(err error) (*DiscoveryError, bool) {
	de, ok := err.(*DiscoveryError)
	return de, ok
}

// NewDiscoveryError creates a new DiscoveryError
func NewDiscoveryError(errType DiscoveryErrorType, backend translation.Backend, crd, message string) *DiscoveryError {
	return &DiscoveryError{
		Type:    errType,
		Backend: backend,
		CRD:     crd,
		Message: message,
	}
}

// NewDiscoveryErrorWithCause creates a new DiscoveryError with an underlying cause
func NewDiscoveryErrorWithCause(errType DiscoveryErrorType, backend translation.Backend, crd, message string, cause error) *DiscoveryError {
	return &DiscoveryError{
		Type:     errType,
		Backend:  backend,
		CRD:      crd,
		Message:  message,
		Original: cause,
	}
}
