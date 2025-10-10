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

// Package utils provides comprehensive testing utilities for the unified replication operator
package utils

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

// TestClient provides a comprehensive testing client with utilities
type TestClient struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// NewTestClient creates a new test client with proper scheme registration
func NewTestClient(existingObjects ...runtime.Object) *TestClient {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = replicationv1alpha1.AddToScheme(scheme)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(existingObjects...).
		Build()

	return &TestClient{
		Client: client,
		Scheme: scheme,
	}
}

// CRDBuilder provides a fluent interface for building test CRDs
type CRDBuilder struct {
	uvr *replicationv1alpha1.UnifiedVolumeReplication
}

// NewCRDBuilder creates a new CRD builder with default values
func NewCRDBuilder() *CRDBuilder {
	return &CRDBuilder{
		uvr: &replicationv1alpha1.UnifiedVolumeReplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-replication",
				Namespace: "default",
			},
			Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
				SourceEndpoint: replicationv1alpha1.Endpoint{
					Cluster:      "source-cluster",
					Region:       "us-east-1",
					StorageClass: "fast-ssd",
				},
				DestinationEndpoint: replicationv1alpha1.Endpoint{
					Cluster:      "dest-cluster",
					Region:       "us-west-2",
					StorageClass: "backup-hdd",
				},
				VolumeMapping: replicationv1alpha1.VolumeMapping{
					Source: replicationv1alpha1.VolumeSource{
						PvcName:   "test-pvc",
						Namespace: "default",
					},
					Destination: replicationv1alpha1.VolumeDestination{
						VolumeHandle: "vol-12345",
						Namespace:    "default",
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
		},
	}
}

// WithName sets the name of the CRD
func (b *CRDBuilder) WithName(name string) *CRDBuilder {
	b.uvr.Name = name
	return b
}

// WithNamespace sets the namespace of the CRD
func (b *CRDBuilder) WithNamespace(namespace string) *CRDBuilder {
	b.uvr.Namespace = namespace
	return b
}

// WithSourceEndpoint sets the source endpoint
func (b *CRDBuilder) WithSourceEndpoint(cluster, region, storageClass string) *CRDBuilder {
	b.uvr.Spec.SourceEndpoint = replicationv1alpha1.Endpoint{
		Cluster:      cluster,
		Region:       region,
		StorageClass: storageClass,
	}
	return b
}

// WithDestinationEndpoint sets the destination endpoint
func (b *CRDBuilder) WithDestinationEndpoint(cluster, region, storageClass string) *CRDBuilder {
	b.uvr.Spec.DestinationEndpoint = replicationv1alpha1.Endpoint{
		Cluster:      cluster,
		Region:       region,
		StorageClass: storageClass,
	}
	return b
}

// WithVolumeMapping sets the volume mapping
func (b *CRDBuilder) WithVolumeMapping(sourcePvc, sourceNs, destHandle, destNs string) *CRDBuilder {
	b.uvr.Spec.VolumeMapping = replicationv1alpha1.VolumeMapping{
		Source: replicationv1alpha1.VolumeSource{
			PvcName:   sourcePvc,
			Namespace: sourceNs,
		},
		Destination: replicationv1alpha1.VolumeDestination{
			VolumeHandle: destHandle,
			Namespace:    destNs,
		},
	}
	return b
}

// WithReplicationState sets the replication state
func (b *CRDBuilder) WithReplicationState(state replicationv1alpha1.ReplicationState) *CRDBuilder {
	b.uvr.Spec.ReplicationState = state
	return b
}

// WithReplicationMode sets the replication mode
func (b *CRDBuilder) WithReplicationMode(mode replicationv1alpha1.ReplicationMode) *CRDBuilder {
	b.uvr.Spec.ReplicationMode = mode
	return b
}

// WithSchedule sets the schedule configuration
func (b *CRDBuilder) WithSchedule(mode replicationv1alpha1.ScheduleMode, rpo, rto string) *CRDBuilder {
	b.uvr.Spec.Schedule = replicationv1alpha1.Schedule{
		Mode: mode,
		Rpo:  rpo,
		Rto:  rto,
	}
	return b
}

// WithCephExtensions adds Ceph extensions
func (b *CRDBuilder) WithCephExtensions(mirroringMode string, startTime *metav1.Time) *CRDBuilder {
	if b.uvr.Spec.Extensions == nil {
		b.uvr.Spec.Extensions = &replicationv1alpha1.Extensions{}
	}
	b.uvr.Spec.Extensions.Ceph = &replicationv1alpha1.CephExtensions{
		MirroringMode: &mirroringMode,
	}
	return b
}

