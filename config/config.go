package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Default config path relative to user home.
const defaultConfigDir = ".config/pvectl"
const defaultConfigFile = "config.yaml"

// Environment variable names for overrides.
const (
	EnvCluster = "PVECTL_CLUSTER"
	EnvUser    = "PVECTL_USER"
	EnvContext = "PVECTL_CONTEXT"
	EnvConfig  = "PVECTL_CONFIG"
)

// Cluster holds connection details for a Proxmox VE cluster/node.
type Cluster struct {
	Server      string `yaml:"server"`
	Port        int    `yaml:"port,omitempty"`
	Insecure    bool   `yaml:"insecure,omitempty"`
	CAFile      string `yaml:"certificate-authority,omitempty"`
	Fingerprint string `yaml:"fingerprint,omitempty"`
}

// NamedCluster binds a name to cluster connection data.
type NamedCluster struct {
	Name    string  `yaml:"name"`
	Cluster Cluster `yaml:"cluster"`
}

// User holds authentication credentials.
type User struct {
	TokenID     string `yaml:"token-id,omitempty"`
	TokenSecret string `yaml:"token-secret,omitempty"`
	Username    string `yaml:"username,omitempty"`
	Password    string `yaml:"password,omitempty"`
}

// NamedUser binds a name to user credentials.
type NamedUser struct {
	Name string `yaml:"name"`
	User User   `yaml:"user"`
}

// Context ties a cluster and user together.
type Context struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

// NamedContext binds a name to a context definition.
type NamedContext struct {
	Name    string  `yaml:"name"`
	Context Context `yaml:"context"`
}

// Config is the top-level kubeconfig-style configuration.
type Config struct {
	Clusters       []NamedCluster `yaml:"clusters"`
	Users          []NamedUser    `yaml:"users"`
	Contexts       []NamedContext `yaml:"contexts"`
	CurrentContext string         `yaml:"current-context"`
}

// ResolvedConfig is the final flattened result after resolving
// the active context, cluster, and user.
type ResolvedConfig struct {
	ClusterName string
	Cluster     Cluster
	UserName    string
	User        User
	ContextName string
}

// Overrides captures CLI flag and env-var overrides.
type Overrides struct {
	ConfigPath string
	Cluster    string
	User       string
	Context    string
	Insecure   bool
	CAFile     string
	Verbosity  int
}

// DefaultConfigPath returns ~/.config/pvectl/config.yaml.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", defaultConfigDir, defaultConfigFile)
	}
	return filepath.Join(home, defaultConfigDir, defaultConfigFile)
}

// Load reads and parses the config file. If the file does not exist,
// an empty Config is returned without error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &cfg, nil
}

// ConfigPath returns the effective config file path considering
// flag override, env override, and default (in that priority order).
func ConfigPath(overrides Overrides) string {
	if overrides.ConfigPath != "" {
		return overrides.ConfigPath
	}
	if v := os.Getenv(EnvConfig); v != "" {
		return v
	}
	return DefaultConfigPath()
}

// Resolve loads the config and resolves the active context, cluster,
// and user, applying overrides from flags and environment variables.
func Resolve(overrides Overrides) (*ResolvedConfig, error) {
	path := ConfigPath(overrides)

	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	// Determine context name: flag > env > current-context in file.
	ctxName := cfg.CurrentContext
	if v := os.Getenv(EnvContext); v != "" {
		ctxName = v
	}
	if overrides.Context != "" {
		ctxName = overrides.Context
	}

	// Find the context.
	var ctx *Context
	for i := range cfg.Contexts {
		if cfg.Contexts[i].Name == ctxName {
			ctx = &cfg.Contexts[i].Context
			break
		}
	}

	// Determine cluster name: flag > env > context's cluster.
	clusterName := ""
	if ctx != nil {
		clusterName = ctx.Cluster
	}
	if v := os.Getenv(EnvCluster); v != "" {
		clusterName = v
	}
	if overrides.Cluster != "" {
		clusterName = overrides.Cluster
	}

	// Determine user name: flag > env > context's user.
	userName := ""
	if ctx != nil {
		userName = ctx.User
	}
	if v := os.Getenv(EnvUser); v != "" {
		userName = v
	}
	if overrides.User != "" {
		userName = overrides.User
	}

	// Look up cluster.
	var cluster Cluster
	for i := range cfg.Clusters {
		if cfg.Clusters[i].Name == clusterName {
			cluster = cfg.Clusters[i].Cluster
			break
		}
	}

	// Look up user.
	var user User
	for i := range cfg.Users {
		if cfg.Users[i].Name == userName {
			user = cfg.Users[i].User
			break
		}
	}

	return &ResolvedConfig{
		ClusterName: clusterName,
		Cluster:     cluster,
		UserName:    userName,
		User:        user,
		ContextName: ctxName,
	}, nil
}
