package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pvectl",
	Short: "CLI tool for managing Proxmox VE clusters",
	Long:  "pvectl is a kubectl-inspired CLI tool for managing Proxmox VE clusters from the terminal.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
