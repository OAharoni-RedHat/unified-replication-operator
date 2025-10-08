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

package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/yaml"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

func TestSampleYAML_ValidatesAgainstSchema(t *testing.T) {
	// Read the sample YAML file
	samplePath := filepath.Join("..", "..", "config", "samples", "replication_v1alpha1_unifiedvolumereplication.yaml")
	sampleData, err := os.ReadFile(samplePath)
	require.NoError(t, err, "Failed to read sample YAML file")

	// Create a scheme with our types
	scheme := runtime.NewScheme()
	err = replicationv1alpha1.AddToScheme(scheme)
	require.NoError(t, err, "Failed to add to scheme")

	// Register the GVK
	gvk := schema.GroupVersionKind{
		Group:   "replication.unified.io",
		Version: "v1alpha1",
		Kind:    "UnifiedVolumeReplication",
	}
	scheme.AddKnownTypeWithName(gvk, &replicationv1alpha1.UnifiedVolumeReplication{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind("UnifiedVolumeReplicationList"), &replicationv1alpha1.UnifiedVolumeReplicationList{})

	// Create a decoder
	codecs := serializer.NewCodecFactory(scheme)
	decoder := codecs.UniversalDeserializer()

	// Decode the YAML
	obj, decodedGVK, err := decoder.Decode(sampleData, nil, nil)
	require.NoError(t, err, "Failed to decode sample YAML")

	// Verify it's the correct type
	assert.Equal(t, "replication.unified.io", decodedGVK.Group)
	assert.Equal(t, "v1alpha1", decodedGVK.Version)
	assert.Equal(t, "UnifiedVolumeReplication", decodedGVK.Kind)

	// Cast to our type and verify the structure
	uvr, ok := obj.(*replicationv1alpha1.UnifiedVolumeReplication)
	require.True(t, ok, "Decoded object is not a UnifiedVolumeReplication")

	// Verify key fields
	assert.Equal(t, "app-data-dr-sample", uvr.Name)
	assert.Equal(t, "prod-us-east-1", uvr.Spec.SourceEndpoint.Cluster)
	assert.Equal(t, "dr-us-west-2", uvr.Spec.DestinationEndpoint.Cluster)
	assert.Equal(t, replicationv1alpha1.ReplicationStateSource, uvr.Spec.ReplicationState)
	assert.Equal(t, replicationv1alpha1.ReplicationModeAsynchronous, uvr.Spec.ReplicationMode)

	// Verify extensions
	require.NotNil(t, uvr.Spec.Extensions)
	require.NotNil(t, uvr.Spec.Extensions.Ceph)
	assert.Equal(t, "journal", *uvr.Spec.Extensions.Ceph.MirroringMode)
}

func TestCRDGeneration_ProducesValidYAML(t *testing.T) {
	// Read the generated CRD file
	crdPath := filepath.Join("..", "..", "config", "crd", "bases", "replication.unified.io_unifiedvolumereplications.yaml")
	crdData, err := os.ReadFile(crdPath)
	require.NoError(t, err, "Failed to read generated CRD file")

	// Parse as YAML to verify it's valid
	var crd interface{}
	err = yaml.Unmarshal(crdData, &crd)
	require.NoError(t, err, "Generated CRD is not valid YAML")

	// Basic structure validation
	crdMap, ok := crd.(map[string]interface{})
	require.True(t, ok, "CRD root is not a map")

	// Verify basic CRD structure
	assert.Equal(t, "CustomResourceDefinition", crdMap["kind"])
	assert.Equal(t, "apiextensions.k8s.io/v1", crdMap["apiVersion"])

	// Verify metadata
	metadata, ok := crdMap["metadata"].(map[string]interface{})
	require.True(t, ok, "metadata is not a map")
	assert.Equal(t, "unifiedvolumereplications.replication.unified.io", metadata["name"])

	// Verify spec
	spec, ok := crdMap["spec"].(map[string]interface{})
	require.True(t, ok, "spec is not a map")
	assert.Equal(t, "replication.unified.io", spec["group"])

	// Verify names
	names, ok := spec["names"].(map[string]interface{})
	require.True(t, ok, "names is not a map")
	assert.Equal(t, "UnifiedVolumeReplication", names["kind"])
	assert.Equal(t, "unifiedvolumereplications", names["plural"])

	// Verify short names
	shortNames, ok := names["shortNames"].([]interface{})
	require.True(t, ok, "shortNames is not an array")
	assert.Contains(t, shortNames, "uvr")
	assert.Contains(t, shortNames, "unifiedvr")
}

func TestAPITypes_BasicValidation(t *testing.T) {
	// Test creating a valid UnifiedVolumeReplication in memory
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "test-cluster",
				Region:       "us-east-1",
				StorageClass: "fast-ssd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dr-cluster",
				Region:       "us-west-2",
				StorageClass: "backup-hdd",
			},
			VolumeMapping: replicationv1alpha1.VolumeMapping{
				Source: replicationv1alpha1.VolumeSource{
					PvcName:   "data-pvc",
					Namespace: "app",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "vol-123",
					Namespace:    "app-backup",
				},
			},
			ReplicationState: replicationv1alpha1.ReplicationStateSource,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeInterval,
				Rpo:  "30m",
				Rto:  "10m",
			},
		},
	}

	// Verify the object is properly constructed
	assert.Equal(t, "test-cluster", uvr.Spec.SourceEndpoint.Cluster)
	assert.Equal(t, replicationv1alpha1.ReplicationStateSource, uvr.Spec.ReplicationState)
	assert.Equal(t, replicationv1alpha1.ReplicationModeAsynchronous, uvr.Spec.ReplicationMode)
	assert.Equal(t, replicationv1alpha1.ScheduleModeInterval, uvr.Spec.Schedule.Mode)
}

func TestPrinterColumns_Configuration(t *testing.T) {
	// Read the generated CRD file
	crdPath := filepath.Join("..", "..", "config", "crd", "bases", "replication.unified.io_unifiedvolumereplications.yaml")
	crdData, err := os.ReadFile(crdPath)
	require.NoError(t, err, "Failed to read generated CRD file")

	// Parse as YAML
	var crd map[string]interface{}
	err = yaml.Unmarshal(crdData, &crd)
	require.NoError(t, err, "Generated CRD is not valid YAML")

	// Navigate to printer columns
	spec := crd["spec"].(map[string]interface{})
	versions := spec["versions"].([]interface{})
	version := versions[0].(map[string]interface{})

	printerColumns, ok := version["additionalPrinterColumns"].([]interface{})
	require.True(t, ok, "additionalPrinterColumns not found or not an array")

	// Verify expected printer columns exist
	expectedColumns := []string{"State", "Mode", "Source", "Ready", "Age"}
	foundColumns := make(map[string]bool)

	for _, col := range printerColumns {
		column := col.(map[string]interface{})
		name := column["name"].(string)
		foundColumns[name] = true
	}

	for _, expected := range expectedColumns {
		assert.True(t, foundColumns[expected], "Expected printer column %s not found", expected)
	}
}
