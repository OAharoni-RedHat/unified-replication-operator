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
)

func TestTranslationMap_Basic(t *testing.T) {
	// Create a simple mapping
	unified2Backend := map[string]string{
		"source":  "primary",
		"replica": "secondary",
	}

	tm := NewTranslationMap(unified2Backend)

	t.Run("forward translation", func(t *testing.T) {
		backend, exists := tm.ToBackend("source")
		assert.True(t, exists)
		assert.Equal(t, "primary", backend)

		backend, exists = tm.ToBackend("replica")
		assert.True(t, exists)
		assert.Equal(t, "secondary", backend)

		_, exists = tm.ToBackend("invalid")
		assert.False(t, exists)
	})

	t.Run("reverse translation", func(t *testing.T) {
		unified, exists := tm.FromBackend("primary")
		assert.True(t, exists)
		assert.Equal(t, "source", unified)

		unified, exists = tm.FromBackend("secondary")
		assert.True(t, exists)
		assert.Equal(t, "replica", unified)

		_, exists = tm.FromBackend("invalid")
		assert.False(t, exists)
	})

	t.Run("get values", func(t *testing.T) {
		unifiedValues := tm.GetUnifiedValues()
		assert.Len(t, unifiedValues, 2)
		assert.Contains(t, unifiedValues, "source")
		assert.Contains(t, unifiedValues, "replica")

		backendValues := tm.GetBackendValues()
		assert.Len(t, backendValues, 2)
		assert.Contains(t, backendValues, "primary")
		assert.Contains(t, backendValues, "secondary")
	})

	t.Run("validation success", func(t *testing.T) {
		err := tm.Validate()
		assert.NoError(t, err)
	})
}

func TestTranslationMap_ValidationFailures(t *testing.T) {
	t.Run("inconsistent mapping", func(t *testing.T) {
		tm := &TranslationMap{
			UnifiedToBackend: map[string]string{
				"source": "primary",
			},
			BackendToUnified: map[string]string{
				"primary": "different", // Inconsistent!
			},
		}

		err := tm.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "inconsistent mapping")
	})

	t.Run("missing reverse mapping", func(t *testing.T) {
		tm := &TranslationMap{
			UnifiedToBackend: map[string]string{
				"source": "primary",
			},
			BackendToUnified: map[string]string{}, // Empty reverse map
		}

		err := tm.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing in reverse mapping")
	})

	t.Run("extra reverse mapping", func(t *testing.T) {
		tm := &TranslationMap{
			UnifiedToBackend: map[string]string{}, // Empty forward map
			BackendToUnified: map[string]string{
				"primary": "source",
			},
		}

		err := tm.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing in forward mapping")
	})
}

func TestEngine_StateTranslation(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name         string
		backend      Backend
		unifiedState string
		backendState string
		shouldError  bool
	}{
		// Ceph tests
		{"ceph source", BackendCeph, "source", "primary", false},
		{"ceph replica", BackendCeph, "replica", "secondary", false},
		{"ceph syncing", BackendCeph, "syncing", "resync", false},
		{"ceph promoting", BackendCeph, "promoting", "resync-promote", false},
		{"ceph demoting", BackendCeph, "demoting", "resync-demote", false},
		{"ceph failed", BackendCeph, "failed", "error", false},

		// Trident tests
		{"trident source", BackendTrident, "source", "established", false},
		{"trident replica", BackendTrident, "replica", "established-replica", false},
		{"trident promoting", BackendTrident, "promoting", "promoted", false},
		{"trident demoting", BackendTrident, "demoting", "reestablished", false},
		{"trident syncing", BackendTrident, "syncing", "establishing", false},
		{"trident failed", BackendTrident, "failed", "error", false},

		// PowerStore tests
		{"powerstore source", BackendPowerStore, "source", "source", false},
		{"powerstore replica", BackendPowerStore, "replica", "destination", false},
		{"powerstore promoting", BackendPowerStore, "promoting", "promoting", false},
		{"powerstore demoting", BackendPowerStore, "demoting", "demoting", false},
		{"powerstore syncing", BackendPowerStore, "syncing", "syncing", false},
		{"powerstore failed", BackendPowerStore, "failed", "failed", false},

		// Error cases
		{"invalid backend", "invalid", "source", "", true},
		{"invalid state", BackendCeph, "invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test forward translation
			result, err := engine.TranslateStateToBackend(tt.backend, tt.unifiedState)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.backendState, result)

				// Test reverse translation
				reverseResult, err := engine.TranslateStateFromBackend(tt.backend, tt.backendState)
				assert.NoError(t, err)
				assert.Equal(t, tt.unifiedState, reverseResult)
			}
		})
	}
}

