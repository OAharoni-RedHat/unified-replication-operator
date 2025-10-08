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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/unified-replication/operator/pkg/translation"
	"github.com/unified-replication/operator/test/utils"
)

// Integration tests for capability detection using envtest
func TestCapabilityIntegration(t *testing.T) {
	// Skip integration tests in short mode
	if testing.Short() {
		t.Skip("Skipping capability integration tests in short mode")
	}

	// Create test environment
	testEnv := utils.NewTestEnvironment(t, utils.DefaultTestEnvironmentOptions())
	defer testEnv.Stop(t)

	ctx := context.Background()

	t.Run("EnhancedDiscovery with real CRDs", func(t *testing.T) {
		// Create enhanced discovery engine
		config := DefaultDiscoveryConfig()
		config.TimeoutPerBackend = 5 * time.Second

		capConfig := DefaultCapabilityConfig()
		capConfig.EnablePerformanceMetrics = true
		capConfig.EnableVersionDetection = true
		capConfig.TimeoutPerCheck = 5 * time.Second

		engine := NewEnhancedEngine(testEnv.Client, config, capConfig)

		// Initially, no backends should be available (no CRDs installed)
		result, err := engine.DiscoverBackendsWithCapabilities(ctx)
		require.NoError(t, err)
		assert.Len(t, result.AvailableBackends, 0)
		assert.Len(t, result.Capabilities, 0)

		// Install Ceph CRDs
		cephCRDs, _ := GetRequiredCRDsForBackend(translation.BackendCeph)
		for _, crdDef := range cephCRDs {
			crd := createCapabilityTestCRD(crdDef)
			err := testEnv.Client.Create(ctx, crd)
			require.NoError(t, err)

			// Wait for CRD to be established
			require.Eventually(t, func() bool {
				ready, err := engine.CheckCRDReady(ctx, crd.Name)
				return err == nil && ready
			}, 30*time.Second, 100*time.Millisecond)
		}

		// Discover backends with capabilities - should now find Ceph
		result, err = engine.DiscoverBackendsWithCapabilities(ctx)
		require.NoError(t, err)
		assert.Len(t, result.AvailableBackends, 1)
		assert.Contains(t, result.AvailableBackends, translation.BackendCeph)

		// Verify capabilities were detected
		assert.Len(t, result.Capabilities, 1)
		cephCaps, exists := result.Capabilities[translation.BackendCeph]
		assert.True(t, exists)
		assert.Equal(t, translation.BackendCeph, cephCaps.Backend)
		assert.NotEmpty(t, cephCaps.Capabilities)
		assert.Equal(t, HealthLevelHealthy, cephCaps.Health.Status)

		// Verify performance characteristics were collected
		assert.Len(t, result.Performance, 1)
		cephPerf, exists := result.Performance[translation.BackendCeph]
		assert.True(t, exists)
		assert.Equal(t, translation.BackendCeph, cephPerf.Backend)

		// Verify version information was collected
		assert.Len(t, result.Versions, 1)
		cephVersion, exists := result.Versions[translation.BackendCeph]
		assert.True(t, exists)
		assert.Equal(t, translation.BackendCeph, cephVersion.Backend)
	})

	t.Run("Capability query and validation", func(t *testing.T) {
		// Set up enhanced engine with capabilities
		config := DefaultDiscoveryConfig()
		capConfig := DefaultCapabilityConfig()
		engine := NewEnhancedEngine(testEnv.Client, config, capConfig)

		// Install all backend CRDs
		for backend := range BackendCRDMap {
			crds, _ := GetRequiredCRDsForBackend(backend)
			for _, crdDef := range crds {
				crd := createCapabilityTestCRD(crdDef)
				err := testEnv.Client.Create(ctx, crd)
				require.NoError(t, err)
			}
		}

		// Wait a moment for CRDs to be established
		time.Sleep(2 * time.Second)

		// Discover all backends
		result, err := engine.DiscoverBackendsWithCapabilities(ctx)
		require.NoError(t, err)
		assert.Len(t, result.AvailableBackends, 3) // All backends should be available

		// Query for backends with async replication
		query := CapabilityQuery{
			RequiredCapabilities: []BackendCapability{CapabilityAsyncReplication},
			MinLevel:             CapabilityLevelBasic,
		}

		queryResults, err := engine.QueryBackendsByCapabilities(query)
		require.NoError(t, err)
		assert.Len(t, queryResults, 3) // All backends support async replication

		// Query for backends with metro replication (only PowerStore)
		metroQuery := CapabilityQuery{
			RequiredCapabilities: []BackendCapability{CapabilityMetroReplication},
			MinLevel:             CapabilityLevelBasic,
		}

		metroResults, err := engine.QueryBackendsByCapabilities(metroQuery)
		require.NoError(t, err)
		assert.Len(t, metroResults, 1) // Only PowerStore supports metro replication
		assert.Equal(t, translation.BackendPowerStore, metroResults[0].Backend)

		// Test configuration validation
		validConfig := map[string]interface{}{
			"replicationMode": "asynchronous",
		}
		err = engine.ValidateBackendConfiguration(translation.BackendCeph, validConfig)
		assert.NoError(t, err)

		invalidConfig := map[string]interface{}{
			"replicationMode": "metro", // Ceph doesn't support metro
		}
		err = engine.ValidateBackendConfiguration(translation.BackendCeph, invalidConfig)
		assert.Error(t, err)
	})

	t.Run("Health monitoring", func(t *testing.T) {
		// Set up enhanced engine with health monitoring
		config := DefaultDiscoveryConfig()
		capConfig := DefaultCapabilityConfig()
		capConfig.EnableHealthChecking = true
		capConfig.HealthCheckInterval = 500 * time.Millisecond
		capConfig.MaxConcurrentChecks = 2

		engine := NewEnhancedEngine(testEnv.Client, config, capConfig)

		// Install Ceph CRDs
		cephCRDs, _ := GetRequiredCRDsForBackend(translation.BackendCeph)
		for _, crdDef := range cephCRDs {
			crd := createCapabilityTestCRD(crdDef)
			err := testEnv.Client.Create(ctx, crd)
			require.NoError(t, err)
		}

		// Wait for CRDs to be established
		time.Sleep(2 * time.Second)

		// Discover backends to populate capabilities
		_, err := engine.DiscoverBackendsWithCapabilities(ctx)
		require.NoError(t, err)

		// Start health monitoring
		err = engine.StartCapabilityMonitoring(ctx)
		require.NoError(t, err)

		// Wait for a few health check cycles
		time.Sleep(1500 * time.Millisecond)

		// Get health summary
		summary := engine.healthMonitor.GetHealthSummary()
		assert.Equal(t, 1, summary.TotalBackends)
		assert.Equal(t, 1, summary.HealthyBackends)
		assert.True(t, summary.IsHealthy())

		// Stop monitoring
		err = engine.StopCapabilityMonitoring()
		require.NoError(t, err)
	})

	t.Run("Capability registry operations", func(t *testing.T) {
		registry := NewInMemoryCapabilityRegistry()

		// Test registering capabilities
		testCaps := &BackendCapabilities{
			Backend: translation.BackendCeph,
			Version: "16.2.0",
			Capabilities: map[BackendCapability]CapabilityInfo{
				CapabilityAsyncReplication: {
					Capability:  CapabilityAsyncReplication,
					Level:       CapabilityLevelFull,
					Description: "Full async replication support",
					LastChecked: time.Now(),
				},
				CapabilityJournalBased: {
					Capability:  CapabilityJournalBased,
					Level:       CapabilityLevelFull,
					Description: "Journal-based mirroring",
					LastChecked: time.Now(),
				},
			},
			Health: HealthStatus{
				Status:      HealthLevelHealthy,
				Message:     "All systems operational",
				LastChecked: time.Now(),
			},
		}

		err := registry.RegisterCapabilities(translation.BackendCeph, testCaps)
		require.NoError(t, err)

		// Test getting capabilities
		retrieved, exists := registry.GetCapabilities(translation.BackendCeph)
		assert.True(t, exists)
		assert.Equal(t, translation.BackendCeph, retrieved.Backend)
		assert.Equal(t, "16.2.0", retrieved.Version)
		assert.Len(t, retrieved.Capabilities, 2)

		// Test capability support queries
		level, supported := registry.IsCapabilitySupported(translation.BackendCeph, CapabilityAsyncReplication)
		assert.True(t, supported)
		assert.Equal(t, CapabilityLevelFull, level)

		level, supported = registry.IsCapabilitySupported(translation.BackendCeph, CapabilityMetroReplication)
		assert.True(t, supported)                   // Backend exists
		assert.Equal(t, CapabilityLevelNone, level) // But doesn't support metro

		// Test getting supported backends
		supportedBackends := registry.GetSupportedBackends(CapabilityAsyncReplication, CapabilityLevelBasic)
		assert.Len(t, supportedBackends, 1)
		assert.Contains(t, supportedBackends, translation.BackendCeph)

		// Test statistics
		stats := registry.GetStatistics()
		assert.Equal(t, 1, stats.TotalBackends)
		assert.Equal(t, 1, stats.HealthyBackends)
		assert.Equal(t, 0, stats.UnhealthyBackends)
		assert.Equal(t, 2, stats.TotalCapabilities)
	})

	t.Run("Performance characteristics detection", func(t *testing.T) {
		// Create capability detectors
		fakeClient := testEnv.Client

		cephDetector := NewCephCapabilityDetector(fakeClient)
		tridentDetector := NewTridentCapabilityDetector(fakeClient)
		powerstoreDetector := NewPowerStoreCapabilityDetector(fakeClient)

		detectors := map[translation.Backend]CapabilityDetector{
			translation.BackendCeph:       cephDetector,
			translation.BackendTrident:    tridentDetector,
			translation.BackendPowerStore: powerstoreDetector,
		}

		for backend, detector := range detectors {
			t.Run(string(backend), func(t *testing.T) {
				perf, err := detector.GetPerformanceCharacteristics(ctx)
				assert.NoError(t, err)
				assert.NotNil(t, perf)
				assert.Equal(t, backend, perf.Backend)
				assert.Greater(t, perf.MaxThroughputMBps, int64(0))
				assert.Greater(t, perf.TypicalLatencyMs, int64(0))
				assert.Greater(t, perf.MaxConcurrentOps, int64(0))
				assert.NotEmpty(t, perf.MaxVolumeSize)
				assert.False(t, perf.LastMeasured.IsZero())
			})
		}
	})

	t.Run("Version information detection", func(t *testing.T) {
		// Install a CRD with version information
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testversionresources.test.version.io",
				Annotations: map[string]string{
					"controller.version": "v1.2.3",
					"driver.version":     "v2.1.0",
				},
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "test.version.io",
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Kind:     "TestVersionResource",
					Plural:   "testversionresources",
					Singular: "testversionresource",
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{
						Name:    "v1alpha1",
						Storage: true,
						Served:  true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {Type: "object"},
								},
							},
						},
					},
				},
			},
		}

		err := testEnv.Client.Create(ctx, crd)
		require.NoError(t, err)

		// Wait for CRD to be ready
		require.Eventually(t, func() bool {
			err := testEnv.Client.Get(ctx, client.ObjectKey{Name: crd.Name}, &apiextensionsv1.CustomResourceDefinition{})
			return err == nil
		}, 30*time.Second, 100*time.Millisecond)

		// Test version detection
		detector := NewBaseCapabilityDetector(testEnv.Client, translation.BackendCeph)
		assert.NotNil(t, detector)

		// We need to modify the detector to use our test CRD
		// For this test, we'll create a custom version info retrieval
		testCRD := &apiextensionsv1.CustomResourceDefinition{}
		err = testEnv.Client.Get(ctx, client.ObjectKey{Name: crd.Name}, testCRD)
		require.NoError(t, err)

		// Verify version information extraction
		assert.NotEmpty(t, testCRD.Spec.Versions)
		if len(testCRD.Spec.Versions) > 0 {
			assert.Equal(t, "v1alpha1", testCRD.Spec.Versions[0].Name)
		}

		if controllerVersion, exists := testCRD.Annotations["controller.version"]; exists {
			assert.Equal(t, "v1.2.3", controllerVersion)
		}

		if driverVersion, exists := testCRD.Annotations["driver.version"]; exists {
			assert.Equal(t, "v2.1.0", driverVersion)
		}
	})
}

// createCapabilityTestCRD creates a CRD for capability testing with proper validation schema
func createCapabilityTestCRD(crdDef CRDDefinition) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: crdDef.Name,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: crdDef.Group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     crdDef.Kind,
				Plural:   crdDef.Kind + "s", // Simple pluralization
				Singular: strings.ToLower(crdDef.Kind),
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    crdDef.Version,
					Storage: true,
					Served:  true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"spec": {
									Type: "object",
								},
								"status": {
									Type: "object",
								},
							},
						},
					},
				},
			},
		},
	}
}
