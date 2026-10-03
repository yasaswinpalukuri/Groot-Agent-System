// Package mtls provides TLS and mTLS configuration helpers for the Groot IAM gateway.
//
// Why mTLS between gateway and agent_service?
// Regular TLS authenticates the server to the client — your browser verifies the
// gateway's certificate. mTLS requires BOTH sides to present certificates. Even
// if an attacker gets inside the Docker network (container escape, misconfigured
// firewall), they cannot call agent_service without a valid X.509 certificate
// signed by Groot's private CA. No password, no API key — the cert IS the
// credential.
//
// Why TLS 1.2 minimum and not 1.3 only?
// TLS 1.3 is preferred (Go's crypto/tls selects it automatically when both sides
// support it). TLS 1.2 minimum is the PCI-DSS requirement and covers clients
// that haven't upgraded. Below 1.2 (SSL 3.0, TLS 1.0, TLS 1.1) are broken and
// never acceptable.
//
// Why AEAD-only cipher suites?
// CBC mode cipher suites are vulnerable to BEAST (TLS 1.0) and Lucky13 timing
// attacks. AEAD (AES-GCM, ChaCha20-Poly1305) provides authenticated encryption —
// the MAC covers the ciphertext, not plaintext, eliminating the padding oracle
// attack surface. We explicitly list AEAD suites to ensure a future Go upgrade
// that adds a CBC suite doesn't silently weaken our config.
package mtls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// ServerTLSConfig returns a *tls.Config for the IAM gateway server.
// It requires client certificates signed by the provided CA — this is the
// mTLS configuration that agent_service must satisfy.
//
// certFile/keyFile: the gateway's server certificate and private key
// caFile: the CA certificate used to verify client certificates
func ServerTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("mtls: load server certificate %q: %w", certFile, err)
	}

	clientCA, err := LoadCertPool(caFile)
	if err != nil {
		return nil, fmt.Errorf("mtls: load client CA %q: %w", caFile, err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},

		// RequireAndVerifyClientCert: the server requests a client cert AND verifies
		// it against ClientCAs. A missing or invalid cert aborts the handshake.
		// This is the setting that makes it mTLS rather than regular TLS.
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCA,

		MinVersion: tls.VersionTLS12,

		// CurvePreferences: prefer X25519 (fastest, constant-time, no cache timing
		// attack risk) then P-256 (widely supported). Exclude P-384/P-521 — they
		// are slower with no practical security gain at this key size.
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},

		// CipherSuites applies only to TLS 1.2; TLS 1.3 cipher suites are
		// non-configurable in Go (the three TLS 1.3 suites are all AEAD and
		// always enabled). We list AEAD suites only — no CBC, no RC4, no 3DES.
		CipherSuites: aead12CipherSuites(),
	}, nil
}

// ClientTLSConfig returns a *tls.Config for agent_service (or any service that
// must present a client certificate to the gateway).
//
// certFile/keyFile: the client's certificate and private key
// caFile: the CA certificate used to verify the gateway's server certificate
func ClientTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("mtls: load client certificate %q: %w", certFile, err)
	}

	serverCA, err := LoadCertPool(caFile)
	if err != nil {
		return nil, fmt.Errorf("mtls: load server CA %q: %w", caFile, err)
	}

	return &tls.Config{
		// Certificates: the client cert the gateway's ServerTLSConfig will verify
		Certificates: []tls.Certificate{cert},
		// RootCAs: only trust the gateway's cert if it chains to Groot's CA
		RootCAs:    serverCA,
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
	}, nil
}

// LoadCertPool reads a PEM-encoded CA certificate file and returns an
// *x509.CertPool. Returns an error if the file contains no valid certificates.
func LoadCertPool(caFile string) (*x509.CertPool, error) {
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", caFile, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("no valid PEM certificates in %q", caFile)
	}
	return pool, nil
}

// aead12CipherSuites returns the list of TLS 1.2 AEAD cipher suites we accept.
// These provide authenticated encryption — the tag covers the ciphertext,
// eliminating padding oracle attacks (Lucky13, BEAST).
func aead12CipherSuites() []uint16 {
	return []uint16{
		// ECDHE + AES-256-GCM (strongest)
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		// ECDHE + AES-128-GCM (faster on hardware without AES-NI)
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		// ECDHE + ChaCha20-Poly1305 (preferred on ARM / mobile without AES-NI)
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	}
}
