package util

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

// CertificateConfig holds configuration for certificate generation
type CertificateConfig struct {
	Hosts    []string // Hostnames and IP addresses
	ValidFor time.Duration
}

// GenerateSelfSignedCertificate generates a self-signed certificate in memory.
// Returns a tls.Certificate ready for use with HTTP servers.
func GenerateSelfSignedCertificate(config CertificateConfig) (tls.Certificate, error) {
	// Set default validity period (100 years)
	if config.ValidFor == 0 {
		config.ValidFor = 100 * 365 * 24 * time.Hour
	}

	// Generate ECDSA private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Calculate serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate serial number: %w", err)
	}

	// Set up certificate template
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Scriptling Server"},
			CommonName:   "localhost",
		},
		NotBefore:             now,
		NotAfter:              now.Add(config.ValidFor),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add hosts to certificate
	for _, h := range config.Hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	// Add localhost defaults if no hosts specified
	if len(template.DNSNames) == 0 && len(template.IPAddresses) == 0 {
		template.DNSNames = []string{"localhost"}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	}

	// Generate certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Create tls.Certificate
	cert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
		Leaf:        &template,
	}

	return cert, nil
}

// GenerateSelfSignedCertificatePEM generates a self-signed certificate and returns
// the certificate and private key as PEM-encoded bytes.
func GenerateSelfSignedCertificatePEM(config CertificateConfig) (certPEM, keyPEM []byte, err error) {
	// Set default validity period (100 years)
	if config.ValidFor == 0 {
		config.ValidFor = 100 * 365 * 24 * time.Hour
	}

	// Generate ECDSA private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Calculate serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	// Set up certificate template
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Scriptling Server"},
			CommonName:   "localhost",
		},
		NotBefore:             now,
		NotAfter:              now.Add(config.ValidFor),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Add hosts to certificate
	for _, h := range config.Hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	// Add localhost defaults if no hosts specified
	if len(template.DNSNames) == 0 && len(template.IPAddresses) == 0 {
		template.DNSNames = []string{"localhost"}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	}

	// Generate certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Encode to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	// Encode private key to PEM
	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privBytes,
	})

	return certPEM, keyPEM, nil
}

// ParseAddress extracts host and port from an address string.
// Supports formats: "host:port", ":port", "host"
func ParseAddress(addr string) (host string, port string, err error) {
	// Default values
	host = "127.0.0.1"
	port = "8000"

	if addr == "" {
		return host, port, nil
	}

	// Check if it starts with :
	if strings.HasPrefix(addr, ":") {
		port = addr[1:]
		return host, port, nil
	}

	// Split host:port
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		// Maybe just a port or just a host
		if strings.Contains(addr, ":") {
			// IPv6 address without port
			host = addr
			return host, port, nil
		}
		// Try as port number
		if _, err := net.LookupPort("tcp", addr); err == nil {
			port = addr
			return host, port, nil
		}
		// Must be a hostname
		host = addr
		return host, port, nil
	}

	if h != "" {
		host = h
	}
	if p != "" {
		port = p
	}

	return host, port, nil
}

// GetCertificateHosts extracts hostnames/IPs from an address for certificate generation
func GetCertificateHosts(addr string) []string {
	host, _, err := ParseAddress(addr)
	if err != nil {
		return []string{"localhost"}
	}

	hosts := []string{"localhost"}

	// Add parsed host if it's not empty and not a wildcard
	if host != "" && host != "0.0.0.0" && host != "::" {
		hosts = append(hosts, host)
	}

	// Add common localhost addresses
	hosts = append(hosts, "127.0.0.1", "::1")

	return hosts
}
