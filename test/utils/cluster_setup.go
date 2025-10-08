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

package utils

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
)

// TestEnvironment provides a comprehensive test environment with Kubernetes API server
type TestEnvironment struct {
	Environment *envtest.Environment
	Config      *rest.Config
	Client      client.Client
	Scheme      *runtime.Scheme
	ctx         context.Context
	cancelFunc  context.CancelFunc
}

// TestEnvironmentOptions configures the test environment
type TestEnvironmentOptions struct {
	// CRDPaths specifies paths to CRD directories
	CRDPaths []string

	// UseExistingCluster indicates whether to use an existing cluster
	UseExistingCluster bool

	// AttachControlPlaneOutput attaches control plane output to test logs
	AttachControlPlaneOutput bool

	// BinaryAssetsDirectory specifies the directory containing test binaries
	BinaryAssetsDirectory string

	// Timeout specifies the timeout for environment setup
	Timeout time.Duration
}

// DefaultTestEnvironmentOptions returns default options for test environment
func DefaultTestEnvironmentOptions() *TestEnvironmentOptions {
	return &TestEnvironmentOptions{
		CRDPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
		},
		UseExistingCluster:       false,
		AttachControlPlaneOutput: false,
		Timeout:                  60 * time.Second,
	}
}

// NewTestEnvironment creates a new test environment with envtest
func NewTestEnvironment(t *testing.T, options *TestEnvironmentOptions) *TestEnvironment {
	if options == nil {
		options = DefaultTestEnvironmentOptions()
	}

	// Set up logging
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	// Create scheme
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))
	require.NoError(t, replicationv1alpha1.AddToScheme(scheme))

	// Create envtest environment
	env := &envtest.Environment{
		CRDDirectoryPaths:        options.CRDPaths,
		ErrorIfCRDPathMissing:    true,
		UseExistingCluster:       &options.UseExistingCluster,
		AttachControlPlaneOutput: options.AttachControlPlaneOutput,
	}

	if options.BinaryAssetsDirectory != "" {
		env.BinaryAssetsDirectory = options.BinaryAssetsDirectory
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)

	// Start the test environment
	config, err := env.Start()
	require.NoError(t, err, "Failed to start test environment")

	// Create client
	k8sClient, err := client.New(config, client.Options{Scheme: scheme})
	require.NoError(t, err, "Failed to create Kubernetes client")

	return &TestEnvironment{
		Environment: env,
		Config:      config,
		Client:      k8sClient,
		Scheme:      scheme,
		ctx:         ctx,
		cancelFunc:  cancel,
	}
}

// Stop stops the test environment and cleans up resources
func (te *TestEnvironment) Stop(t *testing.T) {
	if te.cancelFunc != nil {
		te.cancelFunc()
	}

	if te.Environment != nil {
		err := te.Environment.Stop()
		require.NoError(t, err, "Failed to stop test environment")
	}
}

// Context returns the context for the test environment
func (te *TestEnvironment) Context() context.Context {
	return te.ctx
}

// CreateNamespace creates a namespace in the test environment
func (te *TestEnvironment) CreateNamespace(t *testing.T, name string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err := te.Client.Create(te.ctx, ns)
	require.NoError(t, err, "Failed to create namespace %s", name)
}

// DeleteNamespace deletes a namespace from the test environment
func (te *TestEnvironment) DeleteNamespace(t *testing.T, name string) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	err := te.Client.Delete(te.ctx, ns)
	if err != nil && !apierrors.IsNotFound(err) {
		require.NoError(t, err, "Failed to delete namespace %s", name)
	}
}

