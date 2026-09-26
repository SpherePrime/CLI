package voice

// The idle watchdog keeps the resident server from eating the model's memory
// forever. It runs as a detached Prime child, so the memory is returned even
// after every Prime window has closed: dictation refreshes a timestamp in
// the state file, and whoever notices the timestamp going stale stops the
// server.

import (
	"context"
	"os"
	"strconv"
	"time"
)

// serverWatchdogInterval is how often the detached watcher re-checks whether
// the server has gone stale.
const serverWatchdogInterval = 10 * time.Second

// ServerWatchdogOptions parameterises one watchdog run. The flags mirror the
// state Prime recorded when it started the server, so the watcher can tell
// whether that server is still the current one.
type ServerWatchdogOptions struct {
	PID       int
	Port      int
	StatePath string
	Idle      time.Duration
	// PrimeExe overrides the binary re-invoked after a stop, empty means
	// the running one.
	PrimeExe string
	// Args are Prime's startup arguments to replay.
	Args []string
	// RestartPrime asks for the replay after a stop; the interactive TUI
	// sets it, tool commands do not.
	RestartPrime bool
	// CheckEvery shortens the poll loop in tests.
	CheckEvery time.Duration
	// Kill and Healthy exist so tests can watch decisions without a server.
	Kill    func(int) error
	Healthy func(context.Context, int) bool
	Restart func() error
}

// startIdleWatchdog detaches a Prime child that keeps running after this
// process exits, which is what stops the server once it goes stale.
func startIdleWatchdog(state ServerState, statePath string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{
		"voice", "server", "watchdog",
		"--pid", strconv.Itoa(state.PID),
		"--port", strconv.Itoa(state.Port),
		"--idle", ServerIdleTimeout.String(),
		"--state", statePath,
	}
	// The child must not re-enter Prime's session bootstrap, and argv[0] may
	// be a bare launcher name, so record enough context for it to find the
	// real binary when needed. Arguments are repeated one per flag so a
	// value that looks like a flag still survives the trip.
	replay := terminalRestartArgs(os.Args)
	if replay != nil {
		args = append(args, "--restart-prime", "--prime", os.Args[0])
		for _, arg := range replay {
			args = append(args, "--arg", arg)
		}
	}
	_, err = spawnDetached(serverSpec{Exe: exe, Args: args})
	return err
}

// terminalRestartArgs decides whether Prime's own command line is worth
// bringing back after a kill. Only interactive sessions are: voice commands
// are tools, and replaying "server start" would resurrect the server the
// watchdog was asked to stop, forever.
func terminalRestartArgs(argv []string) []string {
	if len(argv) < 1 {
		return nil
	}
	for _, arg := range primeCommandArgs(argv[1:]) {
		if arg == "voice" || arg == "help" || arg == "version" {
			return nil
		}
	}
	return argv[1:]
}

// RunServerWatchdog blocks until the managed server is stale, replaced, or
// gone, stopping it when needed. After a stop, and when the recorded server
// looks dead while a Prime instance still holds the same layout, the watchdog
// replays the original Prime command line detached so the user's terminal
// comes back on its own; the next dictation then warms a fresh server.
func RunServerWatchdog(ctx context.Context, opts ServerWatchdogOptions) error {
	idle := opts.Idle
	if idle <= 0 {
		idle = ServerIdleTimeout
	}
	every := opts.CheckEvery
	if every <= 0 {
		every = serverWatchdogInterval
	}
	healthy := opts.Healthy
	if healthy == nil {
		healthy = serverHealthy
	}
	kill := opts.Kill
	if kill == nil {
		kill = killProcess
	}

	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			state, ok := loadServerState(opts.StatePath)
			if !ok || state.PID != opts.PID {
				// Replaced by a newer server, or already cleaned up.
				return nil
			}
			if time.Since(state.LastUsed) < idle {
				continue
			}
			if healthy(ctx, opts.Port) {
				if err := kill(opts.PID); err != nil {
					return err
				}
				time.Sleep(serverKillGrace)
			}
			clearServerState(opts.StatePath)
			restartPrime(ctx, opts, healthy)
			return nil
		}
	}
}

// restartPrime re-launches the user's Prime command line detached, after
// removing any leftover non-resumable session so the TUI opens clean. A live
// Prime process means a terminal still exists, so it stays hands-off.
func restartPrime(ctx context.Context, opts ServerWatchdogOptions, healthy func(context.Context, int) bool) {
	if opts.Restart != nil {
		_ = opts.Restart()
		return
	}
	if !opts.RestartPrime || primeRunning(opts.PrimeExe, opts.Args) {
		return
	}
	clearInterruptedSessions()
	exe := pickPrimeExe(opts.PrimeExe, currentPrimeExe())
	if exe == "" {
		return
	}
	startCtx, cancel := context.WithTimeout(ctx, engineProbeTimeout)
	defer cancel()
	if healthy(startCtx, opts.Port) {
		return // a fresh server already took over the port
	}
	_, _ = spawnDetached(serverSpec{
		Exe:        exe,
		Args:       append([]string{}, opts.Args...),
		NewConsole: true,
	})
}
