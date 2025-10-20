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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/unified-replication/operator/pkg/translation"
)

func TestCapabilityTypes(t *testing.T) {
	t.Run("BackendCapability constants", func(t *testing.T) {
		// Test that all capability constants are defined
		capabilities := []BackendCapability{
			CapabilityAsyncReplication,
			CapabilitySyncReplication,
			CapabilityMetroReplication,
			CapabilitySourcePromotion,
			CapabilityReplicaDemotion,
			CapabilityFailover,
			CapabilityFailback,
			CapabilityResync,
			CapabilitySnapshotBased,
			CapabilityJournalBased,
			CapabilityAutoResync,
			CapabilityScheduledSync,
			CapabilityVolumeGroups,
			CapabilityConsistencyGroups,
			CapabilityHighThroughput,
			CapabilityLowLatency,
			CapabilityMultiRegion,
			CapabilityMultiCloud,
			CapabilityMetrics,
			CapabilityAlerting,
			CapabilityLogging,
		}

		for _, cap := range capabilities {
			assert.NotEmpty(t, string(cap), "Capability constant should not be empty")
		}
	})

	t.Run("CapabilityLevel constants", func(t *testing.T) {
		levels := []CapabilityLevel{
			CapabilityLevelFull,
			CapabilityLevelPartial,
			CapabilityLevelBasic,
			CapabilityLevelNone,
			CapabilityLevelUnknown,
		}

		for _, level := range levels {
			assert.NotEmpty(t, string(level), "Capability level constant should not be empty")
		}
	})

	t.Run("HealthLevel constants", func(t *testing.T) {
		levels := []HealthLevel{
			HealthLevelHealthy,
			HealthLevelDegraded,
			HealthLevelUnhealthy,
			HealthLevelUnknown,
		}

		for _, level := range levels {
			assert.NotEmpty(t, string(level), "Health level constant should not be empty")
		}
	})
}

func TestCapabilityInfo(t *testing.T) {
	t.Run("CapabilityInfo structure", func(t *testing.T) {
		info := CapabilityInfo{
			Capability:   CapabilityAsyncReplication,
			Level:        CapabilityLevelFull,
			Version:      "v1.0",
			Description:  "Full async replication support",
			Limitations:  []string{"Max 100 volumes"},
			Requirements: []string{"CSI driver v1.5+"},
			LastChecked:  time.Now(),
		}

		assert.Equal(t, CapabilityAsyncReplication, info.Capability)
		assert.Equal(t, CapabilityLevelFull, info.Level)
		assert.Equal(t, "v1.0", info.Version)
		assert.Equal(t, "Full async replication support", info.Description)
		assert.Len(t, info.Limitations, 1)
		assert.Len(t, info.Requirements, 1)
		assert.False(t, info.LastChecked.IsZero())
	})
}

func TestBackendCapabilities(t *testing.T) {
	t.Run("BackendCapabilities structure", func(t *testing.T) {
		capabilities := BackendCapabilities{
			Backend: translation.BackendCeph,
			Capabilities: map[BackendCapability]CapabilityInfo{
				CapabilityAsyncReplication: {
					Capability:  CapabilityAsyncReplication,
					Level:       CapabilityLevelFull,
					LastChecked: time.Now(),
				},
			},
			Version:     "16.2.0",
			LastUpdated: time.Now(),
			Health: HealthStatus{
				Status:      HealthLevelHealthy,
				LastChecked: time.Now(),
			},
		}

		assert.Equal(t, translation.BackendCeph, capabilities.Backend)
		assert.Len(t, capabilities.Capabilities, 1)
		assert.Equal(t, "16.2.0", capabilities.Version)
		assert.Equal(t, HealthLevelHealthy, capabilities.Health.Status)
	})
}

