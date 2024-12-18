package etcd

import (
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"strings"
)

// UnmarshalCaddyfile implements caddyfile.Unmarshaler. Syntax:
//
//	storage etcd {
//	    prefix <key_prefix>
//	    endpoints <endpoint1> [<endpoint2>...]
//	    timeout <duration>
//	    caddyfile <path>
//	    disable_caddyfile_load
//	}
//
// JSON configuration example:
//
//	{
//	    "storage": {
//	        "module": "etcd",
//	        "key_prefix": "/caddy",
//	        "endpoints": ["http://localhost:2379"],
//	        "lock_timeout": "5m",
//	        "tls": {
//	            "cert_file": "/path/to/cert.pem",
//	            "key_file": "/path/to/key.pem",
//	            "ca_file": "/path/to/ca.pem",
//	            "server_name": "etcd.example.com",
//	            "skip_verify": false
//	        },
//	        "auth": {
//	            "username": "user",
//	            "password": "pass"
//	        },
//	        "connection": {
//	            "dial_timeout": "5s",
//	            "keepalive_time": "30s",
//	            "keepalive_timeout": "10s",
//	            "auto_sync_interval": "5m",
//	            "request_timeout": "10s",
//	            "reject_old_cluster": true
//	        }
//	    }
//	}
func (c *Cluster) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // skip storage token
	d.Next() // skip etcd token

	// Initialize config with defaults
	cfg, err := NewClusterConfig()
	if err != nil {
		return err
	}
	c.cfg = cfg

	for d.NextBlock(0) {
		switch d.Val() {
		case "prefix":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if err := WithPrefix(d.Val())(c.cfg); err != nil {
				return err
			}

		case "endpoints":
			if !d.NextArg() {
				return d.ArgErr()
			}
			endpoints := d.RemainingArgs()
			if len(endpoints) == 0 {
				endpoints = []string{d.Val()}
			}
			if err := WithServers(strings.Join(endpoints, ","))(c.cfg); err != nil {
				return err
			}

		case "timeout":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if err := WithTimeout(d.Val())(c.cfg); err != nil {
				return err
			}

		case "caddyfile":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if err := WithCaddyFile(d.Val())(c.cfg); err != nil {
				return err
			}

		case "disable_caddyfile_load":
			if err := WithDisableCaddyfileLoad("disable")(c.cfg); err != nil {
				return err
			}

		case "auth":
			if !d.NextArg() {
				return d.ArgErr()
			}
			username := d.Val()
			if !d.NextArg() {
				return d.ArgErr()
			}
			password := d.Val()
			if err := WithUsername(username)(c.cfg); err != nil {
				return err
			}
			if err := WithPassword(password)(c.cfg); err != nil {
				return err
			}

		case "tls":
			for d.NextBlock(1) {
				switch d.Val() {
				case "cert":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithTLSCert(d.Val())(c.cfg); err != nil {
						return err
					}
				case "key":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithTLSKey(d.Val())(c.cfg); err != nil {
						return err
					}
				case "ca":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithTLSCA(d.Val())(c.cfg); err != nil {
						return err
					}
				case "server_name":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithTLSServerName(d.Val())(c.cfg); err != nil {
						return err
					}
				case "insecure_skip_verify":
					if err := WithTLSSkipVerify("true")(c.cfg); err != nil {
						return err
					}
				default:
					return d.Errf("unknown tls option %s", d.Val())
				}
			}

		default:
			return d.Errf("unknown subdirective %s", d.Val())
		}
	}

	return nil
}
