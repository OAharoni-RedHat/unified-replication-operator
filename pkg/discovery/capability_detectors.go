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

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/unified-replication/operator/pkg/translation"
)

// BaseCapabilityDetector provides common functionality for capability detection
type BaseCapabilityDetector struct {
	client  client.Client
	backend translation.Backend
}

// NewBaseCapabilityDetector creates a new base capability detector
func NewBaseCapabilityDetector(client client.Client, backend translation.Backend) *BaseCapabilityDetector {
	return &BaseCapabilityDetector{
		client:  client,
		backend: backend,
	}
}

// GetVersionInfo retrieves version information from CRD
func (bcd *BaseCapabilityDetector) GetVersionInfo(ctx context.Context) (*VersionInfo, error) {
	crds, exists := GetRequiredCRDsForBackend(bcd.backend)
	if !exists || len(crds) == 0 {
		return nil, fmt.Errorf("no CRDs defined for backend %s", bcd.backend)
	}

	// Use the first CRD to get version info
	primaryCRD := crds[0]

	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := bcd.client.Get(ctx, types.NamespacedName{Name: primaryCRD.Name}, crd)
	if err != nil {
		return nil, fmt.Errorf("failed to get CRD %s: %w", primaryCRD.Name, err)
	}

	versionInfo := &VersionInfo{
		Backend:      bcd.backend,
		LastDetected: time.Now(),
	}

	// Extract version information from CRD
	if len(crd.Spec.Versions) > 0 {
		// Find the storage version
		for _, version := range crd.Spec.Versions {
			if version.Storage {
				versionInfo.APIVersion = version.Name
				break
			}
		}
		if versionInfo.APIVersion == "" {
			versionInfo.APIVersion = crd.Spec.Versions[0].Name
		}
	}

	// Try to extract controller version from annotations or labels
	if controllerVersion, exists := crd.Annotations["controller.version"]; exists {
		versionInfo.ControllerVersion = controllerVersion
	}

	if driverVersion, exists := crd.Annotations["driver.version"]; exists {
		versionInfo.DriverVersion = driverVersion
	}

	// Set a general version based on API version
	versionInfo.Version = versionInfo.APIVersion

	return versionInfo, nil
}

// CheckHealth performs basic health checks
func (bcd *BaseCapabilityDetector) CheckHealth(ctx context.Context) (*HealthStatus, error) {
	crds, exists := GetRequiredCRDsForBackend(bcd.backend)
	if !exists {
		return &HealthStatus{
			Status:      HealthLevelUnhealthy,
			Message:     "No CRDs defined for backend",
			LastChecked: time.Now(),
		}, nil
	}

	healthStatus := &HealthStatus{
		Status:      HealthLevelHealthy,
		LastChecked: time.Now(),
		Checks:      make([]HealthCheck, 0, len(crds)),
	}

	healthyChecks := 0
	for _, crdDef := range crds {
		check := HealthCheck{
			Name:        fmt.Sprintf("CRD-%s", crdDef.Kind),
			LastChecked: time.Now(),
		}

		startTime := time.Now()
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := bcd.client.Get(ctx, types.NamespacedName{Name: crdDef.Name}, crd)
		check.Duration = time.Since(startTime)

		if err != nil {
			check.Status = HealthLevelUnhealthy
			check.Message = fmt.Sprintf("CRD not found: %v", err)
		} else {
			// Check if CRD is established
			established := false
			for _, condition := range crd.Status.Conditions {
				if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
					established = true
					break
				}
			}

			if established {
				check.Status = HealthLevelHealthy
				check.Message = "CRD is established and ready"
				healthyChecks++
			} else {
				check.Status = HealthLevelDegraded
				check.Message = "CRD exists but not established"
			}
		}

		healthStatus.Checks = append(healthStatus.Checks, check)
	}

	// Determine overall health
	if healthyChecks == len(crds) {
		healthStatus.Status = HealthLevelHealthy
		healthStatus.Message = "All CRDs are healthy"
	} else if healthyChecks > 0 {
		healthStatus.Status = HealthLevelDegraded
		healthStatus.Message = fmt.Sprintf("%d/%d CRDs are healthy", healthyChecks, len(crds))
	} else {
		healthStatus.Status = HealthLevelUnhealthy
		healthStatus.Message = "No CRDs are healthy"
	}

	return healthStatus, nil
}

// GetPerformanceCharacteristics returns basic performance characteristics
func (bcd *BaseCapabilityDetector) GetPerformanceCharacteristics(ctx context.Context) (*PerformanceCharacteristics, error) {
	// Base implementation returns default characteristics
	return &PerformanceCharacteristics{
		Backend:      bcd.backend,
		LastMeasured: time.Now(),
		// Default values - should be overridden by specific implementations
		TypicalLatencyMs: 100,
		MaxConcurrentOps: 10,
	}, nil
}