func TestInMemoryCapabilityRegistry(t *testing.T) {
	t.Run("NewInMemoryCapabilityRegistry", func(t *testing.T) {
		registry := NewInMemoryCapabilityRegistry()
		assert.NotNil(t, registry)
		assert.NotNil(t, registry.capabilities)
		assert.NotNil(t, registry.detectors)
	})

	t.Run("RegisterCapabilities", func(t *testing.T) {
		registry := NewInMemoryCapabilityRegistry()

		capabilities := &BackendCapabilities{
			Backend: translation.BackendCeph,
			Capabilities: map[BackendCapability]CapabilityInfo{
				CapabilityAsyncReplication: {
					Capability: CapabilityAsyncReplication,
					Level:      CapabilityLevelFull,
				},
			},
		}

		err := registry.RegisterCapabilities(translation.BackendCeph, capabilities)
		assert.NoError(t, err)

		// Test nil capabilities
		err = registry.RegisterCapabilities(translation.BackendTrident, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "capabilities cannot be nil")
	})

	t.Run("GetCapabilities", func(t *testing.T) {
		registry := NewInMemoryCapabilityRegistry()

		capabilities := &BackendCapabilities{
			Backend: translation.BackendCeph,
		}

		// Register capabilities
		err := registry.RegisterCapabilities(translation.BackendCeph, capabilities)
		require.NoError(t, err)

		// Get existing capabilities
		retrieved, exists := registry.GetCapabilities(translation.BackendCeph)
		assert.True(t, exists)
		assert.NotNil(t, retrieved)
		assert.Equal(t, translation.BackendCeph, retrieved.Backend)

		// Get non-existing capabilities
		_, exists = registry.GetCapabilities(translation.BackendTrident)
		assert.False(t, exists)
	})

	t.Run("UpdateCapabilities", func(t *testing.T) {
		registry := NewInMemoryCapabilityRegistry()

		// Register initial capabilities
		initial := &BackendCapabilities{
			Backend: translation.BackendCeph,
			Version: "16.1.0",
			Capabilities: map[BackendCapability]CapabilityInfo{
				CapabilityAsyncReplication: {
					Capability: CapabilityAsyncReplication,
					Level:      CapabilityLevelPartial,
				},
			},
		}
		err := registry.RegisterCapabilities(translation.BackendCeph, initial)
		require.NoError(t, err)

		// Update capabilities
		updated := &BackendCapabilities{
			Backend: translation.BackendCeph,
			Version: "16.2.0",
			Capabilities: map[BackendCapability]CapabilityInfo{
				CapabilityAsyncReplication: {
					Capability: CapabilityAsyncReplication,
					Level:      CapabilityLevelFull,
				},
				CapabilitySyncReplication: {
					Capability: CapabilitySyncReplication,
					Level:      CapabilityLevelBasic,
				},
			},
		}

		err = registry.UpdateCapabilities(translation.BackendCeph, updated)
		assert.NoError(t, err)

		// Verify update
		retrieved, exists := registry.GetCapabilities(translation.BackendCeph)
		assert.True(t, exists)
		assert.Equal(t, "16.2.0", retrieved.Version)
		assert.Len(t, retrieved.Capabilities, 2)
		assert.Equal(t, CapabilityLevelFull, retrieved.Capabilities[CapabilityAsyncReplication].Level)
	})

	t.Run("IsCapabilitySupported", func(t *testing.T) {
		registry := NewInMemoryCapabilityRegistry()

		capabilities := &BackendCapabilities{
			Backend: translation.BackendCeph,
			Capabilities: map[BackendCapability]CapabilityInfo{
				CapabilityAsyncReplication: {
					Capability: CapabilityAsyncReplication,
					Level:      CapabilityLevelFull,
				},
			},
		}
		err := registry.RegisterCapabilities(translation.BackendCeph, capabilities)
		require.NoError(t, err)

		// Test supported capability
		level, supported := registry.IsCapabilitySupported(translation.BackendCeph, CapabilityAsyncReplication)
		assert.True(t, supported)
		assert.Equal(t, CapabilityLevelFull, level)

		// Test unsupported capability
		level, supported = registry.IsCapabilitySupported(translation.BackendCeph, CapabilitySyncReplication)
		assert.True(t, supported) // Backend exists
		assert.Equal(t, CapabilityLevelNone, level)

		// Test non-existent backend
		level, supported = registry.IsCapabilitySupported(translation.BackendTrident, CapabilityAsyncReplication)
		assert.False(t, supported)
		assert.Equal(t, CapabilityLevelUnknown, level)
	})

	t.Run("GetSupportedBackends", func(t *testing.T) {
		registry := NewInMemoryCapabilityRegistry()

		// Register capabilities for multiple backends
		cephCaps := &BackendCapabilities{
			Backend: translation.BackendCeph,
			Capabilities: map[BackendCapability]CapabilityInfo{
				CapabilityAsyncReplication: {Level: CapabilityLevelFull},
			},
		}
		tridentCaps := &BackendCapabilities{
			Backend: translation.BackendTrident,
			Capabilities: map[BackendCapability]CapabilityInfo{
				CapabilityAsyncReplication: {Level: CapabilityLevelPartial},
				CapabilitySyncReplication:  {Level: CapabilityLevelFull},
			},
		}

		err := registry.RegisterCapabilities(translation.BackendCeph, cephCaps)
		require.NoError(t, err)
		err = registry.RegisterCapabilities(translation.BackendTrident, tridentCaps)
		require.NoError(t, err)

		// Test with basic level requirement
		backends := registry.GetSupportedBackends(CapabilityAsyncReplication, CapabilityLevelBasic)
		assert.Len(t, backends, 2)
		assert.Contains(t, backends, translation.BackendCeph)
		assert.Contains(t, backends, translation.BackendTrident)

		// Test with full level requirement
		backends = registry.GetSupportedBackends(CapabilityAsyncReplication, CapabilityLevelFull)
		assert.Len(t, backends, 1)
		assert.Contains(t, backends, translation.BackendCeph)

		// Test with unsupported capability
		backends = registry.GetSupportedBackends(CapabilityMetroReplication, CapabilityLevelBasic)
		assert.Len(t, backends, 0)
	})
}

