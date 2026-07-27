package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/eltaline/pvectl/config"
	"github.com/eltaline/pvectl/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage pvectl configuration",
}

// --- config view ---

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Display the current configuration (secrets are masked)",
	RunE:  runConfigView,
}

func runConfigView(cmd *cobra.Command, args []string) error {
	cfgPath := config.ConfigPath(CfgOverrides)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	masked := cfg.MaskSecrets()
	data, err := yaml.Marshal(masked)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	fmt.Fprint(os.Stdout, string(data))
	return nil
}

// --- get-contexts ---

type contextRow struct {
	Current string
	Name    string
	Cluster string
	User    string
}

func (r contextRow) TableHeaders() []string { return []string{"Current", "Name", "Cluster", "User"} }
func (r contextRow) TableRow() []string      { return []string{r.Current, r.Name, r.Cluster, r.User} }
func (r contextRow) WideHeaders() []string   { return nil }
func (r contextRow) WideRow() []string       { return nil }

var getContextsCmd = &cobra.Command{
	Use:     "get-contexts",
	Short:   "List all contexts defined in the configuration",
	Aliases: []string{"get-context"},
	RunE:    runGetContexts,
}

func runGetContexts(cmd *cobra.Command, args []string) error {
	cfgPath := config.ConfigPath(CfgOverrides)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	opts, err := ResolveOutputOptions(os.Stdout)
	if err != nil {
		return err
	}

	var items []output.Printable
	for _, nc := range cfg.Contexts {
		cur := ""
		if nc.Name == cfg.CurrentContext {
			cur = "*"
		}
		items = append(items, contextRow{
			Current: cur,
			Name:    nc.Name,
			Cluster: nc.Context.Cluster,
			User:    nc.Context.User,
		})
	}
	return output.Print(items, opts)
}

// --- current-context ---

var currentContextCmd = &cobra.Command{
	Use:   "current-context",
	Short: "Display the current context name",
	RunE:  runCurrentContext,
}

func runCurrentContext(cmd *cobra.Command, args []string) error {
	cfgPath := config.ConfigPath(CfgOverrides)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if cfg.CurrentContext == "" {
		return fmt.Errorf("no current context is set")
	}
	fmt.Fprintln(os.Stdout, cfg.CurrentContext)
	return nil
}

// --- use-context ---

var useContextCmd = &cobra.Command{
	Use:   "use-context NAME",
	Short: "Set the current context",
	Args:  cobra.ExactArgs(1),
	RunE:  runUseContext,
}

func runUseContext(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfgPath := config.ConfigPath(CfgOverrides)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	found := false
	for _, nc := range cfg.Contexts {
		if nc.Name == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("context %q not found", name)
	}

	cfg.CurrentContext = name
	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Switched to context %q.\n", name)
	return nil
}

// --- set-cluster ---

var (
	setClusterServer      string
	setClusterPort        int
	setClusterInsecure    string
	setClusterCAFile      string
	setClusterFingerprint string
)

var setClusterCmd = &cobra.Command{
	Use:   "set-cluster NAME",
	Short: "Set or update a cluster entry in the configuration",
	Args:  cobra.ExactArgs(1),
	RunE:  runSetCluster,
}

func init() {
	setClusterCmd.Flags().StringVar(&setClusterServer, "server", "", "server address (hostname or IP)")
	setClusterCmd.Flags().IntVar(&setClusterPort, "port", 0, "API port (default 8006)")
	setClusterCmd.Flags().StringVar(&setClusterInsecure, "insecure-skip-tls-verify", "", "skip TLS verification (true/false)")
	setClusterCmd.Flags().StringVar(&setClusterCAFile, "certificate-authority", "", "path to CA certificate file")
	setClusterCmd.Flags().StringVar(&setClusterFingerprint, "fingerprint", "", "server TLS fingerprint")
}

func runSetCluster(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfgPath := config.ConfigPath(CfgOverrides)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	// Find existing cluster or start fresh.
	var cluster config.Cluster
	for _, nc := range cfg.Clusters {
		if nc.Name == name {
			cluster = nc.Cluster
			break
		}
	}

	if cmd.Flags().Changed("server") {
		cluster.Server = setClusterServer
	}
	if cmd.Flags().Changed("port") {
		cluster.Port = setClusterPort
	}
	if cmd.Flags().Changed("insecure-skip-tls-verify") {
		v, err := strconv.ParseBool(setClusterInsecure)
		if err != nil {
			return fmt.Errorf("invalid value for --insecure-skip-tls-verify: %w", err)
		}
		cluster.Insecure = v
	}
	if cmd.Flags().Changed("certificate-authority") {
		cluster.CAFile = setClusterCAFile
	}
	if cmd.Flags().Changed("fingerprint") {
		cluster.Fingerprint = setClusterFingerprint
	}

	cfg.SetCluster(name, cluster)
	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Cluster %q set.\n", name)
	return nil
}

// --- set-credentials ---

var (
	setCredTokenID     string
	setCredTokenSecret string
	setCredUsername     string
	setCredPassword    string
)

var setCredentialsCmd = &cobra.Command{
	Use:   "set-credentials NAME",
	Short: "Set or update a user entry in the configuration",
	Args:  cobra.ExactArgs(1),
	RunE:  runSetCredentials,
}

func init() {
	setCredentialsCmd.Flags().StringVar(&setCredTokenID, "token-id", "", "API token ID (e.g. user@realm!tokenname)")
	setCredentialsCmd.Flags().StringVar(&setCredTokenSecret, "token-secret", "", "API token secret")
	setCredentialsCmd.Flags().StringVar(&setCredUsername, "username", "", "username for ticket auth")
	setCredentialsCmd.Flags().StringVar(&setCredPassword, "password", "", "password for ticket auth")
}

func runSetCredentials(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfgPath := config.ConfigPath(CfgOverrides)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	var user config.User
	for _, nu := range cfg.Users {
		if nu.Name == name {
			user = nu.User
			break
		}
	}

	if cmd.Flags().Changed("token-id") {
		user.TokenID = setCredTokenID
	}
	if cmd.Flags().Changed("token-secret") {
		user.TokenSecret = setCredTokenSecret
	}
	if cmd.Flags().Changed("username") {
		user.Username = setCredUsername
	}
	if cmd.Flags().Changed("password") {
		user.Password = setCredPassword
	}

	cfg.SetUser(name, user)
	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "User %q set.\n", name)
	return nil
}

// --- delete-context ---

var deleteContextCmd = &cobra.Command{
	Use:   "delete-context NAME",
	Short: "Delete a context from the configuration",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeleteContext,
}

func runDeleteContext(cmd *cobra.Command, args []string) error {
	name := args[0]
	cfgPath := config.ConfigPath(CfgOverrides)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	if !cfg.DeleteContext(name) {
		return fmt.Errorf("context %q not found", name)
	}

	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Deleted context %q.\n", name)
	return nil
}

// Register all config subcommands.
func init() {
	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(getContextsCmd)
	configCmd.AddCommand(currentContextCmd)
	configCmd.AddCommand(useContextCmd)
	configCmd.AddCommand(setClusterCmd)
	configCmd.AddCommand(setCredentialsCmd)
	configCmd.AddCommand(deleteContextCmd)
	rootCmd.AddCommand(configCmd)
}
