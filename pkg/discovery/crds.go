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
	"github.com/unified-replication/operator/pkg/translation"
)

// CRDDefinition defines a CRD that needs to be checked for backend discovery
type CRDDefinition struct {
	Name      string `json:"name"`
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Required  bool   `json:"required"`  // Whether this CRD is required for the backend to be considered available
	Namespace string `json:"namespace"` // Namespace scope (empty for cluster-scoped)
}

// Backend CRD definitions based on the analysis

// CephCRDs defines the CRDs required for Ceph-CSI backend
// Ceph uses volume-replication-operator CRDs
var CephCRDs = []CRDDefinition{
	{
		Name:     "volumereplicationclasses.replication.storage.openshift.io",
		Group:    "replication.storage.openshift.io",
		Version:  "v1alpha1",
		Kind:     "VolumeReplicationClass",
		Required: true,
	},
	{
		Name:     "volumereplications.replication.storage.openshift.io",
		Group:    "replication.storage.openshift.io",
		Version:  "v1alpha1",
		Kind:     "VolumeReplication",
		Required: true,
	},
}

// TridentCRDs defines the CRDs required for Trident backend
// Trident has its own set of replication CRDs
var TridentCRDs = []CRDDefinition{
	{
		Name:     "tridentmirrorrelationships.trident.netapp.io",
		Group:    "trident.netapp.io",
		Version:  "v1",
		Kind:     "TridentMirrorRelationship",
		Required: true,
	},
	{
		Name:     "tridentactionmirrorupdates.trident.netapp.io",
		Group:    "trident.netapp.io",
		Version:  "v1",
		Kind:     "TridentActionMirrorUpdate",
		Required: false, // Optional - used for imperative actions
	},
	{
		Name:     "tridentvolumes.trident.netapp.io",
		Group:    "trident.netapp.io",
		Version:  "v1",
		Kind:     "TridentVolume",
		Required: true, // Required for volume management
	},
}

// PowerStoreCRDs defines the CRDs required for PowerStore backend
// PowerStore uses Dell CSM replication CRDs
var PowerStoreCRDs = []CRDDefinition{
	{
		Name:     "dellcsireplicationgroups.replication.storage.dell.com",
		Group:    "replication.storage.dell.com",
		Version:  "v1",
		Kind:     "DellCSIReplicationGroup",
		Required: true,
	},
}

// BackendCRDMap maps backends to their required CRDs
var BackendCRDMap = map[translation.Backend][]CRDDefinition{
	translation.BackendCeph:       CephCRDs,
	translation.BackendTrident:    TridentCRDs,
	translation.BackendPowerStore: PowerStoreCRDs,
}

// GetRequiredCRDsForBackend returns the CRDs required for a specific backend
func GetRequiredCRDsForBackend(backend translation.Backend) ([]CRDDefinition, bool) {
	crds, exists := BackendCRDMap[backend]
	return crds, exists
}

// GetAllCRDs returns all CRDs across all backends
func GetAllCRDs() []CRDDefinition {
	var allCRDs []CRDDefinition
	for _, crds := range BackendCRDMap {
		allCRDs = append(allCRDs, crds...)
	}
	return allCRDs
}

// GetBackendFromCRD attempts to identify which backend a CRD belongs to
func GetBackendFromCRD(crdName string) (translation.Backend, bool) {
	for backend, crds := range BackendCRDMap {
		for _, crd := range crds {
			if crd.Name == crdName {
				return backend, true
			}
		}
	}
	return "", false
}

// GetRequiredCRDs returns only the required CRDs for a backend
func GetRequiredCRDs(backend translation.Backend) []CRDDefinition {
	crds, exists := BackendCRDMap[backend]
	if !exists {
		return nil
	}

	var required []CRDDefinition
	for _, crd := range crds {
		if crd.Required {
			required = append(required, crd)
		}
	}
	return required
}

// GetOptionalCRDs returns only the optional CRDs for a backend
func GetOptionalCRDs(backend translation.Backend) []CRDDefinition {
	crds, exists := BackendCRDMap[backend]
	if !exists {
		return nil
	}

	var optional []CRDDefinition
	for _, crd := range crds {
		if !crd.Required {
			optional = append(optional, crd)
		}
	}
	return optional
}

// CRDExists checks if a CRD definition represents an existing CRD
func (crd CRDDefinition) String() string {
	return crd.Name
}

// FullName returns the full name of the CRD in group/version format
func (crd CRDDefinition) FullName() string {
	return crd.Group + "/" + crd.Version + "/" + crd.Kind
}

// IsClusterScoped returns true if the CRD is cluster-scoped
func (crd CRDDefinition) IsClusterScoped() bool {
	return crd.Namespace == ""
}

// BackendRequirements defines what's needed for a backend to be considered available
type BackendRequirements struct {
	Backend      translation.Backend `json:"backend"`
	RequiredCRDs []CRDDefinition     `json:"required_crds"`
	OptionalCRDs []CRDDefinition     `json:"optional_crds"`
	MinRequired  int                 `json:"min_required"` // Minimum number of required CRDs that must be present
}

// GetBackendRequirements returns the requirements for a specific backend
func GetBackendRequirements(backend translation.Backend) BackendRequirements {
	return BackendRequirements{
		Backend:      backend,
		RequiredCRDs: GetRequiredCRDs(backend),
		OptionalCRDs: GetOptionalCRDs(backend),
		MinRequired:  len(GetRequiredCRDs(backend)),
	}
}

// GetAllBackendRequirements returns requirements for all supported backends
func GetAllBackendRequirements() map[translation.Backend]BackendRequirements {
	requirements := make(map[translation.Backend]BackendRequirements)
	for _, backend := range translation.GetSupportedBackends() {
		requirements[backend] = GetBackendRequirements(backend)
	}
	return requirements
}
