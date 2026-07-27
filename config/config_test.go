package config

import (
	"os"
	"path/filepath"
	"testing"
)

const testConfig = `clusters:
  - name: prod
    cluster:
      server: 10.0.0.1
      port: 8006
      insecure: false
  - name: staging
    cluster:
      server: 10.0.0.2
      port: 8006
      insecure: true

users:
  - name: admin
    user:
      token-id: root@pam!mytoken
      token-secret: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
  - name: readonly
    user:
      username: reader@pve
      password: secret

contexts:
  - name: prod-admin
    context:
      cluster: prod
      user: admin
  - name: staging-readonly
    context:
      cluster: staging
      user: readonly

current-context: prod-admin
`

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	path := writeTestConfig(t, testConfig)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.Clusters) != 2 {
		t.Errorf("expected 2 clusters, got %d", len(cfg.Clusters))
	}
	if len(cfg.Users) != 2 {
		t.Errorf("expected 2 users, got %d", len(cfg.Users))
	}
	if len(cfg.Contexts) != 2 {
		t.Errorf("expected 2 contexts, got %d", len(cfg.Contexts))
	}
	if cfg.CurrentContext != "prod-admin" {
		t.Errorf("expected current-context prod-admin, got %s", cfg.CurrentContext)
	}
	if cfg.Clusters[0].Cluster.Server != "10.0.0.1" {
		t.Errorf("expected server 10.0.0.1, got %s", cfg.Clusters[0].Cluster.Server)
	}
	if cfg.Users[0].User.TokenID != "root@pam!mytoken" {
		t.Errorf("unexpected token-id: %s", cfg.Users[0].User.TokenID)
	}
}

func TestLoadMissing(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load() should not error for missing file, got: %v", err)
	}
	if len(cfg.Clusters) != 0 {
		t.Error("expected empty config for missing file")
	}
}

