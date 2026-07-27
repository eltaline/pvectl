package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eltaline/pvectl/config"
)

const testCfgYAML = `clusters:
  - name: prod
    cluster:
      server: 10.0.0.1
      port: 8006
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
      password: secretpass

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

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigView_masksSecrets(t *testing.T) {
	path := writeTempConfig(t, testCfgYAML)

	old := CfgOverrides
	CfgOverrides = config.Overrides{ConfigPath: path}
	defer func() { CfgOverrides = old }()

	var buf bytes.Buffer
	origStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runConfigView(configViewCmd, nil)
	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runConfigView error: %v", err)
	}

	buf.ReadFrom(r)
	out := buf.String()

	// Token secret should be masked.
	if strings.Contains(out, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee") {
		t.Error("token-secret was not masked")
	}
	// Password should be masked.
	if strings.Contains(out, "secretpass") {
		t.Error("password was not masked")
	}
	// Server should remain visible.
	if !strings.Contains(out, "10.0.0.1") {
		t.Error("expected server 10.0.0.1 in output")
	}
}

func TestGetContexts(t *testing.T) {
	path := writeTempConfig(t, testCfgYAML)

	oldCfg := CfgOverrides
	oldFmt := OutputFormat
	CfgOverrides = config.Overrides{ConfigPath: path}
	OutputFormat = "table"
	defer func() {
		CfgOverrides = oldCfg
		OutputFormat = oldFmt
	}()

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := runGetContexts(getContextsCmd, nil)
	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runGetContexts error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "prod-admin") {
		t.Error("expected prod-admin in output")
	}
	if !strings.Contains(out, "staging-readonly") {
		t.Error("expected staging-readonly in output")
	}
	// Current context should have an asterisk.
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "prod-admin") && strings.Contains(line, "*") {
			return
		}
	}
	t.Error("expected * marker on prod-admin context")
}

func TestCurrentContext(t *testing.T) {
	path := writeTempConfig(t, testCfgYAML)

	oldCfg := CfgOverrides
	CfgOverrides = config.Overrides{ConfigPath: path}
	defer func() { CfgOverrides = oldCfg }()

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w

	err := runCurrentContext(currentContextCmd, nil)
	w.Close()
	os.Stdout = origStdout

	if err != nil {
		t.Fatalf("runCurrentContext error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := strings.TrimSpace(buf.String())
	if out != "prod-admin" {
		t.Errorf("expected prod-admin, got %q", out)
	}
}

func TestUseContext(t *testing.T) {
	path := writeTempConfig(t, testCfgYAML)

	oldCfg := CfgOverrides
	CfgOverrides = config.Overrides{ConfigPath: path}
	defer func() { CfgOverrides = oldCfg }()

	err := runUseContext(useContextCmd, []string{"staging-readonly"})
	if err != nil {
		t.Fatalf("runUseContext error: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CurrentContext != "staging-readonly" {
		t.Errorf("expected current-context staging-readonly, got %s", cfg.CurrentContext)
	}
}

func TestUseContext_notFound(t *testing.T) {
	path := writeTempConfig(t, testCfgYAML)

	oldCfg := CfgOverrides
	CfgOverrides = config.Overrides{ConfigPath: path}
	defer func() { CfgOverrides = oldCfg }()

	err := runUseContext(useContextCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent context")
	}
}

func TestDeleteContext(t *testing.T) {
	path := writeTempConfig(t, testCfgYAML)

	oldCfg := CfgOverrides
	CfgOverrides = config.Overrides{ConfigPath: path}
	defer func() { CfgOverrides = oldCfg }()

	err := runDeleteContext(deleteContextCmd, []string{"prod-admin"})
	if err != nil {
		t.Fatalf("runDeleteContext error: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Contexts) != 1 {
		t.Errorf("expected 1 context, got %d", len(cfg.Contexts))
	}
	// current-context should be cleared since we deleted the active context.
	if cfg.CurrentContext != "" {
		t.Errorf("expected empty current-context, got %s", cfg.CurrentContext)
	}
}

func TestDeleteContext_notFound(t *testing.T) {
	path := writeTempConfig(t, testCfgYAML)

	oldCfg := CfgOverrides
	CfgOverrides = config.Overrides{ConfigPath: path}
	defer func() { CfgOverrides = oldCfg }()

	err := runDeleteContext(deleteContextCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent context")
	}
}

func TestSetCluster_new(t *testing.T) {
	path := writeTempConfig(t, testCfgYAML)

	oldCfg := CfgOverrides
	CfgOverrides = config.Overrides{ConfigPath: path}
	defer func() { CfgOverrides = oldCfg }()

	setClusterServer = "10.0.0.3"
	setClusterPort = 8007
	setClusterCmd.Flags().Set("server", "10.0.0.3")
	setClusterCmd.Flags().Set("port", "8007")

	err := runSetCluster(setClusterCmd, []string{"newcluster"})
	if err != nil {
		t.Fatalf("runSetCluster error: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Clusters) != 3 {
		t.Fatalf("expected 3 clusters, got %d", len(cfg.Clusters))
	}
	last := cfg.Clusters[2]
	if last.Name != "newcluster" || last.Cluster.Server != "10.0.0.3" || last.Cluster.Port != 8007 {
		t.Errorf("unexpected cluster: %+v", last)
	}
}

func TestSetCredentials_new(t *testing.T) {
	path := writeTempConfig(t, testCfgYAML)

	oldCfg := CfgOverrides
	CfgOverrides = config.Overrides{ConfigPath: path}
	defer func() { CfgOverrides = oldCfg }()

	setCredentialsCmd.Flags().Set("token-id", "user@pam!tok")
	setCredentialsCmd.Flags().Set("token-secret", "secret123")

	err := runSetCredentials(setCredentialsCmd, []string{"newuser"})
	if err != nil {
		t.Fatalf("runSetCredentials error: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(cfg.Users))
	}
	last := cfg.Users[2]
	if last.Name != "newuser" || last.User.TokenID != "user@pam!tok" || last.User.TokenSecret != "secret123" {
		t.Errorf("unexpected user: %+v", last)
	}
}
