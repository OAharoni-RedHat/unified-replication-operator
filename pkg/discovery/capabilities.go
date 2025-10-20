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
	"time"

	"github.com/unified-replication/operator/pkg/translation"
)

// BackendCapability represents a specific capability that a backend can support
type BackendCapability string

const (
	// Core replication capabilities
	CapabilityAsyncReplication BackendCapability = "async_replication"
	CapabilitySyncReplication  BackendCapability = "sync_replication"
	CapabilityMetroReplication BackendCapability = "metro_replication"

	// State management capabilities
	CapabilitySourcePromotion BackendCapability = "source_promotion"
	CapabilityReplicaDemotion BackendCapability = "replica_demotion"
	CapabilityFailover        BackendCapability = "failover"
	CapabilityFailback        BackendCapability = "failback"
	CapabilityResync          BackendCapability = "resync"

	// Advanced features
	CapabilitySnapshotBased     BackendCapability = "snapshot_based"
	CapabilityJournalBased      BackendCapability = "journal_based"
	CapabilityAutoResync        BackendCapability = "auto_resync"
	CapabilityScheduledSync     BackendCapability = "scheduled_sync"
	CapabilityVolumeGroups      BackendCapability = "volume_groups"
	CapabilityConsistencyGroups BackendCapability = "consistency_groups"

	// Performance and scaling
	CapabilityHighThroughput BackendCapability = "high_throughput"
	CapabilityLowLatency     BackendCapability = "low_latency"
	CapabilityMultiRegion    BackendCapability = "multi_region"
	CapabilityMultiCloud     BackendCapability = "multi_cloud"

	// Management capabilities
	CapabilityMetrics  BackendCapability = "metrics"
	CapabilityAlerting BackendCapability = "alerting"
	CapabilityLogging  BackendCapability = "logging"
)

// CapabilityLevel indicates the level of support for a capability
type CapabilityLevel string

const (
	CapabilityLevelFull    CapabilityLevel = "full"    // Fully supported
	CapabilityLevelPartial CapabilityLevel = "partial" // Partially supported
	CapabilityLevelBasic   CapabilityLevel = "basic"   // Basic support
	CapabilityLevelNone    CapabilityLevel = "none"    // Not supported
	CapabilityLevelUnknown CapabilityLevel = "unknown" // Support level unknown
)

// CapabilityInfo provides detailed information about a capability
type CapabilityInfo struct {
	Capability   BackendCapability `json:"capability"`
	Level        CapabilityLevel   `json:"level"`
	Version      string            `json:"version,omitempty"`      // Version when capability was introduced
	Description  string            `json:"description,omitempty"`  // Human-readable description
	Limitations  []string          `json:"limitations,omitempty"`  // Known limitations
	Requirements []string          `json:"requirements,omitempty"` // Prerequisites for this capability
	LastChecked  time.Time         `json:"last_checked"`
}

// BackendCapabilities represents all capabilities for a backend
type BackendCapabilities struct {
	Backend      translation.Backend                  `json:"backend"`
	Capabilities map[BackendCapability]CapabilityInfo `json:"capabilities"`
	Version      string                               `json:"version,omitempty"`
	LastUpdated  time.Time                            `json:"last_updated"`
	Health       HealthStatus                         `json:"health"`
}

// HealthStatus represents the health status of a backend
type HealthStatus struct {
	Status      HealthLevel   `json:"status"`
	Message     string        `json:"message,omitempty"`
	LastChecked time.Time     `json:"last_checked"`
	Checks      []HealthCheck `json:"checks,omitempty"`
}

// HealthLevel indicates the health level of a backend
type HealthLevel string

const (
	HealthLevelHealthy   HealthLevel = "healthy"   // All systems operational
	HealthLevelDegraded  HealthLevel = "degraded"  // Some issues but functional
	HealthLevelUnhealthy HealthLevel = "unhealthy" // Major issues, limited functionality
	HealthLevelUnknown   HealthLevel = "unknown"   // Health status unknown
)

// HealthCheck represents a specific health check
type HealthCheck struct {
	Name        string        `json:"name"`
	Status      HealthLevel   `json:"status"`
	Message     string        `json:"message,omitempty"`
	LastChecked time.Time     `json:"last_checked"`
	Duration    time.Duration `json:"duration,omitempty"`
}

// PerformanceCharacteristics represents performance metrics for a backend
type PerformanceCharacteristics struct {
	Backend           translation.Backend `json:"backend"`
	MaxThroughputMBps int64               `json:"max_throughput_mbps,omitempty"`
	TypicalLatencyMs  int64               `json:"typical_latency_ms,omitempty"`
	MaxConcurrentOps  int64               `json:"max_concurrent_ops,omitempty"`
	MaxVolumeSize     string              `json:"max_volume_size,omitempty"`
	MaxVolumesPerRG   int64               `json:"max_volumes_per_rg,omitempty"` // Replication Group
	SupportedRegions  []string            `json:"supported_regions,omitempty"`
	LastMeasured      time.Time           `json:"last_measured"`
}

// VersionInfo represents version information for a backend
type VersionInfo struct {
	Backend             translation.Backend `json:"backend"`
	Version             string              `json:"version"`
	APIVersion          string              `json:"api_version,omitempty"`
	ControllerVersion   string              `json:"controller_version,omitempty"`
	DriverVersion       string              `json:"driver_version,omitempty"`
	MinSupportedVersion string              `json:"min_supported_version,omitempty"`
	MaxSupportedVersion string              `json:"max_supported_version,omitempty"`
	DeprecationWarnings []string            `json:"deprecation_warnings,omitempty"`
	LastDetected        time.Time           `json:"last_detected"`
}

