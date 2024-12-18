package etcd

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"os"
	"strings"
	"time"
)

func pipeline(commits []backoff.Operation, rollbacks []backoff.Operation, b backoff.BackOff) error {
	var err error
	for idx, commit := range commits {
		err = backoff.Retry(commit, b)
		if err != nil {
			for i := idx - 1; i >= 0; i-- {
				switch {
				case i >= len(rollbacks):
					continue
				default:
					if errR := backoff.Retry(rollbacks[i], b); errR != nil {
						err = errors.Wrapf(err, "error on rollback: %s", errR)
					}
				}
			}
			break
		}
	}
	return err
}

func getClient(c *ClusterConfig) (*clientv3.Client, error) {
	// Initialize logger if not already set
	if logger == nil {
		logger = zap.NewNop()
	}

	cfg := clientv3.Config{
		Endpoints:            c.ServerIP,
		DialTimeout:          c.Connection.DialTimeout.Duration,
		DialKeepAliveTime:    c.Connection.KeepAliveTime.Duration,
		DialKeepAliveTimeout: c.Connection.KeepAliveTimeout.Duration,
		AutoSyncInterval:     c.Connection.AutoSyncInterval.Duration,
		RejectOldCluster:     c.Connection.RejectOldCluster,
	}

	// Add TLS config if enabled
	if c.TLS.CertFile != "" {
		tlsConfig, err := createTLSConfig(c)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create TLS config")
		}
		cfg.TLS = tlsConfig
	}

	// Add auth if configured
	if c.Auth.Username != "" {
		cfg.Username = c.Auth.Username
		cfg.Password = c.Auth.Password
	}

	cli, err := clientv3.New(cfg)
	if err != nil {
		logger.Error("failed to create etcd client",
			zap.Strings("endpoints", c.ServerIP),
			zap.Error(err))
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return nil, ConnectionError{
				Endpoints: c.ServerIP,
				Err:       ErrNoConnection,
			}
		}
		return nil, ConnectionError{
			Endpoints: c.ServerIP,
			Err:       ErrClusterDown,
		}
	}

	// Verify connection with a ping
	ctx, cancel := context.WithTimeout(context.Background(), c.Connection.DialTimeout.Duration)
	defer cancel()

	if _, err := cli.Get(ctx, "ping"); err != nil {
		cli.Close()
		logger.Error("failed to verify etcd connection",
			zap.Strings("endpoints", c.ServerIP),
			zap.Error(err))
		return nil, ConnectionError{
			Endpoints: c.ServerIP,
			Err:       ErrNoConnection,
		}
	}

	logger.Debug("connected to etcd cluster",
		zap.Strings("endpoints", c.ServerIP))

	return cli, nil
}

// createTLSConfig creates a TLS configuration for etcd client
func createTLSConfig(c *ClusterConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		ServerName:         c.TLS.ServerName,
		InsecureSkipVerify: c.TLS.SkipVerify,
	}

	if c.TLS.CertFile != "" && c.TLS.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.TLS.CertFile, c.TLS.KeyFile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to load client cert/key pair")
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if c.TLS.CAFile != "" {
		caData, err := os.ReadFile(c.TLS.CAFile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to read CA file")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
		tlsConfig.RootCAs = pool
	}

	return tlsConfig, nil
}

func tx(txs ...backoff.Operation) []backoff.Operation {
	return txs
}

func get(cli *clientv3.Client, key string, dst *bytes.Buffer) backoff.Operation {
	return func() error {
		resp, err := cli.Get(context.Background(), key)
		if err != nil {
			return errors.Wrap(err, "failed to retrieve value from etcd")
		}

		if len(resp.Kvs) == 0 {
			return nil
		}

		b, err := base64.StdEncoding.DecodeString(string(resp.Kvs[0].Value))
		if err != nil {
			return errors.Wrap(err, "failed to decode base64 value from etcd")
		}

		if _, err := dst.Write(b); err != nil {
			return errors.Wrap(err, "failed to write etcd value to destination buffer")
		}
		return nil
	}
}

