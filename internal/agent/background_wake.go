package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/shell"
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
	b.WriteString("\nThis is an automatic notice, not a message from the user. Continue the interrupted work using this result. If the task is already complete, report the outcome to the user briefly.")
	return b.String()
}

// backgroundWakeIdlePoll is how often a pending wake-up checks whether the
// session's current turn has ended.
var backgroundWakeIdlePoll = 500 * time.Millisecond

// wakeOnBackgroundJob delivers a finished background job to the session that
// started it. A job that finishes while a turn is still running is held until
// that turn ends: the agent may have moved on to other work or be writing its
// final reply, and dropping the wake-up there loses the result for good. The
// wake-up is skipped only when the agent already read the job's final output
// through job_output. Callbacks run detached: the tool call that spawned the
// watcher is long gone by the time a long job finishes.
func (c *coordinator) wakeOnBackgroundJob(sessionID, shellID, description, command string, exitCode int, output string) {
	if sessionID == "" {
		return
	}
	prompt := backgroundJobWakePrompt(shellID, description, command, exitCode, output)
	ctx := message.WithHiddenUserMessage(context.Background())
	go func() {
		for c.currentAgent().IsSessionBusy(sessionID) {
			time.Sleep(backgroundWakeIdlePoll)
		}
		if backgroundJobResultSeen(shellID) {
			slog.Debug("Background job wake-up skipped: agent already read the result",
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

func backgroundJobResultSeen(shellID string) bool {
	bgShell, ok := shell.GetBackgroundShellManager().Get(shellID)
	return ok && bgShell.ResultSeen()
}

func mcpJobWakePrompt(jobID, toolName, status, output string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Background MCP job %s (%s) finished with status %s.\n", jobID, toolName, status)
	if tail := strings.TrimSpace(output); tail != "" {
		fmt.Fprintf(&b, "\nResult:\n%s\n", tail)
	}
	fmt.Fprintf(&b, "\nUse %s with op=result and job_id=%s for the full result. This is an automatic notice, not a message from the user. Continue the interrupted work using this result. If the task is already complete, report the outcome to the user briefly.", tools.MCPJobToolName, jobID)
	return b.String()
}

func (c *coordinator) wakeOnMCPJob(sessionID, jobID, toolName, status, output string) {
	if sessionID == "" {
		return
	}
	prompt := mcpJobWakePrompt(jobID, toolName, status, output)
	ctx := message.WithHiddenUserMessage(context.Background())
	go func() {
		for c.currentAgent().IsSessionBusy(sessionID) {
			time.Sleep(backgroundWakeIdlePoll)
		}
		if tools.MCPJobResultSeen(jobID) {
			return
		}
		runner := c.backgroundWakeRunner
		if runner == nil {
			runner = func(ctx context.Context, sessionID, prompt string) {
				if _, err := c.Run(ctx, sessionID, prompt); err != nil {
					slog.Debug("MCP job wake-up could not start a run", "session_id", sessionID, "job_id", jobID, "error", err)
				}
			}
		}
		runner(ctx, sessionID, prompt)
	}()
}
