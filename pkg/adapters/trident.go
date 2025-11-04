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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TridentMirrorRelationship GVK
var TridentMirrorRelationshipGVK = schema.GroupVersionKind{
	Group:   "trident.netapp.io",
	Version: "v1",
	Kind:    "TridentMirrorRelationship",
}

// TridentActionMirrorUpdate GVK
var TridentActionMirrorUpdateGVK = schema.GroupVersionKind{
	Group:   "trident.netapp.io",
	Version: "v1",
	Kind:    "TridentActionMirrorUpdate",
}

// Note: TridentAdapter v1alpha1 implementation has been removed.
// Use TridentV1Alpha2Adapter in trident_v1alpha2.go for v1alpha2 API support.
