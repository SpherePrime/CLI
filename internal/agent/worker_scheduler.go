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

func (c *coordinator) wakeOnAgentJob(job *agentJob) {
	if c.currentAgent() == nil && c.backgroundWakeRunner == nil {
		return
	}
	job.mu.Lock()
	generation := job.generation
	shouldWake := job.wake && !job.seen
	job.mu.Unlock()
	if !shouldWake {
		return
	}
	go func() {
		ctx := message.WithHiddenUserMessage(c.workerContext())
		for {
			if ctx.Err() != nil {
				return
			}
			if a := c.currentAgent(); a != nil && a.IsSessionBusy(job.rootSessionID) {
				select {
				case <-ctx.Done():
					return
				case <-time.After(backgroundWakeIdlePoll):
				}
				continue
			}
			job.mu.Lock()
			if job.seen || generation != job.generation || job.status == "running" || job.status == "queued" {
				job.mu.Unlock()
				return
			}
			job.seen = true
			prompt := fmt.Sprintf("Worker %s (%s), job %s, finished with status %s.\n%s\n\nThis is an automatic team notice. Continue the task using this result. Do not restart completed work or reply to this notice if no work remains.", job.name, job.agent, job.id, job.status, job.output)
			job.mu.Unlock()
			if c.backgroundWakeRunner != nil {
				c.backgroundWakeRunner(ctx, job.rootSessionID, prompt)
			} else {
				_, _ = c.Run(ctx, job.rootSessionID, prompt)
			}
			return
		}
	}()
}

func (c *coordinator) workerContext() context.Context {
	if c.workerLifecycle != nil {
		return c.workerLifecycle
	}
	return context.Background()
}
