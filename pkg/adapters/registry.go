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

package adapters

import (
	"context"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/unified-replication/operator/pkg/translation"
)

// Registry manages v1alpha2 adapters
type Registry interface {
	// v1alpha2 Adapter management
	GetVolumeReplicationAdapter(backend translation.Backend) VolumeReplicationAdapter
	GetVolumeGroupReplicationAdapter(backend translation.Backend) VolumeGroupReplicationAdapter
	RegisterVolumeReplicationAdapter(backend translation.Backend, adapter VolumeReplicationAdapter)
	RegisterVolumeGroupReplicationAdapter(backend translation.Backend, adapter VolumeGroupReplicationAdapter)

	// Lifecycle management
	Initialize(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// DefaultRegistry implements the Registry interface
type DefaultRegistry struct {
	// v1alpha2 support
	vrAdapters  map[translation.Backend]VolumeReplicationAdapter
	vgrAdapters map[translation.Backend]VolumeGroupReplicationAdapter

	mu          sync.RWMutex
	initialized bool
}

// NewRegistry creates a new adapter registry
func NewRegistry() Registry {
	return &DefaultRegistry{
		vrAdapters:  make(map[translation.Backend]VolumeReplicationAdapter),
		vgrAdapters: make(map[translation.Backend]VolumeGroupReplicationAdapter),
	}
}


// Initialize initializes the registry
func (r *DefaultRegistry) Initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return nil
	}

	logger := log.FromContext(ctx).WithName("adapter-registry")
	logger.Info("Initializing adapter registry")

	r.initialized = true
	logger.Info("Adapter registry initialized successfully")
	return nil
}

// Shutdown shuts down the registry
func (r *DefaultRegistry) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.initialized {
		return nil
	}

	logger := log.FromContext(ctx).WithName("adapter-registry")
	logger.Info("Shutting down adapter registry")

	r.initialized = false
	logger.Info("Adapter registry shutdown completed")
	return nil
}



// v1alpha2 Adapter Management Methods

// GetVolumeReplicationAdapter returns the v1alpha2 adapter for single volume replication
func (r *DefaultRegistry) GetVolumeReplicationAdapter(backend translation.Backend) VolumeReplicationAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.vrAdapters[backend]
}

// GetVolumeGroupReplicationAdapter returns the v1alpha2 adapter for volume group replication
func (r *DefaultRegistry) GetVolumeGroupReplicationAdapter(backend translation.Backend) VolumeGroupReplicationAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.vgrAdapters[backend]
}

// RegisterVolumeReplicationAdapter registers a v1alpha2 volume replication adapter
func (r *DefaultRegistry) RegisterVolumeReplicationAdapter(backend translation.Backend, adapter VolumeReplicationAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.vrAdapters[backend] = adapter
}

// RegisterVolumeGroupReplicationAdapter registers a v1alpha2 volume group replication adapter
func (r *DefaultRegistry) RegisterVolumeGroupReplicationAdapter(backend translation.Backend, adapter VolumeGroupReplicationAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.vgrAdapters[backend] = adapter
}
