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
	"strings"
)

// Engine implements the Translator interface with static mapping tables
type Engine struct {
	// No state needed for static translation engine
}

// NewEngine creates a new translation engine
func NewEngine() *Engine {
	return &Engine{}
}

// TranslateStateToBackend translates unified state to backend-specific state
func (e *Engine) TranslateStateToBackend(backend Backend, unifiedState string) (string, error) {
	stateMap, err := GetStateMap(backend)
	if err != nil {
		return "", err
	}

	backendState, exists := stateMap.ToBackend(unifiedState)
	if !exists {
		return "", NewTranslationError(ErrorTypeInvalidValue, backend, "state", unifiedState,
			"unified state not supported by backend")
	}

	return backendState, nil
}

// TranslateStateFromBackend translates backend-specific state to unified state
func (e *Engine) TranslateStateFromBackend(backend Backend, backendState string) (string, error) {
	stateMap, err := GetStateMap(backend)
	if err != nil {
		return "", err
	}

	unifiedState, exists := stateMap.FromBackend(backendState)
	if !exists {
		return "", NewTranslationError(ErrorTypeInvalidValue, backend, "state", backendState,
			"backend state not recognized")
	}

	return unifiedState, nil
}

// TranslateModeToBackend translates unified mode to backend-specific mode
func (e *Engine) TranslateModeToBackend(backend Backend, unifiedMode string) (string, error) {
	modeMap, err := GetModeMap(backend)
	if err != nil {
		return "", err
	}

	backendMode, exists := modeMap.ToBackend(unifiedMode)
	if !exists {
		return "", NewTranslationError(ErrorTypeInvalidValue, backend, "mode", unifiedMode,
			"unified mode not supported by backend")
	}

	return backendMode, nil
}

// TranslateModeFromBackend translates backend-specific mode to unified mode
func (e *Engine) TranslateModeFromBackend(backend Backend, backendMode string) (string, error) {
	modeMap, err := GetModeMap(backend)
	if err != nil {
		return "", err
	}

	unifiedMode, exists := modeMap.FromBackend(backendMode)
	if !exists {
		return "", NewTranslationError(ErrorTypeInvalidValue, backend, "mode", backendMode,
			"backend mode not recognized")
	}

	return unifiedMode, nil
}

// ValidateTranslation validates that a translation is bidirectionally consistent
func (e *Engine) ValidateTranslation(backend Backend) error {
	// Validate state map
	stateMap, err := GetStateMap(backend)
	if err != nil {
		return err
	}

	if err := stateMap.Validate(); err != nil {
		return NewTranslationErrorWithCause(ErrorTypeInconsistentMapping, backend, "state", "",
			"state mapping validation failed", err)
	}

	// Validate mode map
	modeMap, err := GetModeMap(backend)
	if err != nil {
		return err
	}

	if err := modeMap.Validate(); err != nil {
		return NewTranslationErrorWithCause(ErrorTypeInconsistentMapping, backend, "mode", "",
			"mode mapping validation failed", err)
	}

	return nil
}

// GetSupportedStates returns all supported states for a backend
func (e *Engine) GetSupportedStates(backend Backend) ([]string, error) {
	stateMap, err := GetStateMap(backend)
	if err != nil {
		return nil, err
	}

	return stateMap.GetUnifiedValues(), nil
}

// GetSupportedModes returns all supported modes for a backend
func (e *Engine) GetSupportedModes(backend Backend) ([]string, error) {
	modeMap, err := GetModeMap(backend)
	if err != nil {
		return nil, err
	}

	return modeMap.GetUnifiedValues(), nil
}

// Backend-specific translation functions for convenience

// TranslateUnifiedToCeph translates unified state and mode to Ceph-specific values
func (e *Engine) TranslateUnifiedToCeph(state, mode string) (cephState, cephMode string, err error) {
	cephState, err = e.TranslateStateToBackend(BackendCeph, state)
	if err != nil {
		return "", "", err
	}

	cephMode, err = e.TranslateModeToBackend(BackendCeph, mode)
	if err != nil {
		return "", "", err
	}

	return cephState, cephMode, nil
}

// TranslateCephToUnified translates Ceph-specific state and mode to unified values
func (e *Engine) TranslateCephToUnified(cephState, cephMode string) (state, mode string, err error) {
	state, err = e.TranslateStateFromBackend(BackendCeph, cephState)
	if err != nil {
		return "", "", err
	}

	mode, err = e.TranslateModeFromBackend(BackendCeph, cephMode)
	if err != nil {
		return "", "", err
	}

	return state, mode, nil
}

