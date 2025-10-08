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
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// CertificateConfig contains configuration for certificate generation
type CertificateConfig struct {
	ServiceName      string
	Namespace        string
	CommonName       string
	Organization     []string
	DNSNames         []string
	ValidityDuration time.Duration
}

// DefaultCertificateConfig returns default certificate configuration
func DefaultCertificateConfig(namespace, serviceName string) *CertificateConfig {
	return &CertificateConfig{
		ServiceName:  serviceName,
		Namespace:    namespace,
		CommonName:   fmt.Sprintf("%s.%s.svc", serviceName, namespace),
		Organization: []string{"unified-replication-operator"},
		DNSNames: []string{
			serviceName,
			fmt.Sprintf("%s.%s", serviceName, namespace),
			fmt.Sprintf("%s.%s.svc", serviceName, namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
		},
		ValidityDuration: 365 * 24 * time.Hour, // 1 year
	}
}

// TLSCertificate represents a TLS certificate and key pair
type TLSCertificate struct {
	CertPEM []byte
	KeyPEM  []byte
	CAPEM   []byte
}

// GenerateSelfSignedCertificate generates a self-signed certificate for webhook
func GenerateSelfSignedCertificate(config *CertificateConfig) (*TLSCertificate, error) {
	// Generate CA private key
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA key: %w", err)
	}

	// Generate CA certificate
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: config.Organization,
			CommonName:   "unified-replication-operator-ca",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(config.ValidityDuration),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	// Generate server private key
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server key: %w", err)
	}

	// Generate server certificate
	serverTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: config.Organization,
			CommonName:   config.CommonName,
		},
		DNSNames:    config.DNSNames,
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(config.ValidityDuration),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	// Parse CA cert for signing
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	serverCertDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create server certificate: %w", err)
	}

	// Encode certificates and keys to PEM
	certPEM := encodeCertificatePEM(serverCertDER)
	keyPEM := encodePrivateKeyPEM(serverKey)
	caPEM := encodeCertificatePEM(caCertDER)

	return &TLSCertificate{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		CAPEM:   caPEM,
	}, nil
}

// encodeCertificatePEM encodes a certificate to PEM format
func encodeCertificatePEM(certDER []byte) []byte {
	var buf bytes.Buffer
	pem.Encode(&buf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	return buf.Bytes()
}

// encodePrivateKeyPEM encodes a private key to PEM format
func encodePrivateKeyPEM(key *rsa.PrivateKey) []byte {
	var buf bytes.Buffer
	pem.Encode(&buf, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	return buf.Bytes()
}

// ValidateCertificate validates that a certificate is valid and not expired
func ValidateCertificate(certPEM []byte) error {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check expiration
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("certificate not yet valid (valid from %s)", cert.NotBefore)
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("certificate expired on %s", cert.NotAfter)
	}

	return nil
}

// GetCertificateExpiry returns the expiration time of a certificate
func GetCertificateExpiry(certPEM []byte) (time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return time.Time{}, fmt.Errorf("failed to decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert.NotAfter, nil
}

// IsCertificateExpiringSoon checks if certificate expires within duration
func IsCertificateExpiringSoon(certPEM []byte, duration time.Duration) (bool, error) {
	expiry, err := GetCertificateExpiry(certPEM)
	if err != nil {
		return false, err
	}

	return time.Until(expiry) < duration, nil
}
