// Package etcd implements a distributed storage backend for Caddy using etcd.
// It provides cluster-wide synchronization of TLS certificates and other Caddy assets
// using etcd as the distributed storage layer.
package etcd

import (
	"context"
	"fmt"
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/certmagic"
	"go.etcd.io/etcd/api/v3/mvccpb"
	"go.uber.org/zap"
	"log"
	"strings"
)

// logger provides structured logging using Caddy's logger
var logger *zap.Logger
var cluster *Cluster

// Interface guards
var (
	_ caddy.StorageConverter = (*Cluster)(nil)
	_ certmagic.Storage      = (*Cluster)(nil)
	_ caddyfile.Unmarshaler  = (*Cluster)(nil)
	_ caddy.Provisioner      = (*Cluster)(nil)
	_ caddy.Validator        = (*Cluster)(nil)
)

// Cluster implements certmagic.Storage using etcd as the storage backend.
// It provides distributed locking, atomic operations, and consistent storage
// across multiple Caddy instances in a cluster.
type Cluster struct {
	srv Service        // The etcd service interface
	cfg *ClusterConfig // Configuration for etcd connection and behavior
}

// CaddyModule returns the Caddy module information.
func (Cluster) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "caddy.storage.etcd",
		New: func() caddy.Module {
			if cluster == nil {
				opts, err := ConfigOptsFromEnvironment()
				if err != nil {
					log.Fatal("failed to get config from environment", err)
				}

				c, err := NewClusterConfig(opts...)
				if err != nil {
					log.Fatal("failed to create cluster config", err)
				}

				cluster = &Cluster{cfg: c}
			}
			return cluster
		},
	}
}

// Provision sets up the storage backend.
func (c *Cluster) Provision(ctx caddy.Context) error {
	logger = ctx.Logger()

	// If no configuration exists yet, try environment
	if c.cfg == nil {
		opts, err := ConfigOptsFromEnvironment()
		if err != nil {
			logger.Error("failed to get config from environment",
				zap.Error(err))
			return err
		}

		cfg, err := NewClusterConfig(opts...)
		if err != nil {
			logger.Error("failed to create cluster config",
				zap.Error(err))
			return err
		}
		c.cfg = cfg
	}

	// Create service using existing config
	srv, err := NewService(c.cfg)
	if err != nil {
		logger.Error("failed to create etcd service",
			zap.Error(err))
		return err
	}

	c.srv = srv
	logger.Info("etcd storage backend provisioned",
		zap.String("prefix", c.cfg.KeyPrefix),
		zap.Strings("endpoints", c.cfg.ServerIP))
	return nil
}

// CertMagicStorage converts c to a certmagic.Storage instance.
func (c Cluster) CertMagicStorage() (certmagic.Storage, error) {
	return c, nil
}

// Validate implements caddy.Validator and validates the configuration.
func (c *Cluster) Validate() error {
	if c.srv == nil {
		return fmt.Errorf("etcd service not initialized")
	}
	return nil
}

// Lock acquires a lock at the given key.
func (c Cluster) Lock(_ context.Context, key string) error {
	return c.srv.Lock(key)
}

// Unlock releases the lock at the given key.
func (c Cluster) Unlock(_ context.Context, key string) error {
	return c.srv.Unlock(key)
}

// Store saves the given value at the given key.
func (c Cluster) Store(_ context.Context, key string, value []byte) error {
	return c.srv.Store(key, value)
}

// Load retrieves the value at the given key.
func (c Cluster) Load(_ context.Context, key string) ([]byte, error) {
	return c.srv.Load(key)
}

// Delete deletes the value at the given key.
func (c Cluster) Delete(_ context.Context, key string) error {
	return c.srv.Delete(key)
}

// Exists returns true if the key exists and is accessible.
func (c Cluster) Exists(_ context.Context, key string) bool {
	_, err := c.srv.Metadata(key)
	return err == nil
}

// List returns all keys that match prefix.
func (c Cluster) List(_ context.Context, prefix string, recursive bool) ([]string, error) {
	// Normalize the prefix to always start with /
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	// Trim any trailing slashes for consistency
	prefix = strings.TrimSuffix(prefix, "/")

	var filters []func(*mvccpb.KeyValue) bool
	if recursive {
		filters = append(filters, FilterRemoveDirectories())
	} else {
		// For non-recursive listing, ensure prefix has trailing slash for exact matching
		searchPrefix := prefix + "/"
		filters = append(filters, FilterExactPrefix(searchPrefix, c.srv.prefix()))
	}

	return c.srv.List(prefix, filters...)
}

// Stat returns information about the given key.
func (c Cluster) Stat(_ context.Context, key string) (certmagic.KeyInfo, error) {
	md, err := c.srv.Metadata(key)
	if err != nil {
		return certmagic.KeyInfo{}, err
	}
	return certmagic.KeyInfo{
		Key:        md.Path,
		Modified:   md.Timestamp,
		Size:       int64(md.Size),
		IsTerminal: !md.IsDir,
	}, nil
}
