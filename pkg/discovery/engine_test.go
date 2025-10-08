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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/unified-replication/operator/pkg/translation"
)

func TestCRDDefinitions(t *testing.T) {
	t.Run("GetRequiredCRDsForBackend", func(t *testing.T) {
		// Test Ceph CRDs
		cephCRDs, exists := GetRequiredCRDsForBackend(translation.BackendCeph)
		assert.True(t, exists)
		assert.Len(t, cephCRDs, 2) // VolumeReplicationClass and VolumeReplication

		// Test Trident CRDs
		tridentCRDs, exists := GetRequiredCRDsForBackend(translation.BackendTrident)
		assert.True(t, exists)
		assert.Len(t, tridentCRDs, 3) // TridentMirrorRelationship, TridentActionMirrorUpdate, TridentVolume

		// Test PowerStore CRDs
		powerstoreCRDs, exists := GetRequiredCRDsForBackend(translation.BackendPowerStore)
		assert.True(t, exists)
		assert.Len(t, powerstoreCRDs, 1) // DellCSIReplicationGroup

		// Test unknown backend
		_, exists = GetRequiredCRDsForBackend("unknown")
		assert.False(t, exists)
	})

	t.Run("GetBackendFromCRD", func(t *testing.T) {
		backend, found := GetBackendFromCRD("volumereplicationclasses.replication.storage.openshift.io")
		assert.True(t, found)
		assert.Equal(t, translation.BackendCeph, backend)

		backend, found = GetBackendFromCRD("tridentmirrorrelationships.trident.netapp.io")
		assert.True(t, found)
		assert.Equal(t, translation.BackendTrident, backend)

		backend, found = GetBackendFromCRD("dellcsireplicationgroups.replication.storage.dell.com")
		assert.True(t, found)
		assert.Equal(t, translation.BackendPowerStore, backend)

		_, found = GetBackendFromCRD("unknown.crd.io")
		assert.False(t, found)
	})

	t.Run("GetRequiredCRDs", func(t *testing.T) {
		cephRequired := GetRequiredCRDs(translation.BackendCeph)
		assert.Len(t, cephRequired, 2) // Both Ceph CRDs are required
		for _, crd := range cephRequired {
			assert.True(t, crd.Required)
		}

		tridentRequired := GetRequiredCRDs(translation.BackendTrident)
		assert.Len(t, tridentRequired, 2) // TridentMirrorRelationship and TridentVolume are required

		tridentOptional := GetOptionalCRDs(translation.BackendTrident)
		assert.Len(t, tridentOptional, 1) // TridentActionMirrorUpdate is optional
	})

	t.Run("CRDDefinition methods", func(t *testing.T) {
		crd := CRDDefinition{
			Name:    "test.example.com",
			Group:   "example.com",
			Version: "v1",
			Kind:    "Test",
		}

		assert.Equal(t, "test.example.com", crd.String())
		assert.Equal(t, "example.com/v1/Test", crd.FullName())
		assert.True(t, crd.IsClusterScoped())

		crd.Namespace = "test-ns"
		assert.False(t, crd.IsClusterScoped())
	})
}