// WithTridentExtensions adds Trident extensions (currently empty, reserved for future use)
func (b *CRDBuilder) WithTridentExtensions() *CRDBuilder {
	if b.uvr.Spec.Extensions == nil {
		b.uvr.Spec.Extensions = &replicationv1alpha1.Extensions{}
	}
	b.uvr.Spec.Extensions.Trident = &replicationv1alpha1.TridentExtensions{}
	return b
}

// WithPowerStoreExtensions adds PowerStore extensions
func (b *CRDBuilder) WithPowerStoreExtensions(rpoSettings string, volumeGroups []string) *CRDBuilder {
	if b.uvr.Spec.Extensions == nil {
		b.uvr.Spec.Extensions = &replicationv1alpha1.Extensions{}
	}
	b.uvr.Spec.Extensions.Powerstore = &replicationv1alpha1.PowerStoreExtensions{}
	return b
}

// WithStatus sets the status
func (b *CRDBuilder) WithStatus(conditions []metav1.Condition, observedGeneration int64) *CRDBuilder {
	b.uvr.Status = replicationv1alpha1.UnifiedVolumeReplicationStatus{
		Conditions:         conditions,
		ObservedGeneration: observedGeneration,
	}
	return b
}

// WithLabels adds labels to the CRD
func (b *CRDBuilder) WithLabels(labels map[string]string) *CRDBuilder {
	if b.uvr.Labels == nil {
		b.uvr.Labels = make(map[string]string)
	}
	for k, v := range labels {
		b.uvr.Labels[k] = v
	}
	return b
}

// WithAnnotations adds annotations to the CRD
func (b *CRDBuilder) WithAnnotations(annotations map[string]string) *CRDBuilder {
	if b.uvr.Annotations == nil {
		b.uvr.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		b.uvr.Annotations[k] = v
	}
	return b
}

// Build returns the completed UnifiedVolumeReplication
func (b *CRDBuilder) Build() *replicationv1alpha1.UnifiedVolumeReplication {
	return b.uvr.DeepCopy()
}

// MockDataGenerator provides utilities for generating test data
type MockDataGenerator struct {
	rand *rand.Rand
}

