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

// State translation maps based on CRD analysis

// CephStateMap defines the translation between unified and Ceph states
// Ceph uses volume-replication-operator CRDs with states: primary, secondary, resync
// Note: To maintain bidirectional consistency, we create extended backend-specific states
var CephStateMap = NewTranslationMap(map[string]string{
	"source":    "primary",        // Volume is primary and accepting writes
	"replica":   "secondary",      // Volume is secondary (read-only replica)
	"syncing":   "resync",         // Volume needs resynchronization
	"promoting": "resync-promote", // Promoting uses extended resync state
	"demoting":  "resync-demote",  // Demoting uses extended resync state
	"failed":    "error",          // Failed state maps to error condition
})

// TridentStateMap defines the translation between unified and Trident states
// Trident uses TridentMirrorRelationship with states: established, promoted, reestablished
// Note: We use extended states to maintain bidirectional consistency
var TridentStateMap = NewTranslationMap(map[string]string{
	"source":    "established",         // Source volume with established mirror relationship
	"replica":   "established-replica", // Extended state for replica distinction
	"promoting": "promoted",            // Volume being promoted from replica to source
	"demoting":  "reestablished",       // Volume being demoted, relationship reestablished
	"syncing":   "established-syncing", // Extended state for syncing
	"failed":    "established-failed",  // Extended state for failed
})

// PowerStoreStateMap defines the translation between unified and PowerStore states
// PowerStore uses DellCSIReplicationGroups with states: source, destination, promoting, demoting, syncing, failed
var PowerStoreStateMap = NewTranslationMap(map[string]string{
	"source":    "source",      // Volume is source in replication group
	"replica":   "destination", // Volume is destination in replication group
	"promoting": "promoting",   // Volume is being promoted
	"demoting":  "demoting",    // Volume is being demoted
	"syncing":   "syncing",     // Volume is synchronizing
	"failed":    "failed",      // Failed state remains as failed
})

// Mode translation maps based on CRD analysis

// CephModeMap defines the translation between unified and Ceph modes
// Ceph supports sync/async modes
var CephModeMap = NewTranslationMap(map[string]string{
	"synchronous":  "sync",  // Synchronous replication
	"asynchronous": "async", // Asynchronous replication
})

// TridentModeMap defines the translation between unified and Trident modes
// Trident uses capitalized mode names
var TridentModeMap = NewTranslationMap(map[string]string{
	"synchronous":  "Sync",  // Synchronous replication
	"asynchronous": "Async", // Asynchronous replication
})

// PowerStoreModeMap defines the translation between unified and PowerStore modes
// PowerStore supports SYNC and ASYNC modes
var PowerStoreModeMap = NewTranslationMap(map[string]string{
	"synchronous":  "SYNC",  // Synchronous replication
	"asynchronous": "ASYNC", // Asynchronous replication
})

// BackendStateMaps provides easy access to state maps by backend
var BackendStateMaps = map[Backend]*TranslationMap{
	BackendCeph:       CephStateMap,
	BackendTrident:    TridentStateMap,
	BackendPowerStore: PowerStoreStateMap,
}

// BackendModeMaps provides easy access to mode maps by backend
var BackendModeMaps = map[Backend]*TranslationMap{
	BackendCeph:       CephModeMap,
	BackendTrident:    TridentModeMap,
	BackendPowerStore: PowerStoreModeMap,
}

// GetStateMap returns the state translation map for a backend
func GetStateMap(backend Backend) (*TranslationMap, error) {
	stateMap, exists := BackendStateMaps[backend]
	if !exists {
		return nil, NewTranslationError(ErrorTypeUnsupportedMapping, backend, "backend", string(backend),
			"backend not supported for state translation")
	}
	return stateMap, nil
}

// GetModeMap returns the mode translation map for a backend
func GetModeMap(backend Backend) (*TranslationMap, error) {
	modeMap, exists := BackendModeMaps[backend]
	if !exists {
		return nil, NewTranslationError(ErrorTypeUnsupportedMapping, backend, "backend", string(backend),
			"backend not supported for mode translation")
	}
	return modeMap, nil
}

// GetSupportedBackends returns all supported backends
func GetSupportedBackends() []Backend {
	backends := make([]Backend, 0, len(BackendStateMaps))
	for backend := range BackendStateMaps {
		backends = append(backends, backend)
	}
	return backends
}

// IsBackendSupported checks if a backend is supported
func IsBackendSupported(backend Backend) bool {
	_, exists := BackendStateMaps[backend]
	return exists
}