func TestCapabilityDetectors(t *testing.T) {
	// Create fake client with established CRDs
	cephCRDs, _ := GetRequiredCRDsForBackend(translation.BackendCeph)
	var objects []client.Object
	for _, crdDef := range cephCRDs {
		crd := createCRD(crdDef.Name, crdDef.Group, crdDef.Version, crdDef.Kind, true)
		objects = append(objects, crd)
	}
	fakeClient := createFakeClient(objects...)

	t.Run("CephCapabilityDetector", func(t *testing.T) {
		detector := NewCephCapabilityDetector(fakeClient)
		assert.NotNil(t, detector)

		// Test capability detection
		capabilities, err := detector.DetectCapabilities(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, capabilities)
		assert.Equal(t, translation.BackendCeph, capabilities.Backend)

		// Verify some expected capabilities
		assert.Contains(t, capabilities.Capabilities, CapabilityAsyncReplication)
		assert.Contains(t, capabilities.Capabilities, CapabilityJournalBased)
		assert.Contains(t, capabilities.Capabilities, CapabilitySnapshotBased)

		// Test health check
		health, err := detector.CheckHealth(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, health)
		assert.Equal(t, HealthLevelHealthy, health.Status)

		// Test performance characteristics
		perf, err := detector.GetPerformanceCharacteristics(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, perf)
		assert.Equal(t, translation.BackendCeph, perf.Backend)

		// Test version info
		version, err := detector.GetVersionInfo(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, version)
		assert.Equal(t, translation.BackendCeph, version.Backend)
	})

	t.Run("TridentCapabilityDetector", func(t *testing.T) {
		detector := NewTridentCapabilityDetector(fakeClient)
		assert.NotNil(t, detector)

		capabilities, err := detector.DetectCapabilities(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, capabilities)
		assert.Equal(t, translation.BackendTrident, capabilities.Backend)

		// Verify some expected capabilities
		assert.Contains(t, capabilities.Capabilities, CapabilityAsyncReplication)
		assert.Contains(t, capabilities.Capabilities, CapabilitySyncReplication)
		assert.Contains(t, capabilities.Capabilities, CapabilityFailover)
		assert.Contains(t, capabilities.Capabilities, CapabilityConsistencyGroups)
	})

	t.Run("PowerStoreCapabilityDetector", func(t *testing.T) {
		detector := NewPowerStoreCapabilityDetector(fakeClient)
		assert.NotNil(t, detector)

		capabilities, err := detector.DetectCapabilities(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, capabilities)
		assert.Equal(t, translation.BackendPowerStore, capabilities.Backend)

		// Verify some expected capabilities
		assert.Contains(t, capabilities.Capabilities, CapabilityAsyncReplication)
		assert.Contains(t, capabilities.Capabilities, CapabilityMetroReplication)
		assert.Contains(t, capabilities.Capabilities, CapabilityVolumeGroups)
		assert.Contains(t, capabilities.Capabilities, CapabilityLowLatency)
	})
}