func TestEngine_ModeTranslation(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name        string
		backend     Backend
		unifiedMode string
		backendMode string
		shouldError bool
	}{
		// Ceph tests
		{"ceph synchronous", BackendCeph, "synchronous", "sync", false},
		{"ceph asynchronous", BackendCeph, "asynchronous", "async", false},
		{"ceph eventual", BackendCeph, "eventual", "async-eventual", false},

		// Trident tests
		{"trident synchronous", BackendTrident, "synchronous", "Sync", false},
		{"trident asynchronous", BackendTrident, "asynchronous", "Async", false},
		{"trident eventual", BackendTrident, "eventual", "AsyncEventual", false},

		// PowerStore tests
		{"powerstore synchronous", BackendPowerStore, "synchronous", "SYNC", false},
		{"powerstore asynchronous", BackendPowerStore, "asynchronous", "ASYNC", false},
		{"powerstore eventual", BackendPowerStore, "eventual", "ASYNC_EVENTUAL", false},

		// Error cases
		{"invalid backend", "invalid", "synchronous", "", true},
		{"invalid mode", BackendCeph, "invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test forward translation
			result, err := engine.TranslateModeToBackend(tt.backend, tt.unifiedMode)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.backendMode, result)

				// Test reverse translation
				reverseResult, err := engine.TranslateModeFromBackend(tt.backend, tt.backendMode)
				assert.NoError(t, err)
				assert.Equal(t, tt.unifiedMode, reverseResult)
			}
		})
	}
}

func TestEngine_BackendSpecificFunctions(t *testing.T) {
	engine := NewEngine()

	t.Run("ceph functions", func(t *testing.T) {
		cephState, cephMode, err := engine.TranslateUnifiedToCeph("source", "asynchronous")
		assert.NoError(t, err)
		assert.Equal(t, "primary", cephState)
		assert.Equal(t, "async", cephMode)

		state, mode, err := engine.TranslateCephToUnified("primary", "async")
		assert.NoError(t, err)
		assert.Equal(t, "source", state)
		assert.Equal(t, "asynchronous", mode)
	})

	t.Run("trident functions", func(t *testing.T) {
		tridentState, tridentMode, err := engine.TranslateUnifiedToTrident("replica", "synchronous")
		assert.NoError(t, err)
		assert.Equal(t, "established-replica", tridentState)
		assert.Equal(t, "Sync", tridentMode)

		state, mode, err := engine.TranslateTridentToUnified("established-replica", "Sync")
		assert.NoError(t, err)
		assert.Equal(t, "replica", state)
		assert.Equal(t, "synchronous", mode)
	})

	t.Run("powerstore functions", func(t *testing.T) {
		powerstoreState, powerstoreMode, err := engine.TranslateUnifiedToPowerStore("replica", "eventual")
		assert.NoError(t, err)
		assert.Equal(t, "destination", powerstoreState)
		assert.Equal(t, "ASYNC_EVENTUAL", powerstoreMode)

		state, mode, err := engine.TranslatePowerStoreToUnified("destination", "ASYNC_EVENTUAL")
		assert.NoError(t, err)
		assert.Equal(t, "replica", state)
		assert.Equal(t, "eventual", mode)
	})
}

func TestEngine_ErrorHandling(t *testing.T) {
	engine := NewEngine()

	t.Run("invalid state error", func(t *testing.T) {
		_, err := engine.TranslateStateToBackend(BackendCeph, "invalid-state")
		assert.Error(t, err)

		translationErr, ok := err.(*TranslationError)
		assert.True(t, ok)
		assert.Equal(t, ErrorTypeInvalidValue, translationErr.Type)
		assert.Equal(t, BackendCeph, translationErr.Backend)
		assert.Equal(t, "state", translationErr.Field)
		assert.Equal(t, "invalid-state", translationErr.Value)
	})

	t.Run("invalid mode error", func(t *testing.T) {
		_, err := engine.TranslateModeToBackend(BackendTrident, "invalid-mode")
		assert.Error(t, err)

		translationErr, ok := err.(*TranslationError)
		assert.True(t, ok)
		assert.Equal(t, ErrorTypeInvalidValue, translationErr.Type)
		assert.Equal(t, BackendTrident, translationErr.Backend)
		assert.Equal(t, "mode", translationErr.Field)
		assert.Equal(t, "invalid-mode", translationErr.Value)
	})

	t.Run("unsupported backend error", func(t *testing.T) {
		_, err := engine.TranslateStateToBackend("unsupported", "source")
		assert.Error(t, err)

		translationErr, ok := err.(*TranslationError)
		assert.True(t, ok)
		assert.Equal(t, ErrorTypeUnsupportedMapping, translationErr.Type)
	})
}