// CephCapabilityDetector implements capability detection for Ceph
type CephCapabilityDetector struct {
	*BaseCapabilityDetector
}

// NewCephCapabilityDetector creates a new Ceph capability detector
func NewCephCapabilityDetector(client client.Client) CapabilityDetector {
	return &CephCapabilityDetector{
		BaseCapabilityDetector: NewBaseCapabilityDetector(client, translation.BackendCeph),
	}
}

// DetectCapabilities detects Ceph-specific capabilities
func (ccd *CephCapabilityDetector) DetectCapabilities(ctx context.Context) (*BackendCapabilities, error) {
	capabilities := &BackendCapabilities{
		Backend:      translation.BackendCeph,
		Capabilities: make(map[BackendCapability]CapabilityInfo),
		LastUpdated:  time.Now(),
	}

	// Core replication capabilities
	capabilities.Capabilities[CapabilityAsyncReplication] = CapabilityInfo{
		Capability:  CapabilityAsyncReplication,
		Level:       CapabilityLevelFull,
		Description: "Ceph RBD mirroring supports asynchronous replication",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilitySyncReplication] = CapabilityInfo{
		Capability:  CapabilitySyncReplication,
		Level:       CapabilityLevelBasic,
		Description: "Ceph supports basic synchronous replication",
		LastChecked: time.Now(),
	}

	// State management capabilities
	capabilities.Capabilities[CapabilitySourcePromotion] = CapabilityInfo{
		Capability:  CapabilitySourcePromotion,
		Level:       CapabilityLevelFull,
		Description: "Ceph supports promoting replica to primary",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityReplicaDemotion] = CapabilityInfo{
		Capability:  CapabilityReplicaDemotion,
		Level:       CapabilityLevelFull,
		Description: "Ceph supports demoting primary to secondary",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityResync] = CapabilityInfo{
		Capability:  CapabilityResync,
		Level:       CapabilityLevelFull,
		Description: "Ceph supports resynchronization of mirrors",
		LastChecked: time.Now(),
	}

	// Advanced features
	capabilities.Capabilities[CapabilityJournalBased] = CapabilityInfo{
		Capability:  CapabilityJournalBased,
		Level:       CapabilityLevelFull,
		Description: "Ceph supports journal-based mirroring",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilitySnapshotBased] = CapabilityInfo{
		Capability:  CapabilitySnapshotBased,
		Level:       CapabilityLevelFull,
		Description: "Ceph supports snapshot-based mirroring",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityAutoResync] = CapabilityInfo{
		Capability:  CapabilityAutoResync,
		Level:       CapabilityLevelPartial,
		Description: "Ceph supports automatic resync with configuration",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityScheduledSync] = CapabilityInfo{
		Capability:  CapabilityScheduledSync,
		Level:       CapabilityLevelFull,
		Description: "Ceph supports scheduled snapshot mirroring",
		LastChecked: time.Now(),
	}

	// Performance characteristics
	capabilities.Capabilities[CapabilityHighThroughput] = CapabilityInfo{
		Capability:  CapabilityHighThroughput,
		Level:       CapabilityLevelFull,
		Description: "Ceph is designed for high throughput workloads",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityMultiRegion] = CapabilityInfo{
		Capability:  CapabilityMultiRegion,
		Level:       CapabilityLevelFull,
		Description: "Ceph supports multi-region deployments",
		LastChecked: time.Now(),
	}

	return capabilities, nil
}

// GetPerformanceCharacteristics returns Ceph-specific performance characteristics
func (ccd *CephCapabilityDetector) GetPerformanceCharacteristics(ctx context.Context) (*PerformanceCharacteristics, error) {
	return &PerformanceCharacteristics{
		Backend:           translation.BackendCeph,
		MaxThroughputMBps: 1000,                     // Typical for Ceph RBD
		TypicalLatencyMs:  5,                        // Low latency for local operations
		MaxConcurrentOps:  100,                      // High concurrency support
		MaxVolumeSize:     "16TB",                   // Ceph RBD limit
		SupportedRegions:  []string{"multi-region"}, // Supports cross-region
		LastMeasured:      time.Now(),
	}, nil
}

// ValidateCapability validates a specific Ceph capability
func (ccd *CephCapabilityDetector) ValidateCapability(ctx context.Context, capability BackendCapability) (*CapabilityInfo, error) {
	capabilities, err := ccd.DetectCapabilities(ctx)
	if err != nil {
		return nil, err
	}

	capInfo, exists := capabilities.Capabilities[capability]
	if !exists {
		return &CapabilityInfo{
			Capability:  capability,
			Level:       CapabilityLevelNone,
			Description: "Capability not supported by Ceph",
			LastChecked: time.Now(),
		}, nil
	}

	return &capInfo, nil
}

