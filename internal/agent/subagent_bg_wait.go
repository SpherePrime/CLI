package agent

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/shell"
)

var (
	subagentBgWaitPoll   = 500 * time.Millisecond
	subagentBgSettleWait = 200 * time.Millisecond
)

type backgroundToolResultMeta struct {
	Background     bool   `json:"background"`
	AutoBackground bool   `json:"auto_background"`
	ShellID        string `json:"shell_id"`
	JobID          string `json:"job_id"`
}

func reviewVerdict(output string) string {
	if reviewApproved(output) {
		return "APPROVED"
	}
	if changesVerdict.MatchString(output) {
		return "CHANGES_REQUIRED"
	}
	return ""
}

func settleWaitDuration() time.Duration {
	if d := backgroundWakeIdlePoll + 100*time.Millisecond; d > subagentBgSettleWait {
		return d
	}
	return subagentBgSettleWait
}

func (c *coordinator) hasActiveSubagentBackgroundJobs(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	for _, s := range shell.GetBackgroundShellManager().RunningForSession(sessionID) {
		if !s.ResultSeen() {
			return true
		}
	}
	return tools.SessionHasRunningMCPJobs(sessionID)
}

func parseBackgroundToolResultMeta(raw string) (backgroundToolResultMeta, bool) {
	if raw == "" {
		return backgroundToolResultMeta{}, false
	}
	var meta backgroundToolResultMeta
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return backgroundToolResultMeta{}, false
	}
	return meta, true
}

func (c *coordinator) shellWaitingForResult(sessionID, shellID string, runningShells map[string]bool, hasPendingWake bool) bool {
	if shellID == "" {
		return false
	}
	if runningShells[shellID] {
		return true
	}
	if !hasPendingWake {
		return false
	}
	bgShell, ok := shell.GetBackgroundShellManager().Get(shellID)
	if !ok || bgShell.SessionID != sessionID || bgShell.ResultSeen() {
		return false
	}
	_, _, done, err := bgShell.GetOutput()
	return !done || !shell.IsInterrupt(err)
}

func (c *coordinator) mcpJobWaitingForResult(jobID string, hasPendingWake bool) bool {
	if jobID == "" {
		return false
	}
	if tools.MCPJobIsRunning(jobID) {
		return true
	}
	return hasPendingWake && tools.MCPJobPendingWake(jobID)
}

func (c *coordinator) shouldWaitForSubagentBackgroundJobs(ctx context.Context, job *agentJob, output string) bool {
	if job == nil || job.sessionID == "" {
		return false
	}
	if job.agent == config.AgentReview && reviewVerdict(output) != "" {
		return false
	}
	hasActive := c.hasActiveSubagentBackgroundJobs(job.sessionID)
	hasPendingWake := c.hasPendingWake(job.sessionID)
	if !hasActive && !hasPendingWake {
		return false
	}
	if c.messages == nil {
		return false
	}
	msgs, err := c.messages.List(ctx, job.sessionID)
	if err != nil {
		return false
	}
	startIdx := 0
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == message.User {
			startIdx = i + 1
			break
		}
	}
	turnMsgs := msgs[startIdx:]
	if len(turnMsgs) == 0 {
		return false
	}
	runningShells := make(map[string]bool)
	for _, s := range shell.GetBackgroundShellManager().RunningForSession(job.sessionID) {
		if !s.ResultSeen() {
			runningShells[s.ID] = true
		}
	}
	for _, msg := range turnMsgs {
		if msg.Role != message.Tool {
			continue
		}
		for _, tr := range msg.ToolResults() {
			meta, ok := parseBackgroundToolResultMeta(tr.Metadata)
			if !ok {
				continue
			}
			if meta.AutoBackground && c.shellWaitingForResult(job.sessionID, meta.ShellID, runningShells, hasPendingWake) {
				return true
			}
		}
	}
	var finalBatch []message.Message
	for i := len(turnMsgs) - 1; i >= 0; i-- {
		if turnMsgs[i].Role == message.Tool {
			finalBatch = append(finalBatch, turnMsgs[i])
			continue
		}
		if len(finalBatch) > 0 {
			break
		}
	}
	for _, msg := range finalBatch {
		for _, tr := range msg.ToolResults() {
			meta, ok := parseBackgroundToolResultMeta(tr.Metadata)
			if !ok {
				continue
			}
			if meta.Background && c.shellWaitingForResult(job.sessionID, meta.ShellID, runningShells, hasPendingWake) {
				return true
			}
			if meta.Background && c.mcpJobWaitingForResult(meta.JobID, hasPendingWake) {
				return true
			}
		}
	}
	return false
}

func (c *coordinator) waitForSubagentBackgroundJobs(ctx context.Context, job *agentJob) bool {
	ticker := time.NewTicker(subagentBgWaitPoll)
	defer ticker.Stop()
	for {
		job.mu.Lock()
		hasPending := len(job.pending) > 0
		stopping := job.stopping
		job.mu.Unlock()
		if stopping || ctx.Err() != nil {
			return false
		}
		if hasPending {
			return true
		}
		if !c.hasActiveSubagentBackgroundJobs(job.sessionID) && !c.hasPendingWake(job.sessionID) {
			timer := time.NewTimer(settleWaitDuration())
			select {
			case <-ctx.Done():
				timer.Stop()
				return false
			case <-job.wakeCh:
				timer.Stop()
			case <-timer.C:
			}
			job.mu.Lock()
			hasPending = len(job.pending) > 0
			stopping = job.stopping
			job.mu.Unlock()
			if stopping || ctx.Err() != nil {
				return false
			}
			return hasPending
		}
		select {
		case <-ctx.Done():
			return false
		case <-job.wakeCh:
		case <-ticker.C:
		}
	}
}
