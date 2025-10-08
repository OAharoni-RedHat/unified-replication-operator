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

package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	ctrl "sigs.k8s.io/controller-runtime"
)

// TestSecurityValidator tests input validation and sanitization
func TestSecurityValidator(t *testing.T) {
	sv := NewSecurityValidator()

	t.Run("SanitizeInput", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"normal-string", "normal-string"},
			{"with\x00null", "withnull"},
			{"with\x01control", "withcontrol"},
			{"with\ttab\nand newline", "with\ttab\nand newline"},
		}

		for _, tt := range tests {
			result := sv.SanitizeInput(tt.input)
			assert.Equal(t, tt.expected, result)
		}
	})

	t.Run("ValidateName", func(t *testing.T) {
		validNames := []string{
			"valid-name",
			"valid-name-123",
			"a",
			"my-app",
		}

		for _, name := range validNames {
			err := sv.ValidateName(name)
			assert.NoError(t, err, "Name %s should be valid", name)
		}

		invalidNames := []string{
			"",
			"Invalid-With-Caps",
			"-starts-with-dash",
			"ends-with-dash-",
			"has spaces",
			"has_underscores",
		}

		for _, name := range invalidNames {
			err := sv.ValidateName(name)
			assert.Error(t, err, "Name %s should be invalid", name)
		}
	})

	t.Run("ValidateNoScriptInjection", func(t *testing.T) {
		dangerousInputs := []string{
			"<script>alert('xss')</script>",
			"javascript:void(0)",
			"onerror=alert(1)",
			"${command}",
			"{{template}}",
			"$(whoami)",
			"`command`",
		}

		for _, input := range dangerousInputs {
			err := sv.ValidateNoScriptInjection(input)
			assert.Error(t, err, "Should detect dangerous pattern in: %s", input)
		}

		safeInputs := []string{
			"normal-string",
			"my-replication-name",
			"cluster.example.com",
		}

		for _, input := range safeInputs {
			err := sv.ValidateNoScriptInjection(input)
			assert.NoError(t, err, "Safe input should pass: %s", input)
		}
	})

	t.Run("ValidateNoPathTraversal", func(t *testing.T) {
		dangerousInputs := []string{
			"../etc/passwd",
			"..\\windows\\system32",
			"/etc/shadow",
			"/proc/self/environ",
			"C:\\Windows\\System32",
		}

		for _, input := range dangerousInputs {
			err := sv.ValidateNoPathTraversal(input)
			assert.Error(t, err, "Should detect path traversal in: %s", input)
		}
	})

	t.Run("ValidateClusterName", func(t *testing.T) {
		validClusters := []string{
			"prod-cluster",
			"cluster1",
			"my-k8s-cluster",
		}

		for _, cluster := range validClusters {
			err := sv.ValidateClusterName(cluster)
			assert.NoError(t, err, "Cluster name %s should be valid", cluster)
		}

		invalidClusters := []string{
			"",
			"Cluster-With-Caps",
			"-starts-dash",
			"ends-",
			"has spaces",
		}

		for _, cluster := range invalidClusters {
			err := sv.ValidateClusterName(cluster)
			assert.Error(t, err, "Cluster name %s should be invalid", cluster)
		}
	})

	t.Run("ValidateScheduleExpression", func(t *testing.T) {
		validExpressions := []string{
			"", // Empty is allowed
			"15m",
			"1h",
			"30s",
			"1d",
			"120m",
		}

		for _, expr := range validExpressions {
			err := sv.ValidateScheduleExpression(expr)
			assert.NoError(t, err, "Expression %s should be valid", expr)
		}

		invalidExpressions := []string{
			"15",
			"15minutes",
			"1.5h",
			"invalid",
			"-15m",
		}

		for _, expr := range invalidExpressions {
			err := sv.ValidateScheduleExpression(expr)
			assert.Error(t, err, "Expression %s should be invalid", expr)
		}
	})

	t.Run("ValidateSecretReference", func(t *testing.T) {
		validRef := &SecretReference{
			Name:      "my-secret",
			Namespace: "default",
			Key:       "password",
		}

		err := sv.ValidateSecretReference(validRef)
		assert.NoError(t, err)

		nilRef := (*SecretReference)(nil)
		err = sv.ValidateSecretReference(nilRef)
		assert.Error(t, err)

		invalidRef := &SecretReference{
			Name:      "",
			Namespace: "default",
			Key:       "key",
		}
		err = sv.ValidateSecretReference(invalidRef)
		assert.Error(t, err)
	})
}

