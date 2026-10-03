package mtls_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yasaswinpalukuri/groot-iam/pkg/mtls"
)

// ── Test certificate infrastructure ──────────────────────────────────────────

// testCerts holds file paths for certs written to a temp directory.
type testCerts struct {
	dir          string
	caCertFile   string
	srvCertFile  string
	srvKeyFile   string
	cliCertFile  string
	cliKeyFile   string
	// wrong CA — signed by a different root, not trusted by the main CA pool
	wrongCACertFile string
	wrongCLICertFile string
	wrongCLIKeyFile  string
}

// generateTestCerts creates a complete PKI in a temp directory:
//   - A root CA (Groot Test CA)
//   - A server cert signed by that CA
//   - A client cert signed by that CA
//   - A "wrong CA" and a client cert signed by the wrong CA
//
// WHY ECDSA instead of RSA?
// Faster key generation (important for tests), smaller keys, and ECDSA is the
// modern default. Our mTLS config accepts both RSA and ECDSA.
func generateTestCerts(t *testing.T) testCerts {
	t.Helper()
	dir := t.TempDir()

	// Generate root CA
	caKey, caCert, caCertPEM := mustGenerateCACert(t, "Groot Test CA")
	caCertFile := writePEM(t, dir, "ca.crt", "CERTIFICATE", caCert.Raw)

	// Server cert signed by our CA
	srvKey, srvCertDER := mustGenerateLeafCert(t, "groot-gateway", []string{"localhost"}, caKey, caCert)
	srvCertFile := writePEM(t, dir, "srv.crt", "CERTIFICATE", srvCertDER)
	srvKeyFile := writeECKeyPEM(t, dir, "srv.key", srvKey)

	// Client cert signed by our CA
	cliKey, cliCertDER := mustGenerateLeafCert(t, "agent-service", nil, caKey, caCert)
	cliCertFile := writePEM(t, dir, "cli.crt", "CERTIFICATE", cliCertDER)
	cliKeyFile := writeECKeyPEM(t, dir, "cli.key", cliKey)

	// Wrong CA + client cert signed by wrong CA
	wrongCAKey, wrongCACert, _ := mustGenerateCACert(t, "Wrong CA")
	wrongCACertFile := writePEM(t, dir, "wrong-ca.crt", "CERTIFICATE", wrongCACert.Raw)
	wrongCliKey, wrongCliCertDER := mustGenerateLeafCert(t, "evil-agent", nil, wrongCAKey, wrongCACert)
	wrongCLICertFile := writePEM(t, dir, "wrong-cli.crt", "CERTIFICATE", wrongCliCertDER)
	wrongCLIKeyFile := writeECKeyPEM(t, dir, "wrong-cli.key", wrongCliKey)

	_ = caCertPEM // used for documentation; suppress unused warning

	return testCerts{
		dir:             dir,
		caCertFile:      caCertFile,
		srvCertFile:     srvCertFile,
		srvKeyFile:      srvKeyFile,
		cliCertFile:     cliCertFile,
		cliKeyFile:      cliKeyFile,
		wrongCACertFile: wrongCACertFile,
		wrongCLICertFile: wrongCLICertFile,
		wrongCLIKeyFile:  wrongCLIKeyFile,
	}
}

// ── mTLS connection tests ─────────────────────────────────────────────────────

// TestValidClientCertIsAccepted verifies that a client presenting a cert
// signed by the trusted CA can complete the mTLS handshake.
func TestValidClientCertIsAccepted(t *testing.T) {
	certs := generateTestCerts(t)

	serverCfg, err := mtls.ServerTLSConfig(certs.srvCertFile, certs.srvKeyFile, certs.caCertFile)
	if err != nil {
		t.Fatalf("ServerTLSConfig: %v", err)
	}
	clientCfg, err := mtls.ClientTLSConfig(certs.cliCertFile, certs.cliKeyFile, certs.caCertFile)
	if err != nil {
		t.Fatalf("ClientTLSConfig: %v", err)
	}

	if err := tlsHandshake(t, serverCfg, clientCfg); err != nil {
		t.Errorf("valid client cert should be accepted: %v", err)
	}
}

// TestMissingClientCertIsRefused verifies that a client presenting NO certificate
// is rejected by the gateway. This is the core mTLS guarantee: unauthenticated
// clients cannot reach the backend.
func TestMissingClientCertIsRefused(t *testing.T) {
	certs := generateTestCerts(t)

	serverCfg, err := mtls.ServerTLSConfig(certs.srvCertFile, certs.srvKeyFile, certs.caCertFile)
	if err != nil {
		t.Fatalf("ServerTLSConfig: %v", err)
	}

	// Client config with NO client certificate (normal TLS, not mTLS)
	caCertPEM, _ := os.ReadFile(certs.caCertFile)
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caCertPEM)
	noCertClientCfg := &tls.Config{
		RootCAs:    caPool,
		MinVersion: tls.VersionTLS12,
	}

	if err := tlsHandshake(t, serverCfg, noCertClientCfg); err == nil {
		t.Error("client without cert should be refused")
	}
}