// TridentCapabilityDetector implements capability detection for Trident
type TridentCapabilityDetector struct {
	*BaseCapabilityDetector
}

// NewTridentCapabilityDetector creates a new Trident capability detector
func NewTridentCapabilityDetector(client client.Client) CapabilityDetector {
	return &TridentCapabilityDetector{
		BaseCapabilityDetector: NewBaseCapabilityDetector(client, translation.BackendTrident),
	}
}

// DetectCapabilities detects Trident-specific capabilities
func (tcd *TridentCapabilityDetector) DetectCapabilities(ctx context.Context) (*BackendCapabilities, error) {
	capabilities := &BackendCapabilities{
		Backend:      translation.BackendTrident,
		Capabilities: make(map[BackendCapability]CapabilityInfo),
		LastUpdated:  time.Now(),
	}

	// Core replication capabilities
	capabilities.Capabilities[CapabilityAsyncReplication] = CapabilityInfo{
		Capability:  CapabilityAsyncReplication,
		Level:       CapabilityLevelFull,
		Description: "Trident supports asynchronous SnapMirror replication",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilitySyncReplication] = CapabilityInfo{
		Capability:  CapabilitySyncReplication,
		Level:       CapabilityLevelFull,
		Description: "Trident supports synchronous SnapMirror replication",
		LastChecked: time.Now(),
	}

	// State management capabilities
	capabilities.Capabilities[CapabilitySourcePromotion] = CapabilityInfo{
		Capability:  CapabilitySourcePromotion,
		Level:       CapabilityLevelFull,
		Description: "Trident supports promoting mirror destinations",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityFailover] = CapabilityInfo{
		Capability:  CapabilityFailover,
		Level:       CapabilityLevelFull,
		Description: "Trident supports automated failover operations",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityFailback] = CapabilityInfo{
		Capability:  CapabilityFailback,
		Level:       CapabilityLevelFull,
		Description: "Trident supports failback operations",
		LastChecked: time.Now(),
	}

	// Advanced features
	capabilities.Capabilities[CapabilitySnapshotBased] = CapabilityInfo{
		Capability:  CapabilitySnapshotBased,
		Level:       CapabilityLevelFull,
		Description: "Trident uses NetApp snapshot technology",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityScheduledSync] = CapabilityInfo{
		Capability:  CapabilityScheduledSync,
		Level:       CapabilityLevelFull,
		Description: "Trident supports scheduled SnapMirror updates",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityConsistencyGroups] = CapabilityInfo{
		Capability:  CapabilityConsistencyGroups,
		Level:       CapabilityLevelFull,
		Description: "Trident supports NetApp consistency groups",
		LastChecked: time.Now(),
	}

	// Performance characteristics
	capabilities.Capabilities[CapabilityLowLatency] = CapabilityInfo{
		Capability:  CapabilityLowLatency,
		Level:       CapabilityLevelFull,
		Description: "NetApp storage provides low latency performance",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityMultiCloud] = CapabilityInfo{
		Capability:  CapabilityMultiCloud,
		Level:       CapabilityLevelFull,
		Description: "Trident supports multi-cloud deployments",
		LastChecked: time.Now(),
	}

	return capabilities, nil
}

// GetPerformanceCharacteristics returns Trident-specific performance characteristics
func (tcd *TridentCapabilityDetector) GetPerformanceCharacteristics(ctx context.Context) (*PerformanceCharacteristics, error) {
	return &PerformanceCharacteristics{
		Backend:           translation.BackendTrident,
		MaxThroughputMBps: 2000,    // High throughput for NetApp
		TypicalLatencyMs:  2,       // Very low latency
		MaxConcurrentOps:  200,     // High concurrency
		MaxVolumeSize:     "100TB", // Large volume support
		SupportedRegions:  []string{"multi-cloud", "hybrid-cloud"},
		LastMeasured:      time.Now(),
	}, nil
}

// ValidateCapability validates a specific Trident capability
func (tcd *TridentCapabilityDetector) ValidateCapability(ctx context.Context, capability BackendCapability) (*CapabilityInfo, error) {
	capabilities, err := tcd.DetectCapabilities(ctx)
	if err != nil {
		return nil, err
	}

	capInfo, exists := capabilities.Capabilities[capability]
	if !exists {
		return &CapabilityInfo{
			Capability:  capability,
			Level:       CapabilityLevelNone,
			Description: "Capability not supported by Trident",
			LastChecked: time.Now(),
		}, nil
	}

	return &capInfo, nil
}

// PowerStoreCapabilityDetector implements capability detection for PowerStore
type PowerStoreCapabilityDetector struct {
	*BaseCapabilityDetector
}

