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
	"regexp"
	"strings"
)

// SecurityValidator provides security validation for resources
type SecurityValidator struct {
	allowedNamespaces []string
	blockedPatterns   []*regexp.Regexp
	maxNameLength     int
	maxValueLength    int
}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator() *SecurityValidator {
	return &SecurityValidator{
		allowedNamespaces: []string{}, // Empty means all namespaces allowed
		blockedPatterns:   make([]*regexp.Regexp, 0),
		maxNameLength:     253, // Kubernetes max
		maxValueLength:    1024,
	}
}

// SanitizeInput sanitizes user input to prevent injection attacks
func (sv *SecurityValidator) SanitizeInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Remove control characters except newline and tab
	sanitized := make([]rune, 0, len(input))
	for _, r := range input {
		if r >= 32 || r == '\n' || r == '\t' {
			sanitized = append(sanitized, r)
		}
	}

	return string(sanitized)
}

// ValidateName validates a resource name
func (sv *SecurityValidator) ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	if len(name) > sv.maxNameLength {
		return fmt.Errorf("name too long: %d characters (max: %d)", len(name), sv.maxNameLength)
	}

	// Kubernetes naming validation
	nameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("invalid name format: must be lowercase alphanumeric with dashes")
	}

	return nil
}

// ValidateNamespace validates a namespace
func (sv *SecurityValidator) ValidateNamespace(namespace string) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}

	// Check if namespace is in allowed list (if restricted)
	if len(sv.allowedNamespaces) > 0 {
		allowed := false
		for _, ns := range sv.allowedNamespaces {
			if ns == namespace {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("namespace %s is not allowed", namespace)
		}
	}

	return sv.ValidateName(namespace)
}

// ValidateStringLength validates string length
func (sv *SecurityValidator) ValidateStringLength(value string, fieldName string) error {
	if len(value) > sv.maxValueLength {
		return fmt.Errorf("%s too long: %d characters (max: %d)", fieldName, len(value), sv.maxValueLength)
	}
	return nil
}

// ValidateNoScriptInjection checks for potential script injection
func (sv *SecurityValidator) ValidateNoScriptInjection(value string) error {
	dangerousPatterns := []string{
		"<script",
		"javascript:",
		"onerror=",
		"onclick=",
		"${", // Template injection
		"{{", // Template injection
		"$(", // Command injection
		"`",  // Backtick command execution
	}

	valueLower := strings.ToLower(value)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(valueLower, pattern) {
			return fmt.Errorf("potentially dangerous pattern detected: %s", pattern)
		}
	}

	return nil
}

// ValidateNoPathTraversal checks for path traversal attempts
func (sv *SecurityValidator) ValidateNoPathTraversal(value string) error {
	dangerousPatterns := []string{
		"../",
		"..\\",
		"/etc/",
		"/proc/",
		"/sys/",
		"C:\\",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(value, pattern) {
			return fmt.Errorf("potential path traversal detected: %s", pattern)
		}
	}

	return nil
}

// ValidateNoSQLInjection checks for SQL injection patterns
func (sv *SecurityValidator) ValidateNoSQLInjection(value string) error {
	// Note: This operator shouldn't handle SQL, but good practice
	sqlPatterns := []string{
		"DROP TABLE",
		"DELETE FROM",
		"INSERT INTO",
		"'; --",
		"' OR '1'='1",
		"UNION SELECT",
	}

	valueUpper := strings.ToUpper(value)
	for _, pattern := range sqlPatterns {
		if strings.Contains(valueUpper, pattern) {
			return fmt.Errorf("potential SQL injection pattern detected")
		}
	}

	return nil
}

// ValidateInput performs comprehensive input validation
func (sv *SecurityValidator) ValidateInput(value, fieldName string) error {
	// Sanitize first
	sanitized := sv.SanitizeInput(value)

	// Length check
	if err := sv.ValidateStringLength(sanitized, fieldName); err != nil {
		return err
	}

	// Injection checks
	if err := sv.ValidateNoScriptInjection(sanitized); err != nil {
		return fmt.Errorf("%s: %w", fieldName, err)
	}

	if err := sv.ValidateNoPathTraversal(sanitized); err != nil {
		return fmt.Errorf("%s: %w", fieldName, err)
	}

	return nil
}

// SetAllowedNamespaces sets the list of allowed namespaces
func (sv *SecurityValidator) SetAllowedNamespaces(namespaces []string) {
	sv.allowedNamespaces = namespaces
}

// AddBlockedPattern adds a pattern to block
func (sv *SecurityValidator) AddBlockedPattern(pattern string) error {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	sv.blockedPatterns = append(sv.blockedPatterns, regex)
	return nil
}

// ValidateAgainstBlockedPatterns checks if value matches any blocked pattern
func (sv *SecurityValidator) ValidateAgainstBlockedPatterns(value string) error {
	for _, pattern := range sv.blockedPatterns {
		if pattern.MatchString(value) {
			return fmt.Errorf("value matches blocked pattern")
		}
	}
	return nil
}

// SecretReference represents a reference to a Kubernetes secret
type SecretReference struct {
	Name      string
	Namespace string
	Key       string
}

// ValidateSecretReference validates a secret reference
func (sv *SecurityValidator) ValidateSecretReference(ref *SecretReference) error {
	if ref == nil {
		return fmt.Errorf("secret reference cannot be nil")
	}

	if err := sv.ValidateName(ref.Name); err != nil {
		return fmt.Errorf("invalid secret name: %w", err)
	}

	if err := sv.ValidateNamespace(ref.Namespace); err != nil {
		return fmt.Errorf("invalid secret namespace: %w", err)
	}

	if ref.Key == "" {
		return fmt.Errorf("secret key cannot be empty")
	}

	return nil
}

// ValidateClusterName validates a cluster name
func (sv *SecurityValidator) ValidateClusterName(clusterName string) error {
	if clusterName == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	// Cluster names should be DNS-compatible
	clusterRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !clusterRegex.MatchString(clusterName) {
		return fmt.Errorf("invalid cluster name format")
	}

	return sv.ValidateStringLength(clusterName, "cluster name")
}

// ValidateStorageClass validates a storage class name
func (sv *SecurityValidator) ValidateStorageClass(storageClass string) error {
	if storageClass == "" {
		return fmt.Errorf("storage class cannot be empty")
	}

	return sv.ValidateName(storageClass)
}

// ValidateScheduleExpression validates RPO/RTO expressions
func (sv *SecurityValidator) ValidateScheduleExpression(expr string) error {
	if expr == "" {
		return nil // Empty is allowed (optional field)
	}

	// Must match pattern: number + unit (s, m, h, d)
	scheduleRegex := regexp.MustCompile(`^[0-9]+(s|m|h|d)$`)
	if !scheduleRegex.MatchString(expr) {
		return fmt.Errorf("invalid schedule expression: must be like '15m', '1h', '30s'")
	}

	return nil
}