func TestLoadInvalid(t *testing.T) {
	path := writeTestConfig(t, "{{invalid yaml")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestResolveCurrentContext(t *testing.T) {
	path := writeTestConfig(t, testConfig)
	rc, err := Resolve(Overrides{ConfigPath: path})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if rc.ContextName != "prod-admin" {
		t.Errorf("expected context prod-admin, got %s", rc.ContextName)
	}
	if rc.ClusterName != "prod" {
		t.Errorf("expected cluster prod, got %s", rc.ClusterName)
	}
	if rc.Cluster.Server != "10.0.0.1" {
		t.Errorf("expected server 10.0.0.1, got %s", rc.Cluster.Server)
	}
	if rc.UserName != "admin" {
		t.Errorf("expected user admin, got %s", rc.UserName)
	}
	if rc.User.TokenID != "root@pam!mytoken" {
		t.Errorf("expected token-id root@pam!mytoken, got %s", rc.User.TokenID)
	}
}

func TestResolveFlagOverrides(t *testing.T) {
	path := writeTestConfig(t, testConfig)
	rc, err := Resolve(Overrides{
		ConfigPath: path,
		Context:    "staging-readonly",
	})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if rc.ContextName != "staging-readonly" {
		t.Errorf("expected context staging-readonly, got %s", rc.ContextName)
	}
	if rc.ClusterName != "staging" {
		t.Errorf("expected cluster staging, got %s", rc.ClusterName)
	}
	if rc.UserName != "readonly" {
		t.Errorf("expected user readonly, got %s", rc.UserName)
	}
}

func TestResolveClusterUserOverride(t *testing.T) {
	path := writeTestConfig(t, testConfig)
	rc, err := Resolve(Overrides{
		ConfigPath: path,
		Cluster:    "staging",
		User:       "readonly",
	})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	// Context is still prod-admin (from current-context), but cluster/user overridden.
	if rc.ClusterName != "staging" {
		t.Errorf("expected cluster staging, got %s", rc.ClusterName)
	}
	if rc.Cluster.Server != "10.0.0.2" {
		t.Errorf("expected server 10.0.0.2, got %s", rc.Cluster.Server)
	}
	if rc.UserName != "readonly" {
		t.Errorf("expected user readonly, got %s", rc.UserName)
	}
}

func TestResolveEnvOverrides(t *testing.T) {
	path := writeTestConfig(t, testConfig)

	t.Setenv(EnvContext, "staging-readonly")
	rc, err := Resolve(Overrides{ConfigPath: path})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if rc.ContextName != "staging-readonly" {
		t.Errorf("expected context staging-readonly from env, got %s", rc.ContextName)
	}
}

func TestResolveEnvClusterUser(t *testing.T) {
	path := writeTestConfig(t, testConfig)

	t.Setenv(EnvCluster, "staging")
	t.Setenv(EnvUser, "readonly")
	rc, err := Resolve(Overrides{ConfigPath: path})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if rc.ClusterName != "staging" {
		t.Errorf("expected cluster staging from env, got %s", rc.ClusterName)
	}
	if rc.UserName != "readonly" {
		t.Errorf("expected user readonly from env, got %s", rc.UserName)
	}
}

func TestResolveFlagOverridesEnv(t *testing.T) {
	path := writeTestConfig(t, testConfig)

	t.Setenv(EnvContext, "staging-readonly")
	rc, err := Resolve(Overrides{
		ConfigPath: path,
		Context:    "prod-admin",
	})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	// Flag takes precedence over env.
	if rc.ContextName != "prod-admin" {
		t.Errorf("expected flag to override env, got context %s", rc.ContextName)
	}
}

func TestResolveEnvConfig(t *testing.T) {
	path := writeTestConfig(t, testConfig)

	t.Setenv(EnvConfig, path)
	resolved := ConfigPath(Overrides{})
	if resolved != path {
		t.Errorf("expected config path %s from env, got %s", path, resolved)
	}
}

func TestSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")
	cfg := &Config{
		Clusters: []NamedCluster{
			{Name: "test", Cluster: Cluster{Server: "10.0.0.1", Port: 8006}},
		},
		Users: []NamedUser{
			{Name: "admin", User: User{
				Username:  "root@pam",
				Ticket:    "PVE:ticket:123",
				CSRFToken: "csrf-abc",
			}},
		},
		CurrentContext: "test-ctx",
	}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(loaded.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(loaded.Users))
	}
	if loaded.Users[0].User.Ticket != "PVE:ticket:123" {
		t.Errorf("ticket = %q", loaded.Users[0].User.Ticket)
	}
	if loaded.Users[0].User.CSRFToken != "csrf-abc" {
		t.Errorf("csrf = %q", loaded.Users[0].User.CSRFToken)
	}
}

func TestSetUser_update(t *testing.T) {
	cfg := &Config{
		Users: []NamedUser{
			{Name: "admin", User: User{Username: "old@pam"}},
		},
	}
	cfg.SetUser("admin", User{Username: "new@pam", Ticket: "t"})
	if cfg.Users[0].User.Username != "new@pam" {
		t.Errorf("expected updated username, got %s", cfg.Users[0].User.Username)
	}
	if len(cfg.Users) != 1 {
		t.Errorf("expected 1 user, got %d", len(cfg.Users))
	}
}

func TestSetUser_add(t *testing.T) {
	cfg := &Config{}
	cfg.SetUser("new-user", User{Username: "test@pam"})
	if len(cfg.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(cfg.Users))
	}
	if cfg.Users[0].Name != "new-user" {
		t.Errorf("name = %q", cfg.Users[0].Name)
	}
}

func TestSetCluster_add(t *testing.T) {
	cfg := &Config{}
	cfg.SetCluster("mycluster", Cluster{Server: "10.0.0.1", Port: 8006})
	if len(cfg.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(cfg.Clusters))
	}
	if cfg.Clusters[0].Cluster.Server != "10.0.0.1" {
		t.Errorf("server = %q", cfg.Clusters[0].Cluster.Server)
	}
}