func TestEnhancedEngine(t *testing.T) {
	// Create fake client with all CRDs
	var objects []client.Object
	for backend := range BackendCRDMap {
		crds, _ := GetRequiredCRDsForBackend(backend)
		for _, crdDef := range crds {
			crd := createCRD(crdDef.Name, crdDef.Group, crdDef.Version, crdDef.Kind, true)
			objects = append(objects, crd)
		}
	}
	fakeClient := createFakeClient(objects...)

	t.Run("NewEnhancedEngine", func(t *testing.T) {
		config := DefaultDiscoveryConfig()
		capConfig := DefaultCapabilityConfig()

		engine := NewEnhancedEngine(fakeClient, config, capConfig)
		assert.NotNil(t, engine)
		assert.NotNil(t, engine.Engine)
		assert.NotNil(t, engine.capabilityRegistry)
		assert.Len(t, engine.capabilityDetectors, 3)
	})

	t.Run("DiscoverBackendsWithCapabilities", func(t *testing.T) {
		config := DefaultDiscoveryConfig()
		capConfig := DefaultCapabilityConfig()
		capConfig.EnablePerformanceMetrics = true
		capConfig.EnableVersionDetection = true

		engine := NewEnhancedEngine(fakeClient, config, capConfig)

		result, err := engine.DiscoverBackendsWithCapabilities(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.DiscoveryResult)
		assert.Len(t, result.AvailableBackends, 3) // All backends should be available

		// Check that capabilities were detected
		assert.Len(t, result.Capabilities, 3)
		for _, backend := range result.AvailableBackends {
			assert.Contains(t, result.Capabilities, backend)
			assert.Contains(t, result.Performance, backend)
			assert.Contains(t, result.Versions, backend)
		}
	})

	t.Run("QueryBackendsByCapabilities", func(t *testing.T) {
		config := DefaultDiscoveryConfig()
		capConfig := DefaultCapabilityConfig()

		engine := NewEnhancedEngine(fakeClient, config, capConfig)

		// First discover backends to populate capabilities
		_, err := engine.DiscoverBackendsWithCapabilities(context.Background())
		require.NoError(t, err)

		// Query for backends with async replication
		query := CapabilityQuery{
			RequiredCapabilities: []BackendCapability{CapabilityAsyncReplication},
			MinLevel:             CapabilityLevelBasic,
		}

		results, err := engine.QueryBackendsByCapabilities(query)
		assert.NoError(t, err)
		assert.NotEmpty(t, results)

		// All backends should support async replication
		assert.Len(t, results, 3)
		for _, result := range results {
			assert.Greater(t, result.Score, 0.0)
			assert.Contains(t, result.Capabilities.Capabilities, CapabilityAsyncReplication)
		}
	})

	t.Run("ValidateBackendConfiguration", func(t *testing.T) {
		config := DefaultDiscoveryConfig()
		capConfig := DefaultCapabilityConfig()

		engine := NewEnhancedEngine(fakeClient, config, capConfig)

		// First discover backends to populate capabilities
		_, err := engine.DiscoverBackendsWithCapabilities(context.Background())
		require.NoError(t, err)

		// Test valid configuration
		validConfig := map[string]interface{}{
			"replicationMode": "asynchronous",
		}
		err = engine.ValidateBackendConfiguration(translation.BackendCeph, validConfig)
		assert.NoError(t, err)

		// Test invalid configuration
		invalidConfig := map[string]interface{}{
			"replicationMode": "unknown_mode",
		}
		err = engine.ValidateBackendConfiguration(translation.BackendCeph, invalidConfig)
		assert.Error(t, err)
	})
}

