package etcd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("invalid duration")
	}
	return nil
}

// ConnectionConfig holds etcd connection settings
type ConnectionConfig struct {
	DialTimeout      Duration `json:"dial_timeout,string,omitempty"`
	KeepAliveTime    Duration `json:"keepalive_time,string,omitempty"`
	KeepAliveTimeout Duration `json:"keepalive_timeout,string,omitempty"`
	AutoSyncInterval Duration `json:"auto_sync_interval,string,omitempty"`
	RequestTimeout   Duration `json:"request_timeout,string,omitempty"`
	RejectOldCluster bool     `json:"reject_old_cluster,omitempty"`
}

// ClusterConfig maintains configuration for connecting to and interacting with etcd.
// It includes settings for:
//   - Server endpoints and key prefixes
//   - TLS security and authentication
//   - Connection timeouts and keepalive settings
//   - Lock timeouts and operational parameters
//   - Optional Caddyfile loading configuration
type TLSConfig struct {
	CertFile   string `json:"cert_file,omitempty"`
	KeyFile    string `json:"key_file,omitempty"`
	CAFile     string `json:"ca_file,omitempty"`
	ServerName string `json:"server_name,omitempty"`
	SkipVerify bool   `json:"skip_verify,omitempty"`
}

// AuthConfig holds authentication credentials
type AuthConfig struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type ClusterConfig struct {
	KeyPrefix        string           `json:"key_prefix,omitempty"`
	ServerIP         []string         `json:"endpoints,omitempty"`
	LockTimeout      Duration         `json:"lock_timeout,string,omitempty"`
	CaddyFile        []byte           `json:"-"` // Not exposed in JSON
	CaddyFilePath    string           `json:"-"` // Not exposed in JSON
	DisableCaddyLoad bool             `json:"disable_caddyfile_load,omitempty"`
	TLS              TLSConfig        `json:"tls,omitempty"`
	Auth             AuthConfig       `json:"auth,omitempty"`
	Connection       ConnectionConfig `json:"connection,omitempty"`
}

// WithDialTimeout sets the dial timeout for etcd connections
func WithDialTimeout(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		d, err := time.ParseDuration(s)
		if err != nil {
			return errors.Wrap(err, "invalid dial timeout duration format")
		}
		if d < time.Second {
			return errors.New("dial timeout must be at least 1 second")
		}
		c.Connection.DialTimeout = Duration{d}
		return nil
	}
}

// WithKeepAliveTime sets the keepalive time for etcd connections
func WithKeepAliveTime(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		d, err := time.ParseDuration(s)
		if err != nil {
			return errors.Wrap(err, "invalid keepalive time duration format")
		}
		if d < time.Second {
			return errors.New("keepalive time must be at least 1 second")
		}
		c.Connection.KeepAliveTime = Duration{d}
		return nil
	}
}

// WithKeepAliveTimeout sets the keepalive timeout for etcd connections
func WithKeepAliveTimeout(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		d, err := time.ParseDuration(s)
		if err != nil {
			return errors.Wrap(err, "invalid keepalive timeout duration format")
		}
		if d < time.Second {
			return errors.New("keepalive timeout must be at least 1 second")
		}
		c.Connection.KeepAliveTimeout = Duration{d}
		return nil
	}
}

// WithAutoSyncInterval sets the auto sync interval for etcd connections
func WithAutoSyncInterval(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		d, err := time.ParseDuration(s)
		if err != nil {
			return errors.Wrap(err, "invalid auto sync interval duration format")
		}
		if d < time.Second {
			return errors.New("auto sync interval must be at least 1 second")
		}
		c.Connection.AutoSyncInterval = Duration{d}
		return nil
	}
}

// WithRequestTimeout sets the request timeout for etcd operations
func WithRequestTimeout(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		d, err := time.ParseDuration(s)
		if err != nil {
			return errors.Wrap(err, "invalid request timeout duration format")
		}
		if d < time.Second {
			return errors.New("request timeout must be at least 1 second")
		}
		c.Connection.RequestTimeout = Duration{d}
		return nil
	}
}

