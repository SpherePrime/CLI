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
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const AgentJobsToolName = "agent_jobs"

// agentJobsWaitCap bounds how long agent_jobs(wait) blocks, so a batch of
// slow agents cannot stall the parent run. The call is repeatable: a timed
// out wait simply says "still running" and the model polls again.
const agentJobsWaitCap = 2 * time.Minute

// agentJob is one sub-agent started with the agent tool's background mode.
// The coordinator registry is the only record of it: the job outlives the
// step that started it and is polled through the agent_jobs tool.
type agentJob struct {
	id        string
	agent     string
	sessionID string
	prompt    string
	startedAt time.Time

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
	j.mu.Unlock()
	close(j.done)
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
	var (
		timer *time.Timer
		tick  <-chan time.Time
	)
	if timeout > 0 {
		timer = time.NewTimer(timeout)
		tick = timer.C
	}
	select {
	case <-j.done:
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
const agentJobsDescription = `Manage background agents started with agent(background=true).

Operations:
- list (default): one line per job: id, agent name, status, runtime, output size.
- get: status and output of one job. With wait=true it blocks up to 2 minutes
  for the job to finish; a job still running when the wait ends returns its
  current status, so just call again or keep working.
- wait_all: blocks up to 2 minutes until every running job is finished,
  then reports all of them.
- kill: stop a running job.

Background jobs keep running while you do other work, so starting several
and checking back later is the intended pattern. A finished job's output is
its final answer plus the files it touched, exactly like a synchronous agent
call would have returned.`

// startAgentJob registers a background sub-agent run. The sub-session is
// created synchronously so it is immediately visible in the session list
// even while the model is still running; the execution itself happens on a
// detached goroutine that survives the caller's context being canceled.
func (c *coordinator) startAgentJob(ctx context.Context, params subAgentParams, agentName string) (*agentJob, error) {
	agentToolSessionID := c.sessions.CreateAgentToolSessionID(params.AgentMessageID, params.ToolCallID)
	session, err := c.sessions.CreateTaskSession(ctx, agentToolSessionID, params.SessionID, params.SessionTitle)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	if params.SessionSetup != nil {
		params.SessionSetup(session.ID)
	}

	// Get model configuration
	model := params.Agent.Model()
	maxTokens := model.CatwalkCfg.DefaultMaxTokens
	if model.ModelCfg.MaxTokens != 0 {
		maxTokens = model.ModelCfg.MaxTokens
	}

	providerCfg, ok := c.cfg.Config().Providers.Get(model.ModelCfg.Provider)
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
	jobCtx, cancel := context.WithCancel(context.Background())

	c.jobMu.Lock()
	c.agentJobSeq++
	job := &agentJob{
		id:        fmt.Sprintf("aj-%d", c.agentJobSeq),
		agent:     agentName,
		sessionID: session.ID,
		prompt:    params.Prompt,
		startedAt: time.Now(),
		status:    "running",
		cancel:    cancel,
		done:      make(chan struct{}),
	}
	c.agentJobs[job.id] = job
	c.jobMu.Unlock()

	go c.runAgentJob(job, jobCtx, params, session.ID, model, maxTokens, providerCfg)
	return job, nil
}

// runAgentJob is the background twin of runSubAgent: same execution,
// results parked on the job instead of handed back as a tool response.
func (c *coordinator) runAgentJob(job *agentJob, ctx context.Context, params subAgentParams, sessionID string, model Model, maxTokens int64, providerCfg config.ProviderConfig) {
	result, err := c.invokeSubAgent(ctx, params, sessionID, model, maxTokens, providerCfg)
	if err != nil {
		failure := fmt.Sprintf("Failed to generate response: %s", err)
		status := "error"
		if ctx.Err() != nil || errors.Is(err, context.Canceled) {
			status = "canceled"
			failure = "canceled before it finished"
		}
		job.finish(status, failure)
		return
	}

	// Update parent session cost on a best-effort basis, like the
	// synchronous path.
	if cerr := c.updateParentSessionCost(context.Background(), sessionID, params.SessionID); cerr != nil {
		slog.Warn("Update parent session cost for agent job", "job", job.id, "error", cerr)
	}

	output := subAgentOutput(result)
	if output == "" {
		job.finish("error", "Sub-agent completed but produced no text output.")
		return
	}
	job.finish("done", output)
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
			switch params.Op {
			case "", "list":
				return c.listAgentJobs(), nil
			case "get":
				return c.getAgentJob(ctx, params)
			case "wait_all":
				return c.waitAllAgentJobs(ctx), nil
			case "kill":
				return c.killAgentJob(params), nil
			default:
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("unknown op %q; use list, get, wait_all, or kill", params.Op)), nil
			}
		},
	)
}

type agentJobsParams struct {
	Op    string `json:"op" description:"list (default), get, wait_all, or kill"`
	JobID string `json:"job_id,omitempty" description:"The job id, for get and kill"`
	Wait  bool   `json:"wait,omitempty" description:"For get: block up to 2 minutes for the job to finish"`
}

func (c *coordinator) registrySnapshot() []*agentJob {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	jobs := make([]*agentJob, 0, len(c.agentJobs))
	for _, j := range c.agentJobs {
		jobs = append(jobs, j)
	}
	sort.Slice(jobs, func(i, k int) bool {
		a, b := jobSeq(jobs[i]), jobSeq(jobs[k])
		return a < b
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

func (c *coordinator) listAgentJobs() fantasy.ToolResponse {
	jobs := c.registrySnapshot()
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

func (c *coordinator) getAgentJob(ctx context.Context, params agentJobsParams) (fantasy.ToolResponse, error) {
	if params.JobID == "" {
		return fantasy.NewTextErrorResponse("job_id is required for get"), nil
	}
	job := c.findJob(params.JobID)
	if job == nil {
		return fantasy.NewTextErrorResponse(
			fmt.Sprintf("background agent job not found: %s", params.JobID)), nil
	}

	if params.Wait {
		if !job.wait(ctx, agentJobsWaitCap) {
			status, _ := job.snapshot()
			return fantasy.NewTextResponse(fmt.Sprintf(
				"Job %s is still %s after a 2 minute wait. Call agent_jobs(get) again to poll, or keep working and check back later.",
				job.id, status)), nil
		}
	}

	status, output := job.snapshot()
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
	}
	return fantasy.NewTextResponse(b.String()), nil
}

func (c *coordinator) waitAllAgentJobs(ctx context.Context) fantasy.ToolResponse {
	deadline := time.Now().Add(agentJobsWaitCap)
	for _, job := range c.registrySnapshot() {
		status, _ := job.snapshot()
		if status != "running" {
			continue
		}
		_ = job.wait(ctx, time.Until(deadline))
	}
	return c.listAgentJobs()
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
	if cancel := job.cancel; cancel != nil {
		cancel()
	}
	return fantasy.NewTextResponse(fmt.Sprintf("Job %s canceled.", job.id))
}

func (c *coordinator) findJob(id string) *agentJob {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	return c.agentJobs[id]
}
