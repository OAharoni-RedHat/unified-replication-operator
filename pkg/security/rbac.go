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
	"fmt"
)

// RBACPolicy defines required RBAC permissions
type RBACPolicy struct {
	Name        string
	Description string
	Rules       []RBACRule
}

// RBACRule defines a single RBAC rule
type RBACRule struct {
	APIGroups []string
	Resources []string
	Verbs     []string
}

// GetMinimalRBACPolicy returns the minimal required RBAC policy
func GetMinimalRBACPolicy() *RBACPolicy {
	return &RBACPolicy{
		Name:        "unified-replication-operator",
		Description: "Minimal required permissions for Unified Replication Operator",
		Rules: []RBACRule{
			// UnifiedVolumeReplication resources
			{
				APIGroups: []string{"replication.unified.io"},
				Resources: []string{"unifiedvolumereplications"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
			{
				APIGroups: []string{"replication.unified.io"},
				Resources: []string{"unifiedvolumereplications/status"},
				Verbs:     []string{"get", "update", "patch"},
			},
			{
				APIGroups: []string{"replication.unified.io"},
				Resources: []string{"unifiedvolumereplications/finalizers"},
				Verbs:     []string{"update"},
			},

			// Ceph-CSI VolumeReplication resources
			{
				APIGroups: []string{"replication.storage.openshift.io"},
				Resources: []string{"volumereplications", "volumereplicationclasses"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},

			// Trident resources (if available)
			{
				APIGroups: []string{"trident.netapp.io"},
				Resources: []string{"tridentmirrorrelationships", "tridentactionmirrorupdates"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},

			// PowerStore resources (if available)
			{
				APIGroups: []string{"replication.dell.com"},
				Resources: []string{"dellcsireplicationgroups"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},

			// Core resources
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"create", "patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"storage.k8s.io"},
				Resources: []string{"storageclasses"},
				Verbs:     []string{"get", "list", "watch"},
			},

			// CRD discovery
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

// GetReadOnlyRBACPolicy returns a read-only RBAC policy (for monitoring/operators)
func GetReadOnlyRBACPolicy() *RBACPolicy {
	return &RBACPolicy{
		Name:        "unified-replication-operator-readonly",
		Description: "Read-only permissions for monitoring",
		Rules: []RBACRule{
			{
				APIGroups: []string{"replication.unified.io"},
				Resources: []string{"unifiedvolumereplications"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"replication.unified.io"},
				Resources: []string{"unifiedvolumereplications/status"},
				Verbs:     []string{"get"},
			},
		},
	}
}

// GenerateRoleYAML generates Kubernetes Role YAML for the policy
func (p *RBACPolicy) GenerateRoleYAML(namespace string) string {
	yaml := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/name: unified-replication-operator
    app.kubernetes.io/component: rbac
rules:
`, p.Name, namespace)

	for _, rule := range p.Rules {
		yaml += "- apiGroups:\n"
		for _, group := range rule.APIGroups {
			if group == "" {
				yaml += `  - ""`
			} else {
				yaml += fmt.Sprintf("  - %s\n", group)
			}
			yaml += "\n"
		}
		yaml += "  resources:\n"
		for _, resource := range rule.Resources {
			yaml += fmt.Sprintf("  - %s\n", resource)
		}
		yaml += "  verbs:\n"
		for _, verb := range rule.Verbs {
			yaml += fmt.Sprintf("  - %s\n", verb)
		}
	}

	return yaml
}

// GenerateClusterRoleYAML generates Kubernetes ClusterRole YAML for the policy
func (p *RBACPolicy) GenerateClusterRoleYAML() string {
	yaml := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: %s
  labels:
    app.kubernetes.io/name: unified-replication-operator
    app.kubernetes.io/component: rbac
rules:
`, p.Name)

	for _, rule := range p.Rules {
		yaml += "- apiGroups:\n"
		for _, group := range rule.APIGroups {
			if group == "" {
				yaml += `  - ""`
			} else {
				yaml += fmt.Sprintf("  - %s\n", group)
			}
			yaml += "\n"
		}
		yaml += "  resources:\n"
		for _, resource := range rule.Resources {
			yaml += fmt.Sprintf("  - %s\n", resource)
		}
		yaml += "  verbs:\n"
		for _, verb := range rule.Verbs {
			yaml += fmt.Sprintf("  - %s\n", verb)
		}
	}

	return yaml
}

// ValidatePermissions validates that all required permissions are present
func (p *RBACPolicy) ValidatePermissions(grantedRules []RBACRule) error {
	for _, required := range p.Rules {
		if !hasPermission(grantedRules, required) {
			return fmt.Errorf("missing required permission: %v", required)
		}
	}
	return nil
}

// hasPermission checks if a required permission is granted
func hasPermission(granted []RBACRule, required RBACRule) bool {
	for _, rule := range granted {
		if matches(rule, required) {
			return true
		}
	}
	return false
}

// matches checks if a granted rule covers a required rule
func matches(granted, required RBACRule) bool {
	// Check API groups
	if !containsAll(granted.APIGroups, required.APIGroups) {
		return false
	}

	// Check resources
	if !containsAll(granted.Resources, required.Resources) {
		return false
	}

	// Check verbs
	if !containsAll(granted.Verbs, required.Verbs) {
		return false
	}

	return true
}

// containsAll checks if all required items are in the granted list
func containsAll(granted, required []string) bool {
	for _, req := range required {
		found := false
		for _, grant := range granted {
			if grant == "*" || grant == req {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