// WithRejectOldCluster sets whether to reject connecting to old clusters
func WithRejectOldCluster(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		c.Connection.RejectOldCluster = strings.ToLower(s) == "true"
		return nil
	}
}

// validateEnvPairs checks that environment variables that must be set together are present
func validateEnvPairs(vars map[string]string) error {
	// Check TLS cert/key pair
	hasCert := vars["CADDY_CLUSTERING_ETCD_TLS_CERT"] != ""
	hasKey := vars["CADDY_CLUSTERING_ETCD_TLS_KEY"] != ""
	if hasCert != hasKey {
		return errors.New("both TLS cert and key must be provided together")
	}

	// Check auth credentials
	hasUser := vars["CADDY_CLUSTERING_ETCD_USERNAME"] != ""
	hasPass := vars["CADDY_CLUSTERING_ETCD_PASSWORD"] != ""
	if hasUser != hasPass {
		return errors.New("both username and password must be provided for authentication")
	}

	return nil
}

// WithUsername sets the username for etcd authentication
func WithUsername(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		c.Auth.Username = s
		return nil
	}
}

// WithPassword sets the password for etcd authentication
func WithPassword(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		c.Auth.Password = s
		return nil
	}
}

// WithTLSCert sets the client certificate file for TLS
func WithTLSCert(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		if err := validateFileExists(s); err != nil {
			return errors.Wrap(err, "TLS cert file does not exist")
		}
		c.TLS.CertFile = s
		return nil
	}
}

// WithTLSKey sets the client key file for TLS
func WithTLSKey(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		if err := validateFileExists(s); err != nil {
			return errors.Wrap(err, "TLS key file does not exist")
		}
		c.TLS.KeyFile = s
		return nil
	}
}

// WithTLSCA sets the CA certificate file for TLS
func WithTLSCA(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		if err := validateFileExists(s); err != nil {
			return errors.Wrap(err, "TLS CA file does not exist")
		}
		c.TLS.CAFile = s
		return nil
	}
}

// WithTLSServerName sets the server name for TLS verification
func WithTLSServerName(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		c.TLS.ServerName = s
		return nil
	}
}

// WithTLSSkipVerify sets whether to skip TLS verification
func WithTLSSkipVerify(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		c.TLS.SkipVerify = strings.ToLower(s) == "true"
		return nil
	}
}

// validateFileExists checks if a file exists and is readable
func validateFileExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("file does not exist")
		}
		return errors.Wrap(err, "error checking file")
	}
	if info.IsDir() {
		return errors.New("path is a directory, not a file")
	}
	// Check if file is readable
	f, err := os.Open(path)
	if err != nil {
		return errors.Wrap(err, "file is not readable")
	}
	f.Close()
	return nil
}

// validateServerURL checks if a server URL is valid
func validateServerURL(server string) error {
	u, err := url.Parse(server)
	if err != nil {
		return errors.Wrap(err, "invalid URL format")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("URL scheme must be http or https")
	}
	if u.Host == "" {
		return errors.New("URL must include host")
	}
	return nil
}

// validateTimeout checks if a timeout string is valid
func validateTimeout(timeout string) error {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return errors.Wrap(err, "invalid duration format")
	}
	if d < time.Second {
		return errors.New("timeout must be at least 1 second")
	}
	return nil
}

// ConfigOption represents a functional option for ClusterConfig
type ConfigOption func(c *ClusterConfig) error

// NewClusterConfig returns a new configuration with options passed as functional
// options and validates the configuration
func NewClusterConfig(opts ...ConfigOption) (*ClusterConfig, error) {
	c := &ClusterConfig{
		KeyPrefix:   "/caddy",
		LockTimeout: Duration{5 * time.Minute},
		Connection: ConnectionConfig{
			DialTimeout:      Duration{5 * time.Second},  // Wrap in Duration{}
			KeepAliveTime:    Duration{30 * time.Second}, // Wrap in Duration{}
			KeepAliveTimeout: Duration{10 * time.Second}, // Wrap in Duration{}
			AutoSyncInterval: Duration{5 * time.Minute},  // Wrap in Duration{}
			RequestTimeout:   Duration{10 * time.Second}, // Wrap in Duration{}
			RejectOldCluster: true,
		},
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}
	if len(c.ServerIP) == 0 {
		c.ServerIP = []string{"http://127.0.0.1:2379"}
	}

	// Validate the configuration
	if err := c.Validate(); err != nil {
		return nil, err
	}

	return c, nil
}