func TestSetCluster_update(t *testing.T) {
	cfg := &Config{
		Clusters: []NamedCluster{
			{Name: "c1", Cluster: Cluster{Server: "old"}},
		},
	}
	cfg.SetCluster("c1", Cluster{Server: "new", Port: 9999})
	if len(cfg.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(cfg.Clusters))
	}
	if cfg.Clusters[0].Cluster.Server != "new" {
		t.Errorf("expected server new, got %s", cfg.Clusters[0].Cluster.Server)
	}
}

func TestSetContext_add(t *testing.T) {
	cfg := &Config{}
	cfg.SetContext("ctx1", Context{Cluster: "c1", User: "u1"})
	if len(cfg.Contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(cfg.Contexts))
	}
	if cfg.Contexts[0].Context.Cluster != "c1" {
		t.Errorf("cluster = %q", cfg.Contexts[0].Context.Cluster)
	}
}

func TestSetContext_update(t *testing.T) {
	cfg := &Config{
		Contexts: []NamedContext{
			{Name: "ctx1", Context: Context{Cluster: "old", User: "old"}},
		},
	}
	cfg.SetContext("ctx1", Context{Cluster: "new", User: "new"})
	if len(cfg.Contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(cfg.Contexts))
	}
	if cfg.Contexts[0].Context.Cluster != "new" {
		t.Errorf("expected cluster new, got %s", cfg.Contexts[0].Context.Cluster)
	}
}

func TestDeleteContext_existing(t *testing.T) {
	cfg := &Config{
		Contexts: []NamedContext{
			{Name: "ctx1", Context: Context{Cluster: "c1", User: "u1"}},
			{Name: "ctx2", Context: Context{Cluster: "c2", User: "u2"}},
		},
		CurrentContext: "ctx1",
	}
	ok := cfg.DeleteContext("ctx1")
	if !ok {
		t.Error("expected DeleteContext to return true")
	}
	if len(cfg.Contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(cfg.Contexts))
	}
	if cfg.CurrentContext != "" {
		t.Errorf("expected empty current-context, got %s", cfg.CurrentContext)
	}
}

func TestDeleteContext_nonexistent(t *testing.T) {
	cfg := &Config{}
	ok := cfg.DeleteContext("nope")
	if ok {
		t.Error("expected DeleteContext to return false for nonexistent")
	}
}

func TestMaskSecrets(t *testing.T) {
	cfg := &Config{
		Users: []NamedUser{
			{Name: "admin", User: User{
				TokenID:     "root@pam!mytoken",
				TokenSecret: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				Password:    "secretpass",
				Ticket:      "PVE:ticket:longvalue",
				CSRFToken:   "csrf-token-value",
			}},
		},
		Clusters: []NamedCluster{
			{Name: "prod", Cluster: Cluster{Server: "10.0.0.1"}},
		},
	}
	masked := cfg.MaskSecrets()
	u := masked.Users[0].User

	// TokenID should not be masked.
	if u.TokenID != "root@pam!mytoken" {
		t.Errorf("TokenID was modified: %s", u.TokenID)
	}
	// Secrets should be masked.
	if u.TokenSecret == "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Error("TokenSecret was not masked")
	}
	if u.Password == "secretpass" {
		t.Error("Password was not masked")
	}
	if u.Ticket == "PVE:ticket:longvalue" {
		t.Error("Ticket was not masked")
	}
	if u.CSRFToken == "csrf-token-value" {
		t.Error("CSRFToken was not masked")
	}
	// Original should be unchanged.
	if cfg.Users[0].User.TokenSecret != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Error("original config was modified")
	}
}

func TestResolveMissingConfig(t *testing.T) {
	rc, err := Resolve(Overrides{ConfigPath: "/nonexistent/config.yaml"})
	if err != nil {
		t.Fatalf("Resolve() should not error for missing config, got: %v", err)
	}
	if rc.ContextName != "" {
		t.Errorf("expected empty context, got %s", rc.ContextName)
	}
}
