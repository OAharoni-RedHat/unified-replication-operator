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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Test helpers shared across discovery package tests

// createFakeClient creates a fake Kubernetes client for testing
func createFakeClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = apiextensionsv1.AddToScheme(scheme)

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}

// createCRD creates a CRD for testing with proper establishment status
func createCRD(name, group, version, kind string, established bool) *apiextensionsv1.CustomResourceDefinition {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: group,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind: kind,
			},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    version,
					Storage: true,
					Served:  true,
				},
			},
		},
	}

	if established {
		crd.Status.Conditions = []apiextensionsv1.CustomResourceDefinitionCondition{
			{
				Type:   apiextensionsv1.Established,
				Status: apiextensionsv1.ConditionTrue,
			},
		}
	}

	return crd
}
