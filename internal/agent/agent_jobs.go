package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/hyper"
	"github.com/SpherePrime/CLI/internal/agent/notify"
	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const AgentJobsToolName = "agent_jobs"

// agentJob is one sub-agent started with the agent tool's background mode.
// The coordinator registry is the only record of it: the job outlives the
// step that started it and is polled through the agent_jobs tool.
type agentJob struct {
	id              string
	agent           string
	sessionID       string
	parentSessionID string
	prompt          string
	startedAt       time.Time
	updatedAt       time.Time
	rootSessionID   string
	name            string
	params          subAgentParams
	pending         []string
	wake            bool
	seen            bool
	generation      uint64
	hasRun          bool
	queuedAt        time.Time

	mu     sync.Mutex
	status string // "running", "done", "error", "canceled"
	output string
	cancel context.CancelFunc
	done   chan struct{}
}

func (j *agentJob) finish(status, output string) {
	j.mu.Lock()
	j.status = status
	j.output = output
	j.updatedAt = time.Now()
	if j.done != nil {
		close(j.done)
	}
	j.mu.Unlock()
}

// snapshot is a point-in-time read of the job state, safe from any goroutine.
func (j *agentJob) snapshot() (status, output string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status, j.output
}

// wait blocks until the job finishes, the context is canceled, or timeout
// elapses. It reports whether the job is done when it returns.
func (j *agentJob) wait(ctx context.Context, timeout time.Duration) bool {
	j.mu.Lock()
	done := j.done
	j.mu.Unlock()
	var (
		timer *time.Timer
		tick  <-chan time.Time
	)
	if timeout > 0 {
		timer = time.NewTimer(timeout)
		tick = timer.C
	}
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	case <-tick:
		return false
	}
}

// agentJobsDescription is the tool text shown to the model. It stays
// English like every other tool description: the catalog is for the UI,
// tools are for the model.
const agentJobsDescription = `Inspect or stop the agents of this team.

Operations:
- list (default): one line per agent: id, agent name, status, runtime, output size.
- get: status and output of one agent that already finished.
- kill: stop a running agent.

Do not poll or wait for agents. Their results and messages arrive
automatically as team notices once your current turn ends, so end the turn
or keep working on your own part instead of checking on them.`

// startAgentJob registers a background sub-agent run. The sub-session is
// created synchronously so it is immediately visible in the session list
// even while the model is still running; the execution itself happens on a
// detached goroutine that survives the caller's context being canceled.
func (c *coordinator) startAgentJob(ctx context.Context, params subAgentParams, agentName string) (*agentJob, error) {
	return c.launchAgentJob(ctx, params, agentName, true)
}

func (c *coordinator) launchAgentJob(ctx context.Context, params subAgentParams, agentName string, wake bool) (*agentJob, error) {
	if c.cfg.Config().Options.DisableMultiAgent {
		return nil, errors.New("agent orchestration is disabled")
	}
	parent, err := c.sessions.Get(ctx, params.SessionID)
	if err != nil {
		return nil, err
	}
	if c.sessions.IsAgentToolSession(parent.ID) {
		return nil, errors.New("workers cannot launch nested agents; request assignments from the primary chat using agent_message")
	}
	root, err := c.teamRoot(ctx, params.SessionID)
	if err != nil {
		return nil, err
	}
	agentToolSessionID := c.sessions.CreateAgentToolSessionID(params.AgentMessageID, params.ToolCallID)
	session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, params.SessionID, params.SessionTitle)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	permission.InheritSessionSkipRequests(c.permissions, params.SessionID, session.ID)

	if params.SessionSetup != nil {
		params.SessionSetup(session.ID)
	}

	// Get model configuration
	model := params.Agent.Model()
	_, ok := c.cfg.Config().Providers.Get(model.ModelCfg.Provider)
	if !ok {
		// The session already exists, so undo it before reporting the
		// error instead of leaving an orphan in the session list.
		if derr := c.sessions.Delete(ctx, session.ID); derr != nil {
			slog.Warn("Delete orphaned agent job session", "session", session.ID, "error", derr)
		}
		return nil, errModelProviderNotConfigured
	}

	// Detached from the caller: background jobs survive the step that
	// started them, like background shells, and are stopped only via
	// agent_jobs(kill).
	jobCtx, cancel := context.WithCancel(c.workerContext())

	c.jobMu.Lock()
	c.agentJobSeq++
	job := &agentJob{
		id:              session.ID,
		agent:           agentName,
		sessionID:       session.ID,
		parentSessionID: params.SessionID,
		prompt:          params.Prompt,
		startedAt:       time.Now(),
		status:          "queued",
		updatedAt:       time.Now(),
		queuedAt:        time.Now(),
		rootSessionID:   root,
		name:            strings.TrimPrefix(params.SessionTitle, "Agent Session: "+agentName+" | "),
		params:          params,
		wake:            wake,
		cancel:          cancel,
		done:            make(chan struct{}),
	}
	if c.agentJobs == nil {
		c.agentJobs = make(map[string]*agentJob)
	}
	c.agentJobs[job.id] = job
	c.jobMu.Unlock()

	c.reserveWorker(job)
	go c.executeWorker(job, jobCtx)
	return job, nil
}

