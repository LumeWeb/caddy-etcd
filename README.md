# caddy-etcd [DEPRECATED]

[![Build Status](https://travis-ci.com/BTBurke/caddy-etcd.svg?branch=master)](https://travis-ci.com/BTBurke/caddy-etcd)  <a href="https://godoc.org/github.com/BTBurke/caddy-etcd"><img src="https://img.shields.io/badge/godoc-reference-blue.svg"></a>

> **DEPRECATION NOTICE**: This etcd storage backend is deprecated and not recommended for production use. The implementation introduces significant operational complexity and potential reliability challenges. For clustered storage needs, please consider using the S3 storage backend ([github.com/techknowlogick/certmagic-s3](https://github.com/techknowlogick/certmagic-s3)) which provides better reliability and simpler operations while meeting Caddy's distributed storage requirements.

This was a clustering plugin for Caddy that stored Caddy-managed certificates and other assets in etcd rather than the filesystem. It implemented a virtual filesystem on top of etcd storage to allow multiple Caddy instances to share configuration information and TLS certificates without sharing a filesystem.

## Configuration

Caddy clustering plugins can be configured through environment variables or JSON configuration. To enable this plugin, you must set `CADDY_CLUSTERING="etcd"` when using environment variables, or include the appropriate JSON configuration.

### Environment Variables Configuration

The table below lists the available environment variable options:


| Environment Variable | Function | Default |
| --- | --- | ---|
| CADDY_CLUSTERING_ETCD_SERVERS | Comma or semicolon separated list of etcd servers (full URLs required, e.g.: https://127.0.0.1:2379) | http://127.0.0.1:2379 |
| CADDY_CLUSTERING_ETCD_PREFIX | Key prefix for Caddy-managed resources in etcd | /caddy |
| CADDY_CLUSTERING_ETCD_TIMEOUT | Lock timeout for Caddy resources (Go duration format: 5m, 30s) | 5m |
| CADDY_CLUSTERING_ETCD_CADDYFILE | Path to local Caddyfile for initial cluster configuration | |
| CADDY_CLUSTERING_ETCD_CADDYFILE_LOADER | Enable/disable Caddyfile loading from etcd | enable |

### TLS Configuration
| Environment Variable | Function | Default |
| --- | --- | ---|
| CADDY_CLUSTERING_ETCD_TLS_CERT | Path to TLS client certificate | |
| CADDY_CLUSTERING_ETCD_TLS_KEY | Path to TLS client key | |
| CADDY_CLUSTERING_ETCD_TLS_CA | Path to CA certificate for server verification | |
| CADDY_CLUSTERING_ETCD_TLS_SERVER_NAME | Expected server name for verification | |
| CADDY_CLUSTERING_ETCD_TLS_SKIP_VERIFY | Skip TLS verification (not recommended) | false |

### Authentication
| Environment Variable | Function | Default |
| --- | --- | ---|
| CADDY_CLUSTERING_ETCD_USERNAME | Username for etcd authentication | |
| CADDY_CLUSTERING_ETCD_PASSWORD | Password for etcd authentication | |

### Connection Settings
| Environment Variable | Function | Default |
| --- | --- | ---|
| CADDY_CLUSTERING_ETCD_DIAL_TIMEOUT | Timeout for establishing connection | 5s |
| CADDY_CLUSTERING_ETCD_KEEPALIVE_TIME | Interval between keepalive probes | 5s |
| CADDY_CLUSTERING_ETCD_KEEPALIVE_TIMEOUT | Time to wait for keepalive response | 5s |
| CADDY_CLUSTERING_ETCD_AUTO_SYNC_INTERVAL | Interval for endpoint auto-sync | 5m |
| CADDY_CLUSTERING_ETCD_REQUEST_TIMEOUT | Timeout for individual requests | 5s |

### JSON Configuration

The plugin can be configured using Caddy's JSON config format. Here's an example:

```json
{
  "storage": {
    "module": "etcd",
    "config": {
      "endpoints": ["https://127.0.0.1:2379"],
      "key_prefix": "/caddy",
      "lock_timeout": "5m",
      "tls": {
        "cert_file": "/path/to/cert.pem",
        "key_file": "/path/to/key.pem",
        "ca_file": "/path/to/ca.pem",
        "server_name": "etcd.example.com"
      },
      "auth": {
        "username": "etcd_user",
        "password": "etcd_password"
      },
      "connection": {
        "dial_timeout": "5s",
        "keepalive_time": "5s",
        "request_timeout": "5s"
      }
    }
  }
}
```

### Caddyfile Examples

While the plugin itself is configured through environment variables or JSON, here are some example Caddyfile configurations that work well with etcd clustering:

```caddyfile
# Basic HTTPS server with automatic certificates
https://example.com {
    respond "Hello from clustered Caddy!"
}

# Multiple domains sharing certificates
(common) {
    tls {
        client_auth {
            mode require
            trusted_ca_cert_file /path/to/ca.pem
        }
    }
}

https://site1.example.com {
    import common
    respond "Site 1"
}

https://site2.example.com {
    import common
    respond "Site 2"
}
```

## Breaking Changes

### Version 2.0
- Removed support for legacy environment variable names
- Changed default connection timeout from 3s to 5s
- TLS configuration now requires both cert and key files when enabled
- Changed key prefix format in etcd storage

## Building Caddy with this Plugin

This plugin requires caddy to be built with go modules.  **It cannot be built by the build server on caddyserver.com because it currently lacks module support.**  

This project uses [mage](https://github.com/magefile/mage) to build the caddy binary.  To build, first download mage:

```
go get -u github.com/magefile/mage
```

You must have the following binaries available on your system to run the build:

```
go >= 1.11
sed
git
```

Then build by running

```
mage build
```

See the `magefile.go` for other customizations, such as including other plugins in your custom build.

## Testing

The project includes both unit tests and integration tests:

### Unit Tests
```bash
go test -v -cover -race -mod vendor
```

### Integration Tests
Integration tests require Docker and include:
- Automatic test environment setup/teardown
- Single node and cluster testing
- TLS and authentication testing
- Failure scenario testing

To run integration tests:
```bash
./test-docker.sh detach && \
go test -v -cover -race -mod vendor -tags=integration && \
docker stop etcd
```

## Features

Current Features:
- [x] Distributed certificate storage and sharing
- [x] Mutual TLS authentication support
- [x] Username/password authentication
- [x] Automatic connection management and failover
- [x] Distributed locking with configurable timeouts
- [x] Atomic transactions for data consistency
- [x] Configurable connection parameters
- [x] Comprehensive test coverage
- [x] JSON configuration support
- [x] Environment variable configuration
- [x] Automatic endpoint discovery
- [x] High availability support
- [x] Metrics and monitoring support

Planned Features:
- [ ] Dynamic Caddyfile configuration updates
- [ ] Automatic backup and restore
- [ ] Enhanced monitoring and alerting
- [ ] Rate limiting and circuit breaking
- [ ] Custom storage policies
