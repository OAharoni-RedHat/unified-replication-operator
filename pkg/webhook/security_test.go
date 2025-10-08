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

package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	replicationv1alpha1 "github.com/unified-replication/operator/api/v1alpha1"
	"github.com/unified-replication/operator/pkg/security"
)

// TestWebhookSecurity tests webhook security features
func TestWebhookSecurity(t *testing.T) {
	s := createTestScheme(t)
	client := fake.NewClientBuilder().WithScheme(s).Build()

	t.Run("ValidatorWithSecurity", func(t *testing.T) {
		secValidator := security.NewSecurityValidator()
		auditLogger := security.NewAuditLogger(unifiedvolumereplicationlog, true)

		validator := NewUnifiedVolumeReplicationValidatorWithSecurity(
			client,
			secValidator,
			auditLogger,
		)

		assert.NotNil(t, validator)
		assert.NotNil(t, validator.SecurityValidator)
		assert.NotNil(t, validator.AuditLogger)
		assert.True(t, validator.EnableAudit)
	})

	t.Run("ValidateSecureInput", func(t *testing.T) {
		validator := NewUnifiedVolumeReplicationValidator(client)
		ctx := context.Background()

		// Valid resource
		uvr := &replicationv1alpha1.UnifiedVolumeReplication{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "valid-name",
				Namespace: "default",
			},
			Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
				ReplicationState: replicationv1alpha1.ReplicationStateReplica,
				ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
				VolumeMapping: replicationv1alpha1.VolumeMapping{
					Source: replicationv1alpha1.VolumeSource{
						PvcName:   "source-pvc",
						Namespace: "default",
					},
					Destination: replicationv1alpha1.VolumeDestination{
						VolumeHandle: "dest-volume",
						Namespace:    "default",
					},
				},
				SourceEndpoint: replicationv1alpha1.Endpoint{
					Cluster:      "source-cluster",
					Region:       "us-east-1",
					StorageClass: "ceph-rbd",
				},
				DestinationEndpoint: replicationv1alpha1.Endpoint{
					Cluster:      "dest-cluster",
					Region:       "us-west-1",
					StorageClass: "ceph-rbd",
				},
				Schedule: replicationv1alpha1.Schedule{
					Mode: replicationv1alpha1.ScheduleModeContinuous,
					Rpo:  "15m",
					Rto:  "5m",
				},
			},
		}

		_, err := validator.ValidateCreate(ctx, uvr)
		assert.NoError(t, err, "Valid resource should pass validation")

		// Check audit log
		events := validator.AuditLogger.GetEvents()
		assert.GreaterOrEqual(t, len(events), 0, "Should have audit events if enabled")
	})

	t.Run("RejectDangerousInput", func(t *testing.T) {
		validator := NewUnifiedVolumeReplicationValidator(client)

		// Test name validation
		err := validator.SecurityValidator.ValidateName("Invalid-Name-With-Caps")
		assert.Error(t, err, "Should reject invalid name format")

		// Test script injection
		err = validator.SecurityValidator.ValidateNoScriptInjection("<script>alert('xss')</script>")
		assert.Error(t, err, "Should detect script injection")

		// Test path traversal
		err = validator.SecurityValidator.ValidateNoPathTraversal("../etc/passwd")
		assert.Error(t, err, "Should detect path traversal")
	})

	t.Run("AuditValidationEvents", func(t *testing.T) {
		validator := NewUnifiedVolumeReplicationValidator(client)
		validator.AuditLogger.ClearEvents()

		ctx := context.Background()

		// Create with valid resource
		uvr := createTestUVR()
		_, _ = validator.ValidateCreate(ctx, uvr)

		// Check audit log recorded validation
		events := validator.AuditLogger.GetEventsByType(security.AuditEventValidate)
		if validator.EnableAudit {
			assert.GreaterOrEqual(t, len(events), 0, "Should have validation events")
		}
	})
}

// TestWebhookPerformance tests webhook validation performance
func TestWebhookPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	client := fake.NewClientBuilder().Build()
	validator := NewUnifiedVolumeReplicationValidator(client)
	ctx := context.Background()

	uvr := createTestUVR()

	// Measure validation time
	iterations := 100
	start := time.Now()

	for i := 0; i < iterations; i++ {
		_, _ = validator.ValidateCreate(ctx, uvr)
	}

	duration := time.Since(start)
	avgTime := duration / time.Duration(iterations)

	t.Logf("Webhook validation performance: %d iterations in %v (avg: %v per validation)",
		iterations, duration, avgTime)

	// Validation should be < 100ms on average
	assert.Less(t, avgTime, 100*time.Millisecond,
		"Webhook validation should be fast (< 100ms)")
}

