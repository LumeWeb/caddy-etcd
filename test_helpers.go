package etcd

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// testHelper provides common test infrastructure for etcd integration testing.
// It manages test lifecycle including:
// - Setting up test etcd connections
// - Creating temporary directories and files
// - Cleaning up resources after tests
// - Generating test certificates for TLS testing
type testHelper struct {
	t      *testing.T        // Testing context
	client *clientv3.Client  // etcd client connection
	cfg    *ClusterConfig    // Test configuration
	tmpDir string            // Temporary directory for test files
}

// newTestHelper creates a new test helper with an etcd client
// newTestHelper creates a new test helper with an etcd client connection.
// It skips the test if etcd is not available.
// The caller must call cleanup() when done to release resources.
func newTestHelper(t *testing.T) *testHelper {
	cfg := &ClusterConfig{
		KeyPrefix: fmt.Sprintf("/caddy-test-%d", time.Now().UnixNano()),
		ServerIP:  []string{"http://127.0.0.1:2379"},
		Connection: ConnectionConfig(struct {
			DialTimeout      Duration
			KeepAliveTime    Duration
			KeepAliveTimeout Duration
			AutoSyncInterval Duration
			RequestTimeout   Duration
			RejectOldCluster bool
		}{
			DialTimeout:      Duration{10 * time.Second},
			KeepAliveTime:    Duration{30 * time.Second},
			KeepAliveTimeout: Duration{10 * time.Second},
			AutoSyncInterval: Duration{5 * time.Minute},
			RequestTimeout:   Duration{30 * time.Second},
		}),
	}

	cli, err := getClient(cfg)
	if err != nil {
		t.Skip("Failed to connect to etcd (is it running?):", err)
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "caddy-etcd-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	return &testHelper{
		t:      t,
		client: cli,
		cfg:    cfg,
		tmpDir: tmpDir,
	}
}

// cleanup removes test data and closes connections
// cleanup removes test data from etcd and cleans up resources.
// This includes:
// - Deleting test keys from etcd
// - Closing the client connection
// - Removing temporary directories
func (h *testHelper) cleanup() {
	if h == nil || h.client == nil {
		return
	}
	
	ctx := context.Background()
	if h.client != nil {
		_, err := h.client.Delete(ctx, h.cfg.KeyPrefix, clientv3.WithPrefix())
		if err != nil {
			h.t.Logf("Failed to cleanup test data: %v", err)
		}
	}

	h.client.Close()
	if h.tmpDir != "" {
		os.RemoveAll(h.tmpDir)
	}
}

// generateTestCerts creates temporary TLS certificates for testing.
// It generates a complete certificate chain including:
// - A self-signed CA certificate with RSA 2048-bit key
// - A client certificate signed by the CA with RSA 2048-bit key
// - All corresponding private keys
//
// The certificates are written to temporary files that are cleaned up
// when the test completes.
//
// Returns:
//   - certFile: Path to the client certificate PEM file
//   - keyFile: Path to the client private key PEM file
//   - caFile: Path to the CA certificate PEM file
//
// The caller must ensure cleanup() is called to remove the temporary files.
func (h *testHelper) generateTestCerts() (certFile, keyFile, caFile string) {
	// Generate CA cert
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		h.t.Fatalf("Failed to generate CA private key: %v", err)
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		h.t.Fatalf("Failed to create CA certificate: %v", err)
	}

	// Generate client cert
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName: "Test Client",
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		SubjectKeyId: []byte{1, 2, 3, 4},
	}

	clientPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		h.t.Fatalf("Failed to generate client private key: %v", err)
	}

	clientBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &clientPrivKey.PublicKey, caPrivKey)
	if err != nil {
		h.t.Fatalf("Failed to create client certificate: %v", err)
	}

	// Write certificates to files
	certFile = filepath.Join(h.tmpDir, "client.crt")
	keyFile = filepath.Join(h.tmpDir, "client.key")
	caFile = filepath.Join(h.tmpDir, "ca.crt")

	certOut, err := os.Create(certFile)
	if err != nil {
		h.t.Fatalf("Failed to create cert file: %v", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: clientBytes})
	certOut.Close()

	keyOut, err := os.Create(keyFile)
	if err != nil {
		h.t.Fatalf("Failed to create key file: %v", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientPrivKey)})
	keyOut.Close()

	caOut, err := os.Create(caFile)
	if err != nil {
		h.t.Fatalf("Failed to create CA file: %v", err)
	}
	pem.Encode(caOut, &pem.Block{Type: "CERTIFICATE", Bytes: caBytes})
	caOut.Close()

	return certFile, keyFile, caFile
}