// WaitForCRDReady waits for CRDs to be ready in the cluster
func (te *TestEnvironment) WaitForCRDReady(t *testing.T, crdName string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := te.Client.Get(te.ctx, client.ObjectKey{Name: crdName}, crd)
		if err == nil {
			// Check if CRD is established
			for _, condition := range crd.Status.Conditions {
				if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
					return // CRD is ready
				}
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	require.Fail(t, "Timeout waiting for CRD %s to be ready", crdName)
}

// TestSuite provides a comprehensive test suite with setup and teardown
type TestSuite struct {
	Environment   *TestEnvironment
	AssertHelper  *AssertionHelper
	DataGenerator *MockDataGenerator
	Manipulator   *CRDManipulator
	TestClient    *TestClient
}

// NewTestSuite creates a new comprehensive test suite
func NewTestSuite(t *testing.T, options *TestEnvironmentOptions) *TestSuite {
	// Create test environment
	env := NewTestEnvironment(t, options)

	// Create test client wrapper
	testClient := &TestClient{
		Client: env.Client,
		Scheme: env.Scheme,
	}

	// Create utilities
	assertHelper := NewAssertionHelper(t)
	dataGenerator := NewMockDataGenerator(time.Now().UnixNano())
	manipulator := NewCRDManipulator(testClient)

	return &TestSuite{
		Environment:   env,
		AssertHelper:  assertHelper,
		DataGenerator: dataGenerator,
		Manipulator:   manipulator,
		TestClient:    testClient,
	}
}

// Cleanup cleans up the test suite
func (ts *TestSuite) Cleanup(t *testing.T) {
	ts.Environment.Stop(t)
}

// CreateTestNamespace creates a test namespace with a random name
func (ts *TestSuite) CreateTestNamespace(t *testing.T) string {
	namespaceName := fmt.Sprintf("test-ns-%d", time.Now().UnixNano())
	ts.Environment.CreateNamespace(t, namespaceName)

	// Schedule cleanup
	t.Cleanup(func() {
		ts.Environment.DeleteNamespace(t, namespaceName)
	})

	return namespaceName
}

// SetupBasicCRD creates a basic CRD for testing
func (ts *TestSuite) SetupBasicCRD(t *testing.T, namespace string) *replicationv1alpha1.UnifiedVolumeReplication {
	uvr := NewCRDBuilder().
		WithName(ts.DataGenerator.RandomName("test-uvr")).
		WithNamespace(namespace).
		Build()

	err := ts.Manipulator.CreateCRD(ts.Environment.Context(), uvr)
	require.NoError(t, err, "Failed to create test CRD")

	// Schedule cleanup
	t.Cleanup(func() {
		_ = ts.Manipulator.DeleteCRD(ts.Environment.Context(), uvr)
	})

	return uvr
}

// WaitForCRDCondition waits for a specific condition on a CRD
func (ts *TestSuite) WaitForCRDCondition(t *testing.T, name, namespace, conditionType string, status metav1.ConditionStatus, timeout time.Duration) {
	err := ts.Manipulator.WaitForCRDCondition(ts.Environment.Context(), name, namespace, conditionType, status, timeout)
	require.NoError(t, err, "Failed to wait for CRD condition")
}

// TestCluster provides utilities for managing test clusters
type TestCluster struct {
	Name        string
	Namespace   string
	Environment *TestEnvironment
}

// NewTestCluster creates a new test cluster
func NewTestCluster(t *testing.T, name, namespace string, env *TestEnvironment) *TestCluster {
	cluster := &TestCluster{
		Name:        name,
		Namespace:   namespace,
		Environment: env,
	}

	// Create namespace if it doesn't exist
	env.CreateNamespace(t, namespace)

	return cluster
}

// CreatePVC creates a PVC in the test cluster
func (tc *TestCluster) CreatePVC(t *testing.T, name, storageClass string, size string) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: tc.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(size),
				},
			},
		},
	}

	if storageClass != "" {
		pvc.Spec.StorageClassName = &storageClass
	}

	err := tc.Environment.Client.Create(tc.Environment.Context(), pvc)
	require.NoError(t, err, "Failed to create PVC %s", name)

	return pvc
}

// DeletePVC deletes a PVC from the test cluster
func (tc *TestCluster) DeletePVC(t *testing.T, name string) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: tc.Namespace,
		},
	}

	err := tc.Environment.Client.Delete(tc.Environment.Context(), pvc)
	if err != nil && !apierrors.IsNotFound(err) {
		require.NoError(t, err, "Failed to delete PVC %s", name)
	}
}

// Performance tracking
type PerformanceTracker struct {
	operations map[string]time.Duration
	startTimes map[string]time.Time
}

// NewPerformanceTracker creates a new performance tracker
func NewPerformanceTracker() *PerformanceTracker {
	return &PerformanceTracker{
		operations: make(map[string]time.Duration),
		startTimes: make(map[string]time.Time),
	}
}

// StartOperation starts tracking an operation
func (pt *PerformanceTracker) StartOperation(name string) {
	pt.startTimes[name] = time.Now()
}

// EndOperation ends tracking an operation and records the duration
func (pt *PerformanceTracker) EndOperation(name string) time.Duration {
	startTime, exists := pt.startTimes[name]
	if !exists {
		return 0
	}

	duration := time.Since(startTime)
	pt.operations[name] = duration
	delete(pt.startTimes, name)

	return duration
}

// GetOperationDuration returns the duration of a completed operation
func (pt *PerformanceTracker) GetOperationDuration(name string) time.Duration {
	return pt.operations[name]
}

// GetAllOperations returns all recorded operations and their durations
func (pt *PerformanceTracker) GetAllOperations() map[string]time.Duration {
	result := make(map[string]time.Duration)
	for name, duration := range pt.operations {
		result[name] = duration
	}
	return result
}

// Reset clears all recorded operations
func (pt *PerformanceTracker) Reset() {
	pt.operations = make(map[string]time.Duration)
	pt.startTimes = make(map[string]time.Time)
}