func TestCapabilityConfig(t *testing.T) {
	t.Run("DefaultCapabilityConfig", func(t *testing.T) {
		config := DefaultCapabilityConfig()
		assert.NotNil(t, config)
		assert.True(t, config.EnableCapabilityDetection)
		assert.True(t, config.EnableHealthChecking)
		assert.False(t, config.EnablePerformanceMetrics) // Should be disabled by default
		assert.True(t, config.EnableVersionDetection)
		assert.Equal(t, 1*time.Minute, config.HealthCheckInterval)
		assert.Equal(t, 5*time.Minute, config.CapabilityRefreshInterval)
		assert.Equal(t, 30*time.Second, config.TimeoutPerCheck)
		assert.Equal(t, 3, config.MaxConcurrentChecks)
	})

	t.Run("CustomCapabilityConfig", func(t *testing.T) {
		config := &CapabilityConfig{
			EnableCapabilityDetection: false,
			EnableHealthChecking:      false,
			EnablePerformanceMetrics:  true,
			EnableVersionDetection:    false,
			HealthCheckInterval:       30 * time.Second,
			CapabilityRefreshInterval: 2 * time.Minute,
			TimeoutPerCheck:           10 * time.Second,
			MaxConcurrentChecks:       5,
		}

		assert.False(t, config.EnableCapabilityDetection)
		assert.False(t, config.EnableHealthChecking)
		assert.True(t, config.EnablePerformanceMetrics)
		assert.False(t, config.EnableVersionDetection)
		assert.Equal(t, 30*time.Second, config.HealthCheckInterval)
		assert.Equal(t, 2*time.Minute, config.CapabilityRefreshInterval)
		assert.Equal(t, 10*time.Second, config.TimeoutPerCheck)
		assert.Equal(t, 5, config.MaxConcurrentChecks)
	})
}

func TestCapabilityValidation(t *testing.T) {
	registry := NewInMemoryCapabilityRegistry()

	// Register Ceph capabilities
	cephCaps := &BackendCapabilities{
		Backend: translation.BackendCeph,
		Capabilities: map[BackendCapability]CapabilityInfo{
			CapabilityAsyncReplication: {Level: CapabilityLevelFull},
			CapabilityJournalBased:     {Level: CapabilityLevelFull},
			CapabilitySnapshotBased:    {Level: CapabilityLevelFull},
		},
	}
	err := registry.RegisterCapabilities(translation.BackendCeph, cephCaps)
	require.NoError(t, err)

	t.Run("ValidateReplicationMode", func(t *testing.T) {
		// Valid async mode
		config := map[string]interface{}{
			"replicationMode": "asynchronous",
		}
		err := registry.ValidateConfiguration(translation.BackendCeph, config)
		assert.NoError(t, err)

		// Invalid mode
		config = map[string]interface{}{
			"replicationMode": "unknown",
		}
		err = registry.ValidateConfiguration(translation.BackendCeph, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown replication mode")
	})

	t.Run("ValidateCephExtensions", func(t *testing.T) {
		// Valid journal mode
		config := map[string]interface{}{
			"extensions": map[string]interface{}{
				"ceph": map[string]interface{}{
					"mirroringMode": "journal",
				},
			},
		}
		err := registry.ValidateConfiguration(translation.BackendCeph, config)
		assert.NoError(t, err)

		// Valid snapshot mode
		config = map[string]interface{}{
			"extensions": map[string]interface{}{
				"ceph": map[string]interface{}{
					"mirroringMode": "snapshot",
				},
			},
		}
		err = registry.ValidateConfiguration(translation.BackendCeph, config)
		assert.NoError(t, err)

		// Invalid mode
		config = map[string]interface{}{
			"extensions": map[string]interface{}{
				"ceph": map[string]interface{}{
					"mirroringMode": "unknown",
				},
			},
		}
		err = registry.ValidateConfiguration(translation.BackendCeph, config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown ceph mirroring mode")
	})
}
