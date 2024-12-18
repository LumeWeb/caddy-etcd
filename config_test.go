package etcd

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrefix(t *testing.T) {
	tcs := []struct {
		Path   string
		Expect string
	}{
		{Path: "/caddy/", Expect: "/caddy"},
		{Path: "\\caddy", Expect: "/caddy"},
		{Path: "\\caddy", Expect: "/caddy"},
		{Path: "//caddy", Expect: "/caddy"},
		{Path: "//caddy//test", Expect: "/caddy/test"},
		{Path: "caddy", Expect: "/caddy"},
		{Path: "caddy//test", Expect: "/caddy/test"},
	}
	for _, tc := range tcs {
		c, err := NewClusterConfig(WithPrefix(tc.Path))
		assert.NoError(t, err)
		assert.Equal(t, tc.Expect, c.KeyPrefix)
	}
}

func TestServers(t *testing.T) {
	tcs := []struct {
		Name      string
		SString   string
		Expect    []string
		ShouldErr bool
	}{
		{Name: "1", SString: "http://127.0.0.1:2379", Expect: []string{"http://127.0.0.1:2379"}, ShouldErr: false},
		{Name: "2 comma", SString: "http://127.0.0.1:2379,http://127.0.0.1:2380", Expect: []string{"http://127.0.0.1:2379", "http://127.0.0.1:2380"}, ShouldErr: false},
		{Name: "2 comma ws", SString: "http://127.0.0.1:2379, http://127.0.0.1:2380", Expect: []string{"http://127.0.0.1:2379", "http://127.0.0.1:2380"}, ShouldErr: false},
		{Name: "2 comma ws2", SString: "http://127.0.0.1:2379 , http://127.0.0.1:2380", Expect: []string{"http://127.0.0.1:2379", "http://127.0.0.1:2380"}, ShouldErr: false},
		{Name: "2 semicolon", SString: "http://127.0.0.1:2379;http://127.0.0.1:2380", Expect: []string{"http://127.0.0.1:2379", "http://127.0.0.1:2380"}, ShouldErr: false},
		{Name: "no scheme", SString: "127.0.0.1:2379", Expect: []string{}, ShouldErr: true},
		{Name: "https", SString: "https://127.0.0.1:2379", Expect: []string{"https://127.0.0.1:2379"}, ShouldErr: false},
		{Name: "no scheme dns", SString: "etcd", Expect: []string{}, ShouldErr: true},
		{Name: "scheme dns", SString: "http://etcd", Expect: []string{"http://etcd"}, ShouldErr: false},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			c, err := NewClusterConfig(WithServers(tc.SString))
			switch {
			case tc.ShouldErr:
				assert.Error(t, err)
				assert.Nil(t, c)
			default:
				assert.NoError(t, err)
				assert.Equal(t, tc.Expect, c.ServerIP)
			}
		})
	}
}

func TestCaddyfile(t *testing.T) {
	caddyfile := []byte("example.com {\n\tproxy http://127.0.0.1:8080\n}")
	tcs := []struct {
		Name string
		Path string
	}{
		{Name: "absolute path", Path: "/tmp"},
		{Name: "relative path", Path: "./"},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			f, err := ioutil.TempFile(tc.Path, "Caddyfile")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			if _, err := f.Write(caddyfile); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			c, err := NewClusterConfig(WithCaddyFile(f.Name()))
			assert.NoError(t, err)
			assert.Equal(t, caddyfile, c.CaddyFile)
		})
	}

}

func TestTimeout(t *testing.T) {
	tcs := []struct {
		Name      string
		Input     string
		Expected  time.Duration
		ShouldErr bool
	}{
		{Name: "ok", Input: "5m", Expected: time.Minute * 5, ShouldErr: false},
		{Name: "ok small", Input: "2s", Expected: time.Second * 2, ShouldErr: false},
		{Name: "not ok", Input: "2y", Expected: time.Second, ShouldErr: true},
	}
	for _, tc := range tcs {
		c, err := NewClusterConfig(WithTimeout(tc.Input))
		switch {
		case tc.ShouldErr:
			assert.Nil(t, c)
			assert.Error(t, err)
		default:
			assert.NoError(t, err)
			assert.Equal(t, tc.Expected, c.LockTimeout.Duration)
		}
	}
}