// NewMockDataGenerator creates a new mock data generator
func NewMockDataGenerator(seed int64) *MockDataGenerator {
	return &MockDataGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// RandomName generates a random Kubernetes-compatible name
func (g *MockDataGenerator) RandomName(prefix string) string {
	suffix := g.rand.Intn(9999)
	return fmt.Sprintf("%s-%04d", strings.ToLower(prefix), suffix)
}

// RandomClusterName generates a random cluster name
func (g *MockDataGenerator) RandomClusterName() string {
	clusters := []string{"prod-cluster", "staging-cluster", "dev-cluster", "dr-cluster", "backup-cluster"}
	return clusters[g.rand.Intn(len(clusters))]
}

// RandomRegion generates a random AWS-style region
func (g *MockDataGenerator) RandomRegion() string {
	regions := []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1", "ca-central-1"}
	return regions[g.rand.Intn(len(regions))]
}

// RandomStorageClass generates a random storage class name
func (g *MockDataGenerator) RandomStorageClass() string {
	classes := []string{"fast-ssd", "slow-hdd", "ceph-rbd", "trident-nas", "powerstore-block"}
	return classes[g.rand.Intn(len(classes))]
}

// RandomReplicationState generates a random replication state
func (g *MockDataGenerator) RandomReplicationState() replicationv1alpha1.ReplicationState {
	states := []replicationv1alpha1.ReplicationState{
		replicationv1alpha1.ReplicationStateSource,
		replicationv1alpha1.ReplicationStateReplica,
		replicationv1alpha1.ReplicationStatePromoting,
		replicationv1alpha1.ReplicationStateDemoting,
		replicationv1alpha1.ReplicationStateSyncing,
		replicationv1alpha1.ReplicationStateFailed,
	}
	return states[g.rand.Intn(len(states))]
}

// RandomReplicationMode generates a random replication mode
func (g *MockDataGenerator) RandomReplicationMode() replicationv1alpha1.ReplicationMode {
	modes := []replicationv1alpha1.ReplicationMode{
		replicationv1alpha1.ReplicationModeSynchronous,
		replicationv1alpha1.ReplicationModeAsynchronous,
		replicationv1alpha1.ReplicationModeEventual,
	}
	return modes[g.rand.Intn(len(modes))]
}

// RandomScheduleMode generates a random schedule mode
func (g *MockDataGenerator) RandomScheduleMode() replicationv1alpha1.ScheduleMode {
	modes := []replicationv1alpha1.ScheduleMode{
		replicationv1alpha1.ScheduleModeContinuous,
		replicationv1alpha1.ScheduleModeInterval,
		replicationv1alpha1.ScheduleModeManual,
	}
	return modes[g.rand.Intn(len(modes))]
}

// RandomTimePattern generates a random time pattern (RPO/RTO)
func (g *MockDataGenerator) RandomTimePattern() string {
	values := []int{5, 10, 15, 30, 60, 120}
	units := []string{"s", "m", "h"}

	value := values[g.rand.Intn(len(values))]
	unit := units[g.rand.Intn(len(units))]

	return fmt.Sprintf("%d%s", value, unit)
}

// RandomCondition generates a random Kubernetes condition
func (g *MockDataGenerator) RandomCondition() metav1.Condition {
	types := []string{"Ready", "Synced", "Available", "Progressing"}
	statuses := []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse, metav1.ConditionUnknown}
	reasons := []string{"ReplicationActive", "SyncInProgress", "NetworkLatency", "ConfigurationError"}

	return metav1.Condition{
		Type:               types[g.rand.Intn(len(types))],
		Status:             statuses[g.rand.Intn(len(statuses))],
		Reason:             reasons[g.rand.Intn(len(reasons))],
		Message:            "Generated test condition",
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
}

// GenerateRandomCRD creates a completely randomized CRD for testing
func (g *MockDataGenerator) GenerateRandomCRD() *replicationv1alpha1.UnifiedVolumeReplication {
	builder := NewCRDBuilder().
		WithName(g.RandomName("random-replication")).
		WithNamespace("default").
		WithSourceEndpoint(g.RandomClusterName(), g.RandomRegion(), g.RandomStorageClass()).
		WithDestinationEndpoint(g.RandomClusterName(), g.RandomRegion(), g.RandomStorageClass()).
		WithVolumeMapping(g.RandomName("pvc"), "default", g.RandomName("vol"), "default").
		WithReplicationState(g.RandomReplicationState()).
		WithReplicationMode(g.RandomReplicationMode()).
		WithSchedule(g.RandomScheduleMode(), g.RandomTimePattern(), g.RandomTimePattern())

	// Randomly add extensions
	if g.rand.Float32() < 0.3 { // 30% chance of Ceph extensions
		builder.WithCephExtensions("journal", &metav1.Time{Time: time.Now()})
	}

	if g.rand.Float32() < 0.3 { // 30% chance of Trident extensions
		builder.WithTridentExtensions()
	}

	if g.rand.Float32() < 0.3 { // 30% chance of PowerStore extensions
		builder.WithPowerStoreExtensions("Five_Minutes", []string{g.RandomName("group")})
	}

	return builder.Build()
}

// CRDManipulator provides utilities for manipulating CRDs in tests
type CRDManipulator struct {
	client *TestClient
}

// NewCRDManipulator creates a new CRD manipulator
func NewCRDManipulator(client *TestClient) *CRDManipulator {
	return &CRDManipulator{client: client}
}

// CreateCRD creates a CRD in the test cluster
func (m *CRDManipulator) CreateCRD(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	return m.client.Client.Create(ctx, uvr)
}

// UpdateCRD updates a CRD in the test cluster
func (m *CRDManipulator) UpdateCRD(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	return m.client.Client.Update(ctx, uvr)
}

// UpdateCRDStatus updates the status of a CRD
func (m *CRDManipulator) UpdateCRDStatus(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	return m.client.Client.Status().Update(ctx, uvr)
}

// DeleteCRD deletes a CRD from the test cluster
func (m *CRDManipulator) DeleteCRD(ctx context.Context, uvr *replicationv1alpha1.UnifiedVolumeReplication) error {
	return m.client.Client.Delete(ctx, uvr)
}

// GetCRD retrieves a CRD from the test cluster
func (m *CRDManipulator) GetCRD(ctx context.Context, name, namespace string) (*replicationv1alpha1.UnifiedVolumeReplication, error) {
	uvr := &replicationv1alpha1.UnifiedVolumeReplication{}
	key := types.NamespacedName{Name: name, Namespace: namespace}
	err := m.client.Client.Get(ctx, key, uvr)
	return uvr, err
}

// ListCRDs lists all CRDs in a namespace
func (m *CRDManipulator) ListCRDs(ctx context.Context, namespace string) (*replicationv1alpha1.UnifiedVolumeReplicationList, error) {
	list := &replicationv1alpha1.UnifiedVolumeReplicationList{}
	err := m.client.Client.List(ctx, list, client.InNamespace(namespace))
	return list, err
}

// WaitForCRDCondition waits for a specific condition on a CRD
func (m *CRDManipulator) WaitForCRDCondition(ctx context.Context, name, namespace, conditionType string, status metav1.ConditionStatus, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		uvr, err := m.GetCRD(ctx, name, namespace)
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		for _, condition := range uvr.Status.Conditions {
			if condition.Type == conditionType && condition.Status == status {
				return nil
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for condition %s=%s on %s/%s", conditionType, status, namespace, name)
}