// NewPowerStoreCapabilityDetector creates a new PowerStore capability detector
func NewPowerStoreCapabilityDetector(client client.Client) CapabilityDetector {
	return &PowerStoreCapabilityDetector{
		BaseCapabilityDetector: NewBaseCapabilityDetector(client, translation.BackendPowerStore),
	}
}

// DetectCapabilities detects PowerStore-specific capabilities
func (pcd *PowerStoreCapabilityDetector) DetectCapabilities(ctx context.Context) (*BackendCapabilities, error) {
	capabilities := &BackendCapabilities{
		Backend:      translation.BackendPowerStore,
		Capabilities: make(map[BackendCapability]CapabilityInfo),
		LastUpdated:  time.Now(),
	}

	// Core replication capabilities
	capabilities.Capabilities[CapabilityAsyncReplication] = CapabilityInfo{
		Capability:  CapabilityAsyncReplication,
		Level:       CapabilityLevelFull,
		Description: "PowerStore supports asynchronous replication",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilitySyncReplication] = CapabilityInfo{
		Capability:  CapabilitySyncReplication,
		Level:       CapabilityLevelFull,
		Description: "PowerStore supports synchronous replication",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityMetroReplication] = CapabilityInfo{
		Capability:  CapabilityMetroReplication,
		Level:       CapabilityLevelFull,
		Description: "PowerStore supports Metro (active-active) replication",
		LastChecked: time.Now(),
	}

	// State management capabilities
	capabilities.Capabilities[CapabilitySourcePromotion] = CapabilityInfo{
		Capability:  CapabilitySourcePromotion,
		Level:       CapabilityLevelFull,
		Description: "PowerStore supports role reversal operations",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityReplicaDemotion] = CapabilityInfo{
		Capability:  CapabilityReplicaDemotion,
		Level:       CapabilityLevelFull,
		Description: "PowerStore supports role changes",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityFailover] = CapabilityInfo{
		Capability:  CapabilityFailover,
		Level:       CapabilityLevelFull,
		Description: "PowerStore supports automated failover",
		LastChecked: time.Now(),
	}

	// Advanced features
	capabilities.Capabilities[CapabilityVolumeGroups] = CapabilityInfo{
		Capability:  CapabilityVolumeGroups,
		Level:       CapabilityLevelFull,
		Description: "PowerStore supports replication groups",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityConsistencyGroups] = CapabilityInfo{
		Capability:  CapabilityConsistencyGroups,
		Level:       CapabilityLevelFull,
		Description: "PowerStore provides application consistency",
		LastChecked: time.Now(),
	}

	// Performance characteristics
	capabilities.Capabilities[CapabilityHighThroughput] = CapabilityInfo{
		Capability:  CapabilityHighThroughput,
		Level:       CapabilityLevelFull,
		Description: "PowerStore is optimized for high throughput",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityLowLatency] = CapabilityInfo{
		Capability:  CapabilityLowLatency,
		Level:       CapabilityLevelFull,
		Description: "PowerStore provides sub-millisecond latency",
		LastChecked: time.Now(),
	}

	capabilities.Capabilities[CapabilityMultiRegion] = CapabilityInfo{
		Capability:  CapabilityMultiRegion,
		Level:       CapabilityLevelFull,
		Description: "PowerStore supports multi-site deployments",
		LastChecked: time.Now(),
	}

	return capabilities, nil
}

// GetPerformanceCharacteristics returns PowerStore-specific performance characteristics
func (pcd *PowerStoreCapabilityDetector) GetPerformanceCharacteristics(ctx context.Context) (*PerformanceCharacteristics, error) {
	return &PerformanceCharacteristics{
		Backend:           translation.BackendPowerStore,
		MaxThroughputMBps: 3000,    // Very high throughput
		TypicalLatencyMs:  1,       // Sub-millisecond latency
		MaxConcurrentOps:  500,     // Very high concurrency
		MaxVolumeSize:     "256TB", // Large volume support
		MaxVolumesPerRG:   1000,    // Large replication groups
		SupportedRegions:  []string{"multi-site", "metro"},
		LastMeasured:      time.Now(),
	}, nil
}

// ValidateCapability validates a specific PowerStore capability
func (pcd *PowerStoreCapabilityDetector) ValidateCapability(ctx context.Context, capability BackendCapability) (*CapabilityInfo, error) {
	capabilities, err := pcd.DetectCapabilities(ctx)
	if err != nil {
		return nil, err
	}

	capInfo, exists := capabilities.Capabilities[capability]
	if !exists {
		return &CapabilityInfo{
			Capability:  capability,
			Level:       CapabilityLevelNone,
			Description: "Capability not supported by PowerStore",
			LastChecked: time.Now(),
		}, nil
	}

	return &capInfo, nil
}