func TestConnectionSettings(t *testing.T) {
	tcs := []struct {
		name      string
		opts      []ConfigOption
		validate  func(*testing.T, *ClusterConfig)
		shouldErr bool
	}{
		{
			name: "valid dial timeout",
			opts: []ConfigOption{WithDialTimeout("10s")},
			validate: func(t *testing.T, c *ClusterConfig) {
				assert.Equal(t, 10*time.Second, c.Connection.DialTimeout.Duration)
			},
		},
		{
			name:      "invalid dial timeout",
			opts:      []ConfigOption{WithDialTimeout("0s")},
			shouldErr: true,
		},
		{
			name: "valid keepalive settings",
			opts: []ConfigOption{
				WithKeepAliveTime("30s"),
				WithKeepAliveTimeout("10s"),
			},
			validate: func(t *testing.T, c *ClusterConfig) {
				assert.Equal(t, 30*time.Second, c.Connection.KeepAliveTime.Duration)
				assert.Equal(t, 10*time.Second, c.Connection.KeepAliveTimeout.Duration)
			},
		},
		{
			name: "valid auto sync and request timeout",
			opts: []ConfigOption{
				WithAutoSyncInterval("5m"),
				WithRequestTimeout("10s"),
			},
			validate: func(t *testing.T, c *ClusterConfig) {
				assert.Equal(t, 5*time.Minute, c.Connection.AutoSyncInterval.Duration)
				assert.Equal(t, 10*time.Second, c.Connection.RequestTimeout.Duration)
			},
		},
		{
			name: "reject old cluster",
			opts: []ConfigOption{WithRejectOldCluster("true")},
			validate: func(t *testing.T, c *ClusterConfig) {
				assert.True(t, c.Connection.RejectOldCluster)
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := NewClusterConfig(tc.opts...)
			if tc.shouldErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			tc.validate(t, cfg)
		})
	}
}