// CapabilityDetector interface for detecting backend capabilities
type CapabilityDetector interface {
	// DetectCapabilities detects all capabilities for a backend
	DetectCapabilities(ctx context.Context) (*BackendCapabilities, error)

	// CheckHealth performs health checks on the backend
	CheckHealth(ctx context.Context) (*HealthStatus, error)

	// GetPerformanceCharacteristics retrieves performance metrics
	GetPerformanceCharacteristics(ctx context.Context) (*PerformanceCharacteristics, error)

	// GetVersionInfo retrieves version information
	GetVersionInfo(ctx context.Context) (*VersionInfo, error)

	// ValidateCapability checks if a specific capability is supported
	ValidateCapability(ctx context.Context, capability BackendCapability) (*CapabilityInfo, error)
}

// CapabilityRegistry manages capability information for all backends
type CapabilityRegistry interface {
	// RegisterCapabilities registers capabilities for a backend
	RegisterCapabilities(backend translation.Backend, capabilities *BackendCapabilities) error

	// GetCapabilities returns capabilities for a backend
	GetCapabilities(backend translation.Backend) (*BackendCapabilities, bool)

	// GetAllCapabilities returns capabilities for all registered backends
	GetAllCapabilities() map[translation.Backend]*BackendCapabilities

	// UpdateCapabilities updates capabilities for a backend
	UpdateCapabilities(backend translation.Backend, capabilities *BackendCapabilities) error

	// RefreshCapabilities refreshes capabilities for a backend
	RefreshCapabilities(ctx context.Context, backend translation.Backend) error

	// RefreshAllCapabilities refreshes capabilities for all backends
	RefreshAllCapabilities(ctx context.Context) error

	// IsCapabilitySupported checks if a backend supports a capability
	IsCapabilitySupported(backend translation.Backend, capability BackendCapability) (CapabilityLevel, bool)

	// GetSupportedBackends returns backends that support a specific capability
	GetSupportedBackends(capability BackendCapability, minLevel CapabilityLevel) []translation.Backend

	// ValidateConfiguration validates if a configuration is supported by backend capabilities
	ValidateConfiguration(backend translation.Backend, config map[string]interface{}) error
}

// CapabilityFilter represents filters for capability queries
type CapabilityFilter struct {
	Capabilities []BackendCapability `json:"capabilities,omitempty"`
	MinLevel     CapabilityLevel     `json:"min_level,omitempty"`
	HealthLevel  HealthLevel         `json:"health_level,omitempty"`
	Version      string              `json:"version,omitempty"`
}

// CapabilityQuery represents a query for backends with specific capabilities
type CapabilityQuery struct {
	RequiredCapabilities []BackendCapability `json:"required_capabilities,omitempty"`
	OptionalCapabilities []BackendCapability `json:"optional_capabilities,omitempty"`
	MinLevel             CapabilityLevel     `json:"min_level,omitempty"`
	RequireHealthy       bool                `json:"require_healthy,omitempty"`
	MinVersion           string              `json:"min_version,omitempty"`
}

// CapabilityQueryResult represents the result of a capability query
type CapabilityQueryResult struct {
	Backend      translation.Backend  `json:"backend"`
	Capabilities *BackendCapabilities `json:"capabilities"`
	Score        float64              `json:"score"` // Compatibility score 0-1
	Reasons      []string             `json:"reasons,omitempty"`
}

// Enhanced discovery result with capabilities
type EnhancedDiscoveryResult struct {
	*DiscoveryResult
	Capabilities map[translation.Backend]*BackendCapabilities        `json:"capabilities,omitempty"`
	Performance  map[translation.Backend]*PerformanceCharacteristics `json:"performance,omitempty"`
	Versions     map[translation.Backend]*VersionInfo                `json:"versions,omitempty"`
}

// CapabilityConfig configures capability detection behavior
type CapabilityConfig struct {
	EnableCapabilityDetection  bool          `json:"enable_capability_detection"`
	EnableHealthChecking       bool          `json:"enable_health_checking"`
	EnablePerformanceMetrics   bool          `json:"enable_performance_metrics"`
	EnableVersionDetection     bool          `json:"enable_version_detection"`
	HealthCheckInterval        time.Duration `json:"health_check_interval"`
	CapabilityRefreshInterval  time.Duration `json:"capability_refresh_interval"`
	PerformanceMetricsInterval time.Duration `json:"performance_metrics_interval"`
	TimeoutPerCheck            time.Duration `json:"timeout_per_check"`
	MaxConcurrentChecks        int           `json:"max_concurrent_checks"`
}

// DefaultCapabilityConfig returns the default capability configuration
func DefaultCapabilityConfig() *CapabilityConfig {
	return &CapabilityConfig{
		EnableCapabilityDetection:  true,
		EnableHealthChecking:       true,
		EnablePerformanceMetrics:   false, // Disabled by default as it may be expensive
		EnableVersionDetection:     true,
		HealthCheckInterval:        1 * time.Minute,
		CapabilityRefreshInterval:  5 * time.Minute,
		PerformanceMetricsInterval: 15 * time.Minute,
		TimeoutPerCheck:            30 * time.Second,
		MaxConcurrentChecks:        3,
	}
}
