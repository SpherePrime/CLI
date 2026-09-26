package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/SpherePrime/CLI/internal/voice"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

// voiceServerCmd is hidden plumbing, not user-facing UX: Prime's resident
// Whisper server detaches one "prime voice server watchdog" child to stop
// itself after idling, so the engine needs this exact command line to exist.
// Dictation itself lives in the voice MCP server ("prime mcp serve voice").
var (
	voiceWatchdogPID    int
	voiceWatchdogPort   int
	voiceWatchdogIdle   time.Duration
	voiceWatchdogState  string
	voiceWatchdogPrime  string
	voiceWatchdogArgs   []string
	voiceWatchdogReplay bool
)

var voiceServerWatchdogCmd = &cobra.Command{
	Use:    "watchdog",
	Short:  "Stop the managed Whisper server after it goes idle",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE:   runVoiceServerWatchdog,
}

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
	voiceWatchdogFlags := voiceServerWatchdogCmd.Flags()
	voiceWatchdogFlags.IntVar(&voiceWatchdogPID, "pid", 0, "process id of the managed whisper server")
	voiceWatchdogFlags.IntVar(&voiceWatchdogPort, "port", 8000, "port the managed whisper server listens on")
	voiceWatchdogFlags.DurationVar(&voiceWatchdogIdle, "idle", 15*time.Minute, "stop the server after this long without dictation")
	voiceWatchdogFlags.StringVar(&voiceWatchdogState, "state", "", "path of the server state file")
	voiceWatchdogFlags.StringVar(&voiceWatchdogPrime, "prime", "", "the Prime command the watchdog may replay")
	voiceWatchdogFlags.BoolVar(&voiceWatchdogReplay, "restart-prime", false, "re-launch Prime after stopping an idle server")
	voiceWatchdogFlags.StringArrayVar(&voiceWatchdogArgs, "arg", nil, "one startup argument to replay; repeat per argument")

	voiceServerCmd.AddCommand(voiceServerWatchdogCmd)
	hiddenVoiceCmd.AddCommand(voiceServerCmd)
	rootCmd.AddCommand(hiddenVoiceCmd)
}

// runVoiceServerWatchdog executes the detached idle watcher.
func runVoiceServerWatchdog(cmd *cobra.Command, args []string) error {
	if voiceWatchdogPID <= 0 || voiceWatchdogState == "" {
		return errors.New("--pid and --state are required")
	}
	return voice.RunServerWatchdog(commandContext(cmd), voice.ServerWatchdogOptions{
		PID:          voiceWatchdogPID,
		Port:         voiceWatchdogPort,
		StatePath:    voiceWatchdogState,
		Idle:         voiceWatchdogIdle,
		PrimeExe:     voiceWatchdogPrime,
		Args:         voiceWatchdogArgs,
		RestartPrime: voiceWatchdogReplay,
	})
}

// commandContext returns the command's context, or a background one when the
// command was built without one.
func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}
