package etcd

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestTLSConnection(t *testing.T) {
	if !shouldRunIntegration() {
		t.Skip("no etcd server found, skipping")
	}

	h := newTestHelper(t)
	defer h.cleanup()

	// Generate test certificates
	certFile, keyFile, caFile := h.generateTestCerts()

	// Create config with TLS
	cfg := &ClusterConfig{
		KeyPrefix: h.cfg.KeyPrefix,
		ServerIP:  []string{"https://127.0.0.1:2379"},
		TLS: TLSConfig(struct {
			CertFile   string
			KeyFile    string
			CAFile     string
			ServerName string
			SkipVerify bool
		}{
			CertFile:   certFile,
			KeyFile:    keyFile,
			CAFile:     caFile,
			ServerName: "Test Client",
		}),
	}

	// Test TLS connection
	cli, err := getClient(cfg)
	if err != nil {
		// Skip if etcd server doesn't support TLS
		if _, ok := err.(ConnectionError); ok {
			t.Skip("etcd server does not support TLS")
		}
		t.Fatal(err)
	}
	defer func(cli *clientv3.Client) {
		err := cli.Close()
		if err != nil {
			t.Fatal(err)
		}
	}(cli)

	// Verify connection works
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = cli.Get(ctx, "test_key")
	assert.NoError(t, err)
}

func TestHelperCleanup(t *testing.T) {
	if !shouldRunIntegration() {
		t.Skip("no etcd server found, skipping")
	}

	h := newTestHelper(t)

	// Create some test data
	testKey := h.cfg.KeyPrefix + "/test_cleanup"
	testValue := []byte("test data")

	_, err := h.client.Put(context.Background(), testKey, string(testValue))
	assert.NoError(t, err)

	// Verify data exists
	resp, err := h.client.Get(context.Background(), testKey)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(resp.Kvs))

	// Run cleanup
	h.cleanup()

	// Create new client to verify cleanup
	cfg := clientv3.Config{
		Endpoints:   []string{"http://127.0.0.1:2379"},
		DialTimeout: 2 * time.Second,
	}
	cli, err := clientv3.New(cfg)
	assert.NoError(t, err)
	defer func(cli *clientv3.Client) {
		err := cli.Close()
		if err != nil {
			t.Fatal(err)
		}
	}(cli)

	// Verify data was cleaned up
	resp, err = cli.Get(context.Background(), testKey)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(resp.Kvs))

	// Verify temp directory was cleaned up
	_, err = os.Stat(h.tmpDir)
	assert.True(t, os.IsNotExist(err))
}