// runAgentJob is the background twin of runSubAgent: same execution,
// results parked on the job instead of handed back as a tool response.
func (c *coordinator) runAgentJob(job *agentJob, ctx context.Context, params subAgentParams, sessionID string, model Model, maxTokens int64, providerCfg config.ProviderConfig) (string, string) {
	previous, _ := c.sessions.Get(ctx, sessionID)
	result, err := c.invokeSubAgent(ctx, params, sessionID, model, maxTokens, providerCfg)
	if err != nil {
		failure := fmt.Sprintf("Failed to generate response: %s", err)
		status := "error"
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			status = "canceled"
			failure = "canceled before it finished"
		}
		return status, failure
	}

	// Update parent session cost on a best-effort basis, like the
	// synchronous path.
	if cerr := c.updateWorkerCost(context.Background(), sessionID, params.SessionID, previous.Cost); cerr != nil {
		slog.Warn("Update parent session cost for agent job", "job", job.id, "error", cerr)
	}

	output := subAgentOutput(result)
	if output == "" {
		return "error", "Sub-agent completed but produced no text output."
	}
	return "done", output
}

// invokeSubAgent is the shared execution core of runSubAgent and
// runAgentJob: run the agent and surface a re-auth notice when one is
// needed.
func (c *coordinator) invokeSubAgent(ctx context.Context, params subAgentParams, sessionID string, model Model, maxTokens int64, providerCfg config.ProviderConfig) (*fantasy.AgentResult, error) {
	result, err := params.Agent.Run(ctx, SessionAgentCall{
		SessionID:        sessionID,
		Prompt:           params.Prompt,
		MaxOutputTokens:  maxTokens,
		ProviderOptions:  getProviderOptions(model, providerCfg),
		Temperature:      model.ModelCfg.Temperature,
		TopP:             model.ModelCfg.TopP,
		TopK:             callTopK(providerCfg, model.ModelCfg.TopK),
		FrequencyPenalty: model.ModelCfg.FrequencyPenalty,
		PresencePenalty:  model.ModelCfg.PresencePenalty,
		NonInteractive:   true,
		OnAuthRefresh:    c.makeAuthRefreshCallback(providerCfg),
	})
	// Notify only if still unauthorized after retry. AWS SSO is handled
	// transparently inside OnAuthRefresh, so it needs no post-run notice.
	if err != nil && isUnauthorized(err) && c.notify != nil && model.ModelCfg.Provider == hyper.Name {
		c.notify.Publish(pubsub.CreatedEvent, notify.Notification{
			Type:       notify.TypeReAuthenticate,
			ProviderID: model.ModelCfg.Provider,
		})
	}
	return result, err
}