func set(cli *clientv3.Client, key string, value []byte) backoff.Operation {
	return func() error {
		encodedValue := base64.StdEncoding.EncodeToString(value)
		_, err := cli.Put(context.Background(), key, encodedValue)
		if err != nil {
			return errors.Wrap(err, "set: failed to set key value")
		}
		return nil
	}
}

func del(cli *clientv3.Client, key string) backoff.Operation {
	return func() error {
		_, err := cli.Delete(context.Background(), key)
		if err != nil {
			return errors.Wrapf(err, "del: failed to delete key: %s", key)
		}
		return nil
	}
}

func setMD(cli *clientv3.Client, key string, m Metadata) backoff.Operation {
	return func() error {
		jsdata, err := json.Marshal(m)
		if err != nil {
			return errors.Wrap(err, "setmd: failed to marshal metadata")
		}
		_, err = cli.Put(context.Background(), key, base64.StdEncoding.EncodeToString(jsdata))
		if err != nil {
			return errors.Wrap(err, "setmd: failed to set metadata value")
		}
		return nil
	}
}

func getMD(cli *clientv3.Client, key string, m *Metadata) backoff.Operation {
	return func() error {
		// Try direct file lookup first
		resp, err := cli.Get(context.Background(), key)
		if err != nil {
			return errors.Wrap(err, "getmd: failed to get metadata response")
		}
		if len(resp.Kvs) > 0 {
			return unmarshalMDv3(resp.Kvs[0].Value, m)
		}

		// Look for children by using the key as a prefix
		dirResp, err := cli.Get(context.Background(), key+"/", clientv3.WithPrefix())
		if err != nil {
			return errors.Wrap(err, "getmd: failed to get directory contents")
		}

		if len(dirResp.Kvs) == 0 {
			// Extract just the path part after the metadata prefix
			pathStart := strings.Index(key, "/md/")
			if pathStart == -1 {
				return NotExist{key}
			}
			relPath := key[pathStart+4:]
			return NotExist{relPath}
		}

		// It's a directory - aggregate metadata from children
		pathStart := strings.Index(key, "/md/")
		if pathStart != -1 {
			m.Path = key[pathStart+4:]
		} else {
			m.Path = key
		}
		m.IsDir = true
		m.Timestamp = time.Now().UTC()

		for _, kv := range dirResp.Kvs {
			md1 := new(Metadata)
			if err := unmarshalMDv3(kv.Value, md1); err != nil {
				continue
			}
			m.Size += md1.Size
			if md1.Timestamp.After(m.Timestamp) {
				m.Timestamp = md1.Timestamp
			}
		}
		return nil
	}
}

func unmarshalMDv3(value []byte, m *Metadata) error {
	if m == nil {
		return errors.New("unmarshalMD: metadata is nil")
	}
	bjson, err := base64.StdEncoding.DecodeString(string(value))
	if err != nil {
		return errors.Wrap(err, "getmd: failed to decode metadata")
	}
	if err := json.Unmarshal(bjson, m); err != nil {
		return errors.Wrap(err, "getmd: failed to unmarshal metadata response")
	}
	return nil
}

func noop() backoff.Operation {
	return func() error {
		return nil
	}
}

func exists(cli *clientv3.Client, key string, out *bool) backoff.Operation {
	return func() error {
		resp, err := cli.Get(context.Background(), key)
		if err != nil {
			return errors.Wrap(err, "exists: failed to check key")
		}
		*out = len(resp.Kvs) > 0
		return nil
	}
}

func list(cli *clientv3.Client, key string) ([]*mvccpb.KeyValue, error) {
	resp, err := cli.Get(context.Background(), key, clientv3.WithPrefix())
	if err != nil {
		return nil, errors.Wrap(err, "list: unable to get list")
	}
	return resp.Kvs, nil
}
