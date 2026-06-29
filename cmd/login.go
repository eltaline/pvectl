package cmd

import (
	"fmt"
	"os"

	"github.com/eltaline/pvectl/api"
	"github.com/eltaline/pvectl/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate to a Proxmox VE server and save credentials",
	Long: `Authenticate using username/password via POST /access/ticket.
The resulting ticket and CSRF token are saved to the user section
of the configuration file for subsequent requests.`,
	RunE: runLogin,
}

var (
	loginUsername string
	loginPassword string
)

func init() {
	loginCmd.Flags().StringVarP(&loginUsername, "username", "u", "", "Proxmox username (e.g. root@pam)")
	loginCmd.Flags().StringVarP(&loginPassword, "password", "p", "", "password (omit to read from terminal)")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	if loginUsername == "" {
		return fmt.Errorf("--username is required")
	}

	password := loginPassword
	if password == "" {
		fmt.Fprint(os.Stderr, "Password: ")
		raw, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			return fmt.Errorf("reading password: %w", err)
		}
		fmt.Fprintln(os.Stderr)
		password = string(raw)
	}

	rc, err := config.Resolve(CfgOverrides)
	if err != nil {
		return fmt.Errorf("resolving config: %w", err)
	}

	if rc.Cluster.Server == "" {
		return fmt.Errorf("no cluster server configured; set a cluster in config or use --cluster")
	}

	port := rc.Cluster.Port
	if port == 0 {
		port = 8006
	}
	baseURL := fmt.Sprintf("https://%s:%d", rc.Cluster.Server, port)

	opts := []api.ClientOption{
		api.WithInsecure(rc.Cluster.Insecure || CfgOverrides.Insecure),
		api.WithVerbosity(api.Verbosity(CfgOverrides.Verbosity)),
	}
	caFile := rc.Cluster.CAFile
	if CfgOverrides.CAFile != "" {
		caFile = CfgOverrides.CAFile
	}
	if caFile != "" {
		opts = append(opts, api.WithCAFile(caFile))
	}

	client, err := api.NewClient(baseURL, opts...)
	if err != nil {
		return fmt.Errorf("creating API client: %w", err)
	}

	ticket, err := client.Authenticate(loginUsername, password)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Save ticket credentials into the user section of the config.
	cfgPath := config.ConfigPath(CfgOverrides)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	userName := rc.UserName
	if userName == "" {
		userName = "default"
	}

	cfg.SetUser(userName, config.User{
		Username:  loginUsername,
		Ticket:    ticket.Ticket,
		CSRFToken: ticket.CSRFToken,
	})

	if err := config.Save(cfgPath, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Logged in as %s, credentials saved to user %q in %s\n", ticket.Username, userName, cfgPath)
	return nil
}
