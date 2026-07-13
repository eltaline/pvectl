package cmd

import (
	"os"

	"github.com/eltaline/pvectl/config"
	"github.com/eltaline/pvectl/output"
	"github.com/spf13/cobra"
)

// CfgOverrides holds the flag values for config resolution.
var CfgOverrides config.Overrides

// OutputFormat holds the raw value of the -o flag.
var OutputFormat string

// NoHeaders disables printing table headers.
var NoHeaders bool

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
	rootCmd.PersistentFlags().BoolVar(&CfgOverrides.Insecure, "insecure-skip-tls-verify", false, "skip TLS certificate verification")
	rootCmd.PersistentFlags().StringVar(&CfgOverrides.CAFile, "certificate-authority", "", "path to CA certificate file for TLS verification")
	rootCmd.PersistentFlags().CountVarP(&CfgOverrides.Verbosity, "verbose", "v", "increase verbosity (-v for basic, -vv for detailed)")
	rootCmd.PersistentFlags().StringVarP(&OutputFormat, "output", "o", "table", "output format: table, json, yaml, wide")
	rootCmd.PersistentFlags().BoolVar(&NoHeaders, "no-headers", false, "hide table headers in table/wide output")
}

// ResolveOutputOptions returns output.Options from the current flag values,
// using the given writer as the output destination.
func ResolveOutputOptions(w *os.File) (output.Options, error) {
	f, err := output.ParseFormat(OutputFormat)
	if err != nil {
		return output.Options{}, err
	}
	return output.Options{
		Format:    f,
		NoHeaders: NoHeaders,
		Writer:    w,
	}, nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