func TestEngine_CRDOperations(t *testing.T) {
	t.Run("CheckCRDExists", func(t *testing.T) {
		// Create fake client with one CRD
		crd := createCRD("test.example.com", "example.com", "v1", "Test", true)
		fakeClient := createFakeClient(crd)
		engine := NewEngine(fakeClient, nil)

		// Test existing CRD
		exists, err := engine.CheckCRDExists(context.Background(), "test.example.com")
		assert.NoError(t, err)
		assert.True(t, exists)

		// Test non-existing CRD
		exists, err = engine.CheckCRDExists(context.Background(), "nonexistent.example.com")
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("CheckCRDReady", func(t *testing.T) {
		// Create established CRD
		establishedCRD := createCRD("ready.example.com", "example.com", "v1", "Ready", true)
		// Create non-established CRD
		notReadyCRD := createCRD("notready.example.com", "example.com", "v1", "NotReady", false)

		fakeClient := createFakeClient(establishedCRD, notReadyCRD)
		engine := NewEngine(fakeClient, nil)

		// Test ready CRD
		ready, err := engine.CheckCRDReady(context.Background(), "ready.example.com")
		assert.NoError(t, err)
		assert.True(t, ready)

		// Test not ready CRD
		ready, err = engine.CheckCRDReady(context.Background(), "notready.example.com")
		assert.NoError(t, err)
		assert.False(t, ready)

		// Test non-existing CRD
		ready, err = engine.CheckCRDReady(context.Background(), "nonexistent.example.com")
		assert.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("GetCRDInfo", func(t *testing.T) {
		crd := createCRD("info.example.com", "example.com", "v1", "Info", true)
		fakeClient := createFakeClient(crd)
		engine := NewEngine(fakeClient, nil)

		info, err := engine.GetCRDInfo(context.Background(), "info.example.com")
		assert.NoError(t, err)
		assert.Equal(t, "info.example.com", info.Name)
		assert.Equal(t, "example.com", info.Group)
		assert.Equal(t, "v1", info.Version)
		assert.Equal(t, "Info", info.Kind)
		assert.True(t, info.Available)
		assert.True(t, info.Controller)

		// Test non-existing CRD
		info, err = engine.GetCRDInfo(context.Background(), "nonexistent.example.com")
		assert.NoError(t, err)
		assert.Equal(t, "nonexistent.example.com", info.Name)
		assert.False(t, info.Available)
	})
}

func TestBaseDetector(t *testing.T) {
	t.Run("DetectBackend with all CRDs available", func(t *testing.T) {
		// Create all required CRDs for Ceph
		cephCRDs, _ := GetRequiredCRDsForBackend(translation.BackendCeph)
		var objects []client.Object
		for _, crdDef := range cephCRDs {
			crd := createCRD(crdDef.Name, crdDef.Group, crdDef.Version, crdDef.Kind, true)
			objects = append(objects, crd)
		}

		fakeClient := createFakeClient(objects...)
		detector := NewCephDetector(fakeClient)

		result, err := detector.DetectBackend(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, translation.BackendCeph, result.Backend)
		assert.Equal(t, BackendStatusAvailable, result.Status)
		assert.Len(t, result.CRDs, len(cephCRDs))

		// All CRDs should be available
		for _, crdInfo := range result.CRDs {
			assert.True(t, crdInfo.Available)
		}
	})

	t.Run("DetectBackend with missing CRDs", func(t *testing.T) {
		// Create client with no CRDs
		fakeClient := createFakeClient()
		detector := NewCephDetector(fakeClient)

		result, err := detector.DetectBackend(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, translation.BackendCeph, result.Backend)
		assert.Equal(t, BackendStatusUnavailable, result.Status)

		// All CRDs should be unavailable
		for _, crdInfo := range result.CRDs {
			assert.False(t, crdInfo.Available)
		}
	})

	t.Run("DetectBackend with partial CRDs", func(t *testing.T) {
		// Create only one of the required Ceph CRDs
		crd := createCRD("volumereplicationclasses.replication.storage.openshift.io",
			"replication.storage.openshift.io", "v1alpha1", "VolumeReplicationClass", true)
		fakeClient := createFakeClient(crd)
		detector := NewCephDetector(fakeClient)

		result, err := detector.DetectBackend(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, translation.BackendCeph, result.Backend)
		assert.Equal(t, BackendStatusPartial, result.Status)

		// One CRD should be available, one unavailable
		availableCount := 0
		for _, crdInfo := range result.CRDs {
			if crdInfo.Available {
				availableCount++
			}
		}
		assert.Equal(t, 1, availableCount)
	})

	t.Run("GetRequiredCRDs", func(t *testing.T) {
		fakeClient := createFakeClient()
		detector := NewCephDetector(fakeClient)

		requiredCRDs := detector.GetRequiredCRDs()
		assert.Len(t, requiredCRDs, 2) // Ceph has 2 required CRDs

		for _, crd := range requiredCRDs {
			assert.NotEmpty(t, crd.Name)
			assert.NotEmpty(t, crd.Group)
			assert.NotEmpty(t, crd.Kind)
		}
	})

	t.Run("GetBackendType", func(t *testing.T) {
		fakeClient := createFakeClient()

		cephDetector := NewCephDetector(fakeClient)
		assert.Equal(t, translation.BackendCeph, cephDetector.GetBackendType())

		tridentDetector := NewTridentDetector(fakeClient)
		assert.Equal(t, translation.BackendTrident, tridentDetector.GetBackendType())

		powerstoreDetector := NewPowerStoreDetector(fakeClient)
		assert.Equal(t, translation.BackendPowerStore, powerstoreDetector.GetBackendType())
	})

	t.Run("ValidateBackend", func(t *testing.T) {
		// Test successful validation
		cephCRDs, _ := GetRequiredCRDsForBackend(translation.BackendCeph)
		var objects []client.Object
		for _, crdDef := range cephCRDs {
			crd := createCRD(crdDef.Name, crdDef.Group, crdDef.Version, crdDef.Kind, true)
			objects = append(objects, crd)
		}

		fakeClient := createFakeClient(objects...)
		detector := NewCephDetector(fakeClient)

		err := detector.ValidateBackend(context.Background())
		assert.NoError(t, err)

		// Test failed validation (no CRDs)
		emptyClient := createFakeClient()
		emptyDetector := NewCephDetector(emptyClient)

		err = emptyDetector.ValidateBackend(context.Background())
		assert.Error(t, err)
		assert.True(t, IsDiscoveryError(err))
	})
}

func TestEngine_DiscoveryOperations(t *testing.T) {
	t.Run("DiscoverBackends with all backends available", func(t *testing.T) {
		// Create all CRDs for all backends
		var objects []client.Object
		for backend := range BackendCRDMap {
			crds, _ := GetRequiredCRDsForBackend(backend)
			for _, crdDef := range crds {
				crd := createCRD(crdDef.Name, crdDef.Group, crdDef.Version, crdDef.Kind, true)
				objects = append(objects, crd)
			}
		}

		fakeClient := createFakeClient(objects...)
		engine := NewEngine(fakeClient, DefaultDiscoveryConfig())

		result, err := engine.DiscoverBackends(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Backends, 3) // Ceph, Trident, PowerStore
		assert.Len(t, result.AvailableBackends, 3)

		for backend, backendResult := range result.Backends {
			assert.Equal(t, BackendStatusAvailable, backendResult.Status, "Backend %s should be available", backend)
		}
	})

	t.Run("DiscoverBackends with no backends available", func(t *testing.T) {
		fakeClient := createFakeClient() // No CRDs
		engine := NewEngine(fakeClient, DefaultDiscoveryConfig())

		result, err := engine.DiscoverBackends(context.Background())
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Backends, 3)          // All backends checked
		assert.Len(t, result.AvailableBackends, 0) // None available

		for backend, backendResult := range result.Backends {
			assert.Equal(t, BackendStatusUnavailable, backendResult.Status, "Backend %s should be unavailable", backend)
		}
	})

	t.Run("DiscoverBackend specific backend", func(t *testing.T) {
		// Create only Ceph CRDs
		cephCRDs, _ := GetRequiredCRDsForBackend(translation.BackendCeph)
		var objects []client.Object
		for _, crdDef := range cephCRDs {
			crd := createCRD(crdDef.Name, crdDef.Group, crdDef.Version, crdDef.Kind, true)
			objects = append(objects, crd)
		}

		fakeClient := createFakeClient(objects...)
		engine := NewEngine(fakeClient, DefaultDiscoveryConfig())

		// Test Ceph (should be available)
		result, err := engine.DiscoverBackend(context.Background(), translation.BackendCeph)
		assert.NoError(t, err)
		assert.Equal(t, BackendStatusAvailable, result.Status)

		// Test Trident (should be unavailable)
		result, err = engine.DiscoverBackend(context.Background(), translation.BackendTrident)
		assert.NoError(t, err)
		assert.Equal(t, BackendStatusUnavailable, result.Status)
	})

	t.Run("IsBackendAvailable", func(t *testing.T) {
		// Create only Ceph CRDs
		cephCRDs, _ := GetRequiredCRDsForBackend(translation.BackendCeph)
		var objects []client.Object
		for _, crdDef := range cephCRDs {
			crd := createCRD(crdDef.Name, crdDef.Group, crdDef.Version, crdDef.Kind, true)
			objects = append(objects, crd)
		}

		fakeClient := createFakeClient(objects...)
		engine := NewEngine(fakeClient, DefaultDiscoveryConfig())

		// Test available backend
		available, err := engine.IsBackendAvailable(context.Background(), translation.BackendCeph)
		assert.NoError(t, err)
		assert.True(t, available)

		// Test unavailable backend
		available, err = engine.IsBackendAvailable(context.Background(), translation.BackendTrident)
		assert.NoError(t, err)
		assert.False(t, available)
	})

	t.Run("GetAvailableBackends", func(t *testing.T) {
		// Create CRDs for only Ceph and PowerStore
		var objects []client.Object

		// Add Ceph CRDs
		cephCRDs, _ := GetRequiredCRDsForBackend(translation.BackendCeph)
		for _, crdDef := range cephCRDs {
			crd := createCRD(crdDef.Name, crdDef.Group, crdDef.Version, crdDef.Kind, true)
			objects = append(objects, crd)
		}

		// Add PowerStore CRDs
		powerstoreCRDs, _ := GetRequiredCRDsForBackend(translation.BackendPowerStore)
		for _, crdDef := range powerstoreCRDs {
			crd := createCRD(crdDef.Name, crdDef.Group, crdDef.Version, crdDef.Kind, true)
			objects = append(objects, crd)
		}

		fakeClient := createFakeClient(objects...)
		engine := NewEngine(fakeClient, DefaultDiscoveryConfig())

		availableBackends, err := engine.GetAvailableBackends(context.Background())
		assert.NoError(t, err)
		assert.Len(t, availableBackends, 2)
		assert.Contains(t, availableBackends, translation.BackendCeph)
		assert.Contains(t, availableBackends, translation.BackendPowerStore)
		assert.NotContains(t, availableBackends, translation.BackendTrident)
	})
}

func TestDiscoveryConfig(t *testing.T) {
	t.Run("DefaultDiscoveryConfig", func(t *testing.T) {
		config := DefaultDiscoveryConfig()
		assert.NotNil(t, config)
		assert.Equal(t, 5*time.Minute, config.CacheTTL)
		assert.Equal(t, 30*time.Second, config.RefreshInterval)
		assert.Equal(t, 10*time.Second, config.TimeoutPerBackend)
		assert.False(t, config.EnableAutoRefresh)
		assert.Equal(t, 3, config.MaxRetries)
		assert.Equal(t, 1*time.Second, config.RetryDelay)
	})

	t.Run("CustomDiscoveryConfig", func(t *testing.T) {
		config := &DiscoveryConfig{
			CacheTTL:          1 * time.Minute,
			RefreshInterval:   10 * time.Second,
			TimeoutPerBackend: 5 * time.Second,
			EnableAutoRefresh: true,
			MaxRetries:        1,
			RetryDelay:        500 * time.Millisecond,
		}

		fakeClient := createFakeClient()
		engine := NewEngine(fakeClient, config)
		assert.Equal(t, config, engine.config)
	})
}

func TestCaching(t *testing.T) {
	t.Run("GetCachedResult empty cache", func(t *testing.T) {
		fakeClient := createFakeClient()
		engine := NewEngine(fakeClient, DefaultDiscoveryConfig())

		result, valid := engine.GetCachedResult()
		assert.Nil(t, result)
		assert.False(t, valid)
	})

	t.Run("GetCachedResult with valid cache", func(t *testing.T) {
		fakeClient := createFakeClient()
		config := DefaultDiscoveryConfig()
		config.CacheTTL = 1 * time.Hour // Long TTL
		engine := NewEngine(fakeClient, config)

		// Perform discovery to populate cache
		originalResult, err := engine.DiscoverBackends(context.Background())
		assert.NoError(t, err)

		// Get cached result
		cachedResult, valid := engine.GetCachedResult()
		assert.True(t, valid)
		assert.NotNil(t, cachedResult)
		assert.Equal(t, len(originalResult.Backends), len(cachedResult.Backends))
	})

	t.Run("GetCachedResult with expired cache", func(t *testing.T) {
		fakeClient := createFakeClient()
		config := DefaultDiscoveryConfig()
		config.CacheTTL = 1 * time.Millisecond // Very short TTL
		engine := NewEngine(fakeClient, config)

		// Perform discovery to populate cache
		_, err := engine.DiscoverBackends(context.Background())
		assert.NoError(t, err)

		// Wait for cache to expire
		time.Sleep(2 * time.Millisecond)

		// Get cached result (should be expired)
		cachedResult, valid := engine.GetCachedResult()
		assert.False(t, valid)
		assert.Nil(t, cachedResult)
	})
}

func TestDiscoveryError(t *testing.T) {
	t.Run("NewDiscoveryError", func(t *testing.T) {
		err := NewDiscoveryError(ErrorTypeCRDNotFound, translation.BackendCeph, "test-crd", "CRD not found")
		assert.Equal(t, ErrorTypeCRDNotFound, err.Type)
		assert.Equal(t, translation.BackendCeph, err.Backend)
		assert.Equal(t, "test-crd", err.CRD)
		assert.Contains(t, err.Error(), "CRD not found")
	})

	t.Run("IsDiscoveryError", func(t *testing.T) {
		discoveryErr := NewDiscoveryError(ErrorTypeCRDNotFound, translation.BackendCeph, "test", "test")
		assert.True(t, IsDiscoveryError(discoveryErr))

		regularErr := assert.AnError
		assert.False(t, IsDiscoveryError(regularErr))
	})

	t.Run("GetDiscoveryError", func(t *testing.T) {
		originalErr := NewDiscoveryError(ErrorTypeCRDNotFound, translation.BackendCeph, "test", "test")

		extractedErr, ok := GetDiscoveryError(originalErr)
		assert.True(t, ok)
		assert.Equal(t, originalErr, extractedErr)

		_, ok = GetDiscoveryError(assert.AnError)
		assert.False(t, ok)
	})
}

func TestDetectorRegistry(t *testing.T) {
	t.Run("NewDetectorRegistry", func(t *testing.T) {
		fakeClient := createFakeClient()
		registry := NewDetectorRegistry(fakeClient)

		assert.NotNil(t, registry)
		assert.Len(t, registry.detectors, 3)

		// Test all backends are registered
		for _, backend := range translation.GetSupportedBackends() {
			detector, exists := registry.GetDetector(backend)
			assert.True(t, exists, "Detector for backend %s should exist", backend)
			assert.NotNil(t, detector)
			assert.Equal(t, backend, detector.GetBackendType())
		}
	})

	t.Run("DetectAll", func(t *testing.T) {
		fakeClient := createFakeClient()
		registry := NewDetectorRegistry(fakeClient)

		results, err := registry.DetectAll(context.Background())
		assert.NoError(t, err)
		assert.Len(t, results, 3)

		for backend, result := range results {
			assert.Equal(t, backend, result.Backend)
			assert.Equal(t, BackendStatusUnavailable, result.Status) // No CRDs available
		}
	})

	t.Run("RegisterDetector", func(t *testing.T) {
		fakeClient := createFakeClient()
		registry := NewDetectorRegistry(fakeClient)

		// Create custom detector
		customDetector := NewCephDetector(fakeClient)
		registry.RegisterDetector("custom", customDetector)

		detector, exists := registry.GetDetector("custom")
		assert.True(t, exists)
		assert.Equal(t, customDetector, detector)
	})
}
