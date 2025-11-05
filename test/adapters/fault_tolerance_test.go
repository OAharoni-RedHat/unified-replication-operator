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

package adapters_test

import (
	"testing"

	"github.com/unified-replication/operator/pkg/translation"
)

// TestErrorInjection tests adapter behavior with simulated failures
func TestErrorInjection(t *testing.T) {
	t.Skip("TODO: Rewrite fault tolerance tests for v1alpha2 API")
	backends := []translation.Backend{
		translation.BackendTrident,
		translation.BackendPowerStore,
	}

	for _, backend := range backends {
		t.Run(string(backend), func(t *testing.T) {
			testCreateFailure(t, backend)
			testUpdateFailure(t, backend)
			testDeleteFailure(t, backend)
			testStatusFailure(t, backend)
		})
	}
}

func testCreateFailure(t *testing.T, backend translation.Backend) {
	t.Skip("TODO: Rewrite for v1alpha2 API")
	_ = backend
}

func testUpdateFailure(t *testing.T, backend translation.Backend) {
	t.Skip("TODO: Rewrite for v1alpha2 API")
	_ = backend
}

func testDeleteFailure(t *testing.T, backend translation.Backend) {
	t.Skip("TODO: Rewrite for v1alpha2 API")
	_ = backend
}

func testStatusFailure(t *testing.T, backend translation.Backend) {
	t.Skip("TODO: Rewrite for v1alpha2 API")
	_ = backend
}