// TestAuditLogger tests audit logging functionality
func TestAuditLogger(t *testing.T) {
	logger := ctrl.Log.WithName("test")
	al := NewAuditLogger(logger, true)

	t.Run("LogCreate", func(t *testing.T) {
		ctx := context.Background()
		al.LogCreate(ctx, "default", "test-resource", "test-user", "success")

		events := al.GetEvents()
		assert.Len(t, events, 1)
		assert.Equal(t, AuditEventCreate, events[0].EventType)
		assert.Equal(t, "test-resource", events[0].ResourceName)
		assert.Equal(t, "success", events[0].Result)
	})

	t.Run("LogUpdate", func(t *testing.T) {
		al.ClearEvents()
		ctx := context.Background()

		details := map[string]interface{}{
			"old_state": "replica",
			"new_state": "source",
		}

		al.LogUpdate(ctx, "default", "test-resource", "test-user", "success", details)

		events := al.GetEvents()
		assert.Len(t, events, 1)
		assert.Equal(t, AuditEventUpdate, events[0].EventType)
		assert.Contains(t, events[0].Details, "old_state")
	})

	t.Run("LogValidation", func(t *testing.T) {
		al.ClearEvents()
		ctx := context.Background()

		al.LogValidation(ctx, "default", "test-resource", "denied", "invalid configuration")

		events := al.GetEvents()
		assert.Len(t, events, 1)
		assert.Equal(t, AuditEventValidate, events[0].EventType)
		assert.Equal(t, "denied", events[0].Result)
		assert.Equal(t, "invalid configuration", events[0].Reason)
	})

	t.Run("LogStateChange", func(t *testing.T) {
		al.ClearEvents()
		ctx := context.Background()

		al.LogStateChange(ctx, "default", "test-resource", "replica", "source")

		events := al.GetEvents()
		assert.Len(t, events, 1)
		assert.Equal(t, AuditEventStateChange, events[0].EventType)
		assert.Equal(t, "replica", events[0].Details["from_state"])
		assert.Equal(t, "source", events[0].Details["to_state"])
	})

	t.Run("GetEventsByType", func(t *testing.T) {
		al.ClearEvents()
		ctx := context.Background()

		al.LogCreate(ctx, "default", "res1", "user1", "success")
		al.LogUpdate(ctx, "default", "res2", "user1", "success", nil)
		al.LogCreate(ctx, "default", "res3", "user1", "success")

		createEvents := al.GetEventsByType(AuditEventCreate)
		assert.Len(t, createEvents, 2)

		updateEvents := al.GetEventsByType(AuditEventUpdate)
		assert.Len(t, updateEvents, 1)
	})

	t.Run("GetEventsSince", func(t *testing.T) {
		al.ClearEvents()
		ctx := context.Background()

		cutoffTime := time.Now()
		time.Sleep(10 * time.Millisecond)

		al.LogCreate(ctx, "default", "new-resource", "user", "success")

		recent := al.GetEventsSince(cutoffTime)
		assert.Len(t, recent, 1)

		old := al.GetEventsSince(time.Now().Add(1 * time.Hour))
		assert.Len(t, old, 0)
	})

	t.Run("ExportEvents", func(t *testing.T) {
		al.ClearEvents()
		ctx := context.Background()

		al.LogCreate(ctx, "default", "test", "user", "success")

		data, err := al.ExportEvents()
		assert.NoError(t, err)
		assert.NotEmpty(t, data)
		assert.Contains(t, string(data), "CREATE")
	})

	t.Run("EventCountByType", func(t *testing.T) {
		al.ClearEvents()
		ctx := context.Background()

		al.LogCreate(ctx, "default", "r1", "u1", "success")
		al.LogCreate(ctx, "default", "r2", "u1", "success")
		al.LogUpdate(ctx, "default", "r3", "u1", "success", nil)

		createCount := al.GetEventCountByType(AuditEventCreate)
		assert.Equal(t, 2, createCount)

		updateCount := al.GetEventCountByType(AuditEventUpdate)
		assert.Equal(t, 1, updateCount)
	})

	t.Run("DisabledAudit", func(t *testing.T) {
		disabledLogger := NewAuditLogger(logger, false)
		ctx := context.Background()

		disabledLogger.LogCreate(ctx, "default", "test", "user", "success")

		events := disabledLogger.GetEvents()
		assert.Len(t, events, 0, "Should not log when disabled")
	})
}