// agentJobsTool builds the agent_jobs tool over the coordinator registry.
// It is parallel-safe: every read is under the registry lock, and a
// wait-cap of 2 minutes means it never outlives a step of the parent.
func (c *coordinator) agentJobsTool() fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		AgentJobsToolName,
		agentJobsDescription,
		func(ctx context.Context, params agentJobsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			source := tools.GetSessionFromContext(ctx)
			if source == "" {
				return fantasy.NewTextErrorResponse("session id missing from context"), nil
			}
			root, err := c.teamRoot(ctx, source)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			jobs := []*agentJob{}
			for _, job := range c.registrySnapshot() {
				if job.rootSessionID == root {
					jobs = append(jobs, job)
				}
			}
			if params.JobID != "" {
				if _, err := c.teamWorker(ctx, source, params.JobID); err != nil {
					return fantasy.NewTextErrorResponse("background agent job not found in this team"), nil
				}
			}
			switch params.Op {
			case "", "list":
				return c.listAgentJobs(jobs), nil
			case "get":
				return c.getAgentJob(params), nil
			case "kill":
				return c.killAgentJob(params), nil
			default:
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("unknown op %q; use list, get, or kill", params.Op)), nil
			}
		},
	)
}

type agentJobsParams struct {
	Op    string `json:"op" description:"list (default), get, or kill"`
	JobID string `json:"job_id,omitempty" description:"The job id, for get and kill"`
}

func (c *coordinator) registrySnapshot() []*agentJob {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	jobs := make([]*agentJob, 0, len(c.agentJobs))
	for _, j := range c.agentJobs {
		jobs = append(jobs, j)
	}
	sort.Slice(jobs, func(i, k int) bool {
		return jobs[i].startedAt.Before(jobs[k].startedAt)
	})
	return jobs
}

func jobSeq(j *agentJob) int {
	n, _ := parseIntAfterPrefix(j.id, "aj-")
	return n
}

func parseIntAfterPrefix(id, prefix string) (int, bool) {
	if !strings.HasPrefix(id, prefix) {
		return 0, false
	}
	n := 0
	for _, r := range id[len(prefix):] {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func (c *coordinator) listAgentJobs(scoped ...[]*agentJob) fantasy.ToolResponse {
	jobs := c.registrySnapshot()
	if len(scoped) > 0 {
		jobs = scoped[0]
	}
	if len(jobs) == 0 {
		return fantasy.NewTextResponse("No background agent jobs have been started.")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d background agent job(s):\n", len(jobs))
	for _, j := range jobs {
		status, _ := j.snapshot()
		elapsed := time.Since(j.startedAt).Truncate(time.Second)
		var outSize string
		if status == "done" {
			_, out := j.snapshot()
			outSize = fmt.Sprintf("%d chars output", len(out))
		}
		fmt.Fprintf(&b, "%s  %-8s %-9s %6s  %s\n", j.id, j.agent, status, elapsed.String(), outSize)
	}
	return fantasy.NewTextResponse(b.String())
}

func (c *coordinator) getAgentJob(params agentJobsParams) fantasy.ToolResponse {
	if params.JobID == "" {
		return fantasy.NewTextErrorResponse("job_id is required for get")
	}
	job := c.findJob(params.JobID)
	if job == nil {
		return fantasy.NewTextErrorResponse(
			fmt.Sprintf("background agent job not found: %s", params.JobID))
	}

	job.mu.Lock()
	status, output := job.status, job.output
	if status != "queued" && status != "running" {
		job.seen = true
	}
	job.mu.Unlock()
	elapsed := time.Since(job.startedAt).Truncate(time.Second)
	var b strings.Builder
	fmt.Fprintf(&b, "Job %s (agent: %s) status: %s, runtime %s\n", job.id, job.agent, status, elapsed.String())
	switch status {
	case "done":
		b.WriteString("Output:\n")
		b.WriteString(output)
	case "error", "canceled":
		b.WriteString("Error: ")
		b.WriteString(output)
	case "queued", "running":
		b.WriteString("Still working. Its result arrives automatically as a team notice; do not poll.")
	}
	return fantasy.NewTextResponse(b.String())
}

func (c *coordinator) killAgentJob(params agentJobsParams) fantasy.ToolResponse {
	if params.JobID == "" {
		return fantasy.NewTextErrorResponse("job_id is required for kill")
	}
	job := c.findJob(params.JobID)
	if job == nil {
		return fantasy.NewTextErrorResponse(
			fmt.Sprintf("background agent job not found: %s", params.JobID))
	}
	job.mu.Lock()
	job.pending = nil
	cancel := job.cancel
	job.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return fantasy.NewTextResponse(fmt.Sprintf("Job %s canceled.", job.id))
}

func (c *coordinator) findJob(id string) *agentJob {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	return c.agentJobs[id]
}