// Validate checks if the configuration is valid
func (c *ClusterConfig) Validate() error {
	// Check TLS configuration
	if c.TLS.CertFile != "" {
		if c.TLS.KeyFile == "" {
			return errors.New("TLS key file must be provided when cert file is specified")
		}
		if err := validateFileExists(c.TLS.CertFile); err != nil {
			return errors.Wrap(err, "TLS cert file error")
		}
		if err := validateFileExists(c.TLS.KeyFile); err != nil {
			return errors.Wrap(err, "TLS key file error")
		}
	}
	if c.TLS.CAFile != "" {
		if err := validateFileExists(c.TLS.CAFile); err != nil {
			return errors.Wrap(err, "TLS CA file error")
		}
	}

	// Check auth configuration
	if (c.Auth.Username == "") != (c.Auth.Password == "") {
		return errors.New("both username and password must be provided for authentication")
	}

	// Validate server URLs
	for _, server := range c.ServerIP {
		if err := validateServerURL(server); err != nil {
			return errors.Wrap(err, "invalid server URL")
		}
	}

	// Validate timeout
	if c.LockTimeout.Duration < time.Second {
		return errors.New("lock timeout must be at least 1 second")
	}

	return nil
}

// ConfigOptsFromEnvironment reads environment variables and returns options that can be applied via
// NewClusterConfig. It validates required pairs of environment variables and returns any validation errors.
func ConfigOptsFromEnvironment() ([]ConfigOption, error) {
	var opts []ConfigOption
	var env = map[string]func(s string) ConfigOption{
		"CADDY_CLUSTERING_ETCD_SERVERS":           WithServers,
		"CADDY_CLUSTERING_ETCD_PREFIX":            WithPrefix,
		"CADDY_CLUSTERING_ETCD_TIMEOUT":           WithTimeout,
		"CADDY_CLUSTERING_ETCD_CADDYFILE":         WithCaddyFile,
		"CADDY_CLUSTERING_ETCD_CADDYFILE_LOADER":  WithDisableCaddyfileLoad,
		"CADDY_CLUSTERING_ETCD_TLS_CERT":          WithTLSCert,
		"CADDY_CLUSTERING_ETCD_TLS_KEY":           WithTLSKey,
		"CADDY_CLUSTERING_ETCD_TLS_CA":            WithTLSCA,
		"CADDY_CLUSTERING_ETCD_TLS_SERVER_NAME":   WithTLSServerName,
		"CADDY_CLUSTERING_ETCD_TLS_SKIP_VERIFY":   WithTLSSkipVerify,
		"CADDY_CLUSTERING_ETCD_USERNAME":          WithUsername,
		"CADDY_CLUSTERING_ETCD_PASSWORD":          WithPassword,
		"CADDY_CLUSTERING_ETCD_DIAL_TIMEOUT":      WithDialTimeout,
		"CADDY_CLUSTERING_ETCD_KEEPALIVE_TIME":    WithKeepAliveTime,
		"CADDY_CLUSTERING_ETCD_KEEPALIVE_TIMEOUT": WithKeepAliveTimeout,
		"CADDY_CLUSTERING_ETCD_AUTO_SYNC":         WithAutoSyncInterval,
		"CADDY_CLUSTERING_ETCD_REQUEST_TIMEOUT":   WithRequestTimeout,
		"CADDY_CLUSTERING_ETCD_REJECT_OLD":        WithRejectOldCluster,
	}

	// Collect all environment variables first
	envVars := make(map[string]string)
	for e := range env {
		if val := os.Getenv(e); val != "" {
			envVars[e] = val
		}
	}

	// Validate required pairs
	if err := validateEnvPairs(envVars); err != nil {
		return nil, err
	}

	// Create options
	for e, f := range env {
		if val, ok := envVars[e]; ok {
			opts = append(opts, f(val))
		}
	}

	return opts, nil
}

