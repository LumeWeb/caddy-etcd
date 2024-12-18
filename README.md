# caddy-etcd

[![Build Status](https://travis-ci.com/BTBurke/caddy-etcd.svg?branch=master)](https://travis-ci.com/BTBurke/caddy-etcd)  <a href="https://godoc.org/github.com/BTBurke/caddy-etcd"><img src="https://img.shields.io/badge/godoc-reference-blue.svg"></a>

This is a clustering plugin for Caddy that will store Caddy-managed certificates and any other assets in etcd rather than the filesystem.  It implements a virtual filesystem on top of etcd storage in order to allow multiple instances of caddy to share configuration information and TLS certificates without having to share a filesystem.  You must have already set up your own etcd cluster for this plugin to work.

![Beta Quality](https://user-images.githubusercontent.com/414599/53683937-62878b80-3cdd-11e9-9b78-daa5ddb02bcd.png)

## Configuration

Caddy clustering plugins are enabled and configured through environment variables.  The table below lists the available options, but **to enable this plugin
you must first set `CADDY_CLUSTERING="etcd"`.**


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

- [x] Distributed certificate storage and sharing
- [x] Mutual TLS authentication support
- [x] Username/password authentication
- [x] Automatic connection management and failover
- [x] Distributed locking with configurable timeouts
- [x] Atomic transactions for data consistency
- [x] Configurable connection parameters
- [x] Comprehensive test coverage
- [ ] Dynamic Caddyfile configuration updates
