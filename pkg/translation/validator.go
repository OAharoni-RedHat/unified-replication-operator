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
	"fmt"
)

// Validator provides validation utilities for translation consistency
type Validator struct {
	engine *Engine
}

// NewValidator creates a new translation validator
func NewValidator() *Validator {
	return &Validator{
		engine: NewEngine(),
	}
}

// ValidateAllMappings validates all translation mappings for consistency
func (v *Validator) ValidateAllMappings() error {
	// Validate each backend's mappings
	for _, backend := range GetSupportedBackends() {
		if err := v.ValidateBackendMappings(backend); err != nil {
			return fmt.Errorf("validation failed for backend %s: %w", backend, err)
		}
	}
	return nil
}

// ValidateBackendMappings validates all mappings for a specific backend
func (v *Validator) ValidateBackendMappings(backend Backend) error {
	// Validate state mapping
	if err := v.ValidateStateMappingConsistency(backend); err != nil {
		return fmt.Errorf("state mapping validation failed: %w", err)
	}

	// Validate mode mapping
	if err := v.ValidateModeMappingConsistency(backend); err != nil {
		return fmt.Errorf("mode mapping validation failed: %w", err)
	}

	return nil
}

// ValidateStateMappingConsistency validates bidirectional consistency of state mappings
func (v *Validator) ValidateStateMappingConsistency(backend Backend) error {
	stateMap, err := GetStateMap(backend)
	if err != nil {
		return err
	}

	// Test bidirectional consistency for each mapping
	for unifiedState := range stateMap.UnifiedToBackend {
		// Forward translation
		backendState, err := v.engine.TranslateStateToBackend(backend, unifiedState)
		if err != nil {
			return fmt.Errorf("forward state translation failed for %s->%s: %w",
				unifiedState, backend, err)
		}

		// Reverse translation
		reverseUnified, err := v.engine.TranslateStateFromBackend(backend, backendState)
		if err != nil {
			return fmt.Errorf("reverse state translation failed for %s->%s: %w",
				backendState, backend, err)
		}

		// Check consistency
		if reverseUnified != unifiedState {
			return NewTranslationError(ErrorTypeInconsistentMapping, backend, "state", unifiedState,
				fmt.Sprintf("bidirectional translation inconsistent: %s->%s->%s",
					unifiedState, backendState, reverseUnified))
		}
	}

	return nil
}

// ValidateModeMappingConsistency validates bidirectional consistency of mode mappings
func (v *Validator) ValidateModeMappingConsistency(backend Backend) error {
	modeMap, err := GetModeMap(backend)
	if err != nil {
		return err
	}

	// Test bidirectional consistency for each mapping
	for unifiedMode := range modeMap.UnifiedToBackend {
		// Forward translation
		backendMode, err := v.engine.TranslateModeToBackend(backend, unifiedMode)
		if err != nil {
			return fmt.Errorf("forward mode translation failed for %s->%s: %w",
				unifiedMode, backend, err)
		}

		// Reverse translation
		reverseUnified, err := v.engine.TranslateModeFromBackend(backend, backendMode)
		if err != nil {
			return fmt.Errorf("reverse mode translation failed for %s->%s: %w",
				backendMode, backend, err)
		}

		// Check consistency
		if reverseUnified != unifiedMode {
			return NewTranslationError(ErrorTypeInconsistentMapping, backend, "mode", unifiedMode,
				fmt.Sprintf("bidirectional translation inconsistent: %s->%s->%s",
					unifiedMode, backendMode, reverseUnified))
		}
	}

	return nil
}