// TranslateUnifiedToTrident translates unified state and mode to Trident-specific values
func (e *Engine) TranslateUnifiedToTrident(state, mode string) (tridentState, tridentMode string, err error) {
	tridentState, err = e.TranslateStateToBackend(BackendTrident, state)
	if err != nil {
		return "", "", err
	}

	tridentMode, err = e.TranslateModeToBackend(BackendTrident, mode)
	if err != nil {
		return "", "", err
	}

	return tridentState, tridentMode, nil
}

// TranslateTridentToUnified translates Trident-specific state and mode to unified values
func (e *Engine) TranslateTridentToUnified(tridentState, tridentMode string) (state, mode string, err error) {
	state, err = e.TranslateStateFromBackend(BackendTrident, tridentState)
	if err != nil {
		return "", "", err
	}

	mode, err = e.TranslateModeFromBackend(BackendTrident, tridentMode)
	if err != nil {
		return "", "", err
	}

	return state, mode, nil
}

// TranslateUnifiedToPowerStore translates unified state and mode to PowerStore-specific values
func (e *Engine) TranslateUnifiedToPowerStore(state, mode string) (powerstoreState, powerstoreMode string, err error) {
	powerstoreState, err = e.TranslateStateToBackend(BackendPowerStore, state)
	if err != nil {
		return "", "", err
	}

	powerstoreMode, err = e.TranslateModeToBackend(BackendPowerStore, mode)
	if err != nil {
		return "", "", err
	}

	return powerstoreState, powerstoreMode, nil
}

// TranslatePowerStoreToUnified translates PowerStore-specific state and mode to unified values
func (e *Engine) TranslatePowerStoreToUnified(powerstoreState, powerstoreMode string) (state, mode string, err error) {
	state, err = e.TranslateStateFromBackend(BackendPowerStore, powerstoreState)
	if err != nil {
		return "", "", err
	}

	mode, err = e.TranslateModeFromBackend(BackendPowerStore, powerstoreMode)
	if err != nil {
		return "", "", err
	}

	return state, mode, nil
}

// ValidateAllTranslations validates all backend translations for consistency
func (e *Engine) ValidateAllTranslations() error {
	for _, backend := range GetSupportedBackends() {
		if err := e.ValidateTranslation(backend); err != nil {
			return err
		}
	}
	return nil
}

// GetBackendInfo returns information about supported states and modes for a backend
func (e *Engine) GetBackendInfo(backend Backend) (BackendInfo, error) {
	states, err := e.GetSupportedStates(backend)
	if err != nil {
		return BackendInfo{}, err
	}

	modes, err := e.GetSupportedModes(backend)
	if err != nil {
		return BackendInfo{}, err
	}

	stateMap, _ := GetStateMap(backend)
	modeMap, _ := GetModeMap(backend)

	return BackendInfo{
		Backend:         backend,
		SupportedStates: states,
		SupportedModes:  modes,
		BackendStates:   stateMap.GetBackendValues(),
		BackendModes:    modeMap.GetBackendValues(),
	}, nil
}

// BackendInfo provides information about a backend's capabilities
type BackendInfo struct {
	Backend         Backend  `json:"backend"`
	SupportedStates []string `json:"supported_states"`
	SupportedModes  []string `json:"supported_modes"`
	BackendStates   []string `json:"backend_states"`
	BackendModes    []string `json:"backend_modes"`
}

// String returns a human-readable representation of the backend info
func (bi BackendInfo) String() string {
	var sb strings.Builder
	sb.WriteString(string(bi.Backend))
	sb.WriteString(" backend supports:\n")
	sb.WriteString("  States: ")
	sb.WriteString(strings.Join(bi.SupportedStates, ", "))
	sb.WriteString(" -> ")
	sb.WriteString(strings.Join(bi.BackendStates, ", "))
	sb.WriteString("\n  Modes: ")
	sb.WriteString(strings.Join(bi.SupportedModes, ", "))
	sb.WriteString(" -> ")
	sb.WriteString(strings.Join(bi.BackendModes, ", "))
	return sb.String()
}

// DefaultEngine provides a default translation engine instance
var DefaultEngine = NewEngine()