// TestRBACPolicy tests RBAC policy generation and validation
func TestRBACPolicy(t *testing.T) {
	t.Run("GetMinimalRBACPolicy", func(t *testing.T) {
		policy := GetMinimalRBACPolicy()
		assert.NotNil(t, policy)
		assert.NotEmpty(t, policy.Name)
		assert.NotEmpty(t, policy.Rules)
		assert.GreaterOrEqual(t, len(policy.Rules), 5, "Should have at least basic rules")
	})

	t.Run("GenerateClusterRoleYAML", func(t *testing.T) {
		policy := GetMinimalRBACPolicy()
		yaml := policy.GenerateClusterRoleYAML()

		assert.NotEmpty(t, yaml)
		assert.Contains(t, yaml, "kind: ClusterRole")
		assert.Contains(t, yaml, "replication.unified.io")
		assert.Contains(t, yaml, "unifiedvolumereplications")
		assert.Contains(t, yaml, "- get")
		assert.Contains(t, yaml, "- list")
		assert.Contains(t, yaml, "- watch")
	})

	t.Run("GenerateRoleYAML", func(t *testing.T) {
		policy := GetMinimalRBACPolicy()
		yaml := policy.GenerateRoleYAML("default")

		assert.NotEmpty(t, yaml)
		assert.Contains(t, yaml, "kind: Role")
		assert.Contains(t, yaml, "namespace: default")
	})

	t.Run("GetReadOnlyRBACPolicy", func(t *testing.T) {
		policy := GetReadOnlyRBACPolicy()
		assert.NotNil(t, policy)
		assert.Contains(t, policy.Name, "readonly")

		// Should only have get/list/watch verbs
		for _, rule := range policy.Rules {
			for _, verb := range rule.Verbs {
				assert.Contains(t, []string{"get", "list", "watch"}, verb,
					"Read-only policy should only have read verbs")
			}
		}
	})

	t.Run("ValidatePermissions", func(t *testing.T) {
		policy := GetMinimalRBACPolicy()

		// Sufficient permissions
		granted := []RBACRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		}

		err := policy.ValidatePermissions(granted)
		assert.NoError(t, err, "Wildcard permissions should satisfy all requirements")
	})
}

// TLS certificate tests are in pkg/webhook/tls_test.go since the functions are in that package

// TestSecurityIntegration tests security features working together
func TestSecurityIntegration(t *testing.T) {
	t.Run("ValidateAndAudit", func(t *testing.T) {
		sv := NewSecurityValidator()
		logger := ctrl.Log.WithName("test")
		al := NewAuditLogger(logger, true)

		ctx := context.Background()

		// Validate input
		input := "valid-name"
		err := sv.ValidateName(input)

		// Log the result
		if err != nil {
			al.LogValidation(ctx, "default", input, "denied", err.Error())
		} else {
			al.LogValidation(ctx, "default", input, "allowed", "")
		}

		events := al.GetEvents()
		assert.Len(t, events, 1)
		assert.Equal(t, "allowed", events[0].Result)
	})

	t.Run("DetectAndLogThreat", func(t *testing.T) {
		sv := NewSecurityValidator()
		logger := ctrl.Log.WithName("test")
		al := NewAuditLogger(logger, true)
		al.ClearEvents()

		ctx := context.Background()

		// Malicious input
		malicious := "<script>alert('xss')</script>"
		err := sv.ValidateNoScriptInjection(malicious)

		if err != nil {
			al.LogPolicyViolation(ctx, "default", "test-resource", "input-validation", err.Error())
		}

		events := al.GetEvents()
		assert.Len(t, events, 1)
		assert.Equal(t, AuditEventPolicyViolation, events[0].EventType)
	})
}

// BenchmarkSecurityValidation benchmarks security validation performance
func BenchmarkSecurityValidation(b *testing.B) {
	sv := NewSecurityValidator()

	b.Run("ValidateName", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = sv.ValidateName("test-resource-name")
		}
	})

	b.Run("SanitizeInput", func(b *testing.B) {
		input := "test-input-with-some-content"
		for i := 0; i < b.N; i++ {
			_ = sv.SanitizeInput(input)
		}
	})

	b.Run("ValidateNoScriptInjection", func(b *testing.B) {
		input := "normal-safe-input-string"
		for i := 0; i < b.N; i++ {
			_ = sv.ValidateNoScriptInjection(input)
		}
	})
}
