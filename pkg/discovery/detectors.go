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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/unified-replication/operator/pkg/translation"
)

// BaseDetector provides common functionality for all backend detectors
type BaseDetector struct {
	client  client.Client
	backend translation.Backend
	crds    []CRDDefinition
}

// NewBaseDetector creates a new base detector
func NewBaseDetector(client client.Client, backend translation.Backend, crds []CRDDefinition) *BaseDetector {
	return &BaseDetector{
		client:  client,
		backend: backend,
		crds:    crds,
	}
}

// DetectBackend implements the basic CRD detection logic
func (bd *BaseDetector) DetectBackend(ctx context.Context) (*BackendDiscoveryResult, error) {
	logger := log.FromContext(ctx).WithName("detector").WithValues("backend", bd.backend)
	logger.V(1).Info("Starting backend detection")

	result := &BackendDiscoveryResult{
		Backend:     bd.backend,
		Status:      BackendStatusUnknown,
		CRDs:        make([]CRDInfo, 0, len(bd.crds)),
		LastUpdated: time.Now(),
	}

	var availableCRDs, requiredCRDs, missingRequired int

	// Check each CRD
	for _, crdDef := range bd.crds {
		crdInfo := CRDInfo{
			Name:      crdDef.Name,
			Group:     crdDef.Group,
			Version:   crdDef.Version,
			Kind:      crdDef.Kind,
			Available: false,
		}

		// Check if CRD exists and is ready
		ready, err := bd.checkCRDReady(ctx, crdDef.Name)
		if err != nil {
			logger.Error(err, "Failed to check CRD", "crd", crdDef.Name)
			result.Message = "Failed to check CRD availability"
			result.Status = BackendStatusUnknown
			return result, err
		}

		crdInfo.Available = ready
		crdInfo.Controller = ready // For now, assume controller is ready if CRD is ready

		if ready {
			availableCRDs++
		}

		if crdDef.Required {
			requiredCRDs++
			if !ready {
				missingRequired++
			}
		}

		result.CRDs = append(result.CRDs, crdInfo)
	}

	// Determine backend status
	if missingRequired == 0 {
		result.Status = BackendStatusAvailable
		result.Message = "All required CRDs are available"
	} else if missingRequired < requiredCRDs {
		result.Status = BackendStatusPartial
		result.Message = "Some required CRDs are missing"
	} else {
		result.Status = BackendStatusUnavailable
		result.Message = "Required CRDs are not available"
	}

	logger.Info("Backend detection completed",
		"status", result.Status,
		"available_crds", availableCRDs,
		"total_crds", len(bd.crds),
		"missing_required", missingRequired)

	return result, nil
}

// checkCRDReady checks if a CRD exists and is established
func (bd *BaseDetector) checkCRDReady(ctx context.Context, crdName string) (bool, error) {
	engine := &Engine{client: bd.client}
	return engine.CheckCRDReady(ctx, crdName)
}

// GetRequiredCRDs returns the list of required CRDs for this detector
func (bd *BaseDetector) GetRequiredCRDs() []CRDInfo {
	var required []CRDInfo
	for _, crd := range bd.crds {
		if crd.Required {
			required = append(required, CRDInfo{
				Name:    crd.Name,
				Group:   crd.Group,
				Version: crd.Version,
				Kind:    crd.Kind,
			})
		}
	}
	return required
}

// GetBackendType returns the backend type this detector handles
func (bd *BaseDetector) GetBackendType() translation.Backend {
	return bd.backend
}

// ValidateBackend performs basic validation (can be overridden by specific detectors)
func (bd *BaseDetector) ValidateBackend(ctx context.Context) error {
	// Base implementation just checks CRD availability
	result, err := bd.DetectBackend(ctx)
	if err != nil {
		return err
	}

	if result.Status != BackendStatusAvailable {
		return NewDiscoveryError(ErrorTypeCRDNotFound, bd.backend, "", result.Message)
	}

	return nil
}

// CephDetector implements detection for Ceph-CSI backend
type CephDetector struct {
	*BaseDetector
}

