package cmd

import (
	"os"

	"github.com/eltaline/pvectl/config"
	"github.com/spf13/cobra"
)

// CfgOverrides holds the flag values for config resolution.
var CfgOverrides config.Overrides

var rootCmd = &cobra.Command{
	Use:   "pvectl",
	Short: "CLI tool for managing Proxmox VE clusters",
	Long:  "pvectl is a kubectl-inspired CLI tool for managing Proxmox VE clusters from the terminal.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&CfgOverrides.ConfigPath, "config", "", "path to config file (default: ~/.config/pvectl/config.yaml, env: PVECTL_CONFIG)")
	rootCmd.PersistentFlags().StringVar(&CfgOverrides.Cluster, "cluster", "", "override cluster name (env: PVECTL_CLUSTER)")
	rootCmd.PersistentFlags().StringVar(&CfgOverrides.User, "user", "", "override user name (env: PVECTL_USER)")
	rootCmd.PersistentFlags().StringVar(&CfgOverrides.Context, "context", "", "override context name (env: PVECTL_CONTEXT)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
