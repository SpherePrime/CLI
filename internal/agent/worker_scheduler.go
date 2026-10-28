package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/permission"
)

func (c *coordinator) reserveWorker(job *agentJob) bool {
	c.jobMu.Lock()
	defer c.jobMu.Unlock()
	limit := c.cfg.Config().Options.MaxConcurrentAgents
	if limit <= 0 {
		limit = 10
	}
	job.mu.Lock()
	if job.status == "running" {
		job.mu.Unlock()
		return true
	}
	job.mu.Unlock()
	active := 0
	job.mu.Lock()
	queuedAt := job.queuedAt
	job.mu.Unlock()
	earlier := false
	for _, other := range c.agentJobs {
		other.mu.Lock()
		if other.rootSessionID == job.rootSessionID && other.status == "running" {
			active++
		}
		if other != job && other.rootSessionID == job.rootSessionID && other.status == "queued" && other.queuedAt.Before(queuedAt) {
			earlier = true
		}
		other.mu.Unlock()
	}
	if active >= limit || earlier {
		return false
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.status == "running" {
		return true
	}
	if job.status != "queued" {
		return false
	}
	job.status = "running"
	job.updatedAt = time.Now()
	return true
}

func (c *coordinator) executeWorker(job *agentJob, ctx context.Context) {
	defer func() {
		if failure := recover(); failure != nil {
			job.finish("error", fmt.Sprintf("Agent stopped unexpectedly: %v", failure))
			c.wakeOnAgentJob(job)
		}
	}()
	for {
		for !c.reserveWorker(job) {
			select {
			case <-ctx.Done():
				job.finish("canceled", "canceled before it finished")
				c.wakeOnAgentJob(job)
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
		if ctx.Err() != nil {
			job.finish("canceled", "canceled before it finished")
			c.wakeOnAgentJob(job)
			return
		}
		job.mu.Lock()
		params := job.params
		if len(job.pending) > 0 {
			pending := strings.Join(job.pending, "\n\n")
			if !job.hasRun {
				params.Prompt += "\n\n" + pending
			} else {
				params.Prompt = pending
			}
			job.pending = nil
		}
		job.hasRun = true
		job.mu.Unlock()
		permission.InheritSessionSkipRequests(c.permissions, job.parentSessionID, job.sessionID)
		model := params.Agent.Model()
		provider, ok := c.cfg.Config().Providers.Get(model.ModelCfg.Provider)
		if !ok {
			job.finish("error", errModelProviderNotConfigured.Error())
			c.wakeOnAgentJob(job)
			return
		}
		status, output := c.runAgentJob(job, ctx, params, job.sessionID, model, callMaxOutputTokens(model, provider.Type), provider)
		job.mu.Lock()
		if len(job.pending) > 0 && ctx.Err() == nil {
			job.status = "queued"
			job.queuedAt = time.Now()
			job.generation++
			job.seen = false
			job.mu.Unlock()
			continue
		}
		job.status = status
		job.output = output
		job.updatedAt = time.Now()
		close(job.done)
		job.mu.Unlock()
		c.wakeOnAgentJob(job)
		return
	}
}

func (c *coordinator) updateWorkerCost(ctx context.Context, childID, parentID string, previousCost float64) error {
	c.workerCostMu.Lock()
	defer c.workerCostMu.Unlock()
	child, err := c.sessions.Get(ctx, childID)
	if err != nil {
		return err
	}
	parent, err := c.sessions.Get(ctx, parentID)
	if err != nil {
		return err
	}
	parent.Cost += child.Cost - previousCost
	_, err = c.sessions.Save(ctx, parent)
	return err
}

var teamNoticeSettle = 300 * time.Millisecond

func (c *coordinator) wakeOnAgentJob(job *agentJob) {
	if c.currentAgent() == nil && c.backgroundWakeRunner == nil {
		return
	}
	job.mu.Lock()
	shouldWake := job.wake && !job.seen && job.status != "running" && job.status != "queued"
	root := job.rootSessionID
	job.mu.Unlock()
	if !shouldWake {
		return
	}
	c.teamNoticeMu.Lock()
	if c.teamNoticePending == nil {
		c.teamNoticePending = map[string]bool{}
	}
	if c.teamNoticePending[root] {
		c.teamNoticeMu.Unlock()
		return
	}
	c.teamNoticePending[root] = true
	c.teamNoticeMu.Unlock()
	go c.deliverTeamNotices(root)
}

func (c *coordinator) deliverTeamNotices(root string) {
	ctx := message.WithHiddenUserMessage(c.workerContext())
	defer func() {
		c.teamNoticeMu.Lock()
		delete(c.teamNoticePending, root)
		c.teamNoticeMu.Unlock()
		if ctx.Err() == nil {
			for _, job := range c.registrySnapshot() {
				if job.rootSessionID == root {
					c.wakeOnAgentJob(job)
				}
			}
		}
	}()
	select {
	case <-ctx.Done():
	case <-time.After(teamNoticeSettle):
	}
	for ctx.Err() == nil {
		a := c.currentAgent()
		if a != nil && a.IsSessionBusy(root) && !c.hasFailedTeamNotice(root) {
			select {
			case <-ctx.Done():
			case <-time.After(backgroundWakeIdlePoll):
			}
			continue
		}
		prompt, claims := c.collectTeamNotices(root)
		if prompt == "" {
			return
		}
		if c.dispatchTeamNotice(ctx, root, prompt) {
			for _, claim := range claims {
				claim.job.mu.Lock()
				if claim.job.generation == claim.generation {
					claim.job.seen = true
				}
				claim.job.mu.Unlock()
			}
			return
		}
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
}

func (c *coordinator) dispatchTeamNotice(ctx context.Context, root, prompt string) bool {
	if c.backgroundWakeRunner != nil {
		c.backgroundWakeRunner(ctx, root, prompt)
		return true
	}
	marker := &runAcceptance{accepted: make(chan struct{})}
	dispatchContext := context.WithValue(ctx, runAcceptanceKey{}, marker)
	finished := make(chan error, 1)
	go func() {
		_, err := c.Run(dispatchContext, root, prompt)
		finished <- err
	}()
	select {
	case <-marker.accepted:
		return true
	case err := <-finished:
		return err == nil || marker.confirmed()
	case <-ctx.Done():
		return marker.confirmed()
	}
}

func (c *coordinator) hasFailedTeamNotice(root string) bool {
	for _, job := range c.registrySnapshot() {
		job.mu.Lock()
		failed := job.rootSessionID == root && job.wake && !job.seen && (job.status == "error" || job.status == "canceled")
		job.mu.Unlock()
		if failed {
			return true
		}
	}
	return false
}

type teamNoticeClaim struct {
	job        *agentJob
	generation uint64
}

func (c *coordinator) collectTeamNotices(root string) (string, []teamNoticeClaim) {
	var reports []string
	var claims []teamNoticeClaim
	for _, job := range c.registrySnapshot() {
		job.mu.Lock()
		ready := job.rootSessionID == root && job.wake && !job.seen && job.status != "running" && job.status != "queued"
		if ready {
			claims = append(claims, teamNoticeClaim{job: job, generation: job.generation})
			reports = append(reports, fmt.Sprintf("Agent %s (%s, id %s) finished with status %s.\n%s", job.name, job.agent, job.id, job.status, job.output))
		}
		job.mu.Unlock()
	}
	if len(reports) == 0 {
		return "", nil
	}
	return "Automatic team notice with new results from your agents:\n\n" + strings.Join(reports, "\n\n---\n\n") + "\n\nIntegrate these results and assign follow-up or review work if needed. Agents with status error or canceled have stopped: do not keep waiting for them. Diagnose the failure, continue their work yourself or give a worker a new assignment. Results you already handled are not repeated here. If nothing remains to do, finish with a one-line status without restating earlier reports.", claims
}

func (c *coordinator) workerContext() context.Context {
	if c.workerLifecycle != nil {
		return c.workerLifecycle
	}
	return context.Background()
}
