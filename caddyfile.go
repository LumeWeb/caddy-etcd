package etcd

import (
	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"strings"
)

func init() {
	caddy.RegisterModule(Cluster{})
}

// UnmarshalCaddyfile implements caddyfile.Unmarshaler. Syntax:
//
//	storage etcd {
//	    prefix <key_prefix>
//	    endpoints {
//	        <endpoint1>
//	        <endpoint2>
//	        ...
//	    }
//	    timeout <duration>
//	    caddyfile <path>
//	    disable_caddyfile_load
//	    auth {
//	        username <username>
//	        password <password>
//	    }
//	    tls {
//	        cert <path>
//	        key <path>
//	        ca <path>
//	        server_name <name>
//	        insecure_skip_verify
//	    }
//	    connection {
//	        dial_timeout <duration>
//	        keepalive_time <duration>
//	        keepalive_timeout <duration>
//	        auto_sync_interval <duration>
//	        request_timeout <duration>
//	        reject_old_cluster <bool>
//	    }
//	}
func (c *Cluster) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	// First token should be present
	if !d.Next() {
		return d.ArgErr()
	}

	// No additional arguments expected on the same line as 'storage etcd'
	if d.NextArg() {
		return d.ArgErr()
	}

	// Initialize config with defaults
	cfg, err := NewClusterConfig()
	if err != nil {
		return err
	}
	c.cfg = cfg

	// Process the configuration block
	for nesting := d.Nesting(); d.NextBlock(nesting); {
		configKey := d.Val()

		switch configKey {
		case "prefix":
			if !d.NextArg() {
				return d.ArgErr()
			}
			if err := WithPrefix(d.Val())(c.cfg); err != nil {
				return err
			}

		case "endpoints":
			var endpoints []string
			for nesting := d.Nesting(); d.NextBlock(nesting); {
				endpoints = append(endpoints, d.Val())
			}
			if len(endpoints) == 0 {
				return d.Errf("no endpoints specified")
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
			for nesting := d.Nesting(); d.NextBlock(nesting); {
				switch d.Val() {
				case "username":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithUsername(d.Val())(c.cfg); err != nil {
						return err
					}
				case "password":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithPassword(d.Val())(c.cfg); err != nil {
						return err
					}
				default:
					return d.Errf("unknown auth option: %s", d.Val())
				}
			}

		case "tls":
			for nesting := d.Nesting(); d.NextBlock(nesting); {
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
					return d.Errf("unknown tls option: %s", d.Val())
				}
			}

		case "connection":
			for nesting := d.Nesting(); d.NextBlock(nesting); {
				switch d.Val() {
				case "dial_timeout":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithDialTimeout(d.Val())(c.cfg); err != nil {
						return err
					}
				case "keepalive_time":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithKeepAliveTime(d.Val())(c.cfg); err != nil {
						return err
					}
				case "keepalive_timeout":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithKeepAliveTimeout(d.Val())(c.cfg); err != nil {
						return err
					}
				case "auto_sync_interval":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithAutoSyncInterval(d.Val())(c.cfg); err != nil {
						return err
					}
				case "request_timeout":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithRequestTimeout(d.Val())(c.cfg); err != nil {
						return err
					}
				case "reject_old_cluster":
					if !d.NextArg() {
						return d.ArgErr()
					}
					if err := WithRejectOldCluster(d.Val())(c.cfg); err != nil {
						return err
					}
				default:
					return d.Errf("unknown connection option: %s", d.Val())
				}
			}

		default:
			return d.Errf("unknown directive: %s", configKey)
		}
	}
	return nil
}
