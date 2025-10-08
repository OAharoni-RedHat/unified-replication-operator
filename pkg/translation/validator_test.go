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

package translation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_AllMappings(t *testing.T) {
	validator := NewValidator()

	t.Run("validate all mappings success", func(t *testing.T) {
		err := validator.ValidateAllMappings()
		assert.NoError(t, err)
	})

	t.Run("validate specific backend mappings", func(t *testing.T) {
		for _, backend := range GetSupportedBackends() {
			err := validator.ValidateBackendMappings(backend)
			assert.NoError(t, err, "validation failed for backend %s", backend)
		}
	})
}

func TestValidator_StateMapping(t *testing.T) {
	validator := NewValidator()

	t.Run("state mapping consistency", func(t *testing.T) {
		for _, backend := range GetSupportedBackends() {
			err := validator.ValidateStateMappingConsistency(backend)
			assert.NoError(t, err, "state mapping inconsistent for backend %s", backend)
		}
	})

	t.Run("invalid backend", func(t *testing.T) {
		err := validator.ValidateStateMappingConsistency("invalid")
		assert.Error(t, err)
	})
}

func TestValidator_ModeMapping(t *testing.T) {
	validator := NewValidator()

	t.Run("mode mapping consistency", func(t *testing.T) {
		for _, backend := range GetSupportedBackends() {
			err := validator.ValidateModeMappingConsistency(backend)
			assert.NoError(t, err, "mode mapping inconsistent for backend %s", backend)
		}
	})

	t.Run("invalid backend", func(t *testing.T) {
		err := validator.ValidateModeMappingConsistency("invalid")
		assert.Error(t, err)
	})
}

func TestValidator_RoundTrip(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name         string
		backend      Backend
		unifiedState string
		unifiedMode  string
	}{
		{"ceph source async", BackendCeph, "source", "asynchronous"},
		{"ceph replica sync", BackendCeph, "replica", "synchronous"},
		{"trident promoting eventual", BackendTrident, "promoting", "eventual"},
		{"powerstore demoting sync", BackendPowerStore, "demoting", "synchronous"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTranslationRoundTrip(tt.backend, tt.unifiedState, tt.unifiedMode)
			assert.NoError(t, err)
		})
	}

	t.Run("invalid state round trip", func(t *testing.T) {
		err := validator.ValidateTranslationRoundTrip(BackendCeph, "invalid", "asynchronous")
		assert.Error(t, err)
	})

	t.Run("invalid mode round trip", func(t *testing.T) {
		err := validator.ValidateTranslationRoundTrip(BackendCeph, "source", "invalid")
		assert.Error(t, err)
	})
}

func TestValidator_MappingCoverage(t *testing.T) {
	validator := NewValidator()

	t.Run("complete coverage", func(t *testing.T) {
		expectedStates := []string{"source", "replica", "syncing", "promoting", "demoting", "failed"}
		expectedModes := []string{"synchronous", "asynchronous", "eventual"}

		for _, backend := range GetSupportedBackends() {
			err := validator.ValidateMappingCoverage(backend, expectedStates, expectedModes)
			assert.NoError(t, err, "coverage validation failed for backend %s", backend)
		}
	})

	t.Run("missing state coverage", func(t *testing.T) {
		expectedStates := []string{"source", "replica", "missing-state"}
		expectedModes := []string{"synchronous"}

		err := validator.ValidateMappingCoverage(BackendCeph, expectedStates, expectedModes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing-state")
	})

	t.Run("missing mode coverage", func(t *testing.T) {
		expectedStates := []string{"source"}
		expectedModes := []string{"synchronous", "missing-mode"}

		err := validator.ValidateMappingCoverage(BackendCeph, expectedStates, expectedModes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing-mode")
	})

	t.Run("invalid backend coverage", func(t *testing.T) {
		err := validator.ValidateMappingCoverage("invalid", []string{"source"}, []string{"sync"})
		assert.Error(t, err)
	})
}

func TestValidator_Statistics(t *testing.T) {
	validator := NewValidator()

	stats, err := validator.GetMappingStatistics()
	require.NoError(t, err)

	t.Run("basic statistics", func(t *testing.T) {
		assert.Equal(t, 3, stats.TotalBackends) // Ceph, Trident, PowerStore
		assert.Greater(t, stats.TotalStateMappings, 0)
		assert.Greater(t, stats.TotalModeMappings, 0)

		// Check that we have stats for all backends
		assert.Contains(t, stats.BackendStats, BackendCeph)
		assert.Contains(t, stats.BackendStats, BackendTrident)
		assert.Contains(t, stats.BackendStats, BackendPowerStore)
	})

	t.Run("backend statistics", func(t *testing.T) {
		cephStats := stats.BackendStats[BackendCeph]
		assert.Equal(t, BackendCeph, cephStats.Backend)
		assert.Equal(t, 6, cephStats.StateMapSize) // 6 unified states
		assert.Equal(t, 3, cephStats.ModeMapSize)  // 3 unified modes
		assert.Greater(t, cephStats.SupportedStates, 0)
		assert.Greater(t, cephStats.SupportedModes, 0)
		assert.Greater(t, cephStats.BackendStates, 0)
		assert.Greater(t, cephStats.BackendModes, 0)
	})
}

func TestDefaultValidator(t *testing.T) {
	// Test that default validator is properly initialized
	assert.NotNil(t, DefaultValidator)

	// Test basic functionality with default validator
	err := DefaultValidator.ValidateAllMappings()
	assert.NoError(t, err)
}
