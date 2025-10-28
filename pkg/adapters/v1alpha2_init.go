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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/unified-replication/operator/pkg/translation"
)

// RegisterV1Alpha2Adapters registers all v1alpha2 adapters with the registry
func RegisterV1Alpha2Adapters(registry Registry, client client.Client) {
	logger := log.Log.WithName("adapter-registry")
	logger.Info("Registering v1alpha2 adapters")

	// Register Ceph adapters
	cephAdapter := NewCephV1Alpha2Adapter(client)
	registry.RegisterVolumeReplicationAdapter(translation.BackendCeph, cephAdapter)
	registry.RegisterVolumeGroupReplicationAdapter(translation.BackendCeph, cephAdapter)
	logger.Info("Registered Ceph v1alpha2 adapters")

	// Register Trident adapters
	tridentAdapter := NewTridentV1Alpha2Adapter(client)
	registry.RegisterVolumeReplicationAdapter(translation.BackendTrident, tridentAdapter)
	registry.RegisterVolumeGroupReplicationAdapter(translation.BackendTrident, tridentAdapter)
	logger.Info("Registered Trident v1alpha2 adapters")

	// Register Dell PowerStore adapters
	powerstoreAdapter := NewPowerStoreV1Alpha2Adapter(client)
	registry.RegisterVolumeReplicationAdapter(translation.BackendPowerStore, powerstoreAdapter)
	registry.RegisterVolumeGroupReplicationAdapter(translation.BackendPowerStore, powerstoreAdapter)
	logger.Info("Registered Dell PowerStore v1alpha2 adapters")

	logger.Info("All v1alpha2 adapters registered successfully")
}