// WithServers sets the etcd server endpoints.  Multiple endpoints are assumed to
// be separated by a comma, and consist of a full URL, including scheme and port
// (i.e., http://127.0.0.1:2379)  The default config uses port 2379 on localhost.
func WithServers(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		var srvs []string
		switch {
		case strings.Index(s, ";") >= 0:
			srvs = strings.Split(s, ";")
		default:
			srvs = strings.Split(s, ",")
		}
		for _, srv := range srvs {
			csrv := strings.TrimSpace(srv)
			u, err := url.Parse(csrv)
			if err != nil {
				return errors.Wrap(err, "CADDY_CLUSTERING_ETCD_SERVERS is an invalid format: servers should be separated by comma and be a full URL including scheme")
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return errors.New("CADDY_CLUSTERING_ETCD_SERVERS is an invalid format: servers must specify a scheme, either http or https")
			}
			c.ServerIP = append(c.ServerIP, csrv)
		}
		return nil
	}
}

// WithPrefix sets the etcd namespace for caddy data.  Default is `/caddy`.
// Prefixes are normalized to use `/` as a path separator.
func WithPrefix(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		c.KeyPrefix = path.Clean("/" + strings.Trim(strings.Replace(s, "\\", "/", -1), "/"))
		return nil
	}
}

// WithTimeout sets the time locks should be considered abandoned.  Locks that
// exist longer than this setting will be overwritten by the next client that
// acquires the lock.  The default is 5 minutes.  This option takes standard
// Go duration formats such as 5m, 1h, etc.
func WithTimeout(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		d, err := time.ParseDuration(s)
		if err != nil {
			return errors.Wrap(err, "CADDY_CLUSTERING_ETCD_TIMEOUT is an invalid format: must be a go standard time duration")
		}
		c.LockTimeout = Duration{d}
		return nil
	}
}

// WithCaddyFile sets the path to the bootstrap Caddyfile to load on initial start if configuration
// information is not already present in etcd.  The first cluster instance will load this
// file and store it in etcd.  Subsequent members of the cluster will prioritize configuration
// from etcd even if this file is present.  This function will not error even if the Caddyfile is
// not present.  If a caddyfile cannot be read from etcd, from this file, or from the default loader,
// caddy will start with an empty default configuration.
func WithCaddyFile(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		p := path.Clean(s)
		if !strings.HasPrefix(p, "/") {
			// assume a relative directory
			cwd, err := os.Getwd()
			if err != nil {
				log.Print("[WARN] etcd: could not ready configured caddyfile source, this may not indicate a problem if another cluster member has stored this data in etcd")
				return nil
			}
			p = path.Join(cwd, p)
		}
		r, err := ioutil.ReadFile(p)
		if err != nil {
			log.Print("[WARN] etcd: could not ready configured caddyfile source, this may not indicate a problem if another cluster member has stored this data in etcd")
			return nil
		}
		c.CaddyFilePath = p
		c.CaddyFile = r
		return nil
	}
}

// WithDisableCaddyfileLoad will skip all attempts at loading the caddyfile from etcd and force caddy to fall back
// to other enabled caddyfile loader plugins or the default loader
func WithDisableCaddyfileLoad(s string) ConfigOption {
	return func(c *ClusterConfig) error {
		val := strings.ToLower(strings.TrimSpace(s))
		switch val {
		case "disable":
			c.DisableCaddyLoad = true
			return nil
		case "enable", "":
			return nil
		default:
			return errors.New(fmt.Sprintf("CADDY_CLUSTERING_ETCD_CADDYFILE_LOADER is an invalid format: %s is an unknown option", val))
		}
	}
}
