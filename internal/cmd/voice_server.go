package cmd

import (
	"errors"
	"time"

	"github.com/SpherePrime/CLI/internal/voice"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

var (
	voiceWatchdogPID    int
	voiceWatchdogPort   int
	voiceWatchdogIdle   time.Duration
	voiceWatchdogState  string
	voiceWatchdogPrime  string
	voiceWatchdogArgs   []string
	voiceWatchdogReplay bool
)

var voiceServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Show and control the always-on Whisper server",
	Long: `Prime keeps a local whisper.cpp server loaded with the model, so dictation
answers in about a second instead of reloading hundreds of megabytes on every
keypress. The server starts itself when a session needs it and stops after
about fifteen minutes without dictation. If the terminal it belonged to was
killed, Prime brings the window back on its own.

The Whisper engine list prefers this server automatically whenever it runs.`,
	Example: `
# Show the running server, model, and idle time
prime voice server

# Start it now instead of waiting for the first dictation
prime voice server start

# Stop it to free the model's memory right away
prime voice server stop`,
	Args: cobra.NoArgs,
	RunE: runVoiceServerStatus,
}

// voiceServerStatusCmd spells the report out explicitly; "prime voice server"
// alone runs the same thing.
var voiceServerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report whether the Whisper server is running",
	Args:  cobra.NoArgs,
	RunE:  runVoiceServerStatus,
}

var voiceServerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Load the Whisper model into RAM now",
	Args:  cobra.NoArgs,
	RunE:  runVoiceServerStart,
}

var voiceServerTouchCmd = &cobra.Command{
	Use:   "touch",
	Short: "Restart the idle countdown without starting anything",
	Args:  cobra.NoArgs,
	RunE:  runVoiceServerTouch,
}

var voiceServerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Whisper server Prime started",
	Args:  cobra.NoArgs,
	RunE:  runVoiceServerStop,
}

// voiceServerWatchdogCmd is the detached helper that stops the server once it
// goes idle and brings Prime back when its terminal was killed.
var voiceServerWatchdogCmd = &cobra.Command{
	Use:    "watchdog",
	Short:  "Stop the managed Whisper server after it goes idle",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE:   runVoiceServerWatchdog,
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

	voiceServerCmd.AddCommand(voiceServerStatusCmd)
	voiceServerCmd.AddCommand(voiceServerStartCmd)
	voiceServerCmd.AddCommand(voiceServerTouchCmd)
	voiceServerCmd.AddCommand(voiceServerStopCmd)
	voiceServerCmd.AddCommand(voiceServerWatchdogCmd)
	voiceCmd.AddCommand(voiceServerCmd)
}

// runVoiceServerStatus reports whether the managed server is answering.
func runVoiceServerStatus(cmd *cobra.Command, args []string) error {
	info := voice.ServerStatus(commandContext(cmd))
	if info.Listening {
		cmd.Println("Whisper server: " + voice.ServerEndpoint() + " (ready)")
	} else {
		cmd.Println("Whisper server: not running")
	}
	if !info.HasState {
		cmd.Println("Prime has not started one yet. It starts on its own with the first")
		cmd.Println("dictation, or now with: prime voice server start")
		return nil
	}
	cmd.Printf("Managed by Prime: pid %d, model %s, last used %s ago\n",
		info.State.PID, info.State.Model, info.IdleFor.Round(time.Second))
	cmd.Printf("Idle timeout: %s, log: %s\n", voice.ServerIdleTimeout, info.LogFile)
	return nil
}

// runVoiceServerStart loads the model without waiting for a dictation.
func runVoiceServerStart(cmd *cobra.Command, args []string) error {
	settings, _, err := voiceSettingsFromConfig(cmd)
	if err != nil {
		return err
	}
	ctx := commandContext(cmd)
	already := voice.ServerStatus(ctx).Listening
	cmd.Println("Warming the Whisper server (this loads the model into RAM, once)...")
	start := time.Now()
	if err := voice.WarmServer(ctx, settings); err != nil {
		return err
	}
	if already {
		cmd.Println("Whisper server was already running at " + voice.ServerEndpoint())
		return nil
	}
	cmd.Printf("Whisper server ready at %s (model load took %s).\n",
		voice.ServerEndpoint(), time.Since(start).Round(time.Second))
	return nil
}

// runVoiceServerTouch restarts the idle countdown for a running server.
func runVoiceServerTouch(cmd *cobra.Command, args []string) error {
	if !voice.TouchServer() {
		return errors.New("no Prime-managed Whisper server is recorded")
	}
	cmd.Println("Idle countdown restarted.")
	return nil
}

// runVoiceServerStop frees the model's memory right away.
func runVoiceServerStop(cmd *cobra.Command, args []string) error {
	stopped, err := voice.StopServer(commandContext(cmd))
	if err != nil {
		return err
	}
	if stopped {
		cmd.Println("Whisper server stopped.")
	} else {
		cmd.Println("No Prime-managed Whisper server is running.")
	}
	return nil
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