func TestEngine_SupportedValues(t *testing.T) {
	engine := NewEngine()

	t.Run("supported states", func(t *testing.T) {
		states, err := engine.GetSupportedStates(BackendCeph)
		assert.NoError(t, err)
		assert.Contains(t, states, "source")
		assert.Contains(t, states, "replica")
		assert.Contains(t, states, "syncing")
		assert.Contains(t, states, "promoting")
		assert.Contains(t, states, "demoting")
		assert.Contains(t, states, "failed")

		_, err = engine.GetSupportedStates("invalid")
		assert.Error(t, err)
	})

	t.Run("supported modes", func(t *testing.T) {
		modes, err := engine.GetSupportedModes(BackendPowerStore)
		assert.NoError(t, err)
		assert.Contains(t, modes, "synchronous")
		assert.Contains(t, modes, "asynchronous")
		assert.Contains(t, modes, "eventual")

		_, err = engine.GetSupportedModes("invalid")
		assert.Error(t, err)
	})
}

func TestEngine_BackendInfo(t *testing.T) {
	engine := NewEngine()

	info, err := engine.GetBackendInfo(BackendCeph)
	assert.NoError(t, err)
	assert.Equal(t, BackendCeph, info.Backend)
	assert.NotEmpty(t, info.SupportedStates)
	assert.NotEmpty(t, info.SupportedModes)
	assert.NotEmpty(t, info.BackendStates)
	assert.NotEmpty(t, info.BackendModes)

	// Test string representation
	infoStr := info.String()
	assert.Contains(t, infoStr, "ceph")
	assert.Contains(t, infoStr, "States:")
	assert.Contains(t, infoStr, "Modes:")
}

func TestEngine_ValidationIntegration(t *testing.T) {
	engine := NewEngine()

	t.Run("validate all translations", func(t *testing.T) {
		err := engine.ValidateAllTranslations()
		assert.NoError(t, err)
	})

	t.Run("validate specific backend", func(t *testing.T) {
		for _, backend := range GetSupportedBackends() {
			err := engine.ValidateTranslation(backend)
			assert.NoError(t, err, "validation failed for backend %s", backend)
		}
	})
}

func TestTranslationError(t *testing.T) {
	t.Run("basic error", func(t *testing.T) {
		err := NewTranslationError(ErrorTypeInvalidValue, BackendCeph, "state", "invalid", "test message")

		assert.True(t, IsTranslationError(err))
		assert.Contains(t, err.Error(), "invalid_value")
		assert.Contains(t, err.Error(), "ceph")
		assert.Contains(t, err.Error(), "state")
		assert.Contains(t, err.Error(), "invalid")
		assert.Contains(t, err.Error(), "test message")

		extractedErr, ok := GetTranslationError(err)
		assert.True(t, ok)
		assert.Equal(t, ErrorTypeInvalidValue, extractedErr.Type)
		assert.Equal(t, BackendCeph, extractedErr.Backend)
	})

	t.Run("error with cause", func(t *testing.T) {
		originalErr := assert.AnError
		err := NewTranslationErrorWithCause(ErrorTypeInconsistentMapping, BackendTrident, "mode", "sync", "test message", originalErr)

		assert.Contains(t, err.Error(), "caused by:")
		assert.Equal(t, originalErr, err.Unwrap())
	})

	t.Run("error type checking", func(t *testing.T) {
		regularErr := assert.AnError
		assert.False(t, IsTranslationError(regularErr))

		_, ok := GetTranslationError(regularErr)
		assert.False(t, ok)
	})
}

func TestMaps_DirectAccess(t *testing.T) {
	t.Run("supported backends", func(t *testing.T) {
		backends := GetSupportedBackends()
		assert.Contains(t, backends, BackendCeph)
		assert.Contains(t, backends, BackendTrident)
		assert.Contains(t, backends, BackendPowerStore)
	})

	t.Run("backend support check", func(t *testing.T) {
		assert.True(t, IsBackendSupported(BackendCeph))
		assert.True(t, IsBackendSupported(BackendTrident))
		assert.True(t, IsBackendSupported(BackendPowerStore))
		assert.False(t, IsBackendSupported("invalid"))
	})

	t.Run("map retrieval", func(t *testing.T) {
		stateMap, err := GetStateMap(BackendCeph)
		assert.NoError(t, err)
		assert.NotNil(t, stateMap)

		modeMap, err := GetModeMap(BackendCeph)
		assert.NoError(t, err)
		assert.NotNil(t, modeMap)

		_, err = GetStateMap("invalid")
		assert.Error(t, err)

		_, err = GetModeMap("invalid")
		assert.Error(t, err)
	})
}

func TestDefaultEngine(t *testing.T) {
	// Test that default engine is properly initialized
	assert.NotNil(t, DefaultEngine)

	// Test basic functionality with default engine
	result, err := DefaultEngine.TranslateStateToBackend(BackendCeph, "source")
	assert.NoError(t, err)
	assert.Equal(t, "primary", result)
}
