package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// CertificateRequest contains the values needed to generate a certificate.
type CertificateRequest struct {
	CommonName   string
	Organization string
	Country      string
	ValidityDays int
	KeySize      int
}

// CertificateResult contains the generated PEM values.
type CertificateResult struct {
	CertificatePEM string
	PrivateKeyPEM  string
}

// GenerateSelfSignedCertificate generates an RSA private key and a
// self-signed X.509 certificate.
func GenerateSelfSignedCertificate(
	request CertificateRequest,
) (*CertificateResult, error) {
	if request.CommonName == "" {
		return nil, errors.New("common name cannot be empty")
	}

	if request.Organization == "" {
		return nil, errors.New("organization cannot be empty")
	}

	if request.Country == "" {
		return nil, errors.New("country cannot be empty")
	}

	if request.ValidityDays <= 0 {
		return nil, errors.New("validity days must be greater than zero")
	}

	if request.KeySize != 2048 && request.KeySize != 3072 && request.KeySize != 4096 {
		return nil, fmt.Errorf(
			"unsupported RSA key size %d: use 2048, 3072, or 4096",
			request.KeySize,
		)
	}

	// Generate the RSA private key.
	privateKey, err := rsa.GenerateKey(rand.Reader, request.KeySize)
	if err != nil {
		return nil, fmt.Errorf("generate RSA private key: %w", err)
	}

	// Generate a random certificate serial number.
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial number: %w", err)
	}

	now := time.Now().UTC()

	// Define the certificate contents.
	certificateTemplate := x509.Certificate{
		SerialNumber: serialNumber,

		Subject: pkix.Name{
			CommonName:   request.CommonName,
			Organization: []string{request.Organization},
			Country:      []string{request.Country},
		},

		NotBefore: now.Add(-5 * time.Minute),
		NotAfter:  now.AddDate(0, 0, request.ValidityDays),

		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment,

		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},

		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// Because this is self-signed, the certificate is both the child and parent.
	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		&certificateTemplate,
		&certificateTemplate,
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("create self-signed certificate: %w", err)
	}

	certificatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certificateDER,
	})

	if certificatePEM == nil {
		return nil, errors.New("encode certificate as PEM")
	}

	privateKeyDER := x509.MarshalPKCS1PrivateKey(privateKey)

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyDER,
	})

	if privateKeyPEM == nil {
		return nil, errors.New("encode private key as PEM")
	}

	return &CertificateResult{
		CertificatePEM: string(certificatePEM),
		PrivateKeyPEM:  string(privateKeyPEM),
	}, nil
}