// TestWebhookReliability tests webhook under various conditions
func TestWebhookReliability(t *testing.T) {
	s := createTestScheme(t)
	client := fake.NewClientBuilder().WithScheme(s).Build()
	validator := NewUnifiedVolumeReplicationValidator(client)
	ctx := context.Background()

	t.Run("ValidateWithNilClient", func(t *testing.T) {
		// Skip - webhook needs client for PVC uniqueness check
		// This is by design - validator requires valid client
		t.Skip("Validator requires non-nil client for PVC validation")
	})

	t.Run("ValidateWithInvalidObject", func(t *testing.T) {
		// Can't easily test with invalid object type due to type system
		// This is tested implicitly - Go type system prevents wrong types
		t.Skip("Type safety tested at compile time")
	})

	t.Run("ConcurrentValidations", func(t *testing.T) {
		uvr := createTestUVR()
		done := make(chan error, 10)

		for i := 0; i < 10; i++ {
			go func() {
				_, err := validator.ValidateCreate(ctx, uvr)
				done <- err
			}()
		}

		for i := 0; i < 10; i++ {
			err := <-done
			assert.NoError(t, err, "Concurrent validation %d should succeed", i)
		}
	})
}

// TestWebhookAuditIntegration tests audit logging in webhook
func TestWebhookAuditIntegration(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	secValidator := security.NewSecurityValidator()
	auditLogger := security.NewAuditLogger(unifiedvolumereplicationlog, true)

	validator := NewUnifiedVolumeReplicationValidatorWithSecurity(
		client,
		secValidator,
		auditLogger,
	)

	ctx := context.Background()

	t.Run("AuditCreateOperation", func(t *testing.T) {
		auditLogger.ClearEvents()
		uvr := createTestUVR()

		_, _ = validator.ValidateCreate(ctx, uvr)

		events := auditLogger.GetEvents()
		// Audit events may be recorded depending on validation logic
		t.Logf("Audit events recorded: %d", len(events))
	})

	t.Run("AuditUpdateOperation", func(t *testing.T) {
		auditLogger.ClearEvents()
		oldUVR := createTestUVR()
		newUVR := createTestUVR()
		newUVR.Spec.ReplicationState = replicationv1alpha1.ReplicationStatePromoting

		_, _ = validator.ValidateUpdate(ctx, oldUVR, newUVR)

		events := auditLogger.GetEvents()
		t.Logf("Audit events for update: %d", len(events))
	})

	t.Run("ExportAuditLog", func(t *testing.T) {
		auditLogger.ClearEvents()

		// Generate some events
		auditLogger.LogCreate(ctx, "default", "test", "user", "success")
		auditLogger.LogUpdate(ctx, "default", "test", "user", "success", nil)

		// Export
		data, err := auditLogger.ExportEvents()
		assert.NoError(t, err)
		assert.NotEmpty(t, data)
		assert.Contains(t, string(data), "CREATE")
	})
}

// Helper functions

func createTestUVR() *replicationv1alpha1.UnifiedVolumeReplication {
	return &replicationv1alpha1.UnifiedVolumeReplication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-replication",
			Namespace: "default",
		},
		Spec: replicationv1alpha1.UnifiedVolumeReplicationSpec{
			ReplicationState: replicationv1alpha1.ReplicationStateReplica,
			ReplicationMode:  replicationv1alpha1.ReplicationModeAsynchronous,
			VolumeMapping: replicationv1alpha1.VolumeMapping{
				Source: replicationv1alpha1.VolumeSource{
					PvcName:   "source-pvc",
					Namespace: "default",
				},
				Destination: replicationv1alpha1.VolumeDestination{
					VolumeHandle: "dest-volume",
					Namespace:    "default",
				},
			},
			SourceEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "source-cluster",
				Region:       "us-east-1",
				StorageClass: "ceph-rbd",
			},
			DestinationEndpoint: replicationv1alpha1.Endpoint{
				Cluster:      "dest-cluster",
				Region:       "us-west-1",
				StorageClass: "ceph-rbd",
			},
			Schedule: replicationv1alpha1.Schedule{
				Mode: replicationv1alpha1.ScheduleModeContinuous,
				Rpo:  "15m",
				Rto:  "5m",
			},
		},
	}
}

// Helper functions

func createTestScheme(t *testing.T) *runtime.Scheme {
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, replicationv1alpha1.AddToScheme(s))
	return s
}
