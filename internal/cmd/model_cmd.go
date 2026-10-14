package cmd

import (
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

// modelCmd is the parent command for model-related subcommands.
var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Manage model configuration and settings",
	Long:  `Manage model configuration and settings, including reasoning modes and effort levels.`,
}

func init() {
	rootCmd.AddCommand(modelCmd)
}
