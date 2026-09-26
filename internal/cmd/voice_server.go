package cmd

import (
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

// voiceServerCmd is hidden plumbing, not user-facing UX: Prime's resident
// Whisper server is started by the voice plugin and stopped when the plugin
// is deactivated. This command is a no-op placeholder kept so the TUI's
// "voice server watchdog" replay logic does not break on older builds.
var voiceServerCmd = &cobra.Command{
	Use:    "server",
	Short:  "Internal Whisper server plumbing",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var hiddenVoiceCmd = &cobra.Command{
	Use:    "voice",
	Short:  "Internal helpers for Prime's voice MCP server",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	hiddenVoiceCmd.AddCommand(voiceServerCmd)
	rootCmd.AddCommand(hiddenVoiceCmd)
}
