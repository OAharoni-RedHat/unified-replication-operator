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

	"github.com/unified-replication/operator/pkg/translation"
	"github.com/unified-replication/operator/test/utils"
)

// Integration tests using envtest
func TestDiscoveryIntegration(t *testing.T) {
	// Skip integration tests in short mode
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Create test environment
	testEnv := utils.NewTestEnvironment(t, utils.DefaultTestEnvironmentOptions())
	defer testEnv.Stop(t)

	ctx := context.Background()

	t.Run("DiscoverBackends with real CRDs", func(t *testing.T) {
		// Create discovery engine
		config := DefaultDiscoveryConfig()
		config.TimeoutPerBackend = 5 * time.Second
		engine := NewEngine(testEnv.Client, config)

		// Initially, no backends should be available (no CRDs installed)
		result, err := engine.DiscoverBackends(ctx)
		require.NoError(t, err)
		assert.Len(t, result.AvailableBackends, 0)

		// All backends should be unavailable
		for _, backend := range translation.GetSupportedBackends() {
			backendResult := result.Backends[backend]
			assert.Equal(t, BackendStatusUnavailable, backendResult.Status)

			// All CRDs should be unavailable
			for _, crdInfo := range backendResult.CRDs {
				assert.False(t, crdInfo.Available)
			}
		}
	})

	t.Run("DiscoverBackend after installing CRDs", func(t *testing.T) {
		// Create discovery engine
		engine := NewEngine(testEnv.Client, DefaultDiscoveryConfig())

		// Install Ceph CRDs
		cephCRDs, _ := GetRequiredCRDsForBackend(translation.BackendCeph)
		for _, crdDef := range cephCRDs {
			crd := createTestCRD(crdDef)
			err := testEnv.Client.Create(ctx, crd)
			require.NoError(t, err)

			// Wait for CRD to be established
			require.Eventually(t, func() bool {
				ready, err := engine.CheckCRDReady(ctx, crd.Name)
				return err == nil && ready
			}, 30*time.Second, 100*time.Millisecond)
		}

		// Wait for CRDs to be fully ready
		testEnv.WaitForCRDReady(t, cephCRDs[0].Name, 30*time.Second)

		// Discover Ceph backend - should now be available
		result, err := engine.DiscoverBackend(ctx, translation.BackendCeph)
		require.NoError(t, err)
		assert.Equal(t, BackendStatusAvailable, result.Status)

		// All Ceph CRDs should be available
		for _, crdInfo := range result.CRDs {
			assert.True(t, crdInfo.Available, "CRD %s should be available", crdInfo.Name)
		}

		// Other backends should still be unavailable
		tridentResult, err := engine.DiscoverBackend(ctx, translation.BackendTrident)
		require.NoError(t, err)
		assert.Equal(t, BackendStatusUnavailable, tridentResult.Status)
	})

	t.Run("Discovery caching behavior", func(t *testing.T) {
		// Create engine with short cache TTL
		config := DefaultDiscoveryConfig()
		config.CacheTTL = 1 * time.Second
		engine := NewEngine(testEnv.Client, config)

		// First discovery
		result1, err := engine.DiscoverBackends(ctx)
		require.NoError(t, err)

		// Get cached result immediately
		cachedResult, valid := engine.GetCachedResult()
		assert.True(t, valid)
		assert.Equal(t, len(result1.Backends), len(cachedResult.Backends))

		// Wait for cache to expire
		time.Sleep(2 * time.Second)

		// Cache should be expired
		_, valid = engine.GetCachedResult()
		assert.False(t, valid)

		// Refresh cache
		err = engine.RefreshCache(ctx)
		require.NoError(t, err)

		// Cache should be valid again
		_, valid = engine.GetCachedResult()
		assert.True(t, valid)
	})

	t.Run("Auto refresh functionality", func(t *testing.T) {
		// Create engine with auto refresh enabled
		config := DefaultDiscoveryConfig()
		config.EnableAutoRefresh = true
		config.RefreshInterval = 500 * time.Millisecond
		engine := NewEngine(testEnv.Client, config)

		// Start auto refresh
		err := engine.StartAutoRefresh(ctx)
		require.NoError(t, err)

		// Wait for a few refresh cycles
		time.Sleep(1500 * time.Millisecond)

		// Cache should have been refreshed
		cachedResult, valid := engine.GetCachedResult()
		assert.True(t, valid)
		assert.NotNil(t, cachedResult)

		// Stop auto refresh
		err = engine.StopAutoRefresh()
		require.NoError(t, err)

		// Starting again should fail
		err = engine.StartAutoRefresh(ctx)
		assert.Error(t, err)
	})

	t.Run("Permission validation", func(t *testing.T) {
		engine := NewEngine(testEnv.Client, DefaultDiscoveryConfig())

		// Should pass with test environment permissions
		err := engine.ValidateClientPermissions(ctx)
		assert.NoError(t, err)
	})

	t.Run("CRD lifecycle management", func(t *testing.T) {
		engine := NewEngine(testEnv.Client, DefaultDiscoveryConfig())

		// Create a test CRD
		testCRD := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testresources.test.discovery.io",
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "test.discovery.io",
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Kind:     "TestResource",
					Plural:   "testresources",
					Singular: "testresource",
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{
						Name:    "v1",
						Storage: true,
						Served:  true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {
										Type: "object",
									},
								},
							},
						},
					},
				},
			},
		}

		// CRD should not exist initially
		exists, err := engine.CheckCRDExists(ctx, testCRD.Name)
		require.NoError(t, err)
		assert.False(t, exists)

		// Create CRD
		err = testEnv.Client.Create(ctx, testCRD)
		require.NoError(t, err)

		// CRD should exist but may not be ready yet
		exists, err = engine.CheckCRDExists(ctx, testCRD.Name)
		require.NoError(t, err)
		assert.True(t, exists)

		// Wait for CRD to be ready
		require.Eventually(t, func() bool {
			ready, err := engine.CheckCRDReady(ctx, testCRD.Name)
			return err == nil && ready
		}, 30*time.Second, 100*time.Millisecond)

		// Get CRD info
		info, err := engine.GetCRDInfo(ctx, testCRD.Name)
		require.NoError(t, err)
		assert.Equal(t, testCRD.Name, info.Name)
		assert.Equal(t, "test.discovery.io", info.Group)
		assert.Equal(t, "v1", info.Version)
		assert.Equal(t, "TestResource", info.Kind)
		assert.True(t, info.Available)

		// List CRDs in the test group
		crdList, err := engine.ListCRDs(ctx, "test.discovery.io")
		require.NoError(t, err)
		assert.Len(t, crdList, 1)
		assert.Equal(t, testCRD.Name, crdList[0].Name)

		// Clean up
		err = testEnv.Client.Delete(ctx, testCRD)
		require.NoError(t, err)
	})

	t.Run("Backend detector validation", func(t *testing.T) {
		// Test each detector individually
		detectors := map[translation.Backend]BackendDetector{
			translation.BackendCeph:       NewCephDetector(testEnv.Client),
			translation.BackendTrident:    NewTridentDetector(testEnv.Client),
			translation.BackendPowerStore: NewPowerStoreDetector(testEnv.Client),
		}

		for backend, detector := range detectors {
			t.Run(string(backend), func(t *testing.T) {
				// Test detection (should be unavailable without CRDs)
				result, err := detector.DetectBackend(ctx)
				require.NoError(t, err)
				assert.Equal(t, backend, result.Backend)
				assert.Equal(t, BackendStatusUnavailable, result.Status)

				// Test validation (should fail without CRDs)
				err = detector.ValidateBackend(ctx)
				assert.Error(t, err)
				assert.True(t, IsDiscoveryError(err))

				// Test required CRDs
				requiredCRDs := detector.GetRequiredCRDs()
				assert.NotEmpty(t, requiredCRDs)

				// Test backend type
				assert.Equal(t, backend, detector.GetBackendType())
			})
		}
	})

	t.Run("Error handling scenarios", func(t *testing.T) {
		engine := NewEngine(testEnv.Client, DefaultDiscoveryConfig())

		// Test discovery with context timeout
		shortCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		cancel() // Cancel immediately

		_, err := engine.DiscoverBackends(shortCtx)
		assert.Error(t, err)

		// Test invalid backend discovery
		_, err = engine.DiscoverBackend(ctx, "invalid-backend")
		assert.Error(t, err)
		assert.True(t, IsDiscoveryError(err))
	})
}

// createTestCRD creates a CRD for testing with proper validation schema
func createTestCRD(crdDef CRDDefinition) *apiextensionsv1.CustomResourceDefinition {
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
