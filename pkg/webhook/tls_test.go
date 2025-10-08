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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTLSCertificate tests certificate generation and management
func TestTLSCertificate(t *testing.T) {
	t.Run("GenerateSelfSignedCertificate", func(t *testing.T) {
		config := DefaultCertificateConfig("test-namespace", "test-service")

		cert, err := GenerateSelfSignedCertificate(config)
		require.NoError(t, err)
		require.NotNil(t, cert)

		assert.NotEmpty(t, cert.CertPEM)
		assert.NotEmpty(t, cert.KeyPEM)
		assert.NotEmpty(t, cert.CAPEM)
	})

	t.Run("ValidateCertificate", func(t *testing.T) {
		config := DefaultCertificateConfig("test-namespace", "test-service")
		cert, err := GenerateSelfSignedCertificate(config)
		require.NoError(t, err)

		err = ValidateCertificate(cert.CertPEM)
		assert.NoError(t, err, "Generated certificate should be valid")
	})

	t.Run("GetCertificateExpiry", func(t *testing.T) {
		config := DefaultCertificateConfig("test-namespace", "test-service")
		cert, err := GenerateSelfSignedCertificate(config)
		require.NoError(t, err)

		expiry, err := GetCertificateExpiry(cert.CertPEM)
		assert.NoError(t, err)
		assert.True(t, expiry.After(time.Now()), "Certificate should not be expired")
		assert.True(t, expiry.Before(time.Now().Add(2*365*24*time.Hour)), "Expiry should be reasonable")
	})

	t.Run("IsCertificateExpiringSoon", func(t *testing.T) {
		config := DefaultCertificateConfig("test-namespace", "test-service")
		cert, err := GenerateSelfSignedCertificate(config)
		require.NoError(t, err)

		// Should not be expiring in next 30 days
		expiring, err := IsCertificateExpiringSoon(cert.CertPEM, 30*24*time.Hour)
		assert.NoError(t, err)
		assert.False(t, expiring, "New certificate should not be expiring soon")

		// Should be expiring within 2 years
		expiringLongTerm, err := IsCertificateExpiringSoon(cert.CertPEM, 2*365*24*time.Hour)
		assert.NoError(t, err)
		assert.True(t, expiringLongTerm, "Certificate should expire within 2 years")
	})

	t.Run("DefaultCertificateConfig", func(t *testing.T) {
		config := DefaultCertificateConfig("my-namespace", "webhook-service")

		assert.Equal(t, "my-namespace", config.Namespace)
		assert.Equal(t, "webhook-service", config.ServiceName)
		assert.Contains(t, config.DNSNames, "webhook-service")
		assert.Contains(t, config.DNSNames, "webhook-service.my-namespace")
		assert.Contains(t, config.DNSNames, "webhook-service.my-namespace.svc")
		assert.Contains(t, config.DNSNames, "webhook-service.my-namespace.svc.cluster.local")
		assert.Equal(t, 365*24*time.Hour, config.ValidityDuration)
	})

	t.Run("InvalidPEM", func(t *testing.T) {
		invalidPEM := []byte("not a valid PEM")

		err := ValidateCertificate(invalidPEM)
		assert.Error(t, err, "Should fail on invalid PEM")

		_, err = GetCertificateExpiry(invalidPEM)
		assert.Error(t, err, "Should fail to get expiry from invalid PEM")
	})
}