// ValidateTranslationRoundTrip validates a complete round-trip translation
func (v *Validator) ValidateTranslationRoundTrip(backend Backend, unifiedState, unifiedMode string) error {
	// Forward translation
	backendState, err := v.engine.TranslateStateToBackend(backend, unifiedState)
	if err != nil {
		return fmt.Errorf("forward state translation failed: %w", err)
	}

	backendMode, err := v.engine.TranslateModeToBackend(backend, unifiedMode)
	if err != nil {
		return fmt.Errorf("forward mode translation failed: %w", err)
	}

	// Reverse translation
	reverseState, err := v.engine.TranslateStateFromBackend(backend, backendState)
	if err != nil {
		return fmt.Errorf("reverse state translation failed: %w", err)
	}

	reverseMode, err := v.engine.TranslateModeFromBackend(backend, backendMode)
	if err != nil {
		return fmt.Errorf("reverse mode translation failed: %w", err)
	}

	// Validate consistency
	if reverseState != unifiedState {
		return NewTranslationError(ErrorTypeInconsistentMapping, backend, "state", unifiedState,
			fmt.Sprintf("round-trip state translation inconsistent: %s->%s->%s",
				unifiedState, backendState, reverseState))
	}

	if reverseMode != unifiedMode {
		return NewTranslationError(ErrorTypeInconsistentMapping, backend, "mode", unifiedMode,
			fmt.Sprintf("round-trip mode translation inconsistent: %s->%s->%s",
				unifiedMode, backendMode, reverseMode))
	}

	return nil
}

// ValidateMappingCoverage ensures all expected unified values have mappings
func (v *Validator) ValidateMappingCoverage(backend Backend, expectedStates, expectedModes []string) error {
	stateMap, err := GetStateMap(backend)
	if err != nil {
		return err
	}

	// Check state coverage
	for _, expectedState := range expectedStates {
		if _, exists := stateMap.UnifiedToBackend[expectedState]; !exists {
			return NewTranslationError(ErrorTypeMissingMapping, backend, "state", expectedState,
				"required state mapping is missing")
		}
	}

	modeMap, err := GetModeMap(backend)
	if err != nil {
		return err
	}

	// Check mode coverage
	for _, expectedMode := range expectedModes {
		if _, exists := modeMap.UnifiedToBackend[expectedMode]; !exists {
			return NewTranslationError(ErrorTypeMissingMapping, backend, "mode", expectedMode,
				"required mode mapping is missing")
		}
	}

	return nil
}

// GetMappingStatistics returns statistics about translation mappings
func (v *Validator) GetMappingStatistics() (MappingStatistics, error) {
	stats := MappingStatistics{
		BackendStats: make(map[Backend]BackendStatistics),
	}

	for _, backend := range GetSupportedBackends() {
		stateMap, err := GetStateMap(backend)
		if err != nil {
			return stats, err
		}

		modeMap, err := GetModeMap(backend)
		if err != nil {
			return stats, err
		}

		backendStats := BackendStatistics{
			Backend:         backend,
			StateMapSize:    len(stateMap.UnifiedToBackend),
			ModeMapSize:     len(modeMap.UnifiedToBackend),
			SupportedStates: len(stateMap.GetUnifiedValues()),
			SupportedModes:  len(modeMap.GetUnifiedValues()),
			BackendStates:   len(stateMap.GetBackendValues()),
			BackendModes:    len(modeMap.GetBackendValues()),
		}

		stats.BackendStats[backend] = backendStats
		stats.TotalBackends++
		stats.TotalStateMappings += backendStats.StateMapSize
		stats.TotalModeMappings += backendStats.ModeMapSize
	}

	return stats, nil
}

// MappingStatistics provides statistics about translation mappings
type MappingStatistics struct {
	TotalBackends      int                           `json:"total_backends"`
	TotalStateMappings int                           `json:"total_state_mappings"`
	TotalModeMappings  int                           `json:"total_mode_mappings"`
	BackendStats       map[Backend]BackendStatistics `json:"backend_stats"`
}

// BackendStatistics provides statistics for a specific backend
type BackendStatistics struct {
	Backend         Backend `json:"backend"`
	StateMapSize    int     `json:"state_map_size"`
	ModeMapSize     int     `json:"mode_map_size"`
	SupportedStates int     `json:"supported_states"`
	SupportedModes  int     `json:"supported_modes"`
	BackendStates   int     `json:"backend_states"`
	BackendModes    int     `json:"backend_modes"`
}

// DefaultValidator provides a default validator instance
var DefaultValidator = NewValidator()