// TestWrongCAClientCertIsRefused verifies that a client cert signed by an
// untrusted CA is rejected. This prevents a compromised service from
// presenting a self-signed or third-party cert to impersonate agent_service.
func TestWrongCAClientCertIsRefused(t *testing.T) {
	certs := generateTestCerts(t)

	serverCfg, err := mtls.ServerTLSConfig(certs.srvCertFile, certs.srvKeyFile, certs.caCertFile)
	if err != nil {
		t.Fatalf("ServerTLSConfig: %v", err)
	}

	// Client presents cert from wrong CA — our CA pool does not trust it
	wrongClientCfg, err := mtls.ClientTLSConfig(certs.wrongCLICertFile, certs.wrongCLIKeyFile, certs.caCertFile)
	if err != nil {
		t.Fatalf("ClientTLSConfig (wrong CA): %v", err)
	}

	if err := tlsHandshake(t, serverCfg, wrongClientCfg); err == nil {
		t.Error("client cert from wrong CA should be refused")
	}
}

// TestWrongCAServerCertIsRefused verifies that the client refuses to connect
// to a server presenting a cert from an untrusted CA.
func TestWrongCAServerCertIsRefused(t *testing.T) {
	certs := generateTestCerts(t)

	// Server uses wrong CA cert (attacker's server)
	wrongServerCfg := &tls.Config{
		// Use client cert as a stand-in for a "wrong CA" server cert
		// The actual server cert doesn't matter — the client will reject
		// it because it chains to the wrong CA
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.NoClientCert,
	}
	// Load wrong-ca signed cert as server cert
	wrongSrvCert, err := tls.LoadX509KeyPair(certs.wrongCLICertFile, certs.wrongCLIKeyFile)
	if err != nil {
		t.Fatalf("load wrong server cert: %v", err)
	}
	wrongServerCfg.Certificates = []tls.Certificate{wrongSrvCert}

	// Client trusts only our CA — wrong server cert should be rejected
	clientCfg, err := mtls.ClientTLSConfig(certs.cliCertFile, certs.cliKeyFile, certs.caCertFile)
	if err != nil {
		t.Fatalf("ClientTLSConfig: %v", err)
	}

	if err := tlsHandshake(t, wrongServerCfg, clientCfg); err == nil {
		t.Error("server cert from wrong CA should be refused by client")
	}
}

// TestMinTLSVersion verifies that the server refuses TLS 1.1 connections.
func TestMinTLSVersion(t *testing.T) {
	certs := generateTestCerts(t)

	serverCfg, err := mtls.ServerTLSConfig(certs.srvCertFile, certs.srvKeyFile, certs.caCertFile)
	if err != nil {
		t.Fatalf("ServerTLSConfig: %v", err)
	}

	// Client attempts TLS 1.1 (below our minimum)
	caCertPEM, _ := os.ReadFile(certs.caCertFile)
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caCertPEM)
	clientCert, _ := tls.LoadX509KeyPair(certs.cliCertFile, certs.cliKeyFile)
	oldClientCfg := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		MaxVersion:   tls.VersionTLS11, // force TLS 1.1
	}

	if err := tlsHandshake(t, serverCfg, oldClientCfg); err == nil {
		t.Error("TLS 1.1 should be refused (minimum is TLS 1.2)")
	}
}

// ── LoadCertPool tests ────────────────────────────────────────────────────────

func TestLoadCertPoolValidFile(t *testing.T) {
	certs := generateTestCerts(t)
	pool, err := mtls.LoadCertPool(certs.caCertFile)
	if err != nil {
		t.Fatalf("LoadCertPool: %v", err)
	}
	if pool == nil {
		t.Error("expected non-nil pool")
	}
}

func TestLoadCertPoolMissingFile(t *testing.T) {
	_, err := mtls.LoadCertPool("/nonexistent/ca.crt")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadCertPoolEmptyFile(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.crt")
	os.WriteFile(empty, []byte{}, 0644)
	_, err := mtls.LoadCertPool(empty)
	if err == nil {
		t.Error("expected error for empty cert file")
	}
}

// ── TLS handshake helper ──────────────────────────────────────────────────────

// tlsHandshake starts a TLS server with serverCfg, dials it with clientCfg,
// and returns nil if the handshake succeeds or an error if it fails.
// Uses real net.Listen + tls.Dial — this tests the actual TLS stack, not mocks.
func tlsHandshake(t *testing.T, serverCfg, clientCfg *tls.Config) error {
	t.Helper()

	ln, err := tls.Listen("tcp", "127.0.0.1:0", serverCfg)
	if err != nil {
		t.Fatalf("tls.Listen: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	errCh := make(chan error, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- nil // listener closed normally
			return
		}
		defer conn.Close()
		if err := conn.(*tls.Conn).Handshake(); err != nil {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	conn, err := tls.Dial("tcp", addr, clientCfg)
	if err != nil {
		<-errCh // drain server goroutine
		return err
	}
	defer conn.Close()

	if err := conn.Handshake(); err != nil {
		<-errCh
		return err
	}

	return <-errCh
}

// ── Certificate generation helpers ───────────────────────────────────────────

func mustGenerateCACert(t *testing.T, cn string) (*ecdsa.PrivateKey, *x509.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	cert, _ := x509.ParseCertificate(der)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return key, cert, pemBytes
}

func mustGenerateLeafCert(t *testing.T, cn string, dnsNames []string, caKey *ecdsa.PrivateKey, caCert *x509.Certificate) (*ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     dnsNames,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf cert %q: %v", cn, err)
	}
	return key, der
}

func writePEM(t *testing.T, dir, name, pemType string, der []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	pem.Encode(f, &pem.Block{Type: pemType, Bytes: der})
	return path
}

func writeECKeyPEM(t *testing.T, dir, name string, key *ecdsa.PrivateKey) string {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal EC key: %v", err)
	}
	return writePEM(t, dir, name, "EC PRIVATE KEY", der)
}