// NewCephDetector creates a new Ceph detector
func NewCephDetector(client client.Client) BackendDetector {
	return &CephDetector{
		BaseDetector: NewBaseDetector(client, translation.BackendCeph, CephCRDs),
	}
}

// ValidateBackend performs Ceph-specific validation
func (cd *CephDetector) ValidateBackend(ctx context.Context) error {
	// First perform base validation
	if err := cd.BaseDetector.ValidateBackend(ctx); err != nil {
		return err
	}

	// Additional Ceph-specific validation could go here
	// For example, check for specific storage classes, CSI drivers, etc.

	return nil
}

// TridentDetector implements detection for Trident backend
type TridentDetector struct {
	*BaseDetector
}

// NewTridentDetector creates a new Trident detector
func NewTridentDetector(client client.Client) BackendDetector {
	return &TridentDetector{
		BaseDetector: NewBaseDetector(client, translation.BackendTrident, TridentCRDs),
	}
}

// ValidateBackend performs Trident-specific validation
func (td *TridentDetector) ValidateBackend(ctx context.Context) error {
	// First perform base validation
	if err := td.BaseDetector.ValidateBackend(ctx); err != nil {
		return err
	}

	// Additional Trident-specific validation could go here
	// For example, check for Trident controller deployment, backend configurations, etc.

	return nil
}

// PowerStoreDetector implements detection for PowerStore backend
type PowerStoreDetector struct {
	*BaseDetector
}

// NewPowerStoreDetector creates a new PowerStore detector
func NewPowerStoreDetector(client client.Client) BackendDetector {
	return &PowerStoreDetector{
		BaseDetector: NewBaseDetector(client, translation.BackendPowerStore, PowerStoreCRDs),
	}
}

// ValidateBackend performs PowerStore-specific validation
func (psd *PowerStoreDetector) ValidateBackend(ctx context.Context) error {
	// First perform base validation
	if err := psd.BaseDetector.ValidateBackend(ctx); err != nil {
		return err
	}

	// Additional PowerStore-specific validation could go here
	// For example, check for CSI driver deployment, storage classes, etc.

	return nil
}

// DetectorRegistry manages backend detectors
type DetectorRegistry struct {
	detectors map[translation.Backend]BackendDetector
}

// NewDetectorRegistry creates a new detector registry
func NewDetectorRegistry(client client.Client) *DetectorRegistry {
	registry := &DetectorRegistry{
		detectors: make(map[translation.Backend]BackendDetector),
	}

	// Register all backend detectors
	registry.detectors[translation.BackendCeph] = NewCephDetector(client)
	registry.detectors[translation.BackendTrident] = NewTridentDetector(client)
	registry.detectors[translation.BackendPowerStore] = NewPowerStoreDetector(client)

	return registry
}

// GetDetector returns the detector for a specific backend
func (dr *DetectorRegistry) GetDetector(backend translation.Backend) (BackendDetector, bool) {
	detector, exists := dr.detectors[backend]
	return detector, exists
}

// GetAllDetectors returns all registered detectors
func (dr *DetectorRegistry) GetAllDetectors() map[translation.Backend]BackendDetector {
	return dr.detectors
}

// RegisterDetector registers a custom detector for a backend
func (dr *DetectorRegistry) RegisterDetector(backend translation.Backend, detector BackendDetector) {
	dr.detectors[backend] = detector
}

// DetectAll runs detection for all registered backends
func (dr *DetectorRegistry) DetectAll(ctx context.Context) (map[translation.Backend]*BackendDiscoveryResult, error) {
	results := make(map[translation.Backend]*BackendDiscoveryResult)

	for backend, detector := range dr.detectors {
		result, err := detector.DetectBackend(ctx)
		if err != nil {
			// Continue with other backends even if one fails
			result = &BackendDiscoveryResult{
				Backend:     backend,
				Status:      BackendStatusUnavailable,
				Message:     err.Error(),
				LastUpdated: time.Now(),
			}
		}
		results[backend] = result
	}

	return results, nil
}
