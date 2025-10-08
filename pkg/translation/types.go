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

// Backend represents the different storage backends
type Backend string

const (
	// BackendCeph represents Ceph-CSI with volume-replication-operator
	BackendCeph Backend = "ceph"
	// BackendTrident represents NetApp Trident
	BackendTrident Backend = "trident"
	// BackendPowerStore represents Dell PowerStore
	BackendPowerStore Backend = "powerstore"
)

// TranslationError represents various types of translation failures
type TranslationError struct {
	Type     ErrorType
	Backend  Backend
	Field    string
	Value    string
	Message  string
	Original error
}

// ErrorType defines the types of translation errors
type ErrorType string

const (
	// ErrorTypeInvalidValue indicates an invalid input value
	ErrorTypeInvalidValue ErrorType = "invalid_value"
	// ErrorTypeUnsupportedMapping indicates a mapping is not supported
	ErrorTypeUnsupportedMapping ErrorType = "unsupported_mapping"
	// ErrorTypeInconsistentMapping indicates bidirectional mapping inconsistency
	ErrorTypeInconsistentMapping ErrorType = "inconsistent_mapping"
	// ErrorTypeMissingMapping indicates a required mapping is missing
	ErrorTypeMissingMapping ErrorType = "missing_mapping"
)

// Error implements the error interface
func (e *TranslationError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("translation error (%s) for backend %s field %s='%s': %s (caused by: %v)",
			e.Type, e.Backend, e.Field, e.Value, e.Message, e.Original)
	}
	return fmt.Sprintf("translation error (%s) for backend %s field %s='%s': %s",
		e.Type, e.Backend, e.Field, e.Value, e.Message)
}

// Unwrap returns the original error for error wrapping support
func (e *TranslationError) Unwrap() error {
	return e.Original
}

// IsTranslationError checks if an error is a TranslationError
func IsTranslationError(err error) bool {
	_, ok := err.(*TranslationError)
	return ok
}

// GetTranslationError extracts a TranslationError from an error
func GetTranslationError(err error) (*TranslationError, bool) {
	te, ok := err.(*TranslationError)
	return te, ok
}

// NewTranslationError creates a new TranslationError
func NewTranslationError(errType ErrorType, backend Backend, field, value, message string) *TranslationError {
	return &TranslationError{
		Type:    errType,
		Backend: backend,
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// NewTranslationErrorWithCause creates a new TranslationError with an underlying cause
func NewTranslationErrorWithCause(errType ErrorType, backend Backend, field, value, message string, cause error) *TranslationError {
	return &TranslationError{
		Type:     errType,
		Backend:  backend,
		Field:    field,
		Value:    value,
		Message:  message,
		Original: cause,
	}
}

// Translator interface defines the contract for translation operations
type Translator interface {
	// TranslateStateToBackend translates unified state to backend-specific state
	TranslateStateToBackend(backend Backend, unifiedState string) (string, error)

	// TranslateStateFromBackend translates backend-specific state to unified state
	TranslateStateFromBackend(backend Backend, backendState string) (string, error)

	// TranslateModeToBackend translates unified mode to backend-specific mode
	TranslateModeToBackend(backend Backend, unifiedMode string) (string, error)

	// TranslateModeFromBackend translates backend-specific mode to unified mode
	TranslateModeFromBackend(backend Backend, backendMode string) (string, error)

	// ValidateTranslation validates that a translation is bidirectionally consistent
	ValidateTranslation(backend Backend) error

	// GetSupportedStates returns all supported states for a backend
	GetSupportedStates(backend Backend) ([]string, error)

	// GetSupportedModes returns all supported modes for a backend
	GetSupportedModes(backend Backend) ([]string, error)
}

// TranslationMap represents a bidirectional mapping between unified and backend values
type TranslationMap struct {
	UnifiedToBackend map[string]string
	BackendToUnified map[string]string
}

// NewTranslationMap creates a new TranslationMap from a unified-to-backend mapping
func NewTranslationMap(unifiedToBackend map[string]string) *TranslationMap {
	backendToUnified := make(map[string]string)
	for unified, backend := range unifiedToBackend {
		backendToUnified[backend] = unified
	}

	return &TranslationMap{
		UnifiedToBackend: unifiedToBackend,
		BackendToUnified: backendToUnified,
	}
}

// Validate checks that the translation map is bidirectionally consistent
func (tm *TranslationMap) Validate() error {
	// Check that forward and reverse mappings are consistent
	for unified, backend := range tm.UnifiedToBackend {
		if reversedUnified, exists := tm.BackendToUnified[backend]; !exists {
			return fmt.Errorf("backend value '%s' missing in reverse mapping", backend)
		} else if reversedUnified != unified {
			return fmt.Errorf("inconsistent mapping: unified '%s' -> backend '%s' -> unified '%s'",
				unified, backend, reversedUnified)
		}
	}

	// Check that reverse mappings don't have extra entries
	for _, unified := range tm.BackendToUnified {
		if _, exists := tm.UnifiedToBackend[unified]; !exists {
			return fmt.Errorf("unified value '%s' missing in forward mapping", unified)
		}
	}

	return nil
}

// ToBackend translates a unified value to backend value
func (tm *TranslationMap) ToBackend(unifiedValue string) (string, bool) {
	backendValue, exists := tm.UnifiedToBackend[unifiedValue]
	return backendValue, exists
}

// FromBackend translates a backend value to unified value
func (tm *TranslationMap) FromBackend(backendValue string) (string, bool) {
	unifiedValue, exists := tm.BackendToUnified[backendValue]
	return unifiedValue, exists
}

// GetUnifiedValues returns all unified values in the mapping
func (tm *TranslationMap) GetUnifiedValues() []string {
	values := make([]string, 0, len(tm.UnifiedToBackend))
	for unified := range tm.UnifiedToBackend {
		values = append(values, unified)
	}
	return values
}

// GetBackendValues returns all backend values in the mapping
func (tm *TranslationMap) GetBackendValues() []string {
	values := make([]string, 0, len(tm.BackendToUnified))
	for backend := range tm.BackendToUnified {
		values = append(values, backend)
	}
	return values
}
