package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/SpherePrime/CLI/internal/message"
)

// backgroundWakeOutputLimit mirrors tools.BackgroundJobWake's contract: the
// wake prompt carries a bounded tail of the finished job's output.
const backgroundWakeOutputLimit = 2000

// backgroundJobWakePrompt builds the hidden prompt the agent receives when a
// background job it started finishes on its own. The message is delivered as
// a hidden user message: it drives the next turn without posing as the user.
func backgroundJobWakePrompt(shellID, description, command string, exitCode int, output string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Background job %s finished with exit code %d.\n", shellID, exitCode)
	if description != "" {
		fmt.Fprintf(&b, "Description: %s\n", description)
	}
	if command != "" {
		fmt.Fprintf(&b, "Command: %s\n", command)
	}
	if tail := strings.TrimSpace(output); tail != "" {
		if len(tail) > backgroundWakeOutputLimit {
			tail = "… " + tail[len(tail)-backgroundWakeOutputLimit:]
		}
		fmt.Fprintf(&b, "\nOutput:\n%s\n", tail)
	}
	b.WriteString("\nContinue the interrupted work using this result. If the task is already complete, report the outcome briefly.")
	return b.String()
}

// wakeOnBackgroundJob delivers a finished background job to the session that
// started it, but only when the agent is no longer watching: an active turn
// sees the job through job_output on its own, so a wake-up there would only
// duplicate work. When the turn has already ended, a fresh run starts with
// the job's result and the agent resumes where it stopped. Callbacks run
// detached: the tool call that spawned the watcher is long gone by the time
// a long job finishes.
func (c *coordinator) wakeOnBackgroundJob(sessionID, shellID, description, command string, exitCode int, output string) {
	if sessionID == "" {
		return
	}
	prompt := backgroundJobWakePrompt(shellID, description, command, exitCode, output)
	ctx := message.WithHiddenUserMessage(context.Background())
	go func() {
		if c.currentAgent().IsSessionBusy(sessionID) {
			// The agent was still in its turn when the job finished, so
			// it has been watching the job itself and already saw the
			// result. Waking it would start a duplicate continuation.
			slog.Debug("Background job wake-up skipped: agent was watching the job",
				"session_id", sessionID, "shell_id", shellID)
			return
		}
		runner := c.backgroundWakeRunner
		if runner == nil {
			runner = func(ctx context.Context, sessionID, prompt string) {
				if _, err := c.Run(ctx, sessionID, prompt); err != nil {
					slog.Debug("Background job wake-up could not start a run",
						"session_id", sessionID, "shell_id", shellID, "error", err)
				}
			}
		}
		runner(ctx, sessionID, prompt)
	}()
}