func TestValidateEnvPairs(t *testing.T) {
	tcs := []struct {
		name      string
		vars      map[string]string
		shouldErr bool
	}{
		{
			name: "valid TLS pair",
			vars: map[string]string{
				"CADDY_CLUSTERING_ETCD_TLS_CERT": "/path/to/cert",
				"CADDY_CLUSTERING_ETCD_TLS_KEY":  "/path/to/key",
			},
			shouldErr: false,
		},
		{
			name: "missing TLS key",
			vars: map[string]string{
				"CADDY_CLUSTERING_ETCD_TLS_CERT": "/path/to/cert",
			},
			shouldErr: true,
		},
		{
			name: "missing TLS cert",
			vars: map[string]string{
				"CADDY_CLUSTERING_ETCD_TLS_KEY": "/path/to/key",
			},
			shouldErr: true,
		},
		{
			name: "valid auth pair",
			vars: map[string]string{
				"CADDY_CLUSTERING_ETCD_USERNAME": "user",
				"CADDY_CLUSTERING_ETCD_PASSWORD": "pass",
			},
			shouldErr: false,
		},
		{
			name: "missing password",
			vars: map[string]string{
				"CADDY_CLUSTERING_ETCD_USERNAME": "user",
			},
			shouldErr: true,
		},
		{
			name: "missing username",
			vars: map[string]string{
				"CADDY_CLUSTERING_ETCD_PASSWORD": "pass",
			},
			shouldErr: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEnvPairs(tc.vars)
			if tc.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClusterConfigJSONUnmarshal(t *testing.T) {
	jsonConfig := `{
		"key_prefix": "/caddy/test",
		"endpoints": ["http://localhost:2379", "http://localhost:2380"],
	"lock_timeout": "300s",   
		"disable_caddyfile_load": true,
		"tls": {
			"cert_file": "/path/to/cert.pem",
			"key_file": "/path/to/key.pem",
			"ca_file": "/path/to/ca.pem",
			"server_name": "etcd.example.com",
			"skip_verify": false
		},
		"auth": {
			"username": "testuser",
			"password": "testpass"
		},
		"connection": {
			"dial_timeout": "10s",
			"keepalive_time": "30s",
			"keepalive_timeout": "15s",
			"auto_sync_interval": "5m",
			"request_timeout": "20s",
			"reject_old_cluster": true
		}
	}`

	var config ClusterConfig
	err := json.Unmarshal([]byte(jsonConfig), &config)
	require.NoError(t, err)

	// Validate top-level config
	assert.Equal(t, "/caddy/test", config.KeyPrefix)
	assert.Equal(t, []string{"http://localhost:2379", "http://localhost:2380"}, config.ServerIP)
	assert.Equal(t, 5*time.Minute, config.LockTimeout.Duration)
	assert.True(t, config.DisableCaddyLoad)

	// Validate TLS config
	assert.Equal(t, "/path/to/cert.pem", config.TLS.CertFile)
	assert.Equal(t, "/path/to/key.pem", config.TLS.KeyFile)
	assert.Equal(t, "/path/to/ca.pem", config.TLS.CAFile)
	assert.Equal(t, "etcd.example.com", config.TLS.ServerName)
	assert.False(t, config.TLS.SkipVerify)

	// Validate Auth config
	assert.Equal(t, "testuser", config.Auth.Username)
	assert.Equal(t, "testpass", config.Auth.Password)

	// Validate Connection config
	assert.Equal(t, 10*time.Second, config.Connection.DialTimeout.Duration)
	assert.Equal(t, 30*time.Second, config.Connection.KeepAliveTime.Duration)
	assert.Equal(t, 15*time.Second, config.Connection.KeepAliveTimeout.Duration)
	assert.Equal(t, 5*time.Minute, config.Connection.AutoSyncInterval.Duration)
	assert.Equal(t, 20*time.Second, config.Connection.RequestTimeout.Duration)
	assert.True(t, config.Connection.RejectOldCluster)
}

func TestClusterConfigJSONPartialUnmarshal(t *testing.T) {
	jsonConfig := `{
		"key_prefix": "/caddy/partial",
		"endpoints": ["http://localhost:2379"]
	}`

	var config ClusterConfig
	err := json.Unmarshal([]byte(jsonConfig), &config)
	require.NoError(t, err)

	assert.Equal(t, "/caddy/partial", config.KeyPrefix)
	assert.Equal(t, []string{"http://localhost:2379"}, config.ServerIP)
}

func TestClusterConfigJSONMarshal(t *testing.T) {
	config := ClusterConfig{
		KeyPrefix:   "/caddy/marshal",
		ServerIP:    []string{"http://localhost:2379"},
		LockTimeout: Duration{5 * time.Minute},
		TLS: TLSConfig{
			CertFile:   "/path/to/cert.pem",
			SkipVerify: true,
		},
	}

	jsonData, err := json.Marshal(config)
	require.NoError(t, err)

	var unmarshaledConfig map[string]interface{}
	err = json.Unmarshal(jsonData, &unmarshaledConfig)
	require.NoError(t, err)

	assert.Equal(t, "/caddy/marshal", unmarshaledConfig["key_prefix"])
	assert.Contains(t, unmarshaledConfig, "endpoints")
	assert.Contains(t, unmarshaledConfig, "tls")
}

func TestConfigOpts(t *testing.T) {
	caddyfile := []byte("example.com {\n\tproxy http://127.0.0.1:8080\n}")
	f, err := ioutil.TempFile("", "Caddyfile")
	if err != nil {
		t.Fatal(err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Fatal(err)
		}
	}(f.Name())

	if _, err := f.Write(caddyfile); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	set := func(e map[string]string) error {
		for k, v := range e {
			if err := os.Setenv(k, v); err != nil {
				return err
			}
		}
		return nil
	}
	unset := func(e map[string]string) error {
		for k := range e {
			if err := os.Unsetenv(k); err != nil {
				return err
			}
		}
		return nil
	}
	env := map[string]string{
		"CADDY_CLUSTERING_ETCD_SERVERS":          "http://127.0.0.1:2379",
		"CADDY_CLUSTERING_ETCD_PREFIX":           "/test",
		"CADDY_CLUSTERING_ETCD_TIMEOUT":          "30m",
		"CADDY_CLUSTERING_ETCD_CADDYFILE":        f.Name(),
		"CADDY_CLUSTERING_ETCD_CADDYFILE_LOADER": "disable",
	}
	env2 := map[string]string{
		"CADDY_CLUSTERING_ETCD_SERVERS":   "http://127.0.0.1:2379",
		"CADDY_CLUSTERING_ETCD_PREFIX":    "/test",
		"CADDY_CLUSTERING_ETCD_TIMEOUT":   "30y",
		"CADDY_CLUSTERING_ETCD_CADDYFILE": f.Name(),
	}
	tcs := []struct {
		Name      string
		Input     map[string]string
		Expect    ClusterConfig
		ShouldErr bool
	}{
		{Name: "ok", Input: env, Expect: ClusterConfig{
			ServerIP:         []string{"http://127.0.0.1:2379"},
			LockTimeout:      Duration{30 * time.Minute}, // Wrap in Duration{}
			KeyPrefix:        "/test",
			CaddyFile:        caddyfile,
			CaddyFilePath:    f.Name(),
			DisableCaddyLoad: true,
			Connection: ConnectionConfig{
				DialTimeout:      Duration{5 * time.Second},  // Wrap in Duration{}
				KeepAliveTime:    Duration{30 * time.Second}, // Wrap in Duration{}
				KeepAliveTimeout: Duration{10 * time.Second}, // Wrap in Duration{}
				AutoSyncInterval: Duration{5 * time.Minute},  // Wrap in Duration{}
				RequestTimeout:   Duration{10 * time.Second}, // Wrap in Duration{}
				RejectOldCluster: true,
			},
		}, ShouldErr: false},
		{Name: "should err", Input: env2, Expect: ClusterConfig{}, ShouldErr: true},
	}
	for _, tc := range tcs {
		t.Run(tc.Name, func(t *testing.T) {
			if err := unset(tc.Input); err != nil {
				t.Fatal(err)
			}
			if err := set(tc.Input); err != nil {
				t.Fatal(err)
			}
			opts, err := ConfigOptsFromEnvironment()
			require.NoError(t, err)
			c, err := NewClusterConfig(opts...)
			switch {
			case tc.ShouldErr:
				assert.Error(t, err)
				assert.Nil(t, c)
			default:
				assert.NoError(t, err)
				assert.Equal(t, tc.Expect, *c)
			}
			_ = unset(tc.Input)
		})
	}

}
